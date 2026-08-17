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
	if _, err = testDB.Exec("CREATE INDEX idx_blogs_account_access_v2 ON blogs(account, access_time DESC, modify_time DESC, title DESC)"); err != nil {
		t.Fatalf("create recent index: %v", err)
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

	planRows, err := testDB.Query(`EXPLAIN QUERY PLAN SELECT account,title,substr(content,1,100),create_time,modify_time,access_time,modify_num,access_num,auth_type,tags,encrypt
		FROM blogs WHERE account=? AND encrypt=0 AND (auth_type & ?)=0
		AND title NOT LIKE 'sys\_%' ESCAPE '\'
		AND title NOT LIKE 'todolist-%' AND title NOT LIKE 'exercise-%'
		AND title NOT LIKE 'reading\_book\_%' ESCAPE '\'
		AND title NOT LIKE '目标\_%' ESCAPE '\' AND title NOT LIKE '月度目标\_%' ESCAPE '\'
		AND title NOT LIKE '年计划\_%' ESCAPE '\' AND (auth_type & ?) != 0
		ORDER BY access_time DESC,modify_time DESC,title DESC LIMIT ?`, "ztt", module.EAuthType_diary|module.EAuthType_encrypt, module.EAuthType_all, 4)
	if err != nil {
		t.Fatalf("explain recent query: %v", err)
	}
	plan := ""
	for planRows.Next() {
		var id, parent, unused int
		var detail string
		if err := planRows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan recent query plan: %v", err)
		}
		plan += detail + "\n"
	}
	if err := planRows.Err(); err != nil {
		t.Fatalf("iterate recent query plan: %v", err)
	}
	if err := planRows.Close(); err != nil {
		t.Fatalf("close recent query plan: %v", err)
	}
	if !strings.Contains(plan, "idx_blogs_account_access_v2") || strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("recent query does not use access index: %s", plan)
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

func TestListBlogsByTitlePrefixFiltersBeforeLoadingContent(t *testing.T) {
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

	insert := func(account, title, content string) {
		t.Helper()
		_, insertErr := testDB.Exec(`INSERT INTO blogs VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			account, title, content, "2026-07-30 10:00:00", "2026-07-30 10:00:00",
			"", 0, 0, module.EAuthType_private, "", 0)
		if insertErr != nil {
			t.Fatalf("insert %s: %v", title, insertErr)
		}
	}
	insert("ztt", "目标_monthly_2026-07", "本账户目标")
	insert("ztt", "普通博客", strings.Repeat("大", 10000))
	insert("other", "目标_monthly_2026-07", "其他账户目标")

	items, err := ListBlogsByTitlePrefix("ztt", "目标_monthly_")
	if err != nil {
		t.Fatalf("list blogs by title prefix: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1: %+v", len(items), items)
	}
	if items[0].Account != "ztt" || items[0].Title != "目标_monthly_2026-07" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}
