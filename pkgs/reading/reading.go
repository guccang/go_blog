package reading

import (
	"core"
	"module"
	log "mylog"
)

func Info() {
	log.Debug("info reading v1.0")
}

// 初始化reading模块，用于书籍管理、阅读记录、笔记、心得等
func Init() {
	log.Debug("reading module Init")
}

// interface

func AddBookWithAccount(account, title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	actor := getReadingActor(account)
	cmd := &AddBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
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
	actor.Send(cmd)
	book := <-cmd.Response()
	err := <-cmd.Response()
	if book == nil {
		return nil, err.(error)
	}
	return book.(*module.Book), nil
}

func GetBookWithAccount(account, bookID string) *module.Book {
	actor := getReadingActor(account)
	cmd := &GetBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	book := <-cmd.Response()
	if book == nil {
		return nil
	}
	return book.(*module.Book)
}

func GetAllBooksWithAccount(account string) map[string]*module.Book {
	actor := getReadingActor(account)
	cmd := &GetAllBooksCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	actor.Send(cmd)
	books := <-cmd.Response()
	return books.(map[string]*module.Book)
}

func UpdateBookWithAccount(account, bookID string, updates map[string]interface{}) error {
	actor := getReadingActor(account)
	cmd := &UpdateBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
		Updates: updates,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBookWithAccount(account, bookID string) error {
	actor := getReadingActor(account)
	cmd := &DeleteBookCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func StartReadingWithAccount(account, bookID string) error {
	actor := getReadingActor(account)
	cmd := &StartReadingCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func UpdateReadingProgressWithAccount(account, bookID string, currentPage int, notes string) error {
	actor := getReadingActor(account)
	cmd := &UpdateReadingProgressCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
		BookID:      bookID,
		CurrentPage: currentPage,
		Notes:       notes,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func GetReadingRecordWithAccount(account, bookID string) *module.ReadingRecord {
	actor := getReadingActor(account)
	cmd := &GetReadingRecordCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	record := <-cmd.Response()
	if record == nil {
		return nil
	}
	return record.(*module.ReadingRecord)
}

func AddBookNoteWithAccount(account, bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	actor := getReadingActor(account)
	cmd := &AddBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		BookID:   bookID,
		NoteType: noteType,
		Chapter:  chapter,
		Content:  content,
		Page:     page,
		Tags:     tags,
	}
	actor.Send(cmd)
	note := <-cmd.Response()
	err := <-cmd.Response()
	if note == nil {
		return nil, err.(error)
	}
	return note.(*module.BookNote), nil
}

func GetBookNotesWithAccount(account, bookID string) []*module.BookNote {
	actor := getReadingActor(account)
	cmd := &GetBookNotesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	notes := <-cmd.Response()
	return notes.([]*module.BookNote)
}

func UpdateBookNoteWithAccount(account, bookID, noteID string, updates map[string]interface{}) error {
	actor := getReadingActor(account)
	cmd := &UpdateBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
		NoteID:  noteID,
		Updates: updates,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBookNoteWithAccount(account, bookID, noteID string) error {
	actor := getReadingActor(account)
	cmd := &DeleteBookNoteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
		NoteID:  noteID,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func AddBookInsightWithAccount(account, bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	actor := getReadingActor(account)
	cmd := &AddBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		BookID:       bookID,
		Title:        title,
		Content:      content,
		KeyTakeaways: keyTakeaways,
		Applications: applications,
		Rating:       rating,
		Tags:         tags,
	}
	actor.Send(cmd)
	insight := <-cmd.Response()
	err := <-cmd.Response()
	if insight == nil {
		return nil, err.(error)
	}
	return insight.(*module.BookInsight), nil
}

func GetBookInsightsWithAccount(account, bookID string) []*module.BookInsight {
	actor := getReadingActor(account)
	cmd := &GetBookInsightsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		BookID:  bookID,
	}
	actor.Send(cmd)
	insights := <-cmd.Response()
	return insights.([]*module.BookInsight)
}

func UpdateBookInsightWithAccount(account, insightID string, updates map[string]interface{}) error {
	actor := getReadingActor(account)
	cmd := &UpdateBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		InsightID: insightID,
		Updates:   updates,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func DeleteBookInsightWithAccount(account, insightID string) error {
	actor := getReadingActor(account)
	cmd := &DeleteBookInsightCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		InsightID: insightID,
	}
	actor.Send(cmd)
	err := <-cmd.Response()
	if err == nil {
		return nil
	}
	return err.(error)
}

func SearchBooksWithAccount(account, keyword string) []*module.Book {
	actor := getReadingActor(account)
	cmd := &SearchBooksCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Keyword: keyword,
	}
	actor.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func FilterBooksByStatusWithAccount(account, status string) []*module.Book {
	actor := getReadingActor(account)
	cmd := &FilterBooksByStatusCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Status:  status,
	}
	actor.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func FilterBooksByCategoryWithAccount(account, category string) []*module.Book {
	actor := getReadingActor(account)
	cmd := &FilterBooksByCategoryCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Category: category,
	}
	actor.Send(cmd)
	books := <-cmd.Response()
	return books.([]*module.Book)
}

func GetReadingStatisticsWithAccount(account string) map[string]interface{} {
	actor := getReadingActor(account)
	cmd := &GetReadingStatisticsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	actor.Send(cmd)
	stats := <-cmd.Response()
	return stats.(map[string]interface{})
}

func AddReadingPlanWithAccount(account, title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	actor := getReadingActor(account)
	cmd := &AddReadingPlanCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
		Title:       title,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		TargetBooks: targetBooks,
	}
	actor.Send(cmd)
	plan := <-cmd.Response()
	err := <-cmd.Response()
	if plan == nil {
		return nil, err.(error)
	}
	return plan.(*module.ReadingPlan), nil
}

func GetReadingPlanWithAccount(account, planID string) *module.ReadingPlan {
	actor := getReadingActor(account)
	cmd := &GetReadingPlanCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		PlanID:  planID,
	}
	actor.Send(cmd)
	plan := <-cmd.Response()
	if plan == nil {
		return nil
	}
	return plan.(*module.ReadingPlan)
}

func GetAllReadingPlansWithAccount(account string) []*module.ReadingPlan {
	actor := getReadingActor(account)
	cmd := &GetAllReadingPlansCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	actor.Send(cmd)
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
