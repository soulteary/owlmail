package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"
)

func TestMailboxMigrationCLIValidation(t *testing.T) {
	var out, errout bytes.Buffer
	if err := runMailboxMigration(context.Background(), []string{"-help"}, &out, &errout); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"-direction=standalone-to-multiuser"}, {"unexpected"}, {"-offline", "-direction=invalid"}} {
		if err := runMailboxMigration(context.Background(), args, &out, &errout); err == nil {
			t.Fatal("accepted incomplete migration options")
		}
	}
}
