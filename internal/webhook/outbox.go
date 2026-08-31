package webhook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const outboxDirectoryName = ".owlmail-webhook-outbox"

const (
	mailRollbackFencePrefix = ".owlmail-tmp-rollback-"
	mailRollbackFenceSuffix = ".fence"
	mailActiveState         = "active"
	mailRollbackState       = "rollback"
	mailAcceptedState       = "accepted"
)

type deliveryOutbox struct {
	dir           string
	mutex         sync.Mutex
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
	finalPath := filepath.Join(outbox.dir, job.ID+".pending")
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("commit webhook outbox job: %w", err)
	}
	if err := outbox.syncDirectory(outbox.dir); err != nil {
		// Keep the non-consumable pending entry. The caller can reject safely:
		// recovery promotes pending jobs only for durably accepted emails.
		committed = true
		return fmt.Errorf("sync pending webhook outbox job: %w", err)
	}
	committed = true
	return nil
}

// Commit promotes staged jobs for an accepted email into the consumable
// outbox namespace. The caller persists mail acceptance before calling this.
func (outbox *deliveryOutbox) Commit(emailID string) error {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	files, err := os.ReadDir(outbox.dir)
	if err != nil {
		return fmt.Errorf("read pending webhook outbox: %w", err)
	}
	promoted := false
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pending") {
			continue
		}
		pendingPath := filepath.Join(outbox.dir, file.Name())
		encoded, err := os.ReadFile(pendingPath)
		if err != nil {
			return fmt.Errorf("read pending webhook outbox job %s: %w", file.Name(), err)
		}
		var job deliveryJob
		if err := json.Unmarshal(encoded, &job); err != nil {
			return fmt.Errorf("decode pending webhook outbox job %s: %w", file.Name(), err)
		}
		if job.Email == nil || job.Email.ID != emailID {
			continue
		}
		committedPath := strings.TrimSuffix(pendingPath, ".pending") + ".json"
		if err := os.Rename(pendingPath, committedPath); err != nil {
			return fmt.Errorf("commit pending webhook outbox job: %w", err)
		}
		promoted = true
	}
	if promoted {
		if err := outbox.syncDirectory(outbox.dir); err != nil {
			return fmt.Errorf("sync committed webhook outbox job: %w", err)
		}
	}
	return cleanupAcceptedMailFence(filepath.Dir(outbox.dir), emailID)
}

// Discard removes staged jobs for a mail transaction that did not commit.
func (outbox *deliveryOutbox) Discard(emailID string) error {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	files, err := os.ReadDir(outbox.dir)
	if err != nil {
		return err
	}
	removed := false
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pending") {
			continue
		}
		path := filepath.Join(outbox.dir, file.Name())
		encoded, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var job deliveryJob
		if err := json.Unmarshal(encoded, &job); err != nil {
			return err
		}
		if job.Email == nil || job.Email.ID != emailID {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		removed = true
	}
	if removed {
		return outbox.syncDirectory(outbox.dir)
	}
	return nil
}

// AcceptedPendingEmailIDs returns durable recovery keys whose mail may already
// have been deleted. Only an accepted fence authorizes promotion.
func (outbox *deliveryOutbox) AcceptedPendingEmailIDs() ([]string, error) {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	files, err := os.ReadDir(outbox.dir)
	if err != nil {
		return nil, err
	}
	spoolDir := filepath.Dir(outbox.dir)
	accepted := make(map[string]struct{})
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pending") {
			continue
		}
		encoded, err := os.ReadFile(filepath.Join(outbox.dir, file.Name()))
		if err != nil {
			return nil, err
		}
		var job deliveryJob
		if err := json.Unmarshal(encoded, &job); err != nil {
			return nil, err
		}
		if job.Email == nil || job.Email.ID == "" || filepath.Base(job.Email.ID) != job.Email.ID {
			return nil, fmt.Errorf("invalid email ID in pending webhook outbox job %s", file.Name())
		}
		state, err := mailFenceState(spoolDir, job.Email.ID)
		if err != nil {
			return nil, err
		}
		if state == mailAcceptedState {
			accepted[job.Email.ID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(accepted))
	for id := range accepted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func cleanupAcceptedMailFence(spoolDir, emailID string) error {
	fencePath := filepath.Join(spoolDir, mailRollbackFencePrefix+emailID+mailRollbackFenceSuffix)
	state, err := mailFenceState(spoolDir, emailID)
	if err != nil {
		return err
	}
	if state != mailAcceptedState {
		return nil
	}
	if err := os.Remove(fencePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncOutboxDirectory(spoolDir)
}

func mailFenceState(spoolDir, emailID string) (string, error) {
	fencePath := filepath.Join(spoolDir, mailRollbackFencePrefix+emailID+mailRollbackFenceSuffix)
	encoded, err := os.ReadFile(fencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(encoded)), nil
}

// PruneRejectedPending removes staged jobs that can never be promoted because
// the matching mail transaction is explicitly and durably rollback-fenced.
func (outbox *deliveryOutbox) PruneRejectedPending() error {
	outbox.mutex.Lock()
	defer outbox.mutex.Unlock()
	files, err := os.ReadDir(outbox.dir)
	if err != nil {
		return fmt.Errorf("read pending webhook outbox: %w", err)
	}
	removed := false
	spoolDir := filepath.Dir(outbox.dir)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pending") {
			continue
		}
		pendingPath := filepath.Join(outbox.dir, file.Name())
		encoded, err := os.ReadFile(pendingPath)
		if err != nil {
			return fmt.Errorf("read pending webhook outbox job %s: %w", file.Name(), err)
		}
		var job deliveryJob
		if err := json.Unmarshal(encoded, &job); err != nil {
			return fmt.Errorf("decode pending webhook outbox job %s: %w", file.Name(), err)
		}
		if job.Email == nil || job.Email.ID == "" || filepath.Base(job.Email.ID) != job.Email.ID {
			return fmt.Errorf("invalid email ID in pending webhook outbox job %s", file.Name())
		}
		rejected, err := pendingMailRejected(spoolDir, job.Email.ID)
		if err != nil {
			return err
		}
		if !rejected {
			continue
		}
		if err := os.Remove(pendingPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove rejected pending webhook outbox job: %w", err)
		}
		removed = true
	}
	if removed {
		return outbox.syncDirectory(outbox.dir)
	}
	return nil
}

func pendingMailRejected(spoolDir, emailID string) (bool, error) {
	fencePath := filepath.Join(spoolDir, mailRollbackFencePrefix+emailID+mailRollbackFenceSuffix)
	if encoded, err := os.ReadFile(fencePath); err == nil {
		return strings.TrimSpace(string(encoded)) == mailRollbackState, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("read mail rollback fence for pending webhook job: %w", err)
	}
	return false, nil
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
	if err := os.Remove(path); err != nil {
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
