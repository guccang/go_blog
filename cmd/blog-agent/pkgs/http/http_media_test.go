package http

import (
	"archive/zip"
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"persistence"
)

func TestEditorFileType(t *testing.T) {
	var zipContent bytes.Buffer
	zipWriter := zip.NewWriter(&zipContent)
	entry, err := zipWriter.Create("readme.txt")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte("hello")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	tests := []struct {
		name      string
		filename  string
		content   []byte
		wantMIME  string
		wantImage bool
		wantOK    bool
	}{
		{name: "text", filename: "说明.txt", content: []byte("UTF-8 文本"), wantMIME: "text/plain; charset=utf-8", wantOK: true},
		{name: "html", filename: "demo.html", content: []byte("<!doctype html><title>demo</title>"), wantMIME: "text/html; charset=utf-8", wantOK: true},
		{name: "zip", filename: "资料.zip", content: zipContent.Bytes(), wantMIME: "application/zip", wantOK: true},
		{name: "png", filename: "image.png", content: append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 504)...), wantMIME: "image/png", wantImage: true, wantOK: true},
		{name: "extension mismatch", filename: "伪装.txt", content: zipContent.Bytes(), wantOK: false},
		{name: "executable", filename: "tool.exe", content: []byte("MZ executable"), wantOK: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mimeType, _, isImage, ok := editorFileType(test.content, test.filename)
			if ok != test.wantOK || isImage != test.wantImage {
				t.Fatalf("editorFileType() image=%v ok=%v, want image=%v ok=%v", isImage, ok, test.wantImage, test.wantOK)
			}
			if test.wantOK && mimeType != test.wantMIME {
				t.Fatalf("mime type = %q, want %q", mimeType, test.wantMIME)
			}
		})
	}
}

func TestCleanUploadFilename(t *testing.T) {
	if got := cleanUploadFilename("../目录\\报告\n.txt"); got != "报告.txt" {
		t.Fatalf("clean filename = %q", got)
	}
	if got := cleanUploadFilename(strings.Repeat(" ", 3)); got != "" {
		t.Fatalf("blank filename = %q", got)
	}
}

func TestMediaPreviewModes(t *testing.T) {
	tests := []struct {
		name, mode, label string
	}{
		{"README.md", "markdown", "MARKDOWN"},
		{"demo.html", "html", "HTML"},
		{"data.json", "json", "JSON"},
		{"notes.txt", "text", "TXT"},
	}
	for _, test := range tests {
		if got := mediaPreviewMode(test.name); got != test.mode {
			t.Fatalf("mediaPreviewMode(%q) = %q, want %q", test.name, got, test.mode)
		}
		if got := mediaPreviewTypeLabel(test.name); got != test.label {
			t.Fatalf("mediaPreviewTypeLabel(%q) = %q, want %q", test.name, got, test.label)
		}
	}
	if !isEditorTextAsset(persistence.MediaAsset{OriginalName: "说明.md", MIMEType: "text/plain; charset=utf-8"}) {
		t.Fatal("Markdown text asset should support preview")
	}
	if isEditorTextAsset(persistence.MediaAsset{OriginalName: "资料.zip", MIMEType: "application/zip"}) {
		t.Fatal("ZIP asset should not use text preview")
	}
}

func TestMediaViewerTemplateEscapesSourceAndUsesSandbox(t *testing.T) {
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "media_viewer.template"))
	if err != nil {
		t.Fatalf("parse media viewer template: %v", err)
	}
	var output bytes.Buffer
	data := MediaViewerData{
		Name: "demo.html", Mode: "html", TypeLabel: "HTML", MIMEType: "text/html",
		SizeLabel: "1.0 KB", Content: `</textarea><script>alert("x")</script>`,
		DownloadURL: "/media/id?download=1", RenderURL: "/media/render/id",
	}
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatalf("render media viewer template: %v", err)
	}
	page := output.String()
	for _, expected := range []string{`sandbox="allow-scripts allow-popups"`, `data-render-url="/media/render/id"`, `/js/media_viewer.js`, `/js/marked/marked.min.js`, `下载原文件`} {
		if !strings.Contains(page, expected) {
			t.Fatalf("media viewer missing %q", expected)
		}
	}
	if strings.Contains(page, `</textarea><script>alert`) {
		t.Fatal("media source escaped its textarea")
	}
}

func TestHTMLPreviewUsesIsolatedRenderEndpoint(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "media_viewer.js"))
	if err != nil {
		t.Fatalf("read media viewer script: %v", err)
	}
	script := string(content)
	for _, expected := range []string{
		`if (mode === 'html')`,
		`const renderURL = frame.dataset.renderUrl`,
		`frame.src = renderURL`,
		`documentNode.querySelectorAll('script, form, object, embed, base')`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("media viewer script missing %q", expected)
		}
	}
}
