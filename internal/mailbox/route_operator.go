package mailbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// OperatorRoute is selected by a local operator, never returned by the Web API.
type OperatorRoute struct {
	Namespace  string `json:"namespace"`
	Address    string `json:"address"`
	ID         string `json:"route_id"`
	Revision   int64  `json:"revision"`
	State      string `json:"state"`
	MailboxID  string `json:"mailbox_id"`
	Subject    string `json:"subject,omitempty"`
	identityID string
}

type RouteChangeRequest struct {
	Action, Namespace, Address, ExpectedID, TargetSubject, Reason string
	ExpectedRevision                                              int64
}

type RouteChangePlan struct {
	Version          int           `json:"version"`
	Action           string        `json:"action"`
	Current          OperatorRoute `json:"current"`
	NextRevision     int64         `json:"next_revision"`
	TargetSubject    string        `json:"target_subject"`
	TargetMailboxID  string        `json:"target_mailbox_id"`
	Reason           string        `json:"reason"`
	Confirmation     string        `json:"confirm_plan"`
	Applied          bool          `json:"applied"`
	AuditID          string        `json:"audit_id,omitempty"`
	targetIdentityID string
}

type operatorRouteEvidence struct {
	Version  int    `json:"version"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason"`
	PlanHash string `json:"plan_hash"`
}

func operatorRoute(ctx context.Context, tx *sql.Tx, namespace, address string) (OperatorRoute, error) {
	var result OperatorRoute
	if !validIdentity(namespace, "operator") {
		return result, errors.New("valid identity namespace required")
	}
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return result, err
	}
	var version int
	if err = tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return result, err
	}
	if version != schemaVersion {
		return result, errors.New("route operations require the current control schema")
	}
	err = tx.QueryRowContext(ctx, `SELECT r.id,r.revision,r.state,r.mailbox_id,COALESCE(i.subject,''),COALESCE(i.id,'') FROM recipient_route r JOIN mailbox m ON m.id=r.mailbox_id LEFT JOIN identity i ON i.id=m.owner_identity_id AND i.personal_mailbox_id=m.id AND i.namespace=r.namespace WHERE r.namespace=? AND r.address=?`, namespace, normalized).Scan(&result.ID, &result.Revision, &result.State, &result.MailboxID, &result.Subject, &result.identityID)
	if err != nil {
		return result, err
	}
	result.Namespace, result.Address = namespace, normalized
	return result, nil
}

// InspectOperatorRoute is a point-in-time local view and makes no changes.
func (s *Store) InspectOperatorRoute(ctx context.Context, namespace, address string) (OperatorRoute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OperatorRoute{}, err
	}
	defer func() { _ = tx.Rollback() }()
	return operatorRoute(ctx, tx, namespace, address)
}

func (s *Store) PlanRouteChange(ctx context.Context, request RouteChangeRequest) (RouteChangePlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RouteChangePlan{}, err
	}
	defer func() { _ = tx.Rollback() }()
	return planRouteChange(ctx, tx, request)
}

func planRouteChange(ctx context.Context, tx *sql.Tx, request RouteChangeRequest) (RouteChangePlan, error) {
	var plan RouteChangePlan
	if request.Action != "block" && request.Action != "reassign" {
		return plan, errors.New("action must be block or reassign")
	}
	id, err := uuid.Parse(request.ExpectedID)
	if err != nil || id.String() != request.ExpectedID || request.ExpectedRevision < 1 || request.ExpectedRevision == int64(^uint64(0)>>1) {
		return plan, errors.New("canonical expected route ID and bounded positive revision required")
	}
	if !validOperatorReason(request.Reason) {
		return plan, errors.New("a short nonsecret reason is required")
	}
	current, err := operatorRoute(ctx, tx, request.Namespace, request.Address)
	if err != nil {
		return plan, err
	}
	if current.ID != request.ExpectedID || current.Revision != request.ExpectedRevision {
		return plan, ErrRouteChanged
	}
	if current.identityID == "" || current.Subject == "" {
		return plan, errors.New("operator changes require an existing personal route")
	}
	plan = RouteChangePlan{Version: 1, Action: request.Action, Current: current, NextRevision: current.Revision + 1, TargetSubject: current.Subject, TargetMailboxID: current.MailboxID, Reason: request.Reason, targetIdentityID: current.identityID}
	if request.Action == "block" {
		if current.State != "bound" || request.TargetSubject != "" {
			return RouteChangePlan{}, errors.New("block requires a bound route and no target subject")
		}
	} else {
		if current.State != "blocked" || !validIdentity(request.Namespace, request.TargetSubject) {
			return RouteChangePlan{}, errors.New("reassign requires a blocked route and an existing subject")
		}
		if err = operatorBlockEvidence(ctx, tx, current); err != nil {
			return RouteChangePlan{}, err
		}
		plan.TargetSubject = request.TargetSubject
		if err = tx.QueryRowContext(ctx, `SELECT i.id,m.id FROM identity i JOIN mailbox m ON m.id=i.personal_mailbox_id AND m.owner_identity_id=i.id WHERE i.namespace=? AND i.subject=? AND m.state='personal' AND m.retired_at IS NULL AND m.merged_into_id IS NULL`, request.Namespace, request.TargetSubject).Scan(&plan.targetIdentityID, &plan.TargetMailboxID); err != nil {
			return RouteChangePlan{}, err
		}
	}
	var pending int
	err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM merge_journal WHERE state='pending' AND (source_id IN (?,?) OR target_id IN (?,?))) + (SELECT count(*) FROM retention_journal WHERE state='pending' AND mailbox_id IN (?,?))`, current.MailboxID, plan.TargetMailboxID, current.MailboxID, plan.TargetMailboxID, current.MailboxID, plan.TargetMailboxID).Scan(&pending)
	if err != nil {
		return RouteChangePlan{}, err
	}
	if pending != 0 {
		return RouteChangePlan{}, errors.New("finish affected merge or retention recovery before changing a route")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return RouteChangePlan{}, err
	}
	digest := sha256.Sum256(raw)
	plan.Confirmation = hex.EncodeToString(digest[:])
	return plan, nil
}

func operatorBlockEvidence(ctx context.Context, tx *sql.Tx, current OperatorRoute) error {
	rows, err := tx.QueryContext(ctx, `SELECT evidence FROM binding_audit WHERE route_id=? AND revision=? AND identity_id=? AND source_id=? AND target_id=?`, current.ID, current.Revision, current.identityID, current.MailboxID, current.MailboxID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return err
		}
		var evidence operatorRouteEvidence
		if json.Unmarshal([]byte(raw), &evidence) == nil && evidence.Version == 1 && evidence.Kind == "operator_route_block" && validOperatorReason(evidence.Reason) {
			digest, decodeErr := hex.DecodeString(evidence.PlanHash)
			if decodeErr == nil && len(digest) == sha256.Size {
				found = true
			}
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if !found {
		return errors.New("blocked route lacks matching operator-block evidence")
	}
	return nil
}

// ApplyRouteChange atomically changes only future routing and appends operator
// evidence. It never creates identities, changes mailbox owners or merges mail.
// CLI callers must additionally declare all writers stopped; Open's writer fence
// excludes upgraded processes, not old binaries or raw SQL administrators.
func (s *Store) ApplyRouteChange(ctx context.Context, request RouteChangeRequest, confirmation string) (RouteChangePlan, error) {
	if s.readOnlySchema > 0 {
		return RouteChangePlan{}, ErrReadOnly
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RouteChangePlan{}, err
	}
	defer func() { _ = tx.Rollback() }()
	plan, err := planRouteChange(ctx, tx, request)
	if err != nil {
		return RouteChangePlan{}, err
	}
	if confirmation == "" || confirmation != plan.Confirmation {
		return RouteChangePlan{}, errors.New("confirmation does not match the current exact route plan")
	}
	state := "blocked"
	kind := "operator_route_block"
	if request.Action == "reassign" {
		state, kind = "bound", "operator_route_reassign"
		if plan.TargetMailboxID == plan.Current.MailboxID {
			kind = "operator_route_resume"
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE recipient_route SET mailbox_id=?,state=?,revision=?,bound_at=CASE WHEN ?='bound' THEN ? ELSE bound_at END WHERE id=? AND revision=? AND mailbox_id=? AND state=?`, plan.TargetMailboxID, state, plan.NextRevision, state, time.Now().Unix(), plan.Current.ID, plan.Current.Revision, plan.Current.MailboxID, plan.Current.State)
	if err != nil {
		return RouteChangePlan{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return RouteChangePlan{}, err
	}
	if changed != 1 {
		return RouteChangePlan{}, ErrRouteChanged
	}
	evidence, err := json.Marshal(operatorRouteEvidence{1, kind, plan.Reason, plan.Confirmation})
	if err != nil {
		return RouteChangePlan{}, err
	}
	plan.AuditID = uuid.NewString()
	if _, err = tx.ExecContext(ctx, `INSERT INTO binding_audit(id,identity_id,route_id,source_id,target_id,evidence,revision,created_at) VALUES(?,?,?,?,?,?,?,?)`, plan.AuditID, plan.targetIdentityID, plan.Current.ID, plan.Current.MailboxID, plan.TargetMailboxID, string(evidence), plan.NextRevision, time.Now().Unix()); err != nil {
		return RouteChangePlan{}, fmt.Errorf("record operator route audit: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return RouteChangePlan{}, err
	}
	plan.Applied = true
	return plan, nil
}

func validOperatorReason(reason string) bool {
	if reason == "" || len(reason) > 256 || strings.TrimSpace(reason) != reason {
		return false
	}
	for _, r := range reason {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
