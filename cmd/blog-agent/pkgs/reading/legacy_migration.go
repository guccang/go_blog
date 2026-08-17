package reading

import (
	"blog"
	"config"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"os"
	"path/filepath"
	"strings"
)

const legacyReadingPrefix = "reading_book_"

type persistedReadingBook struct {
	Book *module.Book `json:"book"`
}

// MigrateLegacyReadingBooks 恢复旧 blogs_txt 中尚未进入 SQLite 的读书数据。
// 已存在且可解析的 SQLite 记录不会被旧文件覆盖，重复启动是幂等的。
func MigrateLegacyReadingBooks() error {
	root := filepath.Join(config.GetExePath(), "blogs_txt")
	accounts, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy reading root: %w", err)
	}

	migrated := 0
	for _, accountEntry := range accounts {
		if !accountEntry.IsDir() {
			continue
		}
		count, migrateErr := migrateLegacyReadingAccount(accountEntry.Name(), filepath.Join(root, accountEntry.Name()))
		if migrateErr != nil {
			return fmt.Errorf("migrate legacy reading for %s: %w", accountEntry.Name(), migrateErr)
		}
		migrated += count
	}
	log.InfoF(log.ModuleReading, "legacy reading_book_*.md migration completed: %d records restored", migrated)
	return nil
}

func migrateLegacyReadingAccount(account, accountRoot string) (int, error) {
	migrated := 0
	err := filepath.WalkDir(accountRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isLegacyReadingFile(entry.Name()) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		changed, err := importLegacyReadingBook(account, entry.Name(), content)
		if err != nil {
			log.ErrorF(log.ModuleReading, "skip invalid legacy reading file %s: %v", path, err)
			return nil
		}
		if changed {
			migrated++
		}
		return nil
	})
	return migrated, err
}

func isLegacyReadingFile(name string) bool {
	return strings.HasPrefix(name, legacyReadingPrefix) && strings.EqualFold(filepath.Ext(name), ".md")
}

func importLegacyReadingBook(account, fileName string, content []byte) (bool, error) {
	var source persistedReadingBook
	if err := json.Unmarshal(content, &source); err != nil {
		return false, fmt.Errorf("parse reading JSON: %w", err)
	}
	if source.Book == nil || source.Book.ID == "" || source.Book.Title == "" {
		return false, fmt.Errorf("reading JSON has no valid book")
	}

	titleWithExt := fileName
	titleWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	targetTitle := titleWithExt
	for _, candidate := range []string{titleWithExt, titleWithoutExt} {
		existing := blog.GetBlogWithAccount(account, candidate)
		if existing == nil {
			continue
		}
		var current persistedReadingBook
		if json.Unmarshal([]byte(existing.Content), &current) == nil &&
			current.Book != nil && current.Book.ID != "" && current.Book.Title != "" {
			return false, nil
		}
		targetTitle = candidate
		break
	}

	data := &module.UploadedBlogData{
		Title:    targetTitle,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     "reading",
		Account:  account,
	}
	if blog.GetBlogWithAccount(account, targetTitle) == nil {
		if result := blog.AddBlogWithAccount(account, data); result != 0 {
			return false, fmt.Errorf("add reading blog failed: %d", result)
		}
		return true, nil
	}
	if result := blog.ModifyBlogWithAccount(account, data); result != 0 {
		return false, fmt.Errorf("repair reading blog failed: %d", result)
	}
	return true, nil
}
