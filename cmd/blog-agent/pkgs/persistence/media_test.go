package persistence

import (
	"database/sql"
	"testing"
)

func TestEnsureMediaAssetColumnsMigratesOriginalName(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE media_assets (
		id TEXT PRIMARY KEY, account TEXT NOT NULL, storage_name TEXT NOT NULL,
		mime_type TEXT NOT NULL, size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create old media table: %v", err)
	}
	if err := ensureMediaAssetColumns(db); err != nil {
		t.Fatalf("migrate media table: %v", err)
	}
	if err := ensureMediaAssetColumns(db); err != nil {
		t.Fatalf("repeat media migration: %v", err)
	}
	var originalName string
	if err := db.QueryRow("SELECT original_name FROM media_assets LIMIT 1").Scan(&originalName); err != sql.ErrNoRows {
		t.Fatalf("query original_name error = %v, want %v", err, sql.ErrNoRows)
	}
}
