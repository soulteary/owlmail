package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/types"
)

// ErrDeliveryReplayConflict identifies an ineligible or already pending replay.
var ErrDeliveryReplayConflict = errors.New("webhook replay conflict")

// DeliveryOutcome exposes diagnostic metadata without destination URLs or credentials.
type DeliveryOutcome struct {
	Target        string `json:"target"`
	StatusCode    int    `json:"status_code"`
	Attempts      int    `json:"attempts"`
	ErrorCategory string `json:"error_category,omitempty"`
}

// DeliveryRecord is a bounded terminal delivery summary. Payloads are not retained.
type DeliveryRecord struct {
	ID          string            `json:"id"`
	EmailID     string            `json:"email_id"`
	RetryOf     string            `json:"retry_of,omitempty"`
	CompletedAt time.Time         `json:"completed_at"`
	Status      string            `json:"status"`
	Outcomes    []DeliveryOutcome `json:"outcomes"`
}

type deliveryHistory struct {
	mu             sync.Mutex
	records        map[string]DeliveryRecord
	path           string
	persistenceErr error
	recoveryErr    error
}

func newDeliveryHistory(spool string) (*deliveryHistory, error) {
	h := &deliveryHistory{records: make(map[string]DeliveryRecord)}
	if spool == "" {
		return h, nil
	}
	dir := filepath.Join(spool, ".owlmail-meta", "webhook-history")
	h.path = filepath.Join(dir, "history.json")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return h, err
	}
	f, err := os.Open(h.path)
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, 2<<20+1))
	if err != nil {
		return h, err
	}
	if len(data) > 2<<20 {
		return h.quarantine(fmt.Errorf("webhook history exceeds 2 MiB"))
	}
	var records []DeliveryRecord
	if err = json.Unmarshal(data, &records); err != nil {
		return h.quarantine(fmt.Errorf("invalid webhook history"))
	}
	if len(records) > 1000 {
		return h.quarantine(fmt.Errorf("webhook history exceeds record limit"))
	}
	for _, r := range records {
		h.records[r.ID] = r
	}
	h.prune()
	return h, nil
}
func (h *deliveryHistory) quarantine(cause error) (*deliveryHistory, error) {
	if err := os.Rename(h.path, h.path+".corrupt-"+uuid.NewString()); err != nil {
		return h, errors.Join(cause, err)
	}
	return h, cause
}

func (h *deliveryHistory) prune() {
	for id, r := range h.records {
		if time.Since(r.CompletedAt) > 24*time.Hour {
			delete(h.records, id)
		}
	}
	records := h.listLocked()
	encodedBytes := 2 // JSON array brackets
	for index, r := range records {
		encoded, _ := json.Marshal(r)
		size := len(encoded) + 1 // conservative comma allowance
		if index >= 1000 || encodedBytes+size > 2<<20 {
			delete(h.records, r.ID)
			continue
		}
		encodedBytes += size
	}
}
func (h *deliveryHistory) listLocked() []DeliveryRecord {
	r := make([]DeliveryRecord, 0, len(h.records))
	for _, v := range h.records {
		v.Outcomes = append([]DeliveryOutcome(nil), v.Outcomes...)
		r = append(r, v)
	}
	sort.Slice(r, func(i, j int) bool {
		if r[i].CompletedAt.Equal(r[j].CompletedAt) {
			return r[i].ID < r[j].ID
		}
		return r[i].CompletedAt.After(r[j].CompletedAt)
	})
	return r
}
func (h *deliveryHistory) record(job deliveryJob, results []Result) {
	h.mu.Lock()
	defer h.mu.Unlock()
	emailID := ""
	if job.Email != nil {
		emailID = job.Email.ID
	}
	r := DeliveryRecord{ID: job.ID, EmailID: emailID, RetryOf: job.RetryOf, CompletedAt: time.Now().UTC(), Status: "succeeded", Outcomes: make([]DeliveryOutcome, 0, len(results))}
	for _, result := range results {
		o := DeliveryOutcome{Target: result.Target, StatusCode: result.StatusCode, Attempts: result.Attempts}
		if result.Err != nil {
			r.Status = "failed"
			o.ErrorCategory = "transport"
			if result.Target == "" {
				o.ErrorCategory = "queue"
			} else if result.StatusCode != 0 {
				o.ErrorCategory = "http_status"
			}
		}
		r.Outcomes = append(r.Outcomes, o)
	}
	h.records[r.ID] = r
	h.prune()
	h.persistenceErr = h.persist()
}
func (h *deliveryHistory) persist() error {
	if h.path == "" {
		return nil
	}
	data, err := json.Marshal(h.listLocked())
	if err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return fmt.Errorf("webhook history exceeds 2 MiB")
	}
	f, err := os.CreateTemp(filepath.Dir(h.path), ".history-*")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), h.path); err != nil {
		return err
	}
	return syncOutboxDirectory(filepath.Dir(h.path))
}

// DeliveryHistory returns detached terminal summaries and persistence health.
func (service *Service) DeliveryHistory() ([]DeliveryRecord, bool) {
	service.history.mu.Lock()
	defer service.history.mu.Unlock()
	service.history.prune()
	return service.history.listLocked(), service.history.persistenceErr == nil && service.history.recoveryErr == nil
}

// RetryDelivery replays only failed configured targets using the original idempotency key.
// The caller must supply a retained, detached email snapshot with the same ID.
func (service *Service) RetryDelivery(id string, email *types.Email) error {
	service.handoffMutex.Lock()
	if !service.accepting {
		service.handoffMutex.Unlock()
		return fmt.Errorf("webhook service is shutting down")
	}
	service.handoffs.Add(1)
	service.handoffMutex.Unlock()
	defer service.handoffs.Done()
	service.history.mu.Lock()
	record, ok := service.history.records[id]
	service.history.mu.Unlock()
	if !ok || record.Status != "failed" {
		return fmt.Errorf("%w: delivery is not a retained failed record", ErrDeliveryReplayConflict)
	}
	if email == nil || email.ID != record.EmailID {
		return fmt.Errorf("%w: email source is unavailable", ErrDeliveryReplayConflict)
	}
	if _, loaded := service.replays.LoadOrStore(id, true); loaded {
		return fmt.Errorf("%w: delivery replay already pending", ErrDeliveryReplayConflict)
	}
	queued := false
	defer func() {
		if !queued {
			service.replays.Delete(id)
		}
	}()
	targets := make([]string, 0)
	for _, o := range record.Outcomes {
		if o.ErrorCategory != "" && o.Target != "" {
			targets = append(targets, o.Target)
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("%w: queue-only failures must recover through the queue", ErrDeliveryReplayConflict)
	}
	for _, name := range targets {
		found := false
		for _, t := range service.dispatcher.targets {
			found = found || t.name == name
		}
		if !found {
			return fmt.Errorf("%w: failed target is no longer configured", ErrDeliveryReplayConflict)
		}
	}
	snapshot, err := cloneDeliveryEmail(email)
	if err != nil {
		return err
	}
	job := deliveryJob{ID: id, RetryOf: id, EnqueuedAt: time.Now().UTC(), Email: snapshot, Targets: targets}
	if service.outbox != nil {
		if err = service.outbox.Store(job); err != nil {
			return err
		}
		if err = service.outbox.Commit(email.ID); err != nil {
			return err
		}
		service.wakeOutbox()
		queued = true
		return nil
	}
	if service.queue != nil {
		err = service.queue.Enqueue(service.ctx, job)
		queued = err == nil
		return err
	}
	select {
	case service.memoryQueue <- job:
		queued = true
		return nil
	default:
		return errors.New("webhook queue is full")
	}
}
