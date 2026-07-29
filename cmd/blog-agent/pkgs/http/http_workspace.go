package http

import (
	"blog"
	t "html/template"
	"module"
	h "net/http"
	"net/url"
	"regexp"
	control "service"
	"strings"
	"time"
)

var (
	localPreviewImagePattern  = regexp.MustCompile(`!\[[^\]\r\n]*\]\(\s*(/media/[a-zA-Z0-9_-]{8,128})(?:\s+["'][^"'\r\n]*["'])?\s*\)`)
	markdownImagePattern      = regexp.MustCompile(`!\[[^\]\r\n]*\]\([^)]+\)`)
	markdownLinkPattern       = regexp.MustCompile(`\[([^\]\r\n]+)\]\([^)]+\)`)
	htmlTagPattern            = regexp.MustCompile(`<[^>]+>`)
	markdownLinePrefixPattern = regexp.MustCompile(`^\s*(?:#{1,6}\s+|>\s*|[-+*]\s+|\d+[.)]\s+)`)
	previewWhitespacePattern  = regexp.MustCompile(`\s+`)
)

type MainPageData struct {
	RECENT_LINKS []LinkData
	USER_ACCOUNT string
	USER_AVATAR  string
}

type QueryPageData struct {
	QUERY        string
	USER_ACCOUNT string
	USER_AVATAR  string
}

func HandleMain(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMain", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	account := getAccountFromRequest(r)
	emitUsageHook(r, account, blog.HookPageOpened, "content_workspace", "page", "main", "main", "", nil, map[string]any{"status": "success"})
	renderMainPage(w, account)
}

func HandleAskPage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAskPage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	account := getAccountFromRequest(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	emitUsageHook(r, account, blog.HookPageOpened, "pi_agent", "page", "ask", "问我的博客", query, nil, map[string]any{"status": "success"})
	renderQueryPage(w, "ask.template", QueryPageData{
		QUERY: query, USER_ACCOUNT: account, USER_AVATAR: generateUserAvatar(account),
	})
}

func PageSearchResults(w h.ResponseWriter, account, query string) {
	renderQueryPage(w, "search_results.template", QueryPageData{
		QUERY: strings.TrimSpace(query), USER_ACCOUNT: account, USER_AVATAR: generateUserAvatar(account),
	})
}

func renderMainPage(w h.ResponseWriter, account string) {
	blogs := control.ListRecentBlogSummaries(account, 4, module.EAuthType_all)
	data := MainPageData{
		RECENT_LINKS: make([]LinkData, 0, len(blogs)),
		USER_ACCOUNT: account,
		USER_AVATAR:  generateUserAvatar(account),
	}
	for _, item := range blogs {
		preview, imageURL := buildMainBlogPreview(item.Title, item.Content)
		data.RECENT_LINKS = append(data.RECENT_LINKS, LinkData{
			URL:         "/get?blogname=" + url.QueryEscape(item.Title),
			DESC:        item.Title,
			ACCESS_TIME: recentTimeLabel(item.AccessTime, item.ModifyTime, time.Now()),
			PREVIEW:     preview,
			IMAGE_URL:   imageURL,
		})
	}
	tmpl, err := t.ParseFiles(GetTemplatePath("main.template"))
	if err != nil {
		h.Error(w, "Failed to parse main.template", h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		h.Error(w, "Failed to render main.template", h.StatusInternalServerError)
	}
}

func buildMainBlogPreview(title, content string) (string, string) {
	imageURL := ""
	if match := localPreviewImagePattern.FindStringSubmatch(content); len(match) > 1 {
		imageURL = match[1]
	}

	text := localPreviewImagePattern.ReplaceAllString(content, "")
	text = markdownImagePattern.ReplaceAllString(text, "")
	text = markdownLinkPattern.ReplaceAllString(text, "$1")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	parts := make([]string, 0, 3)
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		trimmed = markdownLinePrefixPattern.ReplaceAllString(trimmed, "")
		trimmed = strings.Trim(strings.TrimSpace(trimmed), "`*_~")
		trimmed = previewWhitespacePattern.ReplaceAllString(trimmed, " ")
		if trimmed == "" || trimmed == title {
			continue
		}
		parts = append(parts, trimmed)
		if len(parts) == 3 {
			break
		}
	}
	preview := truncatePreview(strings.Join(parts, " "), 140)
	if preview == "" {
		preview = "打开文章继续阅读"
	}
	return preview, imageURL
}

func truncatePreview(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func renderQueryPage(w h.ResponseWriter, name string, data QueryPageData) {
	tmpl, err := t.ParseFiles(GetTemplatePath(name))
	if err != nil {
		h.Error(w, "Failed to parse "+name, h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		h.Error(w, "Failed to render "+name, h.StatusInternalServerError)
	}
}

func recentTimeLabel(accessTime, modifyTime string, now time.Time) string {
	raw := strings.TrimSpace(accessTime)
	if raw == "" {
		raw = strings.TrimSpace(modifyTime)
	}
	value, err := time.ParseInLocation("2006-01-02 15:04:05", raw, now.Location())
	if err != nil {
		return raw
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	itemDay := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case itemDay.Equal(today):
		return value.Format("15:04")
	case itemDay.Equal(today.AddDate(0, 0, -1)):
		return "昨天"
	case value.Year() == now.Year():
		return value.Format("01-02")
	default:
		return value.Format("2006-01-02")
	}
}
