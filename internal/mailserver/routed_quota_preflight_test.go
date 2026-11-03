package mailserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoutedQuotaPreflightRejectsBeforePreparingAnyCopy(t *testing.T) {
	for _, test := range []struct {
		name   string
		limits func(*RoutedIngressOptions)
		seed   bool
	}{
		{name: "raw exceeds mailbox bytes", limits: func(o *RoutedIngressOptions) { o.MaxDiskBytes = int64(len(routedMessage) - 1) }},
		{name: "group exceeds global bytes", limits: func(o *RoutedIngressOptions) { o.MaxTotalDiskBytes = int64(2*len(routedMessage) - 1) }},
		{name: "group exceeds global messages", limits: func(o *RoutedIngressOptions) { o.MaxTotalMessages = 1 }},
		{name: "later target already full", limits: func(o *RoutedIngressOptions) { o.MaxMessages = 1 }, seed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, _ := routedOptions(t)
			test.limits(options)
			dir := t.TempDir()
			server := newRoutedTestServer(t, dir, options)
			if test.seed {
				if err := routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
					t.Fatal(err)
				}
			}
			before := len(server.GetAllEmail())
			prepared := 0
			server.beforeStoreCommit = func(*Email) error { prepared++; return nil }
			// Bob comes first to ensure Alice's later refusal precedes all copies.
			if err := routedTestSession(t, server, "bob@example.com", "alice@example.com").Data(strings.NewReader(routedMessage)); err == nil {
				t.Fatal("over quota group accepted")
			}
			if prepared != 0 {
				t.Fatalf("prepared %d copies before obvious quota refusal", prepared)
			}
			if len(server.GetAllEmail()) != before {
				t.Fatal("rejected group changed committed mail")
			}
			entries, err := os.ReadDir(filepath.Join(dir, ingressDirectory))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatal("rejected staging was not removed")
			}
		})
	}
}

func TestRoutedQuotaPreparationStopsAtFirstExactOverflow(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprintf("global=%t", global), func(t *testing.T) {
			options, _ := routedOptions(t)
			// All raw copies fit; decoded attachment and metadata do not. The
			// first exact copy must stop preparation before Bob's copy is made.
			if global {
				options.MaxTotalDiskBytes = int64(2 * len(routedMessage))
			} else {
				options.MaxDiskBytes = int64(len(routedMessage))
			}
			dir := t.TempDir()
			server := newRoutedTestServer(t, dir, options)
			prepared := 0
			server.beforeStoreCommit = func(*Email) error { prepared++; return nil }
			if err := routedTestSession(t, server, "alice@example.com", "bob@example.com").Data(strings.NewReader(routedMessage)); err == nil {
				t.Fatal("exact byte overflow accepted")
			}
			if prepared != 1 {
				t.Fatalf("prepared %d copies, want one", prepared)
			}
			if len(server.GetAllEmail()) != 0 {
				t.Fatal("partial group was published")
			}
			// A retry after increasing capacity proves local reservations never
			// become persistent charges and that both recipients still commit.
			server.routedIngress.options.MaxDiskBytes = 1 << 20
			server.routedIngress.options.MaxTotalDiskBytes = 1 << 20
			if err := routedTestSession(t, server, "alice@example.com", "bob@example.com").Data(strings.NewReader(routedMessage)); err != nil {
				t.Fatal(err)
			}
			if len(server.GetAllEmail()) != 2 {
				t.Fatal("retry did not publish complete group")
			}
		})
	}
}

func TestRoutedQuotaFinalCheckRechecksPreviouslyPreparedFiles(t *testing.T) {
	options, _ := routedOptions(t)
	options.MaxDiskBytes = 20 << 10
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	prepared := 0
	var firstID string
	server.beforeStoreCommit = func(email *Email) error {
		prepared++
		if prepared == 1 {
			firstID = email.ID
			return nil
		}
		// Model a prepared file changing after its incremental charge. Only the
		// final durable check can see this increase in the earlier target.
		entries, err := os.ReadDir(filepath.Join(dir, ingressDirectory))
		if err != nil {
			return err
		}
		if len(entries) != 1 {
			return fmt.Errorf("staging directories: %d", len(entries))
		}
		path := filepath.Join(dir, ingressDirectory, entries[0].Name(), metadataDirectoryName, firstID+".json")
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString(strings.Repeat(" ", 30<<10))
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	if err := routedTestSession(t, server, "alice@example.com", "bob@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("final durable byte check was bypassed")
	}
	if prepared != 2 {
		t.Fatalf("prepared %d copies, want two", prepared)
	}
	if len(server.GetAllEmail()) != 0 {
		t.Fatal("rejected transaction published mail")
	}
}
