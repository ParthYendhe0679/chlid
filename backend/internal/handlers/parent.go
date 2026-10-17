package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

type applicationInput struct {
	MaritalStatus    *string   `json:"marital_status" validate:"omitempty,oneof=MARRIED SINGLE_FEMALE SINGLE_MALE DIVORCED WIDOWED"`
	YearsMarried     *float64  `json:"years_married" validate:"omitempty,min=0,max=80"`
	ApplicantDOB     *string   `json:"applicant_dob"`
	SpouseDOB        *string   `json:"spouse_dob"`
	ExistingChildren *int      `json:"existing_children" validate:"omitempty,min=0,max=30"`
	AnnualIncomeINR  *float64  `json:"annual_income_inr" validate:"omitempty,min=0,max=1000000000"`
	State            *string   `json:"state" validate:"omitempty,max=60"`
	District         *string   `json:"district" validate:"omitempty,max=80"`
	Address          *string   `json:"address" validate:"omitempty,max=500"`
	PrefAgeGroup     *string   `json:"pref_age_group" validate:"omitempty,oneof=0-2 2-4 4-8 8-18"`
	PrefGender       *string   `json:"pref_gender" validate:"omitempty,oneof=ANY MALE FEMALE"`
	PrefStates       *[]string `json:"pref_states" validate:"omitempty,max=40,dive,max=60"`
	OpenSpecialNeeds *bool     `json:"open_special_needs"`
	OpenSiblings     *bool     `json:"open_siblings"`
}

// profile is the full, merged profile used for validation and storage.
type profile struct {
	MaritalStatus    string
	YearsMarried     *float64
	ApplicantDOB     string
	SpouseDOB        *string
	ExistingChildren int
	AnnualIncomeINR  *float64
	State, District  string
	Address          *string
	PrefAgeGroup     string
	PrefGender       string
	PrefStates       []string
	OpenSpecialNeeds bool
	OpenSiblings     bool
}

func (p *profile) apply(in applicationInput) {
	if in.MaritalStatus != nil {
		p.MaritalStatus = *in.MaritalStatus
	}
	if in.YearsMarried != nil {
		p.YearsMarried = in.YearsMarried
	}
	if in.ApplicantDOB != nil {
		p.ApplicantDOB = *in.ApplicantDOB
	}
	if in.SpouseDOB != nil {
		p.SpouseDOB = in.SpouseDOB
	}
	if in.ExistingChildren != nil {
		p.ExistingChildren = *in.ExistingChildren
	}
	if in.AnnualIncomeINR != nil {
		p.AnnualIncomeINR = in.AnnualIncomeINR
	}
	if in.State != nil {
		p.State = strings.TrimSpace(*in.State)
	}
	if in.District != nil {
		p.District = strings.TrimSpace(*in.District)
	}
	if in.Address != nil {
		p.Address = in.Address
	}
	if in.PrefAgeGroup != nil {
		p.PrefAgeGroup = *in.PrefAgeGroup
	}
	if in.PrefGender != nil {
		p.PrefGender = *in.PrefGender
	}
	if in.PrefStates != nil {
		p.PrefStates = *in.PrefStates
	}
	if in.OpenSpecialNeeds != nil {
		p.OpenSpecialNeeds = *in.OpenSpecialNeeds
	}
	if in.OpenSiblings != nil {
		p.OpenSiblings = *in.OpenSiblings
	}
	if p.MaritalStatus != "MARRIED" {
		p.SpouseDOB, p.YearsMarried = nil, nil
	}
	if p.SpouseDOB != nil && *p.SpouseDOB == "" {
		p.SpouseDOB = nil
	}
}

// validate checks required fields and the eligibility rules of section 3.2.
func (p *profile) validate(lang string) error {
	switch {
	case p.MaritalStatus == "":
		return httpx.E(httpx.CodeValidation, "Field 'marital_status' is required.")
	case p.ApplicantDOB == "":
		return httpx.E(httpx.CodeValidation, "Field 'applicant_dob' is required.")
	case p.State == "" || p.District == "":
		return httpx.E(httpx.CodeValidation, "State and district are required.")
	case p.PrefAgeGroup == "" || p.PrefGender == "":
		return httpx.E(httpx.CodeValidation, "Preferred age group and gender are required.")
	case p.MaritalStatus == "MARRIED" && p.SpouseDOB == nil:
		return httpx.E(httpx.CodeValidation, "Field 'spouse_dob' is required for a married couple.")
	}
	if p.PrefStates == nil {
		p.PrefStates = []string{}
	}
	in := rules.EligibilityInput{MaritalStatus: p.MaritalStatus, ApplicantDOB: p.ApplicantDOB, YearsMarried: p.YearsMarried, ExistingChildren: p.ExistingChildren}
	if p.SpouseDOB != nil {
		in.SpouseDOB = *p.SpouseDOB
	}
	res, err := rules.CheckEligibility(in, lang, time.Now().In(rules.IST))
	if err != nil {
		return httpx.E(httpx.CodeValidation, err.Error())
	}
	if !res.Eligible {
		var msgs []string
		for _, r := range res.Reasons {
			if r.Code == "MARRIAGE_DURATION" || r.Code == "CHILDREN_COUNT" || r.Code == "NO_AGE_GROUP" {
				msgs = append(msgs, r.Message)
			}
		}
		return httpx.E(httpx.CodeValidation, "These details are not eligible for adoption: "+strings.Join(msgs, " "))
	}
	ok := false
	for _, g := range res.AllowedAgeGroups {
		if g == p.PrefAgeGroup {
			ok = true
		}
	}
	if !ok {
		return httpx.E(httpx.CodeValidation, fmt.Sprintf("With these details you can adopt children in these age groups: %s. Please choose one of them.", strings.Join(res.AllowedAgeGroups, ", ")))
	}
	if p.MaritalStatus == "SINGLE_MALE" && p.PrefGender == "FEMALE" {
		return httpx.E(httpx.CodeValidation, "A single man cannot adopt a girl child. Please choose 'Boy' or 'Any'.")
	}
	return nil
}

// myApplicationID returns the parent's application id, or NOT_FOUND.
func (s *Server) myApplicationID(ctx context.Context, u *httpx.User) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT id FROM applications WHERE parent_id=$1`, u.ID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, httpx.E(httpx.CodeNotFound, "You have not created your application yet.")
	}
	return id, err
}

// assignAgency picks an approved agency near the parent (same district, then same state, then least loaded)
// and its least busy social worker. This is a workload rule, not AI.
func assignAgency(ctx context.Context, tx pgx.Tx, state, district string) (*uuid.UUID, *uuid.UUID, error) {
	var agency uuid.UUID
	err := tx.QueryRow(ctx, `SELECT a.id FROM agencies a WHERE a.is_approved
		ORDER BY (lower(a.district)=lower($2) AND lower(a.state)=lower($1)) DESC, (lower(a.state)=lower($1)) DESC,
		  (SELECT count(*) FROM applications ap WHERE ap.agency_id=a.id AND ap.status NOT IN ('COMPLETED','LAPSED','DEBARRED','WITHDRAWN'))::float / GREATEST(a.staff_count,1),
		  a.created_at LIMIT 1`, state, district).Scan(&agency)
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var worker uuid.UUID
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u WHERE u.agency_id=$1 AND u.is_active
		ORDER BY (u.designation='SOCIAL_WORKER') DESC, (SELECT count(*) FROM applications ap WHERE ap.social_worker_id=u.id) LIMIT 1`, agency).Scan(&worker)
	if err == pgx.ErrNoRows {
		return &agency, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &agency, &worker, nil
}

// POST /applications
func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in applicationInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p := profile{}
	p.apply(in)
	if err := p.validate(s.lang(r)); err != nil {
		return err
	}
	var addrEnc *string
	if p.Address != nil && strings.TrimSpace(*p.Address) != "" {
		e, err := s.Box.Encrypt(strings.TrimSpace(*p.Address))
		if err != nil {
			return err
		}
		addrEnc = &e
	}
	var appID uuid.UUID
	var due time.Time
	err := s.tx(r.Context(), func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM applications WHERE parent_id=$1)`, u.ID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return httpx.E(httpx.CodeValidation, "You already have an application. Update it from your profile page.")
		}
		agency, worker, err := assignAgency(r.Context(), tx, p.State, p.District)
		if err != nil {
			return err
		}
		err = tx.QueryRow(r.Context(), `INSERT INTO applications (parent_id, agency_id, social_worker_id, marital_status, years_married, applicant_dob, spouse_dob,
			existing_children, annual_income_inr, state, district, address_enc, pref_age_group, pref_gender, pref_states, open_special_needs, open_siblings)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id, registered_at`,
			u.ID, agency, worker, p.MaritalStatus, p.YearsMarried, p.ApplicantDOB, p.SpouseDOB, p.ExistingChildren, p.AnnualIncomeINR,
			p.State, p.District, addrEnc, p.PrefAgeGroup, p.PrefGender, p.PrefStates, p.OpenSpecialNeeds, p.OpenSiblings).Scan(&appID, &due)
		if err != nil {
			return err
		}
		start := due
		due = start.Add(workflow.StepLimit["DOC_UPLOAD_30D"])
		if err := workflow.AddDeadline(r.Context(), tx, &appID, nil, agency, "DOC_UPLOAD_30D", start, nil); err != nil {
			return err
		}
		_ = workflow.NotifyWorker(r.Context(), tx, appID, "NEW_CASE", "New parent application", u.FullName+" created an application assigned to you.", "/agency/cases/"+appID.String())
		_ = workflow.NotifyParent(r.Context(), tx, appID, "DOC_DEADLINE", "Upload your documents", "Please upload all required documents within 30 days.", "/parent/documents")
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "APPLICATION_CREATED", "application", appID.String(), nil))
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"application_id": appID, "status": "REGISTERED", "doc_deadline": due.UTC().Format(time.RFC3339)})
	return nil
}

type requiredDoc struct {
	Category     string     `json:"category"`
	Uploaded     bool       `json:"uploaded"`
	Status       *string    `json:"status"`
	DocumentID   *uuid.UUID `json:"document_id,omitempty"`
	ReviewerNote *string    `json:"reviewer_note"`
}

func (s *Server) requiredDocs(ctx context.Context, appID uuid.UUID, marital string, children int) ([]requiredDoc, error) {
	rows, err := s.Pool.Query(ctx, `SELECT category::text, id, status::text, reviewer_note FROM documents WHERE application_id=$1 AND is_current`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type cur struct {
		id     uuid.UUID
		status string
		note   *string
	}
	have := map[string]cur{}
	for rows.Next() {
		var c string
		var x cur
		if err := rows.Scan(&c, &x.id, &x.status, &x.note); err != nil {
			return nil, err
		}
		have[c] = x
	}
	out := []requiredDoc{}
	for _, c := range rules.RequiredCategories(marital, children) {
		d := requiredDoc{Category: c}
		if x, ok := have[c]; ok {
			id, st := x.id, x.status
			d.Uploaded, d.DocumentID, d.Status, d.ReviewerNote = true, &id, &st, x.note
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GET /applications/me
func (s *Server) myApplication(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var (
		id                               uuid.UUID
		status, marital, state, district string
		prefAge, prefGender              string
		years, income                    *float64
		dob                              time.Time
		spouse                           *time.Time
		children, used                   int
		addrEnc                          *string
		prefStates                       []string
		special, siblings                bool
		registered                       time.Time
		submitted, seniority             *time.Time
		hsrValid, debarred               *time.Time
		agencyName, workerName           *string
	)
	err := s.Pool.QueryRow(r.Context(), `SELECT a.id, a.status::text, a.marital_status, a.years_married::float8, a.applicant_dob, a.spouse_dob, a.existing_children,
		a.annual_income_inr::float8, a.state, a.district, a.address_enc, a.pref_age_group, a.pref_gender, a.pref_states, a.open_special_needs, a.open_siblings,
		a.registered_at, a.docs_submitted_at, a.seniority_date, a.hsr_valid_until, a.referrals_used, a.debarred_until, ag.name, sw.full_name
		FROM applications a LEFT JOIN agencies ag ON ag.id=a.agency_id LEFT JOIN users sw ON sw.id=a.social_worker_id WHERE a.parent_id=$1`, u.ID).
		Scan(&id, &status, &marital, &years, &dob, &spouse, &children, &income, &state, &district, &addrEnc, &prefAge, &prefGender, &prefStates,
			&special, &siblings, &registered, &submitted, &seniority, &hsrValid, &used, &debarred, &agencyName, &workerName)
	if err == pgx.ErrNoRows {
		return httpx.E(httpx.CodeNotFound, "You have not created your application yet.")
	}
	if err != nil {
		return err
	}
	var address *string
	if addrEnc != nil {
		if a, err := s.Box.Decrypt(*addrEnc); err == nil {
			address = &a
		}
	}
	docs, err := s.requiredDocs(r.Context(), id, marital, children)
	if err != nil {
		return err
	}
	var docDeadline *string
	if status == "REGISTERED" || status == "DOCS_CORRECTION" || status == "LAPSED" {
		var due time.Time
		if s.Pool.QueryRow(r.Context(), `SELECT due_at FROM case_deadlines WHERE application_id=$1 AND step='DOC_UPLOAD_30D' ORDER BY started_at DESC LIMIT 1`, id).Scan(&due) == nil {
			docDeadline = tsStr(&due)
		}
	}
	httpx.OK(w, map[string]any{
		"id": id, "status": status, "marital_status": marital, "years_married": years, "applicant_dob": dob.Format("2006-01-02"),
		"spouse_dob": dateStr(spouse), "existing_children": children, "annual_income_inr": income, "state": state, "district": district,
		"address": address, "pref_age_group": prefAge, "pref_gender": prefGender, "pref_states": prefStates,
		"open_special_needs": special, "open_siblings": siblings, "registered_at": registered.UTC().Format(time.RFC3339),
		"doc_deadline": docDeadline, "seniority_date": tsStr(seniority), "hsr_valid_until": dateStr(hsrValid), "referrals_used": used,
		"debarred_until": dateStr(debarred), "agency_name": agencyName, "social_worker_name": workerName, "required_docs": docs,
	})
	return nil
}

// PATCH /applications/me: allowed only in REGISTERED and DOCS_CORRECTION.
func (s *Server) updateApplication(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in applicationInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	err := s.tx(r.Context(), func(tx pgx.Tx) error {
		var (
			id      uuid.UUID
			status  string
			p       profile
			dob     time.Time
			spouse  *time.Time
			addrEnc *string
		)
		err := tx.QueryRow(r.Context(), `SELECT id, status::text, marital_status, years_married::float8, applicant_dob, spouse_dob, existing_children, annual_income_inr::float8,
			state, district, address_enc, pref_age_group, pref_gender, pref_states, open_special_needs, open_siblings FROM applications WHERE parent_id=$1 FOR UPDATE`, u.ID).
			Scan(&id, &status, &p.MaritalStatus, &p.YearsMarried, &dob, &spouse, &p.ExistingChildren, &p.AnnualIncomeINR, &p.State, &p.District, &addrEnc,
				&p.PrefAgeGroup, &p.PrefGender, &p.PrefStates, &p.OpenSpecialNeeds, &p.OpenSiblings)
		if err == pgx.ErrNoRows {
			return httpx.E(httpx.CodeNotFound, "You have not created your application yet.")
		}
		if err != nil {
			return err
		}
		if status != "REGISTERED" && status != "DOCS_CORRECTION" {
			return httpx.E(httpx.CodeTransition, "Your profile is locked because your case has moved past document submission.")
		}
		p.ApplicantDOB = dob.Format("2006-01-02")
		p.SpouseDOB = dateStr(spouse)
		p.apply(in)
		if err := p.validate(s.lang(r)); err != nil {
			return err
		}
		if in.Address != nil {
			addrEnc = nil
			if a := strings.TrimSpace(*in.Address); a != "" {
				e, err := s.Box.Encrypt(a)
				if err != nil {
					return err
				}
				addrEnc = &e
			}
		}
		_, err = tx.Exec(r.Context(), `UPDATE applications SET marital_status=$2, years_married=$3, applicant_dob=$4, spouse_dob=$5, existing_children=$6,
			annual_income_inr=$7, state=$8, district=$9, address_enc=$10, pref_age_group=$11, pref_gender=$12, pref_states=$13, open_special_needs=$14,
			open_siblings=$15, updated_at=now() WHERE id=$1`, id, p.MaritalStatus, p.YearsMarried, p.ApplicantDOB, p.SpouseDOB, p.ExistingChildren,
			p.AnnualIncomeINR, p.State, p.District, addrEnc, p.PrefAgeGroup, p.PrefGender, p.PrefStates, p.OpenSpecialNeeds, p.OpenSiblings)
		if err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "APPLICATION_UPDATED", "application", id.String(), nil))
	})
	if err != nil {
		return err
	}
	return s.myApplication(w, r)
}

// GET /applications/me/tracker
func (s *Server) tracker(w http.ResponseWriter, r *http.Request) error {
	id, err := s.myApplicationID(r.Context(), httpx.UserFrom(r.Context()))
	if err != nil {
		return err
	}
	t, err := workflow.BuildTracker(r.Context(), s.Pool, id, s.lang(r), time.Now())
	if err != nil {
		return err
	}
	httpx.OK(w, t)
	return nil
}

const maxUpload = 5 << 20

// POST /applications/me/documents: uploads one document and runs DocCheck AI.
func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(512<<10))
	if err := r.ParseMultipartForm(maxUpload + (512 << 10)); err != nil {
		return httpx.E(httpx.CodeValidation, "The file is too large. The limit is 5 MB.")
	}
	category := strings.TrimSpace(r.FormValue("category"))
	file, hdr, err := r.FormFile("file")
	if err != nil {
		return httpx.E(httpx.CodeValidation, "Please choose a file to upload.")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return httpx.E(httpx.CodeValidation, "The file is empty.")
	}
	if len(data) > maxUpload {
		return httpx.E(httpx.CodeValidation, "The file is too large. The limit is 5 MB.")
	}
	mime, ok := storage.SniffType(data)
	if !ok {
		return httpx.E(httpx.CodeValidation, "Only PDF, JPG and PNG files are allowed. The file content does not match these types.")
	}

	var (
		appID           uuid.UUID
		status, marital string
		children        int
	)
	err = s.Pool.QueryRow(r.Context(), `SELECT id, status::text, marital_status, existing_children FROM applications WHERE parent_id=$1`, u.ID).Scan(&appID, &status, &marital, &children)
	if err == pgx.ErrNoRows {
		return httpx.E(httpx.CodeNotFound, "Please create your profile before uploading documents.")
	}
	if err != nil {
		return err
	}
	if status != "REGISTERED" && status != "DOCS_CORRECTION" {
		return httpx.E(httpx.CodeTransition, "Documents can be uploaded only before submission or when your social worker asks for a correction.")
	}
	allowed := false
	for _, c := range rules.RequiredCategories(marital, children) {
		if c == category {
			allowed = true
		}
	}
	if !allowed {
		return httpx.E(httpx.CodeValidation, "This document category is not needed for your application.")
	}
	var curStatus string
	if err := s.Pool.QueryRow(r.Context(), `SELECT status::text FROM documents WHERE application_id=$1 AND category=$2 AND is_current`, appID, category).Scan(&curStatus); err == nil && curStatus == "VERIFIED" {
		return httpx.E(httpx.CodeTransition, "This document is already verified by your social worker and cannot be changed.")
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var dup bool
	if err := s.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM documents WHERE sha256=$1 AND application_id<>$2)`, hash, appID).Scan(&dup); err != nil {
		return err
	}
	var others []string
	if category == "IDENTITY" || category == "RESIDENCE" {
		rows, err := s.Pool.Query(r.Context(), `SELECT ai_fields->>'address' FROM documents WHERE application_id=$1 AND is_current AND category IN ('IDENTITY','RESIDENCE')
			AND category<>$2 AND ai_fields->>'address' IS NOT NULL`, appID, category)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a string
			if rows.Scan(&a) == nil && a != "" {
				others = append(others, a)
			}
		}
		rows.Close()
	}

	res := s.AI.CheckDocument(r.Context(), ai.DocInput{Data: data, Mime: mime, Category: category, ApplicantNames: ai.ApplicantNames(u.FullName),
		OtherAddresses: others, Duplicate: dup, Now: time.Now().In(rules.IST)})

	key, err := s.Store.Save("docs", data, mime)
	if err != nil {
		return err
	}
	fields, _ := json.Marshal(res.Fields)
	checks, _ := json.Marshal(res.Checks)
	fileName := safeName(hdr.Filename, mime)
	var docID uuid.UUID
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE documents SET is_current=false WHERE application_id=$1 AND category=$2 AND is_current`, appID, category); err != nil {
			return err
		}
		err := tx.QueryRow(r.Context(), `INSERT INTO documents (application_id, category, storage_key, file_name, mime_type, size_bytes, sha256,
			ai_detected_type, ai_fields, ai_confidence, ai_checks, status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
			appID, category, key, fileName, mime, len(data), hash, res.DetectedType, fields, res.Confidence, checks, res.Status).Scan(&docID)
		if err != nil {
			return err
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "DOC_UPLOADED", "document", docID.String(),
			map[string]any{"category": category, "status": res.Status, "sha256": hash}))
	})
	if err != nil {
		return err
	}
	_, _ = s.AI.RecordRun(r.Context(), "DOCCHECK", &u.ID, "document", &docID, res.Model, res.LatencyMs, res, res.Confidence)

	lang := s.lang(r)
	outChecks := make([]map[string]string, 0, len(res.Checks))
	for _, c := range res.Checks {
		outChecks = append(outChecks, map[string]string{"code": c.Code, "result": c.Result, "message": c.Message(lang)})
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"document_id": docID, "status": res.Status, "detected_type": res.DetectedType, "confidence": res.Confidence, "checks": outChecks,
	})
	return nil
}

func safeName(n, mime string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, n)
	if len(n) > 120 {
		n = n[len(n)-120:]
	}
	if n == "" {
		n = "document." + map[string]string{"application/pdf": "pdf", "image/jpeg": "jpg", "image/png": "png"}[mime]
	}
	return n
}

// POST /applications/me/submit
func (s *Server) submitDocuments(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var seniority time.Time
	err := s.tx(r.Context(), func(tx pgx.Tx) error {
		var id uuid.UUID
		var marital string
		var children int
		err := tx.QueryRow(r.Context(), `SELECT id, marital_status, existing_children FROM applications WHERE parent_id=$1`, u.ID).Scan(&id, &marital, &children)
		if err == pgx.ErrNoRows {
			return httpx.E(httpx.CodeNotFound, "You have not created your application yet.")
		}
		if err != nil {
			return err
		}
		docs, err := s.requiredDocs(r.Context(), id, marital, children)
		if err != nil {
			return err
		}
		var missing []string
		for _, d := range docs {
			if !d.Uploaded || (d.Status != nil && *d.Status == "REUPLOAD_REQUESTED") {
				missing = append(missing, rules.HumanType(d.Category))
			}
		}
		if len(missing) > 0 {
			return httpx.E(httpx.CodeValidation, "These documents are still missing or need a new upload: "+strings.Join(missing, ", ")+".")
		}
		from, err := workflow.Transition(r.Context(), tx, id, "DOCS_SUBMITTED")
		if err != nil {
			return err
		}
		err = tx.QueryRow(r.Context(), `UPDATE applications SET docs_submitted_at=COALESCE(docs_submitted_at, now()), seniority_date=COALESCE(seniority_date, now())
			WHERE id=$1 RETURNING seniority_date`, id).Scan(&seniority)
		if err != nil {
			return err
		}
		if err := workflow.CompleteSteps(r.Context(), tx, id, "DOC_UPLOAD_30D"); err != nil {
			return err
		}
		msg := u.FullName + " submitted documents for review."
		if from == "DOCS_CORRECTION" {
			msg = u.FullName + " uploaded the corrected documents."
		}
		_ = workflow.NotifyWorker(r.Context(), tx, id, "DOCS_SUBMITTED", "Documents ready for review", msg, "/agency/cases/"+id.String())
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "DOCS_SUBMITTED", "application", id.String(), map[string]any{"from": from}))
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"status": "DOCS_SUBMITTED", "seniority_date": seniority.UTC().Format(time.RFC3339)})
	return nil
}

// GET /documents/:id/url: a 5-minute link after a permission check; every request is audited.
func (s *Server) documentURL(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	id, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var key string
	var parentID uuid.UUID
	var agencyID *uuid.UUID
	err = s.Pool.QueryRow(r.Context(), `SELECT d.storage_key, a.parent_id, a.agency_id FROM documents d JOIN applications a ON a.id=d.application_id WHERE d.id=$1`, id).
		Scan(&key, &parentID, &agencyID)
	if err == pgx.ErrNoRows {
		return httpx.ErrNotFound
	}
	if err != nil {
		return err
	}
	allowed := (u.Role == httpx.RoleParent && parentID == u.ID) ||
		(u.Role == httpx.RoleAgency && agencyID != nil && u.AgencyID != nil && *agencyID == *u.AgencyID)
	if !allowed {
		return s.denied(r, "document", id.String())
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "VIEW_DOCUMENT", "document", id.String(), nil))
	httpx.OK(w, map[string]any{"url": s.Store.SignedURL(key), "expires_in": int(storage.LinkTTL.Seconds())})
	return nil
}

// GET /files/{prefix}/{name}?exp=&sig= serves a file only with a valid, unexpired signature.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	q := r.URL.Query()
	if !s.Store.VerifyLink(key, q.Get("exp"), q.Get("sig")) {
		httpx.Fail(w, r, httpx.E(httpx.CodeForbidden, "This link has expired. Please open the file again from the website."))
		return
	}
	data, err := s.Store.Read(key)
	if err != nil {
		httpx.Fail(w, r, httpx.ErrNotFound)
		return
	}
	mime, _ := storage.SniffType(data)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}

// GET /referrals/active: the pending referral, or null.
func (s *Server) activeReferral(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var (
		refID, childID          uuid.UUID
		number                  int
		expires                 time.Time
		code, gender            string
		special                 bool
		dob                     time.Time
		summary, csrKey, merKey *string
	)
	err := s.Pool.QueryRow(r.Context(), `SELECT rf.id, rf.referral_number, rf.expires_at, c.id, c.case_code, c.gender, c.is_special_needs, c.dob,
		c.summary_approved, c.csr_storage_key, c.mer_storage_key
		FROM referrals rf JOIN applications a ON a.id=rf.application_id JOIN children c ON c.id=rf.child_id
		WHERE a.parent_id=$1 AND rf.decision='PENDING' AND rf.expires_at > now() ORDER BY rf.starts_at DESC LIMIT 1`, u.ID).
		Scan(&refID, &number, &expires, &childID, &code, &gender, &special, &dob, &summary, &csrKey, &merKey)
	if err == pgx.ErrNoRows {
		httpx.OK(w, nil)
		return nil
	}
	if err != nil {
		return err
	}
	link := func(k *string) string {
		if k == nil {
			return ""
		}
		return s.Store.SignedURL(*k)
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "VIEW_CHILD_REPORTS", "child", childID.String(), map[string]any{"referral_id": refID.String()}))
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	httpx.OK(w, map[string]any{
		"referral_id": refID, "referral_number": number, "expires_at": expires.UTC().Format(time.RFC3339),
		"child": map[string]any{
			"case_code": code, "age_text": rules.AgeText(dob, time.Now().In(rules.IST), s.lang(r)), "gender": gender, "is_special_needs": special,
			"summary_approved": str(summary), "csr_url": link(csrKey), "mer_url": link(merKey),
		},
	})
	return nil
}

var (
	warnRemaining = rules.Text{
		"You have used {used} of 3 referrals. If you do not reserve a child in 3 referrals, you will be blocked for 1 year (Regulation 9(3)).",
		"आपने 3 में से {used} रेफ़रल इस्तेमाल कर लिए हैं। अगर 3 रेफ़रल में आप कोई बच्चा आरक्षित नहीं करते, तो आप 1 साल के लिए रोक दिए जाएंगे (विनियम 9(3))।",
		"तुम्ही 3 पैकी {used} संदर्भ वापरले आहेत. 3 संदर्भांमध्ये मूल आरक्षित न केल्यास तुम्हाला 1 वर्षासाठी प्रतिबंधित केले जाईल (विनियम 9(3)).",
	}
	warnDebarred = rules.Text{
		"No child was reserved in 3 referrals. Under Regulation 9(3) you cannot apply again until {date}.",
		"3 रेफ़रल में कोई बच्चा आरक्षित नहीं किया गया। विनियम 9(3) के तहत आप {date} तक फिर आवेदन नहीं कर सकते।",
		"3 संदर्भांमध्ये एकही मूल आरक्षित केले नाही. विनियम 9(3) नुसार तुम्ही {date} पर्यंत पुन्हा अर्ज करू शकत नाही.",
	}
	warnReserved = rules.Text{
		"You reserved the child. Matching must be completed within 30 days. If you do not accept the child after reserving, you will move to the bottom of the waiting list (Regulation 11(14)).",
		"आपने बच्चे को आरक्षित किया है। मिलान 30 दिनों में पूरा होना चाहिए। आरक्षित करने के बाद बच्चे को स्वीकार न करने पर आप प्रतीक्षा सूची में सबसे नीचे चले जाएंगे (विनियम 11(14))।",
		"तुम्ही मूल आरक्षित केले आहे. जुळणी 30 दिवसांत पूर्ण व्हायला हवी. आरक्षणानंतर मूल न स्वीकारल्यास तुम्ही प्रतीक्षा यादीत सर्वात खाली जाल (विनियम 11(14)).",
	}
)

// POST /referrals/:id/decision
func (s *Server) referralDecision(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	refID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Decision string `json:"decision" validate:"required,oneof=RESERVED NOT_RESERVED"`
		Reason   string `json:"reason" validate:"max=1000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Decision == "NOT_RESERVED" && len(strings.TrimSpace(in.Reason)) < 5 {
		return httpx.E(httpx.CodeValidation, "Please give a short reason (at least 5 characters).")
	}
	lang := s.lang(r)
	var status, warning string
	err = s.tx(r.Context(), func(tx pgx.Tx) error {
		var appID, childID uuid.UUID
		var parentID uuid.UUID
		var decision string
		var expires time.Time
		var agencyID *uuid.UUID
		err := tx.QueryRow(r.Context(), `SELECT rf.application_id, rf.child_id, a.parent_id, rf.decision, rf.expires_at, a.agency_id
			FROM referrals rf JOIN applications a ON a.id=rf.application_id WHERE rf.id=$1 FOR UPDATE OF rf`, refID).
			Scan(&appID, &childID, &parentID, &decision, &expires, &agencyID)
		if err == pgx.ErrNoRows {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if parentID != u.ID {
			return errForbiddenAudit
		}
		if decision != "PENDING" {
			return httpx.E(httpx.CodeTransition, "A decision was already recorded for this referral.")
		}
		if time.Now().After(expires) {
			return httpx.E(httpx.CodeTransition, "The 48-hour decision window has ended.")
		}
		if in.Decision == "RESERVED" {
			if _, err := workflow.Transition(r.Context(), tx, appID, "RESERVED"); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `UPDATE referrals SET decision='RESERVED', decided_at=now() WHERE id=$1`, refID); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `UPDATE children SET status='RESERVED' WHERE id=$1`, childID); err != nil {
				return err
			}
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "RESERVE_48H"); err != nil {
				return err
			}
			// From reservation on, the child's agency handles matching and the District Magistrate steps,
			// so the case moves to that agency and to the staff member who sent the referral.
			if err := tx.QueryRow(r.Context(), `UPDATE applications a SET agency_id=c.agency_id, social_worker_id=rf.sent_by
				FROM referrals rf JOIN children c ON c.id=rf.child_id WHERE rf.id=$1 AND a.id=rf.application_id RETURNING a.agency_id`, refID).Scan(&agencyID); err != nil {
				return err
			}
			if err := workflow.AddDeadline(r.Context(), tx, &appID, &childID, agencyID, "MATCHING_30D", time.Now(), nil); err != nil {
				return err
			}
			status, warning = "RESERVED", warnReserved.In(lang)
			_ = workflow.NotifyWorker(r.Context(), tx, appID, "REFERRAL_RESERVED", "Child reserved", u.FullName+" reserved the referred child. Matching must finish within 30 days.", "/agency/cases/"+appID.String())
		} else {
			var used int
			if err := tx.QueryRow(r.Context(), `UPDATE applications SET referrals_used = LEAST(referrals_used + 1, 3) WHERE id=$1 RETURNING referrals_used`, appID).Scan(&used); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `UPDATE referrals SET decision='NOT_RESERVED', decision_reason=$2, decided_at=now() WHERE id=$1`, refID, strings.TrimSpace(in.Reason)); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `UPDATE children SET status='READY' WHERE id=$1`, childID); err != nil {
				return err
			}
			if err := workflow.CompleteSteps(r.Context(), tx, appID, "RESERVE_48H"); err != nil {
				return err
			}
			if used >= 3 {
				if _, err := workflow.Transition(r.Context(), tx, appID, "DEBARRED"); err != nil {
					return err
				}
				var until time.Time
				if err := tx.QueryRow(r.Context(), `UPDATE applications SET debarred_until=(now() + interval '1 year')::date WHERE id=$1 RETURNING debarred_until`, appID).Scan(&until); err != nil {
					return err
				}
				status, warning = "DEBARRED", warnDebarred.In(lang, "date", until.Format("2006-01-02"))
			} else {
				if _, err := workflow.Transition(r.Context(), tx, appID, "WAITING"); err != nil {
					return err
				}
				status, warning = "WAITING", warnRemaining.In(lang, "used", fmt.Sprint(used))
			}
			_ = workflow.NotifyWorker(r.Context(), tx, appID, "REFERRAL_NOT_RESERVED", "Referral not reserved", u.FullName+" did not reserve the referred child.", "/agency/cases/"+appID.String())
		}
		return audit.LogTx(r.Context(), tx, audit.FromUser(u, "REFERRAL_DECISION", "referral", refID.String(), map[string]any{"decision": in.Decision}))
	})
	if errors.Is(err, errForbiddenAudit) {
		return s.denied(r, "referral", refID.String())
	}
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"status": status, "warning": warning})
	return nil
}

var errForbiddenAudit = errors.New("forbidden")

// GET /followups/me
func (s *Server) myFollowups(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	rows, err := s.Pool.Query(r.Context(), `SELECT f.visit_number, f.due_date, f.visited_on FROM followups f JOIN applications a ON a.id=f.application_id
		WHERE a.parent_id=$1 ORDER BY f.visit_number`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var n int
		var due time.Time
		var visited *time.Time
		if err := rows.Scan(&n, &due, &visited); err != nil {
			return err
		}
		out = append(out, map[string]any{"visit_number": n, "due_date": due.Format("2006-01-02"), "visited_on": dateStr(visited)})
	}
	httpx.OK(w, out)
	return rows.Err()
}
