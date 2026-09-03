package mailserver

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const sqliteMailboxSchema = `
CREATE TABLE IF NOT EXISTS mailbox_index_v2 (
	id TEXT PRIMARY KEY,
	message_time_seconds INTEGER NOT NULL,
	message_time_nanoseconds INTEGER NOT NULL,
	received_at_seconds INTEGER NOT NULL,
	received_at_nanoseconds INTEGER NOT NULL,
	is_read INTEGER NOT NULL,
	subject_search TEXT NOT NULL,
	text_search TEXT NOT NULL,
	html_search TEXT NOT NULL,
	from_search TEXT NOT NULL,
	visible_recipients_search TEXT NOT NULL,
	bcc_addresses_search TEXT NOT NULL,
	first_from TEXT NOT NULL,
	size INTEGER NOT NULL,
	store_position INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS mailbox_index_v2_time ON mailbox_index_v2(message_time_seconds, message_time_nanoseconds);
CREATE INDEX IF NOT EXISTS mailbox_index_v2_received ON mailbox_index_v2(received_at_seconds, received_at_nanoseconds);
CREATE INDEX IF NOT EXISTS mailbox_index_v2_read ON mailbox_index_v2(is_read);
CREATE INDEX IF NOT EXISTS mailbox_index_v2_store ON mailbox_index_v2(store_position);
`

const sqliteMailboxUpsert = `
INSERT INTO mailbox_index_v2 (
	id, message_time_seconds, message_time_nanoseconds,
	received_at_seconds, received_at_nanoseconds, is_read, subject_search, text_search,
	html_search, from_search, visible_recipients_search, bcc_addresses_search,
	first_from, size, store_position
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	message_time_seconds=excluded.message_time_seconds,
	message_time_nanoseconds=excluded.message_time_nanoseconds,
	received_at_seconds=excluded.received_at_seconds,
	received_at_nanoseconds=excluded.received_at_nanoseconds,
	is_read=excluded.is_read, subject_search=excluded.subject_search,
	text_search=excluded.text_search, html_search=excluded.html_search,
	from_search=excluded.from_search,
	visible_recipients_search=excluded.visible_recipients_search,
	bcc_addresses_search=excluded.bcc_addresses_search,
	first_from=excluded.first_from, size=excluded.size, store_position=excluded.store_position
`

type SQLiteMailboxIndex struct {
	db              *sql.DB
	path            string
	resolvedPath    string
	afterQueryCount func()
}

func NewSQLiteMailboxIndex(path string) (*SQLiteMailboxIndex, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("SQLite mailbox index path cannot be empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite mailbox index path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0750); err != nil {
		return nil, fmt.Errorf("create SQLite mailbox index directory: %w", err)
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("create SQLite mailbox index: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure SQLite mailbox index: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close SQLite mailbox index: %w", err)
	}
	resolvedPath, err := resolveExistingPathIdentity(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite mailbox index identity: %w", err)
	}
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("open SQLite mailbox index: %w", err)
	}
	db.SetMaxOpenConns(1)
	index := &SQLiteMailboxIndex{db: db, path: absolute, resolvedPath: resolvedPath}
	for _, statement := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		sqliteMailboxSchema,
	} {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize SQLite mailbox index: %w", err)
		}
	}
	if err := secureSQLiteMailboxArtifacts(absolute); err != nil {
		_ = db.Close()
		return nil, err
	}
	return index, nil
}

func (index *SQLiteMailboxIndex) Backend() string { return "sqlite" }

func (index *SQLiteMailboxIndex) OwnsPath(path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, databasePath := range []string{index.path, index.resolvedPath} {
		if ownsSQLiteMailboxPathSpelling(absolute, databasePath) {
			return true
		}
	}
	candidateInfo, err := os.Stat(absolute)
	if err != nil {
		return false
	}
	for _, databasePath := range []string{index.path, index.resolvedPath} {
		if ownsSQLiteMailboxPathIdentity(candidateInfo, databasePath) {
			return true
		}
	}
	return false
}

func ownsSQLiteMailboxPathSpelling(candidate, databasePath string) bool {
	for _, owned := range []string{databasePath, databasePath + "-wal", databasePath + "-shm"} {
		if candidate == owned {
			return true
		}
	}
	return false
}

func ownsSQLiteMailboxPathIdentity(candidateInfo os.FileInfo, databasePath string) bool {
	for _, owned := range []string{databasePath, databasePath + "-wal", databasePath + "-shm"} {
		ownedInfo, err := os.Stat(owned)
		if err != nil {
			continue
		}
		if os.SameFile(candidateInfo, ownedInfo) {
			return true
		}
		if !candidateInfo.IsDir() {
			continue
		}
		for directory := filepath.Dir(owned); ; directory = filepath.Dir(directory) {
			directoryInfo, err := os.Stat(directory)
			if err == nil && os.SameFile(candidateInfo, directoryInfo) {
				return true
			}
			parent := filepath.Dir(directory)
			if parent == directory {
				break
			}
		}
	}
	return false
}

func secureSQLiteMailboxArtifacts(path string) error {
	for _, artifact := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(artifact, 0600); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("secure SQLite mailbox index artifact: %w", err)
		}
	}
	return nil
}

func (index *SQLiteMailboxIndex) Close() error { return index.db.Close() }

func (index *SQLiteMailboxIndex) Rebuild(records []IndexedEmail) error {
	transaction, err := index.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	if _, err := transaction.Exec("DELETE FROM mailbox_index_v2"); err != nil {
		return err
	}
	statement, err := transaction.Prepare(sqliteMailboxUpsert)
	if err != nil {
		return err
	}
	defer func() { _ = statement.Close() }()
	for _, record := range records {
		if err := execSQLiteMailboxUpsert(statement, record); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (index *SQLiteMailboxIndex) Upsert(record IndexedEmail) error {
	_, err := index.db.Exec(sqliteMailboxUpsert, sqliteMailboxValues(record)...)
	return err
}

func execSQLiteMailboxUpsert(statement *sql.Stmt, record IndexedEmail) error {
	_, err := statement.Exec(sqliteMailboxValues(record)...)
	return err
}

func sqliteMailboxValues(record IndexedEmail) []interface{} {
	read := 0
	if record.Read {
		read = 1
	}
	return []interface{}{
		record.ID, record.MessageTime.Unix(), record.MessageTime.Nanosecond(),
		record.ReceivedAt.Unix(), record.ReceivedAt.Nanosecond(), read,
		record.SubjectSearch, record.TextSearch, record.HTMLSearch, record.FromSearch,
		record.VisibleRecipientsSearch, record.BCCAddressesSearch, record.FirstFrom,
		record.Size, record.StorePosition,
	}
}

func (index *SQLiteMailboxIndex) Delete(id string) error {
	_, err := index.db.Exec("DELETE FROM mailbox_index_v2 WHERE id = ?", id)
	return err
}

func (index *SQLiteMailboxIndex) Clear() error {
	_, err := index.db.Exec("DELETE FROM mailbox_index_v2")
	return err
}

func (index *SQLiteMailboxIndex) Query(query EmailQuery) ([]IndexedEmailResult, int, error) {
	where, args := sqliteMailboxWhere(query)
	transaction, err := index.db.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = transaction.Rollback() }()
	var total int
	if err := transaction.QueryRow("SELECT COUNT(*) FROM mailbox_index_v2"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if index.afterQueryCount != nil {
		index.afterQueryCount()
	}
	if query.Limit <= 0 {
		if err := transaction.Commit(); err != nil {
			return nil, 0, err
		}
		return []IndexedEmailResult{}, total, nil
	}
	order := sqliteMailboxOrder(query.SortBy, query.SortOrder)
	pageArgs := append(append([]interface{}{}, args...), query.Limit, max(query.Offset, 0))
	rows, err := transaction.Query("SELECT id, is_read FROM mailbox_index_v2"+where+order+" LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	capacity := min(query.Limit, total)
	results := make([]IndexedEmailResult, 0, capacity)
	for rows.Next() {
		var result IndexedEmailResult
		var read int
		if err := rows.Scan(&result.ID, &read); err != nil {
			return nil, 0, err
		}
		result.Read = read != 0
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := transaction.Commit(); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func sqliteMailboxWhere(query EmailQuery) (string, []interface{}) {
	clauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 8)
	if text := strings.ToLower(query.Text); text != "" {
		parts := []string{"instr(subject_search, ?) > 0", "instr(text_search, ?) > 0"}
		args = append(args, text, text)
		if !query.ExcludeHTML {
			parts = append(parts, "instr(html_search, ?) > 0")
			args = append(args, text)
		}
		if query.SearchAddresses {
			parts = append(parts, "instr(from_search, ?) > 0", "instr(visible_recipients_search, ?) > 0")
			args = append(args, text, text)
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if value := strings.ToLower(query.From); value != "" {
		clauses = append(clauses, "instr(from_search, ?) > 0")
		args = append(args, value)
	}
	if value := strings.ToLower(query.To); value != "" {
		clauses = append(clauses, "(instr(visible_recipients_search, ?) > 0 OR instr(bcc_addresses_search, ?) > 0)")
		args = append(args, value, value)
	}
	if query.DateFrom != nil {
		seconds, nanoseconds := query.DateFrom.Unix(), query.DateFrom.Nanosecond()
		clauses = append(clauses, "(message_time_seconds > ? OR (message_time_seconds = ? AND message_time_nanoseconds >= ?))")
		args = append(args, seconds, seconds, nanoseconds)
	}
	if query.DateTo != nil {
		seconds, nanoseconds := query.DateTo.Unix(), query.DateTo.Nanosecond()
		clauses = append(clauses, "(message_time_seconds < ? OR (message_time_seconds = ? AND message_time_nanoseconds <= ?))")
		args = append(args, seconds, seconds, nanoseconds)
	}
	if query.Read != nil {
		value := 0
		if *query.Read {
			value = 1
		}
		clauses = append(clauses, "is_read = ?")
		args = append(args, value)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func sqliteMailboxOrder(sortBy, sortOrder string) string {
	direction := " DESC"
	if sortOrder == "asc" {
		direction = " ASC"
	}
	switch sortBy {
	case "time":
		return " ORDER BY message_time_seconds" + direction + ", message_time_nanoseconds" + direction + ", store_position ASC"
	case "received":
		return " ORDER BY received_at_seconds" + direction + ", received_at_nanoseconds" + direction + ", store_position ASC"
	case "subject":
		return " ORDER BY subject_search" + direction + ", store_position ASC"
	case "from":
		return " ORDER BY first_from" + direction + ", store_position ASC"
	case "size":
		return " ORDER BY size" + direction + ", store_position ASC"
	case "store":
		// Store order is the one sort whose established default is insertion
		// order. Only reverse it when callers explicitly request descending.
		if sortOrder == "desc" {
			return " ORDER BY store_position DESC"
		}
		return " ORDER BY store_position ASC"
	case "":
		return " ORDER BY message_time_seconds DESC, message_time_nanoseconds DESC, store_position ASC"
	default:
		return " ORDER BY store_position ASC"
	}
}
