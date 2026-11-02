package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrAttemptQuota means the fixed-minute authentication/recipient attempt
// budget is exhausted. The budget is durable even for abandoned transactions.
var ErrAttemptQuota = errors.New("SMTP attempt quota exhausted")

// ErrQuotaExceeded defers a merge or rejects ingress without losing source data.
var ErrQuotaExceeded = errors.New("mailbox message or disk quota exhausted")

func upgradeQuotaSchema(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS smtp_attempt_quota (
 kind TEXT NOT NULL CHECK(kind IN ('auth','rcpt')),
 namespace TEXT NOT NULL,
 account_id TEXT NOT NULL,
 window_start INTEGER NOT NULL,
 attempts INTEGER NOT NULL CHECK(attempts>0),
 PRIMARY KEY(kind,namespace,account_id,window_start)
 );
 CREATE INDEX IF NOT EXISTS smtp_attempt_quota_window ON smtp_attempt_quota(window_start);
 CREATE TABLE IF NOT EXISTS smtp_quota_clock (
 id INTEGER PRIMARY KEY CHECK(id=1),
 last_window INTEGER NOT NULL
 );`)
	return err
}

// ConsumeAttempt atomically reserves one attempt. The minute watermark is
// durable and nondecreasing, so restarting or moving the wall clock backwards
// cannot reopen an already exhausted budget. Authentication uses the one
// global key (empty namespace/account), never attacker-controlled usernames.
// RCPT uses a validated authenticated account identity.
func (s *Store) ConsumeAttempt(ctx context.Context, kind, namespace, account string, limit int, at time.Time) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	if limit <= 0 || at.IsZero() || at.Unix() < 0 {
		return errors.New("invalid SMTP attempt quota")
	}
	switch kind {
	case "auth":
		if namespace != "" || account != "" {
			return errors.New("authentication quota must be global")
		}
	case "rcpt":
		if !validIdentity(namespace, account) {
			return errors.New("invalid SMTP quota identity")
		}
	default:
		return errors.New("invalid SMTP quota kind")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	window := at.Unix() / 60
	if _, err = tx.ExecContext(ctx, `INSERT INTO smtp_quota_clock(id,last_window) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET last_window=max(last_window,excluded.last_window)`, window); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT last_window FROM smtp_quota_clock WHERE id=1`).Scan(&window); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO smtp_attempt_quota(kind,namespace,account_id,window_start,attempts) VALUES(?,?,?,?,1) ON CONFLICT(kind,namespace,account_id,window_start) DO UPDATE SET attempts=attempts+1 WHERE attempts<?`, kind, namespace, account, window, limit)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// Bound database growth while preserving the current watermark window.
	// Older windows can never become active again because the clock is clamped.
	if _, err = tx.ExecContext(ctx, `DELETE FROM smtp_attempt_quota WHERE window_start<?`, window-1440); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if affected != 1 {
		return ErrAttemptQuota
	}
	return nil
}
