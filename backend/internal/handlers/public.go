package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"childconnect/internal/audit"
	"childconnect/internal/httpx"
	"childconnect/internal/rules"
)

func (s *Server) lang(r *http.Request) string {
	fallback := ""
	if u := httpx.UserFrom(r.Context()); u != nil {
		fallback = u.Language
	}
	return rules.RequestLang(r, fallback)
}

// POST /public/eligibility
func (s *Server) eligibility(w http.ResponseWriter, r *http.Request) error {
	var in rules.EligibilityInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	res, err := rules.CheckEligibility(in, s.lang(r), time.Now().In(rules.IST))
	if err != nil {
		return httpx.E(httpx.CodeValidation, err.Error())
	}
	httpx.OK(w, res)
	return nil
}

// GET /public/fees
func (s *Server) fees(w http.ResponseWriter, r *http.Request) error {
	httpx.OK(w, rules.OfficialFees(s.lang(r)))
	return nil
}

// GET /public/agencies?q=&state=
func (s *Server) publicAgencies(w http.ResponseWriter, r *http.Request) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if len(q) > 100 || len(state) > 60 {
		return httpx.E(httpx.CodeValidation, "Search text is too long.")
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT name, reg_number, agency_type, district, state, is_approved FROM agencies
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR reg_number ILIKE '%' || $1 || '%' OR district ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR state ILIKE $2)
		  AND (is_approved OR reviewed_at IS NULL)
		ORDER BY is_approved DESC, name LIMIT 50`, q, state)
	if err != nil {
		return err
	}
	defer rows.Close()
	type ag struct {
		Name      string `json:"name"`
		RegNumber string `json:"reg_number"`
		Type      string `json:"type"`
		District  string `json:"district"`
		State     string `json:"state"`
		Verified  bool   `json:"verified"`
	}
	out := []ag{}
	for rows.Next() {
		var a ag
		if err := rows.Scan(&a.Name, &a.RegNumber, &a.Type, &a.District, &a.State, &a.Verified); err != nil {
			return err
		}
		out = append(out, a)
	}
	httpx.OK(w, map[string]any{"agencies": out})
	return rows.Err()
}

// POST /public/scam-check: 5 per minute per IP address.
func (s *Server) scamCheck(w http.ResponseWriter, r *http.Request) error {
	if !s.Limiter.Allow("scam:"+httpx.ClientIP(r), s.Cfg.ScamRatePerMin) {
		return httpx.E(httpx.CodeRateLimited, "Too many checks. Please wait a minute and try again.")
	}
	var in struct {
		Text     string `json:"text" validate:"required,min=10,max=4000"`
		Language string `json:"language"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	lang := in.Language
	if !rules.ValidLang(lang) {
		lang = s.lang(r)
	}
	res := s.AI.CheckScam(r.Context(), in.Text, lang)
	actorID := actorPtr(httpx.UserFrom(r.Context()))
	_, _ = s.AI.RecordRun(r.Context(), "SCAM_CHECK", actorID, "", nil, res.Model, res.LatencyMs, map[string]any{"verdict": res.Verdict, "reasons": res.Reasons}, nil)

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	token := "rt_" + hex.EncodeToString(b)
	s.scamMu.Lock()
	for k, v := range s.scamChecks {
		if time.Since(v.at) > time.Hour {
			delete(s.scamChecks, k)
		}
	}
	s.scamChecks[token] = scamCheck{text: in.Text, verdict: res.Verdict, reasons: res.Reasons, reporter: actorID, at: time.Now()}
	s.scamMu.Unlock()

	httpx.OK(w, map[string]any{"verdict": res.Verdict, "reasons": res.Reasons, "advice": res.Advice, "report_token": token})
	return nil
}

// POST /public/scam-report
func (s *Server) scamReport(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ReportToken string `json:"report_token" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	s.scamMu.Lock()
	c, ok := s.scamChecks[in.ReportToken]
	delete(s.scamChecks, in.ReportToken)
	s.scamMu.Unlock()
	if !ok {
		return httpx.E(httpx.CodeNotFound, "This check has expired or was already reported. Please check the message again.")
	}
	reasons, _ := json.Marshal(c.reasons)
	var id string
	if err := s.Pool.QueryRow(r.Context(), `INSERT INTO scam_reports (reporter_id, text, verdict, reasons) VALUES ($1,$2,$3,$4) RETURNING id`,
		c.reporter, c.text, c.verdict, reasons).Scan(&id); err != nil {
		return err
	}
	e := audit.FromUser(httpx.UserFrom(r.Context()), "SCAM_REPORTED", "scam_report", id, map[string]any{"verdict": c.verdict})
	if e.IP == "" {
		e.IP = httpx.ClientIP(r)
	}
	_ = s.Audit.Log(r.Context(), e)
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

// GET /public/stats (PROPOSED P10): public figures from section 3.8, always with the year.
func (s *Server) publicStats(w http.ResponseWriter, r *http.Request) error {
	httpx.OK(w, map[string]any{
		"source": "Child Connect guide section 3.8 (public figures; always approximate)",
		"facts": []map[string]any{
			{"key": "adoptions_2025_26", "value": 5022, "year": "2025-26", "note": "about 4,603 within India and 419 international; 392 children with special needs"},
			{"key": "waiting_parents", "value": "32,000 to 36,000", "year": "2024-25"},
			{"key": "legally_free_children", "value": "2,100 to 2,650", "year": "2025", "note": "65% to 80% have special needs"},
			{"key": "parents_per_child", "value": 13, "year": "2025"},
			{"key": "average_wait_young_healthy_child_years", "value": "3 to 3.5"},
		},
	})
	return nil
}
