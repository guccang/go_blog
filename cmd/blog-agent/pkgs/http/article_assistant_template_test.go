package http

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
)

func renderArticleTemplateForTest(t *testing.T, enabled bool) string {
	t.Helper()
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "get.template"))
	if err != nil {
		t.Fatalf("parse get.template: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, EditorData{TITLE: "测试文章", PI_ENABLED: enabled}); err != nil {
		t.Fatalf("execute get.template: %v", err)
	}
	return output.String()
}

func TestArticleAssistantRenderedForEligibleArticle(t *testing.T) {
	page := renderArticleTemplateForTest(t, true)
	if !strings.Contains(page, `id="articleAssistantPanel"`) {
		t.Fatalf("eligible article does not render assistant panel")
	}
	if !strings.Contains(page, `data-title="测试文章"`) {
		t.Fatalf("assistant panel does not contain current title")
	}
	scriptIndex := strings.Index(page, `/js/article_assistant.js`)
	bodyEndIndex := strings.Index(page, `</body>`)
	if scriptIndex < 0 || bodyEndIndex < 0 || scriptIndex > bodyEndIndex {
		t.Fatalf("assistant script must be loaded before body closes")
	}
}

func TestArticleAssistantHiddenForProtectedArticle(t *testing.T) {
	page := renderArticleTemplateForTest(t, false)
	if strings.Contains(page, `id="articleAssistantPanel"`) {
		t.Fatalf("protected article renders assistant panel")
	}
}
