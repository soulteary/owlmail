package webhook

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFailedTargetReplayAndHistoryRecovery(t *testing.T) {
	var good, bad atomic.Int32
	var recovered atomic.Bool
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good" {
			good.Add(1)
			w.WriteHeader(204)
			return
		}
		bad.Add(1)
		if recovered.Load() {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(503)
		}
	}))
	defer receiver.Close()
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "good", URL: receiver.URL + "/good"}, {Name: "bad", URL: receiver.URL + "/bad"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	spool := t.TempDir()
	done := make(chan string, 2)
	service, err := NewService(dispatcher, ServiceOptions{SpoolDir: spool, MaxConcurrency: 1, OnResults: func(id string, _ []Result) { done <- id }})
	if err != nil {
		t.Fatal(err)
	}
	email := testEmail()
	if err = service.Enqueue(email); err != nil {
		t.Fatal(err)
	}
	if err = service.Commit(email.ID); err != nil {
		t.Fatal(err)
	}
	var id string
	select {
	case id = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("delivery timeout")
	}
	records, healthy := service.DeliveryHistory()
	if !healthy || len(records) != 1 || records[0].Status != "failed" {
		t.Fatal(records, healthy)
	}
	if err = service.Close(); err != nil {
		t.Fatal(err)
	}
	service, err = NewService(dispatcher, ServiceOptions{SpoolDir: spool, MaxConcurrency: 1, OnResults: func(id string, _ []Result) { done <- id }})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	recovered.Store(true)
	if err = service.RetryDelivery(id, email); err != nil {
		t.Fatal(err)
	}
	select {
	case replay := <-done:
		if replay != id {
			t.Fatal("changed idempotency key")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("replay timeout")
	}
	if good.Load() != 1 || bad.Load() != 2 {
		t.Fatal("replayed successful target", good.Load(), bad.Load())
	}
	records, healthy = service.DeliveryHistory()
	if !healthy || records[0].Status != "succeeded" {
		t.Fatal(records)
	}
	if err = service.RetryDelivery(id, email); err == nil {
		t.Fatal("replayed successful record")
	}
}

func TestHistoryPersistenceFailureIsVisible(t *testing.T) {
	h, err := newDeliveryHistory("")
	if err != nil {
		t.Fatal(err)
	}
	h.path = t.TempDir() // A directory cannot be replaced with a history file.
	h.record(deliveryJob{ID: "delivery", Email: testEmail()}, []Result{{Target: "target"}})
	service := &Service{history: h}
	records, healthy := service.DeliveryHistory()
	if healthy || len(records) != 1 {
		t.Fatal(records, healthy)
	}
}

func TestMalformedJobsAndDiagnosticHistoryDoNotCrashService(t *testing.T) {
	h, err := newDeliveryHistory("")
	if err != nil {
		t.Fatal(err)
	}
	h.record(deliveryJob{ID: "malformed"}, []Result{{Err: errors.New("email is nil")}})
	if h.records["malformed"].Status != "failed" || h.records["malformed"].EmailID != "" {
		t.Fatal(h.records)
	}
	spool := t.TempDir()
	dir := filepath.Join(spool, ".owlmail-meta", "webhook-history")
	if err = os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "history.json"), []byte(`{"truncated":`), 0600); err != nil {
		t.Fatal(err)
	}
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "target", URL: "http://127.0.0.1:1"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(dispatcher, ServiceOptions{SpoolDir: spool, MaxConcurrency: 1})
	if err != nil {
		t.Fatal("diagnostics prevented startup", err)
	}
	defer func() { _ = service.Close() }()
	_, healthy := service.DeliveryHistory()
	if healthy {
		t.Fatal("hidden corrupt history")
	}
	backups, err := filepath.Glob(filepath.Join(dir, "history.json.corrupt-*"))
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
}

func TestHistoryPrunesOldestRecordsToEncodedBudget(t *testing.T) {
	spool := t.TempDir()
	history, err := newDeliveryHistory(spool)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 1000; i++ {
		outcomes := make([]DeliveryOutcome, 32)
		for index := range outcomes {
			outcomes[index] = DeliveryOutcome{Target: strings.Repeat("t", 128), StatusCode: 503, Attempts: 3, ErrorCategory: "http_status"}
		}
		id := fmt.Sprintf("job-%04d", i)
		history.records[id] = DeliveryRecord{ID: id, EmailID: "source", CompletedAt: now.Add(time.Duration(i) * time.Millisecond), Status: "failed", Outcomes: outcomes}
	}
	history.prune()
	if len(history.records) >= 1000 || len(history.records) == 0 {
		t.Fatal(len(history.records))
	}
	if _, ok := history.records["job-0999"]; !ok {
		t.Fatal("newest diagnostic was pruned")
	}
	if err := history.persist(); err != nil {
		t.Fatal(err)
	}
	recovered, err := newDeliveryHistory(spool)
	if err != nil || len(recovered.records) != len(history.records) {
		t.Fatal(err, len(recovered.records), len(history.records))
	}
}
