package mailserver

import (
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/common"
)

type IndexedEmail struct {
	ID                      string
	MessageTime             time.Time
	ReceivedAt              time.Time
	Read                    bool
	SubjectSearch           string
	TextSearch              string
	HTMLSearch              string
	FromSearch              string
	VisibleRecipientsSearch string
	BCCAddressesSearch      string
	FirstFrom               string
	Size                    int64
	StorePosition           int
}

type IndexedEmailResult struct {
	ID   string
	Read bool
}

type MailboxIndex interface {
	Rebuild([]IndexedEmail) error
	Upsert(IndexedEmail) error
	Delete(string) error
	Clear() error
	Query(EmailQuery) ([]IndexedEmailResult, int, error)
	OwnsPath(string) bool
	Backend() string
	Close() error
}

func indexedAddressText(addresses []*mail.Address, includeName bool) string {
	values := make([]string, 0, len(addresses)*2)
	for _, address := range addresses {
		if address == nil {
			continue
		}
		if includeName && address.Name != "" {
			values = append(values, address.Name)
		}
		if address.Address != "" {
			values = append(values, address.Address)
		}
	}
	return strings.ToLower(strings.Join(values, "\x00"))
}

func makeIndexedEmail(email *Email, receivedAt time.Time, position int) IndexedEmail {
	visibleRecipients := make([]*mail.Address, 0, len(email.To)+len(email.CC))
	visibleRecipients = append(visibleRecipients, email.To...)
	visibleRecipients = append(visibleRecipients, email.CC...)
	firstFrom := ""
	if len(email.From) > 0 && email.From[0] != nil {
		firstFrom = strings.ToLower(email.From[0].Address)
	}
	return IndexedEmail{
		ID: email.ID, MessageTime: email.Time, ReceivedAt: receivedAt, Read: email.Read,
		SubjectSearch: strings.ToLower(email.Subject), TextSearch: strings.ToLower(email.Text),
		HTMLSearch: strings.ToLower(email.HTML), FromSearch: indexedAddressText(email.From, true),
		VisibleRecipientsSearch: indexedAddressText(visibleRecipients, true),
		BCCAddressesSearch: indexedAddressText(email.CalculatedBCC, false), FirstFrom: firstFrom,
		Size: email.Size, StorePosition: position,
	}
}

func (ms *MailServer) rebuildMailboxIndex() error {
	if ms.mailboxIndex == nil {
		return nil
	}
	ms.storeMutex.RLock()
	records := make([]IndexedEmail, 0, len(ms.storeOrder))
	for position, id := range ms.storeOrder {
		if email, ok := ms.storeByID[id]; ok {
			records = append(records, makeIndexedEmail(email, ms.receivedAtByID[id], position))
		}
	}
	ms.storeMutex.RUnlock()
	if err := ms.mailboxIndex.Rebuild(records); err != nil {
		return err
	}
	ms.mailboxIndexReady.Store(true)
	return nil
}

func (ms *MailServer) upsertMailboxIndexLocked(email *Email, receivedAt time.Time) {
	if ms.mailboxIndex == nil || !ms.mailboxIndexReady.Load() || email == nil {
		return
	}
	position := 0
	for index, id := range ms.storeOrder {
		if id == email.ID {
			position = index
			break
		}
	}
	if err := ms.mailboxIndex.Upsert(makeIndexedEmail(email, receivedAt, position)); err != nil {
		ms.disableMailboxIndex("update", err)
	}
}

func (ms *MailServer) deleteMailboxIndex(id string) {
	if ms.mailboxIndex == nil || !ms.mailboxIndexReady.Load() {
		return
	}
	if err := ms.mailboxIndex.Delete(id); err != nil {
		ms.disableMailboxIndex("delete", err)
	}
}

func (ms *MailServer) clearMailboxIndex() {
	if ms.mailboxIndex == nil || !ms.mailboxIndexReady.Load() {
		return
	}
	if err := ms.mailboxIndex.Clear(); err != nil {
		ms.disableMailboxIndex("clear", err)
	}
}

func (ms *MailServer) disableMailboxIndex(operation string, err error) {
	if ms.mailboxIndexReady.Swap(false) {
		common.Error("Disabling %s mailbox index after %s failure: %v", ms.mailboxIndex.Backend(), operation, err)
	}
}

func (ms *MailServer) queryIndexedEmailPage(query EmailQuery) ([]emailQueryEntry, int, bool) {
	if ms.mailboxIndex == nil || !ms.mailboxIndexReady.Load() || query.MatchStoreEmail != nil {
		return nil, 0, false
	}
	results, total, err := ms.mailboxIndex.Query(query)
	if err != nil {
		ms.disableMailboxIndex("query", err)
		return nil, 0, false
	}
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	entries := make([]emailQueryEntry, 0, len(results))
	for _, result := range results {
		email, ok := ms.storeByID[result.ID]
		if !ok {
			common.Error("Mailbox index returned missing email %s; using in-memory query", result.ID)
			return nil, 0, false
		}
		entry := snapshotEmailQueryEntry(email, false, false)
		entry.read = result.Read
		entries = append(entries, entry)
	}
	return entries, total, true
}

func (ms *MailServer) mailboxIndexStatus() map[string]interface{} {
	if ms.mailboxIndex == nil {
		return map[string]interface{}{"enabled": false}
	}
	return map[string]interface{}{
		"enabled": true, "ready": ms.mailboxIndexReady.Load(), "backend": ms.mailboxIndex.Backend(),
	}
}

