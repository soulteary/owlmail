package mailserver

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ScopedMailbox exposes only one mailbox. Callers must obtain its ID from the
// authenticated control store, never from a URL or a client-owned header.
type ScopedMailbox struct {
	server *MailServer
	id     string
}

func (ms *MailServer) MailboxView(id string) (*ScopedMailbox, error) {
	parsed, err := uuid.Parse(id)
	if !ms.requireOwnership || err != nil || parsed.String() != id {
		return nil, errors.New("invalid isolated mailbox")
	}
	return &ScopedMailbox{server: ms, id: id}, nil
}
func (v *ScopedMailbox) owns(e *Email) bool {
	return e != nil && e.Ownership != nil && e.Ownership.MailboxID == v.id
}
func (v *ScopedMailbox) project(e *Email) *Email {
	if e == nil {
		return nil
	}
	e.Source = ""
	if e.Envelope != nil {
		e.Envelope.To = append([]string(nil), e.Ownership.MatchedRecipients...)
		e.Envelope.CC = nil
		e.Envelope.BCC = nil
		e.Envelope.CalculatedBCC = nil
	}
	e.CalculatedBCC = nil
	return e
}
func (v *ScopedMailbox) query(q EmailQuery) EmailQuery {
	if q.Limit <= 0 {
		// The legacy query engine treats nonpositive limits as an empty page.
		// Scoped bulk operations explicitly request the complete current snapshot.
		v.server.storeMutex.RLock()
		q.Limit = len(v.server.storeByID)
		v.server.storeMutex.RUnlock()
	}

	previous := q.MatchStoreEmail
	q.MatchStoreEmail = func(e *Email) bool { return v.owns(e) && (previous == nil || previous(e)) }
	return q
}
func (v *ScopedMailbox) List(q EmailQuery) ([]*Email, int) {
	emails, total := v.server.QueryEmails(v.query(q))
	for i := range emails {
		emails[i] = v.project(emails[i])
	}
	return emails, total
}
func (v *ScopedMailbox) Previews(q EmailQuery) ([]EmailPreview, int) {
	return v.server.QueryEmailPreviews(v.query(q))
}
func (v *ScopedMailbox) Get(id string) (*Email, error) {
	e, err := v.server.GetEmail(id)
	if err != nil || !v.owns(e) {
		return nil, ErrEmailNotFound
	}
	return v.project(e), nil
}
func (v *ScopedMailbox) Stats() map[string]interface{} {
	emails, _ := v.List(EmailQuery{})
	unread := 0
	byDate := map[string]int{}
	for _, e := range emails {
		if !e.Read {
			unread++
		}
		byDate[e.Time.Format("2006-01-02")]++
	}
	return map[string]interface{}{"total": len(emails), "unread": unread, "read": len(emails) - unread, "byDate": byDate}
}

// ValidateIDs checks the complete batch before any mutation. It deliberately
// returns the same missing error for foreign and absent messages.
func (v *ScopedMailbox) ValidateIDs(ids []string) error {
	v.server.storeMutex.RLock()
	defer v.server.storeMutex.RUnlock()
	for _, id := range ids {
		if !v.owns(v.server.storeByID[id]) {
			return ErrEmailNotFound
		}
	}
	return nil
}
func (v *ScopedMailbox) Delete(ids []string) (int, error) {
	if err := v.ValidateIDs(ids); err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := v.server.DeleteEmail(id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
func (v *ScopedMailbox) Read(ids []string) (int, error) {
	if err := v.ValidateIDs(ids); err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := v.server.ReadEmail(id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
func (v *ScopedMailbox) IDs(q EmailQuery) []string {
	emails, _ := v.List(q)
	ids := make([]string, 0, len(emails))
	for _, e := range emails {
		ids = append(ids, e.ID)
	}
	return ids
}

type scopedReader struct {
	io.ReadCloser
	release func()
}

func (r *scopedReader) Close() error { err := r.ReadCloser.Close(); r.release(); return err }
func (v *ScopedMailbox) OpenRaw(id string) (io.ReadCloser, int64, error) {
	e, release, err := v.server.AcquireEmailSource(id)
	if err != nil {
		return nil, 0, ErrEmailNotFound
	}
	if !v.owns(e) {
		release()
		return nil, 0, ErrEmailNotFound
	}
	path, err := v.server.GetRawEmail(id)
	if err != nil {
		release()
		return nil, 0, ErrEmailNotFound
	}
	file, err := os.Open(path)
	if err != nil {
		release()
		return nil, 0, ErrEmailNotFound
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		release()
		return nil, 0, err
	}
	return &scopedReader{ReadCloser: file, release: release}, stat.Size(), nil
}
func (v *ScopedMailbox) OpenAttachment(id, name string) (*AttachmentReader, error) {
	e, release, err := v.server.AcquireEmailSource(id)
	if err != nil {
		return nil, ErrEmailNotFound
	}
	if !v.owns(e) {
		release()
		return nil, ErrEmailNotFound
	}
	attachment, err := v.server.OpenEmailAttachment(id, name)
	if err != nil {
		release()
		return nil, ErrEmailNotFound
	}
	attachment.Body = &scopedReader{ReadCloser: attachment.Body, release: release}
	return attachment, nil
}
func (v *ScopedMailbox) aeadLocked() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("owlmail:scoped-cursor:v1:" + v.server.deliveryEpoch))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (v *ScopedMailbox) cursorLocked(sequence uint64) (string, error) {
	aead, err := v.aeadLocked()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, sequence)
	sealed := aead.Seal(nonce, nonce, payload, []byte(v.id))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}
func (v *ScopedMailbox) parseCursorLocked(cursor string) (uint64, error) {
	aead, err := v.aeadLocked()
	if err != nil {
		return 0, err
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(data) != aead.NonceSize()+8+aead.Overhead() {
		return 0, ErrInvalidCursor
	}
	payload, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(v.id))
	if err != nil {
		return 0, ErrInvalidCursor
	}
	seq := binary.BigEndian.Uint64(payload)
	if seq > v.server.deliverySequence {
		return 0, ErrInvalidCursor
	}
	return seq, nil
}
func (v *ScopedMailbox) Cursor() (string, error) {
	ms := v.server
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	ms.initDeliveriesLocked()
	return v.cursorLocked(ms.deliverySequence)
}
func (v *ScopedMailbox) Wait(ctx context.Context, after string, filter DeliveryFilter, count int, timeout time.Duration) (DeliveryResult, error) {
	result := DeliveryResult{Emails: make([]*Email, 0)}
	if err := filter.Validate(); err != nil {
		return result, err
	}
	if count < 1 || count > 100 || timeout <= 0 || timeout > 120*time.Second {
		return result, fmt.Errorf("invalid count or timeout")
	}
	ms := v.server
	ms.storeMutex.Lock()
	ms.initDeliveriesLocked()
	sequence, err := v.parseCursorLocked(after)
	if err != nil {
		ms.storeMutex.Unlock()
		return result, err
	}
	if ms.deliveryClosed {
		ms.storeMutex.Unlock()
		return result, ErrMailboxClosed
	}
	if ms.deliveryWaiters >= 64 || ms.scopedWaiters[v.id] >= 8 {
		ms.storeMutex.Unlock()
		return result, ErrWaitCapacity
	}
	if ms.scopedWaiters == nil {
		ms.scopedWaiters = make(map[string]int)
	}
	ms.deliveryWaiters++
	ms.scopedWaiters[v.id]++
	ms.storeMutex.Unlock()
	defer func() {
		ms.storeMutex.Lock()
		ms.deliveryWaiters--
		ms.scopedWaiters[v.id]--
		if ms.scopedWaiters[v.id] == 0 {
			delete(ms.scopedWaiters, v.id)
		}
		ms.storeMutex.Unlock()
	}()
	started := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		ms.storeMutex.RLock()
		if ms.deliveryClosed {
			ms.storeMutex.RUnlock()
			return result, ErrMailboxClosed
		}
		scanned := ms.deliverySequence
		start := sort.Search(len(ms.deliveryOrder), func(i int) bool { return ms.deliveryOrder[i].sequence > sequence })
		for _, entry := range ms.deliveryOrder[start:] {
			e := ms.storeByID[entry.id]
			if v.owns(e) && filter.Matches(e) {
				result.Emails = append(result.Emails, v.project(cloneEmail(e)))
				if len(result.Emails) == count {
					scanned = entry.sequence
					break
				}
			}
		}
		sequence = scanned
		result.Cursor, err = v.cursorLocked(scanned)
		changed := ms.deliveryChanged
		ms.storeMutex.RUnlock()
		if err != nil {
			return result, err
		}
		result.WaitedMilliseconds = time.Since(started).Milliseconds()
		if len(result.Emails) >= count {
			return result, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timer.C:
			result.TimedOut = true
			result.WaitedMilliseconds = time.Since(started).Milliseconds()
			return result, nil
		}
	}
}
