package persistence

import (
	"bufio"
	"config"
	"crypto/sha256"
	"fmt"
	"module"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MigrationReport struct{ Files, Blogs, Skipped int }

// MigrateMarkdownBlogs 显式导入 blogs_txt 下各账户的 Markdown。
// 不由正常启动流程调用；源文件不会被修改或删除，重复运行是幂等的。
func MigrateMarkdownBlogs(root string) (MigrationReport, error) {
	report := MigrationReport{}
	accounts, err := os.ReadDir(root)
	if err != nil {
		return report, err
	}
	if _, err := requireSQLite().Exec(`CREATE TABLE IF NOT EXISTS blog_migrations (source_path TEXT PRIMARY KEY, checksum TEXT NOT NULL, migrated_at TEXT NOT NULL)`); err != nil {
		return report, err
	}
	for _, accountEntry := range accounts {
		if !accountEntry.IsDir() {
			continue
		}
		account, accountRoot := accountEntry.Name(), filepath.Join(root, accountEntry.Name())
		err := filepath.WalkDir(accountRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			report.Files++
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(accountRoot, path)
			if err != nil {
				return err
			}
			title := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
			info, err := entry.Info()
			if err != nil {
				return err
			}
			stamp := info.ModTime().Format("2006-01-02 15:04:05")
			b := legacyBlogMetadata(account, title)
			if b == nil {
				b = &module.Blog{Title: title, CreateTime: stamp, ModifyTime: stamp, AccessTime: stamp, AuthType: module.EAuthType_private}
			}
			// Markdown 是用户可审计的原始正文，迁移时优先保留它；Redis 仅补充元数据。
			b.Title, b.Content, b.Account = title, string(content), account
			if err := sqliteSaveBlog(account, b); err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			_, _ = requireSQLite().Exec(`INSERT INTO blog_migrations(source_path,checksum,migrated_at) VALUES(?,?,?) ON CONFLICT(source_path) DO UPDATE SET checksum=excluded.checksum,migrated_at=excluded.migrated_at`, path, fmt.Sprintf("%x", sum[:]), time.Now().Format("2006-01-02 15:04:05"))
			report.Blogs++
			return nil
		})
		if err != nil {
			return report, err
		}
		if err := migrateAccountCredential(account, accountRoot); err != nil {
			return report, err
		}
	}
	return report, nil
}

// migrateAccountCredential 将每个账户目录内的 sys_conf 登录凭证迁入 SQLite。
func migrateAccountCredential(account, accountRoot string) error {
	paths := []string{filepath.Join(accountRoot, "sys_conf.md"), filepath.Join(accountRoot, "sys_conf_"+account+".md")}
	for _, path := range paths {
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		values := map[string]string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if password := values["pwd"]; password != "" {
			username := values["admin"]
			if username == "" {
				username = account
			}
			if err := SaveUser(username, password); err != nil {
				return err
			}
		}
	}
	return nil
}

func legacyBlogMetadata(account, title string) *module.Blog {
	if client == nil {
		return nil
	}
	keys := []string{fmt.Sprintf("%s:blog@%s", account, title)}
	if account == config.GetAdminAccount() {
		keys = append(keys, fmt.Sprintf("blog@%s", title))
	}
	for _, key := range keys {
		values, err := client.HGetAll(key).Result()
		if err == nil && len(values) > 0 {
			return toBlog(values)
		}
	}
	return nil
}
