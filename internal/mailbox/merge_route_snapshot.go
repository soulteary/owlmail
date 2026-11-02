package mailbox

import (
	"context"
	"errors"
)

// ErrRouteChanged means an RCPT decision was invalidated before DATA commit.
// It is a temporary refusal, not permission to select another recipient owner.
var ErrRouteChanged = errors.New("recipient route changed during submission")

// WithExpectedRouteSnapshot revalidates RCPT-time routing under the same
// mutation lock used by binding and merges. A pending mailbox may be claimed
// while DATA is arriving, but only an audited claim may change its target.
// Already-bound routes cannot silently follow address reassignment.
func (s *Store) WithExpectedRouteSnapshot(ctx context.Context, namespace string, expected []Route, fn func([]Route) error) error {
	if s.readOnlySchema > 0 {
		return ErrReadOnly
	}
	if !validIdentity(namespace, "ingress") || len(expected) == 0 || len(expected) > 1000 || fn == nil {
		return ErrRouteChanged
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	routes := make([]Route, 0, len(expected))
	seen := map[string]bool{}
	for _, old := range expected {
		address, err := NormalizeAddress(old.Address)
		if err != nil || address != old.Address || old.Namespace != namespace || old.Revision < 1 || seen[address] {
			return ErrRouteChanged
		}
		seen[address] = true
		current, err := queryRoute(ctx, tx, namespace, address)
		if err != nil {
			return err
		}
		if current.State == "blocked" {
			return ErrBlocked
		}
		if current.ID != old.ID {
			return ErrRouteChanged
		}
		unchanged := current.MailboxID == old.MailboxID && current.State == old.State && current.Revision == old.Revision
		if !unchanged {
			if old.State != "pending" || current.State != "bound" || current.Revision <= old.Revision {
				return ErrRouteChanged
			}
			var authorized int
			err = tx.QueryRowContext(ctx, `SELECT count(*) FROM binding_audit a JOIN identity i ON i.id=a.identity_id JOIN mailbox m ON m.id=a.target_id AND m.owner_identity_id=i.id WHERE a.route_id=? AND a.source_id=? AND a.target_id=? AND a.revision=? AND a.evidence='trusted_login' AND i.namespace=? AND i.personal_mailbox_id=m.id AND m.state='personal'`, current.ID, old.MailboxID, current.MailboxID, current.Revision, namespace).Scan(&authorized)
			if err != nil {
				return err
			}
			if authorized != 1 {
				return ErrRouteChanged
			}
		}
		routes = append(routes, current)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return fn(routes)
}
