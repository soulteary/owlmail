package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceConcurrencyLimitAppliesToIndividualTargets(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	dispatcher, err := NewDispatcher(Config{Targets: []Target{
		{Name: "first", URL: receiver.URL},
		{Name: "second", URL: receiver.URL},
	}}, receiver.Client())
	if err != nil {
		t.Fatal(err)
	}
	completed := make(chan []Result, 1)
	service, err := NewService(dispatcher, ServiceOptions{
		MaxConcurrency:  1,
		ShutdownTimeout: time.Second,
		OnResults: func(_ string, results []Result) {
			completed <- results
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	if err := service.Enqueue(testEmail()); err != nil {
		t.Fatal(err)
	}
	select {
	case results := <-completed:
		if len(results) != 2 {
			t.Fatalf("result count = %d, want 2", len(results))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deliveries")
	}
	if got := maximum.Load(); got != 1 {
		t.Fatalf("maximum concurrent target requests = %d, want 1", got)
	}
}

func TestServiceCloseDrainsInFlightDelivery(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "drain", URL: receiver.URL}}}, receiver.Client())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(dispatcher, ServiceOptions{MaxConcurrency: 1, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Enqueue(testEmail()); err != nil {
		t.Fatal(err)
	}
	<-started
	closed := make(chan error, 1)
	go func() { closed <- service.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned before delivery completed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := service.Enqueue(testEmail()); err == nil {
		t.Fatal("enqueue after close should fail")
	}
}

type fakeDeliveryQueue struct {
	claims chan *queueReceipt
	mu     sync.Mutex
	acked  []string
	dead   []string
	count  int64
}

func (queue *fakeDeliveryQueue) Enqueue(_ context.Context, job deliveryJob) error {
	queue.mu.Lock()
	queue.count++
	queue.mu.Unlock()
	queue.claims <- &queueReceipt{id: job.ID, job: job}
	return nil
}

func (queue *fakeDeliveryQueue) Claim(ctx context.Context) (*queueReceipt, error) {
	select {
	case receipt := <-queue.claims:
		return receipt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Millisecond):
		return nil, nil
	}
}

func (queue *fakeDeliveryQueue) Ack(_ context.Context, receipt *queueReceipt) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.acked = append(queue.acked, receipt.id)
	queue.count--
	return nil
}

func (queue *fakeDeliveryQueue) DeadLetter(_ context.Context, receipt *queueReceipt, _ []Result) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.dead = append(queue.dead, receipt.id)
	queue.count--
	return nil
}

func (queue *fakeDeliveryQueue) Pending(_ context.Context) (int64, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.count, nil
}

func (queue *fakeDeliveryQueue) Close() error { return nil }

func TestDurableServiceDeadLettersExhaustedDelivery(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-OwlMail-Delivery-ID") != "replayed-job" {
			t.Errorf("delivery ID = %q", request.Header.Get("X-OwlMail-Delivery-ID"))
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer receiver.Close()
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "failing", URL: receiver.URL}}}, receiver.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	queue := &fakeDeliveryQueue{claims: make(chan *queueReceipt, 1), count: 1}
	queue.claims <- &queueReceipt{id: "redis-entry", job: deliveryJob{ID: "replayed-job", Email: testEmail()}}
	service := &Service{
		dispatcher: dispatcher, queue: queue, ctx: ctx, cancel: cancel,
		shutdownTimeout: time.Second, workerCount: 1, accepting: true,
	}
	service.start()
	deadline := time.Now().Add(time.Second)
	for {
		queue.mu.Lock()
		dead := append([]string(nil), queue.dead...)
		queue.mu.Unlock()
		if len(dead) == 1 {
			if dead[0] != "redis-entry" {
				t.Fatalf("dead letter IDs = %v", dead)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed job was not dead-lettered")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisQueueIntegration(t *testing.T) {
	redisURL := testRedisURL()
	if redisURL == "" {
		t.Skip("set OWLMAIL_TEST_REDIS_URL to run Redis Streams integration coverage")
	}
	ctx := context.Background()
	queue, err := newRedisDeliveryQueue(ctx, redisURL, "owlmail:test", "test-consumer")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = queue.Close() }()
	job := deliveryJob{ID: "integration-job", EnqueuedAt: time.Now().UTC(), Email: testEmail()}
	if err := queue.Enqueue(ctx, job); err != nil {
		t.Fatal(err)
	}
	receipt, err := queue.Claim(ctx)
	if err != nil || receipt == nil || receipt.job.ID != job.ID {
		t.Fatalf("Claim() = %#v, %v", receipt, err)
	}
	if err := queue.Ack(ctx, receipt); err != nil {
		t.Fatal(err)
	}
}

func testRedisURL() string {
	return os.Getenv("OWLMAIL_TEST_REDIS_URL")
}
