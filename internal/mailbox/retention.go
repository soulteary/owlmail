package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrRetentionPending = errors.New("pending mailbox cleanup requires recovery")

// PendingRetention freezes one pending mailbox while old messages are removed.
// Cutoff is applied to durable receivedAt, not the MIME Date header.
type PendingRetention struct {
	ID, MailboxID       string
	Cutoff, EmptyBefore time.Time
}
type PendingRetentionResult struct{ Deleted, Remaining int }

func upgradeRetentionSchema(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE mailbox ADD COLUMN retired_at INTEGER;
 CREATE TABLE retention_journal (
 id TEXT PRIMARY KEY,
 mailbox_id TEXT NOT NULL REFERENCES mailbox(id),
 state TEXT NOT NULL CHECK(state IN ('pending','complete')),
 cutoff INTEGER NOT NULL,
 empty_before INTEGER NOT NULL,
 created_at INTEGER NOT NULL,
 completed_at INTEGER,
 retired INTEGER NOT NULL DEFAULT 0 CHECK(retired IN (0,1))
 );
 CREATE UNIQUE INDEX retention_active_mailbox ON retention_journal(mailbox_id) WHERE state='pending';
 CREATE TABLE retention_audit (
 id TEXT PRIMARY KEY,
 journal_id TEXT NOT NULL UNIQUE REFERENCES retention_journal(id),
 mailbox_id TEXT NOT NULL REFERENCES mailbox(id),
 cutoff INTEGER NOT NULL,
 empty_before INTEGER NOT NULL,
 completed_at INTEGER NOT NULL,
 retired INTEGER NOT NULL CHECK(retired IN (0,1))
 );`)
	return err
}

func readPendingRetention(ctx context.Context, db *sql.DB) ([]PendingRetention, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,mailbox_id,cutoff,empty_before FROM retention_journal WHERE state='pending' ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []PendingRetention
	for rows.Next() {
		var item PendingRetention
		var cutoff, empty int64
		if err = rows.Scan(&item.ID, &item.MailboxID, &cutoff, &empty); err != nil {
			return nil, err
		}
		item.Cutoff = time.Unix(0, cutoff).UTC()
		item.EmptyBefore = time.Unix(0, empty).UTC()
		items = append(items, item)
	}
	return items, rows.Err()
}

// RunPendingRetention recovers durable deletions before claims or SMTP writes
// are opened. A callback must be idempotent and must not call Store methods.
func (s *Store) RunPendingRetention(ctx context.Context, apply func(context.Context, PendingRetention) (PendingRetentionResult, error)) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	if apply == nil {
		return errors.New("retention callback required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runPendingRetentionLocked(ctx, apply)
}
func (s *Store) runPendingRetentionLocked(ctx context.Context, apply func(context.Context, PendingRetention) (PendingRetentionResult, error)) error {
	items, err := readPendingRetention(ctx, s.db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return err
		}
		var valid int
		err = s.db.QueryRowContext(ctx, `SELECT count(*) FROM mailbox m WHERE m.id=? AND m.state='pending' AND m.owner_identity_id IS NULL AND m.merged_into_id IS NULL AND m.retired_at IS NULL AND NOT EXISTS(SELECT 1 FROM merge_journal j WHERE j.source_id=m.id AND j.state='pending')`, item.MailboxID).Scan(&valid)
		if err != nil {
			return err
		}
		if valid != 1 {
			return errors.New("invalid pending retention ownership graph")
		}
		result, err := apply(ctx, item)
		if err != nil {
			return fmt.Errorf("retain pending mailbox %s: %w", item.MailboxID, err)
		}
		if result.Deleted < 0 || result.Remaining < 0 {
			return errors.New("invalid retention result")
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var created int64
		err = tx.QueryRowContext(ctx, `SELECT created_at FROM mailbox WHERE id=?`, item.MailboxID).Scan(&created)
		retired := 0
		if err == nil && result.Remaining == 0 && created <= item.EmptyBefore.Unix() {
			// Pending routes may be recreated with a new mailbox on the next RCPT.
			// Blocked routes remain as deny rules and must never be silently unblocked.
			_, err = tx.ExecContext(ctx, `DELETE FROM recipient_route WHERE mailbox_id=? AND state='pending' AND NOT EXISTS(SELECT 1 FROM binding_audit a WHERE a.route_id=recipient_route.id)`, item.MailboxID)
			if err == nil {
				// A pending route referenced by historical audit cannot be deleted, so
				// retain it as a blocked deny rule rather than allowing historical rebinding.
				_, err = tx.ExecContext(ctx, `UPDATE recipient_route SET state='blocked',revision=revision+1 WHERE mailbox_id=? AND state='pending'`, item.MailboxID)
			}
			if err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE mailbox SET retired_at=? WHERE id=?`, time.Now().Unix(), item.MailboxID)
				retired = 1
			}
		}
		completed := time.Now().Unix()
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE retention_journal SET state='complete',completed_at=?,retired=? WHERE id=?`, completed, retired, item.ID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO retention_audit(id,journal_id,mailbox_id,cutoff,empty_before,completed_at,retired) VALUES(?,?,?,?,?,?,?)`, uuid.NewString(), item.ID, item.MailboxID, item.Cutoff.UnixNano(), item.EmptyBefore.UnixNano(), completed, retired)
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// CleanupPending first completes interrupted cleanup, then schedules a new
// receivedAt sweep. Personal mailboxes and merge sources are never candidates.
// Empty old mailboxes lose pending routes; their identity-free tombstone is
// retained for audit references, and new deliveries create a fresh pending box.
func (s *Store) CleanupPending(ctx context.Context, now time.Time, retention, emptyRetention time.Duration, apply func(context.Context, PendingRetention) (PendingRetentionResult, error)) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	if now.IsZero() || retention <= 0 || emptyRetention <= 0 || apply == nil {
		return errors.New("positive pending retention limits required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.runPendingRetentionLocked(ctx, apply); err != nil {
		return err
	}
	cutoff, emptyBefore := now.Add(-retention), now.Add(-emptyRetention)
	candidateBefore := cutoff
	if emptyBefore.After(candidateBefore) {
		candidateBefore = emptyBefore
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id FROM mailbox m WHERE m.state='pending' AND m.owner_identity_id IS NULL AND m.merged_into_id IS NULL AND m.retired_at IS NULL AND m.created_at<=? AND NOT EXISTS(SELECT 1 FROM merge_journal j WHERE j.source_id=m.id AND j.state='pending') AND NOT EXISTS(SELECT 1 FROM retention_journal j WHERE j.mailbox_id=m.id AND j.cutoff>=?) ORDER BY m.created_at,m.id`, candidateBefore.Unix(), cutoff.UnixNano())
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return err
		}
		// Planning commits before any deletion. Claims/ingress detect this journal
		// on recovery and cannot reactivate a partially cleaned source.
		if _, err = s.db.ExecContext(ctx, `INSERT INTO retention_journal(id,mailbox_id,state,cutoff,empty_before,created_at) VALUES(?,?,'pending',?,?,?)`, uuid.NewString(), id, cutoff.UnixNano(), emptyBefore.UnixNano(), now.Unix()); err != nil {
			return err
		}
		// Plan one mailbox at a time. A disk failure must not freeze every
		// unrelated pending recipient that has not begun cleanup.
		if err = s.runPendingRetentionLocked(ctx, apply); err != nil {
			return err
		}
	}
	return nil
}

func validateRetentionState(ctx context.Context, db *sql.DB) error {
	var invalid int
	for _, query := range []string{
		`SELECT count(*) FROM mailbox m WHERE m.retired_at IS NOT NULL AND (m.state<>'pending' OR m.owner_identity_id IS NOT NULL OR m.merged_into_id IS NOT NULL OR EXISTS(SELECT 1 FROM recipient_route r WHERE r.mailbox_id=m.id AND r.state<>'blocked'))`,
		`SELECT count(*) FROM retention_journal j JOIN mailbox m ON m.id=j.mailbox_id WHERE j.state='pending' AND (m.state<>'pending' OR m.owner_identity_id IS NOT NULL OR m.retired_at IS NOT NULL OR EXISTS(SELECT 1 FROM merge_journal g WHERE g.source_id=m.id AND g.state='pending'))`,
	} {
		if err := db.QueryRowContext(ctx, query).Scan(&invalid); err != nil {
			return err
		}
		if invalid > 0 {
			return errors.New("control database retention graph check failed")
		}
	}
	return nil
}
