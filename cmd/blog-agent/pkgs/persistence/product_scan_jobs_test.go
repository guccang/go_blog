package persistence

import (
	"database/sql"
	"errors"
	"testing"
)

func TestProductScanJobLifecycleAndAccountScope(t *testing.T) {
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
	if _, err := testDB.Exec(`CREATE TABLE product_scan_jobs (
		id TEXT PRIMARY KEY,account TEXT NOT NULL,source_url TEXT NOT NULL,provider TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,product_id TEXT NOT NULL DEFAULT '',error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,started_at TEXT NOT NULL DEFAULT '',finished_at TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatalf("create product scan jobs: %v", err)
	}
	if _, err := testDB.Exec(`CREATE UNIQUE INDEX idx_product_scan_jobs_active_url
		ON product_scan_jobs(account,source_url) WHERE status IN ('queued','running')`); err != nil {
		t.Fatalf("create active job index: %v", err)
	}

	alice := ProductScanJob{ID: "j1", Account: "alice", SourceURL: "https://example.com/game", Status: ProductScanQueued, CreatedAt: "2026-08-12 10:00:00"}
	bob := ProductScanJob{ID: "j2", Account: "bob", SourceURL: alice.SourceURL, Status: ProductScanQueued, CreatedAt: "2026-08-12 10:01:00"}
	for _, job := range []ProductScanJob{alice, bob} {
		if err := SaveProductScanJob(job); err != nil {
			t.Fatalf("save scan job: %v", err)
		}
	}
	duplicate := alice
	duplicate.ID = "j3"
	if err := SaveProductScanJob(duplicate); err == nil {
		t.Fatal("same account and URL should not create two active jobs")
	}
	active, err := GetActiveProductScanJobWithAccount("alice", alice.SourceURL)
	if err != nil || active.ID != "j1" {
		t.Fatalf("active job leaked across account: job=%+v err=%v", active, err)
	}
	claimed, err := ClaimProductScanJob("j1", "2026-08-12 10:02:00")
	if err != nil || !claimed {
		t.Fatalf("claim scan job: claimed=%v err=%v", claimed, err)
	}
	claimed, err = ClaimProductScanJob("j1", "2026-08-12 10:03:00")
	if err != nil || claimed {
		t.Fatalf("running job should not be claimed twice: claimed=%v err=%v", claimed, err)
	}
	if err := RecoverRunningProductScanJobs(); err != nil {
		t.Fatalf("recover running jobs: %v", err)
	}
	queued, err := ListQueuedProductScanJobs()
	if err != nil || len(queued) != 2 {
		t.Fatalf("recovered queue: jobs=%+v err=%v", queued, err)
	}
	claimed, err = ClaimProductScanJob("j1", "2026-08-12 10:04:00")
	if err != nil || !claimed {
		t.Fatalf("claim recovered job: claimed=%v err=%v", claimed, err)
	}
	if err := CompleteProductScanJob("j1", "p1", "2026-08-12 10:05:00"); err != nil {
		t.Fatalf("complete scan job: %v", err)
	}
	completed, err := GetProductScanJob("j1")
	if err != nil || completed.Status != ProductScanSucceeded || completed.ProductID != "p1" {
		t.Fatalf("completed job: job=%+v err=%v", completed, err)
	}
	if _, err := GetActiveProductScanJobWithAccount("alice", alice.SourceURL); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("completed job should not remain active: %v", err)
	}
	if err := FailProductScanJob("j2", "网络超时", "2026-08-12 10:06:00"); err != nil {
		t.Fatalf("fail scan job: %v", err)
	}
	bobJobs, err := ListProductScanJobsWithAccount("bob", 12)
	if err != nil || len(bobJobs) != 1 || bobJobs[0].Status != ProductScanFailed || bobJobs[0].ErrorMessage != "网络超时" {
		t.Fatalf("failed account jobs: jobs=%+v err=%v", bobJobs, err)
	}
}
