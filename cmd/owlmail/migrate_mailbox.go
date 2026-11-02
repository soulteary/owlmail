package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/soulteary/owlmail/internal/mailserver"
)

// runMailboxMigration is independent of the gated multiuser server startup.
// It produces a new offline bundle and never starts listeners or delivery jobs.
func runMailboxMigration(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if stdout == nil {
		return fmt.Errorf("migration output writer required")
	}
	fs := flag.NewFlagSet("migrate-mailbox", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var options mailserver.MailboxMigrationOptions
	fs.StringVar(&options.Source, "source", "", "Absolute stopped source mail directory")
	fs.StringVar(&options.Destination, "destination", "", "Absolute new bundle directory")
	fs.StringVar(&options.Direction, "direction", "", "standalone-to-multiuser or multiuser-to-standalone")
	fs.StringVar(&options.Namespace, "identity-namespace", "", "Explicit destination identity namespace (import only)")
	fs.StringVar(&options.Subject, "subject", "", "Explicit stable destination subject (import only)")
	fs.StringVar(&options.MailboxID, "mailbox-id", "", "Exactly one personal mailbox ID (export only)")
	fs.StringVar(&options.ControlDatabase, "control-database", "", "Absolute stopped source authority database (export only)")
	fs.BoolVar(&options.Offline, "offline", false, "Confirm every source writer has been stopped; hashes additionally detect concurrent modifications")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected migration arguments")
	}
	result, err := mailserver.MigrateMailbox(ctx, options)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}
