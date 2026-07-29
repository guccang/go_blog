package persistence

import (
	"database/sql"
	"errors"
	"fmt"
	"module"
	"os"
	"path/filepath"
	"strings"
	"time"

	"config"
	_ "modernc.org/sqlite"
)

var sqliteDB *sql.DB

// initSQLite 打开博客唯一事实来源。此处只建表，不执行任何历史数据导入。
func initSQLite() error {
	dataDir := filepath.Join(config.GetExePath(), "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "go_blog.db"))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON",
		`CREATE TABLE IF NOT EXISTS blogs (
			account TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL,
			create_time TEXT NOT NULL, modify_time TEXT NOT NULL, access_time TEXT NOT NULL,
			modify_num INTEGER NOT NULL DEFAULT 0, access_num INTEGER NOT NULL DEFAULT 0,
			auth_type INTEGER NOT NULL DEFAULT 1, tags TEXT NOT NULL DEFAULT '', encrypt INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (account, title)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			account TEXT PRIMARY KEY, password TEXT NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS login_sessions (
			session_id TEXT PRIMARY KEY, account TEXT NOT NULL, expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS blog_hooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT NOT NULL, event_type TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '', query TEXT NOT NULL DEFAULT '', session_id TEXT NOT NULL DEFAULT '',
			feature TEXT NOT NULL DEFAULT '', object_type TEXT NOT NULL DEFAULT '', object_id TEXT NOT NULL DEFAULT '',
			context_json TEXT NOT NULL DEFAULT '{}', result_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS blog_chunks (
			account TEXT NOT NULL, blog_title TEXT NOT NULL, chunk_index INTEGER NOT NULL,
			heading TEXT NOT NULL DEFAULT '', content TEXT NOT NULL,
			PRIMARY KEY (account, blog_title, chunk_index)
		)`,
		`CREATE TABLE IF NOT EXISTS media_assets (
			id TEXT PRIMARY KEY, account TEXT NOT NULL, storage_name TEXT NOT NULL,
			mime_type TEXT NOT NULL, size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pi_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT NOT NULL, provider TEXT NOT NULL,
			model TEXT NOT NULL, prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0, total_tokens INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS blog_chunks_fts USING fts5(
			account UNINDEXED, blog_title UNINDEXED, chunk_index UNINDEXED, heading, content
		)`,
		"CREATE INDEX IF NOT EXISTS idx_login_sessions_expiry ON login_sessions(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_blog_hooks_account_created ON blog_hooks(account, created_at DESC, id DESC)",
		"CREATE INDEX IF NOT EXISTS idx_blogs_account_modify ON blogs(account, modify_time DESC, title DESC)",
		"CREATE INDEX IF NOT EXISTS idx_blogs_account_access ON blogs(account, access_time DESC, modify_time DESC)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_account ON media_assets(account, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_pi_usage_account_created ON pi_usage(account, created_at DESC, id DESC)",
		`CREATE VIRTUAL TABLE IF NOT EXISTS blogs_fts USING fts5(account UNINDEXED, title, content,
			content='blogs', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS blogs_ai AFTER INSERT ON blogs BEGIN
			INSERT INTO blogs_fts(rowid, account, title, content) VALUES (new.rowid, new.account, new.title, new.content); END`,
		`CREATE TRIGGER IF NOT EXISTS blogs_ad AFTER DELETE ON blogs BEGIN
			INSERT INTO blogs_fts(blogs_fts, rowid, account, title, content) VALUES ('delete', old.rowid, old.account, old.title, old.content); END`,
		`CREATE TRIGGER IF NOT EXISTS blogs_au AFTER UPDATE ON blogs BEGIN
			INSERT INTO blogs_fts(blogs_fts, rowid, account, title, content) VALUES ('delete', old.rowid, old.account, old.title, old.content);
			INSERT INTO blogs_fts(rowid, account, title, content) VALUES (new.rowid, new.account, new.title, new.content); END`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	if err := ensureBlogHookColumns(db); err != nil {
		db.Close()
		return err
	}
	// SQLite auth_type 已是公开权限的唯一事实来源，清理旧文件时代的公开状态伪博客。
	for _, stmt := range []string{
		"DELETE FROM blog_chunks_fts WHERE blog_title='sys_blog_public_state'",
		"DELETE FROM blog_chunks WHERE blog_title='sys_blog_public_state'",
		"DELETE FROM blogs WHERE title='sys_blog_public_state'",
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return fmt.Errorf("remove legacy public state blog: %w", err)
		}
	}
	sqliteDB = db
	return nil
}

func ensureBlogHookColumns(db *sql.DB) error {
	columns := map[string]string{
		"session_id":   "TEXT NOT NULL DEFAULT ''",
		"feature":      "TEXT NOT NULL DEFAULT ''",
		"object_type":  "TEXT NOT NULL DEFAULT ''",
		"object_id":    "TEXT NOT NULL DEFAULT ''",
		"context_json": "TEXT NOT NULL DEFAULT '{}'",
		"result_json":  "TEXT NOT NULL DEFAULT '{}'",
	}
	rows, err := db.Query("PRAGMA table_info(blog_hooks)")
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for name, definition := range columns {
		if existing[name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE blog_hooks ADD COLUMN " + name + " " + definition); err != nil {
			return err
		}
	}
	return nil
}

func requireSQLite() *sql.DB {
	if sqliteDB == nil {
		panic("sqlite blog storage is not initialized")
	}
	return sqliteDB
}

func sqliteSaveBlog(account string, b *module.Blog) error {
	if b.Account == "" {
		b.Account = account
	}
	tx, err := requireSQLite().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previousContent string
	err = tx.QueryRow("SELECT content FROM blogs WHERE account=? AND title=?", account, b.Title).Scan(&previousContent)
	contentChanged := errors.Is(err, sql.ErrNoRows) || previousContent != b.Content
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(`INSERT INTO blogs(account,title,content,create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt)
		VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(account,title) DO UPDATE SET content=excluded.content,modify_time=excluded.modify_time,
		access_time=excluded.access_time,modify_num=excluded.modify_num,access_num=excluded.access_num,auth_type=excluded.auth_type,tags=excluded.tags,encrypt=excluded.encrypt`,
		account, b.Title, b.Content, b.CreateTime, b.ModifyTime, b.AccessTime, b.ModifyNum, b.AccessNum, b.AuthType, b.Tags, b.Encrypt)
	if err != nil {
		return err
	}
	if contentChanged {
		if err := rebuildBlogChunksTx(tx, account, b.Title, b.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanBlog(row interface{ Scan(...any) error }) (*module.Blog, error) {
	b := &module.Blog{}
	err := row.Scan(&b.Account, &b.Title, &b.Content, &b.CreateTime, &b.ModifyTime, &b.AccessTime, &b.ModifyNum, &b.AccessNum, &b.AuthType, &b.Tags, &b.Encrypt)
	return b, err
}

func sqliteGetBlog(account, title string) *module.Blog {
	b, err := scanBlog(requireSQLite().QueryRow(`SELECT account,title,content,create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt FROM blogs WHERE account=? AND title=?`, account, title))
	if err != nil {
		return nil
	}
	return b
}

func sqliteGetAll(account string) map[string]*module.Blog {
	rows, err := requireSQLite().Query(`SELECT account,title,content,create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt FROM blogs WHERE account=?`, account)
	if err != nil {
		return nil
	}
	defer rows.Close()
	blogs := make(map[string]*module.Blog)
	for rows.Next() {
		if b, err := scanBlog(rows); err == nil {
			blogs[b.Title] = b
		}
	}
	return blogs
}

// ListBlogSummaries 分页返回博客元数据和安全的正文前缀，供内容库生成轻量卡片。
// 日记和加密博客的正文始终返回空字符串。
func ListBlogSummaries(account string, limit, offset, authFlag int) ([]*module.Blog, error) {
	query := `SELECT account,title,
		CASE WHEN encrypt=0 AND (auth_type & ?)=0 THEN substr(content,1,100) ELSE '' END,
		create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt
		FROM blogs WHERE account=?`
	args := []any{module.EAuthType_diary | module.EAuthType_encrypt, account}
	if authFlag != 0 {
		query += " AND (auth_type & ?) != 0"
		args = append(args, authFlag)
	}
	query += " ORDER BY modify_time DESC, title DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := requireSQLite().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*module.Blog, 0, limit)
	for rows.Next() {
		if b, err := scanBlog(rows); err == nil {
			result = append(result, b)
		} else {
			return nil, err
		}
	}
	return result, rows.Err()
}

// ListRecentBlogSummaries 返回最近访问的正式博客及正文前缀，供主页生成轻量预览。
// 系统、管理、日记和加密内容不会进入“继续阅读”。
func ListRecentBlogSummaries(account string, limit, authFlag int) ([]*module.Blog, error) {
	if account == "" || limit <= 0 {
		return []*module.Blog{}, nil
	}
	query := `SELECT account,title,substr(content,1,100),create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt
		FROM blogs WHERE account=? AND encrypt=0 AND (auth_type & ?)=0
		AND title NOT LIKE 'sys\_%' ESCAPE '\'
		AND title NOT LIKE 'todolist-%'
		AND title NOT LIKE 'exercise-%'
		AND title NOT LIKE 'reading\_book\_%' ESCAPE '\'
		AND title NOT LIKE '目标\_%' ESCAPE '\'
		AND title NOT LIKE '月度目标\_%' ESCAPE '\'
		AND title NOT LIKE '年计划\_%' ESCAPE '\'`
	args := []any{account, module.EAuthType_diary | module.EAuthType_encrypt}
	if authFlag != 0 {
		query += " AND (auth_type & ?) != 0"
		args = append(args, authFlag)
	}
	query += ` ORDER BY
		CASE WHEN trim(access_time)='' THEN 1 ELSE 0 END,
		access_time DESC, modify_time DESC, title DESC LIMIT ?`
	args = append(args, limit)
	rows, err := requireSQLite().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*module.Blog, 0, limit)
	for rows.Next() {
		b, scanErr := scanBlog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func CountBlogs(account string) int {
	var n int
	_ = requireSQLite().QueryRow("SELECT COUNT(*) FROM blogs WHERE account=?", account).Scan(&n)
	return n
}

// BlogSearchResult is the minimal, display-safe result returned by SQLite FTS.
// Content is represented by a short highlighted excerpt instead of the full body.
type BlogSearchResult struct {
	Title   string
	Snippet string
}

// SearchBlogsFTS searches one account's non-sensitive blogs through SQLite FTS5.
// A limit of zero explicitly requests every matching result; the homepage uses a
// small limit first and only requests all results after the user asks for them.
func SearchBlogsFTS(account, query string, limit int) ([]BlogSearchResult, error) {
	terms := strings.Fields(query)
	if account == "" || len(terms) == 0 {
		return []BlogSearchResult{}, nil
	}
	if limit < 0 {
		limit = 5
	}
	queryLimit := limit
	if limit == 0 {
		queryLimit = -1 // SQLite uses -1 to mean no LIMIT.
	}
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	rows, err := requireSQLite().Query(`SELECT b.title,
		COALESCE(snippet(blogs_fts, 2, '<mark>', '</mark>', '…', 18), '')
		FROM blogs_fts JOIN blogs b ON b.rowid=blogs_fts.rowid
		WHERE blogs_fts MATCH ? AND b.account=? AND b.encrypt=0 AND (b.auth_type & ?) = 0
		ORDER BY bm25(blogs_fts) LIMIT ?`, strings.Join(quoted, " AND "), account, module.EAuthType_diary, queryLimit)
	if err != nil {
		return nil, err
	}
	results := make([]BlogSearchResult, 0, max(limit, 0))
	seen := make(map[string]bool)
	for rows.Next() {
		var result BlogSearchResult
		if err := rows.Scan(&result.Title, &result.Snippet); err != nil {
			rows.Close()
			return nil, err
		}
		results = append(results, result)
		seen[result.Title] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if limit > 0 && len(results) >= limit {
		return results, nil
	}

	// FTS5 的 unicode61 tokenizer 会把连续中文当成一个长 token。
	// 例如“灵妖养成”无法由精确词“灵妖”召回，因此用 instr 做子串补召回。
	whereParts := make([]string, 0, len(terms))
	args := []any{account, module.EAuthType_diary}
	for _, term := range terms {
		whereParts = append(whereParts, `(instr(lower(title), lower(?)) > 0 OR instr(lower(content), lower(?)) > 0)`)
		args = append(args, term, term)
	}
	fallbackLimit := -1
	if limit > 0 {
		fallbackLimit = limit - len(results) + len(seen)
	}
	args = append(args, fallbackLimit)
	fallbackRows, err := requireSQLite().Query(`SELECT title, content FROM blogs
		WHERE account=? AND encrypt=0 AND (auth_type & ?) = 0 AND `+
		strings.Join(whereParts, " AND ")+`
		ORDER BY modify_time DESC, title DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer fallbackRows.Close()
	for fallbackRows.Next() {
		var title, content string
		if err := fallbackRows.Scan(&title, &content); err != nil {
			return nil, err
		}
		if seen[title] {
			continue
		}
		results = append(results, BlogSearchResult{
			Title:   title,
			Snippet: substringSearchSnippet(title, content, terms),
		})
		seen[title] = true
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, fallbackRows.Err()
}

// SearchBlogsFTSPage 在完整召回顺序上按页返回结果，避免页面一次读取全部匹配项。
func SearchBlogsFTSPage(account, query string, limit, offset int) ([]BlogSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	results, err := SearchBlogsFTS(account, query, offset+limit)
	if err != nil {
		return nil, err
	}
	return sliceBlogSearchResults(results, limit, offset), nil
}

func sliceBlogSearchResults(results []BlogSearchResult, limit, offset int) []BlogSearchResult {
	if limit <= 0 {
		return []BlogSearchResult{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(results) {
		return []BlogSearchResult{}
	}
	end := min(len(results), offset+limit)
	return results[offset:end]
}

func substringSearchSnippet(title, content string, terms []string) string {
	source := strings.Join(strings.Fields(content), " ")
	if source == "" {
		source = title
	}
	sourceRunes := []rune(source)
	lowerRunes := []rune(strings.ToLower(source))
	matchStart, matchLength := -1, 0
	for _, term := range terms {
		termRunes := []rune(strings.ToLower(term))
		if len(termRunes) == 0 {
			continue
		}
		if index := indexRunes(lowerRunes, termRunes); index >= 0 && (matchStart < 0 || index < matchStart) {
			matchStart, matchLength = index, len(termRunes)
		}
	}
	if matchStart < 0 {
		sourceRunes = []rune(title)
		lowerRunes = []rune(strings.ToLower(title))
		for _, term := range terms {
			termRunes := []rune(strings.ToLower(term))
			if index := indexRunes(lowerRunes, termRunes); index >= 0 {
				matchStart, matchLength = index, len(termRunes)
				break
			}
		}
	}
	if matchStart < 0 {
		return string(sourceRunes[:min(len(sourceRunes), 80)])
	}
	start := max(0, matchStart-32)
	end := min(len(sourceRunes), matchStart+matchLength+32)
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(sourceRunes) {
		suffix = "…"
	}
	before := strings.ReplaceAll(string(sourceRunes[start:matchStart]), "<mark>", "")
	match := strings.ReplaceAll(string(sourceRunes[matchStart:matchStart+matchLength]), "<mark>", "")
	after := strings.ReplaceAll(string(sourceRunes[matchStart+matchLength:end]), "<mark>", "")
	return prefix + before + "<mark>" + match + "</mark>" + after + suffix
}

func indexRunes(source, target []rune) int {
	if len(target) == 0 || len(target) > len(source) {
		return -1
	}
	for start := 0; start <= len(source)-len(target); start++ {
		matched := true
		for offset := range target {
			if source[start+offset] != target[offset] {
				matched = false
				break
			}
		}
		if matched {
			return start
		}
	}
	return -1
}

func sqliteDeleteBlog(account, title string) error {
	tx, err := requireSQLite().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM blog_chunks_fts WHERE account=? AND blog_title=?", account, title); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM blog_chunks WHERE account=? AND blog_title=?", account, title); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM blogs WHERE account=? AND title=?", account, title); err != nil {
		return err
	}
	return tx.Commit()
}

func sqliteNow() string { return time.Now().Format("2006-01-02 15:04:05") }

func SaveUser(account, password string) error {
	_, err := requireSQLite().Exec(`INSERT INTO users(account,password,created_at) VALUES(?,?,?) ON CONFLICT(account) DO UPDATE SET password=excluded.password`, account, password, sqliteNow())
	return err
}

func GetUser(account string) *module.User {
	u := &module.User{}
	if err := requireSQLite().QueryRow("SELECT account,password FROM users WHERE account=?", account).Scan(&u.Account, &u.Password); err != nil {
		return nil
	}
	return u
}

// ListBlogAccountsWithoutCredentials reports migrated content owners that do
// not yet have a SQLite login. It never creates or guesses credentials.
func ListBlogAccountsWithoutCredentials() ([]string, error) {
	rows, err := requireSQLite().Query(`SELECT DISTINCT b.account FROM blogs b
		LEFT JOIN users u ON u.account=b.account WHERE u.account IS NULL ORDER BY b.account`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []string{}
	for rows.Next() {
		var account string
		if err := rows.Scan(&account); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func CreateLoginSession(sessionID, account string, expiresAt time.Time) error {
	_, err := requireSQLite().Exec("DELETE FROM login_sessions WHERE account=?", account)
	if err != nil {
		return err
	}
	_, err = requireSQLite().Exec("INSERT INTO login_sessions(session_id,account,expires_at,created_at) VALUES(?,?,?,?)", sessionID, account, expiresAt.Format("2006-01-02 15:04:05"), sqliteNow())
	return err
}

func DeleteLoginSessions(account string) error {
	_, err := requireSQLite().Exec("DELETE FROM login_sessions WHERE account=?", account)
	return err
}

func GetLoginSessionAccount(sessionID string) string {
	var account, expiresAt string
	if err := requireSQLite().QueryRow("SELECT account,expires_at FROM login_sessions WHERE session_id=?", sessionID).Scan(&account, &expiresAt); err != nil {
		return ""
	}
	expires, err := time.Parse("2006-01-02 15:04:05", expiresAt)
	if err != nil || !expires.After(time.Now()) {
		_, _ = requireSQLite().Exec("DELETE FROM login_sessions WHERE session_id=?", sessionID)
		return ""
	}
	return account
}

func CleanupExpiredLoginSessions() {
	_, _ = requireSQLite().Exec("DELETE FROM login_sessions WHERE expires_at <= ?", sqliteNow())
}

// BlogHookEvent is a durable, account-scoped record of a user intent. It keeps
// metadata only; blog bodies are deliberately excluded from the hook stream.
type BlogHookEvent struct {
	ID          int64
	Account     string
	SessionID   string
	EventType   string
	Feature     string
	ObjectType  string
	ObjectID    string
	Title       string
	Query       string
	ContextJSON string
	ResultJSON  string
	CreatedAt   string
}

func RecordBlogHook(event BlogHookEvent) error {
	_, err := requireSQLite().Exec(`INSERT INTO blog_hooks(account,event_type,title,query,session_id,feature,object_type,object_id,context_json,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		event.Account, event.EventType, event.Title, event.Query, event.SessionID, event.Feature, event.ObjectType, event.ObjectID, event.ContextJSON, event.ResultJSON, sqliteNow())
	return err
}

func ListBlogHooks(account string, limit int) ([]BlogHookEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := requireSQLite().Query(`SELECT id,account,session_id,event_type,feature,object_type,object_id,title,query,context_json,result_json,created_at FROM blog_hooks WHERE account=? ORDER BY id DESC LIMIT ?`, account, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]BlogHookEvent, 0, limit)
	for rows.Next() {
		var event BlogHookEvent
		if err := rows.Scan(&event.ID, &event.Account, &event.SessionID, &event.EventType, &event.Feature, &event.ObjectType, &event.ObjectID, &event.Title, &event.Query, &event.ContextJSON, &event.ResultJSON, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
