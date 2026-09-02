package mailserver

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const sqliteMailboxSchema = `
CREATE TABLE IF NOT EXISTS mailbox_index (
	id TEXT PRIMARY KEY,
	message_time INTEGER NOT NULL,
	received_at INTEGER NOT NULL,
	is_read INTEGER NOT NULL,
	subject_search TEXT NOT NULL,
	text_search TEXT NOT NULL,
	html_search TEXT NOT NULL,
	from_search TEXT NOT NULL,
	recipients_search TEXT NOT NULL,
	first_from TEXT NOT NULL,
	size INTEGER NOT NULL,
	store_position INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS mailbox_index_time ON mailbox_index(message_time);
CREATE INDEX IF NOT EXISTS mailbox_index_read ON mailbox_index(is_read);
CREATE INDEX IF NOT EXISTS mailbox_index_store ON mailbox_index(store_position);
`

const sqliteMailboxUpsert = `
INSERT INTO mailbox_index (
	id, message_time, received_at, is_read, subject_search, text_search,
	html_search, from_search, recipients_search, first_from, size, store_position
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	message_time=excluded.message_time, received_at=excluded.received_at,
	is_read=excluded.is_read, subject_search=excluded.subject_search,
	text_search=excluded.text_search, html_search=excluded.html_search,
	from_search=excluded.from_search, recipients_search=excluded.recipients_search,
	first_from=excluded.first_from, size=excluded.size, store_position=excluded.store_position
`

type SQLiteMailboxIndex struct {
	db   *sql.DB
	path string
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
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("open SQLite mailbox index: %w", err)
	}
	db.SetMaxOpenConns(1)
	index := &SQLiteMailboxIndex{db: db, path: absolute}
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
	if err := os.Chmod(absolute, 0600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure SQLite mailbox index: %w", err)
	}
	return index, nil
}

func (index *SQLiteMailboxIndex) Backend() string { return "sqlite" }

func (index *SQLiteMailboxIndex) OwnsPath(path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return absolute == index.path || absolute == index.path+"-wal" || absolute == index.path+"-shm"
}

func (index *SQLiteMailboxIndex) Close() error { return index.db.Close() }

func (index *SQLiteMailboxIndex) Rebuild(records []IndexedEmail) error {
	transaction, err := index.db.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.Exec("DELETE FROM mailbox_index"); err != nil {
		return err
	}
	statement, err := transaction.Prepare(sqliteMailboxUpsert)
	if err != nil {
		return err
	}
	defer statement.Close()
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
		record.ID, record.MessageTime.UnixNano(), record.ReceivedAt.UnixNano(), read,
		record.SubjectSearch, record.TextSearch, record.HTMLSearch, record.FromSearch,
		record.RecipientsSearch, record.FirstFrom, record.Size, record.StorePosition,
	}
}

func (index *SQLiteMailboxIndex) Delete(id string) error {
	_, err := index.db.Exec("DELETE FROM mailbox_index WHERE id = ?", id)
	return err
}

func (index *SQLiteMailboxIndex) Clear() error {
	_, err := index.db.Exec("DELETE FROM mailbox_index")
	return err
}

func (index *SQLiteMailboxIndex) Query(query EmailQuery) ([]IndexedEmailResult, int, error) {
	where, args := sqliteMailboxWhere(query)
	var total int
	if err := index.db.QueryRow("SELECT COUNT(*) FROM mailbox_index"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 {
		return []IndexedEmailResult{}, total, nil
	}
	order := sqliteMailboxOrder(query.SortBy, query.SortOrder)
	pageArgs := append(append([]interface{}{}, args...), query.Limit, max(query.Offset, 0))
	rows, err := index.db.Query("SELECT id, is_read FROM mailbox_index"+where+order+" LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	results := make([]IndexedEmailResult, 0, query.Limit)
	for rows.Next() {
		var result IndexedEmailResult
		var read int
		if err := rows.Scan(&result.ID, &read); err != nil {
			return nil, 0, err
		}
		result.Read = read != 0
		results = append(results, result)
	}
	return results, total, rows.Err()
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
			parts = append(parts, "instr(from_search, ?) > 0", "instr(recipients_search, ?) > 0")
			args = append(args, text, text)
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if value := strings.ToLower(query.From); value != "" {
		clauses = append(clauses, "instr(from_search, ?) > 0")
		args = append(args, value)
	}
	if value := strings.ToLower(query.To); value != "" {
		clauses = append(clauses, "instr(recipients_search, ?) > 0")
		args = append(args, value)
	}
	if query.DateFrom != nil {
		clauses = append(clauses, "message_time >= ?")
		args = append(args, query.DateFrom.UnixNano())
	}
	if query.DateTo != nil {
		clauses = append(clauses, "message_time <= ?")
		args = append(args, query.DateTo.UnixNano())
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
		return " ORDER BY message_time" + direction + ", store_position ASC"
	case "subject":
		return " ORDER BY subject_search" + direction + ", store_position ASC"
	case "from":
		return " ORDER BY first_from" + direction + ", store_position ASC"
	case "size":
		return " ORDER BY size" + direction + ", store_position ASC"
	case "store":
		return " ORDER BY store_position ASC"
	case "":
		return " ORDER BY message_time DESC, store_position ASC"
	default:
		return " ORDER BY store_position ASC"
	}
}
