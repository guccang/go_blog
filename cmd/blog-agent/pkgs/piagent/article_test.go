package piagent

import (
	"persistence"
	"strings"
	"testing"
)

func TestSelectArticleContextSamplesLargeArticle(t *testing.T) {
	content := strings.Repeat("开", 4000) + strings.Repeat("中", 4000) + strings.Repeat("末", 4000)
	context := selectArticleContext(content)
	if !strings.Contains(context, "[文章中段节选]") || !strings.Contains(context, "[文章末段节选]") {
		t.Fatalf("large article context is missing sample markers")
	}
	if len([]rune(context)) > maxArticleContextRunes+40 {
		t.Fatalf("sampled context is too large: %d runes", len([]rune(context)))
	}
	if !strings.Contains(context, "开开") || !strings.Contains(context, "中中") || !strings.Contains(context, "末末") {
		t.Fatalf("sampled context does not cover the beginning, middle and end")
	}
}

func TestBuildRelatedArticleContextExcludesCurrentAndDeduplicates(t *testing.T) {
	results := []persistence.BlogChunkSearchResult{
		{Title: "当前文章", Content: "不应出现"},
		{Title: "关联文章", Heading: "章节一", Content: "第一段"},
		{Title: "关联文章", Heading: "章节二", Content: "重复来源"},
		{Title: "另一篇", Content: "补充内容"},
	}
	context, sources := buildRelatedArticleContext("当前文章", results)
	if strings.Contains(context, "不应出现") {
		t.Fatalf("current article was included in related context")
	}
	if len(sources) != 2 || sources[0] != "关联文章" || sources[1] != "另一篇" {
		t.Fatalf("unexpected sources: %v", sources)
	}
	if strings.Count(context, "## 关联文章") != 1 {
		t.Fatalf("duplicate related article was included")
	}
}
