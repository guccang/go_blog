package reading

import (
	"blog"
	"core"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"time"
)

/*
goroutine 线程安全
 goroutine 会被调度到任意一个线程上，因此会被任意一个线程执行接口
 线程安全原因
 原因1: 	actor使用chan通信，chan是线程安全的
 原因2: 	actor的mailbox是线程安全的

 添加一个功能需要的四个步骤:
  第一步: 实现功能逻辑
  第二步: 实现对应的cmd
  第三步: 在reading.go中添加对应的接口
  第四步: 在http中添加对应的接口

  上述精炼步骤产生过程:
  1. claudecode 实现版本
  2. 手写实现版本
  3. cursor+gpt5实现版本
  4. 最终综合上述不同实现版本的优点，有了上述的实现步骤.
  5. 最终实现版本 基于cmd的可撤回的actor并发模型,依赖于go的interface特性,简化了实现方式，非常特别的体验
*/

// actor
type ReadingActor struct {
	*core.Actor
	Account             string
	books               map[string]*module.Book
	readingRecords      map[string]*module.ReadingRecord
	bookNotes           map[string][]*module.BookNote
	bookInsights        map[string]*module.BookInsight
	readingPlans        map[string]*module.ReadingPlan
	readingGoals        map[string]*module.ReadingGoal
	bookRecommendations map[string]*module.BookRecommendation
	bookCollections     map[string]*module.BookCollection
	readingTimeRecords  map[string][]*module.ReadingTimeRecord
}

// 生成唯一ID
func (ar *ReadingActor) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%s", hex.EncodeToString(bytes)[:16])
}

// 获取当前时间字符串
func (ar *ReadingActor) strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// 书籍管理功能
func (ar *ReadingActor) addBook(title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	if title == "" || author == "" {
		return nil, errors.New("书名和作者不能为空")
	}

	// 检查是否已存在相同书籍
	for _, book := range ar.books {
		if book.Title == title && book.Author == author {
			return nil, errors.New("该书籍已存在")
		}
	}

	bookID := ar.generateID()
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
		AddTime:     ar.strTime(),
		Rating:      0,
		Status:      "unstart",
	}

	ar.books[bookID] = book

	// 创建初始阅读记录
	record := &module.ReadingRecord{
		BookID:           bookID,
		Status:           "unstart",
		StartDate:        "",
		EndDate:          "",
		CurrentPage:      0,
		TotalReadingTime: 0,
		ReadingSessions:  []module.ReadingSession{},
		LastUpdateTime:   ar.strTime(),
	}
	ar.readingRecords[bookID] = record

	// 保存到数据库
	ar.saveBook(ar.Account, book)
	ar.saveReadingRecord(ar.Account, record)

	log.DebugF(log.ModuleReading, "添加书籍成功: %s - %s", title, author)
	return book, nil
}

func (ar *ReadingActor) getBook(bookID string) *module.Book {
	// 从blog系统获取单本书数据
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
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
	return ar.books[bookID]
}

func (ar *ReadingActor) getAllBooks() map[string]*module.Book {
	// 从blog系统获取reading数据
	books := make(map[string]*module.Book)

	// 遍历所有blog，查找reading_book_开头的
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
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
		return ar.books
	}

	return books
}

func (ar *ReadingActor) updateBook(bookID string, updates map[string]interface{}) error {
	// 先从blog系统获取书籍信息，确保我们有最新的数据
	book := ar.getBook(bookID)
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
	ar.books[bookID] = book
	ar.saveBook(ar.Account, book)
	log.DebugF(log.ModuleReading, "更新书籍成功: %s", bookID)
	return nil
}

func (ar *ReadingActor) deleteBook(bookID string) error {
	// 先从blog系统获取书籍信息，确保我们有完整的数据
	book := ar.getBook(bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}

	// 构建blog标题用于删除
	blogTitle := fmt.Sprintf("reading_book_%s.md", book.Title)

	log.DebugF(log.ModuleReading, "准备删除书籍blog: %s (书籍ID: %s)", blogTitle, bookID)

	// 首先检查blog是否存在
	existingBlog := blog.GetBlogWithAccount(ar.Account, blogTitle)
	if existingBlog == nil {
		log.ErrorF(log.ModuleReading, "要删除的blog不存在: %s，可能已经被手动删除", blogTitle)
		// 如果blog不存在，直接删除内存数据即可
	} else {
		log.DebugF(log.ModuleReading, "找到要删除的blog: %s", blogTitle)

		// 从blog系统删除 - 这是关键步骤
		result := blog.DeleteBlogWithAccount(ar.Account, blogTitle)
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
			log.ErrorF(log.ModuleReading, "从blog系统删除书籍失败: %s, %s", blogTitle, errorMsg)
			return fmt.Errorf("删除书籍失败：%s", errorMsg)
		}

		log.DebugF(log.ModuleReading, "从blog系统删除书籍成功: %s", blogTitle)
	}

	// 删除内存中的相关数据
	delete(ar.books, bookID)
	delete(ar.readingRecords, bookID)
	delete(ar.bookNotes, bookID)

	// 删除所有相关的心得（从内存中删除）
	deletedInsights := 0
	for insightID, insight := range ar.bookInsights {
		if insight.BookID == bookID {
			delete(ar.bookInsights, insightID)
			deletedInsights++
		}
	}

	log.DebugF(log.ModuleReading, "删除书籍成功: %s - %s (同时删除了 %d 条心得)", bookID, book.Title, deletedInsights)
	return nil
}

// 阅读记录功能
func (ar *ReadingActor) startReading(account, bookID string) error {
	record, exists := ar.readingRecords[bookID]
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
	record.LastUpdateTime = ar.strTime()

	// 更新书籍状态
	if book, exists := ar.books[bookID]; exists {
		book.Status = "reading"
		ar.saveBook(account, book)
	}

	ar.saveReadingRecord(account, record)
	log.DebugF(log.ModuleReading, "开始阅读: %s", bookID)
	return nil
}

func (ar *ReadingActor) updateReadingProgress(account, bookID string, currentPage int, notes string) error {
	record, exists := ar.readingRecords[bookID]
	if !exists {
		return errors.New("阅读记录不存在")
	}

	book := ar.books[bookID]
	if book == nil {
		return errors.New("书籍不存在")
	}

	oldPage := record.CurrentPage
	record.CurrentPage = currentPage
	record.LastUpdateTime = ar.strTime()

	// 同步更新Book结构体的CurrentPage
	book.CurrentPage = currentPage
	ar.saveBook(account, book)

	// 如果是首次更新进度，自动开始阅读
	if record.Status == "unstart" {
		ar.startReading(account, bookID)
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
		ar.saveBook(account, book)
	}

	ar.saveReadingRecord(account, record)
	log.DebugF(log.ModuleReading, "更新阅读进度: %s - 第%d页", bookID, currentPage)
	return nil
}

func (ar *ReadingActor) getReadingRecord(account, bookID string) *module.ReadingRecord {
	// 从blog系统获取阅读记录
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
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
	return ar.readingRecords[bookID]
}

// 笔记功能
func (ar *ReadingActor) addBookNote(account, bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	if content == "" {
		return nil, errors.New("笔记内容不能为空")
	}

	noteID := ar.generateID()
	note := &module.BookNote{
		ID:         noteID,
		BookID:     bookID,
		Type:       noteType,
		Chapter:    chapter,
		Page:       page,
		Content:    content,
		Tags:       tags,
		CreateTime: ar.strTime(),
		UpdateTime: ar.strTime(),
	}

	if ar.bookNotes[bookID] == nil {
		ar.bookNotes[bookID] = []*module.BookNote{}
	}
	ar.bookNotes[bookID] = append(ar.bookNotes[bookID], note)

	ar.saveBookNotes(account, bookID)
	log.DebugF(log.ModuleReading, "添加笔记成功: %s - %s", bookID, noteType)
	return note, nil
}

func (ar *ReadingActor) getBookNotes(account, bookID string) []*module.BookNote {
	// 从blog系统获取笔记数据
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book      *module.Book       `json:"book"`
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
	notes := ar.bookNotes[bookID]
	if notes == nil {
		return []*module.BookNote{}
	}
	return notes
}

func (ar *ReadingActor) updateBookNote(account, bookID, noteID string, updates map[string]interface{}) error {
	// 先从blog系统加载最新数据
	notes := ar.getBookNotes(account, bookID)
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
	targetNote.UpdateTime = ar.strTime()

	// 更新内存中的数据
	ar.bookNotes[bookID] = notes
	ar.saveBookNotes(account, bookID)
	log.DebugF(log.ModuleReading, "更新笔记成功: %s", noteID)
	return nil
}

func (ar *ReadingActor) deleteBookNote(account, bookID, noteID string) error {
	// 先从blog系统加载最新数据
	notes := ar.getBookNotes(account, bookID)
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
	ar.bookNotes[bookID] = updatedNotes
	ar.saveBookNotes(account, bookID)
	log.DebugF(log.ModuleReading, "删除笔记成功: %s", noteID)
	return nil
}

// 读书感悟功能
func (ar *ReadingActor) addBookInsight(account, bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	if title == "" || content == "" {
		return nil, errors.New("标题和内容不能为空")
	}

	insightID := ar.generateID()
	insight := &module.BookInsight{
		ID:           insightID,
		BookID:       bookID,
		Title:        title,
		Content:      content,
		KeyTakeaways: keyTakeaways,
		Applications: applications,
		Rating:       rating,
		Tags:         tags,
		CreateTime:   ar.strTime(),
		UpdateTime:   ar.strTime(),
	}

	ar.bookInsights[insightID] = insight

	// 更新书籍评分
	if book := ar.books[bookID]; book != nil && rating > 0 {
		book.Rating = float64(rating)
		ar.saveBook(account, book)
	}

	ar.saveBookInsight(account, insight)
	log.DebugF(log.ModuleReading, "添加读书感悟成功: %s - %s", bookID, title)
	return insight, nil
}

func (ar *ReadingActor) getBookInsights(account, bookID string) []*module.BookInsight {
	// 从blog系统聚合指定书籍的全部心得
	var aggregated []*module.BookInsight
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, ins := range data.BookInsights {
					if ins.BookID == bookID {
						aggregated = append(aggregated, ins)
					}
				}
			}
		}
	}
	if len(aggregated) > 0 {
		return aggregated
	}

	// 如果blog中没有找到，则从内存（兼容存量数据）加载
	for _, ins := range ar.bookInsights {
		if ins.BookID == bookID {
			aggregated = append(aggregated, ins)
		}
	}
	if len(aggregated) == 0 {
		return []*module.BookInsight{}
	}
	return aggregated
}

func (ar *ReadingActor) updateBookInsight(account, insightID string, updates map[string]interface{}) error {
	// 先从blog系统查找心得
	var insight *module.BookInsight
	var bookID string

	// 遍历所有书籍数据查找指定的心得
	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
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
		insight, exists = ar.bookInsights[insightID]
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
		if book := ar.getBook(bookID); book != nil && rating > 0 {
			book.Rating = float64(rating)
			ar.books[bookID] = book
			ar.saveBook(account, book)
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
	insight.UpdateTime = ar.strTime()

	// 更新内存中的数据
	ar.bookInsights[insightID] = insight
	ar.saveBookInsight(account, insight)
	log.DebugF(log.ModuleReading, "更新心得成功: %s", insightID)
	return nil
}

func (ar *ReadingActor) deleteBookInsight(account, insightID string) error {
	// 先查找心得并获取bookID
	var foundInsight *module.BookInsight
	var targetBookID string

	// 从blog系统查找心得
	for title, b := range blog.GetBlogsWithAccount(account) {
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
		foundInsight, exists = ar.bookInsights[insightID]
		if !exists {
			return errors.New("心得不存在")
		}
		targetBookID = foundInsight.BookID
	}

	// 从内存中删除
	delete(ar.bookInsights, insightID)

	// 通过saveBook保存更新后的数据
	if book := ar.getBook(targetBookID); book != nil {
		ar.books[targetBookID] = book
		ar.saveBook(account, book)
	}

	log.DebugF(log.ModuleReading, "删除心得成功: %s", insightID)
	return nil
}

// 搜索和筛选功能
func (ar *ReadingActor) searchBooks(keyword string) []*module.Book {
	var results []*module.Book
	keyword = strings.ToLower(keyword)

	for _, book := range ar.books {
		if strings.Contains(strings.ToLower(book.Title), keyword) ||
			strings.Contains(strings.ToLower(book.Author), keyword) ||
			strings.Contains(strings.ToLower(book.Description), keyword) {
			results = append(results, book)
		}
	}

	return results
}

func (ar *ReadingActor) filterBooksByStatus(status string) []*module.Book {
	var results []*module.Book
	for _, book := range ar.books {
		if book.Status == status {
			results = append(results, book)
		}
	}
	return results
}

func (ar *ReadingActor) filterBooksByCategory(category string) []*module.Book {
	var results []*module.Book
	for _, book := range ar.books {
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
func (ar *ReadingActor) getReadingStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	totalBooks := len(ar.books)
	readingBooks := len(ar.filterBooksByStatus("reading"))
	finishedBooks := len(ar.filterBooksByStatus("finished"))
	unstartBooks := len(ar.filterBooksByStatus("unstart"))

	totalPages := 0
	totalReadingTime := 0
	for _, record := range ar.readingRecords {
		if book := ar.books[record.BookID]; book != nil {
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
	stats["total_notes"] = ar.getTotalNotesCount()
	stats["total_insights"] = len(ar.bookInsights)

	return stats
}

// 辅助函数
func (ar *ReadingActor) getTotalNotesCount() int {
	count := 0
	for _, notes := range ar.bookNotes {
		count += len(notes)
	}
	return count
}

// 数据持久化函数
func (ar *ReadingActor) saveBook(account string, book *module.Book) {
	// 将书籍数据保存到blog系统
	title := fmt.Sprintf("reading_book_%s.md", book.Title)

	// 构建完整的书籍数据（包括相关记录）
	data := map[string]interface{}{
		"book": book,
	}

	// 添加阅读记录
	if record, exists := ar.readingRecords[book.ID]; exists {
		data["reading_record"] = record
	}

	// 添加笔记
	if notes, exists := ar.bookNotes[book.ID]; exists {
		data["book_notes"] = notes
	}

	// 添加心得
	var insights []*module.BookInsight
	for _, insight := range ar.bookInsights {
		if insight.BookID == book.ID {
			insights = append(insights, insight)
		}
	}
	if len(insights) > 0 {
		data["book_insights"] = insights
	}

	// 添加阅读计划（包含该书籍的计划）
	var plans []*module.ReadingPlan
	for _, plan := range ar.readingPlans {
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
	for _, goal := range ar.readingGoals {
		goals = append(goals, goal)
	}
	if len(goals) > 0 {
		data["reading_goals"] = goals
	}

	// 添加书籍收藏夹（包含该书籍的收藏夹）
	var collections []*module.BookCollection
	for _, collection := range ar.bookCollections {
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
	if records, exists := ar.readingTimeRecords[book.ID]; exists {
		data["reading_time_records"] = map[string][]*module.ReadingTimeRecord{
			book.ID: records,
		}
	}

	// 序列化为JSON
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.ErrorF(log.ModuleReading, "序列化书籍数据失败: %v", err)
		return
	}

	// 检查是否已存在，决定使用AddBlog还是ModifyBlog
	udb := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Account:  account,
	}

	if _, exists := blog.GetBlogsWithAccount(account)[title]; exists {
		blog.ModifyBlogWithAccount(account, udb)
	} else {
		blog.AddBlogWithAccount(account, udb)
	}
}

func (ar *ReadingActor) saveReadingRecord(account string, record *module.ReadingRecord) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := ar.books[record.BookID]; exists {
		ar.saveBook(account, book)
	}
}

func (ar *ReadingActor) saveBookNotes(account, bookID string) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := ar.books[bookID]; exists {
		ar.saveBook(account, book)
	}
}

func (ar *ReadingActor) saveBookInsight(account string, insight *module.BookInsight) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	if book, exists := ar.books[insight.BookID]; exists {
		ar.saveBook(account, book)
		return
	}

	// 如果内存中未缓存对应书籍，尝试获取后再保存，避免心得无法持久化
	if book := ar.getBook(insight.BookID); book != nil {
		ar.books[insight.BookID] = book
		ar.saveBook(account, book)
		return
	}

	log.ErrorF(log.ModuleReading, "保存心得失败：未找到书籍，book_id=%s", insight.BookID)
}

func (ar *ReadingActor) loadBooks() {
	// 使用账户特定的加载方法
	ar.loadBooksForAccount(ar.Account)
}

func (ar *ReadingActor) loadReadingRecords() {
	// 使用账户特定的加载方法
	ar.loadReadingRecordsForAccount(ar.Account)
}

func (ar *ReadingActor) loadBookNotes() {
	// 使用账户特定的加载方法
	ar.loadBookNotesForAccount(ar.Account)
}

func (ar *ReadingActor) loadBookInsights() {
	// 使用账户特定的加载方法
	ar.loadBookInsightsForAccount(ar.Account)
}

// 加载其他数据的函数
func (ar *ReadingActor) loadReadingPlans() {
	// 使用账户特定的加载方法
	ar.loadReadingPlansForAccount(ar.Account)
}

func (ar *ReadingActor) loadReadingGoals() {
	// 使用账户特定的加载方法
	ar.loadReadingGoalsForAccount(ar.Account)
}

func (ar *ReadingActor) loadBookCollections() {
	// 使用账户特定的加载方法
	ar.loadBookCollectionsForAccount(ar.Account)
}

func (ar *ReadingActor) loadReadingTimeRecords() {
	// 使用账户特定的加载方法
	ar.loadReadingTimeRecordsForAccount(ar.Account)
}

// 账户特定的数据加载方法

func (ar *ReadingActor) loadBooksForAccount(account string) {
	// 从blog系统加载书籍数据 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				Book *module.Book `json:"book"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.Book != nil {
				ar.books[data.Book.ID] = data.Book
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的书籍数量: %d", account, len(ar.books))
}

func (ar *ReadingActor) loadReadingRecordsForAccount(account string) {
	// 从blog系统加载阅读记录 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingRecord *module.ReadingRecord `json:"reading_record"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil && data.ReadingRecord != nil {
				ar.readingRecords[data.ReadingRecord.BookID] = data.ReadingRecord
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的阅读记录数量: %d", account, len(ar.readingRecords))
}

func (ar *ReadingActor) loadBookNotesForAccount(account string) {
	// 从blog系统加载笔记 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
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
					ar.bookNotes[bookData.Book.ID] = data.BookNotes
				}
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的笔记数量: %d", account, ar.getTotalNotesCount())
}

func (ar *ReadingActor) loadBookInsightsForAccount(account string) {
	// 从blog系统加载感悟 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookInsights []*module.BookInsight `json:"book_insights"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, insight := range data.BookInsights {
					ar.bookInsights[insight.ID] = insight
				}
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的感悟数量: %d", account, len(ar.bookInsights))
}

func (ar *ReadingActor) loadReadingPlansForAccount(account string) {
	// 从blog系统加载阅读计划 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingPlans []*module.ReadingPlan `json:"reading_plans"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, plan := range data.ReadingPlans {
					ar.readingPlans[plan.ID] = plan
				}
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的阅读计划数量: %d", account, len(ar.readingPlans))
}

func (ar *ReadingActor) loadReadingGoalsForAccount(account string) {
	// 从blog系统加载阅读目标 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingGoals []*module.ReadingGoal `json:"reading_goals"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, goal := range data.ReadingGoals {
					ar.readingGoals[goal.ID] = goal
				}
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的阅读目标数量: %d", account, len(ar.readingGoals))
}

func (ar *ReadingActor) loadBookCollectionsForAccount(account string) {
	// 从blog系统加载书籍收藏夹 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				BookCollections []*module.BookCollection `json:"book_collections"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for _, collection := range data.BookCollections {
					ar.bookCollections[collection.ID] = collection
				}
			}
		}
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的书籍收藏夹数量: %d", account, len(ar.bookCollections))
}

func (ar *ReadingActor) loadReadingTimeRecordsForAccount(account string) {
	// 从blog系统加载阅读时间记录 - 指定账户
	for title, b := range blog.GetBlogsWithAccount(account) {
		if strings.HasPrefix(title, "reading_book_") {
			var data struct {
				ReadingTimeRecords map[string][]*module.ReadingTimeRecord `json:"reading_time_records"`
			}
			if err := json.Unmarshal([]byte(b.Content), &data); err == nil {
				for bookID, records := range data.ReadingTimeRecords {
					ar.readingTimeRecords[bookID] = records
				}
			}
		}
	}
	totalRecords := 0
	for _, recordList := range ar.readingTimeRecords {
		totalRecords += len(recordList)
	}
	log.DebugF(log.ModuleReading, "加载账户 %s 的阅读时间记录数量: %d", account, totalRecords)
}

// 其他功能实现，这里只列出关键的几个，其他可按需添加

// 阅读计划管理
func (ar *ReadingActor) addReadingPlan(title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	if title == "" || startDate == "" || endDate == "" {
		return nil, errors.New("标题、开始日期和结束日期不能为空")
	}

	planID := ar.generateID()
	plan := &module.ReadingPlan{
		ID:          planID,
		Title:       title,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		TargetBooks: targetBooks,
		Status:      "active",
		Progress:    0.0,
		CreateTime:  ar.strTime(),
		UpdateTime:  ar.strTime(),
	}

	ar.readingPlans[planID] = plan
	ar.saveReadingPlan(plan)
	log.DebugF(log.ModuleReading, "添加阅读计划成功: %s", title)
	return plan, nil
}

func (ar *ReadingActor) getReadingPlan(planID string) *module.ReadingPlan {
	return ar.readingPlans[planID]
}

func (ar *ReadingActor) getAllReadingPlans() []*module.ReadingPlan {
	// 从blog系统获取阅读计划数据
	var plans []*module.ReadingPlan

	for title, b := range blog.GetBlogsWithAccount(ar.Account) {
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
		plans = make([]*module.ReadingPlan, 0, len(ar.readingPlans))
		for _, plan := range ar.readingPlans {
			plans = append(plans, plan)
		}

		// 按创建时间倒序
		sort.Slice(plans, func(i, j int) bool {
			return plans[i].CreateTime > plans[j].CreateTime
		})
	}

	return plans
}

func (ar *ReadingActor) saveReadingPlan(plan *module.ReadingPlan) {
	// 通过saveBook函数保存，因为它会保存完整的书籍数据
	// 这里需要找到相关的书籍来保存
	for _, bookID := range plan.TargetBooks {
		if book, exists := ar.books[bookID]; exists {
			ar.saveBook(ar.Account, book)
			break // 只需要保存一个相关书籍即可
		}
	}
}
