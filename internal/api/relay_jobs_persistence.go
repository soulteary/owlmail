package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maximumRelayJobFileBytes = 64 << 10

func newPersistentRelayJobStore(mailDirectory string) (*relayJobStore, error) {
	store := newRelayJobStore()
	if strings.TrimSpace(mailDirectory) == "" {
		return store, nil
	}
	store.directory = filepath.Join(mailDirectory, ".owlmail-meta", "relay-jobs")
	metadataDirectory := filepath.Dir(store.directory)
	metadataMissing, err := pathDoesNotExist(metadataDirectory)
	if err != nil {
		return nil, fmt.Errorf("inspect relay metadata directory: %w", err)
	}
	jobsMissing, err := pathDoesNotExist(store.directory)
	if err != nil {
		return nil, fmt.Errorf("inspect relay job directory: %w", err)
	}
	if err := os.MkdirAll(store.directory, 0700); err != nil {
		return nil, fmt.Errorf("create relay job directory: %w", err)
	}
	if jobsMissing {
		if err := store.syncDirectory(metadataDirectory); err != nil {
			return nil, fmt.Errorf("sync relay job directory creation: %w", err)
		}
	}
	if metadataMissing {
		if err := store.syncDirectory(mailDirectory); err != nil {
			return nil, fmt.Errorf("sync relay metadata directory creation: %w", err)
		}
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func pathDoesNotExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
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
	sort.Slice(jobs, func(left, right int) bool {
		if jobs[left].CreatedAt.Equal(jobs[right].CreatedAt) {
			return jobs[left].ID < jobs[right].ID
		}
		return jobs[left].CreatedAt.Before(jobs[right].CreatedAt)
	})
	now := store.now().UTC()
	queuedByEmail := make(map[string]string)
	for _, job := range jobs {
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > store.ttl {
			if err := store.removePersistedLocked(job.ID); err != nil {
				return fmt.Errorf("remove expired relay job %s: %w", job.ID, err)
			}
			continue
		}
		if job.CompletedAt != nil {
			job.retainUntil = job.CompletedAt.Add(store.minimumRetention)
		} else if originalID, duplicate := queuedByEmail[job.EmailID]; duplicate {
			completedAt := now
			job.Status = relayJobFailed
			job.ErrorCategory = "duplicate_pending"
			job.UpdatedAt = now
			job.CompletedAt = &completedAt
			job.NextAttemptAt = nil
			job.retainUntil = completedAt.Add(store.minimumRetention)
			if err := store.persistLocked(job); err != nil {
				return fmt.Errorf("mark duplicate relay job %s after %s: %w", job.ID, originalID, err)
			}
		} else {
			queuedByEmail[job.EmailID] = job.ID
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

func validateRelayJobPersistenceBudget(job relayJob) error {
	// Reserve enough space for every field that may be added as the job moves
	// through retry and terminal states. This prevents a successfully accepted
	// job from outgrowing the loader's fixed safety limit later in its lifecycle.
	worstTimestamp := time.Date(9999, time.December, 31, 23, 59, 59, 999999999, time.UTC)
	worst := job
	worst.Status = relayJobFailed
	worst.ErrorCategory = "configuration_changed"
	worst.UpdatedAt = worstTimestamp
	worst.CompletedAt = &worstTimestamp
	worst.NextAttemptAt = &worstTimestamp
	worst.Attempts = defaultRelayMaxAttempts
	data, err := json.Marshal(worst)
	if err != nil {
		return fmt.Errorf("%w: encode relay job: %v", errRelayJobTooLarge, err)
	}
	if len(data) > maximumRelayJobFileBytes {
		return fmt.Errorf("%w: encoded lifecycle state is %d bytes", errRelayJobTooLarge, len(data))
	}
	return nil
}

func (store *relayJobStore) persistLocked(job relayJob) error {
	if store.directory == "" {
		return nil
	}
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode relay job: %w", err)
	}
	if len(data) > maximumRelayJobFileBytes {
		return fmt.Errorf("encoded relay job exceeds %d bytes", maximumRelayJobFileBytes)
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
	return store.syncDirectory(store.directory)
}

func syncRelayJobDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open relay job directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync relay job directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close relay job directory: %w", err)
	}
	return nil
}

func (store *relayJobStore) removePersistedLocked(id string) error {
	if store.directory == "" {
		return nil
	}
	if err := os.Remove(filepath.Join(store.directory, id+".json")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove relay job: %w", err)
	}
	if err := store.syncDirectory(store.directory); err != nil {
		return fmt.Errorf("sync relay job removal: %w", err)
	}
	return nil
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
