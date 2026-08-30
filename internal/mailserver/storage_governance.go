package mailserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/soulteary/owlmail/internal/common"
)

const metadataDirectoryName = ".owlmail-meta"

// StoragePolicy bounds the on-disk and in-memory mailbox.
type StoragePolicy struct {
	MaxAge          time.Duration
	MaxMessages     int
	MaxDiskBytes    int64
	CleanupInterval time.Duration
}

// StorageMetrics reports retention activity through the existing stats API.
type StorageMetrics struct {
	CleanupRuns      uint64    `json:"cleanupRuns"`
	DeletedMessages  uint64    `json:"deletedMessages"`
	ReclaimedBytes   uint64    `json:"reclaimedBytes"`
	LastCleanupAt    time.Time `json:"lastCleanupAt,omitempty"`
	LastCleanupError string    `json:"lastCleanupError,omitempty"`
}

type emailMetadata struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Read     bool      `json:"read"`
	Sequence time.Time `json:"sequence"`
}

// ConfigureStoragePolicy applies limits immediately and starts periodic cleanup.
func (ms *MailServer) ConfigureStoragePolicy(policy StoragePolicy) error {
	if policy.MaxAge < 0 || policy.MaxMessages < 0 || policy.MaxDiskBytes < 0 {
		return fmt.Errorf("storage limits cannot be negative")
	}
	if policy.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}
	if ms.cleanupCancel != nil {
		ms.cleanupCancel()
		ms.cleanupWG.Wait()
		ms.cleanupCancel = nil
	}
	ms.storagePolicy = policy
	if err := ms.CleanupStorage(); err != nil {
		return err
	}
	if policy.MaxAge == 0 && policy.MaxMessages == 0 && policy.MaxDiskBytes == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ms.cleanupCancel = cancel
	ms.cleanupWG.Add(1)
	go func() {
		defer ms.cleanupWG.Done()
		ticker := time.NewTicker(policy.CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := ms.CleanupStorage(); err != nil {
					common.Error("Storage cleanup failed: %v", err)
				}
			}
		}
	}()
	return nil
}

// CleanupStorage removes oldest messages until every configured limit is met.
func (ms *MailServer) CleanupStorage() error {
	emails := ms.GetAllEmail()
	type candidate struct {
		id   string
		size int64
		time time.Time
	}
	items := make([]candidate, 0, len(emails))
	var totalBytes int64
	for _, email := range emails {
		size, err := ms.emailDiskUsage(email.ID)
		if err != nil {
			ms.recordCleanup(0, 0, err)
			return err
		}
		items = append(items, candidate{id: email.ID, size: size, time: ms.receivedAt(email.ID)})
		totalBytes += size
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].time.Equal(items[j].time) {
			return items[i].id < items[j].id
		}
		return items[i].time.Before(items[j].time)
	})

	remove := make(map[string]candidate)
	if ms.storagePolicy.MaxAge > 0 {
		cutoff := time.Now().Add(-ms.storagePolicy.MaxAge)
		for _, item := range items {
			if item.time.Before(cutoff) {
				remove[item.id] = item
				totalBytes -= item.size
			}
		}
	}
	remaining := len(items) - len(remove)
	for _, item := range items {
		if _, selected := remove[item.id]; selected {
			continue
		}
		overCount := ms.storagePolicy.MaxMessages > 0 && remaining > ms.storagePolicy.MaxMessages
		overDisk := ms.storagePolicy.MaxDiskBytes > 0 && totalBytes > ms.storagePolicy.MaxDiskBytes
		if !overCount && !overDisk {
			break
		}
		remove[item.id] = item
		remaining--
		totalBytes -= item.size
	}

	var deleted uint64
	var reclaimed uint64
	for _, item := range items {
		if _, selected := remove[item.id]; !selected {
			continue
		}
		if err := ms.DeleteEmail(item.id); err != nil {
			ms.recordCleanup(deleted, reclaimed, err)
			return err
		}
		deleted++
		reclaimed += uint64(item.size)
	}
	ms.recordCleanup(deleted, reclaimed, nil)
	return nil
}

func (ms *MailServer) recordCleanup(deleted, reclaimed uint64, cleanupErr error) {
	ms.storageMetricsMutex.Lock()
	defer ms.storageMetricsMutex.Unlock()
	ms.storageMetrics.CleanupRuns++
	ms.storageMetrics.DeletedMessages += deleted
	ms.storageMetrics.ReclaimedBytes += reclaimed
	ms.storageMetrics.LastCleanupAt = time.Now().UTC()
	if cleanupErr != nil {
		ms.storageMetrics.LastCleanupError = cleanupErr.Error()
	} else {
		ms.storageMetrics.LastCleanupError = ""
	}
}

func (ms *MailServer) storageStats() map[string]interface{} {
	ms.storageMetricsMutex.RLock()
	metrics := ms.storageMetrics
	ms.storageMetricsMutex.RUnlock()
	diskBytes, _ := ms.mailboxDiskUsage()
	return map[string]interface{}{
		"diskBytes": diskBytes, "maxAgeSeconds": int64(ms.storagePolicy.MaxAge.Seconds()),
		"maxMessages": ms.storagePolicy.MaxMessages, "maxDiskBytes": ms.storagePolicy.MaxDiskBytes,
		"cleanupRuns": metrics.CleanupRuns, "deletedMessages": metrics.DeletedMessages,
		"reclaimedBytes": metrics.ReclaimedBytes, "lastCleanupAt": metrics.LastCleanupAt,
		"lastCleanupError": metrics.LastCleanupError,
	}
}

func (ms *MailServer) mailboxDiskUsage() (int64, error) {
	emails := ms.GetAllEmail()
	var total int64
	for _, email := range emails {
		size, err := ms.emailDiskUsage(email.ID)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func (ms *MailServer) emailDiskUsage(id string) (int64, error) {
	var total int64
	if stat, err := os.Stat(filepath.Join(ms.mailDir, id+".eml")); err == nil {
		total += stat.Size()
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if stat, err := os.Stat(ms.metadataPath(id)); err == nil {
		total += stat.Size()
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	attachmentDir := filepath.Join(ms.mailDir, id)
	err := filepath.WalkDir(attachmentDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			stat, err := entry.Info()
			if err != nil {
				return err
			}
			total += stat.Size()
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	return total, nil
}

func (ms *MailServer) metadataPath(id string) string {
	return filepath.Join(ms.mailDir, metadataDirectoryName, id+".json")
}

func (ms *MailServer) receivedAt(id string) time.Time {
	ms.storeMutex.RLock()
	receivedAt := ms.receivedAtByID[id]
	ms.storeMutex.RUnlock()
	if receivedAt.IsZero() {
		return time.Now().UTC()
	}
	return receivedAt
}

func (ms *MailServer) persistEmailMetadata(email *Email) error {
	if email == nil {
		return nil
	}
	return ms.persistEmailMetadataAt(email, ms.receivedAt(email.ID))
}

func (ms *MailServer) persistEmailMetadataAt(email *Email, receivedAt time.Time) error {
	if email == nil {
		return nil
	}
	dir := filepath.Join(ms.mailDir, metadataDirectoryName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	metadata := emailMetadata{Version: 1, ID: email.ID, Read: email.Read, Sequence: receivedAt}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, ms.metadataPath(email.ID)); err != nil {
		return err
	}
	return syncStorageDirectory(dir)
}

func (ms *MailServer) loadEmailMetadata(id string) (emailMetadata, error) {
	encoded, err := os.ReadFile(ms.metadataPath(id))
	if err != nil {
		return emailMetadata{}, err
	}
	var metadata emailMetadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return emailMetadata{}, err
	}
	if metadata.Version != 1 || metadata.ID != id {
		return emailMetadata{}, fmt.Errorf("invalid metadata for %s", id)
	}
	return metadata, nil
}

func (ms *MailServer) deleteEmailMetadata(id string) error {
	err := os.Remove(ms.metadataPath(id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func syncStorageDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
