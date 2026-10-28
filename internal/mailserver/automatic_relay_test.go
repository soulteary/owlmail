package mailserver

import (
	"errors"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/relay"
)

func automaticTestServer(t *testing.T, directory string, enabled bool) *MailServer {
	t.Helper()
	server, err := NewMailServerWithOptions(0, "127.0.0.1", directory, ServerOptions{AllowEphemeralPort: true, DeferRelayRecovery: true, OutgoingConfig: &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 2525, AutoRelay: enabled}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func automaticStore(t *testing.T, server *MailServer, id string) {
	t.Helper()
	if err := server.storeIncomingEmail(id, strings.NewReader("From: visible@example.com\r\nTo: visible-to@example.com\r\nSubject: durable\r\n\r\nbody"), &Session{from: "sender@example.com", to: []string{"hidden@example.com"}}); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticRelayPersistsBeforeDispatchAndProtectsSource(t *testing.T) {
	server := automaticTestServer(t, t.TempDir(), true)
	automaticStore(t, server, "auto-pending")
	manager := server.automaticRelay.Load()
	if manager.pendingCount.Load() != 1 {
		t.Fatal("missing durable intent")
	}
	intent, err := manager.read("auto-pending")
	if err != nil {
		t.Fatal(err)
	}
	if intent.Envelope.From != "sender@example.com" || len(intent.Recipients) != 1 || intent.Recipients[0] != "hidden@example.com" {
		t.Fatalf("lost SMTP snapshot: %+v", intent)
	}
	if _, ok := server.RelayService().Get(relay.RequestID("auto", "auto-pending")); ok {
		t.Fatal("dispatch before startup gate")
	}
	if err := server.DeleteEmail("auto-pending"); err == nil {
		t.Fatal("pending source was deletable")
	}
	if err := server.DeleteAllEmail(); err == nil {
		t.Fatal("bulk delete removed pending source")
	}
}

func TestAutomaticRelayRollbackDoesNotLeaveIntent(t *testing.T) {
	server := automaticTestServer(t, t.TempDir(), true)
	if err := server.OnSynchronous("new", func(*Email) error { return errors.New("handoff rejected") }); err != nil {
		t.Fatal(err)
	}
	err := server.storeIncomingEmail("auto-rollback", strings.NewReader("From: a@example.com\r\nTo: b@example.com\r\n\r\nbody"), nil)
	if err == nil {
		t.Fatal("accepted rejected transaction")
	}
	if server.automaticRelay.Load().pendingCount.Load() != 0 {
		t.Fatal("rollback retained intent")
	}
	if _, err := os.Stat(filepath.Join(server.automaticRelay.Load().directory, "auto-rollback.json")); !os.IsNotExist(err) {
		t.Fatalf("rollback intent: %v", err)
	}
}

func TestAutomaticRelayReadinessDoesNotWaitForIntentIO(t *testing.T) {
	server := automaticTestServer(t, t.TempDir(), true)
	manager := server.automaticRelay.Load()
	entered, release := make(chan struct{}), make(chan struct{})
	original := manager.syncDirectory
	var blocked sync.Once
	manager.syncDirectory = func(string) error { blocked.Do(func() { close(entered); <-release }); return nil }
	done := make(chan struct{})
	go func() {
		_ = server.storeIncomingEmail("auto-blocked", strings.NewReader("From: a@example.com\r\nTo: b@example.com\r\n\r\nbody"), nil)
		close(done)
	}()
	<-entered
	health := make(chan struct{})
	go func() { _, _ = server.GetRelayJobHealth(); close(health) }()
	select {
	case <-health:
	case <-time.After(time.Second):
		t.Error("readiness blocked on intent filesystem I/O")
	}
	close(release)
	<-done
	manager.syncDirectory = original
}

func TestAutomaticRelayDynamicConfigurationAndNoRestoreReplay(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(0, "127.0.0.1", directory, ServerOptions{AllowEphemeralPort: true, DeferRelayRecovery: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 2525, AutoRelay: true}); err != nil {
		t.Fatal(err)
	}
	automaticStore(t, server, "dynamic-auto")
	if server.automaticRelay.Load() == nil || server.automaticRelay.Load().pendingCount.Load() != 1 {
		t.Fatal("dynamic configuration missed automatic intent")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	restored := automaticTestServer(t, directory, false)
	if restored.automaticRelay.Load().pendingCount.Load() != 1 {
		t.Fatal("accepted intent lost when automatic forwarding disabled")
	}
	if len(restored.automaticRelay.Load().pending) != 1 {
		t.Fatal("restore generated duplicate intents")
	}
}

func TestAutomaticRelayCorruptIntentFailsClosed(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, metadataDirectoryName, relayIntentMetadataDirectoryName)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "corrupt.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(0, "127.0.0.1", directory, ServerOptions{AllowEphemeralPort: true, DeferRelayRecovery: true})
	if server != nil {
		_ = server.Close()
	}
	if err == nil {
		t.Fatal("corrupt intent did not block writable startup")
	}
}

func TestAutomaticRelayDurablePromotionAndSubmittedTombstone(t *testing.T) {
	destination, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{AllowEphemeralPort: true})
	if err != nil {
		t.Fatal(err)
	}
	ready, result := make(chan struct{}), make(chan error, 1)
	go func() { result <- destination.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP startup timeout")
	}
	defer func() { _ = destination.Close(); <-result }()
	directory := t.TempDir()
	source := automaticTestServer(t, directory, true)
	if err := source.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: destination.GetPort(), AutoRelay: true}); err != nil {
		t.Fatal(err)
	}
	automaticStore(t, source, "auto-once")
	manager := source.automaticRelay.Load()
	manager.mu.Lock()
	intent := manager.pending["auto-once"]
	manager.mu.Unlock()
	// Simulate a producer ACK failure after the job was durably submitted.
	original := manager.syncDirectory
	manager.syncDirectory = func(string) error { return errors.New("injected ACK sync failure") }
	if err := manager.promote(intent); err == nil {
		t.Fatal("expected ACK failure")
	}
	id := relay.RequestID("auto", "auto-once")
	job, ok := source.RelayService().Get(id)
	if !ok || !job.HandoffPending {
		t.Fatalf("job dispatched without producer ACK: %+v", job)
	}
	source.RelayService().Start()
	time.Sleep(50 * time.Millisecond)
	if len(destination.GetAllEmail()) != 0 {
		t.Fatal("held job sent before ACK")
	}
	manager.syncDirectory = original
	if err := manager.promote(intent); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(destination.GetAllEmail()) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(destination.GetAllEmail()) != 1 {
		t.Fatal("accepted job was not delivered")
	}
	delivered := destination.GetAllEmail()[0]
	if delivered.Envelope == nil || delivered.Envelope.From != "sender@example.com" || len(delivered.Envelope.To) != 1 || delivered.Envelope.To[0] != "hidden@example.com" {
		t.Fatalf("SMTP delivery lost original envelope: %+v", delivered.Envelope)
	}
	if manager.pendingCount.Load() != 0 {
		t.Fatal("acknowledged intent remains pending")
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	// Simulate bounded job history pruning; the producer tombstone must prevent replay.
	if err := os.Remove(filepath.Join(directory, metadataDirectoryName, relayJobMetadataDirectoryName, id+".json")); err != nil {
		t.Fatal(err)
	}
	restarted := automaticTestServer(t, directory, false)
	if err := restarted.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: destination.GetPort()}); err != nil {
		t.Fatal(err)
	}
	if err := restarted.StartRelayRecovery(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if len(destination.GetAllEmail()) != 1 {
		t.Fatal("submitted tombstone replayed after job history pruning")
	}
	if _, ok := restarted.RelayService().Get(id); ok {
		t.Fatal("submitted tombstone recreated missing job")
	}
}

func TestAutomaticRelayFilteredRecipientsProduceNoIntent(t *testing.T) {
	server := automaticTestServer(t, t.TempDir(), true)
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 2525, AutoRelay: true, DenyRules: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	automaticStore(t, server, "filtered-auto")
	if server.automaticRelay.Load().pendingCount.Load() != 0 {
		t.Fatal("empty filtered recipient set queued work")
	}
	if err := server.DeleteEmail("filtered-auto"); err != nil {
		t.Fatalf("intentional no-delivery message should remain deletable: %v", err)
	}
}

func TestAutomaticRelayRejectsUnacceptedTransaction(t *testing.T) {
	server := automaticTestServer(t, t.TempDir(), true)
	automaticStore(t, server, "unaccepted-auto")
	manager := server.automaticRelay.Load()
	manager.mu.Lock()
	intent := manager.pending["unaccepted-auto"]
	manager.mu.Unlock()
	if err := server.writeRollbackFenceState(intent.EmailID, activeFenceState); err != nil {
		t.Fatal(err)
	}
	if err := manager.promote(intent); err != nil {
		t.Fatal(err)
	}
	if manager.pendingCount.Load() != 0 {
		t.Fatal("uncommitted transaction remained eligible")
	}
	if _, ok := server.RelayService().Get(relay.RequestID("auto", intent.EmailID)); ok {
		t.Fatal("uncommitted transaction created delivery job")
	}
}

func TestAutomaticRelayDirectoryCreationRequiresDurableParentLinks(t *testing.T) {
	root := t.TempDir()
	manager := &automaticRelayManager{directory: filepath.Join(root, metadataDirectoryName, relayIntentMetadataDirectoryName)}
	var synced []string
	injected := errors.New("parent sync failed")
	manager.syncDirectory = func(path string) error { synced = append(synced, path); return injected }
	if err := manager.ensureDirectory(); !errors.Is(err, injected) {
		t.Fatalf("parent failure ignored: %v", err)
	}
	if len(synced) != 1 || synced[0] != filepath.Join(root, metadataDirectoryName) {
		t.Fatalf("wrong parent synced: %v", synced)
	}
	// A failed first sync leaves directories present. Retrying must sync their
	// parent links again, rather than considering their existence durable.
	synced = nil
	manager.syncDirectory = func(path string) error { synced = append(synced, path); return nil }
	if err := manager.ensureDirectory(); err != nil {
		t.Fatal(err)
	}
	if len(synced) != 2 || synced[0] != filepath.Join(root, metadataDirectoryName) || synced[1] != root {
		t.Fatalf("missing durable ancestor sync: %v", synced)
	}
}

func TestAutomaticRelayStandaloneDynamicEnableStartsRecovery(t *testing.T) {
	start := func() *MailServer {
		server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{AllowEphemeralPort: true})
		if err != nil {
			t.Fatal(err)
		}
		ready, result := make(chan struct{}), make(chan error, 1)
		go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
		select {
		case <-ready:
		case err := <-result:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("SMTP bind timeout")
		}
		t.Cleanup(func() { _ = server.Close(); <-result })
		return server
	}
	destination, source := start(), start()
	if source.automaticRelay.Load() != nil {
		t.Fatal("empty standalone server unexpectedly owns Relay")
	}
	if err := source.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: destination.GetPort(), AutoRelay: true}); err != nil {
		t.Fatal(err)
	}
	// Exercise real SMTP acceptance after enabling the previously absent owner.
	address, ok := source.BoundAddress()
	if !ok {
		t.Fatal("source listener absent")
	}
	if err := smtp.SendMail(net.JoinHostPort(address.Host, strconv.Itoa(address.Port)), nil, "raw@example.com", []string{"hidden@example.com"}, []byte("From: visible@example.com\r\nTo: header@example.com\r\nSubject: dynamic\r\n\r\nbody\r\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(destination.GetAllEmail()) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(destination.GetAllEmail()) != 1 {
		t.Fatal("dynamically initialized standalone Relay never started")
	}
}
