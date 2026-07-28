package persistence

import (
	"strings"
	"testing"
)

func TestSubstringSearchSnippetHighlightsChineseSubstring(t *testing.T) {
	snippet := substringSearchSnippet(
		"Agent 分析养成资源循环机制",
		"这篇文章分析灵妖养成资源与任务系统之间的循环机制。",
		[]string{"灵妖"},
	)
	if !strings.Contains(snippet, "<mark>灵妖</mark>") {
		t.Fatalf("expected highlighted Chinese substring, got %q", snippet)
	}
}

func TestIndexRunesFindsChineseTermInsideLongToken(t *testing.T) {
	source := []rune("分析灵妖养成资源循环")
	target := []rune("灵妖")
	if got := indexRunes(source, target); got != 2 {
		t.Fatalf("indexRunes() = %d, want 2", got)
	}
}

func TestChunkSearchTermsSplitsChineseQuestion(t *testing.T) {
	terms := chunkSearchTerms("灵妖的养成资源循环是什么？")
	joined := strings.Join(terms, ",")
	for _, expected := range []string{"灵妖", "养成资源循环", "养成", "资源", "循环"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("chunkSearchTerms() = %v, missing %q", terms, expected)
		}
	}
}

func TestRankChunkSearchCandidatesLimitsEachBlog(t *testing.T) {
	candidates := []chunkSearchCandidate{
		{item: BlogChunkSearchResult{Title: "长文", ChunkIndex: 0, Content: "灵妖养成"}},
		{item: BlogChunkSearchResult{Title: "长文", ChunkIndex: 1, Content: "灵妖资源"}},
		{item: BlogChunkSearchResult{Title: "长文", ChunkIndex: 2, Content: "灵妖循环"}},
		{item: BlogChunkSearchResult{Title: "另一篇", ChunkIndex: 0, Heading: "灵妖", Content: "阵容分析"}},
	}
	result := rankChunkSearchCandidates("灵妖", []string{"灵妖"}, candidates, 4)
	if len(result) != 3 {
		t.Fatalf("rankChunkSearchCandidates() returned %d items, want 3", len(result))
	}
	longArticleChunks := 0
	for _, item := range result {
		if item.Title == "长文" {
			longArticleChunks++
		}
	}
	if longArticleChunks != 2 {
		t.Fatalf("long article returned %d chunks, want 2", longArticleChunks)
	}
}
