package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var ErrReadOnly = errors.New("control store is read-only")

// OpenReadOnly opens an offline control snapshot (schema v1 or v2) without
// chmod, schema migration, or any write access. It does not make live email
// files consistent; migration/export callers must arrange a stopped writer or
// a complete offline snapshot before using it.
func OpenReadOnly(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("control database path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("control database must be regular")
	}
	uri := url.URL{Scheme: "file", Path: path}
	q := uri.Query()
	q.Set("mode", "ro")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	if _, err = db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version != 1 && version != schemaVersion {
		return fail(fmt.Errorf("read-only control schema %d requires explicit upgrade to %d", version, schemaVersion))
	}
	var check string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return fail(errors.New("control database integrity check failed"))
	}
	if err = checkForeignKeys(db); err != nil {
		return fail(err)
	}
	if err = validateControlStateVersion(context.Background(), db, version >= 2); err != nil {
		return fail(err)
	}
	return &Store{db: db, readOnlySchema: version}, nil
}

// CheckPersonalMailboxID validates an operator-selected export target. Pending,
// retired, merged and incompletely claimed mailboxes cannot be exported.
func (s *Store) CheckPersonalMailboxID(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return ErrMailboxUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var valid int
	query := `SELECT count(*) FROM mailbox m JOIN identity i ON i.id=m.owner_identity_id AND i.personal_mailbox_id=m.id WHERE m.id=? AND m.state='personal' AND m.retired_at IS NULL AND m.merged_into_id IS NULL AND NOT EXISTS(SELECT 1 FROM merge_journal j WHERE j.state='pending' AND (j.source_id=m.id OR j.target_id=m.id)) AND NOT EXISTS(SELECT 1 FROM retention_journal j WHERE j.state='pending' AND j.mailbox_id=m.id)`
	if s.readOnlySchema == 1 {
		query = strings.ReplaceAll(query, " AND m.retired_at IS NULL", "")
		query = strings.ReplaceAll(query, " AND NOT EXISTS(SELECT 1 FROM retention_journal j WHERE j.state='pending' AND j.mailbox_id=m.id)", "")
	}
	err = s.db.QueryRowContext(ctx, query, id).Scan(&valid)
	if err != nil {
		return err
	}
	if valid != 1 {
		return ErrMailboxUnavailable
	}
	return nil
}
