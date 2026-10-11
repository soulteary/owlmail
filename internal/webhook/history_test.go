package webhook

import (
	"net/http"
	"net/http/httptest"
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
