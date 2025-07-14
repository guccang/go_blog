package reading

import (
	"module"
	log "mylog"
	"time"
	"fmt"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"errors"
	"sort"
	"encoding/json"
	"blog"
)

func Info() {
	fmt.Println("info reading v1.0")
}

// 全局数据存储
var (
	Books         = make(map[string]*module.Book)
	ReadingRecords = make(map[string]*module.ReadingRecord)
	BookNotes     = make(map[string][]*module.BookNote)
	BookInsights  = make(map[string]*module.BookInsight)
	ReadingPlans  = make(map[string]*module.ReadingPlan)
	ReadingGoals  = make(map[string]*module.ReadingGoal)
	BookRecommendations = make(map[string]*module.BookRecommendation)
	BookCollections = make(map[string]*module.BookCollection)
	ReadingTimeRecords = make(map[string][]*module.ReadingTimeRecord)
)

// 生成唯一ID
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%s", hex.EncodeToString(bytes)[:16])
}

// 获取当前时间字符串
func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// 初始化读书模块
func Init() {
	// 从数据库加载数据
	loadBooks()
	loadReadingRecords()
	loadBookNotes()
	loadBookInsights()
	loadReadingPlans()
	loadReadingGoals()
	loadBookCollections()
	loadReadingTimeRecords()
	
	log.DebugF("Reading module initialized - Books: %d, Records: %d, Notes: %d, Insights: %d, Plans: %d, Goals: %d, Collections: %d", 
		len(Books), len(ReadingRecords), getTotalNotesCount(), len(BookInsights), len(ReadingPlans), len(ReadingGoals), len(BookCollections))
}

// 书籍管理功能
func AddBook(title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	if title == "" || author == "" {
		return nil, errors.New("书名和作者不能为空")
	}

	// 检查是否已存在相同书籍
	for _, book := range Books {
		if book.Title == title && book.Author == author {
			return nil, errors.New("该书籍已存在")
		}
	}

	bookID := generateID()
	book := &module.Book{
		ID:          bookID,
		Title:       title,
		Author:      author,
		ISBN:        isbn,
		Publisher:   publisher,
		PublishDate: publishDate,
		CoverUrl:    coverUrl,
		Description: description,
		TotalPages:  totalPages,
		CurrentPage: 0,
		Category:    category,
		Tags:        tags,
		SourceUrl:   sourceUrl,
		AddTime:     strTime(),
		Rating:      0,
		Status:      "unstart",
	}

	Books[bookID] = book
	
	// 创建初始阅读记录
	record := &module.ReadingRecord{
		BookID:           bookID,
		Status:           "unstart",
		StartDate:        "",
		EndDate:          "",
		CurrentPage:      0,
		TotalReadingTime: 0,
		ReadingSessions:  []module.ReadingSession{},
		LastUpdateTime:   strTime(),
	}
	ReadingRecords[bookID] = record

	// 保存到数据库
	saveBook(book)
	saveReadingRecord(record)

	log.DebugF("添加书籍成功: %s - %s", title, author)
	return book, nil
}

func GetBook(bookID string) *module.Book {
	// 从blog系统获取单本书数据
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.Book != nil && data.Book.ID == bookID {
				return data.Book
			}
		}
	}
	
	// 如果blog中没有找到，则从原有Redis加载（兼容性）
	return Books[bookID]
}

func GetAllBooks() map[string]*module.Book {
	// 从blog系统获取reading数据
	books := make(map[string]*module.Book)
	
	// 遍历所有blog，查找reading_book_开头的
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			// 解析JSON内容
			var data struct {
				Book *module.Book `json:"book"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.Book != nil {
				books[data.Book.ID] = data.Book
			}
		}
	}
	
	// 如果blog中没有数据，则从原有Redis加载（兼容性）
	if len(books) == 0 {
		return Books
	}
	
	return books
}

func UpdateBook(bookID string, updates map[string]interface{}) error {
	// 先从blog系统获取书籍信息，确保我们有最新的数据
	book := GetBook(bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	// 更新字段
	if title, ok := updates["title"].(string); ok {
		book.Title = title
	}
	if author, ok := updates["author"].(string); ok {
		book.Author = author
	}
	if isbn, ok := updates["isbn"].(string); ok {
		book.ISBN = isbn
	}
	if publisher, ok := updates["publisher"].(string); ok {
		book.Publisher = publisher
	}
	if publishDate, ok := updates["publish_date"].(string); ok {
		book.PublishDate = publishDate
	}
	if coverUrl, ok := updates["cover_url"].(string); ok {
		book.CoverUrl = coverUrl
	}
	if description, ok := updates["description"].(string); ok {
		book.Description = description
	}
	if totalPages, ok := updates["total_pages"].(int); ok {
		book.TotalPages = totalPages
	}
	if category, ok := updates["category"].([]string); ok {
		book.Category = category
	}
	if tags, ok := updates["tags"].([]string); ok {
		book.Tags = tags
	}
	if rating, ok := updates["rating"].(float64); ok {
		book.Rating = rating
	}

	// 更新内存中的数据
	Books[bookID] = book
	saveBook(book)
	log.DebugF("更新书籍成功: %s", bookID)
	return nil
}

func DeleteBook(bookID string) error {
	// 先从blog系统获取书籍信息，确保我们有完整的数据
	book := GetBook(bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	// 构建blog标题用于删除
	blogTitle := fmt.Sprintf("reading_book_%s.md", book.Title)
	
	log.DebugF("准备删除书籍blog: %s (书籍ID: %s)", blogTitle, bookID)
	
	// 首先检查blog是否存在
	existingBlog := blog.GetBlog(blogTitle)
	if existingBlog == nil {
		log.ErrorF("要删除的blog不存在: %s，可能已经被手动删除", blogTitle)
		// 如果blog不存在，直接删除内存数据即可
	} else {
		log.DebugF("找到要删除的blog: %s", blogTitle)
		
		// 从blog系统删除 - 这是关键步骤
		result := blog.DeleteBlog(blogTitle)
		if result != 0 {
			var errorMsg string
			switch result {
			case 1:
				errorMsg = "blog不存在"
			case 2:
				errorMsg = "该文件被标记为系统文件，无法删除"
			case 3:
				errorMsg = "数据库删除失败"
			default:
				errorMsg = fmt.Sprintf("未知错误 (错误码: %d)", result)
			}
			log.ErrorF("从blog系统删除书籍失败: %s, %s", blogTitle, errorMsg)
			return fmt.Errorf("删除书籍失败：%s", errorMsg)
		}
		
		log.DebugF("从blog系统删除书籍成功: %s", blogTitle)
	}

	// 删除内存中的相关数据
	delete(Books, bookID)
	delete(ReadingRecords, bookID)
	delete(BookNotes, bookID)
	
	// 删除所有相关的心得（从内存中删除）
	deletedInsights := 0
	for insightID, insight := range BookInsights {
		if insight.BookID == bookID {
			delete(BookInsights, insightID)
			deletedInsights++
		}
	}

	log.DebugF("删除书籍成功: %s - %s (同时删除了 %d 条心得)", bookID, book.Title, deletedInsights)
	return nil
}

// 阅读记录功能
func StartReading(bookID string) error {
	record, exists := ReadingRecords[bookID]
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

	// 更新书籍状态
	if book, exists := Books[bookID]; exists {
		book.Status = "reading"
		saveBook(book)
	}

	saveReadingRecord(record)
	log.DebugF("开始阅读: %s", bookID)
	return nil
}

func UpdateReadingProgress(bookID string, currentPage int, notes string) error {
	record, exists := ReadingRecords[bookID]
	if !exists {
		return errors.New("阅读记录不存在")
	}

	book := Books[bookID]
	if book == nil {
		return errors.New("书籍不存在")
	}

	oldPage := record.CurrentPage
	record.CurrentPage = currentPage
	record.LastUpdateTime = strTime()

	// 同步更新Book结构体的CurrentPage
	book.CurrentPage = currentPage
	saveBook(book)

	// 如果是首次更新进度，自动开始阅读
	if record.Status == "unstart" {
		StartReading(bookID)
	}

	// 创建阅读会话记录
	if currentPage > oldPage {
		session := module.ReadingSession{
			Date:      time.Now().Format("2006-01-02"),
			StartPage: oldPage,
			EndPage:   currentPage,
			Duration:  0, // 可以后续添加时间追踪
			Notes:     notes,
		}
		record.ReadingSessions = append(record.ReadingSessions, session)
	}

	// 检查是否完成阅读
	if currentPage >= book.TotalPages && book.TotalPages > 0 {
		record.Status = "finished"
		record.EndDate = time.Now().Format("2006-01-02")
		book.Status = "finished"
		saveBook(book)
	}

	saveReadingRecord(record)
	log.DebugF("更新阅读进度: %s - 第%d页", bookID, currentPage)
	return nil
}

func GetReadingRecord(bookID string) *module.ReadingRecord {
	// 从blog系统获取阅读记录
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingRecord *module.ReadingRecord `json:"reading_record"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.ReadingRecord != nil && data.ReadingRecord.BookID == bookID {
				return data.ReadingRecord
			}
		}
	}
	
	// 如果blog中没有找到，则从原有Redis加载（兼容性）
	return ReadingRecords[bookID]
}

// 笔记功能
func AddBookNote(bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	if content == "" {
		return nil, errors.New("笔记内容不能为空")
	}

	noteID := generateID()
	note := &module.BookNote{
		ID:         noteID,
		BookID:     bookID,
		Type:       noteType,
		Chapter:    chapter,
		Page:       page,
		Content:    content,
		Tags:       tags,
		CreateTime: strTime(),
		UpdateTime: strTime(),
	}

	if BookNotes[bookID] == nil {
		BookNotes[bookID] = []*module.BookNote{}
	}
	BookNotes[bookID] = append(BookNotes[bookID], note)

	saveBookNotes(bookID)
	log.DebugF("添加笔记成功: %s - %s", bookID, noteType)
	return note, nil
}

func GetBookNotes(bookID string) []*module.BookNote {
	// 从blog系统获取笔记数据
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book      *module.Book        `json:"book"`
				BookNotes []*module.BookNote `json:"book_notes"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				// 检查是否是该书的数据
				if data.Book != nil && data.Book.ID == bookID {
					var bookNotes []*module.BookNote
					// 过滤出属于该书的笔记
					for _, note := range data.BookNotes {
						if note.BookID == bookID {
							bookNotes = append(bookNotes, note)
						}
					}
					return bookNotes
				}
			}
		}
	}
	
	// 如果blog中没有找到，则从原有Redis加载（兼容性）
	notes := BookNotes[bookID]
	if notes == nil {
		return []*module.BookNote{}
	}
	return notes
}

func UpdateBookNote(bookID, noteID string, updates map[string]interface{}) error {
	// 先从blog系统加载最新数据
	notes := GetBookNotes(bookID)
	if notes == nil || len(notes) == 0 {
		return errors.New("笔记不存在")
	}

	var targetNote *module.BookNote
	for _, note := range notes {
		if note.ID == noteID {
			targetNote = note
			break
		}
	}

	if targetNote == nil {
		return errors.New("笔记不存在")
	}

	// 更新字段
	if content, ok := updates["content"].(string); ok {
		targetNote.Content = content
	}
	if chapter, ok := updates["chapter"].(string); ok {
		targetNote.Chapter = chapter
	}
	if page, ok := updates["page"].(int); ok {
		targetNote.Page = page
	}
	if tags, ok := updates["tags"].([]string); ok {
		targetNote.Tags = tags
	}
	targetNote.UpdateTime = strTime()

	// 更新内存中的数据
	BookNotes[bookID] = notes
	saveBookNotes(bookID)
	log.DebugF("更新笔记成功: %s", noteID)
	return nil
}

// 删除笔记
func DeleteBookNote(bookID, noteID string) error {
	// 先从blog系统加载最新数据
	notes := GetBookNotes(bookID)
	if notes == nil || len(notes) == 0 {
		return errors.New("笔记不存在")
	}

	// 查找并删除笔记
	found := false
	var updatedNotes []*module.BookNote
	for _, note := range notes {
		if note.ID == noteID {
			found = true
			// 跳过这个笔记，相当于删除
		} else {
			updatedNotes = append(updatedNotes, note)
		}
	}

	if !found {
		return errors.New("笔记不存在")
	}

	// 更新内存中的数据
	BookNotes[bookID] = updatedNotes
	saveBookNotes(bookID)
	log.DebugF("删除笔记成功: %s", noteID)
	return nil
}

// 读书感悟功能
func AddBookInsight(bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	if title == "" || content == "" {
		return nil, errors.New("标题和内容不能为空")
	}

	insightID := generateID()
	insight := &module.BookInsight{
		ID:           insightID,
		BookID:       bookID,
		Title:        title,
		Content:      content,
		KeyTakeaways: keyTakeaways,
		Applications: applications,
		Rating:       rating,
		Tags:         tags,
		CreateTime:   strTime(),
		UpdateTime:   strTime(),
	}

	BookInsights[insightID] = insight

	// 更新书籍评分
	if book := Books[bookID]; book != nil && rating > 0 {
		book.Rating = float64(rating)
		saveBook(book)
	}

	saveBookInsight(insight)
	log.DebugF("添加读书感悟成功: %s - %s", bookID, title)
	return insight, nil
}

func GetBookInsights(bookID string) []*module.BookInsight {
	// 从blog系统获取感悟数据
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				// 筛选该书的感悟
				var insights []*module.BookInsight
				for _, insight := range data.BookInsights {
					if insight.BookID == bookID {
						insights = append(insights, insight)
					}
				}
				return insights
			}
		}
	}
	
	// 如果blog中没有找到，则从原有Redis加载（兼容性）
	var insights []*module.BookInsight
	for _, insight := range BookInsights {
		if insight.BookID == bookID {
			insights = append(insights, insight)
		}
	}
	return insights
}

// 更新心得
func UpdateBookInsight(insightID string, updates map[string]interface{}) error {
	// 先从blog系统查找心得
	var insight *module.BookInsight
	var bookID string
	
	// 遍历所有书籍数据查找指定的心得
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, ins := range data.BookInsights {
					if ins.ID == insightID {
						insight = ins
						bookID = ins.BookID
						break
					}
				}
				if insight != nil {
					break
				}
			}
		}
	}
	
	// 如果blog中没有找到，从内存中查找
	if insight == nil {
		var exists bool
		insight, exists = BookInsights[insightID]
		if !exists {
			return errors.New("心得不存在")
		}
		bookID = insight.BookID
	}

	// 更新字段
	if title, ok := updates["title"].(string); ok {
		insight.Title = title
	}
	if content, ok := updates["content"].(string); ok {
		insight.Content = content
	}
	if rating, ok := updates["rating"].(int); ok {
		insight.Rating = rating
		// 更新书籍评分
		if book := GetBook(bookID); book != nil && rating > 0 {
			book.Rating = float64(rating)
			Books[bookID] = book
			saveBook(book)
		}
	}
	if takeaways, ok := updates["key_takeaways"].([]string); ok {
		insight.KeyTakeaways = takeaways
	}
	if applications, ok := updates["applications"].([]string); ok {
		insight.Applications = applications
	}
	if tags, ok := updates["tags"].([]string); ok {
		insight.Tags = tags
	}
	insight.UpdateTime = strTime()

	// 更新内存中的数据
	BookInsights[insightID] = insight
	saveBookInsight(insight)
	log.DebugF("更新心得成功: %s", insightID)
	return nil
}

// 删除心得
func DeleteBookInsight(insightID string) error {
	// 先查找心得并获取bookID
	var foundInsight *module.BookInsight
	var targetBookID string
	
	// 从blog系统查找心得
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, insight := range data.BookInsights {
					if insight.ID == insightID {
						foundInsight = insight
						targetBookID = insight.BookID
						break
					}
				}
				if foundInsight != nil {
					break
				}
			}
		}
	}
	
	// 如果blog中没有找到，从内存中查找
	if foundInsight == nil {
		var exists bool
		foundInsight, exists = BookInsights[insightID]
		if !exists {
			return errors.New("心得不存在")
		}
		targetBookID = foundInsight.BookID
	}

	// 从内存中删除
	delete(BookInsights, insightID)
	
	// 通过saveBook保存更新后的数据
	if book := GetBook(targetBookID); book != nil {
		Books[targetBookID] = book
		saveBook(book)
	}
	
	log.DebugF("删除心得成功: %s", insightID)
	return nil
}

// 搜索和筛选功能
func SearchBooks(keyword string) []*module.Book {
	var results []*module.Book
	keyword = strings.ToLower(keyword)
	
	for _, book := range Books {
		if strings.Contains(strings.ToLower(book.Title), keyword) ||
		   strings.Contains(strings.ToLower(book.Author), keyword) ||
		   strings.Contains(strings.ToLower(book.Description), keyword) {
			results = append(results, book)
		}
	}
	
	return results
}

func FilterBooksByStatus(status string) []*module.Book {
	var results []*module.Book
	for _, book := range Books {
		if book.Status == status {
			results = append(results, book)
		}
	}
	return results
}

func FilterBooksByCategory(category string) []*module.Book {
	var results []*module.Book
	for _, book := range Books {
		for _, cat := range book.Category {
			if cat == category {
				results = append(results, book)
				break
			}
		}
	}
	return results
}

// 统计功能
func GetReadingStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	
	totalBooks := len(Books)
	readingBooks := len(FilterBooksByStatus("reading"))
	finishedBooks := len(FilterBooksByStatus("finished"))
	unstartBooks := len(FilterBooksByStatus("unstart"))
	
	totalPages := 0
	totalReadingTime := 0
	for _, record := range ReadingRecords {
		if book := Books[record.BookID]; book != nil {
			if record.Status == "finished" {
				totalPages += book.TotalPages
			} else {
				totalPages += record.CurrentPage
			}
		}
		totalReadingTime += record.TotalReadingTime
	}
	
	stats["total_books"] = totalBooks
	stats["reading_books"] = readingBooks
	stats["finished_books"] = finishedBooks
	stats["unstart_books"] = unstartBooks
	stats["total_pages"] = totalPages
	stats["total_reading_time"] = totalReadingTime
	stats["total_notes"] = getTotalNotesCount()
	stats["total_insights"] = len(BookInsights)
	
	return stats
}

// 辅助函数
func getTotalNotesCount() int {
	count := 0
	for _, notes := range BookNotes {
		count += len(notes)
	}
	return count
}

// 数据持久化函数
func saveBook(book *module.Book) {
	// 将书籍数据保存到blog系统
	title := fmt.Sprintf("reading_book_%s.md", book.Title)
	
	// 构建完整的书籍数据（包括相关记录）
	data := map[string]interface{}{
		"book": book,
	}
	
	// 添加阅读记录
	if record, exists := ReadingRecords[book.ID]; exists {
		data["reading_record"] = record
	}
	
	// 添加笔记
	if notes, exists := BookNotes[book.ID]; exists {
		data["book_notes"] = notes
	}
	
	// 添加心得
	var insights []*module.BookInsight
	for _, insight := range BookInsights {
		if insight.BookID == book.ID {
			insights = append(insights, insight)
		}
	}
	if len(insights) > 0 {
		data["book_insights"] = insights
	}
	
	// 添加阅读计划（包含该书籍的计划）
	var plans []*module.ReadingPlan
	for _, plan := range ReadingPlans {
		for _, targetBookID := range plan.TargetBooks {
			if targetBookID == book.ID {
				plans = append(plans, plan)
				break
			}
		}
	}
	if len(plans) > 0 {
		data["reading_plans"] = plans
	}
	
	// 添加阅读目标
	var goals []*module.ReadingGoal
	for _, goal := range ReadingGoals {
		goals = append(goals, goal)
	}
	if len(goals) > 0 {
		data["reading_goals"] = goals
	}
	
	// 添加书籍收藏夹（包含该书籍的收藏夹）
	var collections []*module.BookCollection
	for _, collection := range BookCollections {
		for _, bookID := range collection.BookIDs {
			if bookID == book.ID {
				collections = append(collections, collection)
				break
			}
		}
	}
	if len(collections) > 0 {
		data["book_collections"] = collections
	}
	
	// 添加阅读时间记录
	if records, exists := ReadingTimeRecords[book.ID]; exists {
		data["reading_time_records"] = map[string][]*module.ReadingTimeRecord{
			book.ID: records,
		}
	}
	
	// 序列化为JSON
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.ErrorF("序列化书籍数据失败: %v", err)
		return
	}
	
	// 检查是否已存在，决定使用AddBlog还是ModifyBlog
	udb := &module.UploadedBlogData{
		Title:   title,
		Content: string(content),
		AuthType: module.EAuthType_private,
	}
	
	if _, exists := blog.Blogs[title]; exists {
		blog.ModifyBlog(udb)
	} else {
		blog.AddBlog(udb)
	}
}

func saveReadingRecord(record *module.ReadingRecord) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := Books[record.BookID]; exists {
		saveBook(book)
	}
}

func saveBookNotes(bookID string) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := Books[bookID]; exists {
		saveBook(book)
	}
}

func saveBookInsight(insight *module.BookInsight) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := Books[insight.BookID]; exists {
		saveBook(book)
	}
}

func loadBooks() {
	// 从blog系统加载书籍数据
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.Book != nil {
				Books[data.Book.ID] = data.Book
			}
		}
	}
	log.DebugF("加载书籍数量: %d", len(Books))
}

func loadReadingRecords() {
	// 从blog系统加载阅读记录
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingRecord *module.ReadingRecord `json:"reading_record"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.ReadingRecord != nil {
				ReadingRecords[data.ReadingRecord.BookID] = data.ReadingRecord
			}
		}
	}
	log.DebugF("加载阅读记录数量: %d", len(ReadingRecords))
}

func loadBookNotes() {
	// 从blog系统加载笔记
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookNotes []*module.BookNote `json:"book_notes"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				// 需要找到对应的bookID
				var bookData struct {
					Book *module.Book `json:"book"`
				}
				if err := json.Unmarshal([]byte(b.Content), &bookData); err == nil && bookData.Book != nil {
					BookNotes[bookData.Book.ID] = data.BookNotes
				}
			}
		}
	}
	log.DebugF("加载笔记数量: %d", getTotalNotesCount())
}

func loadBookInsights() {
	// 从blog系统加载感悟
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, insight := range data.BookInsights {
					BookInsights[insight.ID] = insight
				}
			}
		}
	}
	log.DebugF("加载感悟数量: %d", len(BookInsights))
}

// 新增功能实现

// 阅读计划管理
func AddReadingPlan(title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	if title == "" || startDate == "" || endDate == "" {
		return nil, errors.New("标题、开始日期和结束日期不能为空")
	}

	planID := generateID()
	plan := &module.ReadingPlan{
		ID:          planID,
		Title:       title,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		TargetBooks: targetBooks,
		Status:      "active",
		Progress:    0.0,
		CreateTime:  strTime(),
		UpdateTime:  strTime(),
	}

	ReadingPlans[planID] = plan
	saveReadingPlan(plan)
	log.DebugF("添加阅读计划成功: %s", title)
	return plan, nil
}

func GetReadingPlan(planID string) *module.ReadingPlan {
	return ReadingPlans[planID]
}

func GetAllReadingPlans() []*module.ReadingPlan {
	// 从blog系统获取阅读计划数据
	var plans []*module.ReadingPlan
	
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingPlans []*module.ReadingPlan `json:"reading_plans"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				plans = append(plans, data.ReadingPlans...)
			}
		}
	}
	
	// 如果blog中没有数据，则从原有Redis加载（兼容性）
	if len(plans) == 0 {
		plans = make([]*module.ReadingPlan, 0, len(ReadingPlans))
		for _, plan := range ReadingPlans {
			plans = append(plans, plan)
		}
		
		// 按创建时间倒序
		sort.Slice(plans, func(i, j int) bool {
			return plans[i].CreateTime > plans[j].CreateTime
		})
	}
	
	return plans
}

func UpdateReadingPlanProgress(planID string) error {
	plan, exists := ReadingPlans[planID]
	if !exists {
		return errors.New("阅读计划不存在")
	}

	if len(plan.TargetBooks) == 0 {
		plan.Progress = 0.0
		return nil
	}

	completedBooks := 0
	for _, bookID := range plan.TargetBooks {
		if record, exists := ReadingRecords[bookID]; exists {
			if record.Status == "finished" {
				completedBooks++
			}
		}
	}

	plan.Progress = float64(completedBooks) / float64(len(plan.TargetBooks)) * 100
	plan.UpdateTime = strTime()

	if plan.Progress >= 100.0 {
		plan.Status = "completed"
	}

	saveReadingPlan(plan)
	log.DebugF("更新阅读计划进度: %s - %.1f%%", planID, plan.Progress)
	return nil
}

// 阅读目标管理
func AddReadingGoal(year, month int, targetType string, targetValue int) (*module.ReadingGoal, error) {
	if year < 2000 || year > 3000 {
		return nil, errors.New("年份不合法")
	}
	if targetType == "" || targetValue <= 0 {
		return nil, errors.New("目标类型和目标值不能为空")
	}

	goalID := generateID()
	goal := &module.ReadingGoal{
		ID:           goalID,
		Year:         year,
		Month:        month,
		TargetType:   targetType,
		TargetValue:  targetValue,
		CurrentValue: 0,
		Status:       "active",
		CreateTime:   strTime(),
		UpdateTime:   strTime(),
	}

	ReadingGoals[goalID] = goal
	saveReadingGoal(goal)
	log.DebugF("添加阅读目标成功: %s - %d", targetType, targetValue)
	return goal, nil
}

func GetReadingGoals(year, month int) []*module.ReadingGoal {
	// 从blog系统获取阅读目标数据
	var goals []*module.ReadingGoal
	
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingGoals []*module.ReadingGoal `json:"reading_goals"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, goal := range data.ReadingGoals {
					if goal.Year == year && (month == 0 || goal.Month == month) {
						goals = append(goals, goal)
					}
				}
			}
		}
	}
	
	// 如果blog中没有数据，则从原有Redis加载（兼容性）
	if len(goals) == 0 {
		for _, goal := range ReadingGoals {
			if goal.Year == year && (month == 0 || goal.Month == month) {
				goals = append(goals, goal)
			}
		}
	}
	
	return goals
}

func UpdateReadingGoalProgress(goalID string) error {
	goal, exists := ReadingGoals[goalID]
	if !exists {
		return errors.New("阅读目标不存在")
	}

	currentValue := 0
	switch goal.TargetType {
	case "books":
		// 统计已完成的书籍数量
		for _, record := range ReadingRecords {
			if record.Status == "finished" {
				// 检查完成时间是否在目标时间范围内
				if isInTimeRange(record.EndDate, goal.Year, goal.Month) {
					currentValue++
				}
			}
		}
	case "pages":
		// 统计已阅读的页数
		for _, record := range ReadingRecords {
			if isInTimeRange(record.LastUpdateTime, goal.Year, goal.Month) {
				currentValue += record.CurrentPage
			}
		}
	case "time":
		// 统计阅读时间
		for _, record := range ReadingRecords {
			if isInTimeRange(record.LastUpdateTime, goal.Year, goal.Month) {
				currentValue += record.TotalReadingTime
			}
		}
	}

	goal.CurrentValue = currentValue
	goal.UpdateTime = strTime()

	if currentValue >= goal.TargetValue {
		goal.Status = "completed"
	}

	saveReadingGoal(goal)
	log.DebugF("更新阅读目标进度: %s - %d/%d", goalID, currentValue, goal.TargetValue)
	return nil
}

// 书籍推荐系统
func GenerateBookRecommendations(bookID string) ([]*module.BookRecommendation, error) {
	book := Books[bookID]
	if book == nil {
		return nil, errors.New("书籍不存在")
	}

	recommendations := make([]*module.BookRecommendation, 0)

	// 基于分类推荐
	for _, otherBook := range Books {
		if otherBook.ID == bookID {
			continue
		}

		// 检查分类相似性
		if hasCommonElements(book.Category, otherBook.Category) {
			score := calculateCategoryScore(book.Category, otherBook.Category)
			if score > 0.5 {
				rec := &module.BookRecommendation{
					ID:         generateID(),
					BookID:     otherBook.ID,
					Title:      otherBook.Title,
					Author:     otherBook.Author,
					Reason:     "基于分类相似性推荐",
					Score:      score,
					Tags:       otherBook.Tags,
					SourceType: "category",
					SourceID:   bookID,
					CreateTime: strTime(),
				}
				recommendations = append(recommendations, rec)
			}
		}

		// 基于作者推荐
		if otherBook.Author == book.Author {
			rec := &module.BookRecommendation{
				ID:         generateID(),
				BookID:     otherBook.ID,
				Title:      otherBook.Title,
				Author:     otherBook.Author,
				Reason:     "同作者作品推荐",
				Score:      0.8,
				Tags:       otherBook.Tags,
				SourceType: "author",
				SourceID:   bookID,
				CreateTime: strTime(),
			}
			recommendations = append(recommendations, rec)
		}
	}

	// 按分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// 限制推荐数量
	if len(recommendations) > 10 {
		recommendations = recommendations[:10]
	}

	return recommendations, nil
}

// 时间记录管理
func StartReadingSession(bookID string) (*module.ReadingTimeRecord, error) {
	if _, exists := Books[bookID]; !exists {
		return nil, errors.New("书籍不存在")
	}

	recordID := generateID()
	timeRecord := &module.ReadingTimeRecord{
		ID:         recordID,
		BookID:     bookID,
		StartTime:  strTime(),
		CreateTime: strTime(),
	}

	if ReadingTimeRecords[bookID] == nil {
		ReadingTimeRecords[bookID] = []*module.ReadingTimeRecord{}
	}
	ReadingTimeRecords[bookID] = append(ReadingTimeRecords[bookID], timeRecord)
	saveReadingTimeRecord(timeRecord)

	log.DebugF("开始阅读会话: %s", bookID)
	return timeRecord, nil
}

func EndReadingSession(recordID string, pages int, notes string) error {
	var targetRecord *module.ReadingTimeRecord
	var bookID string

	// 查找记录
	for bid, records := range ReadingTimeRecords {
		for _, record := range records {
			if record.ID == recordID {
				targetRecord = record
				bookID = bid
				break
			}
		}
		if targetRecord != nil {
			break
		}
	}

	if targetRecord == nil {
		return errors.New("阅读记录不存在")
	}

	if targetRecord.EndTime != "" {
		return errors.New("阅读会话已结束")
	}

	endTime := strTime()
	startTime, _ := time.Parse("2006-01-02 15:04:05", targetRecord.StartTime)
	endTimeParsed, _ := time.Parse("2006-01-02 15:04:05", endTime)
	duration := int(endTimeParsed.Sub(startTime).Minutes())

	targetRecord.EndTime = endTime
	targetRecord.Duration = duration
	targetRecord.Pages = pages
	targetRecord.Notes = notes
	saveReadingTimeRecord(targetRecord)

	// 更新总阅读时间
	if record, exists := ReadingRecords[bookID]; exists {
		record.TotalReadingTime += duration
		saveReadingRecord(record)
	}

	log.DebugF("结束阅读会话: %s - %d分钟", bookID, duration)
	return nil
}

// 书籍收藏夹管理
func AddBookCollection(name, description string, bookIDs []string, isPublic bool) (*module.BookCollection, error) {
	if name == "" {
		return nil, errors.New("收藏夹名称不能为空")
	}

	collectionID := generateID()
	collection := &module.BookCollection{
		ID:          collectionID,
		Name:        name,
		Description: description,
		BookIDs:     bookIDs,
		IsPublic:    isPublic,
		Tags:        []string{},
		CreateTime:  strTime(),
		UpdateTime:  strTime(),
	}

	BookCollections[collectionID] = collection
	saveBookCollection(collection)
	log.DebugF("添加收藏夹成功: %s", name)
	return collection, nil
}

func GetBookCollection(collectionID string) *module.BookCollection {
	return BookCollections[collectionID]
}

func GetAllBookCollections() []*module.BookCollection {
	// 从blog系统获取书籍收藏数据
	var collections []*module.BookCollection
	
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookCollections []*module.BookCollection `json:"book_collections"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				collections = append(collections, data.BookCollections...)
			}
		}
	}
	
	// 如果blog中没有数据，则从原有Redis加载（兼容性）
	if len(collections) == 0 {
		collections = make([]*module.BookCollection, 0, len(BookCollections))
		for _, collection := range BookCollections {
			collections = append(collections, collection)
		}
	}
	
	return collections
}

func AddBookToCollection(collectionID, bookID string) error {
	collection, exists := BookCollections[collectionID]
	if !exists {
		return errors.New("收藏夹不存在")
	}

	// 检查书籍是否已存在
	for _, id := range collection.BookIDs {
		if id == bookID {
			return errors.New("书籍已在收藏夹中")
		}
	}

	collection.BookIDs = append(collection.BookIDs, bookID)
	collection.UpdateTime = strTime()
	saveBookCollection(collection)

	log.DebugF("添加书籍到收藏夹: %s -> %s", bookID, collectionID)
	return nil
}

// 高级统计功能
func GetAdvancedReadingStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	
	// 基础统计
	basicStats := GetReadingStatistics()
	for k, v := range basicStats {
		stats[k] = v
	}
	
	// 月度统计
	monthlyStats := getMonthlyReadingStats()
	stats["monthly_stats"] = monthlyStats
	
	// 分类统计
	categoryStats := getCategoryStats()
	stats["category_stats"] = categoryStats
	
	// 作者统计
	authorStats := getAuthorStats()
	stats["author_stats"] = authorStats
	
	// 阅读时间统计
	timeStats := getReadingTimeStats()
	stats["time_stats"] = timeStats
	
	return stats
}

func ExportReadingData(config *module.ExportConfig) (string, error) {
	// 简化的导出实现
	data := make(map[string]interface{})
	
	// 导出书籍数据
	if len(config.BookIDs) > 0 {
		books := make([]*module.Book, 0)
		for _, bookID := range config.BookIDs {
			if book, exists := Books[bookID]; exists {
				books = append(books, book)
			}
		}
		data["books"] = books
	}
	
	// 导出笔记
	if config.IncludeNotes {
		notes := make(map[string][]*module.BookNote)
		for _, bookID := range config.BookIDs {
			if bookNotes, exists := BookNotes[bookID]; exists {
				notes[bookID] = bookNotes
			}
		}
		data["notes"] = notes
	}
	
	// 导出感悟
	if config.IncludeInsights {
		insights := make([]*module.BookInsight, 0)
		for _, insight := range BookInsights {
			for _, bookID := range config.BookIDs {
				if insight.BookID == bookID {
					insights = append(insights, insight)
					break
				}
			}
		}
		data["insights"] = insights
	}
	
	// 根据格式返回数据
	switch config.Format {
	case "json":
		return fmt.Sprintf("%+v", data), nil
	default:
		return "导出格式暂不支持", nil
	}
}

// 辅助函数
func hasCommonElements(a, b []string) bool {
	for _, itemA := range a {
		for _, itemB := range b {
			if itemA == itemB {
				return true
			}
		}
	}
	return false
}

func calculateCategoryScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	
	common := 0
	for _, itemA := range a {
		for _, itemB := range b {
			if itemA == itemB {
				common++
			}
		}
	}
	
	return float64(common) / float64(len(a)+len(b)-common)
}

func isInTimeRange(dateStr string, year, month int) bool {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	
	if date.Year() != year {
		return false
	}
	
	if month > 0 && int(date.Month()) != month {
		return false
	}
	
	return true
}

func getMonthlyReadingStats() []map[string]interface{} {
	// 简化实现，返回最近12个月的统计
	return []map[string]interface{}{}
}

func getCategoryStats() map[string]int {
	categoryCount := make(map[string]int)
	for _, book := range Books {
		for _, category := range book.Category {
			categoryCount[category]++
		}
	}
	return categoryCount
}

func getAuthorStats() map[string]int {
	authorCount := make(map[string]int)
	for _, book := range Books {
		authorCount[book.Author]++
	}
	return authorCount
}

func getReadingTimeStats() map[string]interface{} {
	totalTime := 0
	totalSessions := 0
	
	for _, records := range ReadingTimeRecords {
		for _, record := range records {
			if record.Duration > 0 {
				totalTime += record.Duration
				totalSessions++
			}
		}
	}
	
	avgTime := 0
	if totalSessions > 0 {
		avgTime = totalTime / totalSessions
	}
	
	return map[string]interface{}{
		"total_time":     totalTime,
		"total_sessions": totalSessions,
		"average_time":   avgTime,
	}
}

// 新的数据加载函数
func loadReadingPlans() {
	// 从blog系统加载阅读计划
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingPlans []*module.ReadingPlan `json:"reading_plans"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, plan := range data.ReadingPlans {
					ReadingPlans[plan.ID] = plan
				}
			}
		}
	}
	log.DebugF("加载阅读计划数量: %d", len(ReadingPlans))
}

func loadReadingGoals() {
	// 从blog系统加载阅读目标
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingGoals []*module.ReadingGoal `json:"reading_goals"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, goal := range data.ReadingGoals {
					ReadingGoals[goal.ID] = goal
				}
			}
		}
	}
	log.DebugF("加载阅读目标数量: %d", len(ReadingGoals))
}

func loadBookCollections() {
	// 从blog系统加载书籍收藏夹
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookCollections []*module.BookCollection `json:"book_collections"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, collection := range data.BookCollections {
					BookCollections[collection.ID] = collection
				}
			}
		}
	}
	log.DebugF("加载书籍收藏夹数量: %d", len(BookCollections))
}

func loadReadingTimeRecords() {
	// 从blog系统加载阅读时间记录
	for title, b := range blog.Blogs {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingTimeRecords map[string][]*module.ReadingTimeRecord `json:"reading_time_records"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for bookID, records := range data.ReadingTimeRecords {
					ReadingTimeRecords[bookID] = records
				}
			}
		}
	}
	totalRecords := 0
	for _, recordList := range ReadingTimeRecords {
		totalRecords += len(recordList)
	}
	log.DebugF("加载阅读时间记录数量: %d", totalRecords)
}

// 新的数据保存函数
func saveReadingPlan(plan *module.ReadingPlan) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	// 这里需要找到相关的书籍来保存
	for _, bookID := range plan.TargetBooks {
		if book, exists := Books[bookID]; exists {
			saveBook(book)
			break // 只需要保存一个相关书籍即可
		}
	}
}

func saveReadingGoal(goal *module.ReadingGoal) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	// 这里需要找到相关的书籍来保存
	for _, book := range Books {
		saveBook(book)
		break // 只需要保存一个书籍即可
	}
}

func saveBookCollection(collection *module.BookCollection) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	// 这里需要找到相关的书籍来保存
	for _, bookID := range collection.BookIDs {
		if book, exists := Books[bookID]; exists {
			saveBook(book)
			break // 只需要保存一个相关书籍即可
		}
	}
}

func saveReadingTimeRecord(record *module.ReadingTimeRecord) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := Books[record.BookID]; exists {
		saveBook(book)
	}
} 