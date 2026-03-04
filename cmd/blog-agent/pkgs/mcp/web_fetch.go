package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	log "mylog"
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
// 网页访问工具 - 让 LLM 能够获取互联网数据
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
	searchCallTimes []time.Time // 搜索调用时间记录
	fetchRateMu     sync.Mutex
	fetchCallTimes  []time.Time // 抓取调用时间记录

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
// FetchWebPage - 抓取网页内容
// ============================================================================

// Inner_web_FetchWebPage 抓取指定 URL 网页内容，返回纯文本
func Inner_web_FetchWebPage(arguments map[string]interface{}) string {
	targetURL, err := getStringParam(arguments, "url")
	if err != nil {
		return errorJSON(err.Error())
	}
	maxLen := getOptionalIntParam(arguments, "maxLength", defaultMaxContentLength)

	// URL 安全验证
	if !isValidURL(targetURL) {
		return errorJSON("无效的 URL，仅支持 http:// 和 https:// 协议")
	}

	log.MessageF(log.ModuleMCP, "[WebFetch] 开始抓取: %s", targetURL)

	// 频率限制
	if !checkRateLimit(&fetchRateMu, &fetchCallTimes, maxFetchPerMinute) {
		log.WarnF(log.ModuleMCP, "[WebFetch] 频率限制: 每分钟最多 %d 次", maxFetchPerMinute)
		return errorJSON(fmt.Sprintf("抓取频率过高，每分钟最多 %d 次，请稍后再试", maxFetchPerMinute))
	}

	// HTTP GET
	content, err := fetchURL(targetURL)
	if err != nil {
		log.WarnF(log.ModuleMCP, "[WebFetch] 抓取失败: %s, 错误: %v", targetURL, err)
		return errorJSON(fmt.Sprintf("抓取失败: %v", err))
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

	log.MessageF(log.ModuleMCP, "[WebFetch] 抓取成功: %s, 内容长度: %d 字符, 截断: %v", targetURL, len(runes), truncated)

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

// Inner_web_WebSearch 搜索互联网，返回搜索结果列表
func Inner_web_WebSearch(arguments map[string]interface{}) string {
	query, err := getStringParam(arguments, "query")
	if err != nil {
		return errorJSON(err.Error())
	}
	count := getOptionalIntParam(arguments, "count", defaultSearchCount)
	if count > 10 {
		count = 10
	}

	log.MessageF(log.ModuleMCP, "[WebSearch] 搜索: %s, 数量: %d", query, count)

	// 检查缓存
	searchCacheMu.RLock()
	if entry, ok := searchCache[query]; ok && time.Now().Before(entry.expiry) {
		searchCacheMu.RUnlock()
		log.MessageF(log.ModuleMCP, "[WebSearch] 命中缓存: %s, 结果数: %d", query, len(entry.results))
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
		log.WarnF(log.ModuleMCP, "[WebSearch] 频率限制: 每分钟最多 %d 次", maxSearchPerMinute)
		return errorJSON(fmt.Sprintf("搜索频率过高，每分钟最多 %d 次，请稍后再试", maxSearchPerMinute))
	}

	// 使用 Bing 搜索
	results, err := searchBing(query, count)
	if err != nil {
		log.WarnF(log.ModuleMCP, "[WebSearch] 搜索失败: %v", err)
		return errorJSON(fmt.Sprintf("搜索失败: %v", err))
	}

	log.MessageF(log.ModuleMCP, "[WebSearch] 搜索成功: %s, 结果数: %d", query, len(results))

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
		// 不跟随过多重定向
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
	// 尝试从 Content-Type header 和 HTML 内容检测编码
	encoding, name, _ := charset.DetermineEncoding(body, contentType)
	if name == "utf-8" || name == "" {
		return string(body)
	}

	log.DebugF(log.ModuleMCP, "[WebFetch] 检测到编码: %s, 转换为 UTF-8", name)

	// 转码
	reader := transform.NewReader(bytes.NewReader(body), encoding.NewDecoder())
	decoded, err := ioutil.ReadAll(reader)
	if err != nil {
		log.WarnF(log.ModuleMCP, "[WebFetch] 编码转换失败: %v, 使用原始内容", err)
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
// 使用 strings.Split 按 b_algo 分块，避免嵌套标签导致正则失败
func parseBingResults(htmlContent string, maxCount int) []SearchResult {
	results := []SearchResult{}

	// 按 <li class="b_algo" 分割（每个块就是一个搜索结果）
	parts := strings.Split(htmlContent, `<li class="b_algo"`)
	if len(parts) <= 1 {
		// 尝试不带引号的变体
		parts = strings.Split(htmlContent, `<li class=b_algo`)
	}

	for i, block := range parts {
		if i == 0 {
			continue // 第一块是搜索结果之前的 HTML
		}
		if len(results) >= maxCount {
			break
		}

		result := SearchResult{}

		// 提取标题和 URL: 第一个 <a href="xxx"> 就是搜索结果链接
		linkRe := regexp.MustCompile(`(?s)<a\s[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
		linkMatch := linkRe.FindStringSubmatch(block)
		if linkMatch != nil && len(linkMatch) >= 3 {
			rawURL := html.UnescapeString(linkMatch[1])
			// 解码 Bing 跳转 URL
			result.URL = decodeBingRedirectURL(rawURL)
			// 清理标题：移除 <cite> 标签内容（Bing 会把 URL 显示在标题内）
			titleHTML := linkMatch[2]
			citeRe := regexp.MustCompile(`(?s)<cite[^>]*>.*?</cite>`)
			titleHTML = citeRe.ReplaceAllString(titleHTML, "")
			result.Title = strings.TrimSpace(html.UnescapeString(stripHTMLTags(titleHTML)))
		}

		// 提取摘要: 优先从 <p> 标签获取
		snippetRe := regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
		snippetMatch := snippetRe.FindStringSubmatch(block)
		if snippetMatch != nil && len(snippetMatch) >= 2 {
			result.Snippet = html.UnescapeString(stripHTMLTags(snippetMatch[1]))
		}

		// 如果 <p> 没有摘要，从 b_lineclamp 或 b_caption 中的 <span> 获取
		if result.Snippet == "" {
			captionRe := regexp.MustCompile(`(?s)class="b_caption"[^>]*>(.*?)</div>`)
			captionMatch := captionRe.FindStringSubmatch(block)
			if captionMatch != nil && len(captionMatch) >= 2 {
				result.Snippet = html.UnescapeString(stripHTMLTags(captionMatch[1]))
			}
		}

		// 兜底: 找最长的纯文本段作为摘要
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
			// 截断摘要
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
// Bing 搜索结果链接格式: https://www.bing.com/ck/a?...&u=aHR0cHM6Ly8xxx&ntb=1
// 其中 u= 参数是 base64 编码的实际 URL（去掉前缀 "a1"）
func decodeBingRedirectURL(rawURL string) string {
	// 如果不是 Bing 跳转 URL，直接返回
	if !strings.Contains(rawURL, "bing.com/ck/a") {
		return rawURL
	}

	// 解析 URL 参数
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// 提取 u 参数
	uParam := u.Query().Get("u")
	if uParam == "" {
		return rawURL
	}

	// Bing 的 u 参数通常以 "a1" 开头，后面是 base64 编码的 URL
	encoded := uParam
	if strings.HasPrefix(encoded, "a1") {
		encoded = encoded[2:]
	}

	// base64 解码
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		// 尝试标准 base64
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
	// 1. 移除 script 和 style 标签及其内容
	scriptRe := regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	htmlContent = scriptRe.ReplaceAllString(htmlContent, "")

	styleRe := regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	htmlContent = styleRe.ReplaceAllString(htmlContent, "")

	// 移除 HTML 注释
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlContent = commentRe.ReplaceAllString(htmlContent, "")

	// 2. 块级元素转换为换行
	blockTags := []string{"div", "p", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6",
		"tr", "td", "th", "blockquote", "pre", "article", "section", "header", "footer"}
	for _, tag := range blockTags {
		openRe := regexp.MustCompile(fmt.Sprintf(`(?i)</?%s[^>]*>`, tag))
		htmlContent = openRe.ReplaceAllString(htmlContent, "\n")
	}

	// 3. 移除所有剩余的 HTML 标签
	htmlContent = stripHTMLTags(htmlContent)

	// 4. 解码 HTML 实体
	htmlContent = html.UnescapeString(htmlContent)

	// 5. 压缩空白
	// 将多个空格/制表符压缩为一个空格
	spaceRe := regexp.MustCompile(`[^\S\n]+`)
	htmlContent = spaceRe.ReplaceAllString(htmlContent, " ")

	// 将多个连续空行压缩为一个
	newlineRe := regexp.MustCompile(`\n\s*\n+`)
	htmlContent = newlineRe.ReplaceAllString(htmlContent, "\n\n")

	// 去除每行首尾空格
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
