package mailbox

import (
	"context"
	"database/sql"
	"errors"
)

// PersonalStatus contains no addresses, identities, counts or filesystem paths.
// It describes the current assertion's delivery route, not arbitrary addresses.
type PersonalStatus struct {
	MailboxState  string `json:"mailbox_state"`
	DeliveryState string `json:"delivery_state"`
	HistoryState  string `json:"history_state"`
	Action        string `json:"action"`
}

// LoginStatus derives status from the authority graph without creating bindings.
// Unverified addresses never cause a route or pending-history lookup.
func (s *Store) LoginStatus(ctx context.Context, login Login) (PersonalStatus, error) {
	status := PersonalStatus{MailboxState: "ready", DeliveryState: "unbound", HistoryState: "none", Action: "none"}
	if !validIdentity(login.Namespace, login.Subject) {
		return PersonalStatus{}, ErrMailboxUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	box, err := personalMailbox(ctx, s.db, login.Namespace, login.Subject)
	if err != nil {
		return PersonalStatus{}, err
	}
	if login.Email == "" {
		return status, nil
	}
	if !login.Verified {
		status.DeliveryState, status.Action = "verification_required", "verify_email"
		return status, nil
	}
	address, err := NormalizeAddress(login.Email)
	if err != nil {
		return PersonalStatus{}, ErrMailboxUnavailable
	}
	var routeID, target, state string
	err = s.db.QueryRowContext(ctx, `SELECT id,mailbox_id,state FROM recipient_route WHERE namespace=? AND address=?`, login.Namespace, address).Scan(&routeID, &target, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return PersonalStatus{}, err
	}
	if state == "blocked" || (state == "bound" && target != box) {
		status.DeliveryState, status.Action = "conflict", "contact_operator"
		return status, nil
	}
	if state != "bound" || target != box {
		return status, nil
	}
	status.DeliveryState = "bound"
	// Only this identity's successful binding audit authorizes history status.
	// Route revision matching excludes stale evidence after reassignment.
	var retained, merging, complete int
	err = s.db.QueryRowContext(ctx, `SELECT
 COALESCE(MAX(CASE WHEN src.state='pending' AND src.retired_at IS NULL AND j.id IS NULL THEN 1 ELSE 0 END),0),
 COALESCE(MAX(CASE WHEN j.state='pending' THEN 1 ELSE 0 END),0),
 COALESCE(MAX(CASE WHEN j.state='complete' THEN 1 ELSE 0 END),0)
 FROM binding_audit a
 JOIN identity i ON i.id=a.identity_id
 JOIN recipient_route r ON r.id=a.route_id AND r.revision=a.revision
 JOIN mailbox src ON src.id=a.source_id
 LEFT JOIN merge_journal j ON j.source_id=a.source_id AND j.target_id=a.target_id
 WHERE i.namespace=? AND i.subject=? AND a.route_id=? AND a.target_id=? AND a.source_id<>a.target_id AND a.evidence='trusted_login'`, login.Namespace, login.Subject, routeID, box).Scan(&retained, &merging, &complete)
	if err != nil {
		return PersonalStatus{}, err
	}
	switch {
	case merging != 0:
		status.MailboxState, status.HistoryState, status.Action = "recovering", "merging", "retry"
	case retained != 0:
		status.HistoryState = "retained"
	case complete != 0:
		status.HistoryState = "complete"
	}
	return status, nil
}
