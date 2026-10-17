package workflow

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const day = 24 * time.Hour

// StepLimit is the legal time limit for each step (section 3.3).
var StepLimit = map[string]time.Duration{
	"DOC_UPLOAD_30D":  30 * day,
	"HSR_60D":         60 * day,
	"CSR_MER_10D":     10 * day,
	"RESERVE_48H":     48 * time.Hour,
	"MATCHING_30D":    30 * day,
	"FOSTER_CARE_10D": 10 * day,
	"DM_FILING_10D":   10 * day,
	"DM_ORDER_60D":    60 * day,
}

// Querier is satisfied by both the pool and a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type Deadline struct {
	Step        string   `json:"step"`
	DueAt       string   `json:"due_at"`
	DaysLeft    int      `json:"days_left"`
	RiskScore   *float64 `json:"risk_score"`
	RiskReasons []string `json:"risk_reasons"`
}

// DaysLeft counts whole days to the deadline; negative means overdue.
func DaysLeft(due, now time.Time) int {
	d := due.Sub(now).Hours() / 24
	if d >= 0 {
		return int(math.Ceil(d - 1e-9))
	}
	return -int(math.Ceil(-d - 1e-9))
}

// AddDeadline creates an OPEN deadline. due defaults to start + the legal limit for the step.
func AddDeadline(ctx context.Context, tx pgx.Tx, appID, childID, agencyID *uuid.UUID, step string, start time.Time, due *time.Time) error {
	d := start.Add(StepLimit[step])
	if due != nil {
		d = *due
	}
	_, err := tx.Exec(ctx, `INSERT INTO case_deadlines (application_id, child_id, agency_id, step, started_at, due_at)
		VALUES ($1,$2,$3,$4::deadline_step,$5,$6)`, appID, childID, agencyID, step, start, d)
	return err
}

// CompleteSteps marks open deadlines of the given steps as DONE for an application.
func CompleteSteps(ctx context.Context, tx pgx.Tx, appID uuid.UUID, steps ...string) error {
	_, err := tx.Exec(ctx, `UPDATE case_deadlines SET state='DONE', completed_at=now()
		WHERE application_id=$1 AND step::text = ANY($2) AND state IN ('OPEN','OVERDUE')`, appID, steps)
	return err
}

// CancelSteps cancels open deadlines (for example the 48-hour window after a decision is no longer needed).
func CancelSteps(ctx context.Context, tx pgx.Tx, appID uuid.UUID, steps ...string) error {
	_, err := tx.Exec(ctx, `UPDATE case_deadlines SET state='CANCELLED', completed_at=now()
		WHERE application_id=$1 AND step::text = ANY($2) AND state IN ('OPEN','OVERDUE')`, appID, steps)
	return err
}

// Current returns the most urgent open deadline of an application, or nil.
func Current(ctx context.Context, q Querier, appID uuid.UUID, now time.Time) (*Deadline, error) {
	var step string
	var due time.Time
	var score *float64
	var reasons []byte
	err := q.QueryRow(ctx, `SELECT step::text, due_at, risk_score::float8, risk_reasons FROM case_deadlines
		WHERE application_id=$1 AND state IN ('OPEN','OVERDUE') ORDER BY due_at LIMIT 1`, appID).Scan(&step, &due, &score, &reasons)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return BuildDeadline(step, due, score, reasons, now), nil
}

func BuildDeadline(step string, due time.Time, score *float64, reasons []byte, now time.Time) *Deadline {
	d := &Deadline{Step: step, DueAt: due.UTC().Format(time.RFC3339), DaysLeft: DaysLeft(due, now), RiskScore: score, RiskReasons: []string{}}
	_ = json.Unmarshal(reasons, &d.RiskReasons)
	if d.RiskReasons == nil {
		d.RiskReasons = []string{}
	}
	return d
}
