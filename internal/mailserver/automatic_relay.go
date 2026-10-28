package mailserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/relay"
)

const (
	relayIntentMetadataDirectoryName = "relay-intents"
	maximumAutomaticRelayIntentBytes = 64 << 10
	maximumPendingAutomaticRelays    = 1000
	automaticRelayPrepared           = "prepared"
	automaticRelaySubmitted          = "submitted"
)

type automaticRelayIntent struct {
	Version         int       `json:"version"`
	EmailID         string    `json:"emailId"`
	State           string    `json:"state"`
	RelayTo         string    `json:"relayTo,omitempty"`
	Recipients      []string  `json:"recipients"`
	Envelope        *Envelope `json:"envelope"`
	SMTPTransaction bool      `json:"smtpTransaction,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	JobID           string    `json:"jobId,omitempty"`
}

type automaticRelayManager struct {
	server        *MailServer
	directory     string
	mu            sync.Mutex
	pending       map[string]automaticRelayIntent
	acknowledged  []automaticRelayIntent
	syncDirectory func(string) error
	unhealthy     atomic.Bool
	pendingCount  atomic.Int64
	startOnce     sync.Once
	closeOnce     sync.Once
	started       bool
	closed        bool
	wake          chan struct{}
	stop          chan struct{}
	done          chan struct{}
}

func newAutomaticRelayManager(server *MailServer) (*automaticRelayManager, error) {
	manager := &automaticRelayManager{server: server, directory: filepath.Join(server.mailDir, metadataDirectoryName, relayIntentMetadataDirectoryName), syncDirectory: syncDirectory, pending: make(map[string]automaticRelayIntent), wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	entries, err := os.ReadDir(manager.directory)
	if errors.Is(err, os.ErrNotExist) {
		return manager, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read automatic relay intents: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".intent-") && strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, fmt.Errorf("unexpected automatic relay intent %q", entry.Name())
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if err := validateEmailID(id); err != nil {
			return nil, fmt.Errorf("invalid automatic relay intent ID: %w", err)
		}
		intent, err := manager.read(id)
		if err != nil {
			return nil, err
		}
		if intent.State == automaticRelaySubmitted {
			manager.acknowledged = append(manager.acknowledged, intent)
		}
		if intent.State == automaticRelayPrepared {
			manager.pending[id] = intent
			manager.pendingCount.Store(int64(len(manager.pending)))
			if len(manager.pending) > maximumPendingAutomaticRelays {
				return nil, fmt.Errorf("automatic relay pending intent limit exceeded")
			}
		}
	}
	return manager, nil
}

func (manager *automaticRelayManager) read(id string) (automaticRelayIntent, error) {
	var intent automaticRelayIntent
	path := filepath.Join(manager.directory, id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return intent, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximumAutomaticRelayIntentBytes {
		return intent, fmt.Errorf("automatic relay intent %s is not a bounded regular file", id)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return intent, fmt.Errorf("automatic relay intent %s is not private", id)
	}
	file, err := os.Open(path)
	if err != nil {
		return intent, err
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maximumAutomaticRelayIntentBytes+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return intent, err
	}
	if len(encoded) > maximumAutomaticRelayIntentBytes {
		return intent, fmt.Errorf("automatic relay intent %s exceeds size limit", id)
	}
	if err := json.Unmarshal(encoded, &intent); err != nil {
		return intent, fmt.Errorf("parse automatic relay intent %s: %w", id, err)
	}
	if intent.Version != 1 || intent.EmailID != id || intent.CreatedAt.IsZero() || intent.Envelope == nil || len(intent.Recipients) == 0 || (intent.State != automaticRelayPrepared && intent.State != automaticRelaySubmitted) {
		return intent, fmt.Errorf("invalid automatic relay intent %s", id)
	}
	if intent.State == automaticRelaySubmitted && intent.JobID != relay.RequestID("auto", id) {
		return intent, fmt.Errorf("invalid automatic relay acknowledgement %s", id)
	}
	for _, recipient := range intent.Recipients {
		if strings.TrimSpace(recipient) == "" {
			return intent, fmt.Errorf("empty automatic relay recipient %s", id)
		}
	}
	intent.Envelope.SMTPTransaction = intent.SMTPTransaction
	return intent, nil
}

// Sync the directory links before acknowledging an intent. Repeat the parent
// syncs even when directories already exist: a previous attempt may have created
// them and then failed to make those links durable.
func (manager *automaticRelayManager) ensureDirectory() error {
	if err := os.MkdirAll(manager.directory, 0700); err != nil {
		return err
	}
	parent := filepath.Dir(manager.directory)
	if err := manager.syncDirectory(parent); err != nil {
		return err
	}
	return manager.syncDirectory(filepath.Dir(parent))
}

func (manager *automaticRelayManager) persist(intent automaticRelayIntent) error {
	encoded, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	if len(encoded) > maximumAutomaticRelayIntentBytes {
		return fmt.Errorf("automatic relay intent exceeds durable size limit")
	}
	if err := manager.ensureDirectory(); err != nil {
		return err
	}
	file, err := os.CreateTemp(manager.directory, ".intent-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, filepath.Join(manager.directory, intent.EmailID+".json")); err != nil {
		return err
	}
	return manager.syncDirectory(manager.directory)
}

// prepare runs inside the mailbox transaction before its acceptance fence.
func (manager *automaticRelayManager) prepare(email *Email) (bool, error) {
	config := manager.server.GetOutgoingConfig()
	if config == nil || config.Host == "" || !config.AutoRelay {
		return false, nil
	}
	var recipients []string
	relayTo := config.AutoRelayAddr
	if relayTo != "" {
		recipients = []string{relayTo}
	} else {
		var err error
		recipients, err = manager.server.EffectiveRelayRecipients(email)
		if err != nil {
			return false, err
		}
	}
	// Filtering every recipient out is an intentional no-delivery decision.
	if len(recipients) == 0 {
		return false, nil
	}
	intent := automaticRelayIntent{Version: 1, EmailID: email.ID, State: automaticRelayPrepared, RelayTo: relayTo, Recipients: append([]string(nil), recipients...), Envelope: cloneEnvelope(email.Envelope), CreatedAt: time.Now().UTC(), SMTPTransaction: email.Envelope != nil && email.Envelope.SMTPTransaction}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed {
		return false, fmt.Errorf("automatic relay handoff is closed")
	}
	if _, err := os.Lstat(filepath.Join(manager.directory, email.ID+".json")); err == nil {
		// A committed acknowledgement is retained with the message, independently
		// of bounded job-history retention. Reloading never creates fresh delivery.
		existing, readErr := manager.read(email.ID)
		if readErr != nil {
			return false, readErr
		}
		if existing.State != automaticRelaySubmitted {
			return false, fmt.Errorf("automatic relay handoff already pending")
		}
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		manager.unhealthy.Store(true)
		return false, err
	}
	if len(manager.pending) >= maximumPendingAutomaticRelays {
		return false, fmt.Errorf("automatic relay intent capacity reached")
	}
	if err := manager.persist(intent); err != nil {
		// A failed post-rename sync must not leave an apparently accepted intent
		// after a rejected non-SMTP SaveEmailToStore transaction.
		cleanupErr := os.Remove(filepath.Join(manager.directory, email.ID+".json"))
		if errors.Is(cleanupErr, os.ErrNotExist) {
			cleanupErr = nil
		}
		manager.unhealthy.Store(true)
		return false, errors.Join(fmt.Errorf("persist automatic relay intent: %w", err), cleanupErr)
	}
	manager.pending[email.ID] = intent
	manager.pendingCount.Store(int64(len(manager.pending)))
	return true, nil
}

func (manager *automaticRelayManager) abort(id string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := os.Remove(filepath.Join(manager.directory, id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		manager.unhealthy.Store(true)
		return err
	}
	if err := syncDirectoryIfExists(manager.directory); err != nil {
		manager.unhealthy.Store(true)
		return err
	}
	delete(manager.pending, id)
	manager.pendingCount.Store(int64(len(manager.pending)))
	if len(manager.pending) == 0 {
		manager.unhealthy.Store(false)
	}
	return nil
}

func (manager *automaticRelayManager) notify() {
	select {
	case manager.wake <- struct{}{}:
	default:
	}
}
func (manager *automaticRelayManager) Start() {
	manager.startOnce.Do(func() {
		manager.mu.Lock()
		if manager.closed {
			manager.mu.Unlock()
			return
		}
		manager.started = true
		manager.mu.Unlock()
		go manager.run()
	})
}
func (manager *automaticRelayManager) Close() error {
	manager.closeOnce.Do(func() {
		manager.mu.Lock()
		manager.closed = true
		started := manager.started
		manager.mu.Unlock()
		close(manager.stop)
		if started {
			<-manager.done
		}
	})
	return nil
}
func (manager *automaticRelayManager) run() {
	defer close(manager.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		manager.promotePending()
		select {
		case <-manager.stop:
			return
		case <-manager.wake:
		case <-ticker.C:
		}
	}
}
func (manager *automaticRelayManager) promotePending() {
	manager.mu.Lock()
	pending := make([]automaticRelayIntent, 0, len(manager.pending))
	for _, intent := range manager.pending {
		pending = append(pending, intent)
	}
	manager.mu.Unlock()
	for _, intent := range pending {
		select {
		case <-manager.stop:
			return
		default:
		}
		if err := manager.promote(intent); err != nil {
			manager.mu.Lock()
			manager.unhealthy.Store(true)
			manager.mu.Unlock()
			common.Verbose("Automatic relay handoff for %s remains pending: %v", intent.EmailID, err)
		}
	}
}
func (manager *automaticRelayManager) promote(intent automaticRelayIntent) error {
	if intent.State == automaticRelaySubmitted {
		return manager.commitAcknowledged(intent)
	}
	// Acquire the transaction lock only for the commit decision. The service's
	// Submit acquires its own source lease and must never run under this lock.
	manager.server.storageTransactionMutex.Lock()
	state, err := readRollbackFenceState(rollbackFencePath(manager.server.mailDir, intent.EmailID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		manager.server.storageTransactionMutex.Unlock()
		return err
	}
	rejected := err == nil && state != acceptedFenceState && state != localFenceState
	_, deleteErr := os.Lstat(deletionFencePath(manager.server.mailDir, intent.EmailID))
	_, sourceErr := os.Lstat(filepath.Join(manager.server.mailDir, intent.EmailID+".eml"))
	manager.server.storageTransactionMutex.Unlock()
	if rejected || deleteErr == nil || errors.Is(sourceErr, os.ErrNotExist) {
		return manager.abort(intent.EmailID)
	}
	if sourceErr != nil {
		return sourceErr
	}
	if !errors.Is(deleteErr, os.ErrNotExist) {
		return deleteErr
	}
	service := manager.server.RelayService()
	if service == nil {
		return fmt.Errorf("relay service is unavailable")
	}
	job, err := service.Submit(relay.Request{EmailID: intent.EmailID, RelayTo: intent.RelayTo, ConfirmedRecipients: append([]string(nil), intent.Recipients...), Origin: "auto", Key: intent.EmailID, Envelope: cloneEnvelope(intent.Envelope), DeferDispatch: true})
	if err != nil {
		return err
	}
	intent.State = automaticRelaySubmitted
	intent.JobID = job.ID

	manager.mu.Lock()
	if err := manager.persist(intent); err != nil {
		manager.unhealthy.Store(true)
		manager.mu.Unlock()
		return err
	}
	// From this point onward, even an indeterminate release write can only be
	// reconciled by job ID. Never recreate a delivery from a submitted intent.
	manager.pending[intent.EmailID] = intent
	manager.mu.Unlock()
	return manager.commitAcknowledged(intent)
}

func (manager *automaticRelayManager) recoverAcknowledged(service *relay.Service) {
	for _, intent := range manager.acknowledged {
		if err := service.CommitHandoff(intent.JobID); err != nil && !errors.Is(err, relay.ErrNotFound) {
			manager.mu.Lock()
			manager.pending[intent.EmailID] = intent
			manager.pendingCount.Store(int64(len(manager.pending)))
			manager.mu.Unlock()
			manager.unhealthy.Store(true)
		}
	}
	manager.acknowledged = nil
}

func (manager *automaticRelayManager) commitAcknowledged(intent automaticRelayIntent) error {
	service := manager.server.RelayService()
	if service == nil {
		return fmt.Errorf("relay service is unavailable")
	}
	if err := service.CommitHandoff(intent.JobID); err != nil && !errors.Is(err, relay.ErrNotFound) {
		return err
	}
	manager.mu.Lock()
	delete(manager.pending, intent.EmailID)
	manager.pendingCount.Store(int64(len(manager.pending)))
	if len(manager.pending) == 0 {
		manager.unhealthy.Store(false)
	}
	manager.mu.Unlock()
	// A shared webhook still owns its fence. Otherwise the durable submitted
	// acknowledgement is now the recovery key, independently of job retention.
	if !manager.server.hasSynchronousListener("new") {
		_ = os.Remove(rollbackFencePath(manager.server.mailDir, intent.EmailID))
		_ = syncDirectory(manager.server.mailDir)
	}
	return nil
}

func (ms *MailServer) pendingAutomaticRelay(id string) bool {
	manager := ms.automaticRelay.Load()
	if manager == nil {
		return false
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if id == "" {
		return len(manager.pending) > 0
	}
	_, ok := manager.pending[id]
	return ok
}
func (ms *MailServer) removeAutomaticRelayIntent(id string) error {
	if manager := ms.automaticRelay.Load(); manager != nil {
		return manager.abort(id)
	}
	path := filepath.Join(ms.mailDir, metadataDirectoryName, relayIntentMetadataDirectoryName, id+".json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectoryIfExists(filepath.Dir(path))
}

type relayReadiness struct {
	service   *relay.Service
	automatic *automaticRelayManager
}

func (health relayReadiness) Snapshot() attachmentstore.HealthStatus {
	status := health.service.Snapshot()
	if health.automatic != nil {
		unhealthy := health.automatic.unhealthy.Load()
		full := health.automatic.pendingCount.Load() >= maximumPendingAutomaticRelays
		if unhealthy || full {
			status.State = attachmentstore.HealthUnready
			status.ErrorCategory = attachmentstore.HealthErrorUnavailable
			status.CheckedAt = time.Now()
		}
	}
	return status
}
