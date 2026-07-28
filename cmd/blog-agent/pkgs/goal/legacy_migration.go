package goal

import (
	"blog"
	"config"
	"encoding/json"
	"fmt"
	log "mylog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"persistence"
)

var legacyGoalFileTitle = regexp.MustCompile(`^目标_(daily|weekly|monthly|yearly)_(.+)$`)

// MigrateLegacyGoals imports only legacy 目标_*.md files into SQLite. Other
// Markdown content is ignored. Existing target fields and tasks are preserved,
// while missing tasks are merged by ID, making repeated startup idempotent.
func MigrateLegacyGoals() error {
	accounts, err := legacyGoalAccounts()
	if err != nil {
		return err
	}
	migrated := 0
	for _, account := range accounts {
		count, err := migrateGoalFilesForAccount(account)
		if err != nil {
			return fmt.Errorf("migrate legacy goals for %s: %w", account, err)
		}
		migrated += count
	}
	log.InfoF(log.ModuleYearPlan, "legacy 目标_*.md migration completed: %d records updated", migrated)
	return nil
}

func legacyGoalAccounts() ([]string, error) {
	accounts, err := persistence.ListBlogAccounts()
	if err != nil {
		return nil, fmt.Errorf("list goal migration accounts: %w", err)
	}
	seen := make(map[string]bool, len(accounts))
	for _, account := range accounts {
		seen[account] = true
	}
	root := filepath.Join(config.GetExePath(), "blogs_txt")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() && !seen[entry.Name()] {
			accounts = append(accounts, entry.Name())
			seen[entry.Name()] = true
		}
	}
	return accounts, nil
}

func migrateGoalFilesForAccount(account string) (int, error) {
	root := config.GetBlogsPath(account)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return 0, nil
	}
	migrated := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		title := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		matches := legacyGoalFileTitle.FindStringSubmatch(title)
		if len(matches) != 3 {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		changed, err := importGoalFile(account, matches[1], matches[2], content)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if changed {
			migrated++
		}
		return nil
	})
	return migrated, err
}

func importGoalFile(account, level, period string, content []byte) (bool, error) {
	var source Goal
	if err := json.Unmarshal(content, &source); err != nil {
		return false, fmt.Errorf("parse goal JSON: %w", err)
	}
	if source.Level == "" {
		source.Level = level
	}
	if source.Period == "" {
		source.Period = period
	}
	if source.Level != level || source.Period != period {
		return false, fmt.Errorf("file name and goal content do not match")
	}

	existingBlog := blog.GetBlogWithAccount(account, goalTitle(level, period))
	if existingBlog == nil {
		if source.Tasks == nil {
			source.Tasks = []Task{}
		}
		return true, SaveGoal(account, &source)
	}

	var target Goal
	if err := json.Unmarshal([]byte(existingBlog.Content), &target); err != nil {
		return false, fmt.Errorf("parse existing SQLite goal: %w", err)
	}
	changed := mergeGoalFields(&target, &source)
	if !changed {
		return false, nil
	}
	return true, SaveGoal(account, &target)
}

func mergeGoalFields(target, source *Goal) bool {
	changed := false
	if target.Overview == "" && source.Overview != "" {
		target.Overview = source.Overview
		changed = true
	}
	if target.Judge == "" && source.Judge != "" {
		target.Judge = source.Judge
		changed = true
	}
	if target.ParentID == "" && source.ParentID != "" {
		target.ParentID = source.ParentID
		changed = true
	}
	existingIDs := make(map[string]bool, len(target.Tasks))
	for _, task := range target.Tasks {
		existingIDs[task.ID] = true
	}
	for index, task := range source.Tasks {
		if task.ID == "" {
			task.ID = fmt.Sprintf("legacy-%s-%s-%d", source.Level, source.Period, index)
		}
		if existingIDs[task.ID] {
			continue
		}
		target.Tasks = append(target.Tasks, task)
		existingIDs[task.ID] = true
		changed = true
	}
	return changed
}
