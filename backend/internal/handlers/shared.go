package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"childconnect/internal/audit"
	"childconnect/internal/httpx"
	"childconnect/internal/rules"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// canAccessCase: a parent only their own case; an agency user only cases of their own agency.
func (s *Server) canAccessCase(ctx context.Context, u *httpx.User, appID uuid.UUID) (bool, error) {
	var parentID uuid.UUID
	var agencyID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT parent_id, agency_id FROM applications WHERE id=$1`, appID).Scan(&parentID, &agencyID)
	if err == pgx.ErrNoRows {
		return false, httpx.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	switch u.Role {
	case httpx.RoleParent:
		return parentID == u.ID, nil
	case httpx.RoleAgency:
		return agencyID != nil && u.AgencyID != nil && *agencyID == *u.AgencyID, nil
	}
	return false, nil
}

type messageOut struct {
	ID         uuid.UUID `json:"id"`
	SenderName string    `json:"sender_name"`
	SenderRole string    `json:"sender_role"`
	Body       string    `json:"body"`
	Escalation bool      `json:"from_assistant_escalation"`
	CreatedAt  string    `json:"created_at"`
}

// GET /applications/:id/messages
func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	ok, err := s.canAccessCase(r.Context(), u, appID)
	if err != nil {
		return err
	}
	if !ok {
		return s.denied(r, "application", appID.String())
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT m.id, su.full_name, su.role::text, m.body, m.from_assistant_escalation, m.created_at
		FROM messages m JOIN users su ON su.id=m.sender_id WHERE m.application_id=$1 ORDER BY m.created_at LIMIT 500`, appID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []messageOut{}
	for rows.Next() {
		var m messageOut
		var at time.Time
		if err := rows.Scan(&m.ID, &m.SenderName, &m.SenderRole, &m.Body, &m.Escalation, &at); err != nil {
			return err
		}
		m.CreatedAt = at.UTC().Format(time.RFC3339)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Mark the other side's messages as read.
	_, _ = s.Pool.Exec(r.Context(), `UPDATE messages m SET read_at=now() FROM users su
		WHERE m.application_id=$1 AND su.id=m.sender_id AND su.role<>$2::user_role AND m.read_at IS NULL`, appID, string(u.Role))
	httpx.OK(w, out)
	return nil
}

// POST /applications/:id/messages
func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	appID, err := pathUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Body string `json:"body" validate:"required,max=4000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return httpx.E(httpx.CodeValidation, "The message is empty.")
	}
	ok, err := s.canAccessCase(r.Context(), u, appID)
	if err != nil {
		return err
	}
	if !ok {
		return s.denied(r, "application", appID.String())
	}
	m, err := s.insertMessage(r.Context(), u, appID, body, false)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, m)
	return nil
}

func (s *Server) insertMessage(ctx context.Context, u *httpx.User, appID uuid.UUID, body string, escalation bool) (*messageOut, error) {
	m := &messageOut{SenderName: u.FullName, SenderRole: string(u.Role), Body: body, Escalation: escalation}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var at time.Time
		if err := tx.QueryRow(ctx, `INSERT INTO messages (application_id, sender_id, body, from_assistant_escalation) VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
			appID, u.ID, body, escalation).Scan(&m.ID, &at); err != nil {
			return err
		}
		m.CreatedAt = at.UTC().Format(time.RFC3339)
		preview := body
		if len([]rune(preview)) > 120 {
			preview = string([]rune(preview)[:120]) + "..."
		}
		if u.Role == httpx.RoleParent {
			_, err := tx.Exec(ctx, `INSERT INTO notifications (user_id, type, title, body, link)
				SELECT social_worker_id, 'MESSAGE', $2, $3, $4 FROM applications WHERE id=$1 AND social_worker_id IS NOT NULL`,
				appID, "New message from "+u.FullName, preview, "/agency/cases/"+appID.String())
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO notifications (user_id, type, title, body, link) SELECT parent_id, 'MESSAGE', $2, $3, '/parent/messages' FROM applications WHERE id=$1`,
			appID, "New message from your social worker", preview)
		return err
	})
	return m, err
}

// POST /ai/assistant
func (s *Server) assistant(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		Question string `json:"question" validate:"required,min=2,max=1000"`
		Language string `json:"language"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	lang := in.Language
	if !rules.ValidLang(lang) {
		lang = s.lang(r)
	}
	a, err := s.AI.Ask(r.Context(), strings.TrimSpace(in.Question), lang, &u.ID)
	if err != nil {
		return err
	}
	httpx.OK(w, a)
	return nil
}

// POST /ai/assistant/escalate: sends the question to the social worker as a case message.
func (s *Server) escalate(w http.ResponseWriter, r *http.Request) error {
	u := httpx.UserFrom(r.Context())
	var in struct {
		Question string `json:"question" validate:"required,min=2,max=1000"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	appID, err := s.myApplicationID(r.Context(), u)
	if err != nil {
		return err
	}
	m, err := s.insertMessage(r.Context(), u, appID, "Question sent from the Sahayak assistant: "+strings.TrimSpace(in.Question), true)
	if err != nil {
		return err
	}
	_ = s.Audit.Log(r.Context(), audit.FromUser(u, "ASSISTANT_ESCALATED", "message", m.ID.String(), nil))
	httpx.JSON(w, http.StatusCreated, map[string]any{"message_id": m.ID})
	return nil
}
