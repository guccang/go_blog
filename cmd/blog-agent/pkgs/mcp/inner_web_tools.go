package mcp

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// Web 搜索与网页抓取工具
// ============================================================================

const (
	webFetchTimeout    = 15 * time.Second
	webMaxContentLen   = 5000
	webDefaultCount    = 5
	webDefaultUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Inner_blog_WebSearch 搜索互联网（Bing）
func Inner_blog_WebSearch(arguments map[string]interface{}) string {
	query, err := getStringParam(arguments, "query")
	if err != nil {
		return errorJSON(err.Error())
	}
	count := getOptionalIntParam(arguments, "count", webDefaultCount)
	if count > 10 {
		count = 10
	}

	log.Printf("[WebSearch] 搜索: %s, 数量: %d", query, count)

	results, err := webSearchBing(query, count)
	if err != nil {
		log.Printf("[WebSearch] 搜索失败: %v", err)
		return errorJSON(fmt.Sprintf("搜索失败: %v", err))
	}

	log.Printf("[WebSearch] 搜索成功: %s, 结果数: %d", query, len(results))

	result := map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}
	data, _ := json.Marshal(result)
	return wrapResult(string(data))
}

// Inner_blog_WebFetch 抓取网页内容
func Inner_blog_WebFetch(arguments map[string]interface{}) string {
	targetURL, err := getStringParam(arguments, "url")
	if err != nil {
		return errorJSON(err.Error())
	}
	maxLen := getOptionalIntParam(arguments, "maxLength", webMaxContentLen)

	u, parseErr := url.Parse(targetURL)
	if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errorJSON("无效的 URL，仅支持 http:// 和 https:// 协议")
	}

	log.Printf("[WebFetch] 开始抓取: %s", targetURL)

	content, err := webFetchURL(targetURL)
	if err != nil {
		log.Printf("[WebFetch] 抓取失败: %s, 错误: %v", targetURL, err)
		return errorJSON(fmt.Sprintf("抓取失败: %v", err))
	}

	text := webHTMLToText(content)

	runes := []rune(text)
	truncated := false
	if len(runes) > maxLen {
		text = string(runes[:maxLen])
		truncated = true
	}

	log.Printf("[WebFetch] 抓取成功: %s, 内容长度: %d 字符", targetURL, len(runes))

	result := map[string]interface{}{
		"url":       targetURL,
		"content":   text,
		"length":    len(runes),
		"truncated": truncated,
	}
	data, _ := json.Marshal(result)
	return wrapResult(string(data))
}

// ============================================================================
// 内部实现
// ============================================================================

type webSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func webFetchURL(targetURL string) (string, error) {
	client := &http.Client{Timeout: webFetchTimeout}
	req, err := http.NewRequest("GET", targetURL, nil)
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

	body, err := ioutil.ReadAll(http.MaxBytesReader(nil, resp.Body, 2*1024*1024))
	if err != nil && len(body) == 0 {
		return "", err
	}
	return string(body), nil
}

func webSearchBing(query string, count int) ([]webSearchResult, error) {
	searchURL := fmt.Sprintf("https://cn.bing.com/search?q=%s&count=%d&ensearch=0",
		url.QueryEscape(query), count)

	content, err := webFetchURL(searchURL)
	if err != nil {
		return nil, err
	}
	return webParseBingResults(content, count), nil
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
	// 移除 script/style/comment
	scriptRe := regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	htmlContent = scriptRe.ReplaceAllString(htmlContent, "")
	styleRe := regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	htmlContent = styleRe.ReplaceAllString(htmlContent, "")
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlContent = commentRe.ReplaceAllString(htmlContent, "")

	// 块级标签转换行
	blockRe := regexp.MustCompile(`(?i)</?(?:div|p|br|li|h[1-6]|tr|td|th|blockquote|pre|article|section)[^>]*>`)
	htmlContent = blockRe.ReplaceAllString(htmlContent, "\n")

	// 移除所有标签
	tagRe := regexp.MustCompile(`<[^>]*>`)
	htmlContent = tagRe.ReplaceAllString(htmlContent, "")
	htmlContent = html.UnescapeString(htmlContent)

	// 清理空白
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
