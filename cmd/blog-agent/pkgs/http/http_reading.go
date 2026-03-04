package http

import (
	"control"
	"encoding/json"
	"fmt"
	"lifecountdown"
	"module"
	log "mylog"
	h "net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"view"
)

// HandleReading handles the reading page
func HandleReading(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReading", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	// 检查是否有book参数，如果有则跳转到书籍详情页面
	bookTitle := r.URL.Query().Get("book")
	if bookTitle != "" {
		// 根据书名查找书籍ID
		account := getAccountFromRequest(r)
		books := control.GetAllBooks(account)
		for _, book := range books {
			if book.Title == bookTitle {
				// 跳转到书籍详情页面
				h.Redirect(w, r, fmt.Sprintf("/reading/book/%s", book.ID), 302)
				return
			}
		}
		// 如果没找到对应的书籍，重定向到reading页面
		h.Redirect(w, r, "/reading", 302)
		return
	}

	view.PageReading(w)
}

// HandleReadingDashboard handles the reading dashboard page
// 阅读仪表板页面处理函数
func HandleReadingDashboard(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingDashboard", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageReadingDashboard(w)
}

// HandleBooksAPI handles books API requests
// 读书API处理函数
func HandleBooksAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBooksAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		// 获取排序参数
		sortBy := r.URL.Query().Get("sort_by")
		sortOrder := r.URL.Query().Get("sort_order")

		// 设置默认排序：按添加时间倒序（最新添加的在前）
		if sortBy == "" {
			sortBy = "add_time"
		}
		if sortOrder == "" {
			sortOrder = "desc"
		}

		// 获取所有书籍
		account := getAccountFromRequest(r)
		books := control.GetAllBooks(account)
		booksSlice := make([]*module.Book, 0, len(books))
		for _, book := range books {
			booksSlice = append(booksSlice, book)
		}

		// 应用排序
		sortBooks(booksSlice, sortBy, sortOrder)

		response := map[string]interface{}{
			"success": true,
			"books":   booksSlice,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodPost:
		// 添加新书籍
		var bookData struct {
			Title       string   `json:"title"`
			Author      string   `json:"author"`
			ISBN        string   `json:"isbn"`
			Publisher   string   `json:"publisher"`
			PublishDate string   `json:"publish_date"`
			CoverUrl    string   `json:"cover_url"`
			Description string   `json:"description"`
			TotalPages  int      `json:"total_pages"`
			Category    []string `json:"category"`
			Tags        []string `json:"tags"`
			SourceUrl   string   `json:"source_url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&bookData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		account := getAccountFromRequest(r)
		book, err := control.AddBook(
			account,
			bookData.Title,
			bookData.Author,
			bookData.ISBN,
			bookData.Publisher,
			bookData.PublishDate,
			bookData.CoverUrl,
			bookData.Description,
			bookData.SourceUrl,
			bookData.TotalPages,
			bookData.Category,
			bookData.Tags,
		)

		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"book":    book,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodPut:
		// 编辑书籍
		bookID := r.URL.Query().Get("book_id")
		if bookID == "" {
			h.Error(w, "Book ID is required", h.StatusBadRequest)
			return
		}

		var updateData struct {
			Title       string   `json:"title"`
			Author      string   `json:"author"`
			ISBN        string   `json:"isbn"`
			Publisher   string   `json:"publisher"`
			PublishDate string   `json:"publish_date"`
			CoverUrl    string   `json:"cover_url"`
			Description string   `json:"description"`
			TotalPages  int      `json:"total_pages"`
			Category    []string `json:"category"`
			Tags        []string `json:"tags"`
			SourceUrl   string   `json:"source_url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		// 构建更新数据
		updates := make(map[string]interface{})
		if updateData.Title != "" {
			updates["title"] = updateData.Title
		}
		if updateData.Author != "" {
			updates["author"] = updateData.Author
		}
		if updateData.ISBN != "" {
			updates["isbn"] = updateData.ISBN
		}
		if updateData.Publisher != "" {
			updates["publisher"] = updateData.Publisher
		}
		if updateData.PublishDate != "" {
			updates["publish_date"] = updateData.PublishDate
		}
		if updateData.CoverUrl != "" {
			updates["cover_url"] = updateData.CoverUrl
		}
		if updateData.Description != "" {
			updates["description"] = updateData.Description
		}
		if updateData.TotalPages > 0 {
			updates["total_pages"] = updateData.TotalPages
		}
		if updateData.Category != nil {
			updates["category"] = updateData.Category
		}
		if updateData.Tags != nil {
			updates["tags"] = updateData.Tags
		}
		if updateData.SourceUrl != "" {
			updates["source_url"] = updateData.SourceUrl
		}

		account := getAccountFromRequest(r)
		err := control.UpdateBook(account, bookID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		// 获取更新后的书籍信息
		book := control.GetBook(account, bookID)
		if book == nil {
			h.Error(w, "Book not found after update", h.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"book":    book,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodDelete:
		// 删除书籍
		bookID := r.URL.Query().Get("book_id")
		if bookID == "" {
			log.ErrorF(log.ModuleReading, "删除书籍失败: 缺少book_id参数")
			h.Error(w, "Book ID is required", h.StatusBadRequest)
			return
		}

		log.DebugF(log.ModuleReading, "收到删除书籍请求: book_id=%s", bookID)

		// 先检查书籍是否存在
		account := getAccountFromRequest(r)
		book := control.GetBook(account, bookID)
		if book == nil {
			log.ErrorF(log.ModuleReading, "删除书籍失败: 书籍不存在, book_id=%s", bookID)
			h.Error(w, "书籍不存在", h.StatusBadRequest)
			return
		}
		log.DebugF(log.ModuleReading, "找到要删除的书籍: %s - %s", book.ID, book.Title)

		err := control.DeleteBook(account, bookID)
		if err != nil {
			log.ErrorF(log.ModuleReading, "删除书籍失败: book_id=%s, error=%v", bookID, err)
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		log.DebugF(log.ModuleReading, "书籍删除成功: book_id=%s", bookID)

		response := map[string]interface{}{
			"success": true,
			"message": "Book deleted successfully",
			"book_id": bookID,
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleReadingStatisticsAPI handles reading statistics API
// 读书统计API
func HandleReadingStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingStatisticsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	account := getAccountFromRequest(r)
	stats := control.GetReadingStatistics(account)
	json.NewEncoder(w).Encode(stats)
}

// HandleParseBookURL handles book URL parsing API
// URL解析API
func HandleParseBookURL(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleParseBookURL", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}

	// 简单的URL解析实现（实际应用中可以调用第三方API）
	bookData := parseBookFromURL(requestData.URL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookData)
}

// HandleBookDetail handles book detail page
// 书籍详情页面
func HandleBookDetail(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookDetail", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	// 从URL中提取书籍ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		h.Error(w, "Invalid book ID", h.StatusBadRequest)
		return
	}

	bookID := parts[3]
	account := getAccountFromRequest(r)
	book := control.GetBook(account, bookID)
	if book == nil {
		h.Error(w, "Book not found", h.StatusNotFound)
		return
	}

	view.PageBookDetail(w, book)
}

// HandleBookProgressAPI handles book progress update API
// 书籍进度更新API
func HandleBookProgressAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookProgressAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}

	var requestData struct {
		CurrentPage int `json:"current_page"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	err := control.UpdateReadingProgress(account, bookID, requestData.CurrentPage, "")
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Progress updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// HandleBookFinishAPI handles book finish marking API
// 书籍完成标记API
func HandleBookFinishAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookFinishAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	err := control.FinishBook(account, bookID)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Book marked as finished",
	}
	json.NewEncoder(w).Encode(response)
}

// HandleBookNotesAPI handles book notes API
// 书籍笔记API
func HandleBookNotesAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookNotesAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		// 获取笔记
		account := getAccountFromRequest(r)
		notes := control.GetBookNotes(account, bookID)
		json.NewEncoder(w).Encode(notes)

	case h.MethodPost:
		// 添加笔记
		var noteData struct {
			Chapter string `json:"chapter"`
			Page    int    `json:"page"`
			Content string `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&noteData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		account := getAccountFromRequest(r)
		note, err := control.AddBookNote(account, bookID, "note", noteData.Chapter, noteData.Content, noteData.Page, []string{})
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"note":    note,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodPut:
		// 更新笔记
		noteID := r.URL.Query().Get("note_id")
		if noteID == "" {
			h.Error(w, "Note ID is required", h.StatusBadRequest)
			return
		}

		var updateData struct {
			Chapter string `json:"chapter"`
			Page    int    `json:"page"`
			Content string `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		updates := make(map[string]interface{})
		if updateData.Chapter != "" {
			updates["chapter"] = updateData.Chapter
		}
		if updateData.Page >= 0 {
			updates["page"] = updateData.Page
		}
		if updateData.Content != "" {
			updates["content"] = updateData.Content
		}

		account := getAccountFromRequest(r)
		err := control.UpdateBookNote(account, bookID, noteID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Note updated successfully",
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodDelete:
		// 删除笔记
		noteID := r.URL.Query().Get("note_id")
		if noteID == "" {
			h.Error(w, "Note ID is required", h.StatusBadRequest)
			return
		}

		account := getAccountFromRequest(r)
		err := control.DeleteBookNote(account, bookID, noteID)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Note deleted successfully",
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleBookInsightsAPI handles book insights API
// 书籍心得API
func HandleBookInsightsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookInsightsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		// 获取心得
		account := getAccountFromRequest(r)
		insights := control.GetBookInsights(account, bookID)
		json.NewEncoder(w).Encode(insights)

	case h.MethodPost:
		// 添加心得
		var insightData struct {
			Title    string `json:"title"`
			Rating   int    `json:"rating"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			Takeaway string `json:"takeaway"`
		}

		if err := json.NewDecoder(r.Body).Decode(&insightData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		keyTakeaways := []string{}
		if insightData.Takeaway != "" {
			keyTakeaways = append(keyTakeaways, insightData.Takeaway)
		}

		account := getAccountFromRequest(r)
		insight, err := control.AddBookInsight(
			account,
			bookID,
			insightData.Title,
			insightData.Content,
			keyTakeaways,
			[]string{},
			insightData.Rating,
			[]string{},
		)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"insight": insight,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodPut:
		// 更新心得
		insightID := r.URL.Query().Get("insight_id")
		if insightID == "" {
			h.Error(w, "Insight ID is required", h.StatusBadRequest)
			return
		}

		var updateData struct {
			Title    string `json:"title"`
			Rating   int    `json:"rating"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			Takeaway string `json:"takeaway"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		updates := make(map[string]interface{})
		if updateData.Title != "" {
			updates["title"] = updateData.Title
		}
		if updateData.Content != "" {
			updates["content"] = updateData.Content
		}
		if updateData.Rating > 0 {
			updates["rating"] = updateData.Rating
		}
		if updateData.Takeaway != "" {
			updates["key_takeaways"] = []string{updateData.Takeaway}
		}

		account := getAccountFromRequest(r)
		err := control.UpdateBookInsight(account, insightID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Insight updated successfully",
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodDelete:
		// 删除心得
		insightID := r.URL.Query().Get("insight_id")
		if insightID == "" {
			h.Error(w, "Insight ID is required", h.StatusBadRequest)
			return
		}

		account := getAccountFromRequest(r)
		err := control.DeleteBookInsight(account, insightID)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Insight deleted successfully",
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleReadingPlansAPI handles reading plans API
func HandleReadingPlansAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingPlansAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		account := getAccountFromRequest(r)
		plans := control.GetAllReadingPlans(account)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"plans":   plans,
		})

	case h.MethodPost:
		var planData struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			StartDate   string   `json:"start_date"`
			EndDate     string   `json:"end_date"`
			TargetBooks []string `json:"target_books"`
		}

		if err := json.NewDecoder(r.Body).Decode(&planData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		account := getAccountFromRequest(r)
		plan, err := control.AddReadingPlan(
			account,
			planData.Title,
			planData.Description,
			planData.StartDate,
			planData.EndDate,
			planData.TargetBooks,
		)

		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"plan":    plan,
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleReadingGoalsAPI handles reading goals API
// 阅读目标API
func HandleReadingGoalsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingGoalsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		yearStr := r.URL.Query().Get("year")
		monthStr := r.URL.Query().Get("month")

		year := time.Now().Year()
		month := 0

		if yearStr != "" {
			if y, err := strconv.Atoi(yearStr); err == nil {
				year = y
			}
		}

		if monthStr != "" {
			if m, err := strconv.Atoi(monthStr); err == nil {
				month = m
			}
		}

		goals := control.GetReadingGoals(year, month)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"goals":   goals,
		})

	case h.MethodPost:
		var goalData struct {
			Year        int    `json:"year"`
			Month       int    `json:"month"`
			TargetType  string `json:"target_type"`
			TargetValue int    `json:"target_value"`
		}

		if err := json.NewDecoder(r.Body).Decode(&goalData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		goal, err := control.AddReadingGoal(
			goalData.Year,
			goalData.Month,
			goalData.TargetType,
			goalData.TargetValue,
		)

		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"goal":    goal,
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleBookRecommendationsAPI handles book recommendations API
// 书籍推荐API
func HandleBookRecommendationsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookRecommendationsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}

	recommendations, err := control.GenerateBookRecommendations(bookID)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"recommendations": recommendations,
	})
}

// HandleReadingSessionAPI handles reading session API
// 阅读时间记录API
func HandleReadingSessionAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingSessionAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodPost:
		var sessionData struct {
			BookID string `json:"book_id"`
			Action string `json:"action"` // start or end
			Pages  int    `json:"pages"`
			Notes  string `json:"notes"`
		}

		if err := json.NewDecoder(r.Body).Decode(&sessionData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		if sessionData.Action == "start" {
			session, err := control.StartReadingSession(sessionData.BookID)
			if err != nil {
				h.Error(w, err.Error(), h.StatusBadRequest)
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"session": session,
			})
		} else {
			h.Error(w, "Invalid action", h.StatusBadRequest)
		}

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleBookCollectionsAPI handles book collections API
// 书籍收藏夹API
func HandleBookCollectionsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookCollectionsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		collections := control.GetAllBookCollections()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"collections": collections,
		})

	case h.MethodPost:
		var collectionData struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			BookIDs     []string `json:"book_ids"`
			IsPublic    bool     `json:"is_public"`
		}

		if err := json.NewDecoder(r.Body).Decode(&collectionData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		collection, err := control.AddBookCollection(
			collectionData.Name,
			collectionData.Description,
			collectionData.BookIDs,
			collectionData.IsPublic,
		)

		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"collection": collection,
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleAdvancedReadingStatisticsAPI handles advanced reading statistics API
// 高级统计API
func HandleAdvancedReadingStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAdvancedReadingStatisticsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	stats := control.GetAdvancedReadingStatistics()
	json.NewEncoder(w).Encode(stats)
}

// HandleExportReadingDataAPI handles reading data export API
// 数据导出API
func HandleExportReadingDataAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExportReadingDataAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	var exportConfig module.ExportConfig
	if err := json.NewDecoder(r.Body).Decode(&exportConfig); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}

	data, err := control.ExportReadingData(&exportConfig)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// HandleLifeCountdown handles life countdown page
// 人生倒计时页面处理函数
func HandleLifeCountdown(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdown", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageLifeCountdown(w)
}

// HandleLifeCountdownAPI handles life countdown API
// 人生倒计时API处理函数
func HandleLifeCountdownAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdownAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodPost:
		// 计算人生倒计时数据
		var config lifecountdown.UserConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}

		data := lifecountdown.CalculateLifeCountdown(config)

		response := map[string]interface{}{
			"success": true,
			"data":    data,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodGet:
		// 获取书籍列表用于可视化
		account := getAccountFromRequest(r)
		booksMap := control.GetAllBooks(account)
		bookTitles := make([]string, 0, len(booksMap))

		for _, book := range booksMap {
			if book != nil && book.Title != "" {
				bookTitles = append(bookTitles, book.Title)
			}
		}

		// 如果没有书籍，使用默认列表
		if len(bookTitles) == 0 {
			bookTitles = []string{
				"时间简史", "活着", "百年孤独", "思考快与慢", "人类简史",
				"原则", "三体", "1984", "深度工作", "认知觉醒", "心流",
				"经济学原理", "创新者", "未来简史", "影响力", "黑天鹅",
				"毛泽东传", "邓小平传", "红楼梦", "西游记", "水浒传",
				"三国演义", "论语", "孟子", "老子", "庄子", "史记",
			}
		}

		log.DebugF(log.ModuleReading, "获取书籍列表: 共%d本书", len(bookTitles))

		response := map[string]interface{}{
			"success": true,
			"books":   bookTitles,
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleLifeCountdownConfigAPI handles life countdown configuration API
// 人生倒计时配置API处理函数
func HandleLifeCountdownConfigAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdownConfigAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	account := getAccountFromRequest(r)

	switch r.Method {
	case h.MethodGet:
		// 获取保存的配置
		blog := control.GetBlog(account, "lifecountdown.md")
		if blog == nil {
			// 如果配置不存在，返回默认配置
			defaultConfig := map[string]interface{}{
				"currentAge":        25,
				"expectedLifespan":  80,
				"dailySleepHours":   8.0,
				"dailyStudyHours":   2.0,
				"dailyReadingHours": 1.0,
				"dailyWorkHours":    8.0,
				"averageBookWords":  150000,
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":   true,
				"config":    defaultConfig,
				"isDefault": true,
			})
			return
		}

		// 尝试解析保存的配置
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(blog.Content), &config); err != nil {
			// 解析失败，返回默认配置
			defaultConfig := map[string]interface{}{
				"currentAge":        25,
				"expectedLifespan":  80,
				"dailySleepHours":   8.0,
				"dailyStudyHours":   2.0,
				"dailyReadingHours": 1.0,
				"dailyWorkHours":    8.0,
				"averageBookWords":  150000,
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":   true,
				"config":    defaultConfig,
				"isDefault": true,
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"config":    config,
			"isDefault": false,
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// Helper functions

// parseBookFromURL parses book information from URL (simplified implementation)
// 简单的URL解析函数（可以扩展支持更多网站）
func parseBookFromURL(url string) map[string]interface{} {
	// 这里是一个简化的实现，实际应用中可以：
	// 1. 调用豆瓣API
	// 2. 爬取网页内容
	// 3. 调用其他图书信息API

	result := map[string]interface{}{
		"title":       "示例书籍",
		"author":      "示例作者",
		"publisher":   "示例出版社",
		"isbn":        "9787111111111",
		"description": "这是一个从URL解析的示例书籍信息。实际应用中，这里会调用相应的API或爬虫来获取真实的书籍信息。",
		"cover_url":   "",
		"source_url":  url,
	}

	// 根据不同的URL来源进行不同的解析
	if strings.Contains(url, "douban.com") {
		result["title"] = "豆瓣书籍示例"
		result["description"] = "从豆瓣读书解析的书籍信息"
	} else if strings.Contains(url, "amazon.com") {
		result["title"] = "亚马逊书籍示例"
		result["description"] = "从亚马逊解析的书籍信息"
	}

	return result
}

// sortBooks sorts books based on given criteria
// 书籍排序函数
func sortBooks(books []*module.Book, sortBy string, sortOrder string) {
	if len(books) <= 1 {
		return
	}

	// 根据排序字段确定比较函数
	var compareFunc func(i, j int) bool

	switch sortBy {
	case "add_time":
		// 按添加时间排序
		compareFunc = func(i, j int) bool {
			timeI := parseTimeOrDefault(books[i].AddTime)
			timeJ := parseTimeOrDefault(books[j].AddTime)
			if sortOrder == "desc" {
				return timeI.After(timeJ)
			}
			return timeI.Before(timeJ)
		}
	case "title":
		// 按书名排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Title > books[j].Title
			}
			return books[i].Title < books[j].Title
		}
	case "author":
		// 按作者排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Author > books[j].Author
			}
			return books[i].Author < books[j].Author
		}
	case "rating":
		// 按评分排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Rating > books[j].Rating
			}
			return books[i].Rating < books[j].Rating
		}
	case "progress":
		// 按阅读进度排序
		compareFunc = func(i, j int) bool {
			progressI := calculateProgress(books[i])
			progressJ := calculateProgress(books[j])
			if sortOrder == "desc" {
				return progressI > progressJ
			}
			return progressI < progressJ
		}
	case "status":
		// 按状态排序，优先级：reading > unstart > finished > paused
		compareFunc = func(i, j int) bool {
			priorityI := getStatusPriority(books[i].Status)
			priorityJ := getStatusPriority(books[j].Status)
			if sortOrder == "desc" {
				return priorityI > priorityJ
			}
			return priorityI < priorityJ
		}
	case "pages":
		// 按总页数排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].TotalPages > books[j].TotalPages
			}
			return books[i].TotalPages < books[j].TotalPages
		}
	default:
		// 默认按添加时间排序
		compareFunc = func(i, j int) bool {
			timeI := parseTimeOrDefault(books[i].AddTime)
			timeJ := parseTimeOrDefault(books[j].AddTime)
			return timeI.After(timeJ) // 默认倒序
		}
	}

	sort.Slice(books, compareFunc)
}

// parseTimeOrDefault parses time string or returns zero time on failure
// 解析时间，如果失败则返回零值时间
func parseTimeOrDefault(timeStr string) time.Time {
	if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
		return t
	}
	return time.Time{}
}

// calculateProgress calculates reading progress percentage
// 计算阅读进度百分比
func calculateProgress(book *module.Book) float64 {
	if book.TotalPages <= 0 {
		return 0.0
	}
	return float64(book.CurrentPage) / float64(book.TotalPages) * 100.0
}

// getStatusPriority gets status priority for sorting
// 获取状态优先级（用于排序）
func getStatusPriority(status string) int {
	switch status {
	case "reading":
		return 4
	case "unstart":
		return 3
	case "finished":
		return 2
	case "paused":
		return 1
	default:
		return 0
	}
}
