package http

import (
	"bytes"
	"html/template"
	"net/http/httptest"
	"path/filepath"
	"persistence"
	"piagent"
	"strings"
	"testing"
)

func TestNormalizeProductCard(t *testing.T) {
	card := persistence.ProductCard{
		Name: "  Figma  ", Tags: []string{"协作", "协作", "创作"},
		KeyMechanics: []string{"组件化", "组件化"}, TransferableIdeas: []string{"多人协作", "", "组件化"},
		ResearchSources: []persistence.ProductResearchSource{
			{ID: "S1", URL: "https://figma.com", Kind: "official"},
			{ID: "S2", URL: "javascript:alert(1)", Kind: "forum"},
		},
		Confidence: map[string]string{"core_loop": "HIGH", "bad": "unknown"},
	}
	normalizeProductCard(&card)
	if card.Name != "Figma" || card.ProductType != "未分类" {
		t.Fatalf("unexpected basic fields: %+v", card)
	}
	if len(card.Tags) != 2 || len(card.TransferableIdeas) != 2 || len(card.KeyMechanics) != 1 || len(card.ResearchSources) != 1 || card.Confidence["core_loop"] != "high" {
		t.Fatalf("lists were not normalized: %+v", card)
	}
}

func TestDecodeProductCardRejectsInvalidSourceURL(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/products", strings.NewReader(`{"name":"测试产品","source_url":"javascript:alert(1)"}`))
	recorder := httptest.NewRecorder()
	if _, err := decodeProductCard(recorder, request); err == nil {
		t.Fatal("invalid product URL should be rejected")
	}
}

func TestNormalizeProductScanURL(t *testing.T) {
	got, err := normalizeProductScanURL(" HTTPS://Example.COM/game/#details ")
	if err != nil || got != "https://example.com/game" {
		t.Fatalf("normalize scan URL: got=%q err=%v", got, err)
	}
	if _, err := normalizeProductScanURL("javascript:alert(1)"); err == nil {
		t.Fatal("unsafe scan URL should be rejected")
	}
}

func TestProductDraftToCardPreservesResearchFields(t *testing.T) {
	draft := piagent.ProductDraft{
		Name: "星图游戏", SourceURL: "https://example.com/game", CoreLoop: "探索 → 战斗 → 成长",
		KeyMechanics: []string{"组合构筑"}, ResearchSources: []persistence.ProductResearchSource{{ID: "S1", URL: "https://example.com/game"}},
		Confidence: map[string]string{"core_loop": "high"}, Evidence: map[string][]string{"core_loop": {"S1"}},
	}
	card := productDraftToCard(draft)
	if productScanWorkerCount != 2 || card.Name != draft.Name || len(card.KeyMechanics) != 1 || len(card.ResearchSources) != 1 || card.Confidence["core_loop"] != "high" {
		t.Fatalf("unexpected automatic product card: %+v", card)
	}
}

func TestProductsTemplateHasManualAndAIScanEntrances(t *testing.T) {
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "products.template"))
	if err != nil {
		t.Fatalf("parse products.template: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, ProductsPageData{USER_ACCOUNT: "ztt", USER_AVATAR: "Z"}); err != nil {
		t.Fatalf("execute products.template: %v", err)
	}
	page := output.String()
	for _, expected := range []string{
		`id="scanForm"`, `id="manualAddButton"`, `id="productGrid"`,
		`id="productDialog"`, "可迁移灵感", "缺点与机会", "核心循环", "论坛与用户吐槽", `id="researchSources"`,
		`id="previewModeButton"`, `id="editModeButton"`, `id="productPreview"`, `id="productEditor"`, `id="previewEditButton"`,
		`id="scanJobs"`, `id="scanJobList"`, `id="activeScanCount"`, "完成时自动保存为产品卡",
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("products page missing %q", expected)
		}
	}
}
