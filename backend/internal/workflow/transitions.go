// Package workflow enforces the application status rules (section 14), creates legal deadlines
// (section 3.3) and builds the Case Tracker.
package workflow

import (
	"context"
	"fmt"

	"childconnect/internal/httpx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// allowed lists every permitted status change from section 14. Anything else is INVALID_TRANSITION.
var allowed = map[string][]string{
	"REGISTERED":          {"DOCS_SUBMITTED", "LAPSED", "WITHDRAWN"},
	"DOCS_SUBMITTED":      {"DOCS_CORRECTION", "HSR_IN_PROGRESS", "WITHDRAWN"},
	"DOCS_CORRECTION":     {"DOCS_SUBMITTED", "WITHDRAWN"},
	"HSR_IN_PROGRESS":     {"WAITING", "WITHDRAWN"},
	"WAITING":             {"REFERRED", "WITHDRAWN"},
	"REFERRED":            {"RESERVED", "WAITING", "DEBARRED", "WITHDRAWN"},
	"RESERVED":            {"MATCHED_FOSTER_CARE", "WAITING"},
	"MATCHED_FOSTER_CARE": {"DM_ORDER_PENDING"},
	"DM_ORDER_PENDING":    {"ADOPTED_FOLLOWUP"},
	"ADOPTED_FOLLOWUP":    {"COMPLETED"},
}

func CanTransition(from, to string) bool {
	for _, s := range allowed[from] {
		if s == to {
			return true
		}
	}
	return false
}

// LockStatus reads the application's status and locks the row for the rest of the transaction.
func LockStatus(ctx context.Context, tx pgx.Tx, appID uuid.UUID) (string, error) {
	var st string
	err := tx.QueryRow(ctx, `SELECT status::text FROM applications WHERE id=$1 FOR UPDATE`, appID).Scan(&st)
	if err == pgx.ErrNoRows {
		return "", httpx.ErrNotFound
	}
	return st, err
}

// Transition moves an application to a new status, rejecting any change not in section 14.
func Transition(ctx context.Context, tx pgx.Tx, appID uuid.UUID, to string) (string, error) {
	from, err := LockStatus(ctx, tx, appID)
	if err != nil {
		return "", err
	}
	if !CanTransition(from, to) {
		return from, httpx.E(httpx.CodeTransition, fmt.Sprintf("The case cannot move from %s to %s.", from, to))
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET status=$2::app_status, updated_at=now() WHERE id=$1`, appID, to)
	if err != nil {
		return from, err
	}
	if t, ok := statusNotice[to]; ok {
		_ = NotifyParent(ctx, tx, appID, "STATUS_"+to, t.title, t.body, "/parent")
	}
	return from, nil
}

var statusNotice = map[string]struct{ title, body string }{
	"DOCS_SUBMITTED":      {"Documents submitted", "Your documents were submitted. Your social worker will review them."},
	"DOCS_CORRECTION":     {"Correction needed", "Your social worker asked you to upload a document again."},
	"HSR_IN_PROGRESS":     {"Documents verified", "All your documents are verified. The home study has started."},
	"WAITING":             {"Home study approved", "Your home study report is signed. You are now in the waiting list."},
	"REFERRED":            {"New referral", "A child's profile has been referred to you. You have 48 hours to decide."},
	"RESERVED":            {"Child reserved", "You reserved the child. Matching must finish within 30 days."},
	"MATCHED_FOSTER_CARE": {"Matching recorded", "The match was approved and pre-adoption foster care is recorded."},
	"DM_ORDER_PENDING":    {"Filed with the District Magistrate", "The adoption application was filed with the District Magistrate."},
	"ADOPTED_FOLLOWUP":    {"Adoption order received", "The adoption order is recorded. Four follow-up visits are scheduled."},
	"COMPLETED":           {"Case completed", "All follow-up visits are complete."},
	"LAPSED":              {"Registration lapsed", "Documents were not submitted within 30 days, so the registration lapsed."},
	"DEBARRED":            {"Blocked for 1 year", "No child was reserved in 3 referrals, so the application is blocked for 1 year."},
	"WITHDRAWN":           {"Application closed", "The application was closed."},
}

// NotifyParent notifies the parent who owns the application.
func NotifyParent(ctx context.Context, tx pgx.Tx, appID uuid.UUID, typ, title, body, link string) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications (user_id, type, title, body, link)
		SELECT parent_id, $2, $3, $4, $5 FROM applications WHERE id=$1`, appID, typ, title, body, link)
	return err
}

// NotifyWorker notifies the social worker assigned to the application.
func NotifyWorker(ctx context.Context, tx pgx.Tx, appID uuid.UUID, typ, title, body, link string) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications (user_id, type, title, body, link)
		SELECT social_worker_id, $2, $3, $4, $5 FROM applications WHERE id=$1 AND social_worker_id IS NOT NULL`, appID, typ, title, body, link)
	return err
}
