package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/soulteary/owlmail/internal/mailbox"
)

// runRecipientRoute is an offline operator command, not a serving-mode bypass.
func runRecipientRoute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if stdout == nil {
		return errors.New("route output writer required")
	}
	fs := flag.NewFlagSet("recipient-route", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var request mailbox.RouteChangeRequest
	var database, confirmation string
	var apply, offline bool
	fs.StringVar(&database, "control-database", "", "Absolute control database path")
	fs.StringVar(&request.Action, "action", "inspect", "inspect, block, or reassign; mutations default to a read-only plan")
	fs.StringVar(&request.Namespace, "identity-namespace", "", "Explicit stable identity namespace")
	fs.StringVar(&request.Address, "address", "", "Exact recipient address selected by the operator")
	fs.StringVar(&request.ExpectedID, "expected-route-id", "", "Exact inspected route UUID (block/reassign)")
	fs.Int64Var(&request.ExpectedRevision, "expected-revision", 0, "Exact inspected route revision (block/reassign)")
	fs.StringVar(&request.TargetSubject, "target-subject", "", "Existing stable subject; original owner resumes a mistaken block (reassign only)")
	fs.StringVar(&request.Reason, "reason", "", "Short nonsecret audit reason; never pass credentials in command arguments")
	fs.BoolVar(&apply, "apply", false, "Apply the exact confirmed plan; otherwise inspect/plan without writes")
	fs.BoolVar(&offline, "offline", false, "Declare ALL writers, including old binaries, stopped before applying")
	fs.StringVar(&confirmation, "confirm-plan", "", "Exact SHA256 token returned by the read-only plan")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected route command arguments")
	}
	if request.Action == "inspect" && (apply || offline || confirmation != "" || request.ExpectedID != "" || request.ExpectedRevision != 0 || request.TargetSubject != "" || request.Reason != "") {
		return errors.New("inspect accepts only database, namespace and address")
	}
	if apply && (!offline || confirmation == "") {
		return errors.New("apply requires an explicit offline declaration and exact plan confirmation")
	}
	if !apply && (offline || confirmation != "") {
		return errors.New("offline and confirmation flags require apply")
	}
	reader, err := mailbox.OpenReadOnly(database)
	if err != nil {
		return err
	}
	if request.Action == "inspect" {
		snapshot, inspectErr := reader.InspectOperatorRoute(ctx, request.Namespace, request.Address)
		if err = errors.Join(inspectErr, reader.Close()); err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(snapshot)
	}
	plan, planErr := reader.PlanRouteChange(ctx, request)
	if err = errors.Join(planErr, reader.Close()); err != nil {
		return err
	}
	if !apply {
		return json.NewEncoder(stdout).Encode(plan)
	}
	// Reject a mistyped token before opening writable state or creating a fence.
	if confirmation != plan.Confirmation {
		return errors.New("confirmation does not match the current exact route plan")
	}
	writer, err := mailbox.Open(database, false)
	if err != nil {
		return err
	}
	result, applyErr := writer.ApplyRouteChange(ctx, request, confirmation)
	if err = errors.Join(applyErr, writer.Close()); err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}
