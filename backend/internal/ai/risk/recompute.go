package risk

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recompute marks overdue deadlines and recalculates the risk score of every open deadline.
// It returns the number of deadlines updated.
func (m *Model) Recompute(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	if _, err := pool.Exec(ctx, `UPDATE case_deadlines SET state='OVERDUE' WHERE state='OPEN' AND due_at < now()`); err != nil {
		return 0, err
	}
	// Agency statistics: open cases, staff, share of past deadlines missed.
	type agencyStat struct {
		open, staff, late float64
	}
	stats := map[uuid.UUID]agencyStat{}
	rows, err := pool.Query(ctx, `
		SELECT a.id, a.staff_count,
		  (SELECT count(*) FROM applications ap WHERE ap.agency_id=a.id AND ap.status NOT IN ('COMPLETED','LAPSED','DEBARRED','WITHDRAWN')),
		  COALESCE((SELECT avg(CASE WHEN (d.completed_at IS NOT NULL AND d.completed_at > d.due_at) OR (d.completed_at IS NULL AND d.state='OVERDUE') THEN 1.0 ELSE 0.0 END)
		            FROM case_deadlines d WHERE d.agency_id=a.id AND (d.state IN ('DONE','OVERDUE'))), 0.2)
		FROM agencies a`)
	if err != nil {
		return 0, err
	}
	var totalLoad float64
	for rows.Next() {
		var id uuid.UUID
		var st agencyStat
		var staff int
		var open int64
		if err := rows.Scan(&id, &staff, &open, &st.late); err != nil {
			rows.Close()
			return 0, err
		}
		st.staff, st.open = float64(max(staff, 1)), float64(open)
		stats[id] = st
		totalLoad += st.open / st.staff
	}
	rows.Close()
	avgLoad := 1.0
	if len(stats) > 0 && totalLoad > 0 {
		avgLoad = totalLoad / float64(len(stats))
	}

	rows, err = pool.Query(ctx, `
		SELECT d.id, d.step::text, d.started_at, d.due_at, d.agency_id,
		  COALESCE((SELECT count(*) FROM documents doc WHERE doc.application_id=d.application_id AND (NOT doc.is_current OR doc.status='REUPLOAD_REQUESTED')), 0),
		  COALESCE((SELECT avg(EXTRACT(EPOCH FROM (r.created_at - m.created_at))/86400)
		            FROM messages m JOIN users su ON su.id=m.sender_id AND su.role='AGENCY'
		            JOIN LATERAL (SELECT m2.created_at FROM messages m2 JOIN users pu ON pu.id=m2.sender_id AND pu.role='PARENT'
		                          WHERE m2.application_id=m.application_id AND m2.created_at > m.created_at ORDER BY m2.created_at LIMIT 1) r ON true
		            WHERE m.application_id=d.application_id), 1.5)
		FROM case_deadlines d WHERE d.state IN ('OPEN','OVERDUE')`)
	if err != nil {
		return 0, err
	}
	type upd struct {
		id      uuid.UUID
		score   float64
		reasons []byte
	}
	var ups []upd
	now := time.Now()
	for rows.Next() {
		var id uuid.UUID
		var step string
		var started, due time.Time
		var agencyID *uuid.UUID
		var reup int64
		var reply float64
		if err := rows.Scan(&id, &step, &started, &due, &agencyID, &reup, &reply); err != nil {
			rows.Close()
			return 0, err
		}
		allowed := due.Sub(started).Hours()
		ratio := 0.0
		if allowed > 0 {
			ratio = now.Sub(started).Hours() / allowed
		}
		f := Features{DaysElapsedRatio: clamp(ratio, 0, 1.5), DocsReupload: float64(reup), Step: step, ParentReplyDelay: reply,
			AgencyOpenCases: 10, AgencyStaffCount: 3, AgencyPastLate: 0.2, AvgLoadPerStaff: avgLoad}
		if agencyID != nil {
			if st, ok := stats[*agencyID]; ok {
				f.AgencyOpenCases, f.AgencyStaffCount, f.AgencyPastLate = st.open, st.staff, st.late
			}
		}
		p, reasons := m.Predict(f)
		if now.After(due) {
			reasons = append([]string{"The legal deadline has already passed"}, reasons...)
			if len(reasons) > 3 {
				reasons = reasons[:3]
			}
		}
		if reasons == nil {
			reasons = []string{}
		}
		b, _ := json.Marshal(reasons)
		ups = append(ups, upd{id, p, b})
	}
	rows.Close()
	b := &pgx.Batch{}
	for _, u := range ups {
		b.Queue(`UPDATE case_deadlines SET risk_score=$2, risk_reasons=$3, risk_updated_at=now() WHERE id=$1`, u.id, u.score, u.reasons)
	}
	if err := pool.SendBatch(ctx, b).Close(); err != nil {
		return 0, err
	}
	return len(ups), nil
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
