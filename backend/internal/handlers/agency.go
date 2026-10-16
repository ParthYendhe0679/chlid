package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"childconnect/internal/ai"
	"childconnect/internal/audit"
	"childconnect/internal/httpx"
	"childconnect/internal/rules"
	"childconnect/internal/storage"
	"childconnect/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// agencyCaseApp loads an application of the caller's agency or answers 404/403.
func (s *Server) agencyApp(r *http.Request, appID uuid.UUID) error {
	u := httpx.UserFrom(r.Context())
	ok, err := s.canAccessCase(r.Context(), u, appID)
	if err != nil {
		return err
	}
	if !ok {
		return s.denied(r, "application", appID.String())
	}
	return nil
}

// GET /agency/cases?status=&sort=risk
func (s *Server) agencyCases(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	status := r.URL.Query().Get("status")
	rows, err := s.Pool.Query(r.Context(), `
		SELECT a.id, p.full_name, a.status::text, d.step::text, d.due_at, d.risk_score::float8, d.risk_reasons
		FROM applications a JOIN users p ON p.id=a.parent_id
		LEFT JOIN LATERAL (SELECT step, due_at, risk_score, risk_reasons FROM case_deadlines
		                   WHERE application_id=a.id AND state IN ('OPEN','OVERDUE') ORDER BY due_at LIMIT 1) d ON true
		WHERE a.agency_id=$1 AND ($2='' OR a.status::text=$2) AND ($2<>'' OR a.status <> 'WITHDRAWN')
		ORDER BY (d.due_at IS NULL), (d.due_at < now()) DESC, d.risk_score DESC NULLS LAST, d.due_at, a.registered_at`, *u.AgencyID, status)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := time.Now()
	out := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var name, st string
		var step *string
		var due *time.Time
		var score *float64
		var reasons []byte
		if err := rows.Scan(&id, &name, &st, &step, &due, &score, &reasons); err != nil {
			return err
		}
		item := map[string]any{"application_id": id, "parent_name": name, "status": st, "current_deadline": nil}
		if step != nil {
			item["current_deadline"] = workflow.BuildDeadline(*step, *due, score, reasons, now)
		}
		out = append(out, item)
	}
	httpx.OK(w, out)
	return rows.Err()
}

type caseDoc struct {
	ID             uuid.UUID           `json:"id"`
	Category       string              `json:"category"`
	FileName       string              `json:"file_name"`
	Status         string              `json:"status"`
	AIDetectedType *string             `json:"ai_detected_type"`
	AIConfidence   *float64            `json:"ai_confidence"`
	AIFields       map[string]any      `json:"ai_fields"`
	AIChecks       []map[string]string `json:"ai_checks"`
	ReviewerNote   *string             `json:"reviewer_note"`
	UploadedAt     string              `json:"uploaded_at"`
	ReviewedAt     *string             `json:"reviewed_at"`
}

func localiseChecks(raw []byte, lang string) []map[string]string {
	var cs []ai.Check
	_ = json.Unmarshal(raw, &cs)
	out := make([]map[string]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]string{"code": c.Code, "result": c.Result, "message": c.Message(lang)})
	}
	return out
}

func (s *Server) loadCaseDoc(ctx context.Context, q workflow.Querier, id uuid.UUID, lang string) (*caseDoc, error) {
	var d caseDoc
	var fields, checks []byte
	var up time.Time
	var rev *time.Time
	err := q.QueryRow(ctx, `SELECT id, category::text, file_name, status::text, ai_detected_type, ai_confidence::float8, ai_fields, ai_checks, reviewer_note, uploaded_at, reviewed_at
		FROM documents WHERE id=$1`, id).Scan(&d.ID, &d.Category, &d.FileName, &d.Status, &d.AIDetectedType, &d.AIConfidence, &fields, &checks, &d.ReviewerNote, &up, &rev)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(fields, &d.AIFields)
	if d.AIFields == nil {
		d.AIFields = map[string]any{}
	}
	d.AIChecks = localiseChecks(checks, lang)
	d.UploadedAt = up.UTC().Format(time.RFC3339)
	d.ReviewedAt = tsStr(rev)
	return &d, nil
}

// GET /agency/cases/:id: documents with AI checks, home study report, timeline and follow-ups.
func (s *Server) agencyCase(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	lang := s.lang(r)
	ctx := r.Context()
	var (
		name, email, status, marital, state, district, prefAge, prefGender string
		phone                                                              *string
		dob                                                                time.Time
		spouse, submitted, seniority                                       *time.Time
		children, used                                                     int
		income, years                                                      *float64
		prefStates                                                         []string
		special, siblings                                                  bool
		registered                                                         time.Time
		hsrValid                                                           *time.Time
	)
	err = s.Pool.QueryRow(ctx, `SELECT p.full_name, p.email, p.phone, a.status::text, a.marital_status, a.state, a.district, a.pref_age_group, a.pref_gender,
		a.applicant_dob, a.spouse_dob, a.docs_submitted_at, a.seniority_date, a.existing_children, a.referrals_used, a.annual_income_inr::float8, a.years_married::float8,
		a.pref_states, a.open_special_needs, a.open_siblings, a.registered_at, a.hsr_valid_until
		FROM applications a JOIN users p ON p.id=a.parent_id WHERE a.id=$1`, appID).
		Scan(&name, &email, &phone, &status, &marital, &state, &district, &prefAge, &prefGender, &dob, &spouse, &submitted, &seniority, &children, &used,
			&income, &years, &prefStates, &special, &siblings, &registered, &hsrValid)
	if err != nil {
		return err
	}

	docs := []*caseDoc{}
	rows, err := s.Pool.Query(ctx, `SELECT id FROM documents WHERE application_id=$1 AND is_current ORDER BY array_position(ARRAY['PHOTO','IDENTITY','BIRTH_PROOF','RESIDENCE','INCOME','MEDICAL','MARRIAGE','DIVORCE_OR_DEATH','REFERENCE','CHILD_CONSENT']::doc_category[], category)`, appID)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		d, err := s.loadCaseDoc(ctx, s.Pool, id, lang)
		if err != nil {
			return err
		}
		docs = append(docs, d)
	}

	var hsr map[string]any
	{
		var visit, signed *time.Time
		var notes, rec *string
		var draft, final []byte
		var missing []string
		var score int
		err := s.Pool.QueryRow(ctx, `SELECT visit_date, raw_notes, ai_draft, final_sections, missing_items, completeness_score, recommendation, signed_at
			FROM home_study_reports WHERE application_id=$1`, appID).Scan(&visit, &notes, &draft, &final, &missing, &score, &rec, &signed)
		if err == nil {
			var finalMap map[string]string
			_ = json.Unmarshal(final, &finalMap)
			if finalMap == nil {
				finalMap = map[string]string{}
			}
			var draftObj map[string]any
			var runID any
			if len(draft) > 0 {
				_ = json.Unmarshal(draft, &draftObj)
				runID = draftObj["ai_run_id"]
			}
			hsr = map[string]any{"visit_date": dateStr(visit), "raw_notes": notes, "ai_draft": draftObj, "final_sections": finalMap,
				"missing_items": missing, "completeness_score": score, "recommendation": rec, "signed_at": tsStr(signed), "ai_run_id": runID}
		} else if err != pgx.ErrNoRows {
			return err
		}
	}

	timeline := []map[string]any{}
	rows, err = s.Pool.Query(ctx, `SELECT event_type, event_date, reference_no, created_at FROM legal_events WHERE application_id=$1 ORDER BY event_date, created_at`, appID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var et string
		var d, c time.Time
		var ref *string
		if err := rows.Scan(&et, &d, &ref, &c); err != nil {
			rows.Close()
			return err
		}
		timeline = append(timeline, map[string]any{"event_type": et, "event_date": d.Format("2006-01-02"), "reference_no": ref})
	}
	rows.Close()

	followups := []map[string]any{}
	rows, err = s.Pool.Query(ctx, `SELECT id, visit_number, due_date, visited_on, concern_flags, final_report FROM followups WHERE application_id=$1 ORDER BY visit_number`, appID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		var n int
		var due time.Time
		var visited *time.Time
		var flags []string
		var final []byte
		if err := rows.Scan(&id, &n, &due, &visited, &flags, &final); err != nil {
			rows.Close()
			return err
		}
		var fr map[string]string
		_ = json.Unmarshal(final, &fr)
		followups = append(followups, map[string]any{"id": id, "visit_number": n, "due_date": due.Format("2006-01-02"), "visited_on": dateStr(visited), "concern_flags": flags, "final_report": fr})
	}
	rows.Close()

	referrals := []map[string]any{}
	rows, err = s.Pool.Query(ctx, `SELECT rf.referral_number, c.case_code, rf.decision, rf.starts_at, rf.expires_at, rf.decided_at, rf.decision_reason
		FROM referrals rf JOIN children c ON c.id=rf.child_id WHERE rf.application_id=$1 ORDER BY rf.starts_at`, appID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n int
		var code, dec string
		var st, ex time.Time
		var decided *time.Time
		var reason *string
		if err := rows.Scan(&n, &code, &dec, &st, &ex, &decided, &reason); err != nil {
			rows.Close()
			return err
		}
		referrals = append(referrals, map[string]any{"referral_number": n, "case_code": code, "decision": dec, "starts_at": st.UTC().Format(time.RFC3339),
			"expires_at": ex.UTC().Format(time.RFC3339), "decided_at": tsStr(decided), "decision_reason": reason})
	}
	rows.Close()

	cd, err := workflow.Current(ctx, s.Pool, appID, time.Now())
	if err != nil {
		return err
	}
	_ = s.Audit.Log(ctx, audit.FromUser(u, "VIEW_CASE", "application", appID.String(), nil))
	var hsrOut any
	if hsr != nil {
		hsrOut = hsr
	}
	httpx.OK(w, map[string]any{
		"application_id": appID, "parent_name": name, "status": status, "documents": docs, "hsr": hsrOut, "timeline": timeline,
		"followups": followups, "current_deadline": cd, "referrals": referrals,
		"profile": map[string]any{"email": email, "phone": phone, "marital_status": marital, "state": state, "district": district,
			"applicant_dob": dob.Format("2006-01-02"), "spouse_dob": dateStr(spouse), "years_married": years, "existing_children": children,
			"annual_income_inr": income, "pref_age_group": prefAge, "pref_gender": prefGender, "pref_states": prefStates,
			"open_special_needs": special, "open_siblings": siblings, "registered_at": registered.UTC().Format(time.RFC3339),
			"docs_submitted_at": tsStr(submitted), "seniority_date": tsStr(seniority), "referrals_used": used, "hsr_valid_until": dateStr(hsrValid)},
	})
	return nil
}

// PATCH /agency/documents/:id/review
func (s *Server) reviewDocument(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	docID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Decision string `json:"decision" validate:"required,oneof=VERIFIED REUPLOAD_REQUESTED"`
		Note     string `json:"note" validate:"max=500"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	note := strings.TrimSpace(in.Note)
	if in.Decision == "REUPLOAD_REQUESTED" && len([]rune(note)) < 5 {
		return httpx.E(httpx.CodeValidation, "Please explain what the parent should upload instead (at least 5 characters).")
	}
	var appID uuid.UUID
	var current bool
	var aiStatus, category string
	err = s.Pool.QueryRow(r.Context(), `SELECT application_id, is_current, status::text, category::text FROM documents WHERE id=$1`, docID).Scan(&appID, &current, &aiStatus, &category)
	if err == pgx.ErrNoRows {
		return httpx.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	if !current {
		return httpx.E(httpx.CodeTransition, "This document was replaced by a newer upload.")
	}
	var appStatus string
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		st, err := workflow.LockStatus(r.Context(), tx, appID)
		if err != nil {
			return err
		}
		if st != "DOCS_SUBMITTED" {
			return httpx.E(httpx.CodeTransition, "Documents can be reviewed only after the parent submits them (current status: "+st+").")
		}
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		if _, err := tx.Exec(r.Context(), `UPDATE documents SET status=$2::doc_status, reviewer_id=$3, reviewer_note=$4, reviewed_at=now() WHERE id=$1`,
			docID, in.Decision, u.ID, notePtr); err != nil {
			return err
		}
		appStatus = st
		action := "VERIFY_DOC"
		if in.Decision == "REUPLOAD_REQUESTED" {
			action = "REQUEST_REUPLOAD"
			if _, err := workflow.Transition(r.Context(), tx, appID, "DOCS_CORRECTION"); err != nil {
				return err
			}
			appStatus = "DOCS_CORRECTION"
			_ = workflow.NotifyParent(r.Context(), tx, appID, "REUPLOAD", "Please upload a document again",
				rules.HumanType(category)+": "+note, "/parent/documents")
		}
		// Human decision on the AI result, to measure how useful DocCheck is.
		agree := (in.Decision == "VERIFIED") == (aiStatus == "AI_PASS")
		hd := "REJECTED"
		if agree {
			hd = "ACCEPTED"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE ai_runs SET human_decision=$2 WHERE feature='DOCCHECK' AND target_id=$1`, docID, hd); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, action, "document", docID.String(), map[string]any{"category": category, "ai_status": aiStatus}))
	})
	if err != nil {
		return err
	}
	d, err := s.loadCaseDoc(r.Context(), s.Pool, docID, s.lang(r))
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"document": d, "application_status": appStatus})
	return nil
}

// POST /agency/cases/:id/verify-all: all documents verified, the 60-day home study starts.
func (s *Server) verifyAll(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	var due time.Time
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var marital string
		var children int
		var submitted *time.Time
		var agencyID *uuid.UUID
		if err := tx.QueryRow(r.Context(), `SELECT marital_status, existing_children, docs_submitted_at, agency_id FROM applications WHERE id=$1`, appID).
			Scan(&marital, &children, &submitted, &agencyID); err != nil {
			return err
		}
		docs, err := s.requiredDocs(r.Context(), appID, marital, children)
		if err != nil {
			return err
		}
		var pending []string
		for _, d := range docs {
			if !d.Uploaded || d.Status == nil || *d.Status != "VERIFIED" {
				pending = append(pending, rules.HumanType(d.Category))
			}
		}
		if len(pending) > 0 {
			return httpx.E(httpx.CodeValidation, "These documents are not verified yet: "+strings.Join(pending, ", ")+".")
		}
		if _, err := workflow.Transition(r.Context(), tx, appID, "HSR_IN_PROGRESS"); err != nil {
			return err
		}
		start := time.Now()
		if submitted != nil {
			start = *submitted // Regulation 10(8): 60 days from document submission
		}
		due = start.Add(workflow.StepLimit["HSR_60D"])
		if err := workflow.AddDeadline(r.Context(), tx, &appID, nil, agencyID, "HSR_60D", start, nil); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "VERIFY_ALL_DOCS", "application", appID.String(), nil))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"status": "HSR_IN_PROGRESS", "hsr_due_at": due.UTC().Format(time.RFC3339)})
	return nil
}

// POST /ai/hsr/draft
func (s *Server) hsrDraft(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		ApplicationID uuid.UUID `json:"application_id" validate:"required"`
		VisitDate     string    `json:"visit_date" validate:"required"`
		RawNotes      string    `json:"raw_notes" validate:"required,min=20,max=12000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	vd, err := rules.ParseDate(in.VisitDate)
	if err != nil || vd.After(time.Now()) {
		return httpx.E(httpx.CodeValidation, "The visit date must be today or a past date.")
	}
	if err := s.agencyApp(r, in.ApplicationID); err != nil {
		return err
	}
	var status string
	var signed *time.Time
	_ = s.Pool.QueryRow(r.Context(), `SELECT a.status::text, h.signed_at FROM applications a LEFT JOIN home_study_reports h ON h.application_id=a.id WHERE a.id=$1`, in.ApplicationID).Scan(&status, &signed)
	if status != "HSR_IN_PROGRESS" || signed != nil {
		return httpx.E(httpx.CodeTransition, "A home study report can be drafted only while the home study is in progress and not yet signed.")
	}
	d := s.AI.DraftHSR(r.Context(), in.RawNotes, in.VisitDate)
	runID, err := s.AI.RecordRun(r.Context(), "HSR_DRAFT", &u.ID, "application", &in.ApplicationID, d.Model, d.LatencyMs, d, nil)
	if err != nil {
		return err
	}
	draft, _ := json.Marshal(map[string]any{"sections": d.Sections, "missing_items": d.MissingItems, "completeness_score": d.CompletenessScore, "ai_run_id": runID, "model": d.Model})
	_, err = s.Pool.Exec(r.Context(), `INSERT INTO home_study_reports (application_id, social_worker_id, visit_date, raw_notes, ai_draft)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (application_id) DO UPDATE SET visit_date=$3, raw_notes=$4, ai_draft=$5, social_worker_id=$2`,
		in.ApplicationID, u.ID, in.VisitDate, in.RawNotes, draft)
	if err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "HSR_DRAFTED", "application", in.ApplicationID.String(), map[string]any{"ai_run_id": runID.String(), "model": d.Model}))
	httpx.OK(w, map[string]any{"sections": d.Sections, "missing_items": d.MissingItems, "completeness_score": d.CompletenessScore, "ai_run_id": runID, "model": d.Model})
	return nil
}

func cleanSections(in map[string]string, keys []string) map[string]string {
	out := map[string]string{}
	for _, k := range keys {
		v := strings.TrimSpace(in[k])
		if len(v) > 8000 {
			v = v[:8000]
		}
		out[k] = v
	}
	return out
}

// PUT /agency/hsr/:application_id: saves the human-edited report.
func (s *Server) saveHSR(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		VisitDate      string            `json:"visit_date" validate:"required"`
		FinalSections  map[string]string `json:"final_sections" validate:"required"`
		Recommendation *string           `json:"recommendation" validate:"omitempty,oneof=SUITABLE NOT_SUITABLE NEEDS_MORE_INFO"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if _, err := rules.ParseDate(in.VisitDate); err != nil {
		return httpx.E(httpx.CodeValidation, "The visit date is not valid.")
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	var status string
	var signed *time.Time
	_ = s.Pool.QueryRow(r.Context(), `SELECT a.status::text, h.signed_at FROM applications a LEFT JOIN home_study_reports h ON h.application_id=a.id WHERE a.id=$1`, appID).Scan(&status, &signed)
	if signed != nil {
		return httpx.E(httpx.CodeTransition, "This report is already signed and cannot be changed.")
	}
	if status != "HSR_IN_PROGRESS" {
		return httpx.E(httpx.CodeTransition, "The home study is not in progress for this case.")
	}
	final := cleanSections(in.FinalSections, ai.HSRSections)
	score, missing := ai.Completeness(final)
	b, _ := json.Marshal(final)
	_, err = s.Pool.Exec(r.Context(), `INSERT INTO home_study_reports (application_id, social_worker_id, visit_date, final_sections, missing_items, completeness_score, recommendation)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (application_id) DO UPDATE SET visit_date=$3, final_sections=$4, missing_items=$5, completeness_score=$6, recommendation=$7, social_worker_id=$2`,
		appID, u.ID, in.VisitDate, b, missing, score, in.Recommendation)
	if err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "HSR_SAVED", "application", appID.String(), map[string]any{"completeness": score}))
	httpx.OK(w, map[string]any{"ok": true, "completeness_score": score, "missing_items": missing})
	return nil
}

// POST /agency/hsr/:application_id/sign
func (s *Server) signHSR(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		AIRunID uuid.UUID `json:"ai_run_id" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	var status string
	var validUntil *time.Time
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var final, draft []byte
		var rec *string
		var visit, signed *time.Time
		err := tx.QueryRow(r.Context(), `SELECT final_sections, ai_draft, recommendation, visit_date, signed_at FROM home_study_reports WHERE application_id=$1 FOR UPDATE`, appID).
			Scan(&final, &draft, &rec, &visit, &signed)
		if err == pgx.ErrNoRows {
			return httpx.E(httpx.CodeValidation, "Save the report before signing.")
		}
		if err != nil {
			return err
		}
		if signed != nil {
			return httpx.E(httpx.CodeTransition, "This report is already signed.")
		}
		var fm map[string]string
		_ = json.Unmarshal(final, &fm)
		if score, _ := ai.Completeness(fm); score == 0 || visit == nil {
			return httpx.E(httpx.CodeValidation, "Save the report with a visit date and its sections before signing.")
		}
		if rec == nil || *rec == "NEEDS_MORE_INFO" {
			return httpx.E(httpx.CodeValidation, "Choose 'Suitable' or 'Not suitable' to sign. A report that needs more information can be saved but not signed.")
		}
		var ok bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM ai_runs WHERE id=$1 AND feature='HSR_DRAFT' AND target_id=$2)`, in.AIRunID, appID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpx.E(httpx.CodeValidation, "The AI draft reference does not belong to this case. Draft the report again.")
		}
		// Both the AI draft and the signed version are kept; record whether the human edited it.
		var dm struct {
			Sections map[string]string `json:"sections"`
		}
		_ = json.Unmarshal(draft, &dm)
		hd := "ACCEPTED"
		for _, k := range ai.HSRSections {
			if strings.TrimSpace(dm.Sections[k]) != strings.TrimSpace(fm[k]) {
				hd = "EDITED"
			}
		}
		if _, err := tx.Exec(r.Context(), `UPDATE ai_runs SET human_decision=$2 WHERE id=$1`, in.AIRunID, hd); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE home_study_reports SET signed_at=now(), social_worker_id=$2 WHERE application_id=$1`, appID, u.ID); err != nil {
			return err
		}
		if err := workflow.CompleteSteps(r.Context(), tx, appID, "HSR_60D"); err != nil {
			return err
		}
		if *rec == "SUITABLE" {
			if _, err := workflow.Transition(r.Context(), tx, appID, "WAITING"); err != nil {
				return err
			}
			var vu time.Time
			if err := tx.QueryRow(r.Context(), `UPDATE applications SET hsr_valid_until=(now() + interval '3 years')::date WHERE id=$1 RETURNING hsr_valid_until`, appID).Scan(&vu); err != nil {
				return err
			}
			validUntil, status = &vu, "WAITING"
		} else {
			if _, err := workflow.Transition(r.Context(), tx, appID, "WITHDRAWN"); err != nil {
				return err
			}
			status = "WITHDRAWN"
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "SIGN_HSR", "application", appID.String(), map[string]any{"recommendation": *rec, "ai_run_id": in.AIRunID.String(), "human_decision": hd}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"status": status, "hsr_valid_until": dateStr(validUntil)})
	return nil
}

// ---------- children ----------

var stateCodes = map[string]string{
	"andhra pradesh": "AP", "arunachal pradesh": "AR", "assam": "AS", "bihar": "BR", "chhattisgarh": "CG", "goa": "GA", "gujarat": "GJ",
	"haryana": "HR", "himachal pradesh": "HP", "jharkhand": "JH", "karnataka": "KA", "kerala": "KL", "madhya pradesh": "MP",
	"maharashtra": "MH", "manipur": "MN", "meghalaya": "ML", "mizoram": "MZ", "nagaland": "NL", "odisha": "OD", "punjab": "PB",
	"rajasthan": "RJ", "sikkim": "SK", "tamil nadu": "TN", "telangana": "TS", "tripura": "TR", "uttar pradesh": "UP",
	"uttarakhand": "UK", "west bengal": "WB", "delhi": "DL", "jammu and kashmir": "JK", "ladakh": "LA", "puducherry": "PY", "chandigarh": "CH",
}

// districtCode turns "Thane" into "THN": the first letter, then consonants, then vowels.
func districtCode(d string) string {
	var letters []rune
	for _, r := range strings.ToUpper(d) {
		if r >= 'A' && r <= 'Z' {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return "XXX"
	}
	out := []rune{letters[0]}
	isV := func(r rune) bool { return strings.ContainsRune("AEIOU", r) }
	for _, r := range letters[1:] {
		if len(out) < 3 && !isV(r) {
			out = append(out, r)
		}
	}
	for _, r := range letters[1:] {
		if len(out) < 3 && isV(r) {
			out = append(out, r)
		}
	}
	for len(out) < 3 {
		out = append(out, 'X')
	}
	return string(out[:3])
}

// CaseCodePrefix builds "MH-THN-2026-" for an agency's state and district.
func CaseCodePrefix(state, district string, year int) string {
	sc, ok := stateCodes[strings.ToLower(strings.TrimSpace(state))]
	if !ok {
		sc = districtCode(state)[:2]
	}
	return fmt.Sprintf("%s-%s-%d-", sc, districtCode(district), year)
}

// NextCaseCode returns the next free case code for the prefix (inside a transaction).
func NextCaseCode(ctx context.Context, tx pgx.Tx, prefix string) (string, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, prefix); err != nil {
		return "", err
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(substring(case_code from '(\d+)$')::int), 0) + 1 FROM children WHERE case_code LIKE $1 || '%'`, prefix).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, n), nil
}

// POST /agency/children: the name is encrypted; only the case code is used elsewhere.
func (s *Server) addChild(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		Name           string `json:"name" validate:"required,min=2,max=120"`
		DOB            string `json:"dob" validate:"required"`
		Gender         string `json:"gender" validate:"required,oneof=MALE FEMALE"`
		IsSpecialNeeds bool   `json:"is_special_needs"`
		AdmittedOn     string `json:"admitted_on" validate:"required"`
		CWCLFANumber   string `json:"cwc_lfa_number" validate:"max=80"`
		LFADate        string `json:"lfa_date"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	now := time.Now().In(rules.IST)
	dob, err1 := rules.ParseDate(in.DOB)
	adm, err2 := rules.ParseDate(in.AdmittedOn)
	if err1 != nil || err2 != nil || dob.After(now) || adm.After(now) || adm.Before(dob) {
		return httpx.E(httpx.CodeValidation, "Check the dates: date of birth and admission must be in the past, and admission cannot be before birth.")
	}
	if rules.AgeYears(dob, now) >= 18 {
		return httpx.E(httpx.CodeValidation, "A child must be under 18 years of age.")
	}
	var lfa *time.Time
	if strings.TrimSpace(in.LFADate) != "" {
		d, err := rules.ParseDate(in.LFADate)
		if err != nil || d.After(now) || d.Before(dob) {
			return httpx.E(httpx.CodeValidation, "The legally-free date must be a past date after the date of birth.")
		}
		lfa = &d
		if strings.TrimSpace(in.CWCLFANumber) == "" {
			return httpx.E(httpx.CodeValidation, "The Child Welfare Committee certificate number is required with the legally-free date.")
		}
	}
	nameEnc, err := s.Box.Encrypt(strings.TrimSpace(in.Name))
	if err != nil {
		return err
	}
	var childID uuid.UUID
	var code string
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var state, district string
		if err := tx.QueryRow(r.Context(), `SELECT state, district FROM agencies WHERE id=$1`, *u.AgencyID).Scan(&state, &district); err != nil {
			return err
		}
		code, err = NextCaseCode(r.Context(), tx, CaseCodePrefix(state, district, now.Year()))
		if err != nil {
			return err
		}
		status := "IN_CARE"
		if lfa != nil {
			status = "LEGALLY_FREE"
		}
		var cwc *string
		if c := strings.TrimSpace(in.CWCLFANumber); c != "" {
			cwc = &c
		}
		if err := tx.QueryRow(r.Context(), `INSERT INTO children (agency_id, case_code, name_enc, dob, gender, is_special_needs, admitted_on, cwc_lfa_number, lfa_date, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, *u.AgencyID, code, nameEnc, dob, in.Gender, in.IsSpecialNeeds, adm, cwc, lfa, status).Scan(&childID); err != nil {
			return err
		}
		if lfa != nil {
			// CSR and MER are due within 10 days of the child being declared legally free.
			if err := workflow.AddDeadline(r.Context(), tx, nil, &childID, u.AgencyID, "CSR_MER_10D", *lfa, nil); err != nil {
				return err
			}
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "ADD_CHILD", "child", childID.String(), map[string]any{"case_code": code}))
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"child_id": childID, "case_code": code})
	return nil
}

type childRow struct {
	ID             uuid.UUID `json:"id"`
	CaseCode       string    `json:"case_code"`
	Status         string    `json:"status"`
	Gender         string    `json:"gender"`
	IsSpecialNeeds bool      `json:"is_special_needs"`
	AgeText        string    `json:"age_text"`
}

// GET /agency/children (PROPOSED P1)
func (s *Server) listChildren(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	rows, err := s.Pool.Query(r.Context(), `SELECT id, case_code, status, gender, is_special_needs, dob FROM children WHERE agency_id=$1
		ORDER BY array_position(ARRAY['READY','LEGALLY_FREE','REFERRED','RESERVED','IN_CARE','PLACED'], status), created_at DESC`, *u.AgencyID)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := time.Now().In(rules.IST)
	lang := s.lang(r)
	out := []childRow{}
	for rows.Next() {
		var c childRow
		var dob time.Time
		if err := rows.Scan(&c.ID, &c.CaseCode, &c.Status, &c.Gender, &c.IsSpecialNeeds, &dob); err != nil {
			return err
		}
		c.AgeText = rules.AgeText(dob, now, lang)
		out = append(out, c)
	}
	httpx.OK(w, out)
	return rows.Err()
}

func (s *Server) ownChild(r *http.Request, id uuid.UUID) error {
	u := httpx.UserFrom(r.Context())
	var agency uuid.UUID
	err := s.Pool.QueryRow(r.Context(), `SELECT agency_id FROM children WHERE id=$1`, id).Scan(&agency)
	if err == pgx.ErrNoRows {
		return httpx.ErrNotFound
	}
	if err != nil {
		return err
	}
	if u.AgencyID == nil || agency != *u.AgencyID {
		return s.denied(r, "child", id.String())
	}
	return nil
}

// GET /agency/children/:id (PROPOSED P2). The child's name is never returned.
func (s *Server) getChild(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := s.ownChild(r, id); err != nil {
		return err
	}
	var c childRow
	var dob, adm time.Time
	var lfa, approvedAt *time.Time
	var cwc, csr, mer, merText, draft, approved *string
	err = s.Pool.QueryRow(r.Context(), `SELECT id, case_code, status, gender, is_special_needs, dob, admitted_on, lfa_date, cwc_lfa_number,
		csr_storage_key, mer_storage_key, mer_text, summary_ai_draft, summary_approved, summary_approved_at FROM children WHERE id=$1`, id).
		Scan(&c.ID, &c.CaseCode, &c.Status, &c.Gender, &c.IsSpecialNeeds, &dob, &adm, &lfa, &cwc, &csr, &mer, &merText, &draft, &approved, &approvedAt)
	if err != nil {
		return err
	}
	c.AgeText = rules.AgeText(dob, time.Now().In(rules.IST), s.lang(r))
	link := func(k *string) *string {
		if k == nil {
			return nil
		}
		l := s.Store.SignedURL(*k)
		return &l
	}
	var referral map[string]any
	{
		var parent string
		var exp time.Time
		var dec string
		if s.Pool.QueryRow(r.Context(), `SELECT p.full_name, rf.expires_at, rf.decision FROM referrals rf JOIN applications a ON a.id=rf.application_id
			JOIN users p ON p.id=a.parent_id WHERE rf.child_id=$1 ORDER BY rf.starts_at DESC LIMIT 1`, id).Scan(&parent, &exp, &dec) == nil {
			referral = map[string]any{"parent_name": parent, "expires_at": exp.UTC().Format(time.RFC3339), "decision": dec}
		}
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "VIEW_CHILD", "child", id.String(), nil))
	httpx.OK(w, map[string]any{
		"id": c.ID, "case_code": c.CaseCode, "status": c.Status, "gender": c.Gender, "is_special_needs": c.IsSpecialNeeds, "age_text": c.AgeText,
		"dob": dob.Format("2006-01-02"), "admitted_on": adm.Format("2006-01-02"), "lfa_date": dateStr(lfa), "cwc_lfa_number": cwc,
		"has_csr": csr != nil, "has_mer": mer != nil, "mer_text_extracted": merText != nil && strings.TrimSpace(*merText) != "",
		"csr_url": link(csr), "mer_url": link(mer), "summary_ai_draft": draft, "summary_approved": approved, "summary_approved_at": tsStr(approvedAt),
		"latest_referral": referral,
	})
	return nil
}

// POST /agency/children/:id/reports: multipart fields csr and mer.
func (s *Server) uploadChildReports(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := s.ownChild(r, id); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxUpload+(512<<10))
	if err := r.ParseMultipartForm(2*maxUpload + (512 << 10)); err != nil {
		return httpx.E(httpx.CodeValidation, "The files are too large. Each file may be up to 5 MB.")
	}
	read := func(field string) ([]byte, string, error) {
		f, _, err := r.FormFile(field)
		if err != nil {
			return nil, "", nil
		}
		defer f.Close()
		b := make([]byte, maxUpload+1)
		n, _ := readFull(f, b)
		if n == 0 {
			return nil, "", httpx.E(httpx.CodeValidation, "The "+strings.ToUpper(field)+" file is empty.")
		}
		if n > maxUpload {
			return nil, "", httpx.E(httpx.CodeValidation, "The "+strings.ToUpper(field)+" file is larger than 5 MB.")
		}
		mime, ok := sniff(b[:n])
		if !ok {
			return nil, "", httpx.E(httpx.CodeValidation, "The "+strings.ToUpper(field)+" must be a PDF, JPG or PNG file.")
		}
		return b[:n], mime, nil
	}
	csr, csrMime, err := read("csr")
	if err != nil {
		return err
	}
	mer, merMime, err := read("mer")
	if err != nil {
		return err
	}
	if csr == nil && mer == nil {
		return httpx.E(httpx.CodeValidation, "Upload the CSR, the MER, or both.")
	}
	var status string
	if err := s.Pool.QueryRow(r.Context(), `SELECT status FROM children WHERE id=$1`, id).Scan(&status); err != nil {
		return err
	}
	if status == "REFERRED" || status == "RESERVED" || status == "PLACED" {
		return httpx.E(httpx.CodeTransition, "Reports cannot be replaced while a referral is active or after placement.")
	}
	var csrKey, merKey, merText *string
	if csr != nil {
		k, err := s.Store.Save("children", csr, csrMime)
		if err != nil {
			return err
		}
		csrKey = &k
	}
	if mer != nil {
		k, err := s.Store.Save("children", mer, merMime)
		if err != nil {
			return err
		}
		merKey = &k
		t := s.extractMERText(r.Context(), mer, merMime)
		merText = &t
	}
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		// A new MER needs a new approved summary, so the child leaves READY until then (Referral Lock).
		if _, err := tx.Exec(r.Context(), `UPDATE children SET csr_storage_key=COALESCE($2, csr_storage_key), mer_storage_key=COALESCE($3, mer_storage_key),
			mer_text=COALESCE($4, mer_text),
			summary_approved=CASE WHEN $3::text IS NOT NULL THEN NULL ELSE summary_approved END,
			summary_ai_draft=CASE WHEN $3::text IS NOT NULL THEN NULL ELSE summary_ai_draft END,
			status=CASE WHEN $3::text IS NOT NULL AND status='READY' THEN 'LEGALLY_FREE' ELSE status END
			WHERE id=$1`, id, csrKey, merKey, merText); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE case_deadlines SET state='DONE', completed_at=now() WHERE child_id=$1 AND step='CSR_MER_10D' AND state IN ('OPEN','OVERDUE')
			AND EXISTS (SELECT 1 FROM children WHERE id=$1 AND csr_storage_key IS NOT NULL AND mer_storage_key IS NOT NULL)`, id); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "UPLOAD_CHILD_REPORTS", "child", id.String(), map[string]any{"csr": csr != nil, "mer": mer != nil}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"ok": true, "mer_text_extracted": merText != nil && strings.TrimSpace(*merText) != ""})
	return nil
}

func readFull(f io.Reader, b []byte) (int, error) { return io.ReadFull(f, b) }

func sniff(b []byte) (string, bool) { return storage.SniffType(b) }

func (s *Server) extractMERText(ctx context.Context, data []byte, mime string) string {
	if mime == "application/pdf" {
		if t, err := ai.PDFText(data); err == nil && len(strings.TrimSpace(t)) > 40 {
			return strings.TrimSpace(t)
		}
	}
	if t, err := s.AI.ReadReportText(ctx, data, mime); err == nil {
		return t
	}
	return ""
}

// POST /ai/medical-summary
func (s *Server) medicalSummary(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		ChildID  uuid.UUID `json:"child_id" validate:"required"`
		Language string    `json:"language"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	lang := in.Language
	if !rules.ValidLang(lang) {
		lang = s.lang(r)
	}
	if err := s.ownChild(r, in.ChildID); err != nil {
		return err
	}
	var merKey, merText *string
	if err := s.Pool.QueryRow(r.Context(), `SELECT mer_storage_key, mer_text FROM children WHERE id=$1`, in.ChildID).Scan(&merKey, &merText); err != nil {
		return err
	}
	if merKey == nil {
		return httpx.E(httpx.CodeValidation, "Upload the Medical Examination Report first.")
	}
	text := ""
	if merText != nil {
		text = *merText
	}
	if strings.TrimSpace(text) == "" {
		data, err := s.Store.Read(*merKey)
		if err != nil {
			return err
		}
		mime, _ := sniff(data)
		text = s.extractMERText(r.Context(), data, mime)
		if text == "" {
			return httpx.E(httpx.CodeAIUnavailble, "The text of the medical report could not be read. Please write the summary by hand.")
		}
		_, _ = s.Pool.Exec(r.Context(), `UPDATE children SET mer_text=$2 WHERE id=$1`, in.ChildID, text)
	}
	d, err := s.AI.DraftMedicalSummary(r.Context(), text, lang)
	if err != nil {
		return err
	}
	runID, err := s.AI.RecordRun(r.Context(), "MED_SUMMARY", &u.ID, "child", &in.ChildID, d.Model, d.LatencyMs, map[string]any{"draft": d.Draft, "language": lang}, nil)
	if err != nil {
		return err
	}
	if _, err := s.Pool.Exec(r.Context(), `UPDATE children SET summary_ai_draft=$2 WHERE id=$1`, in.ChildID, d.Draft); err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "MED_SUMMARY_DRAFTED", "child", in.ChildID.String(), map[string]any{"ai_run_id": runID.String(), "model": d.Model}))
	httpx.OK(w, map[string]any{"draft": d.Draft, "ai_run_id": runID, "model": d.Model})
	return nil
}

// POST /agency/children/:id/summary/approve: Referral Lock opens only with CSR + MER + approved summary.
func (s *Server) approveSummary(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		ApprovedText string    `json:"approved_text" validate:"required,min=20,max=12000"`
		AIRunID      uuid.UUID `json:"ai_run_id" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := s.ownChild(r, id); err != nil {
		return err
	}
	var childStatus string
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var csr, mer, draft *string
		var lfa *time.Time
		var status string
		if err := tx.QueryRow(r.Context(), `SELECT csr_storage_key, mer_storage_key, summary_ai_draft, lfa_date, status FROM children WHERE id=$1 FOR UPDATE`, id).
			Scan(&csr, &mer, &draft, &lfa, &status); err != nil {
			return err
		}
		if csr == nil || mer == nil {
			return httpx.E(httpx.CodeValidation, "Upload both the CSR and the MER before approving the summary.")
		}
		if status == "REFERRED" || status == "RESERVED" || status == "PLACED" {
			return httpx.E(httpx.CodeTransition, "The summary cannot be changed while a referral is active or after placement.")
		}
		var ok bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM ai_runs WHERE id=$1 AND feature='MED_SUMMARY' AND target_id=$2)`, in.AIRunID, id).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpx.E(httpx.CodeValidation, "The AI draft reference does not belong to this child. Draft the summary again.")
		}
		text := strings.TrimSpace(in.ApprovedText)
		hd := "EDITED"
		if draft != nil && strings.TrimSpace(*draft) == text {
			hd = "ACCEPTED"
		}
		childStatus = status
		if lfa != nil && (status == "LEGALLY_FREE" || status == "IN_CARE" || status == "READY") {
			childStatus = "READY"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE children SET summary_approved=$2, summary_approved_by=$3, summary_approved_at=now(), status=$4 WHERE id=$1`,
			id, text, u.ID, childStatus); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE ai_runs SET human_decision=$2 WHERE id=$1`, in.AIRunID, hd); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "APPROVE_MED_SUMMARY", "child", id.String(), map[string]any{"human_decision": hd, "child_status": childStatus}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"child_status": childStatus})
	return nil
}

type candidate struct {
	AppID     uuid.UUID
	Parent    string
	Seniority time.Time
	MatchedOn []string
}

// childAgeGroup returns the age group a child falls in today.
func childAgeGroup(dob, now time.Time) string {
	months := (now.Year()-dob.Year())*12 + int(now.Month()) - int(dob.Month())
	if now.Day() < dob.Day() {
		months--
	}
	switch {
	case months <= 24:
		return "0-2"
	case months <= 48:
		return "2-4"
	case months <= 96:
		return "4-8"
	}
	return "8-18"
}

// eligibleParents applies the seniority queue (section 9): WAITING parents with a valid home study,
// matching preferences, oldest seniority first. Rules only; no AI.
func (s *Server) eligibleParents(ctx context.Context, q workflow.Querier, childID uuid.UUID, onlyApp *uuid.UUID) ([]candidate, error) {
	var dob time.Time
	var gender, state string
	var special bool
	if err := q.QueryRow(ctx, `SELECT c.dob, c.gender, c.is_special_needs, a.state FROM children c JOIN agencies a ON a.id=c.agency_id WHERE c.id=$1`, childID).
		Scan(&dob, &gender, &special, &state); err != nil {
		return nil, err
	}
	group := childAgeGroup(dob, time.Now().In(rules.IST))
	rows, err := q.Query(ctx, `SELECT a.id, p.full_name, a.seniority_date, a.marital_status FROM applications a JOIN users p ON p.id=a.parent_id
		WHERE a.status='WAITING' AND a.seniority_date IS NOT NULL AND a.referrals_used < 3
		  AND (a.hsr_valid_until IS NULL OR a.hsr_valid_until >= current_date)
		  AND (a.debarred_until IS NULL OR a.debarred_until < current_date)
		  AND a.pref_age_group=$1 AND (a.pref_gender='ANY' OR a.pref_gender=$2)
		  AND (cardinality(a.pref_states)=0 OR $3 = ANY(a.pref_states))
		  AND (NOT $4 OR a.open_special_needs)
		  AND NOT EXISTS (SELECT 1 FROM referrals rf WHERE rf.application_id=a.id AND rf.child_id=$5)
		  AND ($6::uuid IS NULL OR a.id=$6)
		ORDER BY a.seniority_date, a.registered_at LIMIT 20`, group, gender, state, special, childID, onlyApp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candidate
	for rows.Next() {
		var c candidate
		var marital string
		if err := rows.Scan(&c.AppID, &c.Parent, &c.Seniority, &marital); err != nil {
			return nil, err
		}
		if marital == "SINGLE_MALE" && gender == "FEMALE" {
			continue // a single man cannot adopt a girl child
		}
		c.MatchedOn = []string{"age_group", "gender", "state"}
		if special {
			c.MatchedOn = append(c.MatchedOn, "special_needs")
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seniority.Before(out[j].Seniority) })
	return out, rows.Err()
}

// GET /agency/children/:id/next-parent
func (s *Server) nextParent(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := s.ownChild(r, id); err != nil {
		return err
	}
	var status string
	if err := s.Pool.QueryRow(r.Context(), `SELECT status FROM children WHERE id=$1`, id).Scan(&status); err != nil {
		return err
	}
	if status != "READY" {
		return httpx.E(httpx.CodeTransition, "Referral Lock: the child is not ready. The CSR, the MER and an approved summary are all needed.")
	}
	cs, err := s.eligibleParents(r.Context(), s.Pool, id, nil)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		return httpx.E(httpx.CodeNotFound, "No parent in the waiting list matches this child's profile right now.")
	}
	c := cs[0]
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "VIEW_NEXT_PARENT", "child", id.String(), map[string]any{"application_id": c.AppID.String()}))
	httpx.OK(w, map[string]any{"application_id": c.AppID, "parent_name": c.Parent, "seniority_date": c.Seniority.UTC().Format(time.RFC3339), "matched_on": c.MatchedOn})
	return nil
}

// POST /agency/referrals: 409 if the child is not READY (Referral Lock). The 48-hour timer starts now.
func (s *Server) sendReferral(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		ChildID       uuid.UUID `json:"child_id" validate:"required"`
		ApplicationID uuid.UUID `json:"application_id" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := s.ownChild(r, in.ChildID); err != nil {
		return err
	}
	var refID uuid.UUID
	var expires time.Time
	err := s.tx(r.Context(), func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(r.Context(), `SELECT status FROM children WHERE id=$1 FOR UPDATE`, in.ChildID).Scan(&status); err != nil {
			return err
		}
		if status != "READY" {
			return httpx.E(httpx.CodeTransition, "Referral Lock: the child is not READY. The CSR, the MER and an approved summary are all needed.")
		}
		cs, err := s.eligibleParents(r.Context(), tx, in.ChildID, &in.ApplicationID)
		if err != nil {
			return err
		}
		if len(cs) == 0 {
			return httpx.E(httpx.CodeTransition, "This parent is not eligible for this referral (status, preferences or referral limit).")
		}
		var used int
		if err := tx.QueryRow(r.Context(), `SELECT referrals_used FROM applications WHERE id=$1`, in.ApplicationID).Scan(&used); err != nil {
			return err
		}
		if _, err := workflow.Transition(r.Context(), tx, in.ApplicationID, "REFERRED"); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `INSERT INTO referrals (application_id, child_id, referral_number, sent_by) VALUES ($1,$2,$3,$4) RETURNING id, expires_at`,
			in.ApplicationID, in.ChildID, used+1, u.ID).Scan(&refID, &expires); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE children SET status='REFERRED' WHERE id=$1`, in.ChildID); err != nil {
			return err
		}
		start := time.Now()
		if err := workflow.AddDeadline(r.Context(), tx, &in.ApplicationID, &in.ChildID, u.AgencyID, "RESERVE_48H", start, &expires); err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "SEND_REFERRAL", "referral", refID.String(),
			map[string]any{"child_id": in.ChildID.String(), "application_id": in.ApplicationID.String(), "referral_number": used + 1}))
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"referral_id": refID, "expires_at": expires.UTC().Format(time.RFC3339)})
	return nil
}

// eventRule: which status an event needs and what it moves the case to ("" = no status change).
var eventRule = map[string]struct {
	from []string
	to   string
}{
	"MATCHED":     {[]string{"RESERVED"}, "MATCHED_FOSTER_CARE"},
	"FOSTER_CARE": {[]string{"MATCHED_FOSTER_CARE", "DM_ORDER_PENDING"}, ""},
	"DM_FILED":    {[]string{"MATCHED_FOSTER_CARE"}, "DM_ORDER_PENDING"},
	"DM_ORDER":    {[]string{"DM_ORDER_PENDING"}, "ADOPTED_FOLLOWUP"},
	"BIRTH_CERT":  {[]string{"ADOPTED_FOLLOWUP", "COMPLETED"}, ""},
}

// POST /agency/cases/:id/events: each recorded date creates the next legal deadline.
func (s *Server) recordEvent(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		EventType   string `json:"event_type" validate:"required,oneof=MATCHED FOSTER_CARE DM_FILED DM_ORDER BIRTH_CERT"`
		EventDate   string `json:"event_date" validate:"required"`
		ReferenceNo string `json:"reference_no" validate:"max=80"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	ed, err := rules.ParseDate(in.EventDate)
	if err != nil || ed.After(time.Now()) {
		return httpx.E(httpx.CodeValidation, "The event date must be today or a past date.")
	}
	if err := s.agencyApp(r, appID); err != nil {
		return err
	}
	var status string
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		cur, err := workflow.LockStatus(r.Context(), tx, appID)
		if err != nil {
			return err
		}
		rule := eventRule[in.EventType]
		okFrom := false
		for _, f := range rule.from {
			if f == cur {
				okFrom = true
			}
		}
		if !okFrom {
			return httpx.E(httpx.CodeTransition, fmt.Sprintf("%s cannot be recorded while the case is %s.", in.EventType, cur))
		}
		var dup bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM legal_events WHERE application_id=$1 AND event_type=$2)`, appID, in.EventType).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return httpx.E(httpx.CodeTransition, in.EventType+" is already recorded for this case.")
		}
		var ref *string
		if v := strings.TrimSpace(in.ReferenceNo); v != "" {
			ref = &v
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO legal_events (application_id, event_type, event_date, reference_no, recorded_by) VALUES ($1,$2,$3,$4,$5)`,
			appID, in.EventType, ed, ref, u.ID); err != nil {
			return err
		}
		status = cur
		if rule.to != "" {
			if _, err := workflow.Transition(r.Context(), tx, appID, rule.to); err != nil {
				return err
			}
			status = rule.to
		}
		var childID *uuid.UUID
		_ = tx.QueryRow(r.Context(), `SELECT child_id FROM referrals WHERE application_id=$1 AND decision IN ('RESERVED','ACCEPTED') ORDER BY starts_at DESC LIMIT 1`, appID).Scan(&childID)
		agency := u.AgencyID
		switch in.EventType {
		case "MATCHED":
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "MATCHING_30D"); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `UPDATE referrals SET decision='ACCEPTED' WHERE application_id=$1 AND decision='RESERVED'`, appID); err != nil {
				return err
			}
			if err := workflow.AddDeadline(r.Context(), tx, &appID, childID, agency, "FOSTER_CARE_10D", ed, nil); err != nil {
				return err
			}
			if err := workflow.AddDeadline(r.Context(), tx, &appID, childID, agency, "DM_FILING_10D", ed, nil); err != nil {
				return err
			}
		case "FOSTER_CARE":
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "FOSTER_CARE_10D"); err != nil {
				return err
			}
		case "DM_FILED":
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "DM_FILING_10D"); err != nil {
				return err
			}
			if err := workflow.AddDeadline(r.Context(), tx, &appID, childID, agency, "DM_ORDER_60D", ed, nil); err != nil {
				return err
			}
		case "DM_ORDER":
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "DM_ORDER_60D", "FOSTER_CARE_10D"); err != nil {
				return err
			}
			// Four follow-up visits, every 6 months for 2 years.
			for i := 1; i <= 4; i++ {
				due := ed.AddDate(0, 6*i, 0)
				if _, err := tx.Exec(r.Context(), `INSERT INTO followups (application_id, visit_number, due_date) VALUES ($1,$2,$3)`, appID, i, due); err != nil {
					return err
				}
				start := ed.AddDate(0, 6*(i-1), 0)
				if err := workflow.AddDeadline(r.Context(), tx, &appID, childID, agency, "FOLLOWUP_VISIT", start, &due); err != nil {
					return err
				}
			}
			if childID != nil {
				if _, err := tx.Exec(r.Context(), `UPDATE children SET status='PLACED' WHERE id=$1`, *childID); err != nil {
					return err
				}
			}
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "LEGAL_EVENT", "application", appID.String(),
			map[string]any{"event_type": in.EventType, "event_date": in.EventDate}))
	})
	if err != nil {
		return err
	}
	cd, _ := workflow.Current(r.Context(), s.Pool, appID, time.Now())
	httpx.OK(w, map[string]any{"status": status, "next_deadline": cd})
	return nil
}

func (s *Server) ownFollowup(r *http.Request, id uuid.UUID) (uuid.UUID, int, *time.Time, error) {
	var appID uuid.UUID
	var n int
	var visited *time.Time
	err := s.Pool.QueryRow(r.Context(), `SELECT application_id, visit_number, visited_on FROM followups WHERE id=$1`, id).Scan(&appID, &n, &visited)
	if err == pgx.ErrNoRows {
		return uuid.Nil, 0, nil, httpx.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, 0, nil, err
	}
	if err := s.agencyApp(r, appID); err != nil {
		return uuid.Nil, 0, nil, err
	}
	return appID, n, visited, nil
}

// POST /ai/followup/draft
func (s *Server) followupDraft(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		FollowupID uuid.UUID `json:"followup_id" validate:"required"`
		RawNotes   string    `json:"raw_notes" validate:"required,min=20,max=12000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	_, _, visited, err := s.ownFollowup(r, in.FollowupID)
	if err != nil {
		return err
	}
	if visited != nil {
		return httpx.E(httpx.CodeTransition, "This follow-up report is already signed.")
	}
	d := s.AI.DraftFollowup(r.Context(), in.RawNotes)
	runID, err := s.AI.RecordRun(r.Context(), "FOLLOWUP_DRAFT", &u.ID, "followup", &in.FollowupID, d.Model, d.LatencyMs, d, nil)
	if err != nil {
		return err
	}
	draft, _ := json.Marshal(map[string]any{"sections": d.Sections, "concern_flags": d.ConcernFlags, "ai_run_id": runID, "model": d.Model})
	if _, err := s.Pool.Exec(r.Context(), `UPDATE followups SET raw_notes=$2, ai_draft=$3, concern_flags=$4 WHERE id=$1`, in.FollowupID, in.RawNotes, draft, d.ConcernFlags); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"sections": d.Sections, "concern_flags": d.ConcernFlags, "ai_run_id": runID, "model": d.Model})
	return nil
}

// POST /agency/followups/:id/sign
func (s *Server) signFollowup(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		FinalReport map[string]string `json:"final_report" validate:"required"`
		VisitedOn   string            `json:"visited_on" validate:"required"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	vd, err := rules.ParseDate(in.VisitedOn)
	if err != nil || vd.After(time.Now()) {
		return httpx.E(httpx.CodeValidation, "The visit date must be today or a past date.")
	}
	appID, n, visited, err := s.ownFollowup(r, id)
	if err != nil {
		return err
	}
	if visited != nil {
		return httpx.E(httpx.CodeTransition, "This follow-up report is already signed.")
	}
	final := cleanSections(in.FinalReport, ai.FollowupSections)
	empty := true
	for _, v := range final {
		if v != "" {
			empty = false
		}
	}
	if empty {
		return httpx.E(httpx.CodeValidation, "The report is empty.")
	}
	status := ""
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		cur, err := workflow.LockStatus(r.Context(), tx, appID)
		if err != nil {
			return err
		}
		if cur != "ADOPTED_FOLLOWUP" {
			return httpx.E(httpx.CodeTransition, "Follow-up reports can be signed only after the adoption order.")
		}
		b, _ := json.Marshal(final)
		var draft []byte
		if err := tx.QueryRow(r.Context(), `UPDATE followups SET final_report=$2, visited_on=$3, signed_by=$4 WHERE id=$1 RETURNING ai_draft`, id, b, vd, u.ID).Scan(&draft); err != nil {
			return err
		}
		var dm struct {
			Sections map[string]string `json:"sections"`
			RunID    *uuid.UUID        `json:"ai_run_id"`
		}
		_ = json.Unmarshal(draft, &dm)
		if dm.RunID != nil {
			hd := "ACCEPTED"
			for _, k := range ai.FollowupSections {
				if strings.TrimSpace(dm.Sections[k]) != final[k] {
					hd = "EDITED"
				}
			}
			_, _ = tx.Exec(r.Context(), `UPDATE ai_runs SET human_decision=$2 WHERE id=$1`, *dm.RunID, hd)
		}
		if _, err := tx.Exec(r.Context(), `UPDATE case_deadlines SET state='DONE', completed_at=now() WHERE id = (
			SELECT id FROM case_deadlines WHERE application_id=$1 AND step='FOLLOWUP_VISIT' AND state IN ('OPEN','OVERDUE') ORDER BY due_at LIMIT 1)`, appID); err != nil {
			return err
		}
		status = cur
		var remaining int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM followups WHERE application_id=$1 AND visited_on IS NULL`, appID).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			if _, err := workflow.Transition(r.Context(), tx, appID, "COMPLETED"); err != nil {
				return err
			}
			status = "COMPLETED"
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "SIGN_FOLLOWUP", "followup", id.String(), map[string]any{"visit_number": n}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"ok": true, "status": status})
	return nil
}
