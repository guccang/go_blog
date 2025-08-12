package reading

import (
	"core"
	"module"
	log "mylog"
)

// 读书模块actor
var reading_module *ReadingActor

func Info() {
	log.Debug("info reading v1.0")
}

// 初始化reading模块，用于书籍管理、阅读记录、笔记、心得等
func Init() {
	reading_module = &ReadingActor{
		Actor:               core.NewActor(),
		books:               make(map[string]*module.Book),
		readingRecords:      make(map[string]*module.ReadingRecord),
		bookNotes:           make(map[string][]*module.BookNote),
		bookInsights:        make(map[string]*module.BookInsight),
		readingPlans:        make(map[string]*module.ReadingPlan),
		readingGoals:        make(map[string]*module.ReadingGoal),
		bookRecommendations: make(map[string]*module.BookRecommendation),
		bookCollections:     make(map[string]*module.BookCollection),
		readingTimeRecords:  make(map[string][]*module.ReadingTimeRecord),
	}

	// 从数据库加载数据
	reading_module.loadBooks()
	reading_module.loadReadingRecords()
	reading_module.loadBookNotes()
	reading_module.loadBookInsights()
	reading_module.loadReadingPlans()
	reading_module.loadReadingGoals()
	reading_module.loadBookCollections()
	reading_module.loadReadingTimeRecords()

	reading_module.Start(reading_module)

	log.DebugF("Reading module initialized - Books: %d, Records: %d, Notes: %d, Insights: %d, Plans: %d, Goals: %d, Collections: %d",
		len(reading_module.books), len(reading_module.readingRecords), reading_module.getTotalNotesCount(), len(reading_module.bookInsights), len(reading_module.readingPlans), len(reading_module.readingGoals), len(reading_module.bookCollections))
}

// interface

func AddBook(title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	cmd := &AddBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title:       title,
		Author:      author,
		ISBN:        isbn,
		Publisher:   publisher,
		PublishDate: publishDate,
		CoverUrl:    coverUrl,
		Description: description,
		SourceUrl:   sourceUrl,
		TotalPages:  totalPages,
		Category:    category,
		Tags:        tags,
	}
	reading_module.Send(cmd)
	book := <-cmd.Response()
	err := <-cmd.Response()
	if book == nil {
		return nil, err.(error)
	}
	return book.(*module.Book), nil
}

func GetBook(bookID string) *module.Book {
	cmd := &GetBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	book := <-cmd.Response()
	if book == nil {
		return nil
	}
	return book.(*module.Book)
}

func GetAllBooks() map[string]*module.Book {
	cmd := &GetAllBooksCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	reading_module.Send(cmd)
	books := <-cmd.Response()
	return books.(map[string]*module.Book)
}

func UpdateBook(bookID string, updates map[string]interface{}) error {
	cmd := &UpdateBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID:  bookID,
		Updates: updates,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBook(bookID string) error {
	cmd := &DeleteBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func StartReading(bookID string) error {
	cmd := &StartReadingCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func UpdateReadingProgress(bookID string, currentPage int, notes string) error {
	cmd := &UpdateReadingProgressCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID:      bookID,
		CurrentPage: currentPage,
		Notes:       notes,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func GetReadingRecord(bookID string) *module.ReadingRecord {
	cmd := &GetReadingRecordCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	record := <-cmd.Response()
	if record == nil {
		return nil
	}
	return record.(*module.ReadingRecord)
}

func AddBookNote(bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	cmd := &AddBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID:   bookID,
		NoteType: noteType,
		Chapter:  chapter,
		Content:  content,
		Page:     page,
		Tags:     tags,
	}
	reading_module.Send(cmd)
	note := <-cmd.Response()
	err := <-cmd.Response()
	if note == nil {
		return nil, err.(error)
	}
	return note.(*module.BookNote), nil
}

func GetBookNotes(bookID string) []*module.BookNote {
	cmd := &GetBookNotesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	notes := <-cmd.Response()
	return notes.([]*module.BookNote)
}

func UpdateBookNote(bookID, noteID string, updates map[string]interface{}) error {
	cmd := &UpdateBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID:  bookID,
		NoteID:  noteID,
		Updates: updates,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBookNote(bookID, noteID string) error {
	cmd := &DeleteBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
		NoteID: noteID,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func AddBookInsight(bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	cmd := &AddBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID:       bookID,
		Title:        title,
		Content:      content,
		KeyTakeaways: keyTakeaways,
		Applications: applications,
		Rating:       rating,
		Tags:         tags,
	}
	reading_module.Send(cmd)
	insight := <-cmd.Response()
	err := <-cmd.Response()
	if insight == nil {
		return nil, err.(error)
	}
	return insight.(*module.BookInsight), nil
}

func GetBookInsights(bookID string) []*module.BookInsight {
	cmd := &GetBookInsightsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BookID: bookID,
	}
	reading_module.Send(cmd)
	insights := <-cmd.Response()
	return insights.([]*module.BookInsight)
}

func UpdateBookInsight(insightID string, updates map[string]interface{}) error {
	cmd := &UpdateBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		InsightID: insightID,
		Updates:   updates,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBookInsight(insightID string) error {
	cmd := &DeleteBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		InsightID: insightID,
	}
	reading_module.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func SearchBooks(keyword string) []*module.Book {
	cmd := &SearchBooksCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Keyword: keyword,
	}
	reading_module.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func FilterBooksByStatus(status string) []*module.Book {
	cmd := &FilterBooksByStatusCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Status: status,
	}
	reading_module.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func FilterBooksByCategory(category string) []*module.Book {
	cmd := &FilterBooksByCategoryCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Category: category,
	}
	reading_module.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func GetReadingStatistics() map[string]interface{} {
	cmd := &GetReadingStatisticsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	reading_module.Send(cmd)
	stats := <-cmd.Response()
	return stats.(map[string]interface{})
}

func AddReadingPlan(title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	cmd := &AddReadingPlanCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title:       title,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		TargetBooks: targetBooks,
	}
	reading_module.Send(cmd)
	plan := <-cmd.Response()
	err := <-cmd.Response()
	if plan == nil {
		return nil, err.(error)
	}
	return plan.(*module.ReadingPlan), nil
}

func GetReadingPlan(planID string) *module.ReadingPlan {
	cmd := &GetReadingPlanCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		PlanID: planID,
	}
	reading_module.Send(cmd)
	plan := <-cmd.Response()
	if plan == nil {
		return nil
	}
	return plan.(*module.ReadingPlan)
}

func GetAllReadingPlans() []*module.ReadingPlan {
	cmd := &GetAllReadingPlansCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	reading_module.Send(cmd)
	plans := <-cmd.Response()
	return plans.([]*module.ReadingPlan)
}

func UpdateReadingPlanProgress(planID string) error {
	// 这个函数需要在actor中实现，这里先返回nil以保证编译通过
	// TODO: 实现UpdateReadingPlanProgress的cmd和actor方法
	return nil
}

func AddReadingGoal(year, month int, targetType string, targetValue int) (*module.ReadingGoal, error) {
	// TODO: 实现AddReadingGoal的cmd和actor方法
	return nil, nil
}

func GetReadingGoals(year, month int) []*module.ReadingGoal {
	// TODO: 实现GetReadingGoals的cmd和actor方法
	return nil
}

func UpdateReadingGoalProgress(goalID string) error {
	// TODO: 实现UpdateReadingGoalProgress的cmd和actor方法
	return nil
}

func GenerateBookRecommendations(bookID string) ([]*module.BookRecommendation, error) {
	// TODO: 实现GenerateBookRecommendations的cmd和actor方法
	return nil, nil
}

func StartReadingSession(bookID string) (*module.ReadingTimeRecord, error) {
	// TODO: 实现StartReadingSession的cmd和actor方法
	return nil, nil
}

func EndReadingSession(recordID string, pages int, notes string) error {
	// TODO: 实现EndReadingSession的cmd和actor方法
	return nil
}

func AddBookCollection(name, description string, bookIDs []string, isPublic bool) (*module.BookCollection, error) {
	// TODO: 实现AddBookCollection的cmd和actor方法
	return nil, nil
}

func GetBookCollection(collectionID string) *module.BookCollection {
	// TODO: 实现GetBookCollection的cmd和actor方法
	return nil
}

func GetAllBookCollections() []*module.BookCollection {
	// TODO: 实现GetAllBookCollections的cmd和actor方法
	return nil
}

func AddBookToCollection(collectionID, bookID string) error {
	// TODO: 实现AddBookToCollection的cmd和actor方法
	return nil
}

func GetAdvancedReadingStatistics() map[string]interface{} {
	// TODO: 实现GetAdvancedReadingStatistics的cmd和actor方法
	return nil
}

func ExportReadingData(config *module.ExportConfig) (string, error) {
	// TODO: 实现ExportReadingData的cmd和actor方法
	return "", nil
}

// 兼容性函数，保持原有API不变
// 这些函数在原版本中可能被其他模块调用，为了保持兼容性而保留
