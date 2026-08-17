package persistence

import (
	"database/sql"
	"testing"
	"time"
)

func TestCreateLoginSessionKeepsTwoDevices(t *testing.T) {
	previousDB := sqliteDB
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	testDB.SetMaxOpenConns(1)
	sqliteDB = testDB
	t.Cleanup(func() {
		testDB.Close()
		sqliteDB = previousDB
	})
	if _, err := testDB.Exec(`CREATE TABLE login_sessions (
		session_id TEXT PRIMARY KEY,account TEXT NOT NULL,expires_at TEXT NOT NULL,created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create sessions table: %v", err)
	}

	expiresAt := time.Now().Add(48 * time.Hour)
	for _, sessionID := range []string{"device-1", "device-2"} {
		if err := CreateLoginSession(sessionID, "alice", expiresAt); err != nil {
			t.Fatalf("create %s: %v", sessionID, err)
		}
	}
	if got := GetLoginSessionAccount("device-1"); got != "alice" {
		t.Fatalf("first device account = %q, want alice", got)
	}
	if got := GetLoginSessionAccount("device-2"); got != "alice" {
		t.Fatalf("second device account = %q, want alice", got)
	}

	if err := CreateLoginSession("bob-device", "bob", expiresAt); err != nil {
		t.Fatalf("create other account session: %v", err)
	}
	if err := CreateLoginSession("device-3", "alice", expiresAt); err != nil {
		t.Fatalf("create third device: %v", err)
	}
	if got := GetLoginSessionAccount("device-1"); got != "" {
		t.Fatalf("oldest device should be expired, got account %q", got)
	}
	for _, sessionID := range []string{"device-2", "device-3"} {
		if got := GetLoginSessionAccount(sessionID); got != "alice" {
			t.Fatalf("%s account = %q, want alice", sessionID, got)
		}
	}
	if got := GetLoginSessionAccount("bob-device"); got != "bob" {
		t.Fatalf("other account session was affected: %q", got)
	}
}
