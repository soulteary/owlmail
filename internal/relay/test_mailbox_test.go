package relay

import (
	"fmt"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

type fakeMailbox struct {
	mu        sync.Mutex
	directory string
	config    *outgoing.OutgoingConfig
	emails    map[string]*types.Email
	health    *Service
	send      func(*types.Email, []string, func(error)) error
	record    func(string, time.Time, []string) error
	acquire   func(string)
	leases    int
}

func newFakeMailbox(directory string) *fakeMailbox {
	return &fakeMailbox{directory: directory, emails: make(map[string]*types.Email)}
}
func (m *fakeMailbox) GetMailDir() string { return m.directory }
func (m *fakeMailbox) GetEmail(id string) (*types.Email, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := m.emails[id]
	if email == nil {
		return nil, fmt.Errorf("email not found")
	}
	copy := *email
	copy.Envelope = cloneEnvelope(email.Envelope)
	return &copy, nil
}
func (m *fakeMailbox) AcquireEmailSource(id string) (*types.Email, func(), error) {
	if m.acquire != nil {
		m.acquire(id)
	}
	email, err := m.GetEmail(id)
	if err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	m.leases++
	m.mu.Unlock()
	var once sync.Once
	return email, func() { once.Do(func() { m.mu.Lock(); m.leases--; m.mu.Unlock() }) }, nil
}
func (m *fakeMailbox) GetOutgoingConfig() *outgoing.OutgoingConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config == nil {
		return nil
	}
	copy := *m.config
	return &copy
}
func (m *fakeMailbox) SetOutgoingConfig(config *outgoing.OutgoingConfig) error {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()
	return nil
}
func (m *fakeMailbox) EffectiveRelayRecipients(email *types.Email) ([]string, error) {
	if email.Envelope == nil || len(email.Envelope.To) == 0 {
		return nil, fmt.Errorf("no recipients")
	}
	return append([]string(nil), email.Envelope.To...), nil
}
func (m *fakeMailbox) RelayMail(email *types.Email, _ bool, callback func(error)) error {
	recipients, err := m.EffectiveRelayRecipients(email)
	if err != nil {
		return err
	}
	return m.deliver(email, recipients, callback)
}
func (m *fakeMailbox) RelayMailTo(email *types.Email, to string, callback func(error)) error {
	return m.deliver(email, []string{to}, callback)
}
func (m *fakeMailbox) RelayMailConfirmed(email *types.Email, to []string, callback func(error)) error {
	return m.deliver(email, to, callback)
}
func (m *fakeMailbox) deliver(email *types.Email, to []string, callback func(error)) error {
	if m.send != nil {
		return m.send(email, to, callback)
	}
	callback(nil)
	return nil
}
func (m *fakeMailbox) RecordRelaySuccess(id string, at time.Time, to []string) error {
	if m.record != nil {
		return m.record(id, at, to)
	}
	return nil
}
func (m *fakeMailbox) GetRelayJobHealth() (attachmentstore.HealthStatus, bool) {
	if c := m.GetOutgoingConfig(); c == nil || c.Host == "" {
		return attachmentstore.HealthStatus{}, false
	}
	return m.health.Snapshot(), true
}
func (m *fakeMailbox) Close() error {
	if m.health != nil {
		return m.health.Close()
	}
	return nil
}
