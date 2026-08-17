package persistence

import (
	"database/sql"
	"testing"
)

func TestClipboardItemsAreAccountScoped(t *testing.T) {
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
	if _, err := testDB.Exec(`CREATE TABLE clipboard_items (
		id TEXT PRIMARY KEY,account TEXT NOT NULL,text_content TEXT NOT NULL,
		image_ids_json TEXT NOT NULL,created_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, item := range []ClipboardItem{
		{ID: "a1", Account: "alice", Text: "文本", ImageIDs: []string{"img1"}, CreatedAt: "2026-08-05 10:00:00"},
		{ID: "b1", Account: "bob", Text: "secret", CreatedAt: "2026-08-05 11:00:00"},
	} {
		if err := SaveClipboardItem(item); err != nil {
			t.Fatalf("save item: %v", err)
		}
	}

	items, err := ListClipboardItems("alice", 10)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 || items[0].ID != "a1" || len(items[0].ImageIDs) != 1 {
		t.Fatalf("unexpected items: %+v", items)
	}
	deleted, err := DeleteClipboardItem("bob", "a1")
	if err != nil || deleted {
		t.Fatalf("other account deleted item: deleted=%v err=%v", deleted, err)
	}
}
