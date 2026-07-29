package persistence

import (
	"database/sql"
	"module"
	"strings"
	"testing"
)

func TestListRecentBlogSummariesReturnsFormalPrefixOnly(t *testing.T) {
	previousDB := sqliteDB
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqliteDB = testDB
	t.Cleanup(func() {
		testDB.Close()
		sqliteDB = previousDB
	})

	_, err = testDB.Exec(`CREATE TABLE blogs (
		account TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL,
		create_time TEXT NOT NULL, modify_time TEXT NOT NULL, access_time TEXT NOT NULL,
		modify_num INTEGER NOT NULL, access_num INTEGER NOT NULL,
		auth_type INTEGER NOT NULL, tags TEXT NOT NULL, encrypt INTEGER NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create blogs: %v", err)
	}

	insert := func(account, title, content, modifyTime, accessTime string, authType, encrypt int) {
		t.Helper()
		_, insertErr := testDB.Exec(`INSERT INTO blogs VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			account, title, content, modifyTime, modifyTime, accessTime, 0, 0, authType, "", encrypt)
		if insertErr != nil {
			t.Fatalf("insert %s: %v", title, insertErr)
		}
	}
	longContent := strings.Repeat("文", 5000)
	insert("ztt", "最近读过", longContent, "2026-07-20 10:00:00", "2026-07-29 09:00:00", module.EAuthType_private, 0)
	insert("ztt", "尚未读过", "普通正文", "2026-07-29 10:00:00", "", module.EAuthType_private, 0)
	insert("ztt", "sys_conf", "系统配置", "2026-07-29 11:00:00", "2026-07-29 11:00:00", module.EAuthType_private, 0)
	insert("ztt", "目标_daily_2026-07-29", "目标内容", "2026-07-29 11:00:00", "2026-07-29 11:00:00", module.EAuthType_private, 0)
	insert("ztt", "私密日记", "日记内容", "2026-07-29 11:00:00", "2026-07-29 11:00:00", module.EAuthType_private|module.EAuthType_diary, 0)
	insert("ztt", "加密文章", "密文", "2026-07-29 11:00:00", "2026-07-29 11:00:00", module.EAuthType_private, 1)
	insert("other", "其他账户文章", "不可见", "2026-07-29 12:00:00", "2026-07-29 12:00:00", module.EAuthType_private, 0)

	items, err := ListRecentBlogSummaries("ztt", 4, module.EAuthType_all)
	if err != nil {
		t.Fatalf("list recent summaries: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(items), items)
	}
	if items[0].Title != "最近读过" || items[1].Title != "尚未读过" {
		t.Fatalf("unexpected ordering: %q, %q", items[0].Title, items[1].Title)
	}
	if got := len([]rune(items[0].Content)); got != 100 {
		t.Fatalf("content prefix length = %d, want 100", got)
	}

	allItems, err := ListBlogSummaries("ztt", 20, 0, module.EAuthType_all)
	if err != nil {
		t.Fatalf("list blog summaries: %v", err)
	}
	contents := make(map[string]string, len(allItems))
	for _, item := range allItems {
		contents[item.Title] = item.Content
	}
	if got := len([]rune(contents["最近读过"])); got != 100 {
		t.Fatalf("library content prefix length = %d, want 100", got)
	}
	if contents["私密日记"] != "" || contents["加密文章"] != "" {
		t.Fatalf("protected content leaked into library summaries")
	}
	if _, exists := contents["其他账户文章"]; exists {
		t.Fatalf("other account content leaked into library summaries")
	}
}
