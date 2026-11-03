package mailbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func operatorFixture(t *testing.T) (*Store, RouteChangeRequest, string, string) {
	t.Helper()
	s := testStore(t)
	ctx := context.Background()
	a, err := s.BindLogin(ctx, Login{"main", "alice", "alice@example.test", true}, ExplicitHistory)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.BindLogin(ctx, Login{Namespace: "main", Subject: "bob"}, ExplicitHistory)
	if err != nil {
		t.Fatal(err)
	}
	route, err := s.InspectOperatorRoute(ctx, "main", "alice@example.test")
	if err != nil {
		t.Fatal(err)
	}
	return s, RouteChangeRequest{Action: "block", Namespace: "main", Address: route.Address, ExpectedID: route.ID, ExpectedRevision: route.Revision, Reason: "Address reassigned by the identity authority"}, a.MailboxID, b.MailboxID
}

func TestOperatorRouteBlockReassignPreservesMailboxOwnershipAndAudits(t *testing.T) {
	s, request, oldBox, newBox := operatorFixture(t)
	ctx := context.Background()
	before, err := s.InspectOperatorRoute(ctx, "main", request.Address)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.PlanRouteChange(ctx, request)
	if err != nil || repeated.Confirmation != plan.Confirmation {
		t.Fatal("unstable plan", repeated, err)
	}
	blocked, err := s.ApplyRouteChange(ctx, request, plan.Confirmation)
	if err != nil || !blocked.Applied || blocked.AuditID == "" {
		t.Fatal(blocked, err)
	}
	if _, err = s.ApplyRouteChange(ctx, request, plan.Confirmation); !errors.Is(err, ErrRouteChanged) {
		t.Fatal("stale block accepted", err)
	}
	request.Action, request.ExpectedRevision, request.TargetSubject = "reassign", blocked.NextRevision, "bob"
	plan, err = s.PlanRouteChange(ctx, request)
	if err != nil || plan.TargetMailboxID != newBox || plan.Current.MailboxID != oldBox {
		t.Fatal(plan, err)
	}
	moved, err := s.ApplyRouteChange(ctx, request, plan.Confirmation)
	if err != nil || !moved.Applied {
		t.Fatal(moved, err)
	}
	after, err := s.InspectOperatorRoute(ctx, "main", request.Address)
	if err != nil || after.ID != before.ID || after.Revision != before.Revision+2 || after.State != "bound" || after.Subject != "bob" || after.MailboxID != newBox {
		t.Fatal(after, err)
	}
	for subject, want := range map[string]string{"alice": oldBox, "bob": newBox} {
		box, err := s.PersonalMailbox(ctx, "main", subject)
		if err != nil || box != want {
			t.Fatal("owner changed", subject, box, err)
		}
	}
	var merges, identities int
	if err = s.db.QueryRow(`SELECT count(*) FROM merge_journal`).Scan(&merges); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(`SELECT count(*) FROM identity`).Scan(&identities); err != nil || merges != 0 || identities != 2 {
		t.Fatal("created a user or history merge", merges, identities, err)
	}
	rows, err := s.db.Query(`SELECT evidence FROM binding_audit WHERE id IN (?,?)`, blocked.AuditID, moved.AuditID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var evidence operatorRouteEvidence
		if err = json.Unmarshal([]byte(raw), &evidence); err != nil || evidence.Version != 1 || evidence.Reason != request.Reason || len(evidence.PlanHash) != 64 {
			t.Fatal(evidence, err)
		}
		if evidence.Kind != "operator_route_block" && evidence.Kind != "operator_route_reassign" {
			t.Fatal(evidence)
		}
		count++
	}
	if err = rows.Err(); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal("ownership graph broken", err)
	}
	// An RCPT decision made before the block cannot follow the new target.
	old := Route{ID: before.ID, Namespace: before.Namespace, Address: before.Address, MailboxID: before.MailboxID, State: before.State, Revision: before.Revision}
	if err = s.WithExpectedRouteSnapshot(ctx, "main", []Route{old}, func([]Route) error { t.Fatal("stale recipient followed reassignment"); return nil }); !errors.Is(err, ErrRouteChanged) {
		t.Fatal(err)
	}
}

func TestOperatorRouteConfirmationBindsExactPlan(t *testing.T) {
	s, request, _, _ := operatorFixture(t)
	ctx := context.Background()
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.Reason = "A different decision"
	if _, err = s.ApplyRouteChange(ctx, changed, plan.Confirmation); err == nil {
		t.Fatal("reason changed without confirmation")
	}
	if _, err = s.ApplyRouteChange(ctx, request, ""); err == nil {
		t.Fatal("empty confirmation")
	}
	changed = request
	changed.ExpectedRevision++
	if _, err = s.PlanRouteChange(ctx, changed); !errors.Is(err, ErrRouteChanged) {
		t.Fatal(err)
	}
	changed = request
	changed.ExpectedID = "00000000-0000-0000-0000-000000000000"
	if _, err = s.PlanRouteChange(ctx, changed); !errors.Is(err, ErrRouteChanged) {
		t.Fatal(err)
	}
	for _, reason := range []string{"", " leading", "bad\nreason", string(make([]byte, 257))} {
		changed = request
		changed.Reason = reason
		if _, err = s.PlanRouteChange(ctx, changed); err == nil {
			t.Fatalf("invalid reason %q", reason)
		}
	}
	current, err := s.InspectOperatorRoute(ctx, "main", request.Address)
	if err != nil || current.Revision != plan.Current.Revision || current.State != "bound" {
		t.Fatal("failed confirmation changed state", current, err)
	}
}

func TestOperatorRouteRequiresOwnBlockExistingTargetAndNoRecovery(t *testing.T) {
	s, request, _, _ := operatorFixture(t)
	ctx := context.Background()
	// A bound route cannot be reassigned in one step.
	bad := request
	bad.Action, bad.TargetSubject = "reassign", "bob"
	if _, err := s.PlanRouteChange(ctx, bad); err == nil {
		t.Fatal("block step skipped")
	}
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := s.ApplyRouteChange(ctx, request, plan.Confirmation)
	if err != nil {
		t.Fatal(err)
	}
	request.Action, request.TargetSubject, request.ExpectedRevision = "reassign", "missing", blocked.NextRevision
	if _, err = s.PlanRouteChange(ctx, request); err == nil {
		t.Fatal("unknown subject accepted")
	}
	request.TargetSubject = "bob"
	if _, err = s.db.Exec(`DELETE FROM binding_audit WHERE id=?`, blocked.AuditID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PlanRouteChange(ctx, request); err == nil {
		t.Fatal("unaudited deny route reassigned")
	}
}

func TestOperatorRouteRefusesAffectedPendingMergeWithoutBreakingRecovery(t *testing.T) {
	s, request, _, _ := operatorFixture(t)
	ctx := context.Background()
	if _, err := s.EnsurePending(ctx, "main", "second@example.test", 10); err != nil {
		t.Fatal(err)
	}
	result, err := s.BindLogin(ctx, Login{"main", "alice", "second@example.test", true}, VerifiedEmail)
	if err != nil || result.MergeID == "" {
		t.Fatal(result, err)
	}
	if _, err = s.PlanRouteChange(ctx, request); err == nil {
		t.Fatal("affected merge ignored")
	}
	route, err := s.InspectOperatorRoute(ctx, "main", request.Address)
	if err != nil || route.State != "bound" || route.Revision != request.ExpectedRevision {
		t.Fatal(route, err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
	if err = s.RunPendingMerges(ctx, func(context.Context, Merge) error { return nil }); err != nil {
		t.Fatal("recovery damaged", err)
	}
	if _, err = s.PlanRouteChange(ctx, request); err != nil {
		t.Fatal("finished recovery still blocked", err)
	}
}

func TestOperatorRouteDryRunUsesReadOnlyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(context.Background(), Login{"main", "alice", "alice@example.test", true}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	route, err := s.InspectOperatorRoute(context.Background(), "main", "alice@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	// Remove only this stopped fixture's non-authority fence to prove reads create none.
	if err = os.Remove(path + ".writer-lock"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	request := RouteChangeRequest{Action: "block", Namespace: "main", Address: route.Address, ExpectedID: route.ID, ExpectedRevision: route.Revision, Reason: "Offline operator test"}
	if _, err = reader.PlanRouteChange(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err = reader.ApplyRouteChange(context.Background(), request, "anything"); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("dry run wrote authority", err)
	}
	if _, err = os.Stat(path + ".writer-lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only plan created a fence", err)
	}
}

func TestOperatorRouteAuditFailureRollsBackTheRoute(t *testing.T) {
	s, request, _, _ := operatorFixture(t)
	ctx := context.Background()
	before, err := s.InspectOperatorRoute(ctx, request.Namespace, request.Address)
	if err != nil {
		t.Fatal(err)
	}
	var audits int
	if err = s.db.QueryRow(`SELECT count(*) FROM binding_audit`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER reject_operator_audit BEFORE INSERT ON binding_audit WHEN NEW.evidence LIKE '%operator_route_%' BEGIN SELECT RAISE(ABORT,'injected operator audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyRouteChange(ctx, request, plan.Confirmation); err == nil {
		t.Fatal("audit failure accepted")
	}
	after, err := s.InspectOperatorRoute(ctx, request.Namespace, request.Address)
	if err != nil || after != before {
		t.Fatal("route committed without audit", before, after, err)
	}
	var now int
	if err = s.db.QueryRow(`SELECT count(*) FROM binding_audit`).Scan(&now); err != nil || now != audits {
		t.Fatal("failed audit persisted", audits, now, err)
	}
}

func TestOperatorRouteRefusesTargetRecoveryAndTargetTampering(t *testing.T) {
	s, request, _, _ := operatorFixture(t)
	ctx := context.Background()
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := s.ApplyRouteChange(ctx, request, plan.Confirmation)
	if err != nil {
		t.Fatal(err)
	}
	request.Action, request.TargetSubject, request.ExpectedRevision = "reassign", "bob", blocked.NextRevision
	if _, err = s.EnsurePending(ctx, "main", "bob-second@example.test", 10); err != nil {
		t.Fatal(err)
	}
	merge, err := s.BindLogin(ctx, Login{"main", "bob", "bob-second@example.test", true}, VerifiedEmail)
	if err != nil || merge.MergeID == "" {
		t.Fatal(merge, err)
	}
	if _, err = s.PlanRouteChange(ctx, request); err == nil {
		t.Fatal("target pending recovery ignored")
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal("target graph damaged", err)
	}
	if err = s.RunPendingMerges(ctx, func(context.Context, Merge) error { return nil }); err != nil {
		t.Fatal(err)
	}
	plan, err = s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindLogin(ctx, Login{Namespace: "main", Subject: "charlie"}, ExplicitHistory); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.TargetSubject = "charlie"
	if _, err = s.ApplyRouteChange(ctx, changed, plan.Confirmation); err == nil {
		t.Fatal("changed target reused confirmation")
	}
	route, err := s.InspectOperatorRoute(ctx, request.Namespace, request.Address)
	if err != nil || route.State != "blocked" || route.Revision != request.ExpectedRevision {
		t.Fatal("tampering changed route", route, err)
	}
}

func TestOperatorRouteCanExplicitlyResumeAnAccidentalBlock(t *testing.T) {
	s, request, oldBox, _ := operatorFixture(t)
	ctx := context.Background()
	plan, err := s.PlanRouteChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	block, err := s.ApplyRouteChange(ctx, request, plan.Confirmation)
	if err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExpectedRevision, request.TargetSubject, request.Reason = "reassign", block.NextRevision, "alice", "Resume an accidental block"
	resume, err := s.PlanRouteChange(ctx, request)
	if err != nil || resume.TargetMailboxID != oldBox {
		t.Fatal(resume, err)
	}
	applied, err := s.ApplyRouteChange(ctx, request, resume.Confirmation)
	if err != nil {
		t.Fatal(err)
	}
	route, err := s.InspectOperatorRoute(ctx, "main", request.Address)
	if err != nil || route.State != "bound" || route.MailboxID != oldBox || route.Subject != "alice" || route.Revision != 3 {
		t.Fatal(route, err)
	}
	var raw string
	if err = s.db.QueryRow(`SELECT evidence FROM binding_audit WHERE id=?`, applied.AuditID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var evidence operatorRouteEvidence
	if err = json.Unmarshal([]byte(raw), &evidence); err != nil || evidence.Kind != "operator_route_resume" {
		t.Fatal(evidence, err)
	}
	if err = validateControlState(ctx, s.db); err != nil {
		t.Fatal(err)
	}
}
