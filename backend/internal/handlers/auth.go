package handlers

import (
	"net/http"
	"strings"
	"time"

	"childconnect/internal/audit"
	"childconnect/internal/auth"
	"childconnect/internal/httpx"
	"childconnect/internal/rules"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type userOut struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	AgencyID    *uuid.UUID `json:"agency_id"`
	AgencyName  *string    `json:"agency_name,omitempty"`
	FullName    string     `json:"full_name"`
	Language    string     `json:"language"`
	Designation *string    `json:"designation"`
}

func actorPtr(u *httpx.User) *uuid.UUID {
	if u == nil {
		return nil
	}
	id := u.ID
	return &id
}

func (s *Server) userJSON(r *http.Request, id uuid.UUID) (*userOut, error) {
	var o userOut
	err := s.Pool.QueryRow(r.Context(), `SELECT u.id, u.email, u.role::text, u.agency_id, a.name, u.full_name, u.language, u.designation
		FROM users u LEFT JOIN agencies a ON a.id=u.agency_id WHERE u.id=$1`, id).
		Scan(&o.ID, &o.Email, &o.Role, &o.AgencyID, &o.AgencyName, &o.FullName, &o.Language, &o.Designation)
	return &o, err
}

// POST /auth/register: only parents register themselves (section 15.3).
func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	if !s.Limiter.Allow("register:"+httpx.ClientIP(r), 10) {
		return httpx.E(httpx.CodeRateLimited, "Too many attempts. Please wait a minute.")
	}
	var in struct {
		Email    string `json:"email" validate:"required,email,max=200"`
		Phone    string `json:"phone" validate:"required,min=10,max=20"`
		Password string `json:"password" validate:"required,min=8,max=200"`
		FullName string `json:"full_name" validate:"required,min=2,max=120"`
		Language string `json:"language" validate:"required,oneof=en hi mr"`
		Consent  bool   `json:"consent"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if !in.Consent {
		return httpx.E(httpx.CodeValidation, "You must accept the privacy notice to register.")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	var id uuid.UUID
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `INSERT INTO users (email, phone, password_hash, full_name, role, language, consent_at)
			VALUES ($1,$2,$3,$4,'PARENT',$5,now()) RETURNING id`, email, strings.TrimSpace(in.Phone), hash, strings.TrimSpace(in.FullName), in.Language).Scan(&id)
		if err != nil {
			return err
		}
		e := audit.Entry{ActorID: &id, ActorRole: "PARENT", Action: "REGISTER", TargetType: "user", TargetID: id.String(), IP: httpx.ClientIP(r), Meta: map[string]any{"consent": true}}
		return audit.LogTx(r.Context(), tx, e)
	})
	if err != nil {
		if pe, ok := err.(*pgconn.PgError); ok && pe.Code == "23505" {
			return httpx.E(httpx.CodeValidation, "An account with this email already exists. Please log in.")
		}
		return err
	}
	if err := s.Tokens.SetSession(w, id, "PARENT", 0); err != nil {
		return err
	}
	u, err := s.userJSON(r, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"user": u})
	return nil
}

// POST /auth/login: 5 attempts per minute per address; 5 failures lock the account for 15 minutes.
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email" validate:"required,max=200"`
		Password string `json:"password" validate:"required,max=200"`
		OTP      string `json:"otp"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	ip := httpx.ClientIP(r)
	if !s.Limiter.Allow("login:"+ip+":"+email, s.Cfg.LoginRatePerMin) {
		return httpx.E(httpx.CodeRateLimited, "Too many login attempts. Please wait a minute and try again.")
	}
	var (
		id     uuid.UUID
		hash   string
		role   string
		active bool
		failed int
		locked *time.Time
		ver    int
	)
	err := s.Pool.QueryRow(r.Context(), `SELECT id, password_hash, role::text, is_active, failed_logins, locked_until, token_version FROM users WHERE email=$1`, email).
		Scan(&id, &hash, &role, &active, &failed, &locked, &ver)
	wrong := httpx.E(httpx.CodeUnauth, "The email or password is wrong.")
	if err == pgx.ErrNoRows {
		auth.CheckPassword("$2a$12$C6UzMDM.H6dfI/f/IKcEeO5QzBkX5S6M8yZyJ0kq3m1k0QF6u6m1e", in.Password) // same timing as a real check
		_ = s.Audit.Log(r.Context(), audit.Entry{Action: "LOGIN_FAILED", TargetType: "user", TargetID: email, IP: ip, Meta: map[string]any{"reason": "unknown_email"}})
		return wrong
	}
	if err != nil {
		return err
	}
	if !active {
		return httpx.E(httpx.CodeUnauth, "This account is disabled. Please contact the administrator.")
	}
	if locked != nil && locked.After(time.Now()) {
		return httpx.E(httpx.CodeUnauth, "This account is locked for 15 minutes after 5 failed logins. Try again after "+locked.In(rules.IST).Format("15:04")+" IST.")
	}
	if !auth.CheckPassword(hash, in.Password) {
		failed++
		if failed >= 5 {
			_, _ = s.Pool.Exec(r.Context(), `UPDATE users SET failed_logins=0, locked_until=now() + interval '15 minutes' WHERE id=$1`, id)
		} else {
			_, _ = s.Pool.Exec(r.Context(), `UPDATE users SET failed_logins=$2 WHERE id=$1`, id, failed)
		}
		_ = s.Audit.Log(r.Context(), audit.Entry{ActorID: &id, ActorRole: role, Action: "LOGIN_FAILED", TargetType: "user", TargetID: id.String(), IP: ip, Meta: map[string]any{"failed_count": failed}})
		return wrong
	}
	if _, err := s.Pool.Exec(r.Context(), `UPDATE users SET failed_logins=0, locked_until=NULL WHERE id=$1`, id); err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.Entry{ActorID: &id, ActorRole: role, Action: "LOGIN", TargetType: "user", TargetID: id.String(), IP: ip})
	if err := s.Tokens.SetSession(w, id, role, ver); err != nil {
		return err
	}
	u, err := s.userJSON(r, id)
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"user": u})
	return nil
}

// POST /auth/refresh: a new 15-minute access token from the 7-day refresh token.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(auth.RefreshCookie)
	if err != nil {
		return httpx.ErrUnauth
	}
	claims, err := s.Tokens.Parse(c.Value, "refresh")
	if err != nil {
		s.Tokens.Clear(w)
		return httpx.E(httpx.CodeUnauth, "Your session has expired. Please log in again.")
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return httpx.ErrUnauth
	}
	u, ver, err := s.loadUser(r.Context(), uid)
	if err != nil {
		return err
	}
	if u == nil || ver != claims.Ver {
		s.Tokens.Clear(w)
		return httpx.E(httpx.CodeUnauth, "Your session has expired. Please log in again.")
	}
	if err := s.Tokens.SetSession(w, u.ID, string(u.Role), ver); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

// POST /auth/logout: clears the cookies and invalidates refresh tokens.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	if u == nil {
		if c, err := r.Cookie(auth.RefreshCookie); err == nil {
			if cl, err := s.Tokens.Parse(c.Value, "refresh"); err == nil {
				if id, err := uuid.Parse(cl.Subject); err == nil {
					u, _, _ = s.loadUser(r.Context(), id)
				}
			}
		}
	}
	if u != nil {
		_, _ = s.Pool.Exec(r.Context(), `UPDATE users SET token_version = token_version + 1 WHERE id=$1`, u.ID)
		e := audit.FromUser(u, "LOGOUT", "user", u.ID.String(), nil)
		e.IP = httpx.ClientIP(r)
		_ = s.Audit.Log(r.Context(), e)
	}
	s.Tokens.Clear(w)
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

// GET /auth/me
func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	u, err := s.userJSON(r, httpx.UserFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"user": u})
	return nil
}

// ---------- notifications (PROPOSED P9) ----------

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	rows, err := s.Pool.Query(r.Context(), `SELECT id, type, title, body, link, read_at, created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type n struct {
		ID        uuid.UUID `json:"id"`
		Type      string    `json:"type"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		Link      *string   `json:"link"`
		ReadAt    *string   `json:"read_at"`
		CreatedAt string    `json:"created_at"`
	}
	out := []n{}
	unread := 0
	for rows.Next() {
		var x n
		var read *time.Time
		var created time.Time
		if err := rows.Scan(&x.ID, &x.Type, &x.Title, &x.Body, &x.Link, &read, &created); err != nil {
			return err
		}
		x.ReadAt, x.CreatedAt = tsStr(read), created.UTC().Format(time.RFC3339)
		if read == nil {
			unread++
		}
		out = append(out, x)
	}
	httpx.OK(w, map[string]any{"items": out, "unread": unread})
	return rows.Err()
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) error {
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at, now()) WHERE id=$1 AND user_id=$2`, id, httpx.UserFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request) error {
	_, err := s.Pool.Exec(r.Context(), `UPDATE notifications SET read_at=now() WHERE user_id=$1 AND read_at IS NULL`, httpx.UserFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}
