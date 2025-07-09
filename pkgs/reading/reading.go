package reading

import (
	"module"
	db "persistence"
	log "mylog"
	"time"
	"fmt"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"errors"
	"sort"
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
	return Books[bookID]
}

func GetAllBooks() map[string]*module.Book {
	return Books
}

func UpdateBook(bookID string, updates map[string]interface{}) error {
	book, exists := Books[bookID]
	if !exists {
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

	saveBook(book)
	log.DebugF("更新书籍成功: %s", bookID)
	return nil
}

func DeleteBook(bookID string) error {
	_, exists := Books[bookID]
	if !exists {
		return errors.New("书籍不存在")
	}

	// 删除相关数据
	delete(Books, bookID)
	delete(ReadingRecords, bookID)
	delete(BookNotes, bookID)
	
	// 删除所有相关的心得（从内存中删除）
	for insightID, insight := range BookInsights {
		if insight.BookID == bookID {
			delete(BookInsights, insightID)
		}
	}

	// 从数据库删除
	db.DeleteBook(fmt.Sprintf("book@%s", bookID))
	db.DeleteBook(fmt.Sprintf("reading_record@%s", bookID))
	db.DeleteBook(fmt.Sprintf("book_notes@%s", bookID))
	
	// 删除所有相关的心得（需要遍历删除，因为心得的key是用心得ID，不是bookID）
	for insightID, insight := range BookInsights {
		if insight.BookID == bookID {
			db.DeleteBook(fmt.Sprintf("book_insight@%s", insightID))
		}
	}

	log.DebugF("删除书籍成功: %s", bookID)
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
	notes := BookNotes[bookID]
	if notes == nil {
		return []*module.BookNote{}
	}
	
	// 按创建时间排序
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreateTime > notes[j].CreateTime
	})
	
	return notes
}

func UpdateBookNote(bookID, noteID string, updates map[string]interface{}) error {
	notes := BookNotes[bookID]
	if notes == nil {
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

	saveBookNotes(bookID)
	log.DebugF("更新笔记成功: %s", noteID)
	return nil
}

// 删除笔记
func DeleteBookNote(bookID, noteID string) error {
	notes := BookNotes[bookID]
	if notes == nil {
		return errors.New("笔记不存在")
	}

	// 查找并删除笔记
	for i, note := range notes {
		if note.ID == noteID {
			// 从切片中删除元素
			BookNotes[bookID] = append(notes[:i], notes[i+1:]...)
			saveBookNotes(bookID)
			log.DebugF("删除笔记成功: %s", noteID)
			return nil
		}
	}

	return errors.New("笔记不存在")
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
	var insights []*module.BookInsight
	for _, insight := range BookInsights {
		if insight.BookID == bookID {
			insights = append(insights, insight)
		}
	}
	
	// 按创建时间排序
	sort.Slice(insights, func(i, j int) bool {
		return insights[i].CreateTime > insights[j].CreateTime
	})
	
	return insights
}

// 更新心得
func UpdateBookInsight(insightID string, updates map[string]interface{}) error {
	insight, exists := BookInsights[insightID]
	if !exists {
		return errors.New("心得不存在")
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
		if book := Books[insight.BookID]; book != nil && rating > 0 {
			book.Rating = float64(rating)
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

	saveBookInsight(insight)
	log.DebugF("更新心得成功: %s", insightID)
	return nil
}

// 删除心得
func DeleteBookInsight(insightID string) error {
	_, exists := BookInsights[insightID]
	if !exists {
		return errors.New("心得不存在")
	}

	// 从内存中删除
	delete(BookInsights, insightID)
	
	// 从数据库删除
	db.DeleteBook(fmt.Sprintf("book_insight@%s", insightID))
	
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
	db.SaveBook(book)
}

func saveReadingRecord(record *module.ReadingRecord) {
	db.SaveReadingRecord(record)
}

func saveBookNotes(bookID string) {
	notes := BookNotes[bookID]
	db.SaveBookNotes(bookID, notes)
}

func saveBookInsight(insight *module.BookInsight) {
	db.SaveBookInsight(insight)
}

func loadBooks() {
	// 从数据库加载书籍数据
	books := db.GetAllBooks()
	if books != nil {
		for _, book := range books {
			Books[book.ID] = book
		}
	}
	log.DebugF("加载书籍数量: %d", len(Books))
}

func loadReadingRecords() {
	// 从数据库加载阅读记录
	records := db.GetAllReadingRecords()
	if records != nil {
		for _, record := range records {
			ReadingRecords[record.BookID] = record
		}
	}
	log.DebugF("加载阅读记录数量: %d", len(ReadingRecords))
}

func loadBookNotes() {
	// 从数据库加载笔记
	allNotes := db.GetAllBookNotes()
	if allNotes != nil {
		for bookID, notes := range allNotes {
			BookNotes[bookID] = notes
		}
	}
	log.DebugF("加载笔记数量: %d", getTotalNotesCount())
}

func loadBookInsights() {
	// 从数据库加载感悟
	insights := db.GetAllBookInsights()
	if insights != nil {
		for _, insight := range insights {
			BookInsights[insight.ID] = insight
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
	plans := make([]*module.ReadingPlan, 0, len(ReadingPlans))
	for _, plan := range ReadingPlans {
		plans = append(plans, plan)
	}
	
	// 按创建时间倒序
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreateTime > plans[j].CreateTime
	})
	
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
	goals := make([]*module.ReadingGoal, 0)
	for _, goal := range ReadingGoals {
		if goal.Year == year && (month == 0 || goal.Month == month) {
			goals = append(goals, goal)
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
	collections := make([]*module.BookCollection, 0, len(BookCollections))
	for _, collection := range BookCollections {
		collections = append(collections, collection)
	}
	
	sort.Slice(collections, func(i, j int) bool {
		return collections[i].CreateTime > collections[j].CreateTime
	})
	
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
	plans := db.GetAllReadingPlans()
	if plans != nil {
		for _, plan := range plans {
			ReadingPlans[plan.ID] = plan
		}
	}
	log.DebugF("加载阅读计划数量: %d", len(ReadingPlans))
}

func loadReadingGoals() {
	goals := db.GetAllReadingGoals()
	if goals != nil {
		for _, goal := range goals {
			ReadingGoals[goal.ID] = goal
		}
	}
	log.DebugF("加载阅读目标数量: %d", len(ReadingGoals))
}

func loadBookCollections() {
	collections := db.GetAllBookCollections()
	if collections != nil {
		for _, collection := range collections {
			BookCollections[collection.ID] = collection
		}
	}
	log.DebugF("加载书籍收藏夹数量: %d", len(BookCollections))
}

func loadReadingTimeRecords() {
	records := db.GetAllReadingTimeRecords()
	if records != nil {
		ReadingTimeRecords = records
	}
	totalRecords := 0
	for _, recordList := range ReadingTimeRecords {
		totalRecords += len(recordList)
	}
	log.DebugF("加载阅读时间记录数量: %d", totalRecords)
}

// 新的数据保存函数
func saveReadingPlan(plan *module.ReadingPlan) {
	db.SaveReadingPlan(plan)
}

func saveReadingGoal(goal *module.ReadingGoal) {
	db.SaveReadingGoal(goal)
}

func saveBookCollection(collection *module.BookCollection) {
	db.SaveBookCollection(collection)
}

func saveReadingTimeRecord(record *module.ReadingTimeRecord) {
	db.SaveReadingTimeRecord(record)
} 