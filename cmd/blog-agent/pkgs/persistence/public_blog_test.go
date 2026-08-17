package persistence

import (
	"database/sql"
	"module"
	"reflect"
	"testing"
)

func TestFindPublicBlogAccountsByTitleOnlyReturnsSafePublicBlogs(t *testing.T) {
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
	if _, err := testDB.Exec(`CREATE TABLE blogs (
		account TEXT NOT NULL,title TEXT NOT NULL,auth_type INTEGER NOT NULL,encrypt INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create blogs: %v", err)
	}
	insert := func(account, title string, authType, encrypt int) {
		t.Helper()
		if _, err := testDB.Exec("INSERT INTO blogs VALUES(?,?,?,?)", account, title, authType, encrypt); err != nil {
			t.Fatalf("insert blog: %v", err)
		}
	}
	insert("alice", "GenerateGame", module.EAuthType_public, 0)
	insert("bob", "GenerateGame", module.EAuthType_private, 0)
	insert("carol", "GenerateGame", module.EAuthType_public|module.EAuthType_diary, 0)
	insert("dave", "GenerateGame", module.EAuthType_public, 1)
	insert("eve", "Other", module.EAuthType_public, 0)

	got, err := FindPublicBlogAccountsByTitle("GenerateGame")
	if err != nil {
		t.Fatalf("find public accounts: %v", err)
	}
	if want := []string{"alice"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("accounts = %#v, want %#v", got, want)
	}
}
