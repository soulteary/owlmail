package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrMailboxUnavailable deliberately does not distinguish absent identities,
// pending mailboxes and inconsistent ownership to callers.
var ErrMailboxUnavailable = errors.New("personal mailbox unavailable")

// Merge is a durable, already-authorized transfer of a frozen pending mailbox.
// New deliveries have been redirected before the journal becomes visible.
type Merge struct{ ID, SourceID, TargetID string }

func personalMailbox(ctx context.Context, db *sql.DB, namespace, subject string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT m.id FROM identity i JOIN mailbox m ON m.id=i.personal_mailbox_id AND m.owner_identity_id=i.id WHERE i.namespace=? AND i.subject=? AND m.state='personal' AND m.merged_into_id IS NULL AND NOT EXISTS(SELECT 1 FROM merge_journal j WHERE j.source_id=m.id AND j.state='pending')`, namespace, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMailboxUnavailable
	}
	return id, err
}

// PersonalMailbox resolves a stable identity without creating or claiming data.
// Use WithPersonalMailbox for a local operation that needs serialization with merges.
func (s *Store) PersonalMailbox(ctx context.Context, namespace, subject string) (string, error) {
	if !validIdentity(namespace, subject) {
		return "", ErrMailboxUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return personalMailbox(ctx, s.db, namespace, subject)
}

// WithPersonalMailbox serializes local access with binding, SMTP submission and
// merge recovery. The callback must not call Store methods or perform network I/O.
// Lock order: Store.mu -> storage transaction lock -> message store lock.
func (s *Store) WithPersonalMailbox(ctx context.Context, namespace, subject string, fn func(string) error) error {
	if !validIdentity(namespace, subject) || fn == nil {
		return ErrMailboxUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := personalMailbox(ctx, s.db, namespace, subject)
	if err != nil {
		return err
	}
	return fn(id)
}

// PendingMerges returns a detached recovery snapshot. It never unfreezes a source.
func (s *Store) PendingMerges(ctx context.Context) ([]Merge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return pendingMerges(ctx, s.db)
}
func pendingMerges(ctx context.Context, db *sql.DB) ([]Merge, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,source_id,target_id FROM merge_journal WHERE state='pending' ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []Merge
	for rows.Next() {
		var item Merge
		if err = rows.Scan(&item.ID, &item.SourceID, &item.TargetID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// RunPendingMerges applies every durable transfer in order. The callback must be
// idempotent: a crash after file commit but before the SQLite completion commit
// will repeat it. Sources stay frozen until callback success AND control commit.
// The callback runs under Store.mu and must follow the documented lock order.
func (s *Store) RunPendingMerges(ctx context.Context, apply func(context.Context, Merge) error) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	if apply == nil {
		return errors.New("merge callback required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := pendingMerges(ctx, s.db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = validateMerge(ctx, s.db, item); err != nil {
			return err
		}
		if err = apply(ctx, item); err != nil {
			if errors.Is(err, ErrQuotaExceeded) {
				continue
			}
			return fmt.Errorf("apply mailbox merge %s: %w", item.ID, err)
		}
		// No route or binding can interleave with this transition. SQLite and files
		// are separate durability domains; a failure here safely leaves a retry.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE mailbox SET state='merged',merged_into_id=? WHERE id=? AND state='pending' AND owner_identity_id IS NULL`, item.TargetID, item.SourceID); err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE merge_journal SET state='complete' WHERE id=? AND state='pending'`, item.ID)
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

func validateMerge(ctx context.Context, db *sql.DB, item Merge) error {
	return validateMergeVersion(ctx, db, item, true)
}
func validateMergeVersion(ctx context.Context, db *sql.DB, item Merge, hasRetention bool) error {
	var valid int
	query := `SELECT count(*) FROM merge_journal j JOIN mailbox src ON src.id=j.source_id JOIN mailbox dst ON dst.id=j.target_id JOIN identity i ON i.id=dst.owner_identity_id AND i.personal_mailbox_id=dst.id WHERE j.id=? AND j.source_id=? AND j.target_id=? AND j.state='pending' AND src.state='pending' AND src.owner_identity_id IS NULL AND src.merged_into_id IS NULL AND src.retired_at IS NULL AND dst.state='personal' AND dst.merged_into_id IS NULL AND NOT EXISTS(SELECT 1 FROM recipient_route r WHERE r.mailbox_id=src.id) AND EXISTS(SELECT 1 FROM binding_audit a JOIN recipient_route r ON r.id=a.route_id WHERE a.source_id=src.id AND a.target_id=dst.id AND a.identity_id=i.id AND a.evidence='trusted_login' AND r.namespace=i.namespace AND r.mailbox_id=dst.id)`
	if !hasRetention {
		query = strings.ReplaceAll(query, " AND src.retired_at IS NULL", "")
	}
	err := db.QueryRowContext(ctx, query, item.ID, item.SourceID, item.TargetID).Scan(&valid)
	if err != nil {
		return err
	}
	if valid != 1 {
		return errors.New("invalid mailbox merge ownership graph")
	}
	return nil
}

// RunMergeBatch advances one frozen journal in ID order. targetID scopes
// request-driven work to its already-authorized personal mailbox; an empty target
// is for the maintenance worker. afterID is a fairness cursor, never authority.
// The returned ID advances even on a recoverable callback failure, so one broken
// mailbox cannot starve others. An empty ID means the caller should wrap its cursor.
// The callback reports complete only after every source message is durable.
func (s *Store) RunMergeBatch(ctx context.Context, targetID, afterID string, apply func(context.Context, Merge) (bool, error)) (string, error) {
	if s.readOnlySchema > 0 {
		return "", ErrReadOnly
	}
	if apply == nil {
		return "", errors.New("merge callback required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `SELECT id,source_id,target_id FROM merge_journal WHERE state='pending' AND id>?`
	args := []any{afterID}
	if targetID != "" {
		query += ` AND target_id=?`
		args = append(args, targetID)
	}
	query += ` ORDER BY id LIMIT 1`
	var item Merge
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&item.ID, &item.SourceID, &item.TargetID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err = validateMerge(ctx, s.db, item); err != nil {
		return item.ID, err
	}
	complete, err := apply(ctx, item)
	if errors.Is(err, ErrQuotaExceeded) {
		return item.ID, nil
	}
	if err != nil {
		return item.ID, fmt.Errorf("apply mailbox merge %s: %w", item.ID, err)
	}
	if !complete {
		return item.ID, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return item.ID, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE mailbox SET state='merged',merged_into_id=? WHERE id=? AND state='pending' AND owner_identity_id IS NULL`, item.TargetID, item.SourceID); err != nil {
		return item.ID, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE merge_journal SET state='complete' WHERE id=? AND state='pending'`, item.ID); err != nil {
		return item.ID, err
	}
	return item.ID, tx.Commit()
}

// validateControlState rejects inconsistent ownership even when SQLite's own
// foreign-key and page checks pass. It runs on reopen before listeners start.
func validateControlState(ctx context.Context, db *sql.DB) error {
	return validateControlStateVersion(ctx, db, true)
}
func validateControlStateVersion(ctx context.Context, db *sql.DB, hasRetention bool) error {
	var invalid int
	checks := []string{
		`SELECT count(*) FROM identity i LEFT JOIN mailbox m ON m.id=i.personal_mailbox_id WHERE m.id IS NULL OR m.state<>'personal' OR m.owner_identity_id<>i.id OR m.owner_identity_id IS NULL`,
		`SELECT count(*) FROM mailbox m LEFT JOIN identity i ON i.id=m.owner_identity_id WHERE m.state='personal' AND (i.id IS NULL OR i.personal_mailbox_id<>m.id)`,
		`SELECT count(*) FROM recipient_route r JOIN mailbox m ON m.id=r.mailbox_id LEFT JOIN identity i ON i.id=m.owner_identity_id WHERE (r.state='pending' AND m.state<>'pending') OR (r.state='bound' AND (m.state<>'personal' OR i.namespace<>r.namespace OR i.id IS NULL)) OR m.state='merged'`,
		`SELECT count(*) FROM merge_journal j JOIN mailbox src ON src.id=j.source_id JOIN mailbox dst ON dst.id=j.target_id WHERE dst.state<>'personal' OR (j.state='complete' AND (src.state<>'merged' OR src.merged_into_id<>dst.id OR src.merged_into_id IS NULL)) OR (j.state='pending' AND (src.state<>'pending' OR src.owner_identity_id IS NOT NULL OR src.merged_into_id IS NOT NULL))`,
		`SELECT count(*) FROM mailbox m WHERE m.state='merged' AND NOT EXISTS(SELECT 1 FROM merge_journal j WHERE j.source_id=m.id AND j.target_id=m.merged_into_id AND j.state='complete')`,
	}
	for _, q := range checks {
		if err := db.QueryRowContext(ctx, q).Scan(&invalid); err != nil {
			return err
		}
		if invalid != 0 {
			return errors.New("control database ownership graph check failed")
		}
	}
	items, err := pendingMerges(ctx, db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = validateMergeVersion(ctx, db, item, hasRetention); err != nil {
			return err
		}
	}
	if hasRetention {
		return validateRetentionState(ctx, db)
	}
	return nil
}
