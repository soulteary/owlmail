package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
)

func newRelayReadinessTestAPI(t *testing.T) (*API, *mailserver.MailServer) {
	t.Helper()
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	api := NewAPIWithHTTPSDeferredRecovery(server, 1080, "127.0.0.1", "", "", false, "", "")
	return api, server
}

func prepareRelayReadinessProbe(api *API) {
	// Drive owner checks synchronously so fault injection never races the timer.
	api.relayJobReadiness.mu.Lock()
	api.relayJobReadiness.started = true
	api.relayJobReadiness.mu.Unlock()
}

func TestDurableRelayReadinessPendingFailureAndDirectoryRecovery(t *testing.T) {
	api, server := newRelayReadinessTestAPI(t)
	monitor := api.relayJobReadiness
	if _, enabled := server.GetRelayJobHealth(); enabled {
		t.Fatal("unconfigured relay introduced a readiness dependency")
	}
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "localhost", Port: 2525}); err != nil {
		t.Fatal(err)
	}
	if status, enabled := server.GetRelayJobHealth(); !enabled || status.State != attachmentstore.HealthChecking {
		t.Fatalf("paused recovery: %+v, enabled=%t", status, enabled)
	}
	prepareRelayReadinessProbe(api)
	monitor.probe()
	if !monitor.Snapshot().Ready() {
		t.Fatal(monitor.Snapshot())
	}
	backup := monitor.directory + "-backup"
	if err := os.Rename(monitor.directory, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(monitor.directory, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	monitor.probe()
	if monitor.Snapshot().Ready() {
		t.Fatal("writable mail root concealed unusable relay subdirectory")
	}
	if err := mailserver.CheckMailDirectory(server.GetMailDir()); err != nil {
		t.Fatalf("mail root unexpectedly failed: %v", err)
	}
	if err := os.Remove(monitor.directory); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, monitor.directory); err != nil {
		t.Fatal(err)
	}
	monitor.probe()
	if status := monitor.Snapshot(); !status.Ready() {
		t.Fatalf("restored directory did not recover: %+v", status)
	}
	entries, err := os.ReadDir(monitor.directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("health scratch artifacts: %v, %v", entries, err)
	}
}

func TestDurableRelayScratchProbeCannotConcealUnresolvedRecordWrite(t *testing.T) {
	api, _ := newRelayReadinessTestAPI(t)
	prepareRelayReadinessProbe(api)
	monitor, store := api.relayJobReadiness, api.relayJobs
	monitor.probe()
	job, err := store.create("failed-completion", "")
	if err != nil {
		t.Fatal(err)
	}
	store.syncDirectory = func(string) error { return errors.New("sensitive filesystem path and credentials") }
	store.complete(job.ID, nil)
	if monitor.Snapshot().Ready() {
		t.Fatal("failed completion not reflected immediately")
	}
	monitor.probe() // Scratch files work; re-persisting the actual job still fails.
	if monitor.Snapshot().Ready() || store.lastPersistenceErr == nil {
		t.Fatal("successful scratch probe concealed unresolved completion")
	}
	encoded, _ := json.Marshal(monitor.Snapshot())
	if strings.Contains(string(encoded), "sensitive") {
		t.Fatalf("raw persistence error leaked: %s", encoded)
	}
	store.syncDirectory = syncRelayJobDirectory
	monitor.probe()
	if !monitor.Snapshot().Ready() || store.lastPersistenceErr != nil {
		t.Fatalf("specific record was not recovered: %+v, %v", monitor.Snapshot(), store.lastPersistenceErr)
	}
	persisted, err := readPersistedRelayJobs(store.directory)
	if err != nil || len(persisted) != 1 || persisted[0].Status != relayJobSucceeded {
		t.Fatalf("completion not durably repaired: %+v, %v", persisted, err)
	}
}

func TestDurableRelayRejectedWriteRequiresStorageRecoveryEvenAfterCleanup(t *testing.T) {
	api, _ := newRelayReadinessTestAPI(t)
	prepareRelayReadinessProbe(api)
	monitor, store := api.relayJobReadiness, api.relayJobs
	monitor.probe()
	calls := 0
	store.syncDirectory = func(path string) error {
		calls++
		if calls == 1 {
			return errors.New("failed write sync")
		}
		return syncRelayJobDirectory(path)
	}
	if _, err := store.create("rejected", ""); !errors.Is(err, errRelayJobPersistence) {
		t.Fatalf("create error = %v", err)
	}
	if store.lastPersistenceErr != nil {
		t.Fatal("failed job cleanup unexpectedly unresolved")
	}
	if monitor.Snapshot().Ready() {
		t.Fatal("successful rejection cleanup concealed failed write")
	}
	monitor.probe()
	if !monitor.Snapshot().Ready() {
		t.Fatal("successful directory check failed to recover rejected write")
	}
}

func TestDurableRelayProbeCannotOverwriteAnInterveningRejectedWrite(t *testing.T) {
	api, _ := newRelayReadinessTestAPI(t)
	prepareRelayReadinessProbe(api)
	monitor, store := api.relayJobReadiness, api.relayJobs
	monitor.probe()
	monitor.check = func(path string) error {
		calls := 0
		store.syncDirectory = func(directory string) error {
			calls++
			if calls == 1 {
				return errors.New("write failed during directory check")
			}
			return syncRelayJobDirectory(directory)
		}
		if _, err := store.create("intervening-rejection", ""); !errors.Is(err, errRelayJobPersistence) {
			t.Fatalf("create error = %v", err)
		}
		return checkRelayJobDirectory(path)
	}
	monitor.probe()
	if monitor.Snapshot().Ready() {
		t.Fatal("scratch probe overwrote a newer rejected write")
	}
	monitor.check = checkRelayJobDirectory
	monitor.probe()
	if !monitor.Snapshot().Ready() {
		t.Fatal("later clean probe did not recover")
	}
}

func TestDurableRelayReadinessCapacityRespectsCompletedRecordRetention(t *testing.T) {
	api, _ := newRelayReadinessTestAPI(t)
	prepareRelayReadinessProbe(api)
	monitor, store := api.relayJobReadiness, api.relayJobs
	store.limit = 1
	monitor.probe()
	job, err := store.create("capacity", "")
	if err != nil {
		t.Fatal(err)
	}
	if monitor.Snapshot().Ready() {
		t.Fatal("full pending status capacity reported ready")
	}
	store.complete(job.ID, nil)
	if monitor.Snapshot().Ready() {
		t.Fatal("minimum status retention ignored")
	}
	store.mu.Lock()
	completed := store.jobs[job.ID]
	completed.retainUntil = time.Now().Add(-time.Second)
	store.jobs[job.ID] = completed
	store.refreshReadinessCapacityLocked()
	store.mu.Unlock()
	if !monitor.Snapshot().Ready() {
		t.Fatal("evictable completed status incorrectly caused backpressure")
	}
}

func TestDurableRelayReadinessDoesNotReopenFailedRecoveryStore(t *testing.T) {
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	directory := filepath.Join(server.GetMailDir(), ".owlmail-meta", "relay-jobs")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(invalid, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	api := NewAPIWithHTTPSDeferredRecovery(server, 1080, "127.0.0.1", "", "", false, "", "")
	if api.relayJobsPersistenceErr == nil {
		t.Fatal("corrupt recovery record accepted")
	}
	if err := os.Remove(invalid); err != nil {
		t.Fatal(err)
	}
	prepareRelayReadinessProbe(api)
	api.relayJobReadiness.probe()
	if api.relayJobReadiness.Snapshot().Ready() {
		t.Fatal("scratch probe silently reopened a failed recovery owner")
	}
}

func TestDurableRelayBlockedProbeKeepsCacheAndShutdownResponsive(t *testing.T) {
	api, _ := newRelayReadinessTestAPI(t)
	prepareRelayReadinessProbe(api)
	monitor := api.relayJobReadiness
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	monitor.check = func(string) error { close(entered); <-release; return nil }
	go func() { monitor.probe(); close(finished) }()
	<-entered
	if monitor.Snapshot().Ready() {
		t.Fatal("blocked probe retained healthy cache")
	}
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := api.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("blocked storage affected liveness")
	}
	closed := make(chan struct{})
	go func() { _ = monitor.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for blocked filesystem")
	}
	if monitor.Snapshot().State != attachmentstore.HealthClosed {
		t.Fatal("closed monitor not reflected")
	}
	close(release)
	<-finished
	if monitor.Snapshot().State != attachmentstore.HealthClosed {
		t.Fatal("finished probe replaced shutdown state")
	}
}

func TestDurableRelayHTTPReadinessFailsAndRecoversWhileLivenessStaysHealthy(t *testing.T) {
	api, server := newRelayReadinessTestAPI(t)
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "localhost", Port: 2525}); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	finished := make(chan error, 1)
	go func() { finished <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-finished:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP did not start")
	}
	prepareRelayReadinessProbe(api)
	api.relayJobReadiness.probe()
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, storage := server.GetServiceReadiness()
		if storage.Ready() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("local storage did not become ready")
		}
		time.Sleep(time.Millisecond)
	}
	check := func(path string, want int) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		resp, err := api.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s = %d want %d: %s", path, resp.StatusCode, want, body)
		}
		if path == "/readyz" && !strings.Contains(string(body), "relay_job_storage") {
			t.Fatal("durable readiness diagnostics missing")
		}
	}
	check("/readyz", http.StatusOK)
	job, err := api.relayJobs.create("ready-completion", "")
	if err != nil {
		t.Fatal(err)
	}
	api.relayJobs.syncDirectory = func(string) error { return errors.New("durable completion failed") }
	api.relayJobs.complete(job.ID, nil)
	check("/readyz", http.StatusServiceUnavailable)
	check("/healthz", http.StatusOK)
	api.relayJobs.syncDirectory = syncRelayJobDirectory
	api.relayJobReadiness.probe()
	check("/readyz", http.StatusOK)
	_ = server.Close()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP did not stop")
	}
}
