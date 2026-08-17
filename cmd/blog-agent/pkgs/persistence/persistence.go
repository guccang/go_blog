package persistence

import (
	"fmt"
	"module"
	log "mylog"
	"sync"
)

var persistence sync.Mutex

func Info() {
	log.Debug(log.ModulePersistence, "SQLite persistence")
}

// Init initializes the only runtime storage: SQLite.
func Init() {
	persistence.Lock()
	defer persistence.Unlock()
	if err := initSQLite(); err != nil {
		panic(fmt.Sprintf("initialize SQLite blog storage failed: %v", err))
	}
	log.InfoF(log.ModulePersistence, "SQLite storage initialized")
	go func() {
		if err := EnsureBlogChunks(); err != nil {
			log.ErrorF(log.ModulePersistence, "build blog chunks failed: %v", err)
			return
		}
		log.InfoF(log.ModulePersistence, "blog chunk index ready")
	}()
}

func SaveBlog(account string, item *module.Blog) {
	persistence.Lock()
	defer persistence.Unlock()
	if err := sqliteSaveBlog(account, item); err != nil {
		log.ErrorF(log.ModulePersistence, "sqlite save blog title=%s err=%s", item.Title, err.Error())
	}
}

func SaveBlogs(account string, blogs map[string]*module.Blog) {
	persistence.Lock()
	defer persistence.Unlock()
	for _, item := range blogs {
		if err := sqliteSaveBlog(account, item); err != nil {
			log.ErrorF(log.ModulePersistence, "sqlite save blog title=%s err=%s", item.Title, err.Error())
		}
	}
}

func GetBlogsByAccount(account string) map[string]*module.Blog {
	persistence.Lock()
	defer persistence.Unlock()
	return sqliteGetAll(account)
}

func GetBlogWithAccount(account, title string) *module.Blog {
	persistence.Lock()
	defer persistence.Unlock()
	return sqliteGetBlog(account, title)
}

// FindPublicBlogAccountsByTitle returns owners of an unencrypted, non-diary public blog.
// It is used to resolve legacy public links that do not contain an account parameter.
func FindPublicBlogAccountsByTitle(title string) ([]string, error) {
	persistence.Lock()
	defer persistence.Unlock()
	rows, err := requireSQLite().Query(`SELECT account FROM blogs
		WHERE title=? AND encrypt=0 AND (auth_type & ?) != 0 AND (auth_type & ?) = 0
		ORDER BY account`, title, module.EAuthType_public, module.EAuthType_diary|module.EAuthType_encrypt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]string, 0, 1)
	for rows.Next() {
		var account string
		if err := rows.Scan(&account); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func ListBlogsByTitlePrefixWithAccount(account, prefix string) ([]*module.Blog, error) {
	persistence.Lock()
	defer persistence.Unlock()
	return ListBlogsByTitlePrefix(account, prefix)
}

// ListBlogAccounts returns every account that owns blog content. It is used by
// one-time, idempotent data migrations that must preserve multi-account data.
func ListBlogAccounts() ([]string, error) {
	persistence.Lock()
	defer persistence.Unlock()
	rows, err := requireSQLite().Query("SELECT DISTINCT account FROM blogs ORDER BY account")
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

func DeleteBlogWithAccount(account, title string) int {
	persistence.Lock()
	defer persistence.Unlock()
	if err := sqliteDeleteBlog(account, title); err != nil {
		log.ErrorF(log.ModulePersistence, "sqlite delete blog title=%s err=%s", title, err.Error())
		return 1
	}
	return 0
}

func SaveBlogWithAccount(account string, item *module.Blog) { SaveBlog(account, item) }
func SaveBlogsWithAccount(account string, blogs map[string]*module.Blog) {
	SaveBlogs(account, blogs)
}
