// Package handlers implements the REST API in section 15. Every request passes through request ID,
// CORS, rate limits, authentication, role checks and audit (section 12.2).
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"childconnect/internal/ai"
	"childconnect/internal/ai/risk"
	"childconnect/internal/audit"
	"childconnect/internal/auth"
	"childconnect/internal/config"
	"childconnect/internal/crypto"
	"childconnect/internal/httpx"
	"childconnect/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Pool    *pgxpool.Pool
	Cfg     *config.Config
	Box     *crypto.Box
	Audit   *audit.Logger
	Store   *storage.Local
	AI      *ai.Service
	Risk    *risk.Model
	Tokens  *auth.Tokens
	Limiter *auth.Limiter

	scamMu     sync.Mutex
	scamChecks map[string]scamCheck // report_token -> checked message, kept for 1 hour
}

type scamCheck struct {
	text     string
	verdict  string
	reasons  []string
	reporter *uuid.UUID
	at       time.Time
}

func (s *Server) Routes() http.Handler {
	if s.scamChecks == nil {
		s.scamChecks = map[string]scamCheck{}
	}
	r := chi.NewRouter()
	r.Use(httpx.RequestID, middleware.Recoverer, s.logRequests, s.cors, s.securityHeaders)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.OK(w, map[string]any{"ok": true, "ai_enabled": s.AI.Enabled(), "model": s.AI.ModelName()})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.authenticate)
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			httpx.OK(w, map[string]any{"ok": true, "ai_enabled": s.AI.Enabled(), "model": s.AI.ModelName(), "embed_model": s.AI.EmbedModelName()})
		})
		r.Get("/files/{prefix}/{name}", s.serveFile)

		// Public (section 15.2)
		r.Method("POST", "/public/eligibility", httpx.Handler(s.eligibility))
		r.Method("GET", "/public/fees", httpx.Handler(s.fees))
		r.Method("GET", "/public/agencies", httpx.Handler(s.publicAgencies))
		r.Method("POST", "/public/scam-check", httpx.Handler(s.scamCheck))
		r.Method("POST", "/public/scam-report", httpx.Handler(s.scamReport))
		r.Method("GET", "/public/stats", httpx.Handler(s.publicStats))

		// Login (section 15.3)
		r.Method("POST", "/auth/register", httpx.Handler(s.register))
		r.Method("POST", "/auth/login", httpx.Handler(s.login))
		r.Method("POST", "/auth/refresh", httpx.Handler(s.refresh))
		r.Method("POST", "/auth/logout", httpx.Handler(s.logout))
		r.With(s.require()).Method("GET", "/auth/me", httpx.Handler(s.me))

		// Notifications for every signed-in role (PROPOSED P9)
		r.With(s.require()).Method("GET", "/notifications", httpx.Handler(s.listNotifications))
		r.With(s.require()).Method("POST", "/notifications/read-all", httpx.Handler(s.readAllNotifications))
		r.With(s.require()).Method("POST", "/notifications/{id}/read", httpx.Handler(s.readNotification))

		// Parent (section 15.4)
		r.Group(func(r chi.Router) {
			r.Use(s.require(httpx.RoleParent))
			r.Method("POST", "/applications", httpx.Handler(s.createApplication))
			r.Method("GET", "/applications/me", httpx.Handler(s.myApplication))
			r.Method("PATCH", "/applications/me", httpx.Handler(s.updateApplication))
			r.Method("GET", "/applications/me/tracker", httpx.Handler(s.tracker))
			r.Method("POST", "/applications/me/documents", httpx.Handler(s.uploadDocument))
			r.Method("POST", "/applications/me/submit", httpx.Handler(s.submitDocuments))
			r.Method("GET", "/referrals/active", httpx.Handler(s.activeReferral))
			r.Method("POST", "/referrals/{id}/decision", httpx.Handler(s.referralDecision))
			r.Method("GET", "/followups/me", httpx.Handler(s.myFollowups))
			r.With(s.rateAI).Method("POST", "/ai/assistant/escalate", httpx.Handler(s.escalate))
		})

		// Parent and agency (section 15.5)
		r.Group(func(r chi.Router) {
			r.Use(s.require(httpx.RoleParent, httpx.RoleAgency))
			r.Method("GET", "/documents/{id}/url", httpx.Handler(s.documentURL))
			r.Method("GET", "/applications/{id}/messages", httpx.Handler(s.listMessages))
			r.Method("POST", "/applications/{id}/messages", httpx.Handler(s.sendMessage))
			r.With(s.rateAI).Method("POST", "/ai/assistant", httpx.Handler(s.assistant))
		})

		// Agency (section 15.6), own agency only
		r.Group(func(r chi.Router) {
			r.Use(s.require(httpx.RoleAgency), s.requireApprovedAgency)
			r.Method("GET", "/agency/cases", httpx.Handler(s.agencyCases))
			r.Method("GET", "/agency/cases/{id}", httpx.Handler(s.agencyCase))
			r.Method("PATCH", "/agency/documents/{id}/review", httpx.Handler(s.reviewDocument))
			r.Method("POST", "/agency/cases/{id}/verify-all", httpx.Handler(s.verifyAll))
			r.Method("POST", "/agency/cases/{id}/events", httpx.Handler(s.recordEvent))
			r.Method("PUT", "/agency/hsr/{id}", httpx.Handler(s.saveHSR))
			r.Method("POST", "/agency/hsr/{id}/sign", httpx.Handler(s.signHSR))
			r.Method("POST", "/agency/children", httpx.Handler(s.addChild))
			r.Method("GET", "/agency/children", httpx.Handler(s.listChildren))
			r.Method("GET", "/agency/children/{id}", httpx.Handler(s.getChild))
			r.Method("POST", "/agency/children/{id}/reports", httpx.Handler(s.uploadChildReports))
			r.Method("POST", "/agency/children/{id}/summary/approve", httpx.Handler(s.approveSummary))
			r.Method("GET", "/agency/children/{id}/next-parent", httpx.Handler(s.nextParent))
			r.Method("POST", "/agency/referrals", httpx.Handler(s.sendReferral))
			r.Method("POST", "/agency/followups/{id}/sign", httpx.Handler(s.signFollowup))
			r.With(s.rateAI).Method("POST", "/ai/hsr/draft", httpx.Handler(s.hsrDraft))
			r.With(s.rateAI).Method("POST", "/ai/medical-summary", httpx.Handler(s.medicalSummary))
			r.With(s.rateAI).Method("POST", "/ai/followup/draft", httpx.Handler(s.followupDraft))
		})

		// Admin (section 15.7)
		r.Group(func(r chi.Router) {
			r.Use(s.require(httpx.RoleAdmin))
			r.Method("GET", "/admin/radar", httpx.Handler(s.radar))
			r.Method("POST", "/admin/radar/recompute", httpx.Handler(s.recompute))
			r.Method("GET", "/admin/agency-scorecard", httpx.Handler(s.scorecard))
			r.Method("GET", "/admin/child-alerts", httpx.Handler(s.childAlerts))
			r.Method("GET", "/admin/agencies", httpx.Handler(s.adminAgencies))
			r.Method("PATCH", "/admin/agencies/{id}", httpx.Handler(s.setAgencyApproval))
			r.Method("POST", "/admin/knowledge", httpx.Handler(s.addKnowledge))
			r.Method("GET", "/admin/knowledge", httpx.Handler(s.listKnowledge))
			r.Method("GET", "/admin/scam-reports", httpx.Handler(s.scamReports))
			r.Method("PATCH", "/admin/scam-reports/{id}", httpx.Handler(s.reviewScamReport))
			r.Method("GET", "/admin/audit", httpx.Handler(s.auditList))
			r.Method("GET", "/admin/audit/verify", httpx.Handler(s.auditVerify))
			r.Method("GET", "/admin/model", httpx.Handler(s.modelInfo))
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Fail(w, r, httpx.E(httpx.CodeNotFound, "This address does not exist."))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Fail(w, r, httpx.E(httpx.CodeNotFound, "This method is not supported here."))
	})
	return r
}

// ---------- middleware ----------

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		if r.Method == http.MethodOptions {
			return
		}
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "ms", time.Since(start).Milliseconds(), "rid", httpx.RequestIDFrom(r.Context()))
	})
}

// cors allows only the website's own address (section 12.2 step 2).
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == s.Cfg.FrontendOrigin {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With")
			h.Set("Access-Control-Expose-Headers", "X-Request-Id")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/v1/files/") {
			// Files may be previewed inside the website (document review), and nowhere else.
			h.Set("Content-Security-Policy", "frame-ancestors 'self' "+s.Cfg.FrontendOrigin)
		} else {
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("Cache-Control", "no-store")
		}
		if s.Cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate reads the cc_session cookie and loads the user. Anonymous requests continue without a user.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.AccessCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := s.Tokens.Parse(c.Value, "access")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		uid, err := uuid.Parse(claims.Subject)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		u, ver, err := s.loadUser(r.Context(), uid)
		if err != nil || u == nil || ver != claims.Ver {
			next.ServeHTTP(w, r)
			return
		}
		u.IP = httpx.ClientIP(r)
		next.ServeHTTP(w, r.WithContext(httpx.WithUser(r.Context(), u)))
	})
}

func (s *Server) loadUser(ctx context.Context, id uuid.UUID) (*httpx.User, int, error) {
	u := &httpx.User{}
	var role string
	var active bool
	var ver int
	err := s.Pool.QueryRow(ctx, `SELECT id, email, full_name, role::text, agency_id, designation, language, is_active, token_version FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Email, &u.FullName, &role, &u.AgencyID, &u.Designation, &u.Language, &active, &ver)
	if err == pgx.ErrNoRows || (err == nil && !active) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	u.Role = httpx.Role(role)
	return u, ver, nil
}

// require checks login and role on every request (section 6.4). Refused attempts are written to the audit log.
func (s *Server) require(roles ...httpx.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := httpx.UserFrom(r.Context())
			if u == nil {
				httpx.Fail(w, r, httpx.ErrUnauth)
				return
			}
			if len(roles) > 0 {
				ok := false
				for _, role := range roles {
					if u.Role == role {
						ok = true
					}
				}
				if !ok {
					s.forbidden(w, r, "ROLE", r.URL.Path)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// forbidden answers 403 and records the attempt.
func (s *Server) forbidden(w http.ResponseWriter, r *http.Request, targetType, targetID string) {
	u := httpx.UserFrom(r.Context())
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "FORBIDDEN_ATTEMPT", targetType, targetID, map[string]any{"method": r.Method, "path": r.URL.Path}))
	httpx.Fail(w, r, httpx.ErrForbidden)
}

func (s *Server) denied(r *http.Request, targetType, targetID string) error {
	u := httpx.UserFrom(r.Context())
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "FORBIDDEN_ATTEMPT", targetType, targetID, map[string]any{"method": r.Method, "path": r.URL.Path}))
	return httpx.ErrForbidden
}

func (s *Server) requireApprovedAgency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := httpx.UserFrom(r.Context())
		var ok bool
		if u.AgencyID == nil || s.Pool.QueryRow(r.Context(), `SELECT is_approved FROM agencies WHERE id=$1`, *u.AgencyID).Scan(&ok) != nil || !ok {
			httpx.Fail(w, r, httpx.E(httpx.CodeForbidden, "Your agency is waiting for approval by an oversight officer."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateAI limits AI endpoints to 10 requests per minute per user (section 16.2).
func (s *Server) rateAI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := httpx.UserFrom(r.Context())
		if u != nil && !s.Limiter.Allow("ai:"+u.ID.String(), s.Cfg.AIRatePerMin) {
			httpx.Fail(w, r, httpx.E(httpx.CodeRateLimited, "Too many AI requests. Please wait a minute and try again."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- small helpers ----------

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func tsStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func (s *Server) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
