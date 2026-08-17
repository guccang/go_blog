package http

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	h "net/http"
	"net/url"
	"persistence"
	"strings"
	"time"
)

type ProductsPageData struct {
	USER_ACCOUNT string
	USER_AVATAR  string
}

func HandleProducts(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleProducts", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	account := getAccountFromRequest(r)
	PageProducts(w, ProductsPageData{USER_ACCOUNT: account, USER_AVATAR: generateUserAvatar(account)})
}

func HandleProductsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleProductsAPI", r)
	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		cards, err := persistence.ListProductCardsWithAccount(account)
		if err != nil {
			writeProductError(w, "产品库加载失败", h.StatusInternalServerError)
			return
		}
		writeProductJSON(w, h.StatusOK, map[string]any{"success": true, "products": cards})
	case h.MethodPost:
		card, err := decodeProductCard(w, r)
		if err != nil {
			writeProductError(w, err.Error(), h.StatusBadRequest)
			return
		}
		card.ID, err = newProductID()
		if err != nil {
			writeProductError(w, "产品编号生成失败", h.StatusInternalServerError)
			return
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		card.CreatedAt, card.UpdatedAt = now, now
		if err := persistence.SaveProductCardWithAccount(account, card); err != nil {
			writeProductError(w, "产品保存失败", h.StatusInternalServerError)
			return
		}
		writeProductJSON(w, h.StatusCreated, map[string]any{"success": true, "product": card})
	case h.MethodPut:
		card, err := decodeProductCard(w, r)
		if err != nil {
			writeProductError(w, err.Error(), h.StatusBadRequest)
			return
		}
		card.ID = strings.TrimSpace(r.URL.Query().Get("id"))
		if card.ID == "" {
			writeProductError(w, "缺少产品编号", h.StatusBadRequest)
			return
		}
		card.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
		updated, err := persistence.UpdateProductCardWithAccount(account, card)
		if err != nil {
			writeProductError(w, "产品更新失败", h.StatusInternalServerError)
			return
		}
		if !updated {
			writeProductError(w, "产品不存在", h.StatusNotFound)
			return
		}
		saved, err := persistence.GetProductCardWithAccount(account, card.ID)
		if err != nil {
			writeProductError(w, "产品读取失败", h.StatusInternalServerError)
			return
		}
		writeProductJSON(w, h.StatusOK, map[string]any{"success": true, "product": saved})
	case h.MethodPatch:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeProductError(w, "缺少产品编号", h.StatusBadRequest)
			return
		}
		viewed, err := persistence.MarkProductCardViewedWithAccount(account, id)
		if err != nil {
			writeProductError(w, "产品已读状态更新失败", h.StatusInternalServerError)
			return
		}
		if !viewed {
			writeProductError(w, "产品不存在", h.StatusNotFound)
			return
		}
		writeProductJSON(w, h.StatusOK, map[string]any{"success": true})
	case h.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeProductError(w, "缺少产品编号", h.StatusBadRequest)
			return
		}
		deleted, err := persistence.DeleteProductCardWithAccount(account, id)
		if err != nil {
			writeProductError(w, "产品删除失败", h.StatusInternalServerError)
			return
		}
		if !deleted {
			writeProductError(w, "产品不存在", h.StatusNotFound)
			return
		}
		writeProductJSON(w, h.StatusOK, map[string]any{"success": true})
	default:
		writeProductError(w, "不支持的请求方法", h.StatusMethodNotAllowed)
	}
}

func HandleProductScan(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleProductScan", r)
	account := getAccountFromRequest(r)
	if r.Method == h.MethodGet {
		jobs, err := persistence.ListProductScanJobsWithAccount(account, 12)
		if err != nil {
			writeProductError(w, "扫描任务加载失败", h.StatusInternalServerError)
			return
		}
		writeProductJSON(w, h.StatusOK, map[string]any{"success": true, "jobs": jobs})
		return
	}
	if r.Method != h.MethodPost {
		writeProductError(w, "不支持的请求方法", h.StatusMethodNotAllowed)
		return
	}
	var request struct {
		URL      string `json:"url"`
		Provider string `json:"provider"`
	}
	decoder := json.NewDecoder(h.MaxBytesReader(w, r.Body, 32<<10))
	if err := decoder.Decode(&request); err != nil {
		writeProductError(w, "请求内容无效", h.StatusBadRequest)
		return
	}
	normalizedURL, err := normalizeProductScanURL(request.URL)
	if err != nil {
		writeProductError(w, err.Error(), h.StatusBadRequest)
		return
	}
	migratePIProvidersToJSON(account)
	active, err := persistence.GetActiveProductScanJobWithAccount(account, normalizedURL)
	if err == nil {
		writeProductJSON(w, h.StatusAccepted, map[string]any{"success": true, "job": active, "reused": true})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeProductError(w, "扫描任务创建失败", h.StatusInternalServerError)
		return
	}
	jobID, err := newProductID()
	if err != nil {
		writeProductError(w, "扫描任务编号生成失败", h.StatusInternalServerError)
		return
	}
	job := persistence.ProductScanJob{
		ID: jobID, Account: account, SourceURL: normalizedURL, Provider: strings.TrimSpace(request.Provider),
		Status: persistence.ProductScanQueued, CreatedAt: productTimestamp(),
	}
	if err := persistence.SaveProductScanJob(job); err != nil {
		if active, lookupErr := persistence.GetActiveProductScanJobWithAccount(account, normalizedURL); lookupErr == nil {
			writeProductJSON(w, h.StatusAccepted, map[string]any{"success": true, "job": active, "reused": true})
			return
		}
		writeProductError(w, "扫描任务保存失败", h.StatusInternalServerError)
		return
	}
	enqueueProductScanJob(job.ID)
	writeProductJSON(w, h.StatusAccepted, map[string]any{"success": true, "job": job, "reused": false})
}

func normalizeProductScanURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("请输入产品网址")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("产品网址必须是完整的 HTTP 或 HTTPS 地址")
	}
	if parsed.User != nil {
		return "", errors.New("产品网址不能包含登录凭据")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path != "/" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	return parsed.String(), nil
}

func decodeProductCard(w h.ResponseWriter, r *h.Request) (persistence.ProductCard, error) {
	var card persistence.ProductCard
	decoder := json.NewDecoder(h.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&card); err != nil {
		return card, errors.New("产品卡内容无效")
	}
	normalizeProductCard(&card)
	if card.Name == "" {
		return card, errors.New("产品名称不能为空")
	}
	if err := validateOptionalProductURL(card.SourceURL, "来源链接"); err != nil {
		return card, err
	}
	if err := validateOptionalProductURL(card.CoverURL, "封面链接"); err != nil {
		return card, err
	}
	return card, nil
}

func normalizeProductCard(card *persistence.ProductCard) {
	card.Name = limitProductField(card.Name, 120)
	card.SourceURL = limitProductField(card.SourceURL, 1000)
	card.CoverURL = limitProductField(card.CoverURL, 1000)
	card.ProductType = limitProductField(card.ProductType, 40)
	if card.ProductType == "" {
		card.ProductType = "未分类"
	}
	card.Summary = limitProductField(card.Summary, 600)
	card.Positioning = limitProductField(card.Positioning, 800)
	card.TargetUsers = limitProductField(card.TargetUsers, 800)
	card.Problem = limitProductField(card.Problem, 1000)
	card.CoreLoop = limitProductField(card.CoreLoop, 1600)
	card.CoreMechanism = limitProductField(card.CoreMechanism, 2400)
	card.FeedbackRewards = limitProductField(card.FeedbackRewards, 1200)
	card.SocialMechanism = limitProductField(card.SocialMechanism, 1000)
	card.Surprise = limitProductField(card.Surprise, 1000)
	card.Retention = limitProductField(card.Retention, 1200)
	card.BusinessModel = limitProductField(card.BusinessModel, 1000)
	card.CompetitiveEdge = limitProductField(card.CompetitiveEdge, 1000)
	card.KeyMechanics = normalizeProductFields(card.KeyMechanics, 8)
	card.Strengths = normalizeProductFields(card.Strengths, 8)
	card.UserComplaints = normalizeProductFields(card.UserComplaints, 8)
	card.TransferableIdeas = normalizeProductFields(card.TransferableIdeas, 8)
	card.Opportunities = normalizeProductFields(card.Opportunities, 8)
	card.Tags = normalizeProductFields(card.Tags, 12)
	card.ResearchSources = normalizeProductSources(card.ResearchSources)
	card.Confidence = normalizeProductConfidence(card.Confidence)
	card.Evidence = normalizeProductEvidence(card.Evidence)
	card.LastResearchedAt = limitProductField(card.LastResearchedAt, 19)
}

func normalizeProductFields(values []string, max int) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = limitProductField(value, 160)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
		if len(result) == max {
			break
		}
	}
	return result
}

func limitProductField(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

func normalizeProductSources(sources []persistence.ProductResearchSource) []persistence.ProductResearchSource {
	result := make([]persistence.ProductResearchSource, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		source.ID = limitProductField(source.ID, 20)
		source.Title = limitProductField(source.Title, 180)
		source.Kind = limitProductField(source.Kind, 20)
		source.Snippet = limitProductField(source.Snippet, 420)
		if source.ID == "" || validateOptionalProductURL(source.URL, "研究来源") != nil {
			continue
		}
		if _, exists := seen[source.URL]; exists {
			continue
		}
		seen[source.URL] = struct{}{}
		result = append(result, source)
		if len(result) == 16 {
			break
		}
	}
	return result
}

func normalizeProductConfidence(values map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range values {
		key = limitProductField(key, 40)
		value = strings.ToLower(strings.TrimSpace(value))
		if key != "" && (value == "high" || value == "medium" || value == "low") {
			result[key] = value
		}
	}
	return result
}

func normalizeProductEvidence(values map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for key, sourceIDs := range values {
		key = limitProductField(key, 40)
		if key != "" {
			result[key] = normalizeProductFields(sourceIDs, 8)
		}
	}
	return result
}

func validateOptionalProductURL(rawURL, field string) error {
	if rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s必须是完整的 HTTP 或 HTTPS 地址", field)
	}
	return nil
}

func newProductID() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func writeProductError(w h.ResponseWriter, message string, status int) {
	writeProductJSON(w, status, map[string]any{"success": false, "error": message})
}

func writeProductJSON(w h.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
