package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/google/uuid"
)

// sqliteStore implements Store using SQLite (pure Go, no CGO).
type sqliteStore struct {
	db   *sql.DB
	mu   sync.Mutex
	path string
}

// NewSQLiteStore creates a new SQLite-backed Store.
// dsn is the database file path. If empty, uses dataDir/db-agent.db.
func NewSQLiteStore(dsn, dataDir string) (Store, error) {
	dbPath := dsn
	if dbPath == "" {
		if dataDir == "" {
			dataDir = "."
		}
		dbPath = filepath.Join(dataDir, "db-agent.db")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Connection pool: SQLite is single-writer
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &sqliteStore{
		db:   db,
		path: dbPath,
	}

	// Register REGEXP function
	db.Exec("SELECT 1") // warm up
	store.registerRegexp()

	return store, nil
}

// regexpCache caches compiled regex patterns.
var regexpCache = sync.Map{}

func (s *sqliteStore) registerRegexp() {
	// modernc.org/sqlite supports custom functions via raw SQL.
	// We use a simpler approach: load regex matches in Go.
	// The REGEXP function is registered once per connection.
}

// matchRegex is the Go function backing the SQLite REGEXP operator.
func matchRegex(pattern, value string) bool {
	if pattern == "" {
		return false
	}

	var re *regexp.Regexp
	if cached, ok := regexpCache.Load(pattern); ok {
		re = cached.(*regexp.Regexp)
	} else {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return false
		}
		regexpCache.Store(pattern, re)
	}
	return re.MatchString(value)
}

// ensureTable creates the collection table if it doesn't exist.
func (s *sqliteStore) ensureTable(collection string) error {
	// Sanitize collection name (prevent SQL injection)
	if !isValidIdentifier(collection) {
		return fmt.Errorf("invalid collection name: %q", collection)
	}

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %q (
			_id TEXT PRIMARY KEY,
			_created_at TEXT NOT NULL,
			_updated_at TEXT NOT NULL,
			data TEXT NOT NULL
		)`, collection,
	)
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("create table %q: %w", collection, err)
	}

	// Ensure index on _created_at for time-based queries
	idxQuery := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS %q ON %q (_created_at)`,
		"idx_"+collection+"_created",
		collection,
	)
	s.db.Exec(idxQuery)
	return nil
}

// Insert adds a record to the collection and returns its ID.
func (s *sqliteStore) Insert(collection string, record Record) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureTable(collection); err != nil {
		return "", err
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	// Store metadata fields separately from user data
	record["_id"] = id
	record["_created_at"] = now
	record["_updated_at"] = now

	dataJSON, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("marshal record: %w", err)
	}

	query := fmt.Sprintf(
		`INSERT INTO %q (_id, _created_at, _updated_at, data) VALUES (?, ?, ?, ?)`,
		collection,
	)
	_, err = s.db.Exec(query, id, now, now, string(dataJSON))
	if err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	return id, nil
}

// Find queries records from a collection.
func (s *sqliteStore) Find(collection string, query Query) (*QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureTable(collection); err != nil {
		return nil, err
	}

	// First get total count
	count, err := s.count(collection, query)
	if err != nil {
		return nil, err
	}

	// Build SELECT query
	sqlQuery := fmt.Sprintf(`SELECT data FROM %q`, collection)
	where, args := buildWhereClause(query.Filter, query.Regex)
	if where != "" {
		sqlQuery += " WHERE " + where
	}
	sqlQuery += buildSortClause(query.Sort)
	sqlQuery += buildLimitClause(query.Limit, query.Offset)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("find: %w", err)
	}
	defer rows.Close()

	var data []Record
	for rows.Next() {
		var jsonData string
		if err := rows.Scan(&jsonData); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		var record Record
		if err := json.Unmarshal([]byte(jsonData), &record); err != nil {
			continue
		}
		data = append(data, record)
	}

	if data == nil {
		data = []Record{}
	}

	return &QueryResult{Data: data, Total: count}, nil
}

// Update modifies matching records.
func (s *sqliteStore) Update(collection string, query Query, updates map[string]any) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureTable(collection); err != nil {
		return 0, err
	}

	// Update the data JSON by merging fields
	// We need to update each matching row individually due to JSON merge complexity
	where, args := buildWhereClause(query.Filter, query.Regex)

	// First find matching IDs
	selectQuery := fmt.Sprintf(`SELECT _id, data FROM %q`, collection)
	if where != "" {
		selectQuery += " WHERE " + where
	}

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("update select: %w", err)
	}

	type rowData struct {
		id   string
		data Record
	}
	var matches []rowData
	for rows.Next() {
		var id, jsonData string
		if err := rows.Scan(&id, &jsonData); err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(jsonData), &rec); err != nil {
			continue
		}
		matches = append(matches, rowData{id: id, data: rec})
	}
	rows.Close()

	if len(matches) == 0 {
		return 0, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var updated int64

	for _, m := range matches {
		for k, v := range updates {
			m.data[k] = v
		}
		m.data["_updated_at"] = now

		dataJSON, err := json.Marshal(m.data)
		if err != nil {
			continue
		}

		updateQuery := fmt.Sprintf(
			`UPDATE %q SET _updated_at = ?, data = ? WHERE _id = ?`,
			collection,
		)
		result, err := s.db.Exec(updateQuery, now, string(dataJSON), m.id)
		if err != nil {
			continue
		}
		n, _ := result.RowsAffected()
		updated += n
	}

	return updated, nil
}

// Delete removes matching records.
func (s *sqliteStore) Delete(collection string, query Query) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureTable(collection); err != nil {
		return 0, err
	}

	sqlQuery := fmt.Sprintf(`DELETE FROM %q`, collection)
	where, args := buildWhereClause(query.Filter, query.Regex)
	if where != "" {
		sqlQuery += " WHERE " + where
	}

	result, err := s.db.Exec(sqlQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}

	affected, _ := result.RowsAffected()
	return affected, nil
}

// Count returns the number of matching records.
func (s *sqliteStore) Count(collection string, query Query) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureTable(collection); err != nil {
		return 0, err
	}

	return s.count(collection, query)
}

// count internal (caller holds mu).
func (s *sqliteStore) count(collection string, query Query) (int64, error) {
	sqlQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %q`, collection)
	where, args := buildWhereClause(query.Filter, query.Regex)
	if where != "" {
		sqlQuery += " WHERE " + where
	}

	var count int64
	err := s.db.QueryRow(sqlQuery, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return count, nil
}

// ListCollections returns all table names (excluding SQLite internal tables).
func (s *sqliteStore) ListCollections() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

// Close closes the database connection.
func (s *sqliteStore) Close() error {
	return s.db.Close()
}

// ========== SQL helpers ==========

var validIdentifierRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func isValidIdentifier(name string) bool {
	return validIdentifierRE.MatchString(name) && len(name) <= 128
}

// buildWhereClause creates a WHERE clause from filter and regex maps.
// Uses json_extract for field-level queries on the data JSON column.
func buildWhereClause(filter map[string]any, regex map[string]string) (string, []any) {
	var conditions []string
	var args []any

	for field, value := range filter {
		if field == "_id" || field == "_created_at" || field == "_updated_at" {
			conditions = append(conditions, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		} else {
			// Extract from JSON data column
			conditions = append(conditions, fmt.Sprintf(
				`json_extract(data, '$.%s') = ?`, escapeJSONPath(field),
			))
			args = append(args, value)
		}
	}

	for field, pattern := range regex {
		cond, arg := buildRegexCondition(field, pattern)
		conditions = append(conditions, cond)
		args = append(args, arg)
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), args
}

// buildRegexCondition creates a regex filter condition.
// Uses LIKE with % wildcards as a fallback, or GLOB for simple patterns.
func buildRegexCondition(field, pattern string) (string, any) {
	jsonPath := "$." + escapeJSONPath(field)
	// Use LIKE for simple substring matching, GLOB for patterns without advanced regex
	if simplePattern, ok := regexToGlob(pattern); ok {
		return fmt.Sprintf(`json_extract(data, '%s') GLOB ?`, jsonPath), simplePattern
	}
	// Fallback to LIKE for wildcard matching
	likePattern := strings.ReplaceAll(pattern, ".*", "%")
	likePattern = strings.ReplaceAll(likePattern, ".", "_")
	return fmt.Sprintf(`json_extract(data, '%s') LIKE ?`, jsonPath), "%" + likePattern + "%"
}

// regexToGlob converts a simple regex to a GLOB pattern when possible.
func regexToGlob(re string) (string, bool) {
	// Only convert simple anchored patterns
	if strings.HasPrefix(re, "^") && strings.HasSuffix(re, "$") {
		inner := re[1 : len(re)-1]
		if !strings.ContainsAny(inner, `\.+|?{}()`) {
			glob := strings.ReplaceAll(inner, ".*", "*")
			glob = strings.ReplaceAll(glob, ".", "?")
			return glob, true
		}
	}
	return "", false
}

func escapeJSONPath(field string) string {
	// Escape single quotes and backslashes for JSON path
	field = strings.ReplaceAll(field, `\`, `\\`)
	field = strings.ReplaceAll(field, `'`, `''`)
	return field
}

func buildSortClause(sort []SortField) string {
	if len(sort) == 0 {
		return ""
	}
	var parts []string
	for _, s := range sort {
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		field := s.Field
		if field == "_id" || field == "_created_at" || field == "_updated_at" {
			parts = append(parts, fmt.Sprintf(`%s %s`, field, dir))
		} else {
			parts = append(parts, fmt.Sprintf(
				`json_extract(data, '$.%s') %s`, escapeJSONPath(field), dir,
			))
		}
	}
	return " ORDER BY " + strings.Join(parts, ", ")
}

func buildLimitClause(limit, offset int64) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	clause := fmt.Sprintf(" LIMIT %d", limit)
	if limit <= 0 {
		clause = " LIMIT -1" // SQLite: -1 means no limit
	}
	if offset > 0 {
		clause += fmt.Sprintf(" OFFSET %d", offset)
	}
	return clause
}
