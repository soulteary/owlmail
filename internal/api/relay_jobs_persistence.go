package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maximumRelayJobFileBytes = 64 << 10

func newPersistentRelayJobStore(mailDirectory string) (*relayJobStore, error) {
	store := newRelayJobStore()
	if strings.TrimSpace(mailDirectory) == "" {
		return store, nil
	}
	store.directory = filepath.Join(mailDirectory, ".owlmail-meta", "relay-jobs")
	if err := os.MkdirAll(store.directory, 0700); err != nil {
		return nil, fmt.Errorf("create relay job directory: %w", err)
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *relayJobStore) load() error {
	entries, err := os.ReadDir(store.directory)
	if err != nil {
		return fmt.Errorf("read relay job directory: %w", err)
	}
	jobs := make([]relayJob, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(store.directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect relay job %s: %w", entry.Name(), err)
		}
		if info.Size() > maximumRelayJobFileBytes {
			return fmt.Errorf("relay job %s exceeds %d bytes", entry.Name(), maximumRelayJobFileBytes)
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open relay job %s: %w", entry.Name(), err)
		}
		decoder := json.NewDecoder(io.LimitReader(file, maximumRelayJobFileBytes+1))
		decoder.DisallowUnknownFields()
		var job relayJob
		decodeErr := decoder.Decode(&job)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode relay job %s: %w", entry.Name(), decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close relay job %s: %w", entry.Name(), closeErr)
		}
		if entry.Name() != job.ID+".json" || !validPersistedRelayJob(job) {
			return fmt.Errorf("invalid relay job file %s", entry.Name())
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(left, right int) bool { return jobs[left].CreatedAt.Before(jobs[right].CreatedAt) })
	now := store.now().UTC()
	for _, job := range jobs {
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > store.ttl {
			store.removePersistedLocked(job.ID)
			continue
		}
		if job.CompletedAt != nil {
			job.retainUntil = job.CompletedAt.Add(store.minimumRetention)
		}
		if len(store.order) >= store.limit {
			return fmt.Errorf("persisted relay jobs exceed limit %d", store.limit)
		}
		store.jobs[job.ID] = job
		store.order = append(store.order, job.ID)
	}
	return nil
}

func validPersistedRelayJob(job relayJob) bool {
	if len(job.ID) != 32 || job.EmailID == "" || len(job.RelayTo) > defaultRelayRecipientMaxBytes || job.CreatedAt.IsZero() || job.UpdatedAt.IsZero() {
		return false
	}
	for _, char := range job.ID {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	switch job.Status {
	case relayJobQueued:
		return job.CompletedAt == nil
	case relayJobSucceeded, relayJobFailed:
		return job.CompletedAt != nil
	default:
		return false
	}
}

func (store *relayJobStore) persistLocked(job relayJob) error {
	if store.directory == "" {
		return nil
	}
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode relay job: %w", err)
	}
	temporary, err := os.CreateTemp(store.directory, ".relay-job-*")
	if err != nil {
		return fmt.Errorf("create relay job temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure relay job temporary file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write relay job: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync relay job: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close relay job: %w", err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(store.directory, job.ID+".json")); err != nil {
		return fmt.Errorf("commit relay job: %w", err)
	}
	return nil
}

func (store *relayJobStore) removePersistedLocked(id string) {
	if store.directory == "" {
		return
	}
	if err := os.Remove(filepath.Join(store.directory, id+".json")); err != nil && !os.IsNotExist(err) {
		// Removal is best effort during retention cleanup; a later startup prunes
		// the same completed record again.
		return
	}
}

func (store *relayJobStore) queued() []relayJob {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now().UTC())
	result := make([]relayJob, 0)
	for _, id := range store.order {
		if job := store.jobs[id]; job.Status == relayJobQueued {
			result = append(result, job)
		}
	}
	return result
}
