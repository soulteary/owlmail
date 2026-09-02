package mailserver

import (
	"fmt"
	"os"
	"path/filepath"
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

// ValidateMailboxIndexPath rejects locations managed by OwlMail's storage and
// recovery machinery. Those paths are atomically replaced, moved, or removed
// without consulting the optional derived index, so sharing their namespace
// would eventually detach, corrupt, or delete the live SQLite database.
func ValidateMailboxIndexPath(mailDir, indexPath string) error {
	if strings.TrimSpace(indexPath) == "" {
		return nil
	}
	mailPath := ResolveMailDirectory(mailDir)
	mailRoot, err := filepath.Abs(mailPath)
	if err != nil {
		return fmt.Errorf("resolve mail directory: %w", err)
	}
	absoluteIndex, err := filepath.Abs(indexPath)
	if err != nil {
		return fmt.Errorf("resolve mailbox index path: %w", err)
	}
	if err := rejectManagedMailboxIndexPath(mailRoot, absoluteIndex); err != nil {
		return err
	}

	resolvedMailRoot, err := resolveExistingPathIdentity(mailPath)
	if err != nil {
		return fmt.Errorf("resolve mail directory identity: %w", err)
	}
	resolvedIndex, err := resolveExistingPathIdentity(indexPath)
	if err != nil {
		return fmt.Errorf("resolve mailbox index path identity: %w", err)
	}
	return rejectManagedMailboxIndexPath(resolvedMailRoot, resolvedIndex)
}

func rejectManagedMailboxIndexPath(mailRoot, indexPath string) error {
	relative, inside, err := relativePathWithinRoot(mailRoot, indexPath)
	if err != nil {
		return fmt.Errorf("compare mailbox index and mail paths: %w", err)
	}
	if !inside {
		return nil
	}
	if relative == "." {
		return fmt.Errorf("mailbox index path must not replace the mail directory")
	}
	firstComponent := strings.Split(relative, string(filepath.Separator))[0]
	switch {
	case strings.EqualFold(firstComponent, metadataDirectoryName):
		return fmt.Errorf("mailbox index path must not be inside the OwlMail metadata directory")
	case strings.EqualFold(firstComponent, quarantineDirName):
		return fmt.Errorf("mailbox index path must not be inside the OwlMail quarantine directory")
	case strings.EqualFold(firstComponent, webhookOutboxDirectoryName):
		return fmt.Errorf("mailbox index path must not be inside the OwlMail webhook outbox directory")
	case strings.HasPrefix(strings.ToLower(firstComponent), strings.ToLower(storageTempPrefix)):
		return fmt.Errorf("mailbox index path must not use the OwlMail transaction artifact namespace")
	}
	return nil
}

// relativePathWithinRoot compares both the host's native spelling and a folded
// spelling. The latter protects case-insensitive mounts even when the host path
// package cannot infer the mount's case behavior; on sensitive filesystems it
// conservatively reserves case aliases.
func relativePathWithinRoot(root, path string) (string, bool, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", false, err
	}
	if !pathIsOutsideRoot(relative) {
		return relative, true, nil
	}
	foldedRelative, err := filepath.Rel(strings.ToLower(root), strings.ToLower(path))
	if err != nil || pathIsOutsideRoot(foldedRelative) {
		return "", false, nil
	}
	return foldedRelative, true, nil
}

func pathIsOutsideRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// resolveExistingPathIdentity follows symlinks through the deepest existing
// prefix, then rejoins any not-yet-created suffix. This covers a new SQLite
// file below an existing directory alias without requiring the file to exist.
func resolveExistingPathIdentity(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	missing := make([]string, 0)
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing path prefix for %q", absolute)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
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
		BCCAddressesSearch:      indexedAddressText(email.CalculatedBCC, false), FirstFrom: firstFrom,
		Size: email.Size, StorePosition: position,
	}
}

func (ms *MailServer) rebuildMailboxIndex() error {
	if ms.mailboxIndex == nil {
		return nil
	}
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	ms.resetMailboxStorePositionsLocked()
	records := make([]IndexedEmail, 0, len(ms.storeOrder))
	for position, id := range ms.storeOrder {
		if email, ok := ms.storeByID[id]; ok {
			records = append(records, makeIndexedEmail(email, ms.receivedAtByID[id], position))
		}
	}
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
	position := ms.mailboxStorePositionLocked(email.ID)
	if err := ms.mailboxIndex.Upsert(makeIndexedEmail(email, receivedAt, position)); err != nil {
		ms.disableMailboxIndex("update", err)
	}
}

func (ms *MailServer) mailboxStorePositionLocked(id string) int {
	if ms.storePositionByID == nil {
		ms.resetMailboxStorePositionsLocked()
	}
	if position, exists := ms.storePositionByID[id]; exists {
		return position
	}
	position := ms.nextStorePosition
	ms.nextStorePosition++
	ms.storePositionByID[id] = position
	return position
}

func (ms *MailServer) resetMailboxStorePositionsLocked() {
	ms.storePositionByID = make(map[string]int, len(ms.storeOrder))
	for position, id := range ms.storeOrder {
		ms.storePositionByID[id] = position
	}
	ms.nextStorePosition = len(ms.storeOrder)
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
