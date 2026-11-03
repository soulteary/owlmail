package mailbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrConflict   = errors.New("recipient is already bound to another identity")
	ErrBlocked    = errors.New("recipient route is blocked")
	ErrLimit      = errors.New("pending mailbox limit reached")
	ErrUnverified = errors.New("verified email evidence required")
)

const schemaVersion = 2
const schema = `
CREATE TABLE mailbox (
 id TEXT PRIMARY KEY,
 state TEXT NOT NULL CHECK(state IN ('pending','personal','merged')),
 owner_identity_id TEXT UNIQUE REFERENCES identity(id) DEFERRABLE INITIALLY DEFERRED,
 merged_into_id TEXT REFERENCES mailbox(id),
 created_at INTEGER NOT NULL,
 claimed_at INTEGER,
 CHECK((state='pending' AND owner_identity_id IS NULL AND merged_into_id IS NULL)
 OR (state='personal' AND owner_identity_id IS NOT NULL AND merged_into_id IS NULL)
 OR (state='merged' AND owner_identity_id IS NULL AND merged_into_id IS NOT NULL AND merged_into_id<>id))
);
CREATE TABLE identity (
 id TEXT PRIMARY KEY,
 namespace TEXT NOT NULL,
 subject TEXT NOT NULL,
 personal_mailbox_id TEXT NOT NULL UNIQUE REFERENCES mailbox(id) DEFERRABLE INITIALLY DEFERRED,
 created_at INTEGER NOT NULL,
 last_seen_at INTEGER NOT NULL,
 UNIQUE(namespace,subject)
);
CREATE TABLE recipient_route (
 id TEXT PRIMARY KEY,
 namespace TEXT NOT NULL,
 address TEXT NOT NULL,
 mailbox_id TEXT NOT NULL REFERENCES mailbox(id),
 state TEXT NOT NULL CHECK(state IN ('pending','bound','blocked')),
 revision INTEGER NOT NULL CHECK(revision>0),
 created_at INTEGER NOT NULL,
 bound_at INTEGER,
 UNIQUE(namespace,address)
);
CREATE TABLE merge_journal (
 id TEXT PRIMARY KEY,
 source_id TEXT NOT NULL UNIQUE REFERENCES mailbox(id),
 target_id TEXT NOT NULL REFERENCES mailbox(id),
 state TEXT NOT NULL CHECK(state IN ('pending','complete')),
 created_at INTEGER NOT NULL,
 CHECK(source_id<>target_id)
);
CREATE TABLE binding_audit (
 id TEXT PRIMARY KEY,
 identity_id TEXT NOT NULL REFERENCES identity(id),
 route_id TEXT NOT NULL REFERENCES recipient_route(id),
 source_id TEXT NOT NULL REFERENCES mailbox(id),
 target_id TEXT NOT NULL REFERENCES mailbox(id),
 evidence TEXT NOT NULL,
 revision INTEGER NOT NULL,
 created_at INTEGER NOT NULL
);
PRAGMA user_version=1;
`

// Store is a single-process control store. All mutations serialize with route
// resolution so callers can hold WithRouteSnapshot through file submission.
// The mutex does not promise cross-process writer coordination.
type Store struct {
	db             *sql.DB
	mu             sync.Mutex
	readOnlySchema int
	writerFile     *os.File
	closeOnce      sync.Once
	closeErr       error
}

type Route struct {
	ID, Namespace, Address, MailboxID, State string
	Revision                                 int64
}
type Login struct {
	Namespace, Subject, Email string
	Verified                  bool
}
type ClaimPolicy string

const (
	VerifiedEmail   ClaimPolicy = "verified-email"
	ExplicitHistory ClaimPolicy = "explicit-history"
)

type ClaimResult struct {
	IdentityID, MailboxID string
	Conflict              bool
	MergeID               string
}

// Open creates a database only when explicitly requested. Existing nonempty
// databases with an unknown schema are never silently initialized.
func Open(path string, create bool) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("control database path must be absolute")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	path = filepath.Join(parent, filepath.Base(path))
	// Reject a final symlink before opening or changing permissions. Parent
	// directories must remain under the operator's control throughout use.
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("control database must be regular, not a symlink")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	fresh := false
	if errors.Is(err, os.ErrNotExist) && create {
		file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		fresh = err == nil
	}
	if err != nil {
		return nil, fmt.Errorf("open control database: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("control database must be regular")
	}
	if err = singleLinkControlFile(file); err != nil {
		return nil, err
	}
	writer, err := openWriterFence(path + ".writer-lock")
	if err != nil {
		return nil, err
	}
	owned := false
	defer func() {
		if !owned {
			_ = writer.Close()
		}
	}()
	if err = sameControlFile(path, info); err != nil {
		return nil, err
	}
	if err = file.Chmod(0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", controlSQLiteURI(path, "rw"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	for _, q := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA synchronous=FULL"} {
		if _, err = db.Exec(q); err != nil {
			return fail(err)
		}
	}
	if err = sameControlFile(path, info); err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if fresh {
		tx, txErr := db.Begin()
		if txErr != nil {
			return fail(txErr)
		}
		if _, err = tx.Exec(schema); err != nil {
			_ = tx.Rollback()
			return fail(err)
		}
		if err = upgradeControlSchema(tx); err != nil {
			_ = tx.Rollback()
			return fail(err)
		}
		if err = tx.Commit(); err != nil {
			return fail(err)
		}
	} else if version == 1 {
		tx, txErr := db.Begin()
		if txErr != nil {
			return fail(txErr)
		}
		if err = upgradeControlSchema(tx); err != nil {
			_ = tx.Rollback()
			return fail(err)
		}
		if err = tx.Commit(); err != nil {
			return fail(err)
		}
	} else if version != schemaVersion {
		return fail(fmt.Errorf("unsupported control database schema: %d", version))
	}
	var check string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return fail(errors.New("control database integrity check failed"))
	}
	if err = checkForeignKeys(db); err != nil {
		return fail(err)
	}
	if err = validateControlState(context.Background(), db); err != nil {
		return fail(err)
	}
	if err = sameControlFile(path, info); err != nil {
		return fail(err)
	}
	owned = true
	return &Store{db: db, writerFile: writer}, nil
}
func checkForeignKeys(db *sql.DB) error {
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return errors.New("control database foreign key check failed")
	}
	return rows.Err()
}

// Close retains the writer fence until SQLite has released its connections.
// Repeated cleanup calls return the same result without closing another file.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closeErr = s.db.Close()
		if s.writerFile != nil {
			s.closeErr = errors.Join(s.closeErr, s.writerFile.Close())
		}
	})
	return s.closeErr
}
func validIdentity(namespace, subject string) bool {
	for _, v := range []string{namespace, subject} {
		if v == "" || len(v) > 256 || strings.TrimSpace(v) != v || strings.ContainsAny(v, "\r\n\x00") {
			return false
		}
	}
	return true
}
func queryRoute(ctx context.Context, tx *sql.Tx, namespace, address string) (Route, error) {
	var r Route
	err := tx.QueryRowContext(ctx, `SELECT id,namespace,address,mailbox_id,state,revision FROM recipient_route WHERE namespace=? AND address=?`, namespace, address).Scan(&r.ID, &r.Namespace, &r.Address, &r.MailboxID, &r.State, &r.Revision)
	if err == nil {
		var cleaning int
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM retention_journal WHERE mailbox_id=? AND state='pending'`, r.MailboxID).Scan(&cleaning)
		if err == nil && cleaning > 0 {
			return Route{}, ErrRetentionPending
		}
	}
	return r, err
}

// EnsurePending is for a separately authenticated SMTP ingress. It is not an
// authentication API. Limits must also be enforced before calling it.
func (s *Store) EnsurePending(ctx context.Context, namespace, address string, maxPending int) (Route, error) {
	if s.readOnlySchema > 0 {
		return Route{}, ErrReadOnly
	}
	address, err := NormalizeAddress(address)
	if err != nil {
		return Route{}, err
	}
	if !validIdentity(namespace, "ingress") || maxPending <= 0 {
		return Route{}, errors.New("invalid ingress limits or namespace")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Route{}, err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := queryRoute(ctx, tx, namespace, address)
	if err == nil {
		if r.State == "blocked" {
			return Route{}, ErrBlocked
		}
		return r, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Route{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM mailbox m WHERE m.state='pending' AND EXISTS(SELECT 1 FROM recipient_route r WHERE r.mailbox_id=m.id AND r.state='pending')`).Scan(&count); err != nil {
		return Route{}, err
	}
	if count >= maxPending {
		return Route{}, ErrLimit
	}
	now := time.Now().Unix()
	r = Route{ID: uuid.NewString(), Namespace: namespace, Address: address, MailboxID: uuid.NewString(), State: "pending", Revision: 1}
	if _, err = tx.ExecContext(ctx, `INSERT INTO mailbox(id,state,created_at) VALUES(?,'pending',?)`, r.MailboxID, now); err != nil {
		return Route{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO recipient_route(id,namespace,address,mailbox_id,state,revision,created_at) VALUES(?,?,?,?,'pending',1,?)`, r.ID, namespace, address, r.MailboxID, now); err != nil {
		return Route{}, err
	}
	return r, tx.Commit()
}

// BindLogin only accepts evidence verified by the trusted upstream adapter.
// On conflict the identity retains its own mailbox, but never acquires the
// conflicting route or its history.
func (s *Store) BindLogin(ctx context.Context, login Login, policy ClaimPolicy) (ClaimResult, error) {
	if s.readOnlySchema > 0 {
		return ClaimResult{}, ErrReadOnly
	}
	if !validIdentity(login.Namespace, login.Subject) {
		return ClaimResult{}, errors.New("invalid identity")
	}
	if policy != VerifiedEmail && policy != ExplicitHistory {
		return ClaimResult{}, errors.New("invalid claim policy")
	}
	address := ""
	if login.Email != "" && login.Verified {
		var err error
		address, err = NormalizeAddress(login.Email)
		if err != nil {
			return ClaimResult{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result := ClaimResult{}
	err = tx.QueryRowContext(ctx, `SELECT id,personal_mailbox_id FROM identity WHERE namespace=? AND subject=?`, login.Namespace, login.Subject).Scan(&result.IdentityID, &result.MailboxID)
	fresh := errors.Is(err, sql.ErrNoRows)
	if err != nil && !fresh {
		return result, err
	}
	var route Route
	routeExists := false
	if address != "" {
		route, err = queryRoute(ctx, tx, login.Namespace, address)
		routeExists = err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
	}
	now := time.Now().Unix()
	if fresh {
		result.IdentityID = uuid.NewString()
		result.MailboxID = uuid.NewString()
		if routeExists && route.State == "pending" && policy == VerifiedEmail {
			result.MailboxID = route.MailboxID
			_, err = tx.ExecContext(ctx, `UPDATE mailbox SET state='personal',owner_identity_id=?,claimed_at=? WHERE id=? AND state='pending'`, result.IdentityID, now, result.MailboxID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO mailbox(id,state,owner_identity_id,created_at,claimed_at) VALUES(?,'personal',?,?,?)`, result.MailboxID, result.IdentityID, now, now)
		}
		if err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity(id,namespace,subject,personal_mailbox_id,created_at,last_seen_at) VALUES(?,?,?,?,?,?)`, result.IdentityID, login.Namespace, login.Subject, result.MailboxID, now, now); err != nil {
			return result, err
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE identity SET last_seen_at=? WHERE id=?`, now, result.IdentityID); err != nil {
			return result, err
		}
	}
	if address != "" {
		if routeExists && (route.State == "blocked" || (route.State == "bound" && route.MailboxID != result.MailboxID)) {
			result.Conflict = true
			if _, err = tx.ExecContext(ctx, `INSERT INTO binding_audit(id,identity_id,route_id,source_id,target_id,evidence,revision,created_at) VALUES(?,?,?,?,?,'trusted_login_conflict',?,?)`, uuid.NewString(), result.IdentityID, route.ID, route.MailboxID, result.MailboxID, route.Revision, now); err != nil {
				return result, err
			}
		} else if !routeExists || route.State == "pending" {
			source := route.MailboxID
			if !routeExists {
				route = Route{ID: uuid.NewString(), MailboxID: result.MailboxID, Revision: 0}
				source = result.MailboxID
				_, err = tx.ExecContext(ctx, `INSERT INTO recipient_route(id,namespace,address,mailbox_id,state,revision,created_at,bound_at) VALUES(?,?,?,?,'bound',1,?,?)`, route.ID, login.Namespace, address, result.MailboxID, now, now)
			} else {
				_, err = tx.ExecContext(ctx, `UPDATE recipient_route SET mailbox_id=?,state='bound',revision=revision+1,bound_at=? WHERE id=?`, result.MailboxID, now, route.ID)
				if err == nil && source != result.MailboxID && policy == VerifiedEmail {
					result.MergeID = uuid.NewString()
					_, err = tx.ExecContext(ctx, `INSERT INTO merge_journal(id,source_id,target_id,state,created_at) VALUES(?,?,?,'pending',?)`, result.MergeID, source, result.MailboxID, now)
				}
			}
			if err != nil {
				return result, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO binding_audit(id,identity_id,route_id,source_id,target_id,evidence,revision,created_at) VALUES(?,?,?,?,?,'trusted_login',?,?)`, uuid.NewString(), result.IdentityID, route.ID, source, result.MailboxID, route.Revision+1, now); err != nil {
				return result, err
			}
		}
	}
	return result, tx.Commit()
}

// WithRouteSnapshot serializes a short local durable submission with binding.
// The callback must not call Store methods or perform network operations.
func (s *Store) WithRouteSnapshot(ctx context.Context, namespace string, recipients []string, fn func([]Route) error) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	routes := make([]Route, 0, len(recipients))
	for _, recipient := range recipients {
		address, err := NormalizeAddress(recipient)
		if err != nil {
			return err
		}
		route, err := queryRoute(ctx, tx, namespace, address)
		if err != nil {
			return err
		}
		if route.State == "blocked" {
			return ErrBlocked
		}
		routes = append(routes, route)
	}
	// Release the DB connection before the callback, but retain the mutation
	// lock. Binding cannot interleave with the local commit.
	if err = tx.Commit(); err != nil {
		return err
	}
	return fn(routes)
}

func upgradeControlSchema(tx *sql.Tx) error {
	if err := upgradeRetentionSchema(tx); err != nil {
		return err
	}
	if err := upgradeQuotaSchema(tx); err != nil {
		return err
	}
	_, err := tx.Exec("PRAGMA user_version=2")
	return err
}
