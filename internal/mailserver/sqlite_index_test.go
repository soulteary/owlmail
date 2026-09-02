package mailserver

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

type blockingRebuildMailboxIndex struct {
	MailboxIndex
	mutex   sync.Mutex
	block   bool
	started chan struct{}
	release chan struct{}
}

type blockingDeleteMailboxIndex struct {
	MailboxIndex
	started chan struct{}
	release chan struct{}
}

func (index *blockingDeleteMailboxIndex) Delete(id string) error {
	err := index.MailboxIndex.Delete(id)
	close(index.started)
	<-index.release
	return err
}

func (index *blockingRebuildMailboxIndex) blockNextRebuild() (<-chan struct{}, chan<- struct{}) {
	index.mutex.Lock()
	defer index.mutex.Unlock()
	index.block = true
	index.started = make(chan struct{})
	index.release = make(chan struct{})
	return index.started, index.release
}

func (index *blockingRebuildMailboxIndex) Rebuild(records []IndexedEmail) error {
	index.mutex.Lock()
	if !index.block {
		index.mutex.Unlock()
		return index.MailboxIndex.Rebuild(records)
	}
	index.block = false
	started, release := index.started, index.release
	index.mutex.Unlock()
	close(started)
	<-release
	return index.MailboxIndex.Rebuild(records)
}

func TestSQLiteMailboxIndexQueryAndRebuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailbox.db")
	index, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := index.Close(); err != nil {
			t.Errorf("close SQLite index: %v", err)
		}
	}()
	now := time.Now().UTC()
	records := []IndexedEmail{
		{ID: "one", MessageTime: now.Add(-time.Hour), ReceivedAt: now, SubjectSearch: "alpha", TextSearch: "first body", FromSearch: "alice@example.test", VisibleRecipientsSearch: "team@example.test", BCCAddressesSearch: "secret@example.test", FirstFrom: "alice@example.test", StorePosition: 0},
		{ID: "two", MessageTime: now, ReceivedAt: now, Read: true, SubjectSearch: "beta", TextSearch: "second body", FromSearch: "bob@example.test", VisibleRecipientsSearch: "team@example.test", FirstFrom: "bob@example.test", StorePosition: 1},
	}
	if err := index.Rebuild(records); err != nil {
		t.Fatal(err)
	}
	unread := false
	results, total, err := index.Query(EmailQuery{Text: "alpha", Read: &unread, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].ID != "one" {
		t.Fatalf("query = %#v, total %d", results, total)
	}
	results, total, err = index.Query(EmailQuery{Text: "secret@example.test", SearchAddresses: true, Limit: 10})
	if err != nil || total != 0 || len(results) != 0 {
		t.Fatalf("free-text search exposed BCC = %#v, total %d, err %v", results, total, err)
	}
	results, total, err = index.Query(EmailQuery{To: "secret@example.test", Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "one" {
		t.Fatalf("BCC address query = %#v, total %d, err %v", results, total, err)
	}
	results, total, err = index.Query(EmailQuery{To: "hidden person", Limit: 10})
	if err != nil || total != 0 || len(results) != 0 {
		t.Fatalf("BCC display name query = %#v, total %d, err %v", results, total, err)
	}
	results, total, err = index.Query(EmailQuery{SortBy: "store", SortOrder: "desc", Limit: 10})
	if err != nil || total != 2 || len(results) != 2 || results[0].ID != "two" || results[1].ID != "one" {
		t.Fatalf("descending store query = %#v, total %d, err %v", results, total, err)
	}
	if err := index.Rebuild(records[1:]); err != nil {
		t.Fatal(err)
	}
	results, total, err = index.Query(EmailQuery{Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "two" {
		t.Fatalf("rebuilt query = %#v, total %d, err %v", results, total, err)
	}
	if !index.OwnsPath(path) || !index.OwnsPath(path+"-wal") || !index.OwnsPath(filepath.Dir(path)) || index.OwnsPath(filepath.Join(filepath.Dir(path), "other.db")) {
		t.Fatal("SQLite artifact ownership mismatch")
	}
	if runtime.GOOS != "windows" {
		for _, artifact := range []string{path, path + "-wal", path + "-shm"} {
			info, err := os.Stat(artifact)
			if err != nil {
				t.Fatalf("stat SQLite artifact %s: %v", artifact, err)
			}
			if got := info.Mode().Perm(); got != 0600 {
				t.Fatalf("SQLite artifact %s permissions = %o, want 600", artifact, got)
			}
		}
	}
	results, total, err = index.Query(EmailQuery{Limit: int(^uint(0) >> 1)})
	if err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("unbounded compatibility query = %#v, total %d, err %v", results, total, err)
	}
}

func TestSQLiteMailboxIndexOwnsFilesystemAliases(t *testing.T) {
	mailDirectory := t.TempDir()
	path := filepath.Join(mailDirectory, ".index", "mailbox.db")
	index, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = index.Close() }()

	alias := filepath.Join(t.TempDir(), "mail-link")
	if err := os.Symlink(mailDirectory, alias); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	for _, owned := range []string{
		filepath.Join(alias, ".index"),
		filepath.Join(alias, ".index", "mailbox.db"),
		filepath.Join(alias, ".index", "mailbox.db-wal"),
	} {
		if !index.OwnsPath(owned) {
			t.Errorf("OwnsPath(%q) = false for filesystem alias", owned)
		}
	}
	if index.OwnsPath(filepath.Join(alias, "other.db")) {
		t.Fatal("OwnsPath accepted unrelated file through filesystem alias")
	}
	caseVariant := filepath.Join(filepath.Dir(path), "MAILBOX.DB")
	caseFile, err := os.OpenFile(caseVariant, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	switch {
	case err == nil:
		if closeErr := caseFile.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if index.OwnsPath(caseVariant) {
			t.Fatal("OwnsPath conflated distinct case-variant files")
		}
	case errors.Is(err, os.ErrExist):
		if !index.OwnsPath(caseVariant) {
			t.Fatal("OwnsPath missed a case alias on a case-insensitive filesystem")
		}
	default:
		t.Fatalf("create case-variant ownership probe: %v", err)
	}
}

func TestSQLiteMailboxIndexPreservesWideMessageTimeRange(t *testing.T) {
	index, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = index.Close() }()
	receivedAt := time.Now().UTC()
	ancient := time.Date(1200, time.January, 2, 3, 4, 5, 123, time.UTC)
	modern := time.Date(2026, time.January, 2, 3, 4, 5, 456, time.UTC)
	future := time.Date(2500, time.January, 2, 3, 4, 5, 789, time.UTC)
	if err := index.Rebuild([]IndexedEmail{
		{ID: "ancient", MessageTime: ancient, ReceivedAt: receivedAt, StorePosition: 0},
		{ID: "modern", MessageTime: modern, ReceivedAt: receivedAt, StorePosition: 1},
		{ID: "future", MessageTime: future, ReceivedAt: receivedAt, StorePosition: 2},
	}); err != nil {
		t.Fatal(err)
	}
	results, total, err := index.Query(EmailQuery{SortBy: "time", SortOrder: "asc", Limit: 10})
	if err != nil || total != 3 || len(results) != 3 || results[0].ID != "ancient" || results[1].ID != "modern" || results[2].ID != "future" {
		t.Fatalf("wide-range time order = %#v, total %d, err %v", results, total, err)
	}
	dateFrom := time.Date(2300, time.January, 1, 0, 0, 0, 0, time.UTC)
	results, total, err = index.Query(EmailQuery{DateFrom: &dateFrom, Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "future" {
		t.Fatalf("wide-range lower bound = %#v, total %d, err %v", results, total, err)
	}
	dateTo := time.Date(1500, time.January, 1, 0, 0, 0, 0, time.UTC)
	results, total, err = index.Query(EmailQuery{DateTo: &dateTo, Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "ancient" {
		t.Fatalf("wide-range upper bound = %#v, total %d, err %v", results, total, err)
	}
}

func TestSQLiteMailboxIndexUsesOneSnapshotForCountAndPage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailbox.db")
	index, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = index.Close() }()
	writer, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	now := time.Now().UTC()
	first := IndexedEmail{ID: "first", MessageTime: now, ReceivedAt: now}
	second := IndexedEmail{ID: "second", MessageTime: now.Add(time.Second), ReceivedAt: now.Add(time.Second)}
	if err := index.Rebuild([]IndexedEmail{first}); err != nil {
		t.Fatal(err)
	}
	index.afterQueryCount = func() {
		if err := writer.Upsert(second); err != nil {
			t.Errorf("concurrent index update: %v", err)
		}
	}

	results, total, err := index.Query(EmailQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].ID != "first" {
		t.Fatalf("mixed query snapshot = %#v, total %d", results, total)
	}
	index.afterQueryCount = nil
	results, total, err = index.Query(EmailQuery{Limit: 10})
	if err != nil || total != 2 || len(results) != 2 {
		t.Fatalf("query after concurrent update = %#v, total %d, err %v", results, total, err)
	}
}

func TestMailServerUsesSQLiteIndexAndSynchronizesMutations(t *testing.T) {
	directory := t.TempDir()
	indexPath := filepath.Join(directory, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	for position, value := range []struct{ id, subject string }{{"mail-one", "Alpha"}, {"mail-two", "Beta"}} {
		if err := os.WriteFile(filepath.Join(directory, value.id+".eml"), []byte("message"), 0600); err != nil {
			t.Fatal(err)
		}
		email := &types.Email{ID: value.id, Subject: value.subject, Text: value.subject + " body", Time: time.Now().Add(time.Duration(position) * time.Second)}
		envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
		if err := server.SaveEmailToStore(value.id, false, envelope, email); err != nil {
			t.Fatal(err)
		}
	}
	results, total := server.QueryEmailPreviews(EmailQuery{Text: "beta", Limit: 10})
	if total != 1 || len(results) != 1 || results[0].ID != "mail-two" {
		t.Fatalf("indexed previews = %#v, total %d", results, total)
	}
	if err := server.ReadEmail("mail-two"); err != nil {
		t.Fatal(err)
	}
	read := true
	results, total = server.QueryEmailPreviews(EmailQuery{Read: &read, Limit: 10})
	if total != 1 || len(results) != 1 || results[0].ID != "mail-two" {
		t.Fatalf("indexed read query = %#v, total %d", results, total)
	}
	if err := server.DeleteEmail("mail-two"); err != nil {
		t.Fatal(err)
	}
	_, total = server.QueryEmailPreviews(EmailQuery{Limit: 10})
	if total != 1 {
		t.Fatalf("indexed total after delete = %d", total)
	}
	stats := server.GetEmailStats()["index"].(map[string]interface{})
	if stats["enabled"] != true || stats["ready"] != true || stats["backend"] != "sqlite" {
		t.Fatalf("index stats = %#v", stats)
	}
}

func TestMailboxIndexUsesStableConstantTimeStorePositions(t *testing.T) {
	directory := t.TempDir()
	index, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	save := func(id string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(directory, id+".eml"), validMessage(id), 0600); err != nil {
			t.Fatal(err)
		}
		email := &types.Email{ID: id, Subject: id, Time: time.Now()}
		envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
		if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"position-one", "position-two", "position-three"} {
		save(id)
	}
	if err := server.DeleteEmail("position-two"); err != nil {
		t.Fatal(err)
	}
	save("position-four")

	server.storeMutex.RLock()
	thirdPosition := server.storePositionByID["position-three"]
	fourthPosition := server.storePositionByID["position-four"]
	nextPosition := server.nextStorePosition
	server.storeMutex.RUnlock()
	if thirdPosition != 2 || fourthPosition != 3 || nextPosition != 4 {
		t.Fatalf("stable positions = third %d, fourth %d, next %d", thirdPosition, fourthPosition, nextPosition)
	}
	results, total := server.QueryEmailPreviews(EmailQuery{SortBy: "store", SortOrder: "desc", Limit: 10})
	if total != 3 || len(results) != 3 || results[0].ID != "position-four" || results[1].ID != "position-three" || results[2].ID != "position-one" {
		t.Fatalf("descending stable store order = %#v, total %d", results, total)
	}
}

func TestMailboxIndexRebuildSerializesReadStateMutations(t *testing.T) {
	directory := t.TempDir()
	sqliteIndex, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	index := &blockingRebuildMailboxIndex{MailboxIndex: sqliteIndex}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = sqliteIndex.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "read-during-rebuild"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), validMessage(id), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Unread", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	started, release := index.blockNextRebuild()
	rebuilt := make(chan error, 1)
	go func() { rebuilt <- server.rebuildMailboxIndex() }()
	<-started
	serialized := !server.storeMutex.TryLock()
	if !serialized {
		server.storeMutex.Unlock()
	}
	readDone := make(chan error, 1)
	go func() { readDone <- server.ReadEmail(id) }()
	close(release)
	if err := <-rebuilt; err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if !serialized {
		t.Fatal("mailbox index rebuild released the store lock before committing its snapshot")
	}
	read := true
	results, total := server.QueryEmailPreviews(EmailQuery{Read: &read, Limit: 10})
	if total != 1 || len(results) != 1 || results[0].ID != id {
		t.Fatalf("read state after overlapping rebuild = %#v, total %d", results, total)
	}
}

func TestMailboxIndexDeletionSerializesReadStateMutations(t *testing.T) {
	directory := t.TempDir()
	sqliteIndex, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	index := &blockingDeleteMailboxIndex{
		MailboxIndex: sqliteIndex,
		started:      make(chan struct{}),
		release:      make(chan struct{}),
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = sqliteIndex.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "read-during-delete"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), validMessage(id), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Unread", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	deleted := make(chan struct{})
	go func() {
		server.deleteEmailFromRuntimeState(id)
		close(deleted)
	}()
	<-index.started
	serialized := !server.storeMutex.TryLock()
	if !serialized {
		server.storeMutex.Unlock()
		close(index.release)
		<-deleted
		t.Fatal("runtime-state deletion released the store lock after deleting the index row")
	}
	readDone := make(chan error, 1)
	go func() { readDone <- server.ReadEmail(id) }()
	close(index.release)
	<-deleted
	if err := <-readDone; err == nil {
		t.Fatal("ReadEmail found message after runtime-state deletion")
	}
	results, total := server.QueryEmailPreviews(EmailQuery{Limit: 10})
	if total != 0 || len(results) != 0 {
		t.Fatalf("indexed query retained deleted message: %#v, total %d", results, total)
	}
}

func TestValidateMailboxIndexPathRejectsManagedNamespaces(t *testing.T) {
	directory := t.TempDir()
	for name, path := range map[string]string{
		"metadata root":  filepath.Join(directory, metadataDirectoryName),
		"metadata child": filepath.Join(directory, metadataDirectoryName, "mailbox.db"),
		"deletion fence": deletionFencePath(directory, "message"),
		"rollback fence": rollbackFencePath(directory, "message"),
		"temporary file": filepath.Join(directory, storageTempPrefix+"mailbox.db"),
		"temporary tree": filepath.Join(directory, storageTempPrefix+"index", "mailbox.db"),
		"quarantine":     filepath.Join(directory, quarantineDirName, "mailbox.db"),
		"webhook outbox": filepath.Join(directory, webhookOutboxDirectoryName, "mailbox.db"),
		"mail directory": directory,
		"metadata case alias": filepath.Join(
			directory, strings.ToUpper(metadataDirectoryName), "mailbox.db",
		),
		"temporary case alias": filepath.Join(
			directory, strings.ToUpper(storageTempPrefix)+"index", "mailbox.db",
		),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateMailboxIndexPath(directory, path); err == nil {
				t.Fatalf("ValidateMailboxIndexPath(%q) accepted managed storage", path)
			}
		})
	}
	for name, path := range map[string]string{
		"root index":   filepath.Join(directory, "mailbox.db"),
		"nested index": filepath.Join(directory, ".index", "mailbox.db"),
		"external":     filepath.Join(t.TempDir(), "mailbox.db"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateMailboxIndexPath(directory, path); err != nil {
				t.Fatalf("ValidateMailboxIndexPath(%q) rejected safe storage: %v", path, err)
			}
		})
	}
}

func TestValidateMailboxIndexPathResolvesFilesystemAliases(t *testing.T) {
	directory := t.TempDir()
	metadataRoot := filepath.Join(directory, metadataDirectoryName)
	if err := os.MkdirAll(metadataRoot, 0755); err != nil {
		t.Fatal(err)
	}
	managedAlias := filepath.Join(directory, "metadata-link")
	if err := os.Symlink(metadataRoot, managedAlias); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if path := filepath.Join(managedAlias, "mailbox.db"); ValidateMailboxIndexPath(directory, path) == nil {
		t.Fatalf("ValidateMailboxIndexPath(%q) accepted a symlink into managed storage", path)
	}

	externalRoot := t.TempDir()
	outwardDirectory := t.TempDir()
	outwardMetadata := filepath.Join(outwardDirectory, metadataDirectoryName)
	if err := os.Symlink(externalRoot, outwardMetadata); err != nil {
		t.Fatal(err)
	}
	if path := filepath.Join(outwardMetadata, "mailbox.db"); ValidateMailboxIndexPath(outwardDirectory, path) == nil {
		t.Fatalf("ValidateMailboxIndexPath(%q) accepted a managed namespace symlinked outside the mail directory", path)
	}

	externalAlias := filepath.Join(directory, "external-link")
	if err := os.Symlink(externalRoot, externalAlias); err != nil {
		t.Fatal(err)
	}
	if path := filepath.Join(externalAlias, "mailbox.db"); ValidateMailboxIndexPath(directory, path) != nil {
		t.Fatalf("ValidateMailboxIndexPath(%q) rejected a symlink to external storage", path)
	}
}

func TestReloadRebuildsSQLiteStorePositionsAfterSorting(t *testing.T) {
	directory := t.TempDir()
	index, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	laterID, earlierID := "Aa11Bb22", "Zz99Yy88"
	later := time.Unix(1_700_000_100, 0)
	earlier := later.Add(-time.Minute)
	for _, item := range []struct {
		id       string
		received time.Time
	}{{laterID, later}, {earlierID, earlier}} {
		path := filepath.Join(directory, item.id+".eml")
		if err := os.WriteFile(path, validMessage(item.id), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, item.received, item.received); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.LoadMailsFromDirectory(); err != nil {
		t.Fatal(err)
	}
	results, total := server.QueryEmailPreviews(EmailQuery{SortBy: "store", Limit: 10})
	if total != 2 || len(results) != 2 || results[0].ID != earlierID || results[1].ID != laterID {
		t.Fatalf("indexed reload order = %#v, total %d", results, total)
	}
}

func TestStartupPreservesSQLiteIndexInsideGeneratedIDDirectory(t *testing.T) {
	directory := t.TempDir()
	indexDirectory := filepath.Join(directory, "Ab12Cd34")
	indexPath := filepath.Join(indexDirectory, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("startup moved the configured SQLite index: %v", err)
	}
	if _, err := os.Stat(indexDirectory); err != nil {
		t.Fatalf("startup moved the configured SQLite directory: %v", err)
	}
	status := server.GetEmailStats()["index"].(map[string]interface{})
	if status["ready"] != true {
		t.Fatalf("configured index was not ready after startup: %#v", status)
	}
}

func TestStartupPreservesSQLiteIndexWithEMLSuffix(t *testing.T) {
	directory := t.TempDir()
	indexPath := filepath.Join(directory, "Ab12Cd34.eml")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("startup parsed or moved the SQLite index as EML: %v", err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("SQLite index was published as email: %d", got)
	}
	status := server.GetEmailStats()["index"].(map[string]interface{})
	if status["ready"] != true {
		t.Fatalf("configured index was not ready after startup: %#v", status)
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatalf("delete-all treated the SQLite index as an email: %v", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("delete-all removed the SQLite index: %v", err)
	}
}

func TestDeleteAllPreservesNestedSQLiteIndex(t *testing.T) {
	directory := t.TempDir()
	indexPath := filepath.Join(directory, ".index", "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	id := "nested-index-mail"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte("message"), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Nested index", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("nested SQLite index was removed: %v", err)
	}
	results, total := server.QueryEmailPreviews(EmailQuery{Limit: 10})
	if total != 0 || len(results) != 0 {
		t.Fatalf("query after delete-all = %#v, total %d", results, total)
	}
	status := server.GetEmailStats()["index"].(map[string]interface{})
	if status["ready"] != true {
		t.Fatalf("nested index was disabled: %#v", status)
	}
}

func TestDeleteRejectsSQLiteIndexInsideEmailDirectory(t *testing.T) {
	directory := t.TempDir()
	id := "index-owner"
	indexPath := filepath.Join(directory, id, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte("message"), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Index owner", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	if err := server.DeleteEmail(id); err == nil || !strings.Contains(err.Error(), "mailbox index") {
		t.Fatalf("DeleteEmail() error = %v, want protected-index error", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("SQLite index was removed: %v", err)
	}
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("email was removed despite protected index: %v", err)
	}
	if err := server.DeleteAllEmail(); err == nil || !strings.Contains(err.Error(), "mailbox index") {
		t.Fatalf("DeleteAllEmail() error = %v, want protected-index error", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("SQLite index was removed by delete-all: %v", err)
	}
}
