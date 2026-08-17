package piagent

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	stdhttp "net/http"
	"net/url"
	"persistence"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxProductPageBytes      = 1 << 20
	maxProductTextRunes      = 12000
	maxProductPromptRunes    = 36000
	productResearchRequests  = 10
	productPageCacheTTL      = 24 * time.Hour
	productSearchCacheTTL    = 12 * time.Hour
	productRobotsCacheTTL    = 24 * time.Hour
	productDomainMinInterval = 3 * time.Second
	productPageTimeout       = 35 * time.Second
	productAIReserve         = 105 * time.Second
	productResearchWorkers   = 2
)

var (
	productTitlePattern       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	productMetaPattern        = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	productAttrPattern        = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*["']([^"']*)["']`)
	productScriptPattern      = regexp.MustCompile(`(?is)<(?:script|style|noscript|svg)\b[^>]*>.*?</(?:script|style|noscript|svg)>`)
	productJSONLDPattern      = regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
	productMainPattern        = regexp.MustCompile(`(?is)<main\b[^>]*>(.*?)</main>`)
	productArticlePattern     = regexp.MustCompile(`(?is)<article\b[^>]*>(.*?)</article>`)
	productAnchorPattern      = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	productTagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	productSpacePattern       = regexp.MustCompile(`\s+`)
	productInlineSpacePattern = regexp.MustCompile(`[\t\f\v ]+`)
	productSearchResultBlock  = regexp.MustCompile(`(?is)<li\b[^>]*class=["'][^"']*\bb_algo\b[^"']*["'][^>]*>(.*?)</li>`)
	productSearchLinkPattern  = regexp.MustCompile(`(?is)<h2[^>]*>\s*<a\b[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	productSearchTextPattern  = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)

	productResearchSlot = make(chan struct{}, productResearchWorkers)
	productDomainMu     sync.Mutex
	productDomainLast   = make(map[string]time.Time)
	productPageCacheMu  sync.RWMutex
	productPageCache    = make(map[string]productPageCacheEntry)
	productSearchMu     sync.RWMutex
	productSearchCache  = make(map[string]productSearchCacheEntry)
	productRobotsMu     sync.RWMutex
	productRobotsCache  = make(map[string]productRobotsCacheEntry)
)

type productLink struct {
	URL  string
	Text string
}

type productPage struct {
	URL            string
	Title          string
	Description    string
	SiteName       string
	ImageURL       string
	StructuredData string
	Text           string
	Links          []productLink
}

type productResearchDocument struct {
	SourceID string `json:"source_id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Content  string `json:"content"`
}

type productResearch struct {
	Primary   productPage
	Documents []productResearchDocument
	Sources   []persistence.ProductResearchSource
}

type productPromptPayload struct {
	ProductName string                              `json:"product_name"`
	PrimaryURL  string                              `json:"primary_url"`
	Documents   []productResearchDocument           `json:"documents"`
	Sources     []persistence.ProductResearchSource `json:"sources"`
}

type productWebResult struct {
	Title   string
	URL     string
	Snippet string
	Kind    string
}

type productRequestBudget struct {
	remaining int
}

type productHTTPResponse struct {
	StatusCode int
	Header     stdhttp.Header
	FinalURL   string
	Data       []byte
}

type productPageCacheEntry struct {
	Page   productPage
	Expiry time.Time
}

type productSearchCacheEntry struct {
	Results []productWebResult
	Expiry  time.Time
}

type productRobotRule struct {
	Path  string
	Allow bool
}

type productRobotsCacheEntry struct {
	Rules  []productRobotRule
	Expiry time.Time
}

func acquireProductResearchSlot(ctx context.Context) error {
	select {
	case productResearchSlot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("等待产品研究任务: %w", ctx.Err())
	}
}

func releaseProductResearchSlot() {
	<-productResearchSlot
}

func (research productResearch) promptPayload() productPromptPayload {
	documents := make([]productResearchDocument, 0, len(research.Documents))
	remaining := maxProductPromptRunes
	for _, document := range research.Documents {
		if remaining <= 0 {
			break
		}
		content := []rune(document.Content)
		if len(content) > remaining {
			content = content[:remaining]
		}
		document.Content = string(content)
		documents = append(documents, document)
		remaining -= len(content)
	}
	return productPromptPayload{
		ProductName: research.Primary.Title, PrimaryURL: research.Primary.URL,
		Documents: documents, Sources: research.Sources,
	}
}

func researchProduct(ctx context.Context, rawURL string) (productResearch, error) {
	budget := &productRequestBudget{remaining: productResearchRequests}
	validatedURL, err := validateProductURL(ctx, rawURL)
	if err != nil {
		return productResearch{}, err
	}
	primary, primaryErr := fetchProductPage(ctx, validatedURL.String(), budget, true)
	if primaryErr != nil {
		primary = productPage{URL: validatedURL.String(), Title: productNameFromURL(validatedURL)}
	}
	research := productResearch{Primary: primary}
	if primaryErr == nil {
		research.addPage(primary, "official", "官网入口")
	} else {
		research.addUnavailablePrimary(primary, primaryErr)
	}

	if primaryErr == nil {
		for _, link := range selectOfficialProductLinks(primary, 2) {
			if !productResearchCanContinue(ctx) {
				break
			}
			page, fetchErr := fetchProductPage(ctx, link.URL, budget, true)
			if fetchErr != nil {
				continue
			}
			research.addPage(page, "official", link.Text)
		}
	}

	searchResults := make([]productWebResult, 0, 10)
	for _, query := range productResearchQueries(primary.Title) {
		if !productResearchCanContinue(ctx) {
			break
		}
		results, searchErr := searchProductWeb(ctx, query, budget)
		if searchErr != nil {
			continue
		}
		searchResults = append(searchResults, results...)
	}
	searchResults = deduplicateProductResults(searchResults, primary.URL)
	for _, result := range searchResults {
		research.addSearchResult(result)
	}

	for _, candidate := range selectExternalProductPages(searchResults, 2) {
		if !productResearchCanContinue(ctx) {
			break
		}
		page, fetchErr := fetchProductPage(ctx, candidate.URL, budget, true)
		if fetchErr != nil {
			continue
		}
		research.markSourceFetched(candidate.URL)
		research.addDocumentForExistingSource(page, candidate.Kind, candidate.URL)
	}
	if ctx.Err() != nil {
		return productResearch{}, errors.New("产品研究总时限已用尽，未能进入 AI 分析阶段")
	}
	return research, nil
}

func productResearchCanContinue(ctx context.Context) bool {
	return ctx.Err() == nil && productTimeRemaining(ctx) > productAIReserve
}

func productNameFromURL(parsed *url.URL) string {
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	labels := strings.Split(host, ".")
	name := ""
	if len(labels) > 0 {
		name = labels[0]
	}
	for _, generic := range []string{"store", "play", "apps", "app", "steam", "steampowered"} {
		if name == generic && len(labels) > 1 {
			name = labels[1]
			break
		}
	}
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	if name == "" {
		return host
	}
	return name
}

func (research *productResearch) addPage(page productPage, kind, fallbackTitle string) {
	title := page.Title
	if title == "" {
		title = fallbackTitle
	}
	sourceID := fmt.Sprintf("S%d", len(research.Sources)+1)
	research.Sources = append(research.Sources, persistence.ProductResearchSource{
		ID: sourceID, Title: limitProductText(title, 180), URL: page.URL, Kind: kind,
		Snippet: limitProductText(page.Description, 360), Fetched: true,
	})
	content := page.Text
	if page.StructuredData != "" {
		content = "结构化资料：" + page.StructuredData + "\n页面正文：" + content
	}
	research.Documents = append(research.Documents, productResearchDocument{
		SourceID: sourceID, Kind: kind, Title: title, URL: page.URL, Content: content,
	})
}

func (research *productResearch) addUnavailablePrimary(page productPage, fetchErr error) {
	research.Sources = append(research.Sources, persistence.ProductResearchSource{
		ID: "S1", Title: limitProductText(page.Title+" 官网", 180), URL: page.URL, Kind: "official",
		Snippet: "官网未成功读取：" + productFetchErrorLabel(fetchErr), Fetched: false,
	})
}

func productFetchErrorLabel(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return "读取超时，已降级使用搜索和外部资料"
	}
	message := limitProductText(err.Error(), 180)
	if message == "" {
		return "读取失败，已降级使用搜索和外部资料"
	}
	return message
}

func (research *productResearch) addSearchResult(result productWebResult) {
	for _, source := range research.Sources {
		if sameProductURL(source.URL, result.URL) {
			return
		}
	}
	research.Sources = append(research.Sources, persistence.ProductResearchSource{
		ID: fmt.Sprintf("S%d", len(research.Sources)+1), Title: limitProductText(result.Title, 180),
		URL: result.URL, Kind: result.Kind, Snippet: limitProductText(result.Snippet, 420), Fetched: false,
	})
}

func (research *productResearch) markSourceFetched(rawURL string) {
	for index := range research.Sources {
		if sameProductURL(research.Sources[index].URL, rawURL) {
			research.Sources[index].Fetched = true
			return
		}
	}
}

func (research *productResearch) addDocumentForExistingSource(page productPage, kind, requestedURL string) {
	for _, source := range research.Sources {
		if sameProductURL(source.URL, requestedURL) {
			research.Documents = append(research.Documents, productResearchDocument{
				SourceID: source.ID, Kind: kind, Title: source.Title, URL: page.URL, Content: page.Text,
			})
			return
		}
	}
}

func productResearchQueries(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	quoted := `"` + name + `"`
	return []string{
		quoted + " review features gameplay pricing 评测 核心玩法",
		quoted + " user feedback forum reddit steam taptap 用户评价 吐槽",
	}
}

func selectOfficialProductLinks(page productPage, limit int) []productLink {
	base, err := url.Parse(page.URL)
	if err != nil {
		return nil
	}
	type scoredLink struct {
		productLink
		score int
	}
	candidates := make([]scoredLink, 0)
	seen := map[string]struct{}{}
	for _, link := range page.Links {
		parsed, err := url.Parse(link.URL)
		if err != nil || !strings.EqualFold(parsed.Hostname(), base.Hostname()) || sameProductURL(link.URL, page.URL) {
			continue
		}
		lower := strings.ToLower(parsed.Path + " " + link.Text)
		if strings.Contains(lower, "login") || strings.Contains(lower, "sign up") || strings.Contains(lower, "privacy") || strings.Contains(lower, "terms") {
			continue
		}
		score := 0
		for _, keyword := range []string{"features", "feature", "gameplay", "how it works", "product", "pricing", "about", "玩法", "功能", "价格", "关于"} {
			if strings.Contains(lower, keyword) {
				score += 2
			}
		}
		if score == 0 {
			continue
		}
		normalized := normalizeProductURL(link.URL)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, scoredLink{productLink: link, score: score})
	}
	for left := 0; left < len(candidates); left++ {
		for right := left + 1; right < len(candidates); right++ {
			if candidates[right].score > candidates[left].score {
				candidates[left], candidates[right] = candidates[right], candidates[left]
			}
		}
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]productLink, len(candidates))
	for index := range candidates {
		result[index] = candidates[index].productLink
	}
	return result
}

func selectExternalProductPages(results []productWebResult, limit int) []productWebResult {
	selected := make([]productWebResult, 0, limit)
	for _, wantedKind := range []string{"forum", "review"} {
		for _, result := range results {
			if result.Kind == wantedKind {
				selected = append(selected, result)
				break
			}
		}
	}
	for _, result := range results {
		if len(selected) >= limit {
			break
		}
		alreadySelected := false
		for _, item := range selected {
			alreadySelected = alreadySelected || sameProductURL(item.URL, result.URL)
		}
		if !alreadySelected {
			selected = append(selected, result)
		}
	}
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func deduplicateProductResults(results []productWebResult, primaryURL string) []productWebResult {
	primary, _ := url.Parse(primaryURL)
	seen := map[string]struct{}{}
	result := make([]productWebResult, 0, len(results))
	for _, item := range results {
		parsed, err := url.Parse(item.URL)
		if err != nil || parsed.Hostname() == "" || strings.EqualFold(parsed.Hostname(), primary.Hostname()) {
			continue
		}
		key := normalizeProductURL(item.URL)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		item.Kind = classifyProductResult(item)
		result = append(result, item)
		if len(result) == 8 {
			break
		}
	}
	return result
}

func classifyProductResult(result productWebResult) string {
	parsed, _ := url.Parse(result.URL)
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range []string{"reddit.com", "steamcommunity.com", "taptap.cn", "nga.cn", "v2ex.com", "zhihu.com", "news.ycombinator.com", "producthunt.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return "forum"
		}
	}
	text := strings.ToLower(result.Title + " " + result.Snippet)
	for _, keyword := range []string{"review", "评测", "测评", "体验", "评价"} {
		if strings.Contains(text, keyword) {
			return "review"
		}
	}
	return "search"
}

func fetchProductPage(ctx context.Context, rawURL string, budget *productRequestBudget, checkRobots bool) (productPage, error) {
	parsed, err := validateProductURL(ctx, rawURL)
	if err != nil {
		return productPage{}, err
	}
	cacheKey := normalizeProductURL(parsed.String())
	productPageCacheMu.RLock()
	cached, exists := productPageCache[cacheKey]
	productPageCacheMu.RUnlock()
	if exists && time.Now().Before(cached.Expiry) {
		return cached.Page, nil
	}
	if checkRobots {
		allowed, robotsErr := productRobotsAllowed(ctx, parsed, budget)
		if robotsErr != nil {
			return productPage{}, robotsErr
		}
		if !allowed {
			return productPage{}, errors.New("目标网站的 robots.txt 不允许抓取该页面")
		}
	}
	response, err := requestProductURL(ctx, parsed, budget, "text/html,application/xhtml+xml", maxProductPageBytes)
	if err != nil {
		return productPage{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return productPage{}, fmt.Errorf("产品页面返回 HTTP %d", response.StatusCode)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return productPage{}, errors.New("URL 不是 HTML 页面")
	}
	page := parseProductPage(response.FinalURL, string(response.Data))
	productPageCacheMu.Lock()
	productPageCache[cacheKey] = productPageCacheEntry{Page: page, Expiry: time.Now().Add(productPageCacheTTL)}
	productPageCacheMu.Unlock()
	return page, nil
}

func searchProductWeb(ctx context.Context, query string, budget *productRequestBudget) ([]productWebResult, error) {
	query = strings.TrimSpace(query)
	productSearchMu.RLock()
	cached, exists := productSearchCache[query]
	productSearchMu.RUnlock()
	if exists && time.Now().Before(cached.Expiry) {
		return append([]productWebResult(nil), cached.Results...), nil
	}
	searchURL, _ := url.Parse("https://www.bing.com/search?q=" + url.QueryEscape(query) + "&count=6")
	response, err := requestProductURL(ctx, searchURL, budget, "text/html,application/xhtml+xml", maxProductPageBytes)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Web 搜索返回 HTTP %d", response.StatusCode)
	}
	results := parseProductSearchResults(string(response.Data), 6)
	productSearchMu.Lock()
	productSearchCache[query] = productSearchCacheEntry{Results: results, Expiry: time.Now().Add(productSearchCacheTTL)}
	productSearchMu.Unlock()
	return results, nil
}

func parseProductSearchResults(document string, limit int) []productWebResult {
	blocks := productSearchResultBlock.FindAllStringSubmatch(document, -1)
	results := make([]productWebResult, 0, limit)
	for _, block := range blocks {
		if len(block) < 2 {
			continue
		}
		link := productSearchLinkPattern.FindStringSubmatch(block[1])
		if len(link) < 3 {
			continue
		}
		rawURL := decodeProductSearchURL(html.UnescapeString(link[1]))
		parsed, err := url.Parse(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			continue
		}
		title := html.UnescapeString(productTagPattern.ReplaceAllString(link[2], " "))
		snippet := ""
		if match := productSearchTextPattern.FindStringSubmatch(block[1]); len(match) > 1 {
			snippet = html.UnescapeString(productTagPattern.ReplaceAllString(match[1], " "))
		}
		results = append(results, productWebResult{
			Title: limitProductText(title, 180), URL: rawURL, Snippet: limitProductText(snippet, 420),
		})
		if len(results) == limit {
			break
		}
	}
	return results
}

func decodeProductSearchURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || !strings.Contains(strings.ToLower(parsed.Hostname()), "bing.com") || !strings.Contains(parsed.Path, "/ck/a") {
		return rawURL
	}
	encoded := parsed.Query().Get("u")
	if strings.HasPrefix(encoded, "a1") {
		encoded = encoded[2:]
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err == nil && strings.HasPrefix(string(decoded), "http") {
		return string(decoded)
	}
	return rawURL
}

func productRobotsAllowed(ctx context.Context, target *url.URL, budget *productRequestBudget) (bool, error) {
	root := target.Scheme + "://" + target.Host
	productRobotsMu.RLock()
	cached, exists := productRobotsCache[root]
	productRobotsMu.RUnlock()
	if !exists || time.Now().After(cached.Expiry) {
		robotsURL, _ := url.Parse(root + "/robots.txt")
		response, err := requestProductURL(ctx, robotsURL, budget, "text/plain,*/*", 256<<10)
		if err != nil || response.StatusCode == stdhttp.StatusNotFound {
			cached = productRobotsCacheEntry{Expiry: time.Now().Add(productRobotsCacheTTL)}
		} else if response.StatusCode >= 200 && response.StatusCode < 300 {
			cached = productRobotsCacheEntry{Rules: parseProductRobots(string(response.Data)), Expiry: time.Now().Add(productRobotsCacheTTL)}
		} else if response.StatusCode == stdhttp.StatusUnauthorized || response.StatusCode == stdhttp.StatusForbidden {
			return false, nil
		} else {
			cached = productRobotsCacheEntry{Expiry: time.Now().Add(productRobotsCacheTTL)}
		}
		productRobotsMu.Lock()
		productRobotsCache[root] = cached
		productRobotsMu.Unlock()
	}
	path := target.EscapedPath()
	if path == "" {
		path = "/"
	}
	bestLength, allowed := -1, true
	for _, rule := range cached.Rules {
		if rule.Path != "" && strings.HasPrefix(path, rule.Path) && len(rule.Path) > bestLength {
			bestLength, allowed = len(rule.Path), rule.Allow
		}
	}
	return allowed, nil
}

func parseProductRobots(document string) []productRobotRule {
	type group struct {
		agents []string
		rules  []productRobotRule
	}
	groups := make([]group, 0)
	current := group{}
	hasRules := false
	flush := func() {
		if len(current.agents) > 0 {
			groups = append(groups, current)
		}
		current, hasRules = group{}, false
	}
	for _, rawLine := range strings.Split(strings.ReplaceAll(document, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
		switch key {
		case "user-agent":
			if hasRules {
				flush()
			}
			current.agents = append(current.agents, strings.ToLower(value))
		case "allow", "disallow":
			if len(current.agents) == 0 || value == "" {
				continue
			}
			hasRules = true
			current.rules = append(current.rules, productRobotRule{Path: value, Allow: key == "allow"})
		}
	}
	flush()
	selected := make([]productRobotRule, 0)
	exactFound := false
	for _, group := range groups {
		for _, agent := range group.agents {
			if strings.Contains("guccang-product-research", agent) && agent != "*" {
				if !exactFound {
					selected, exactFound = nil, true
				}
				selected = append(selected, group.rules...)
			} else if agent == "*" && !exactFound {
				selected = append(selected, group.rules...)
			}
		}
	}
	return selected
}

func requestProductURL(ctx context.Context, parsed *url.URL, budget *productRequestBudget, accept string, maxBytes int64) (productHTTPResponse, error) {
	var lastResponse productHTTPResponse
	for attempt := 0; attempt < 2; attempt++ {
		if err := budget.consume(); err != nil {
			return productHTTPResponse{}, err
		}
		if err := waitForProductDomain(ctx, parsed.Hostname()); err != nil {
			return productHTTPResponse{}, err
		}
		response, err := performProductRequest(ctx, parsed, budget, accept, maxBytes)
		if err != nil {
			return productHTTPResponse{}, err
		}
		lastResponse = response
		if response.StatusCode != stdhttp.StatusTooManyRequests || attempt == 1 {
			return response, nil
		}
		delay := parseProductRetryAfter(response.Header.Get("Retry-After"))
		if delay <= 0 || delay > 15*time.Second {
			return response, nil
		}
		if err := waitProductContext(ctx, delay); err != nil {
			return productHTTPResponse{}, err
		}
	}
	return lastResponse, nil
}

func performProductRequest(ctx context.Context, parsed *url.URL, budget *productRequestBudget, accept string, maxBytes int64) (productHTTPResponse, error) {
	transport := &stdhttp.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := lookupPublicProductIPs(ctx, host)
			if err != nil {
				return nil, err
			}
			dialer := net.Dialer{Timeout: 8 * time.Second}
			var lastErr error
			for _, ip := range ips {
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout: 8 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &stdhttp.Client{
		Transport: transport,
		Timeout:   productPageTimeout,
		CheckRedirect: func(req *stdhttp.Request, via []*stdhttp.Request) error {
			if len(via) >= 3 {
				return errors.New("重定向次数过多")
			}
			if err := budget.consume(); err != nil {
				return err
			}
			if _, err := validateProductURL(req.Context(), req.URL.String()); err != nil {
				return err
			}
			return waitForProductDomain(req.Context(), req.URL.Hostname())
		},
	}
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, parsed.String(), nil)
	if err != nil {
		return productHTTPResponse{}, err
	}
	req.Header.Set("User-Agent", "GUCCANG-Product-Research/1.0")
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.7")
	response, err := client.Do(req)
	if err != nil {
		return productHTTPResponse{}, fmt.Errorf("抓取页面: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return productHTTPResponse{}, err
	}
	if int64(len(data)) > maxBytes {
		return productHTTPResponse{}, errors.New("页面内容超过研究上限")
	}
	return productHTTPResponse{
		StatusCode: response.StatusCode, Header: response.Header.Clone(),
		FinalURL: response.Request.URL.String(), Data: data,
	}, nil
}

func (budget *productRequestBudget) consume() error {
	if budget.remaining <= 0 {
		return errors.New("本次研究已达到最多 10 个网络请求")
	}
	budget.remaining--
	return nil
}

func waitForProductDomain(ctx context.Context, host string) error {
	host = strings.ToLower(host)
	wait := reserveProductDomainDelay(host, time.Now(), productDomainMinInterval+productRequestJitter())
	if wait > 0 {
		return waitProductContext(ctx, wait)
	}
	return nil
}

func reserveProductDomainDelay(host string, now time.Time, gap time.Duration) time.Duration {
	productDomainMu.Lock()
	defer productDomainMu.Unlock()
	scheduled := productDomainLast[host].Add(gap)
	if scheduled.Before(now) {
		scheduled = now
	}
	productDomainLast[host] = scheduled
	return scheduled.Sub(now)
}

func productRequestJitter() time.Duration {
	buffer := []byte{0}
	if _, err := rand.Read(buffer); err != nil {
		return 500 * time.Millisecond
	}
	return time.Duration(buffer[0]%21) * 100 * time.Millisecond
}

func waitProductContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseProductRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := stdhttp.ParseTime(value); err == nil {
		return time.Until(when)
	}
	return 0
}

func validateProductURL(ctx context.Context, rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("请输入完整的产品网址")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("产品网址仅支持 HTTP 或 HTTPS")
	}
	if parsed.User != nil {
		return nil, errors.New("产品网址不能包含登录凭据")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, errors.New("不能扫描本机或内网网址")
	}
	if _, err := lookupPublicProductIPs(ctx, host); err != nil {
		return nil, err
	}
	parsed.Fragment = ""
	return parsed, nil
}

func lookupPublicProductIPs(ctx context.Context, host string) ([]net.IP, error) {
	if parsed := net.ParseIP(host); parsed != nil {
		if isBlockedProductIP(parsed) {
			return nil, errors.New("不能扫描本机或内网网址")
		}
		return []net.IP{parsed}, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("解析产品网址失败: %w", err)
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		if isBlockedProductIP(address.IP) {
			return nil, errors.New("不能扫描解析到内网的地址")
		}
		ips = append(ips, address.IP)
	}
	if len(ips) == 0 {
		return nil, errors.New("产品网址没有可用地址")
	}
	return ips, nil
}

func isBlockedProductIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4[0] == 100 && ipv4[1] >= 64 && ipv4[1] <= 127
	}
	return false
}

func parseProductPage(pageURL, document string) productPage {
	meta := make(map[string]string)
	for _, tag := range productMetaPattern.FindAllString(document, -1) {
		attrs := make(map[string]string)
		for _, match := range productAttrPattern.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(match[1])] = html.UnescapeString(strings.TrimSpace(match[2]))
		}
		key := strings.ToLower(attrs["property"])
		if key == "" {
			key = strings.ToLower(attrs["name"])
		}
		if key != "" && attrs["content"] != "" {
			meta[key] = attrs["content"]
		}
	}
	title := meta["og:title"]
	if title == "" {
		if match := productTitlePattern.FindStringSubmatch(document); len(match) > 1 {
			title = html.UnescapeString(productTagPattern.ReplaceAllString(match[1], " "))
		}
	}
	description := meta["og:description"]
	if description == "" {
		description = meta["description"]
	}
	structuredParts := make([]string, 0, 3)
	for _, match := range productJSONLDPattern.FindAllStringSubmatch(document, 3) {
		if len(match) > 1 {
			structuredParts = append(structuredParts, limitProductText(html.UnescapeString(match[1]), 2400))
		}
	}
	mainParts := productMainPattern.FindAllStringSubmatch(document, -1)
	if len(mainParts) == 0 {
		mainParts = productArticlePattern.FindAllStringSubmatch(document, -1)
	}
	content := document
	if len(mainParts) > 0 {
		var builder strings.Builder
		for _, part := range mainParts {
			if len(part) > 1 {
				builder.WriteString(part[1])
				builder.WriteString("\n")
			}
		}
		if len([]rune(htmlToProductText(builder.String()))) >= 400 {
			content = builder.String()
		}
	}
	text := htmlToProductText(content)
	textRunes := []rune(text)
	if len(textRunes) > maxProductTextRunes {
		textRunes = textRunes[:maxProductTextRunes]
	}
	return productPage{
		URL: pageURL, Title: limitProductText(title, 240), Description: limitProductText(description, 1000),
		SiteName: limitProductText(meta["og:site_name"], 160), ImageURL: meta["og:image"],
		StructuredData: strings.Join(structuredParts, "\n"), Text: string(textRunes), Links: parseProductLinks(pageURL, document),
	}
}

func htmlToProductText(document string) string {
	document = productScriptPattern.ReplaceAllString(document, " ")
	document = productTagPattern.ReplaceAllString(document, " ")
	document = html.UnescapeString(document)
	return strings.TrimSpace(productSpacePattern.ReplaceAllString(document, " "))
}

func parseProductLinks(pageURL, document string) []productLink {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	links := make([]productLink, 0)
	seen := map[string]struct{}{}
	for _, match := range productAnchorPattern.FindAllStringSubmatch(document, -1) {
		if len(match) < 3 {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range productAttrPattern.FindAllStringSubmatch(match[1], -1) {
			attrs[strings.ToLower(attr[1])] = html.UnescapeString(strings.TrimSpace(attr[2]))
		}
		target, err := url.Parse(attrs["href"])
		if err != nil || attrs["href"] == "" {
			continue
		}
		resolved := base.ResolveReference(target)
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}
		resolved.Fragment = ""
		key := normalizeProductURL(resolved.String())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		links = append(links, productLink{URL: resolved.String(), Text: limitProductText(htmlToProductText(match[2]), 120)})
		if len(links) == 120 {
			break
		}
	}
	return links
}

func resolveProductAssetURL(pageURL, assetURL string) string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	asset, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(asset)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func normalizeProductURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	parsed.Fragment = ""
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path != "/" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	return parsed.String()
}

func sameProductURL(left, right string) bool {
	return normalizeProductURL(left) == normalizeProductURL(right)
}
