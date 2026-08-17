package persistence

import (
	"database/sql"
	"module"
	"testing"
)

func TestGetPublicBlogMediaAssetRequiresSafePublicReference(t *testing.T) {
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
	for _, stmt := range []string{
		`CREATE TABLE media_assets (id TEXT PRIMARY KEY,account TEXT NOT NULL,storage_name TEXT NOT NULL,original_name TEXT NOT NULL DEFAULT '',mime_type TEXT NOT NULL,size_bytes INTEGER NOT NULL,created_at TEXT NOT NULL)`,
		`CREATE TABLE blogs (account TEXT NOT NULL,title TEXT NOT NULL,content TEXT NOT NULL,auth_type INTEGER NOT NULL,encrypt INTEGER NOT NULL)`,
	} {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	insertAsset := func(id, account string) {
		t.Helper()
		if _, err := testDB.Exec("INSERT INTO media_assets VALUES(?,?,?,?,?,?,?)", id, account, id+".png", id+".png", "image/png", 10, "2026-08-06 10:00:00"); err != nil {
			t.Fatalf("insert asset: %v", err)
		}
	}
	insertBlog := func(account, title, content string, authType, encrypt int) {
		t.Helper()
		if _, err := testDB.Exec("INSERT INTO blogs VALUES(?,?,?,?,?)", account, title, content, authType, encrypt); err != nil {
			t.Fatalf("insert blog: %v", err)
		}
	}

	insertAsset("public-image", "alice")
	insertAsset("private-image", "alice")
	insertAsset("other-account-image", "bob")
	insertAsset("diary-image", "alice")
	insertBlog("alice", "公开文章", "![图](/media/public-image)", module.EAuthType_public, 0)
	insertBlog("alice", "私有文章", "![图](/media/private-image)", module.EAuthType_private, 0)
	insertBlog("alice", "公开日记", "![图](/media/diary-image)", module.EAuthType_public|module.EAuthType_diary, 0)
	insertBlog("alice", "错误账号引用", "![图](/media/other-account-image)", module.EAuthType_public, 0)

	asset, err := GetPublicBlogMediaAsset("public-image")
	if err != nil || asset.Account != "alice" {
		t.Fatalf("public image unavailable: asset=%+v err=%v", asset, err)
	}
	for _, id := range []string{"private-image", "diary-image", "other-account-image"} {
		if asset, err := GetPublicBlogMediaAsset(id); err == nil || asset != nil {
			t.Fatalf("unsafe image %s became public: asset=%+v err=%v", id, asset, err)
		}
	}
}
