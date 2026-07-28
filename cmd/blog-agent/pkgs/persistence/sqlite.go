package persistence

import (
	"database/sql"
	"fmt"
	"module"
	"os"
	"path/filepath"
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
		"CREATE INDEX IF NOT EXISTS idx_login_sessions_expiry ON login_sessions(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_blogs_account_modify ON blogs(account, modify_time DESC, title DESC)",
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
	sqliteDB = db
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
	_, err := requireSQLite().Exec(`INSERT INTO blogs(account,title,content,create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt)
		VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(account,title) DO UPDATE SET content=excluded.content,modify_time=excluded.modify_time,
		access_time=excluded.access_time,modify_num=excluded.modify_num,access_num=excluded.access_num,auth_type=excluded.auth_type,tags=excluded.tags,encrypt=excluded.encrypt`,
		account, b.Title, b.Content, b.CreateTime, b.ModifyTime, b.AccessTime, b.ModifyNum, b.AccessNum, b.AuthType, b.Tags, b.Encrypt)
	return err
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

// ListBlogSummaries 分页取元数据，正文保持空字符串，供首页等列表页惰性展示。
func ListBlogSummaries(account string, limit, offset, authFlag int) ([]*module.Blog, error) {
	query := `SELECT account,title,'' as content,create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt FROM blogs WHERE account=?`
	args := []any{account}
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

func CountBlogs(account string) int {
	var n int
	_ = requireSQLite().QueryRow("SELECT COUNT(*) FROM blogs WHERE account=?", account).Scan(&n)
	return n
}

func sqliteDeleteBlog(account, title string) error {
	_, err := requireSQLite().Exec("DELETE FROM blogs WHERE account=? AND title=?", account, title)
	return err
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
