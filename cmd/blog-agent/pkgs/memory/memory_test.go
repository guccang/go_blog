package memory

import "testing"

func TestValidFile(t *testing.T) {
	valid := []string{"MEMORY", "checkpoint", "goals", "journal_2026-06-11"}
	for _, name := range valid {
		if !ValidFile(name) {
			t.Fatalf("ValidFile(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "memory", "journal_", "journal_2026-13-99", "../etc", "journal_abc"}
	for _, name := range invalid {
		if ValidFile(name) {
			t.Fatalf("ValidFile(%q) = true, want false", name)
		}
	}
}

func TestScoreLine(t *testing.T) {
	terms := tokenize("锻炼 失眠")
	if len(terms) != 2 {
		t.Fatalf("tokenize returned %d terms", len(terms))
	}
	if got := scoreLine("最近失眠，晚上不想锻炼", terms); got != 2 {
		t.Fatalf("scoreLine = %d, want 2", got)
	}
	if got := scoreLine("今天天气不错", terms); got != 0 {
		t.Fatalf("scoreLine = %d, want 0", got)
	}
}

func TestReplaceMarkdownSectionExisting(t *testing.T) {
	doc := "## 喜好\n- 咖啡\n\n## 健康状况\n- 失眠\n\n## 重要的人\n- 妈妈\n"
	out := replaceMarkdownSection(doc, "健康状况", "- 已改善\n- 作息规律")
	if !contains(out, "## 健康状况\n- 已改善\n- 作息规律\n") {
		t.Fatalf("小节未被正确替换:\n%s", out)
	}
	if !contains(out, "## 喜好\n- 咖啡") || !contains(out, "## 重要的人\n- 妈妈") {
		t.Fatalf("其它小节被破坏:\n%s", out)
	}
	if contains(out, "失眠") {
		t.Fatalf("旧内容未被清除:\n%s", out)
	}
}

func TestReplaceMarkdownSectionAppendWhenMissing(t *testing.T) {
	doc := "## 喜好\n- 咖啡\n"
	out := replaceMarkdownSection(doc, "纪念日", "- 6/12 结婚纪念")
	if !contains(out, "## 喜好\n- 咖啡") {
		t.Fatalf("原内容丢失:\n%s", out)
	}
	if !contains(out, "## 纪念日\n- 6/12 结婚纪念\n") {
		t.Fatalf("缺失小节未追加:\n%s", out)
	}
}

func TestReplaceMarkdownSectionEmptyDoc(t *testing.T) {
	out := replaceMarkdownSection("", "喜好", "- 咖啡")
	if out != "## 喜好\n- 咖啡\n" {
		t.Fatalf("空文档新建小节错误: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
