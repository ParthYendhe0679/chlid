package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"childconnect/internal/ai"
	"childconnect/internal/audit"
	"childconnect/internal/httpx"
	"childconnect/internal/rules"
	"childconnect/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const highRisk = 0.6

// GET /admin/radar?agency_id=&step=&level=&district=
func (s *Server) radar(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	var agencyID *uuid.UUID
	if v := q.Get("agency_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return httpx.E(httpx.CodeValidation, "agency_id is not valid.")
		}
		agencyID = &id
	}
	step, level, district := q.Get("step"), q.Get("level"), q.Get("district")
	rows, err := s.Pool.Query(r.Context(), `
		SELECT COALESCE('APP-' || upper(substr(a.id::text, 1, 6)), c.case_code, 'CASE'), COALESCE(ag.name, '-'), COALESCE(ag.district, ''), d.step::text,
		       d.due_at, d.risk_score::float8, d.risk_reasons, d.application_id, d.agency_id
		FROM case_deadlines d
		LEFT JOIN applications a ON a.id=d.application_id
		LEFT JOIN children c ON c.id=d.child_id
		LEFT JOIN agencies ag ON ag.id=d.agency_id
		WHERE d.state IN ('OPEN','OVERDUE')
		  AND ($1::uuid IS NULL OR d.agency_id=$1) AND ($2='' OR d.step::text=$2) AND ($3='' OR ag.district ILIKE $3)
		ORDER BY d.due_at`, agencyID, step, district)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := time.Now()
	type item struct {
		CaseRef       string     `json:"case_ref"`
		Agency        string     `json:"agency"`
		AgencyID      *uuid.UUID `json:"agency_id"`
		District      string     `json:"district"`
		ApplicationID *uuid.UUID `json:"application_id"`
		Step          string     `json:"step"`
		DueAt         string     `json:"due_at"`
		DaysLeft      int        `json:"days_left"`
		RiskScore     *float64   `json:"risk_score"`
		RiskReasons   []string   `json:"risk_reasons"`
		Level         string     `json:"level"`
	}
	summary := map[string]int{"overdue": 0, "high_risk": 0, "on_track": 0}
	items := []item{}
	for rows.Next() {
		var it item
		var due time.Time
		var reasons []byte
		if err := rows.Scan(&it.CaseRef, &it.Agency, &it.District, &it.Step, &due, &it.RiskScore, &reasons, &it.ApplicationID, &it.AgencyID); err != nil {
			return err
		}
		d := workflow.BuildDeadline(it.Step, due, it.RiskScore, reasons, now)
		it.DueAt, it.DaysLeft, it.RiskReasons = d.DueAt, d.DaysLeft, d.RiskReasons
		switch {
		case due.Before(now):
			it.Level = "OVERDUE"
			summary["overdue"]++
		case it.RiskScore != nil && *it.RiskScore >= highRisk:
			it.Level = "HIGH_RISK"
			summary["high_risk"]++
		default:
			it.Level = "ON_TRACK"
			summary["on_track"]++
		}
		if level == "" || level == it.Level {
			items = append(items, it)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Overdue first (most overdue on top), then highest predicted risk, then nearest deadline.
	rank := map[string]int{"OVERDUE": 0, "HIGH_RISK": 1, "ON_TRACK": 2}
	sortItems := func(i, j int) bool {
		a, b := items[i], items[j]
		if rank[a.Level] != rank[b.Level] {
			return rank[a.Level] < rank[b.Level]
		}
		if a.Level == "OVERDUE" {
			return a.DaysLeft < b.DaysLeft
		}
		as, bs := 0.0, 0.0
		if a.RiskScore != nil {
			as = *a.RiskScore
		}
		if b.RiskScore != nil {
			bs = *b.RiskScore
		}
		if as != bs {
			return as > bs
		}
		return a.DaysLeft < b.DaysLeft
	}
	for i := 1; i < len(items); i++ { // insertion sort keeps it simple and stable
		for j := i; j > 0 && sortItems(j, j-1); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	httpx.OK(w, map[string]any{"summary": summary, "items": items})
	return nil
}

// POST /admin/radar/recompute
func (s *Server) recompute(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	start := time.Now()
	n, err := s.Risk.Recompute(r.Context(), s.Pool)
	if err != nil {
		return err
	}
	_, _ = s.AI.RecordRun(r.Context(), "RISK_SCORE", &u.ID, "", nil, s.Risk.Name+"@"+s.Risk.Version, int(time.Since(start).Milliseconds()),
		map[string]any{"updated": n, "trigger": "admin"}, nil)
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "RISK_RECOMPUTE", "case_deadlines", "all", map[string]any{"updated": n}))
	httpx.OK(w, map[string]any{"updated": n})
	return nil
}

// GET /admin/agency-scorecard: average days per step compared with the legal limit.
func (s *Server) scorecard(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.Pool.Query(r.Context(), `
		SELECT ag.id, ag.name,
		  (SELECT count(*) FROM applications ap WHERE ap.agency_id=ag.id AND ap.status NOT IN ('COMPLETED','LAPSED','DEBARRED','WITHDRAWN')),
		  COALESCE((SELECT avg(CASE WHEN (d.completed_at IS NOT NULL AND d.completed_at > d.due_at) OR (d.completed_at IS NULL AND d.due_at < now()) THEN 1.0 ELSE 0.0 END)
		            FROM case_deadlines d WHERE d.agency_id=ag.id AND (d.state='DONE' OR d.due_at < now()) AND d.state <> 'CANCELLED'), 0)
		FROM agencies ag WHERE ag.is_approved ORDER BY ag.name`)
	if err != nil {
		return err
	}
	type score struct {
		ID        uuid.UUID          `json:"agency_id"`
		Agency    string             `json:"agency"`
		AvgDays   map[string]float64 `json:"avg_days"`
		LateRate  float64            `json:"late_rate"`
		OpenCases int                `json:"open_cases"`
	}
	var out []*score
	idx := map[uuid.UUID]*score{}
	for rows.Next() {
		sc := &score{AvgDays: map[string]float64{}}
		var late float64
		if err := rows.Scan(&sc.ID, &sc.Agency, &sc.OpenCases, &late); err != nil {
			rows.Close()
			return err
		}
		sc.LateRate = float64(int(late*100+0.5)) / 100
		out = append(out, sc)
		idx[sc.ID] = sc
	}
	rows.Close()
	// Average days for completed steps; for steps still open past their limit, days used so far.
	rows, err = s.Pool.Query(r.Context(), `SELECT agency_id, step::text, avg(EXTRACT(EPOCH FROM (COALESCE(completed_at, now()) - started_at))/86400)
		FROM case_deadlines WHERE agency_id IS NOT NULL AND state <> 'CANCELLED' AND (state='DONE' OR due_at < now()) AND step <> 'FOLLOWUP_VISIT'
		GROUP BY agency_id, step`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		var step string
		var avg float64
		if err := rows.Scan(&id, &step, &avg); err != nil {
			rows.Close()
			return err
		}
		if sc, ok := idx[id]; ok {
			sc.AvgDays[step] = float64(int(avg*10+0.5)) / 10
		}
	}
	rows.Close()
	if out == nil {
		out = []*score{}
	}
	httpx.OK(w, out)
	return nil
}

// GET /admin/child-alerts: masked data only (case codes).
func (s *Server) childAlerts(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.Pool.Query(r.Context(), `
		SELECT c.case_code, ag.name, 'NO_FAMILY_VISIT_12M', (current_date - COALESCE(c.last_family_visit_on, c.admitted_on))
		FROM children c JOIN agencies ag ON ag.id=c.agency_id
		WHERE c.status IN ('IN_CARE','LEGALLY_FREE','READY') AND COALESCE(c.last_family_visit_on, c.admitted_on) <= current_date - interval '12 months'
		UNION ALL
		SELECT c.case_code, ag.name, 'REPORTS_MISSING_10D', (current_date - c.lfa_date)
		FROM children c JOIN agencies ag ON ag.id=c.agency_id
		WHERE c.lfa_date IS NOT NULL AND c.lfa_date <= current_date - 10 AND (c.csr_storage_key IS NULL OR c.mer_storage_key IS NULL)
		ORDER BY 4 DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var code, agency, reason string
		var days int
		if err := rows.Scan(&code, &agency, &reason, &days); err != nil {
			return err
		}
		out = append(out, map[string]any{"case_code": code, "agency": agency, "reason": reason, "days": days})
	}
	httpx.OK(w, out)
	return rows.Err()
}

type agencyOut struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	RegNumber  string    `json:"reg_number"`
	AgencyType string    `json:"agency_type"`
	State      string    `json:"state"`
	District   string    `json:"district"`
	StaffCount int       `json:"staff_count"`
	Phone      *string   `json:"phone"`
	Email      *string   `json:"email"`
	IsApproved bool      `json:"is_approved"`
	ReviewedAt *string   `json:"reviewed_at"`
	CreatedAt  string    `json:"created_at"`
}

const agencyCols = `id, name, reg_number, agency_type, state, district, staff_count, phone, email, is_approved, reviewed_at, created_at`

func scanAgency(row pgx.Row) (*agencyOut, error) {
	var a agencyOut
	var rev *time.Time
	var created time.Time
	err := row.Scan(&a.ID, &a.Name, &a.RegNumber, &a.AgencyType, &a.State, &a.District, &a.StaffCount, &a.Phone, &a.Email, &a.IsApproved, &rev, &created)
	if err != nil {
		return nil, err
	}
	a.ReviewedAt, a.CreatedAt = tsStr(rev), created.UTC().Format(time.RFC3339)
	return &a, nil
}

// GET /admin/agencies?approved=false lists agencies waiting for approval.
func (s *Server) adminAgencies(w http.ResponseWriter, r *http.Request) error {
	where := "TRUE"
	switch r.URL.Query().Get("approved") {
	case "false":
		where = "NOT is_approved AND reviewed_at IS NULL"
	case "true":
		where = "is_approved"
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT `+agencyCols+` FROM agencies WHERE `+where+` ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*agencyOut{}
	for rows.Next() {
		a, err := scanAgency(rows)
		if err != nil {
			return err
		}
		out = append(out, a)
	}
	httpx.OK(w, out)
	return rows.Err()
}

// PATCH /admin/agencies/:id
func (s *Server) setAgencyApproval(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		IsApproved *bool `json:"is_approved" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	var a *agencyOut
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var err error
		a, err = scanAgency(tx.QueryRow(r.Context(), `UPDATE agencies SET is_approved=$2, reviewed_at=now() WHERE id=$1 RETURNING `+agencyCols, id, *in.IsApproved))
		if err == pgx.ErrNoRows {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		action := "REJECT_AGENCY"
		if *in.IsApproved {
			action = "APPROVE_AGENCY"
			_, _ = tx.Exec(r.Context(), `INSERT INTO notifications (user_id, type, title, body, link)
				SELECT id, 'AGENCY_APPROVED', 'Agency approved', 'Your agency was approved. You can now use the agency pages.', '/agency' FROM users WHERE agency_id=$1`, id)
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, action, "agency", id.String(), map[string]any{"reg_number": a.RegNumber}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, a)
	return nil
}

var headingRe = regexp.MustCompile(`(?m)^\s*(?:#+\s*)?((?:Regulation|Section|Schedule|Rule|FAQ|Fact|विनियम|धारा|नियम|तथ्य)\s*[^\n]{1,80})$`)

// SplitKnowledge splits a text into cited parts: each heading line ("Regulation 9(3) ...") starts a part;
// long parts are cut into about 500 words with overlap. Without headings, parts are cited as "Part n".
func SplitKnowledge(text string) [][2]string {
	idx := headingRe.FindAllStringSubmatchIndex(text, -1)
	var out [][2]string
	if len(idx) == 0 {
		parts := ai.ChunkText(text)
		for i, p := range parts {
			out = append(out, [2]string{fmt.Sprintf("Part %d of %d", i+1, len(parts)), p})
		}
		return out
	}
	if pre := strings.TrimSpace(text[:idx[0][0]]); len(strings.Fields(pre)) >= 20 {
		for _, p := range ai.ChunkText(pre) {
			out = append(out, [2]string{"Introduction", p})
		}
	}
	for i, m := range idx {
		end := len(text)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		cite := strings.TrimRight(strings.TrimSpace(text[m[2]:m[3]]), ":.-")
		body := strings.TrimSpace(text[m[1]:end])
		if body == "" {
			continue
		}
		short := cite
		for _, sep := range []string{": ", " - "} {
			if j := strings.Index(cite, sep); j > 0 && j < 45 {
				short = strings.TrimSpace(cite[:j])
				break
			}
		}
		for _, p := range ai.ChunkText(cite + ". " + body) {
			out = append(out, [2]string{short, p})
		}
	}
	return out
}

// POST /admin/knowledge
func (s *Server) addKnowledge(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		SourceTitle string `json:"source_title" validate:"required,min=3,max=200"`
		SourceURL   string `json:"source_url" validate:"omitempty,url,max=500"`
		Language    string `json:"language" validate:"required,oneof=en hi mr"`
		Text        string `json:"text" validate:"required,min=100,max=400000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	parts := SplitKnowledge(in.Text)
	texts := make([]string, len(parts))
	for i, p := range parts {
		texts[i] = in.SourceTitle + ", " + p[0] + ": " + p[1]
	}
	vecs, model, err := s.AI.Embed(r.Context(), texts, "RETRIEVAL_DOCUMENT")
	if err != nil {
		return httpx.E(httpx.CodeAIUnavailble, "The embedding service is not available right now. Please try again.")
	}
	var url *string
	if in.SourceURL != "" {
		url = &in.SourceURL
	}
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		b := &pgx.Batch{}
		for i, p := range parts {
			b.Queue(`INSERT INTO knowledge_chunks (source_title, citation, source_url, language, content, embedding, embed_model) VALUES ($1,$2,$3,$4,$5,$6::vector,$7)`,
				in.SourceTitle, p[0], url, in.Language, p[1], ai.VectorLiteral(vecs[i]), model)
		}
		if err := tx.SendBatch(r.Context(), b).Close(); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "KNOWLEDGE_ADDED", "knowledge", in.SourceTitle, map[string]any{"chunks": len(parts), "language": in.Language}))
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"chunks_created": len(parts), "embed_model": model})
	return nil
}

// GET /admin/knowledge (PROPOSED P12)
func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.Pool.Query(r.Context(), `SELECT source_title, language, embed_model, count(*), max(created_at), min(source_url)
		FROM knowledge_chunks GROUP BY source_title, language, embed_model ORDER BY max(created_at) DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var title, lang, model string
		var n int
		var at time.Time
		var url *string
		if err := rows.Scan(&title, &lang, &model, &n, &at, &url); err != nil {
			return err
		}
		out = append(out, map[string]any{"source_title": title, "language": lang, "embed_model": model, "chunks": n, "added_at": at.UTC().Format(time.RFC3339), "source_url": url})
	}
	httpx.OK(w, out)
	return rows.Err()
}

// GET /admin/scam-reports
func (s *Server) scamReports(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.Pool.Query(r.Context(), `SELECT id, text, verdict, reasons, reviewed, created_at FROM scam_reports ORDER BY reviewed, created_at DESC LIMIT 200`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var text, verdict string
		var reasons []byte
		var reviewed bool
		var at time.Time
		if err := rows.Scan(&id, &text, &verdict, &reasons, &reviewed, &at); err != nil {
			return err
		}
		var rs []string
		_ = json.Unmarshal(reasons, &rs)
		if rs == nil {
			rs = []string{}
		}
		out = append(out, map[string]any{"id": id, "text": text, "verdict": verdict, "reasons": rs, "reviewed": reviewed, "created_at": at.UTC().Format(time.RFC3339)})
	}
	httpx.OK(w, out)
	return rows.Err()
}

// PATCH /admin/scam-reports/:id (PROPOSED P14)
func (s *Server) reviewScamReport(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Reviewed *bool `json:"reviewed" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	tag, err := s.Pool.Exec(r.Context(), `UPDATE scam_reports SET reviewed=$2 WHERE id=$1`, id, *in.Reviewed)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "SCAM_REPORT_REVIEWED", "scam_report", id.String(), map[string]any{"reviewed": *in.Reviewed}))
	httpx.OK(w, map[string]any{"ok": true, "reviewed": *in.Reviewed})
	return nil
}

// GET /admin/audit?actor=&action=&from=&to=
func (s *Server) auditList(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	actor, action := strings.TrimSpace(q.Get("actor")), strings.TrimSpace(q.Get("action"))
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		d, err := rules.ParseDate(v)
		if err != nil {
			return httpx.E(httpx.CodeValidation, "'from' must be a date (YYYY-MM-DD).")
		}
		from = &d
	}
	if v := q.Get("to"); v != "" {
		d, err := rules.ParseDate(v)
		if err != nil {
			return httpx.E(httpx.CodeValidation, "'to' must be a date (YYYY-MM-DD).")
		}
		d = d.Add(24 * time.Hour)
		to = &d
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT l.id, l.actor_id, l.actor_role::text, u.full_name, l.action, l.target_type, l.target_id, l.ip, l.meta, l.created_at, l.hash
		FROM audit_logs l LEFT JOIN users u ON u.id=l.actor_id
		WHERE ($1='' OR l.actor_id::text ILIKE $1 || '%' OR l.actor_role::text ILIKE $1 OR u.email ILIKE '%' || $1 || '%' OR u.full_name ILIKE '%' || $1 || '%')
		  AND ($2='' OR l.action ILIKE '%' || $2 || '%')
		  AND ($3::timestamptz IS NULL OR l.created_at >= $3) AND ($4::timestamptz IS NULL OR l.created_at < $4)
		ORDER BY l.id DESC LIMIT 300`, actor, action, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var actorID *uuid.UUID
		var role, name, ip *string
		var act, tt, tid, hash string
		var meta []byte
		var at time.Time
		if err := rows.Scan(&id, &actorID, &role, &name, &act, &tt, &tid, &ip, &meta, &at, &hash); err != nil {
			return err
		}
		var m map[string]any
		_ = json.Unmarshal(meta, &m)
		out = append(out, map[string]any{"id": id, "actor_id": actorID, "actor_role": role, "actor_name": name, "action": act, "target_type": tt,
			"target_id": tid, "ip": ip, "meta": m, "created_at": at.UTC().Format(time.RFC3339), "hash": hash})
	}
	httpx.OK(w, out)
	return rows.Err()
}

// GET /admin/audit/verify
func (s *Server) auditVerify(w http.ResponseWriter, r *http.Request) error {
	res, err := s.Audit.Verify(r.Context())
	if err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(httpx.UserFrom(r.Context()), "AUDIT_VERIFY", "audit_logs", "chain", map[string]any{"valid": res.Valid, "rows_checked": res.RowsChecked}))
	httpx.OK(w, res)
	return nil
}

// GET /admin/model: the risk model card and the AI status, for transparency.
func (s *Server) modelInfo(w http.ResponseWriter, r *http.Request) error {
	var runs []map[string]any
	rows, err := s.Pool.Query(r.Context(), `SELECT feature, model, count(*), count(human_decision), count(*) FILTER (WHERE human_decision='ACCEPTED'),
		count(*) FILTER (WHERE human_decision='EDITED'), count(*) FILTER (WHERE human_decision='REJECTED'), avg(latency_ms)::int
		FROM ai_runs GROUP BY feature, model ORDER BY feature`)
	if err == nil {
		for rows.Next() {
			var f, m string
			var n, decided, acc, ed, rej int
			var lat *int
			if rows.Scan(&f, &m, &n, &decided, &acc, &ed, &rej, &lat) == nil {
				runs = append(runs, map[string]any{"feature": f, "model": m, "runs": n, "human_reviewed": decided, "accepted": acc, "edited": ed, "rejected": rej, "avg_latency_ms": lat})
			}
		}
		rows.Close()
	}
	httpx.OK(w, map[string]any{
		"risk_model":  map[string]any{"name": s.Risk.Name, "version": s.Risk.Version, "data": s.Risk.Data, "features": s.Risk.Features, "metrics": s.Risk.Metrics, "high_risk_threshold": s.Risk.Threshold},
		"ai_enabled":  s.AI.Enabled(),
		"ai_model":    s.AI.ModelName(),
		"embed_model": s.AI.EmbedModelName(),
		"ai_runs":     runs,
	})
	return nil
}
