package mailserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/types"
)

const ingressDirectory = ".owlmail-ingress"
const ingressJournal = "journal.json"
const ingressCommit = "COMMIT"

// RoutedIngressOptions configures authenticated system SMTP, not user login.
// The caller retains ownership of Control and must close it after MailServer.
type RoutedIngressOptions struct {
	Accounts            *mailbox.Accounts
	Control             *mailbox.Store
	MaxPending          int
	MaxCreatesPerMinute int
	MaxAuthPerMinute    int
	MaxMessages         int
	MaxDiskBytes        int64
	MaxTotalMessages    int
	MaxTotalDiskBytes   int64
}
type routedIngress struct {
	failed  atomic.Bool
	options RoutedIngressOptions
	// fault is nil in production and models crash boundaries in recovery tests.
	fault func(string, int) error
	clock func() time.Time
}
type routedJournal struct {
	Version       int      `json:"version"`
	TransactionID string   `json:"transactionId"`
	IDs           []string `json:"ids"`
}
type routedPrepared struct {
	email      *Email
	receivedAt time.Time
}

func (ms *MailServer) configureRoutedIngress(options *RoutedIngressOptions) error {
	if options == nil {
		return nil
	}
	if !ms.requireOwnership || options.Accounts == nil || options.Control == nil || options.MaxPending <= 0 || options.MaxCreatesPerMinute <= 0 {
		return errors.New("routed SMTP requires isolated storage, accounts, control store and positive limits")
	}
	if !ms.authRequireTLS {
		return errors.New("routed SMTP requires encrypted authentication")
	}
	if ms.attachmentStore != nil || ms.outgoing != nil {
		return errors.New("routed SMTP does not support S3 or outgoing transports")
	}
	resolved := *options
	if resolved.MaxAuthPerMinute == 0 {
		resolved.MaxAuthPerMinute = 120
	}
	if resolved.MaxMessages == 0 {
		resolved.MaxMessages = 10000
	}
	if resolved.MaxDiskBytes == 0 {
		resolved.MaxDiskBytes = 1 << 30
	}
	if resolved.MaxAuthPerMinute < 1 || resolved.MaxMessages < 1 || resolved.MaxDiskBytes < 1 || resolved.MaxTotalMessages < 0 || resolved.MaxTotalDiskBytes < 0 {
		return errors.New("routed SMTP quotas must be positive")
	}
	ms.routedIngress = &routedIngress{options: resolved}
	return nil
}

// allowAttempt retains the authentication adapter signature, but does not
// accept caller-controlled usernames as quota keys.
func (r *routedIngress) allowAttempt(account string, authentication bool) bool {
	if !authentication || account != "" {
		return false
	}
	return r.consumeAttempt("auth", "", "", r.options.MaxAuthPerMinute)
}
func (r *routedIngress) consumeAttempt(kind, namespace, account string, limit int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	at := time.Now()
	if r.clock != nil {
		at = r.clock()
	}
	err := r.options.Control.ConsumeAttempt(ctx, kind, namespace, account, limit, at)
	if err != nil && !errors.Is(err, mailbox.ErrAttemptQuota) {
		common.Error("Persist SMTP attempt quota: %v", err)
	}
	return err == nil
}
func routedTemporary() error {
	return &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 0}, Message: "Mailbox submission temporarily unavailable"}
}
func routedRecipientRejected() error {
	return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 7, 1}, Message: "Recipient is not permitted"}
}

func (s *Session) routeRecipient(address string) error {
	ingress := s.mailServer.routedIngress
	if ingress == nil || ingress.failed.Load() || s.ingestAccount == nil {
		return errSMTPAuthRequired
	}
	normalized, err := mailbox.NormalizeAddress(address)
	if err != nil || !s.ingestAccount.Allows(normalized) {
		return routedRecipientRejected()
	}
	// Count RCPT attempts, including repeated/abandoned transactions, to avoid
	// an attacker bypassing creation bounds by never sending DATA.
	if !ingress.consumeAttempt("rcpt", s.ingestAccount.Namespace, s.ingestAccount.ID, ingress.options.MaxCreatesPerMinute) {
		return routedTemporary()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := ingress.options.Control.EnsurePending(ctx, s.ingestAccount.Namespace, normalized, ingress.options.MaxPending)
	if err != nil {
		if errors.Is(err, mailbox.ErrBlocked) {
			return routedRecipientRejected()
		}
		return routedTemporary()
	}
	if s.rcptRoutes == nil {
		s.rcptRoutes = map[string]mailbox.Route{}
	}
	s.rcptRoutes[normalized] = route
	for _, existing := range s.to {
		if existing == normalized {
			return nil
		}
	}
	s.to = append(s.to, normalized)
	return nil
}

// storeRoutedIncoming reads the network DATA before taking the control lock.
// Final route resolution and the local durable commit hold Store.mu followed
// by storageTransactionMutex and storeMutex, the same order as claiming.
func (ms *MailServer) storeRoutedIncoming(reader io.Reader, s *Session) error {
	if ms.routedIngress == nil || ms.routedIngress.failed.Load() || s.ingestAccount == nil || len(s.to) == 0 {
		return routedTemporary()
	}
	root := filepath.Join(ms.mailDir, ingressDirectory)
	if err := os.MkdirAll(root, 0700); err != nil {
		return routedTemporary()
	}
	txID := uuid.NewString()
	stage := filepath.Join(root, txID)
	if err := os.Mkdir(stage, 0700); err != nil {
		return routedTemporary()
	}
	preparedJournal := false
	defer func() {
		if !preparedJournal {
			_ = os.RemoveAll(stage)
		}
	}()
	source := filepath.Join(stage, "source.eml")
	if err := writeIngressFile(source, reader, ms.maxMessageBytes); err != nil {
		var smtpErr *smtp.SMTPError
		if errors.As(err, &smtpErr) {
			return smtpErr
		}
		common.Error("Stage routed SMTP DATA: %v", err)
		return routedTemporary()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	expected := make([]mailbox.Route, 0, len(s.to))
	for _, address := range s.to {
		route, ok := s.rcptRoutes[address]
		if !ok {
			return routedTemporary()
		}
		expected = append(expected, route)
	}
	err := ms.routedIngress.options.Control.WithExpectedRouteSnapshot(ctx, s.ingestAccount.Namespace, expected, func(routes []mailbox.Route) error {
		ms.storageTransactionMutex.Lock()
		defer ms.storageTransactionMutex.Unlock()
		// No global event path may observe routed emails or perform external side
		// effects. Per-mailbox outbound delivery is a separately gated feature.
		if ms.hasSynchronousListener("new") || ms.automaticRelay.Load() != nil || ms.webhookService != nil {
			return errors.New("routed SMTP cannot use global delivery listeners")
		}
		grouped := map[string]*types.Ownership{}
		order := []string{}
		for _, route := range routes {
			ownership := grouped[route.MailboxID]
			if ownership == nil {
				ownership = &types.Ownership{MailboxID: route.MailboxID, IngestAccountID: s.ingestAccount.ID, TransactionID: txID}
				grouped[route.MailboxID] = ownership
				order = append(order, route.MailboxID)
			}
			ownership.MatchedRecipients = append(ownership.MatchedRecipients, route.Address)
			ownership.RouteRevisions = append(ownership.RouteRevisions, route.Revision)
		}
		journal := routedJournal{Version: 1, TransactionID: txID}
		copies := make([]routedPrepared, 0, len(order))
		for _, mailboxID := range order {
			id := uuid.NewString()
			journal.IDs = append(journal.IDs, id)
			copy, err := ms.prepareRoutedCopy(stage, source, id, grouped[mailboxID], s)
			if err != nil {
				return err
			}
			copies = append(copies, copy)
		}
		if err := ms.checkRoutedQuotaLocked(stage, copies); err != nil {
			return err
		}
		if err := ms.checkRoutedDestinations(journal); err != nil {
			return err
		}
		if err := writeIngressJournal(stage, journal); err != nil {
			return err
		}
		preparedJournal = true
		if err := syncDirectory(root); err != nil {
			return err
		}
		if err := ms.installRoutedCopies(stage, journal); err != nil {
			return err
		}
		if err := ms.ingressFault("before-commit", 0); err != nil {
			return err
		}
		marker := filepath.Join(stage, ingressCommit)
		f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if _, err = f.WriteString(txID + "\n"); err != nil {
			_ = f.Close()
			_ = os.Remove(marker)
			return err
		}
		if err = f.Sync(); err != nil {
			_ = f.Close()
			_ = os.Remove(marker)
			return err
		}
		if err = f.Close(); err != nil {
			_ = os.Remove(marker)
			return err
		}
		// The marker is now visible and its contents synced. A directory-sync
		// failure is an accepted-but-degraded commit, never permission to roll back
		// or tell the sender that retrying is safe (same policy as accepted fences).
		if err = syncDirectory(stage); err != nil {
			common.Error("Sync routed commit marker directory: %v", err)
		}
		ms.publishRoutedCopies(copies)
		if err = ms.ingressFault("after-commit", 0); err != nil {
			return nil
		}
		// A committed journal is only garbage. Recovery must never recreate a
		// later-deleted copy from it.
		if err = os.RemoveAll(stage); err != nil {
			common.Error("Clean committed routed journal: %v", err)
		} else {
			_ = syncDirectory(root)
		}
		return nil
	})
	if err != nil {
		if preparedJournal {
			// Leave the journal if cleanup fails. Future submissions refuse to proceed
			// until startup recovery can establish a clean authoritative state.
			if cleanupErr := ms.rollbackRoutedJournal(stage); cleanupErr != nil {
				ms.routedIngress.failed.Store(true)
				common.Error("Rollback routed transaction: %v", cleanupErr)
			}
		}
		common.Error("Routed SMTP submission failed: %v", err)
		return routedTemporary()
	}
	return nil
}
func writeIngressFile(path string, reader io.Reader, maxBytes int64) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	n, err := io.Copy(file, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return err
	}
	if n > maxBytes {
		return &smtp.SMTPError{Code: 552, EnhancedCode: smtp.EnhancedCode{5, 3, 4}, Message: "Message too large"}
	}
	if err = file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
func (ms *MailServer) prepareRoutedCopy(stage, source, id string, ownership *types.Ownership, session *Session) (routedPrepared, error) {
	raw, err := os.Open(source)
	if err != nil {
		return routedPrepared{}, err
	}
	err = writeIngressFile(filepath.Join(stage, id+".eml"), raw, ms.maxMessageBytes)
	_ = raw.Close()
	if err != nil {
		return routedPrepared{}, err
	}
	attachments := filepath.Join(stage, id)
	if err = os.Mkdir(attachments, 0700); err != nil {
		return routedPrepared{}, err
	}
	raw, err = os.Open(source)
	if err != nil {
		return routedPrepared{}, err
	}
	copySession := &Session{mailServer: ms, conn: session.conn, from: session.from, to: append([]string(nil), ownership.MatchedRecipients...)}
	email, envelope, err := ms.parseEmailMessage(id, raw, copySession, true, attachments)
	closeErr := raw.Close()
	if err != nil {
		return routedPrepared{}, err
	}
	if closeErr != nil {
		return routedPrepared{}, closeErr
	}
	email.ID = id
	email.Ownership = cloneOwnership(ownership)
	email.Envelope = envelope
	email.Source = filepath.Join(ms.mailDir, id+".eml")
	if email.Time.IsZero() {
		email.Time = time.Now().UTC()
	}
	email.HTML = strings.TrimSpace(sanitizeHTML(email.HTML))
	email.CalculatedBCC = calculateBCC(append([]string(nil), envelope.To...), addressListToStrings(email.To), addressListToStrings(email.CC))
	info, err := os.Stat(source)
	if err != nil {
		return routedPrepared{}, err
	}
	email.Size = info.Size()
	email.SizeHuman = formatBytes(info.Size())
	if ms.beforeStoreCommit != nil {
		if err = ms.beforeStoreCommit(email); err != nil {
			return routedPrepared{}, err
		}
	}
	receivedAt := time.Now().UTC()
	stager := &MailServer{mailDir: stage, requireOwnership: true}
	if err = stager.persistEmailMetadataAt(email, receivedAt); err != nil {
		return routedPrepared{}, err
	}
	if err = syncDirectory(attachments); err != nil {
		return routedPrepared{}, err
	}
	return routedPrepared{email: email, receivedAt: receivedAt}, nil
}
func writeIngressJournal(stage string, journal routedJournal) error {
	encoded, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if err = writeIngressFile(filepath.Join(stage, ingressJournal), strings.NewReader(string(encoded)), 1<<20); err != nil {
		return err
	}
	return syncDirectory(stage)
}
func (ms *MailServer) installRoutedCopies(stage string, journal routedJournal) error {
	metadataDir := filepath.Join(ms.mailDir, metadataDirectoryName)
	if err := os.MkdirAll(metadataDir, 0750); err != nil {
		return err
	}
	for i, id := range journal.IDs {
		for _, relative := range []string{id, id + ".eml", filepath.Join(metadataDirectoryName, id+".json")} {
			destination := filepath.Join(ms.mailDir, relative)
			if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("routed destination already exists or is inaccessible")
			}
			if err := os.Rename(filepath.Join(stage, relative), destination); err != nil {
				return err
			}
		}
		if err := ms.ingressFault("after-install", i); err != nil {
			return err
		}
	}
	if err := syncDirectory(metadataDir); err != nil {
		return err
	}
	return syncDirectory(ms.mailDir)
}
func (ms *MailServer) publishRoutedCopies(copies []routedPrepared) {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	for _, copy := range copies {
		email := cloneEmail(copy.email)
		ms.storeByID[email.ID] = email
		ms.storeOrder = append(ms.storeOrder, email.ID)
		ms.receivedAtByID[email.ID] = copy.receivedAt
		ms.recordDeliveryLocked(email.ID)
		ms.upsertMailboxIndexLocked(email, copy.receivedAt)
		ms.receivedMessages.Add(1)
	}
}
func (ms *MailServer) ingressFault(stage string, index int) error {
	if ms.routedIngress != nil && ms.routedIngress.fault != nil {
		return ms.routedIngress.fault(stage, index)
	}
	return nil
}

func readRoutedJournal(stage string) (routedJournal, error) {
	var journal routedJournal
	info, err := os.Lstat(filepath.Join(stage, ingressJournal))
	if err != nil {
		return journal, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return journal, errors.New("invalid routed journal file")
	}
	encoded, err := os.ReadFile(filepath.Join(stage, ingressJournal))
	if err != nil {
		return journal, err
	}
	if err = json.Unmarshal(encoded, &journal); err != nil {
		return journal, err
	}
	parsed, err := uuid.Parse(journal.TransactionID)
	if err != nil || parsed.String() != journal.TransactionID || filepath.Base(stage) != journal.TransactionID || journal.Version != 1 || len(journal.IDs) == 0 || len(journal.IDs) > 1000 {
		return journal, errors.New("invalid routed journal identity")
	}
	seen := map[string]bool{}
	for _, id := range journal.IDs {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.String() != id || seen[id] {
			return journal, errors.New("invalid routed journal email ID")
		}
		seen[id] = true
	}
	return journal, nil
}
func (ms *MailServer) rollbackRoutedJournal(stage string) error {
	journal, err := readRoutedJournal(stage)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(filepath.Join(stage, ingressCommit)); err == nil {
		return errors.New("cannot roll back a committed routed transaction")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, id := range journal.IDs {
		for _, relative := range []string{id, id + ".eml", filepath.Join(metadataDirectoryName, id+".json")} {
			if err = os.RemoveAll(filepath.Join(ms.mailDir, relative)); err != nil {
				return err
			}
		}
	}
	if err = syncDirectoryIfExists(filepath.Join(ms.mailDir, metadataDirectoryName)); err != nil {
		return err
	}
	if err = syncDirectory(ms.mailDir); err != nil {
		return err
	}
	if err = os.RemoveAll(stage); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(stage))
}

// recoverRoutedIngress runs before ordinary EML loading, including when no
// SMTP ingress is configured. Prepared transactions are rolled back; committed
// journals are cleaned only and never reconstruct subsequently deleted mail.
func (ms *MailServer) recoverRoutedIngress() error {
	if !ms.requireOwnership {
		return nil
	}
	root := filepath.Join(ms.mailDir, ingressDirectory)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		parsed, err := uuid.Parse(entry.Name())
		if err != nil || parsed.String() != entry.Name() || !entry.IsDir() {
			return errors.New("invalid routed transaction directory")
		}
		stage := filepath.Join(root, entry.Name())
		journal, err := readRoutedJournal(stage)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.RemoveAll(stage); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		marker := filepath.Join(stage, ingressCommit)
		info, err := os.Lstat(marker)
		if errors.Is(err, os.ErrNotExist) {
			if err = ms.rollbackRoutedJournal(stage); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 100 {
			return errors.New("invalid routed commit marker")
		}
		value, err := os.ReadFile(marker)
		if err != nil {
			return err
		}
		if string(value) != journal.TransactionID+"\n" {
			return errors.New("invalid routed commit marker identity")
		}
		if err = os.RemoveAll(stage); err != nil {
			return err
		}
	}
	return syncDirectory(root)
}

func (ms *MailServer) checkRoutedDestinations(journal routedJournal) error {
	for _, id := range journal.IDs {
		for _, relative := range []string{id, id + ".eml", filepath.Join(metadataDirectoryName, id+".json")} {
			if _, err := os.Lstat(filepath.Join(ms.mailDir, relative)); !errors.Is(err, os.ErrNotExist) {
				return errors.New("routed destination collision")
			}
		}
	}
	return nil
}

// ErrMailboxQuota is a temporary submission/merge refusal. No journal or
// metadata is changed when a quota preflight fails.
var ErrMailboxQuota = mailbox.ErrQuotaExceeded

type routedUsage struct {
	messages int
	bytes    int64
}

// addUsage avoids overflow even if an existing volume exceeds a newly lowered
// limit. Bytes are logical file lengths, not filesystem allocation blocks.
func addRoutedUsage(current routedUsage, messages int, bytes int64, maxMessages int, maxBytes int64) (routedUsage, error) {
	if messages < 0 || bytes < 0 || current.messages > maxMessages || current.bytes > maxBytes || messages > maxMessages-current.messages || bytes > maxBytes-current.bytes {
		return current, ErrMailboxQuota
	}
	return routedUsage{messages: current.messages + messages, bytes: current.bytes + bytes}, nil
}

func routedDiskUsage(directory, id string) (int64, error) {
	var total int64
	add := func(path string) error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("routed message contains a nonregular file")
		}
		size := info.Size()
		if size < 0 || size > int64(^uint64(0)>>1)-total {
			return errors.New("routed disk usage overflow")
		}
		total += size
		return nil
	}
	for _, path := range []string{filepath.Join(directory, id+".eml"), filepath.Join(directory, metadataDirectoryName, id+".json")} {
		if err := add(path); err != nil {
			return 0, err
		}
	}
	attachments := filepath.Join(directory, id)
	err := filepath.WalkDir(attachments, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		return add(path)
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	return total, nil
}

// checkRoutedQuotaLocked runs inside control.mu -> storage write lock, before
// the prepared journal. All targets are checked together. Usage is derived
// from the same durable messages recovery reloads; no separate charge ledger
// can diverge after rejection, crashes, deletion, or ownership migration.
func (ms *MailServer) checkRoutedQuotaLocked(stage string, copies []routedPrepared) error {
	if ms.routedIngress == nil {
		return errors.New("routed quota requires ingress")
	}
	limits := ms.routedIngress.options
	usage := make(map[string]routedUsage, len(copies))
	for _, copy := range copies {
		usage[copy.email.Ownership.MailboxID] = routedUsage{}
	}
	// A zero global limit is only a raw-component compatibility value. The CLI
	// supplies finite positive defaults. Positive global limits reject new mail;
	// they never evict another person's existing messages.
	globalEnabled := limits.MaxTotalMessages > 0 || limits.MaxTotalDiskBytes > 0
	globalMessages := limits.MaxTotalMessages
	if globalMessages == 0 {
		globalMessages = int(^uint(0) >> 1)
	}
	globalBytes := limits.MaxTotalDiskBytes
	if globalBytes == 0 {
		globalBytes = int64(^uint64(0) >> 1)
	}
	total := routedUsage{}
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	for _, email := range ms.storeByID {
		if email.Ownership == nil {
			return errors.New("isolated message has no ownership")
		}
		box := email.Ownership.MailboxID
		current, wanted := usage[box]
		var bytes int64
		var err error
		if wanted || limits.MaxTotalDiskBytes > 0 {
			bytes, err = routedDiskUsage(ms.mailDir, email.ID)
			if err != nil {
				return err
			}
		}
		if globalEnabled {
			chargedBytes := bytes
			if limits.MaxTotalDiskBytes == 0 {
				chargedBytes = 0
			}
			total, err = addRoutedUsage(total, 1, chargedBytes, globalMessages, globalBytes)
			if err != nil {
				return err
			}
		}
		if wanted {
			usage[box], err = addRoutedUsage(current, 1, bytes, limits.MaxMessages, limits.MaxDiskBytes)
			if err != nil {
				return err
			}
		}
	}
	for _, copy := range copies {
		box := copy.email.Ownership.MailboxID
		bytes, err := routedDiskUsage(stage, copy.email.ID)
		if err != nil {
			return err
		}
		if globalEnabled {
			chargedBytes := bytes
			if limits.MaxTotalDiskBytes == 0 {
				chargedBytes = 0
			}
			total, err = addRoutedUsage(total, 1, chargedBytes, globalMessages, globalBytes)
			if err != nil {
				return err
			}
		}
		usage[box], err = addRoutedUsage(usage[box], 1, bytes, limits.MaxMessages, limits.MaxDiskBytes)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkOwnershipMergeQuotaLocked requires control.mu -> storage write lock ->
// storeMutex, already held by MergeOwnership. A failed merge stays pending and
// retains all source mail; no partial owner rewrite can precede this check.
func (ms *MailServer) checkOwnershipMergeQuotaLocked(sourceID, targetID string) error {
	if ms.routedIngress == nil {
		return nil
	}
	limits := ms.routedIngress.options
	current := routedUsage{}
	for _, email := range ms.storeByID {
		if email.Ownership == nil {
			return errors.New("isolated message has no ownership")
		}
		if email.Ownership.MailboxID != sourceID && email.Ownership.MailboxID != targetID {
			continue
		}
		bytes, err := routedDiskUsage(ms.mailDir, email.ID)
		if err != nil {
			return err
		}
		current, err = addRoutedUsage(current, 1, bytes, limits.MaxMessages, limits.MaxDiskBytes)
		if err != nil {
			return err
		}
	}
	return nil
}
