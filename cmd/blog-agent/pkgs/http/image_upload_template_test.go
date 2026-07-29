package http

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
)

func renderEditorTemplateForTest(t *testing.T, name string, data EditorData) string {
	t.Helper()
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return output.String()
}

func TestCreateEditorOffersLocalImageUpload(t *testing.T) {
	page := renderEditorTemplateForTest(t, "markdown_editor.template", EditorData{})
	for _, expected := range []string{
		`id="btn-upload-image"`,
		`id="image-file-input"`,
		`accept="image/*"`,
		`multiple`,
		`/js/image_upload.js`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("create editor missing %q", expected)
		}
	}
}

func TestEditableBlogOffersLocalImageUpload(t *testing.T) {
	page := renderEditorTemplateForTest(t, "get.template", EditorData{TITLE: "测试"})
	if !strings.Contains(page, `id="btn-upload-image"`) || !strings.Contains(page, `id="image-file-input"`) {
		t.Fatalf("editable blog does not offer local image upload")
	}
}

func TestLargeBlogDoesNotOfferImageUpload(t *testing.T) {
	page := renderEditorTemplateForTest(t, "get.template", EditorData{TITLE: "大文档", IS_LARGE: true})
	if strings.Contains(page, `id="btn-upload-image"`) || strings.Contains(page, `id="image-file-input"`) {
		t.Fatalf("large read-only blog exposes image upload")
	}
}
