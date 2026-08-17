package http

import (
	"encoding/json"
	log "mylog"
	h "net/http"
	"persistence"
	"strings"
	"time"
)

const (
	maxClipboardTextRunes = 200000
	maxClipboardImages    = 8
)

type clipboardItemResponse struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Images    []string `json:"images"`
	CreatedAt string   `json:"created_at"`
}

func HandleClipboard(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleClipboard", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	PageClipboard(w)
}

func HandleClipboardAPI(w h.ResponseWriter, r *h.Request) {
	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		items, err := persistence.ListClipboardItems(account, 50)
		if err != nil {
			writeClipboardError(w, h.StatusInternalServerError, "读取贴板失败")
			return
		}
		responses := make([]clipboardItemResponse, 0, len(items))
		for _, item := range items {
			responses = append(responses, clipboardResponse(item))
		}
		writeClipboardJSON(w, h.StatusOK, map[string]any{"success": true, "data": responses})
	case h.MethodPost:
		var request struct {
			Text     string   `json:"text"`
			ImageIDs []string `json:"image_ids"`
		}
		if err := json.NewDecoder(h.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
			writeClipboardError(w, h.StatusBadRequest, "内容格式无效")
			return
		}
		request.Text = strings.TrimSpace(request.Text)
		if len([]rune(request.Text)) > maxClipboardTextRunes {
			writeClipboardError(w, h.StatusRequestEntityTooLarge, "文字不能超过 20 万字")
			return
		}
		request.ImageIDs = uniqueClipboardImageIDs(request.ImageIDs)
		if len(request.ImageIDs) > maxClipboardImages {
			writeClipboardError(w, h.StatusBadRequest, "每条记录最多支持 8 张图片")
			return
		}
		if request.Text == "" && len(request.ImageIDs) == 0 {
			writeClipboardError(w, h.StatusBadRequest, "请粘贴文字或图片")
			return
		}
		for _, imageID := range request.ImageIDs {
			if _, err := persistence.GetMediaAsset(account, imageID); err != nil {
				writeClipboardError(w, h.StatusBadRequest, "图片不存在或不属于当前账号")
				return
			}
		}
		id, err := newMediaID()
		if err != nil {
			writeClipboardError(w, h.StatusInternalServerError, "生成记录标识失败")
			return
		}
		item := persistence.ClipboardItem{
			ID: id, Account: account, Text: request.Text, ImageIDs: request.ImageIDs,
			CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := persistence.SaveClipboardItem(item); err != nil {
			log.ErrorF(log.ModuleBlog, "save clipboard item failed account=%s: %v", account, err)
			writeClipboardError(w, h.StatusInternalServerError, "保存贴板失败")
			return
		}
		writeClipboardJSON(w, h.StatusCreated, map[string]any{"success": true, "data": clipboardResponse(item)})
	case h.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeClipboardError(w, h.StatusBadRequest, "缺少记录标识")
			return
		}
		deleted, err := persistence.DeleteClipboardItem(account, id)
		if err != nil {
			writeClipboardError(w, h.StatusInternalServerError, "删除贴板失败")
			return
		}
		if !deleted {
			writeClipboardError(w, h.StatusNotFound, "记录不存在")
			return
		}
		writeClipboardJSON(w, h.StatusOK, map[string]any{"success": true})
	default:
		writeClipboardError(w, h.StatusMethodNotAllowed, "不支持该操作")
	}
}

func clipboardResponse(item persistence.ClipboardItem) clipboardItemResponse {
	images := make([]string, 0, len(item.ImageIDs))
	for _, id := range item.ImageIDs {
		images = append(images, "/media/"+id)
	}
	return clipboardItemResponse{ID: item.ID, Text: item.Text, Images: images, CreatedAt: item.CreatedAt}
}

func uniqueClipboardImageIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || strings.ContainsAny(id, `/\\`) {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func writeClipboardJSON(w h.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeClipboardError(w h.ResponseWriter, status int, message string) {
	writeClipboardJSON(w, status, map[string]any{"success": false, "message": message})
}
