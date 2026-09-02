package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestRunMCPStdioHelp(t *testing.T) {
	var stderr bytes.Buffer
	err := runMCPStdio(context.Background(), []string{"-h"}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("runMCPStdio(-h) error = %v", err)
	}
	if !strings.Contains(stderr.String(), "mail-directory") {
		t.Fatalf("help did not include mailbox configuration: %s", stderr.String())
	}
}
