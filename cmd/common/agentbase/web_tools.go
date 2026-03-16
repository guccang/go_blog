package agentbase

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// ============================================================================
// 网页访问工具 - 让 Agent 能够获取互联网数据
// ============================================================================

const (
	defaultMaxContentLength = 5000            // 默认最大返回内容长度（字符）
	defaultSearchCount      = 5               // 默认搜索结果数量
	fetchTimeout            = 15              // HTTP 请求超时（秒）
	maxSearchPerMinute      = 5               // 每分钟最大搜索次数
	maxFetchPerMinute       = 10              // 每分钟最大抓取次数
	searchCacheTTL          = 5 * time.Minute // 搜索缓存有效期
	defaultUserAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// 速率限制器
var (
	searchRateMu    sync.Mutex
	searchCallTimes []time.Time
	fetchRateMu     sync.Mutex
	fetchCallTimes  []time.Time

	// 搜索结果缓存
	searchCacheMu sync.RWMutex
	searchCache   = make(map[string]*searchCacheEntry)
)

type searchCacheEntry struct {
	results []SearchResult
	expiry  time.Time
}

// SearchResult 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// ============================================================================
// WebFetchWebPage - 抓取网页内容
// ============================================================================

// WebFetchWebPage 抓取指定 URL 网页内容，返回纯文本
func WebFetchWebPage(arguments map[string]interface{}) string {
	targetURL, err := GetStringParam(arguments, "url")
	if err != nil {
		return ErrorJSON(err.Error())
	}
	maxLen := GetOptionalIntParam(arguments, "maxLength", defaultMaxContentLength)

	// URL 安全验证
	if !isValidURL(targetURL) {
		return ErrorJSON("无效的 URL，仅支持 http:// 和 https:// 协议")
	}

	log.Printf("[WebFetch] 开始抓取: %s", targetURL)

	// 频率限制
	if !checkRateLimit(&fetchRateMu, &fetchCallTimes, maxFetchPerMinute) {
		log.Printf("[WebFetch] 频率限制: 每分钟最多 %d 次", maxFetchPerMinute)
		return ErrorJSON(fmt.Sprintf("抓取频率过高，每分钟最多 %d 次，请稍后再试", maxFetchPerMinute))
	}

	// HTTP GET
	content, err := fetchURL(targetURL)
	if err != nil {
		log.Printf("[WebFetch] 抓取失败: %s, 错误: %v", targetURL, err)
		return ErrorJSON(fmt.Sprintf("抓取失败: %v", err))
	}

	// HTML 转纯文本
	text := htmlToText(content)

	// 截断内容
	runes := []rune(text)
	truncated := false
	if len(runes) > maxLen {
		text = string(runes[:maxLen])
		truncated = true
	}

	log.Printf("[WebFetch] 抓取成功: %s, 内容长度: %d 字符, 截断: %v", targetURL, len(runes), truncated)

	// 构建结果
	result := map[string]interface{}{
		"url":       targetURL,
		"content":   text,
		"length":    len(runes),
		"truncated": truncated,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ============================================================================
// WebSearch - 搜索互联网
// ============================================================================

// WebSearch 搜索互联网，返回搜索结果列表
func WebSearch(arguments map[string]interface{}) string {
	query, err := GetStringParam(arguments, "query")
	if err != nil {
		return ErrorJSON(err.Error())
	}
	count := GetOptionalIntParam(arguments, "count", defaultSearchCount)
	if count > 10 {
		count = 10
	}

	log.Printf("[WebSearch] 搜索: %s, 数量: %d", query, count)

	// 检查缓存
	searchCacheMu.RLock()
	if entry, ok := searchCache[query]; ok && time.Now().Before(entry.expiry) {
		searchCacheMu.RUnlock()
		log.Printf("[WebSearch] 命中缓存: %s, 结果数: %d", query, len(entry.results))
		cached := entry.results
		if len(cached) > count {
			cached = cached[:count]
		}
		result := map[string]interface{}{"query": query, "results": cached, "count": len(cached), "cached": true}
		data, _ := json.Marshal(result)
		return string(data)
	}
	searchCacheMu.RUnlock()

	// 频率限制
	if !checkRateLimit(&searchRateMu, &searchCallTimes, maxSearchPerMinute) {
		log.Printf("[WebSearch] 频率限制: 每分钟最多 %d 次", maxSearchPerMinute)
		return ErrorJSON(fmt.Sprintf("搜索频率过高，每分钟最多 %d 次，请稍后再试", maxSearchPerMinute))
	}

	// 使用 Bing 搜索
	results, err := searchBing(query, count)
	if err != nil {
		log.Printf("[WebSearch] 搜索失败: %v", err)
		return ErrorJSON(fmt.Sprintf("搜索失败: %v", err))
	}

	log.Printf("[WebSearch] 搜索成功: %s, 结果数: %d", query, len(results))

	// 写入缓存
	searchCacheMu.Lock()
	searchCache[query] = &searchCacheEntry{results: results, expiry: time.Now().Add(searchCacheTTL)}
	searchCacheMu.Unlock()

	// 返回 JSON
	result := map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ============================================================================
// 内部实现函数
// ============================================================================

// isValidURL 验证 URL 安全性
func isValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// checkRateLimit 检查速率限制，返回 true 表示允许调用
func checkRateLimit(mu *sync.Mutex, callTimes *[]time.Time, maxPerMinute int) bool {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// 清除超过1分钟的记录
	valid := make([]time.Time, 0, len(*callTimes))
	for _, t := range *callTimes {
		if t.After(oneMinuteAgo) {
			valid = append(valid, t)
		}
	}
	*callTimes = valid

	// 检查是否超过限制
	if len(*callTimes) >= maxPerMinute {
		return false
	}

	// 记录本次调用
	*callTimes = append(*callTimes, now)
	return true
}

// fetchURL 发送 HTTP GET 请求获取网页内容
func fetchURL(targetURL string) (string, error) {
	client := &http.Client{
		Timeout: time.Duration(fetchTimeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("过多重定向")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 限制读取大小（最大 2MB）
	limitedBody := http.MaxBytesReader(nil, resp.Body, 2*1024*1024)
	body, err := ioutil.ReadAll(limitedBody)
	if err != nil && len(body) == 0 {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 编码检测和转换（处理 GBK/GB2312 等中文编码）
	contentType := resp.Header.Get("Content-Type")
	utf8Body := convertToUTF8(body, contentType)

	return utf8Body, nil
}

// convertToUTF8 检测编码并转换为 UTF-8
func convertToUTF8(body []byte, contentType string) string {
	encoding, name, _ := charset.DetermineEncoding(body, contentType)
	if name == "utf-8" || name == "" {
		return string(body)
	}

	log.Printf("[WebFetch] 检测到编码: %s, 转换为 UTF-8", name)

	reader := transform.NewReader(bytes.NewReader(body), encoding.NewDecoder())
	decoded, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Printf("[WebFetch] 编码转换失败: %v, 使用原始内容", err)
		return string(body)
	}

	return string(decoded)
}

// searchBing 使用 Bing 搜索引擎（使用中国版）
func searchBing(query string, count int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://cn.bing.com/search?q=%s&count=%d&ensearch=0",
		url.QueryEscape(query), count)

	content, err := fetchURL(searchURL)
	if err != nil {
		return nil, fmt.Errorf("搜索请求失败: %w", err)
	}

	return parseBingResults(content, count), nil
}

// parseBingResults 解析 Bing 搜索结果页面
func parseBingResults(htmlContent string, maxCount int) []SearchResult {
	results := []SearchResult{}

	parts := strings.Split(htmlContent, `<li class="b_algo"`)
	if len(parts) <= 1 {
		parts = strings.Split(htmlContent, `<li class=b_algo`)
	}

	for i, block := range parts {
		if i == 0 {
			continue
		}
		if len(results) >= maxCount {
			break
		}

		result := SearchResult{}

		linkRe := regexp.MustCompile(`(?s)<a\s[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
		linkMatch := linkRe.FindStringSubmatch(block)
		if linkMatch != nil && len(linkMatch) >= 3 {
			rawURL := html.UnescapeString(linkMatch[1])
			result.URL = decodeBingRedirectURL(rawURL)
			titleHTML := linkMatch[2]
			citeRe := regexp.MustCompile(`(?s)<cite[^>]*>.*?</cite>`)
			titleHTML = citeRe.ReplaceAllString(titleHTML, "")
			result.Title = strings.TrimSpace(html.UnescapeString(stripHTMLTags(titleHTML)))
		}

		snippetRe := regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
		snippetMatch := snippetRe.FindStringSubmatch(block)
		if snippetMatch != nil && len(snippetMatch) >= 2 {
			result.Snippet = html.UnescapeString(stripHTMLTags(snippetMatch[1]))
		}

		if result.Snippet == "" {
			captionRe := regexp.MustCompile(`(?s)class="b_caption"[^>]*>(.*?)</div>`)
			captionMatch := captionRe.FindStringSubmatch(block)
			if captionMatch != nil && len(captionMatch) >= 2 {
				result.Snippet = html.UnescapeString(stripHTMLTags(captionMatch[1]))
			}
		}

		if result.Snippet == "" {
			spanRe := regexp.MustCompile(`(?s)<span[^>]*>(.*?)</span>`)
			spanMatches := spanRe.FindAllStringSubmatch(block, -1)
			for _, m := range spanMatches {
				text := strings.TrimSpace(stripHTMLTags(m[1]))
				if len([]rune(text)) > len([]rune(result.Snippet)) && len([]rune(text)) > 20 {
					result.Snippet = html.UnescapeString(text)
				}
			}
		}

		if result.URL != "" && result.Title != "" {
			runes := []rune(result.Snippet)
			if len(runes) > 200 {
				result.Snippet = string(runes[:200]) + "..."
			}
			results = append(results, result)
		}
	}

	return results
}

// decodeBingRedirectURL 解码 Bing 跳转 URL，提取实际目标 URL
func decodeBingRedirectURL(rawURL string) string {
	if !strings.Contains(rawURL, "bing.com/ck/a") {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	uParam := u.Query().Get("u")
	if uParam == "" {
		return rawURL
	}

	encoded := uParam
	if strings.HasPrefix(encoded, "a1") {
		encoded = encoded[2:]
	}

	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return rawURL
		}
	}

	decodedURL := string(decoded)
	if strings.HasPrefix(decodedURL, "http") {
		return decodedURL
	}
	return rawURL
}

// ============================================================================
// HTML 处理工具函数
// ============================================================================

// htmlToText 将 HTML 转换为纯文本
func htmlToText(htmlContent string) string {
	scriptRe := regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	htmlContent = scriptRe.ReplaceAllString(htmlContent, "")

	styleRe := regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	htmlContent = styleRe.ReplaceAllString(htmlContent, "")

	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlContent = commentRe.ReplaceAllString(htmlContent, "")

	blockTags := []string{"div", "p", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6",
		"tr", "td", "th", "blockquote", "pre", "article", "section", "header", "footer"}
	for _, tag := range blockTags {
		openRe := regexp.MustCompile(fmt.Sprintf(`(?i)</?%s[^>]*>`, tag))
		htmlContent = openRe.ReplaceAllString(htmlContent, "\n")
	}

	htmlContent = stripHTMLTags(htmlContent)
	htmlContent = html.UnescapeString(htmlContent)

	spaceRe := regexp.MustCompile(`[^\S\n]+`)
	htmlContent = spaceRe.ReplaceAllString(htmlContent, " ")

	newlineRe := regexp.MustCompile(`\n\s*\n+`)
	htmlContent = newlineRe.ReplaceAllString(htmlContent, "\n\n")

	lines := strings.Split(htmlContent, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	htmlContent = strings.Join(lines, "\n")

	return strings.TrimSpace(htmlContent)
}

// stripHTMLTags 移除所有 HTML 标签
func stripHTMLTags(s string) string {
	tagRe := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(tagRe.ReplaceAllString(s, ""))
}
