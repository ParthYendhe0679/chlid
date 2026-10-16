// Package audit writes the hash-chained audit log and verifies it (section 13.1).
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"childconnect/internal/httpx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// lockKey serialises inserts so two entries never share the same previous hash.
const lockKey = 7_424_242

type Entry struct {
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	TargetType string
	TargetID   string
	IP         string
	Meta       map[string]any
}

type Logger struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Logger { return &Logger{pool: pool} }

// FromUser fills actor fields from the request user (nil for anonymous actions).
func FromUser(u *httpx.User, action, targetType, targetID string, meta map[string]any) Entry {
	e := Entry{Action: action, TargetType: targetType, TargetID: targetID, Meta: meta}
	if u != nil {
		id := u.ID
		e.ActorID = &id
		e.ActorRole = string(u.Role)
		e.IP = u.IP
	}
	return e
}

// Log writes one entry in its own transaction.
func (l *Logger) Log(ctx context.Context, e Entry) error {
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := LogTx(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LogTx writes the entry inside the caller's transaction, so the action and its audit row commit together.
func LogTx(ctx context.Context, tx pgx.Tx, e Entry) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
		return err
	}
	prev := genesis
	err := tx.QueryRow(ctx, `SELECT hash FROM audit_logs ORDER BY id DESC LIMIT 1`).Scan(&prev)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if e.Meta == nil {
		e.Meta = map[string]any{}
	}
	meta, err := json.Marshal(e.Meta) // map keys are sorted, so this is canonical
	if err != nil {
		return err
	}
	meta = canonical(meta) // same normalisation the verifier applies
	created := time.Now().UTC().Truncate(time.Microsecond)
	actor := ""
	if e.ActorID != nil {
		actor = e.ActorID.String()
	}
	h := Hash(prev, actor, e.Action, e.TargetType, e.TargetID, created, meta)
	var role any
	if e.ActorRole != "" {
		role = e.ActorRole
	}
	var ip any
	if e.IP != "" {
		ip = e.IP
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_role, action, target_type, target_id, ip, meta, created_at, prev_hash, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, e.ActorID, role, e.Action, e.TargetType, e.TargetID, ip, meta, created, prev, h)
	return err
}

// Hash = sha256(prev_hash + actor_id + action + target_type + target_id + created_at + meta_json).
func Hash(prev, actor, action, targetType, targetID string, created time.Time, meta []byte) string {
	var b strings.Builder
	b.WriteString(prev)
	b.WriteString(actor)
	b.WriteString(action)
	b.WriteString(targetType)
	b.WriteString(targetID)
	b.WriteString(created.UTC().Format("2006-01-02T15:04:05.000000Z"))
	b.Write(meta)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func canonical(raw []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		m = map[string]any{}
	}
	out, _ := json.Marshal(m)
	return out
}

type VerifyResult struct {
	Valid       bool   `json:"valid"`
	RowsChecked int    `json:"rows_checked"`
	BrokenAtID  *int64 `json:"broken_at_id,omitempty"`
}

// Verify recalculates every hash from the first entry and reports the first broken row.
func (l *Logger) Verify(ctx context.Context) (VerifyResult, error) {
	rows, err := l.pool.Query(ctx, `SELECT id, COALESCE(actor_id::text,''), action, target_type, target_id, meta, created_at, prev_hash, hash
		FROM audit_logs ORDER BY id`)
	if err != nil {
		return VerifyResult{}, err
	}
	defer rows.Close()
	prev := genesis
	n := 0
	for rows.Next() {
		var id int64
		var actor, action, tt, tid, ph, h string
		var meta []byte
		var created time.Time
		if err := rows.Scan(&id, &actor, &action, &tt, &tid, &meta, &created, &ph, &h); err != nil {
			return VerifyResult{}, err
		}
		n++
		if ph != prev || Hash(ph, actor, action, tt, tid, created, canonical(meta)) != h {
			return VerifyResult{Valid: false, RowsChecked: n, BrokenAtID: &id}, nil
		}
		prev = h
	}
	return VerifyResult{Valid: true, RowsChecked: n}, rows.Err()
}
