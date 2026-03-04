package reading

import (
	"blog"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== Simple Reading 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

var (
	readingMu      sync.RWMutex
	books          map[string]map[string]*module.Book          // account -> bookID -> Book
	readingRecords map[string]map[string]*module.ReadingRecord // account -> bookID -> Record
	bookNotes      map[string]map[string][]*module.BookNote    // account -> bookID -> Notes
	bookInsights   map[string]map[string]*module.BookInsight   // account -> insightID -> Insight
	readingPlans   map[string]map[string]*module.ReadingPlan   // account -> planID -> Plan
)

func Info() {
	log.Debug(log.ModuleReading, "info reading v2.0 (simple)")
}

func Init() {
	log.Debug(log.ModuleReading, "reading module initialized")
	books = make(map[string]map[string]*module.Book)
	readingRecords = make(map[string]map[string]*module.ReadingRecord)
	bookNotes = make(map[string]map[string][]*module.BookNote)
	bookInsights = make(map[string]map[string]*module.BookInsight)
	readingPlans = make(map[string]map[string]*module.ReadingPlan)
}

// ========== 辅助函数 ==========

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:16]
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func ensureAccountData(account string) {
	if books[account] == nil {
		books[account] = make(map[string]*module.Book)
	}
	if readingRecords[account] == nil {
		readingRecords[account] = make(map[string]*module.ReadingRecord)
	}
	if bookNotes[account] == nil {
		bookNotes[account] = make(map[string][]*module.BookNote)
	}
	if bookInsights[account] == nil {
		bookInsights[account] = make(map[string]*module.BookInsight)
	}
	if readingPlans[account] == nil {
		readingPlans[account] = make(map[string]*module.ReadingPlan)
	}
}

// ========== Book 管理 ==========

func AddBookWithAccount(account, title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)

	if title == "" || author == "" {
		return nil, errors.New("书名和作者不能为空")
	}

	for _, book := range books[account] {
		if book.Title == title && book.Author == author {
			return nil, errors.New("该书籍已存在")
		}
	}

	bookID := generateID()
	book := &module.Book{
		ID: bookID, Title: title, Author: author, ISBN: isbn, Publisher: publisher,
		PublishDate: publishDate, CoverUrl: coverUrl, Description: description,
		TotalPages: totalPages, CurrentPage: 0, Category: category, Tags: tags,
		SourceUrl: sourceUrl, AddTime: strTime(), Rating: 0, Status: "unstart",
	}
	books[account][bookID] = book

	record := &module.ReadingRecord{
		BookID: bookID, Status: "unstart", CurrentPage: 0, LastUpdateTime: strTime(),
		ReadingSessions: []module.ReadingSession{},
	}
	readingRecords[account][bookID] = record

	saveBookToBlog(account, book)
	return book, nil
}

func GetBookWithAccount(account, bookID string) *module.Book {
	readingMu.RLock()
	defer readingMu.RUnlock()

	// 从 blog 系统加载
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil && data.Book != nil && data.Book.ID == bookID {
				return data.Book
			}
		}
	}
	if books[account] != nil {
		return books[account][bookID]
	}
	return nil
}

func GetAllBooksWithAccount(account string) map[string]*module.Book {
	readingMu.RLock()
	defer readingMu.RUnlock()

	result := make(map[string]*module.Book)
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil && data.Book != nil {
				result[data.Book.ID] = data.Book
			}
		}
	}
	if len(result) == 0 && books[account] != nil {
		return books[account]
	}
	return result
}

func UpdateBookWithAccount(account, bookID string, updates map[string]interface{}) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	book := getBookInternal(account, bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	if v, ok := updates["title"].(string); ok {
		book.Title = v
	}
	if v, ok := updates["author"].(string); ok {
		book.Author = v
	}
	if v, ok := updates["isbn"].(string); ok {
		book.ISBN = v
	}
	if v, ok := updates["publisher"].(string); ok {
		book.Publisher = v
	}
	if v, ok := updates["publish_date"].(string); ok {
		book.PublishDate = v
	}
	if v, ok := updates["cover_url"].(string); ok {
		book.CoverUrl = v
	}
	if v, ok := updates["description"].(string); ok {
		book.Description = v
	}
	if v, ok := updates["total_pages"].(int); ok {
		book.TotalPages = v
	}
	if v, ok := updates["category"].([]string); ok {
		book.Category = v
	}
	if v, ok := updates["tags"].([]string); ok {
		book.Tags = v
	}
	if v, ok := updates["rating"].(float64); ok {
		book.Rating = v
	}

	ensureAccountData(account)
	books[account][bookID] = book
	saveBookToBlog(account, book)
	return nil
}

func DeleteBookWithAccount(account, bookID string) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	book := getBookInternal(account, bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	blogTitle := fmt.Sprintf("reading_book_%s.md", book.Title)
	blog.DeleteBlogWithAccount(account, blogTitle)

	ensureAccountData(account)
	delete(books[account], bookID)
	delete(readingRecords[account], bookID)
	delete(bookNotes[account], bookID)
	return nil
}

func getBookInternal(account, bookID string) *module.Book {
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil && data.Book != nil && data.Book.ID == bookID {
				return data.Book
			}
		}
	}
	if books[account] != nil {
		return books[account][bookID]
	}
	return nil
}

// ========== Reading Progress ==========

func StartReadingWithAccount(account, bookID string) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	record, exists := readingRecords[account][bookID]
	if !exists {
		return errors.New("阅读记录不存在")
	}
	if record.Status == "reading" {
		return errors.New("已在阅读中")
	}

	record.Status = "reading"
	if record.StartDate == "" {
		record.StartDate = time.Now().Format("2006-01-02")
	}
	record.LastUpdateTime = strTime()

	if book, exists := books[account][bookID]; exists {
		book.Status = "reading"
		saveBookToBlog(account, book)
	}
	return nil
}

func UpdateReadingProgressWithAccount(account, bookID string, currentPage int, notes string) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	record, exists := readingRecords[account][bookID]
	if !exists {
		return errors.New("阅读记录不存在")
	}

	book := getBookInternal(account, bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	oldPage := record.CurrentPage
	record.CurrentPage = currentPage
	record.LastUpdateTime = strTime()
	book.CurrentPage = currentPage

	if record.Status == "unstart" {
		record.Status = "reading"
		record.StartDate = time.Now().Format("2006-01-02")
		book.Status = "reading"
	}

	if currentPage > oldPage {
		session := module.ReadingSession{
			Date: time.Now().Format("2006-01-02"), StartPage: oldPage, EndPage: currentPage, Notes: notes,
		}
		record.ReadingSessions = append(record.ReadingSessions, session)
	}

	if book.TotalPages > 0 && currentPage >= book.TotalPages {
		record.Status = "finished"
		record.EndDate = time.Now().Format("2006-01-02")
		book.Status = "finished"
	}

	books[account][bookID] = book
	readingRecords[account][bookID] = record
	saveBookToBlog(account, book)
	return nil
}

func GetReadingRecordWithAccount(account, bookID string) *module.ReadingRecord {
	readingMu.RLock()
	defer readingMu.RUnlock()

	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingRecord *module.ReadingRecord `json:"reading_record"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil && data.ReadingRecord != nil && data.ReadingRecord.BookID == bookID {
				return data.ReadingRecord
			}
		}
	}
	if readingRecords[account] != nil {
		return readingRecords[account][bookID]
	}
	return nil
}

// ========== Notes ==========

func AddBookNoteWithAccount(account, bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	readingMu.Lock()
	defer readingMu.Unlock()

	if content == "" {
		return nil, errors.New("笔记内容不能为空")
	}

	ensureAccountData(account)
	note := &module.BookNote{
		ID: generateID(), BookID: bookID, Type: noteType, Chapter: chapter,
		Page: page, Content: content, Tags: tags, CreateTime: strTime(), UpdateTime: strTime(),
	}
	bookNotes[account][bookID] = append(bookNotes[account][bookID], note)

	if book := getBookInternal(account, bookID); book != nil {
		books[account][bookID] = book
		saveBookToBlog(account, book)
	}
	return note, nil
}

func GetBookNotesWithAccount(account, bookID string) []*module.BookNote {
	readingMu.RLock()
	defer readingMu.RUnlock()

	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book      *module.Book       `json:"book"`
				BookNotes []*module.BookNote `json:"book_notes"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil && data.Book != nil && data.Book.ID == bookID {
				return data.BookNotes
			}
		}
	}
	if bookNotes[account] != nil {
		return bookNotes[account][bookID]
	}
	return []*module.BookNote{}
}

func UpdateBookNoteWithAccount(account, bookID, noteID string, updates map[string]interface{}) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	notes := bookNotes[account][bookID]
	for _, note := range notes {
		if note.ID == noteID {
			if v, ok := updates["content"].(string); ok {
				note.Content = v
			}
			if v, ok := updates["chapter"].(string); ok {
				note.Chapter = v
			}
			if v, ok := updates["page"].(int); ok {
				note.Page = v
			}
			if v, ok := updates["tags"].([]string); ok {
				note.Tags = v
			}
			note.UpdateTime = strTime()
			if book := getBookInternal(account, bookID); book != nil {
				books[account][bookID] = book
				saveBookToBlog(account, book)
			}
			return nil
		}
	}
	return errors.New("笔记不存在")
}

func DeleteBookNoteWithAccount(account, bookID, noteID string) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	notes := bookNotes[account][bookID]
	for i, note := range notes {
		if note.ID == noteID {
			bookNotes[account][bookID] = append(notes[:i], notes[i+1:]...)
			if book := getBookInternal(account, bookID); book != nil {
				books[account][bookID] = book
				saveBookToBlog(account, book)
			}
			return nil
		}
	}
	return errors.New("笔记不存在")
}

// ========== Insights ==========

func AddBookInsightWithAccount(account, bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	readingMu.Lock()
	defer readingMu.Unlock()

	if title == "" || content == "" {
		return nil, errors.New("标题和内容不能为空")
	}

	ensureAccountData(account)
	insight := &module.BookInsight{
		ID: generateID(), BookID: bookID, Title: title, Content: content,
		KeyTakeaways: keyTakeaways, Applications: applications, Rating: rating,
		Tags: tags, CreateTime: strTime(), UpdateTime: strTime(),
	}
	bookInsights[account][insight.ID] = insight

	if book := getBookInternal(account, bookID); book != nil && rating > 0 {
		book.Rating = float64(rating)
		books[account][bookID] = book
		saveBookToBlog(account, book)
	}
	return insight, nil
}

func GetBookInsightsWithAccount(account, bookID string) []*module.BookInsight {
	readingMu.RLock()
	defer readingMu.RUnlock()

	var results []*module.BookInsight
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if json.Unmarshal([]byte(b.Content), &data) == nil {
				for _, ins := range data.BookInsights {
					if ins.BookID == bookID {
						results = append(results, ins)
					}
				}
			}
		}
	}
	if len(results) == 0 && bookInsights[account] != nil {
		for _, ins := range bookInsights[account] {
			if ins.BookID == bookID {
				results = append(results, ins)
			}
		}
	}
	return results
}

func UpdateBookInsightWithAccount(account, insightID string, updates map[string]interface{}) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	insight, exists := bookInsights[account][insightID]
	if !exists {
		return errors.New("心得不存在")
	}

	if v, ok := updates["title"].(string); ok {
		insight.Title = v
	}
	if v, ok := updates["content"].(string); ok {
		insight.Content = v
	}
	if v, ok := updates["rating"].(int); ok {
		insight.Rating = v
	}
	if v, ok := updates["key_takeaways"].([]string); ok {
		insight.KeyTakeaways = v
	}
	if v, ok := updates["applications"].([]string); ok {
		insight.Applications = v
	}
	if v, ok := updates["tags"].([]string); ok {
		insight.Tags = v
	}
	insight.UpdateTime = strTime()

	if book := getBookInternal(account, insight.BookID); book != nil {
		books[account][insight.BookID] = book
		saveBookToBlog(account, book)
	}
	return nil
}

func DeleteBookInsightWithAccount(account, insightID string) error {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	if _, exists := bookInsights[account][insightID]; !exists {
		return errors.New("心得不存在")
	}
	delete(bookInsights[account], insightID)
	return nil
}

// ========== Search & Filter ==========

func SearchBooksWithAccount(account, keyword string) []*module.Book {
	readingMu.RLock()
	defer readingMu.RUnlock()

	keyword = strings.ToLower(keyword)
	allBooks := GetAllBooksWithAccount(account)
	var results []*module.Book
	for _, book := range allBooks {
		if strings.Contains(strings.ToLower(book.Title), keyword) ||
			strings.Contains(strings.ToLower(book.Author), keyword) ||
			strings.Contains(strings.ToLower(book.Description), keyword) {
			results = append(results, book)
		}
	}
	return results
}

func FilterBooksByStatusWithAccount(account, status string) []*module.Book {
	readingMu.RLock()
	defer readingMu.RUnlock()

	allBooks := GetAllBooksWithAccount(account)
	var results []*module.Book
	for _, book := range allBooks {
		if book.Status == status {
			results = append(results, book)
		}
	}
	return results
}

func FilterBooksByCategoryWithAccount(account, category string) []*module.Book {
	readingMu.RLock()
	defer readingMu.RUnlock()

	allBooks := GetAllBooksWithAccount(account)
	var results []*module.Book
	for _, book := range allBooks {
		for _, cat := range book.Category {
			if cat == category {
				results = append(results, book)
				break
			}
		}
	}
	return results
}

// ========== Statistics ==========

func GetReadingStatisticsWithAccount(account string) map[string]interface{} {
	readingMu.RLock()
	defer readingMu.RUnlock()

	allBooks := GetAllBooksWithAccount(account)
	stats := make(map[string]interface{})

	reading, finished, unstart := 0, 0, 0
	for _, book := range allBooks {
		switch book.Status {
		case "reading":
			reading++
		case "finished":
			finished++
		case "unstart":
			unstart++
		}
	}

	stats["total_books"] = len(allBooks)
	stats["reading_books"] = reading
	stats["finished_books"] = finished
	stats["unstart_books"] = unstart
	return stats
}

// ========== Reading Plans ==========

func AddReadingPlanWithAccount(account, title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	readingMu.Lock()
	defer readingMu.Unlock()

	ensureAccountData(account)
	plan := &module.ReadingPlan{
		ID: generateID(), Title: title, Description: description,
		StartDate: startDate, EndDate: endDate, TargetBooks: targetBooks,
		CreateTime: strTime(), Status: "active",
	}
	readingPlans[account][plan.ID] = plan
	return plan, nil
}

func GetReadingPlanWithAccount(account, planID string) *module.ReadingPlan {
	readingMu.RLock()
	defer readingMu.RUnlock()
	if readingPlans[account] != nil {
		return readingPlans[account][planID]
	}
	return nil
}

func GetAllReadingPlansWithAccount(account string) []*module.ReadingPlan {
	readingMu.RLock()
	defer readingMu.RUnlock()

	var results []*module.ReadingPlan
	if readingPlans[account] != nil {
		for _, plan := range readingPlans[account] {
			results = append(results, plan)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreateTime > results[j].CreateTime
	})
	return results
}

// ========== 持久化 ==========

func saveBookToBlog(account string, book *module.Book) {
	title := fmt.Sprintf("reading_book_%s.md", book.Title)
	data := map[string]interface{}{"book": book}

	if readingRecords[account] != nil {
		if record, exists := readingRecords[account][book.ID]; exists {
			data["reading_record"] = record
		}
	}
	if bookNotes[account] != nil {
		if notes, exists := bookNotes[account][book.ID]; exists {
			data["book_notes"] = notes
		}
	}

	var insights []*module.BookInsight
	if bookInsights[account] != nil {
		for _, ins := range bookInsights[account] {
			if ins.BookID == book.ID {
				insights = append(insights, ins)
			}
		}
	}
	data["book_insights"] = insights

	content, _ := json.MarshalIndent(data, "", "  ")
	ubd := &module.UploadedBlogData{
		Title: title, Content: string(content), Tags: "reading", AuthType: module.EAuthType_private, Account: account,
	}
	if blog.GetBlogWithAccount(account, title) == nil {
		blog.AddBlogWithAccount(account, ubd)
	} else {
		blog.ModifyBlogWithAccount(account, ubd)
	}
}

// ========== TODO 占位函数 ==========

func UpdateReadingPlanProgress(planID string) error { return nil }
func AddReadingGoal(year, month int, targetType string, targetValue int) (*module.ReadingGoal, error) {
	return nil, nil
}
func GetReadingGoals(year, month int) []*module.ReadingGoal { return nil }
func UpdateReadingGoalProgress(goalID string) error         { return nil }
func GenerateBookRecommendations(bookID string) ([]*module.BookRecommendation, error) {
	return nil, nil
}
func StartReadingSession(bookID string) (*module.ReadingTimeRecord, error) { return nil, nil }
func EndReadingSession(recordID string, pages int, notes string) error     { return nil }
func AddBookCollection(name, description string, bookIDs []string, isPublic bool) (*module.BookCollection, error) {
	return nil, nil
}
func GetBookCollection(collectionID string) *module.BookCollection  { return nil }
func GetAllBookCollections() []*module.BookCollection               { return nil }
func AddBookToCollection(collectionID, bookID string) error         { return nil }
func GetAdvancedReadingStatistics() map[string]interface{}          { return nil }
func ExportReadingData(config *module.ExportConfig) (string, error) { return "", nil }
