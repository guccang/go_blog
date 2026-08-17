package piagent

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseProductDraftNormalizesLists(t *testing.T) {
	content := "```json\n" + `{"name":" Figma ","product_type":"软件","core_loop":"1. 触发：打开文件\n2. 操作：共同编辑","key_mechanics":["组件化","组件化"],"transferable_ideas":["多人协作","多人协作","组件化"],"tags":["创作","协作"],"confidence":{"core_loop":"HIGH","bad":"unknown"},"evidence":{"core_loop":["S1","S1"]}}` + "\n```"
	draft, err := parseProductDraft(content)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if draft.Name != "Figma" || len(draft.TransferableIdeas) != 2 || len(draft.Tags) != 2 || len(draft.KeyMechanics) != 1 || draft.Confidence["core_loop"] != "high" || len(draft.Evidence["core_loop"]) != 1 {
		t.Fatalf("unexpected draft: %+v", draft)
	}
}

func TestParseProductDraftAcceptsStringArrayDrift(t *testing.T) {
	content := `{
		"name":"测试产品",
		"target_users":["独立开发者","小型产品团队"],
		"feedback_rewards":["操作后立即显示结果","完成阶段目标后解锁能力"],
		"key_mechanics":"资源组合\n风险抉择，阶段成长",
		"strengths":"反馈清晰、上手直接",
		"evidence":{"target_users":"S1","feedback_rewards":["S1","S2"]}
	}`
	draft, err := parseProductDraft(content)
	if err != nil {
		t.Fatalf("parse draft with shifted field types: %v", err)
	}
	if draft.TargetUsers != "独立开发者 小型产品团队" {
		t.Fatalf("target users were not merged: %q", draft.TargetUsers)
	}
	if draft.FeedbackRewards != "操作后立即显示结果 完成阶段目标后解锁能力" {
		t.Fatalf("feedback rewards were not merged: %q", draft.FeedbackRewards)
	}
	if len(draft.KeyMechanics) != 3 || len(draft.Strengths) != 2 {
		t.Fatalf("string lists were not split: mechanics=%v strengths=%v", draft.KeyMechanics, draft.Strengths)
	}
	if len(draft.Evidence["target_users"]) != 1 || draft.Evidence["target_users"][0] != "S1" {
		t.Fatalf("single evidence source was not normalized: %v", draft.Evidence)
	}
}

func TestParseProductDraftRejectsObjectForTextField(t *testing.T) {
	_, err := parseProductDraft(`{"name":"测试产品","feedback_rewards":{"instant":"声音反馈"}}`)
	if err == nil || !strings.Contains(err.Error(), "feedback_rewards") {
		t.Fatalf("invalid text field should return a focused error: %v", err)
	}
}

func TestParseProductPageReadsMetadataAndText(t *testing.T) {
	document := `<html><head><title>普通标题</title><meta content="星图产品" property="og:title"><meta name="description" content="产品描述"><meta property="og:image" content="/cover.png"><script type="application/ld+json">{"name":"星图产品"}</script></head><body><script>ignore()</script><main>核心机制是多人协作。<a href="/features">功能</a></main></body></html>`
	page := parseProductPage("https://example.com/product", document)
	if page.Title != "星图产品" || page.Description != "产品描述" || page.ImageURL != "/cover.png" {
		t.Fatalf("unexpected metadata: %+v", page)
	}
	if !strings.Contains(page.Text, "核心机制是多人协作") || strings.Contains(page.Text, "ignore()") {
		t.Fatalf("unexpected text: %q", page.Text)
	}
	if !strings.Contains(page.StructuredData, "星图产品") || len(page.Links) != 1 || page.Links[0].URL != "https://example.com/features" {
		t.Fatalf("structured data or links missing: %+v", page)
	}
	if got := resolveProductAssetURL(page.URL, page.ImageURL); got != "https://example.com/cover.png" {
		t.Fatalf("resolved image URL = %q", got)
	}
}

func TestProductDraftNeedsCompletion(t *testing.T) {
	complete := ProductDraft{
		TargetUsers: "独立设计师与产品团队", Problem: "多人协作时交付割裂",
		CoreLoop: strings.Repeat("核心循环", 25), CoreMechanism: strings.Repeat("机制解释", 35),
	}
	if productDraftNeedsCompletion(complete) {
		t.Fatal("complete draft should not trigger a second pass")
	}
	complete.CoreMechanism = "一句话"
	if !productDraftNeedsCompletion(complete) {
		t.Fatal("short mechanism should trigger a second pass")
	}
}

func TestEnsureProductDraftCompletenessExplainsMissingEvidence(t *testing.T) {
	draft := ProductDraft{}
	ensureProductDraftCompleteness(&draft)
	if draft.TargetUsers == "" || draft.Problem == "" || draft.CoreLoop == "" || draft.CoreMechanism == "" {
		t.Fatalf("missing fields should be explained: %+v", draft)
	}
	if draft.Confidence["target_users"] != "low" || draft.Confidence["core_mechanism"] != "low" {
		t.Fatalf("fallback confidence should be low: %+v", draft.Confidence)
	}
}

func TestParseProductRobotsUsesSpecificAgent(t *testing.T) {
	rules := parseProductRobots("User-agent: *\nDisallow: /\n\nUser-agent: GUCCANG-Product-Research\nAllow: /products\nDisallow: /private")
	if len(rules) != 2 || !rules[0].Allow || rules[1].Allow {
		t.Fatalf("unexpected robot rules: %+v", rules)
	}
}

func TestClassifyProductResult(t *testing.T) {
	if kind := classifyProductResult(productWebResult{URL: "https://www.reddit.com/r/games/x"}); kind != "forum" {
		t.Fatalf("reddit kind = %q", kind)
	}
	if kind := classifyProductResult(productWebResult{URL: "https://example.com/test", Title: "深度评测"}); kind != "review" {
		t.Fatalf("review kind = %q", kind)
	}
}

func TestParseProductSearchResults(t *testing.T) {
	document := `<ol><li class="b_algo"><h2><a href="https://www.reddit.com/r/test/1">玩家讨论</a></h2><div><p>用户反馈摘要</p></div></li></ol>`
	results := parseProductSearchResults(document, 3)
	if len(results) != 1 || results[0].Title != "玩家讨论" || results[0].Snippet != "用户反馈摘要" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestProductURLBlocksLocalAddresses(t *testing.T) {
	for _, rawURL := range []string{"http://127.0.0.1/admin", "http://[::1]/", "ftp://example.com/file", "http://localhost/test"} {
		if _, err := validateProductURL(context.Background(), rawURL); err == nil {
			t.Fatalf("URL should be blocked: %s", rawURL)
		}
	}
	for _, ip := range []string{"10.0.0.1", "172.16.0.1", "192.168.1.1", "100.64.0.1", "169.254.1.1"} {
		if !isBlockedProductIP(net.ParseIP(ip)) {
			t.Fatalf("IP should be blocked: %s", ip)
		}
	}
}

func TestProductResearchReservesTimeForAI(t *testing.T) {
	longContext, cancelLong := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelLong()
	if !productResearchCanContinue(longContext) {
		t.Fatal("research should continue while the AI reserve is available")
	}

	shortContext, cancelShort := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShort()
	if productResearchCanContinue(shortContext) {
		t.Fatal("optional research should stop before consuming the AI reserve")
	}
}

func TestProductDomainReservationsRemainSerialWithConcurrentWorkers(t *testing.T) {
	if capacity := cap(productResearchSlot); capacity != 2 {
		t.Fatalf("product research worker capacity = %d, want 2", capacity)
	}
	host := "serial-test.example"
	now := time.Now()
	if delay := reserveProductDomainDelay(host, now, 3*time.Second); delay != 0 {
		t.Fatalf("first request should run immediately: %v", delay)
	}
	if delay := reserveProductDomainDelay(host, now, 3*time.Second); delay < 3*time.Second {
		t.Fatalf("second same-domain request should be reserved after the first: %v", delay)
	}
}

func TestProductTimeoutErrorsIdentifyStage(t *testing.T) {
	if message := productFetchErrorLabel(context.DeadlineExceeded); !strings.Contains(message, "已降级") {
		t.Fatalf("fetch timeout should describe graceful degradation: %q", message)
	}
	if err := classifyProductAnalysisError(context.DeadlineExceeded); !strings.Contains(err.Error(), "AI 分析超时") {
		t.Fatalf("analysis timeout should identify the AI stage: %v", err)
	}
	if err := classifyProductAnalysisError(errors.New("provider unavailable")); !strings.Contains(err.Error(), "AI 分析失败") {
		t.Fatalf("provider error should identify the AI stage: %v", err)
	}
}

func TestProductNameFromURL(t *testing.T) {
	parsed, err := url.Parse("https://store.steampowered.com/app/123/example")
	if err != nil {
		t.Fatal(err)
	}
	if name := productNameFromURL(parsed); name != "steampowered" {
		t.Fatalf("unexpected fallback product name: %q", name)
	}
}
