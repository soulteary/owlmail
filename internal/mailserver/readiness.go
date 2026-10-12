package mailserver

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

// GetServiceReadiness returns cached listener and local storage state. Health
// HTTP handlers never write to disk or dial SMTP just to answer a probe.
func (ms *MailServer) GetServiceReadiness() (bool, attachmentstore.HealthStatus) {
	ms.readinessMutex.RLock()
	storage := ms.mailStorageHealth
	ms.readinessMutex.RUnlock()
	if storage.State == "" {
		storage.State = attachmentstore.HealthChecking
		storage.ErrorCategory = attachmentstore.HealthErrorPending
	}
	if ms.shuttingDown.Load() {
		storage.State = attachmentstore.HealthClosed
		storage.ErrorCategory = attachmentstore.HealthErrorClosed
	}
	return !ms.shuttingDown.Load() && ms.smtpReady.Load() && (ms.smtpsServer == nil || ms.smtpsReady.Load()), storage
}

func (ms *MailServer) probeMailStorage() {
	status := attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now()}
	err := checkMailDirectory(ms.mailDir)
	if err != nil {
		status.State = attachmentstore.HealthUnready
		status.ErrorCategory = attachmentstore.HealthErrorUnavailable
		if errors.Is(err, os.ErrPermission) {
			status.ErrorCategory = attachmentstore.HealthErrorPermission
		}
		if errors.Is(err, os.ErrNotExist) {
			status.ErrorCategory = attachmentstore.HealthErrorNotFound
		}
	}
	ms.readinessMutex.Lock()
	ms.mailStorageHealth = status
	ms.readinessMutex.Unlock()
}

func checkMailDirectory(directory string) error {
	file, err := os.CreateTemp(directory, storageTempPrefix+"health-")
	if err != nil {
		return err
	}
	path := file.Name()
	_, writeErr := file.Write([]byte("owlmail readiness\n"))
	syncErr := file.Sync()
	closeErr := file.Close()
	removeErr := os.Remove(path)
	return errors.Join(writeErr, syncErr, closeErr, removeErr)
}

// startMailStorageReadiness performs an initial check before SMTP readiness is
// announced, then samples every 30 seconds. Its owner stops it on Serve exit.
func (ms *MailServer) startMailStorageReadiness() func() {
	ms.probeMailStorage()
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ms.probeMailStorage()
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }
}
