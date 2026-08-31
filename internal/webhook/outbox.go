package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const outboxDirectoryName = ".owlmail-webhook-outbox"

type deliveryOutbox struct {
	dir           string
	mutex         sync.Mutex
	removeFile    func(string) error
	syncDirectory func(string) error
}

type outboxEntry struct {
	path string
	job  deliveryJob
}

func newDeliveryOutbox(spoolDir string) (*deliveryOutbox, error) {
	dir := filepath.Join(spoolDir, outboxDirectoryName)
	outbox := &deliveryOutbox{
		dir:           dir,
		removeFile:    os.Remove,
		syncDirectory: syncOutboxDirectory,
	}
	if err := outbox.ensureDirectory(); err != nil {
		return nil, fmt.Errorf("create webhook outbox: %w", err)
	}
	return outbox, nil
}

func (outbox *deliveryOutbox) ensureDirectory() error {
	return os.MkdirAll(outbox.dir, 0750)
}

func (outbox *deliveryOutbox) Store(job deliveryJob) error {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	if err := outbox.ensureDirectory(); err != nil {
		return fmt.Errorf("create webhook outbox: %w", err)
	}
	encoded, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode webhook outbox job: %w", err)
	}
	temporary, err := os.CreateTemp(outbox.dir, ".pending-*.tmp")
	if err != nil {
		return fmt.Errorf("create webhook outbox job: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		return fmt.Errorf("secure webhook outbox job: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		return fmt.Errorf("write webhook outbox job: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync webhook outbox job: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close webhook outbox job: %w", err)
	}
	finalPath := filepath.Join(outbox.dir, job.ID+".json")
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("commit webhook outbox job: %w", err)
	}
	if err := outbox.syncDirectory(outbox.dir); err != nil {
		removeErr := outbox.removeFile(finalPath)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			// The entry is still visible to the worker, so resolve the handoff as
			// committed. Returning an error here would roll back the email while
			// allowing its webhook to be delivered.
			committed = true
			return nil
		}
		resyncErr := outbox.syncDirectory(outbox.dir)
		return errors.Join(
			fmt.Errorf("sync webhook outbox: %w", err),
			wrapOutboxCleanupError("sync webhook outbox cleanup", resyncErr),
		)
	}
	committed = true
	return nil
}

func wrapOutboxCleanupError(message string, err error) error {
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

func (outbox *deliveryOutbox) List() ([]outboxEntry, error) {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	if err := outbox.ensureDirectory(); err != nil {
		return nil, fmt.Errorf("create webhook outbox: %w", err)
	}
	files, err := os.ReadDir(outbox.dir)
	if err != nil {
		return nil, fmt.Errorf("read webhook outbox: %w", err)
	}
	entries := make([]outboxEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		path := filepath.Join(outbox.dir, file.Name())
		encoded, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read webhook outbox job %s: %w", file.Name(), err)
		}
		var job deliveryJob
		if err := json.Unmarshal(encoded, &job); err != nil {
			return nil, fmt.Errorf("decode webhook outbox job %s: %w", file.Name(), err)
		}
		entries = append(entries, outboxEntry{path: path, job: job})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].job.EnqueuedAt.Before(entries[j].job.EnqueuedAt)
	})
	return entries, nil
}

func (outbox *deliveryOutbox) Remove(path string) error {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	if filepath.Dir(path) != outbox.dir {
		return fmt.Errorf("remove webhook outbox job outside spool")
	}
	if err := outbox.removeFile(path); err != nil {
		return fmt.Errorf("remove webhook outbox job: %w", err)
	}
	return outbox.syncDirectory(outbox.dir)
}

func syncOutboxDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}
