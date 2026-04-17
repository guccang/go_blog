package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	webFetchTimeout  = 15 * time.Second
	webMaxContentLen = 5000
	webDefaultCount  = 5
	webDefaultUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

type webSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

var webSearchTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "WebSearch",
		Description: "搜索互联网并返回候选结果的标题、URL 和摘要。用于发现实时信息、技术资料或候选网页；拿到结果后再按需用 WebFetch 抓正文。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "搜索关键词"},
				"count": {"type": "number", "description": "结果数量，默认 5，最大 10"}
			},
			"required": ["query"]
		}`),
	},
}

var webFetchTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "WebFetch",
		Description: "抓取单个 HTTP/HTTPS URL 的网页正文纯文本，并返回内容、长度和截断标记。适用于已经确认目标页面后的精读，不用于全量遍历搜索结果。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {"type": "string", "description": "要抓取的网页 URL"},
				"maxLength": {"type": "number", "description": "最大返回字符数，默认 5000"}
			},
			"required": ["url"]
		}`),
	},
}

func builtinWebSearch(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	var params struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &ToolCallResult{Result: "参数解析失败: " + err.Error(), AgentID: "builtin"}, nil
	}
	if strings.TrimSpace(params.Query) == "" {
		return &ToolCallResult{Result: "错误: query 参数不能为空", AgentID: "builtin"}, nil
	}
	if params.Count <= 0 {
		params.Count = webDefaultCount
	}
	if params.Count > 10 {
		params.Count = 10
	}

	results, err := webSearchBing(ctx, params.Query, params.Count)
	if err != nil {
		return &ToolCallResult{Result: fmt.Sprintf("搜索失败: %v", err), AgentID: "builtin"}, nil
	}

	payload := map[string]any{
		"query":   params.Query,
		"results": results,
		"count":   len(results),
	}
	data, _ := json.Marshal(payload)
	return &ToolCallResult{Result: string(data), AgentID: "builtin"}, nil
}

func builtinWebFetch(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	var params struct {
		URL       string `json:"url"`
		MaxLength int    `json:"maxLength"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &ToolCallResult{Result: "参数解析失败: " + err.Error(), AgentID: "builtin"}, nil
	}
	if strings.TrimSpace(params.URL) == "" {
		return &ToolCallResult{Result: "错误: url 参数不能为空", AgentID: "builtin"}, nil
	}
	if params.MaxLength <= 0 {
		params.MaxLength = webMaxContentLen
	}

	u, err := url.Parse(params.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return &ToolCallResult{Result: "无效的 URL，仅支持 http:// 和 https:// 协议", AgentID: "builtin"}, nil
	}

	content, err := webFetchURL(ctx, params.URL)
	if err != nil {
		return &ToolCallResult{Result: fmt.Sprintf("抓取失败: %v", err), AgentID: "builtin"}, nil
	}

	text := webHTMLToText(content)
	runes := []rune(text)
	truncated := false
	if len(runes) > params.MaxLength {
		text = string(runes[:params.MaxLength])
		truncated = true
	}

	payload := map[string]any{
		"url":       params.URL,
		"content":   text,
		"length":    len(runes),
		"truncated": truncated,
	}
	data, _ := json.Marshal(payload)
	return &ToolCallResult{Result: string(data), AgentID: "builtin"}, nil
}

func webFetchURL(ctx context.Context, targetURL string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{Timeout: webFetchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", webDefaultUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil && len(body) == 0 {
		return "", err
	}
	return string(body), nil
}

func webSearchBing(ctx context.Context, query string, count int) ([]webSearchResult, error) {
	searchURL := fmt.Sprintf("https://cn.bing.com/search?q=%s&count=%d&ensearch=0", url.QueryEscape(query), count)
	log.Printf("[WebSearch] 搜索: %s, 数量: %d", query, count)
	content, err := webFetchURL(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	results := webParseBingResults(content, count)
	log.Printf("[WebSearch] 搜索成功: %s, 结果数: %d", query, len(results))
	return results, nil
}

func webParseBingResults(htmlContent string, maxCount int) []webSearchResult {
	var results []webSearchResult

	parts := strings.Split(htmlContent, `<li class="b_algo"`)
	if len(parts) <= 1 {
		parts = strings.Split(htmlContent, `<li class=b_algo`)
	}

	linkRe := regexp.MustCompile(`(?s)<a\s[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
	tagRe := regexp.MustCompile(`<[^>]*>`)

	for i, block := range parts {
		if i == 0 || len(results) >= maxCount {
			continue
		}

		r := webSearchResult{}
		if m := linkRe.FindStringSubmatch(block); m != nil {
			r.URL = html.UnescapeString(m[1])
			r.Title = strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(m[2], "")))
		}
		if m := snippetRe.FindStringSubmatch(block); m != nil {
			r.Snippet = strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(m[1], "")))
		}
		if r.URL != "" && r.Title != "" {
			runes := []rune(r.Snippet)
			if len(runes) > 200 {
				r.Snippet = string(runes[:200]) + "..."
			}
			results = append(results, r)
		}
	}

	return results
}

func webHTMLToText(htmlContent string) string {
	scriptRe := regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	htmlContent = scriptRe.ReplaceAllString(htmlContent, "")
	styleRe := regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	htmlContent = styleRe.ReplaceAllString(htmlContent, "")
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlContent = commentRe.ReplaceAllString(htmlContent, "")

	blockRe := regexp.MustCompile(`(?i)</?(?:div|p|br|li|h[1-6]|tr|td|th|blockquote|pre|article|section)[^>]*>`)
	htmlContent = blockRe.ReplaceAllString(htmlContent, "\n")

	tagRe := regexp.MustCompile(`<[^>]*>`)
	htmlContent = tagRe.ReplaceAllString(htmlContent, "")
	htmlContent = html.UnescapeString(htmlContent)

	spaceRe := regexp.MustCompile(`[^\S\n]+`)
	htmlContent = spaceRe.ReplaceAllString(htmlContent, " ")
	newlineRe := regexp.MustCompile(`\n\s*\n+`)
	htmlContent = newlineRe.ReplaceAllString(htmlContent, "\n\n")

	lines := strings.Split(htmlContent, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
