// Package jobs runs the background job every 15 minutes inside the server (section 12.2): it expires
// referrals, lapses applications, marks deadlines overdue, recalculates risk scores and sends reminders.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"childconnect/internal/ai"
	"childconnect/internal/ai/risk"
	"childconnect/internal/audit"
	"childconnect/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Runner struct {
	Pool  *pgxpool.Pool
	Risk  *risk.Model
	AI    *ai.Service
	Every time.Duration
}

func (j *Runner) Start(ctx context.Context) {
	go func() {
		j.RunOnce(ctx)
		t := time.NewTicker(j.Every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				j.RunOnce(ctx)
			}
		}
	}()
}

func (j *Runner) RunOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	start := time.Now()
	exp, err := j.expireReferrals(ctx)
	if err != nil {
		slog.Error("job: expire referrals", "err", err)
	}
	lapsed, err := j.lapseApplications(ctx)
	if err != nil {
		slog.Error("job: lapse applications", "err", err)
	}
	if err := j.reminders(ctx); err != nil {
		slog.Error("job: reminders", "err", err)
	}
	if err := j.notifyOverdue(ctx); err != nil {
		slog.Error("job: overdue", "err", err)
	}
	n, err := j.Risk.Recompute(ctx, j.Pool)
	if err != nil {
		slog.Error("job: risk", "err", err)
	} else {
		_, _ = j.AI.RecordRun(ctx, "RISK_SCORE", nil, "", nil, j.Risk.Name+"@"+j.Risk.Version, int(time.Since(start).Milliseconds()), map[string]any{"updated": n, "trigger": "background_job"}, nil)
	}
	slog.Info("background job done", "expired_referrals", exp, "lapsed", lapsed, "risk_scored", n, "ms", time.Since(start).Milliseconds())
}

// expireReferrals: after 48 hours without a decision the referral counts as used (section 14, rules 10 and 11).
func (j *Runner) expireReferrals(ctx context.Context) (int, error) {
	rows, err := j.Pool.Query(ctx, `SELECT id FROM referrals WHERE decision='PENDING' AND expires_at < now()`)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		err := pgx.BeginFunc(ctx, j.Pool, func(tx pgx.Tx) error {
			var appID, childID uuid.UUID
			var dec string
			if err := tx.QueryRow(ctx, `SELECT application_id, child_id, decision FROM referrals WHERE id=$1 FOR UPDATE`, id).Scan(&appID, &childID, &dec); err != nil {
				return err
			}
			if dec != "PENDING" {
				return nil
			}
			if _, err := tx.Exec(ctx, `UPDATE referrals SET decision='EXPIRED', decided_at=now() WHERE id=$1`, id); err != nil {
				return err
			}
			var used int
			if err := tx.QueryRow(ctx, `UPDATE applications SET referrals_used=LEAST(referrals_used+1,3) WHERE id=$1 RETURNING referrals_used`, appID).Scan(&used); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE children SET status='READY' WHERE id=$1 AND status='REFERRED'`, childID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE case_deadlines SET state='DONE', completed_at=now() WHERE application_id=$1 AND step='RESERVE_48H' AND state IN ('OPEN','OVERDUE')`, appID); err != nil {
				return err
			}
			to := "WAITING"
			if used >= 3 {
				to = "DEBARRED"
			}
			if _, err := workflow.Transition(ctx, tx, appID, to); err != nil {
				return err
			}
			if to == "DEBARRED" {
				if _, err := tx.Exec(ctx, `UPDATE applications SET debarred_until=(now() + interval '1 year')::date WHERE id=$1`, appID); err != nil {
					return err
				}
			}
			_ = workflow.NotifyWorker(ctx, tx, appID, "REFERRAL_EXPIRED", "Referral expired", "The 48-hour decision window ended without a decision.", "/agency/cases/"+appID.String())
			return audit.LogTx(ctx, tx, audit.Entry{Action: "SYSTEM_REFERRAL_EXPIRED", TargetType: "referral", TargetID: id.String(), Meta: map[string]any{"new_status": to}})
		})
		if err != nil {
			slog.Error("expire referral", "id", id, "err", err)
		}
	}
	return len(ids), nil
}

// lapseApplications: REGISTERED for more than 30 days without submitting documents (section 14, rule 2).
func (j *Runner) lapseApplications(ctx context.Context) (int, error) {
	rows, err := j.Pool.Query(ctx, `SELECT a.id FROM applications a JOIN case_deadlines d ON d.application_id=a.id AND d.step='DOC_UPLOAD_30D'
		WHERE a.status='REGISTERED' AND d.state IN ('OPEN','OVERDUE') AND d.due_at < now()`)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		err := pgx.BeginFunc(ctx, j.Pool, func(tx pgx.Tx) error {
			if _, err := workflow.Transition(ctx, tx, id, "LAPSED"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE case_deadlines SET state='DONE', completed_at=now() WHERE application_id=$1 AND step='DOC_UPLOAD_30D' AND state IN ('OPEN','OVERDUE')`, id); err != nil {
				return err
			}
			return audit.LogTx(ctx, tx, audit.Entry{Action: "SYSTEM_APPLICATION_LAPSED", TargetType: "application", TargetID: id.String()})
		})
		if err != nil {
			slog.Error("lapse application", "id", id, "err", err)
		}
	}
	return len(ids), nil
}

// reminders: day 20, 25 and 28 of the 30-day document period (section 9).
func (j *Runner) reminders(ctx context.Context) error {
	for _, day := range []int{20, 25, 28} {
		typ := "DOC_REMINDER_" + itoa(day)
		_, err := j.Pool.Exec(ctx, `INSERT INTO notifications (user_id, type, title, body, link)
			SELECT a.parent_id, $1, 'Document deadline reminder', $3, '/parent/documents'
			FROM applications a JOIN case_deadlines d ON d.application_id=a.id AND d.step='DOC_UPLOAD_30D' AND d.state='OPEN'
			WHERE a.status IN ('REGISTERED','DOCS_CORRECTION') AND now() >= d.started_at + make_interval(days => $2::int)
			  AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.user_id=a.parent_id AND n.type=$1)`,
			typ, day, "Day "+itoa(day)+" of 30: please upload and submit your documents before the deadline.")
		if err != nil {
			return err
		}
	}
	return nil
}

// notifyOverdue marks deadlines overdue and tells the assigned social worker once.
func (j *Runner) notifyOverdue(ctx context.Context) error {
	_, err := j.Pool.Exec(ctx, `WITH changed AS (
			UPDATE case_deadlines SET state='OVERDUE' WHERE state='OPEN' AND due_at < now() RETURNING application_id, step)
		INSERT INTO notifications (user_id, type, title, body, link)
		SELECT a.social_worker_id, 'DEADLINE_OVERDUE', 'Legal deadline overdue', 'The ' || c.step::text || ' deadline has passed for a case.', '/agency/cases/' || a.id
		FROM changed c JOIN applications a ON a.id=c.application_id WHERE a.social_worker_id IS NOT NULL`)
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// ReembedKnowledge re-embeds chunks made by the offline embedder once a Gemini key is configured,
// so every chunk is searched with the same model.
func ReembedKnowledge(ctx context.Context, pool *pgxpool.Pool, svc *ai.Service) {
	if !svc.Enabled() {
		return
	}
	rows, err := pool.Query(ctx, `SELECT id, source_title, citation, content FROM knowledge_chunks WHERE embed_model <> $1`, svc.EmbedModelName())
	if err != nil {
		slog.Error("re-embed", "err", err)
		return
	}
	var ids []uuid.UUID
	var texts []string
	for rows.Next() {
		var id uuid.UUID
		var title, cite, content string
		if rows.Scan(&id, &title, &cite, &content) == nil {
			ids = append(ids, id)
			texts = append(texts, title+", "+cite+": "+content)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return
	}
	vecs, model, err := svc.Embed(ctx, texts, "RETRIEVAL_DOCUMENT")
	if err != nil {
		slog.Warn("re-embed with Gemini failed; keeping offline embeddings", "err", err)
		return
	}
	b := &pgx.Batch{}
	for i, id := range ids {
		b.Queue(`UPDATE knowledge_chunks SET embedding=$2::vector, embed_model=$3 WHERE id=$1`, id, ai.VectorLiteral(vecs[i]), model)
	}
	if err := pool.SendBatch(ctx, b).Close(); err != nil {
		slog.Error("re-embed update", "err", err)
		return
	}
	slog.Info("knowledge re-embedded", "chunks", len(ids), "model", model)
}
