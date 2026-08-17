package persistence

import (
	"database/sql"
	"errors"
	"testing"
)

func TestProductCardsAreAccountScoped(t *testing.T) {
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
	if _, err := testDB.Exec(`CREATE TABLE product_cards (
		id TEXT PRIMARY KEY,account TEXT NOT NULL,name TEXT NOT NULL,is_new INTEGER NOT NULL DEFAULT 0,source_url TEXT NOT NULL,
		cover_url TEXT NOT NULL,product_type TEXT NOT NULL,summary TEXT NOT NULL,positioning TEXT NOT NULL,
		target_users TEXT NOT NULL,problem TEXT NOT NULL,core_loop TEXT NOT NULL,core_mechanism TEXT NOT NULL,
		key_mechanics_json TEXT NOT NULL,feedback_rewards TEXT NOT NULL,social_mechanism TEXT NOT NULL,
		surprise TEXT NOT NULL,retention TEXT NOT NULL,business_model TEXT NOT NULL,strengths_json TEXT NOT NULL,
		user_complaints_json TEXT NOT NULL,competitive_edge TEXT NOT NULL,transferable_ideas_json TEXT NOT NULL,
		opportunities_json TEXT NOT NULL,tags_json TEXT NOT NULL,research_sources_json TEXT NOT NULL,
		confidence_json TEXT NOT NULL,evidence_json TEXT NOT NULL,last_researched_at TEXT NOT NULL,
		created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	for _, card := range []ProductCard{
		{ID: "p1", IsNew: true, Name: "Figma", SourceURL: "https://figma.com", CoreLoop: "进入文件 → 共同编辑 → 实时反馈 → 沉淀组件", KeyMechanics: []string{"多人协作"}, TransferableIdeas: []string{"多人协作"}, Tags: []string{"创作"}, ResearchSources: []ProductResearchSource{{ID: "S1", Title: "官网", URL: "https://figma.com", Kind: "official", Fetched: true}}, Confidence: map[string]string{"core_loop": "high"}, Evidence: map[string][]string{"core_loop": {"S1"}}, LastResearchedAt: "2026-08-12 09:59:00", CreatedAt: "2026-08-12 10:00:00", UpdatedAt: "2026-08-12 10:00:00"},
		{ID: "p2", Name: "Linear", CreatedAt: "2026-08-12 11:00:00", UpdatedAt: "2026-08-12 11:00:00"},
	} {
		account := "alice"
		if card.ID == "p2" {
			account = "bob"
		}
		if err := SaveProductCardWithAccount(account, card); err != nil {
			t.Fatalf("save product: %v", err)
		}
	}

	items, err := ListProductCardsWithAccount("alice")
	if err != nil || len(items) != 1 || items[0].Name != "Figma" || !items[0].IsNew || len(items[0].Tags) != 1 || len(items[0].ResearchSources) != 1 || items[0].Confidence["core_loop"] != "high" {
		t.Fatalf("unexpected alice products: items=%+v err=%v", items, err)
	}
	byURL, err := GetProductCardBySourceURLWithAccount("alice", "https://figma.com")
	if err != nil || byURL.ID != "p1" {
		t.Fatalf("get product by source URL: product=%+v err=%v", byURL, err)
	}
	viewed, err := MarkProductCardViewedWithAccount("alice", "p1")
	if err != nil || !viewed {
		t.Fatalf("mark product viewed: viewed=%v err=%v", viewed, err)
	}
	viewedCard, err := GetProductCardWithAccount("alice", "p1")
	if err != nil || viewedCard.IsNew {
		t.Fatalf("NEW marker was not cleared: product=%+v err=%v", viewedCard, err)
	}
	if _, err := GetProductCardWithAccount("alice", "p2"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("other account product leaked: %v", err)
	}
	deleted, err := DeleteProductCardWithAccount("bob", "p1")
	if err != nil || deleted {
		t.Fatalf("other account deleted product: deleted=%v err=%v", deleted, err)
	}

	items[0].Name = "Figma Design"
	items[0].UpdatedAt = "2026-08-12 12:00:00"
	updated, err := UpdateProductCardWithAccount("alice", items[0])
	if err != nil || !updated {
		t.Fatalf("update product: updated=%v err=%v", updated, err)
	}
	got, err := GetProductCardWithAccount("alice", "p1")
	if err != nil || got.Name != "Figma Design" {
		t.Fatalf("get updated product: product=%+v err=%v", got, err)
	}
}

func TestEnsureProductCardColumnsMigratesLegacyTable(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer testDB.Close()
	if _, err := testDB.Exec(`CREATE TABLE product_cards (id TEXT PRIMARY KEY,name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := ensureProductCardColumns(testDB); err != nil {
		t.Fatalf("migrate product columns: %v", err)
	}
	for _, column := range []string{"core_loop", "key_mechanics_json", "research_sources_json", "confidence_json", "evidence_json", "is_new"} {
		var count int
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('product_cards') WHERE name=?`, column).Scan(&count); err != nil || count != 1 {
			t.Fatalf("column %s not added: count=%d err=%v", column, count, err)
		}
	}
}
