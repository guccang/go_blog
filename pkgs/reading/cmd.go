package reading

import (
	"core"
)

// cmd

// 添加书籍cmd
type AddBookCmd struct {
	core.ActorCommand
	Account     string
	Title       string
	Author      string
	ISBN        string
	Publisher   string
	PublishDate string
	CoverUrl    string
	Description string
	SourceUrl   string
	TotalPages  int
	Category    []string
	Tags        []string
}

func (cmd *AddBookCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	book, err := readingActor.addBook(cmd.Title, cmd.Author, cmd.ISBN, cmd.Publisher, cmd.PublishDate, cmd.CoverUrl, cmd.Description, cmd.SourceUrl, cmd.TotalPages, cmd.Category, cmd.Tags)
	cmd.Response() <- book
	cmd.Response() <- err
}

// 获取书籍cmd
type GetBookCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *GetBookCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	book := readingActor.getBook(cmd.BookID)
	cmd.Response() <- book
}

// 获取所有书籍cmd
type GetAllBooksCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllBooksCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	books := readingActor.getAllBooks()
	cmd.Response() <- books
}

// 更新书籍cmd
type UpdateBookCmd struct {
	core.ActorCommand
	Account string
	BookID  string
	Updates map[string]interface{}
}

func (cmd *UpdateBookCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.updateBook(cmd.BookID, cmd.Updates)
	cmd.Response() <- err
}

// 删除书籍cmd
type DeleteBookCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *DeleteBookCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.deleteBook(cmd.BookID)
	cmd.Response() <- err
}

// 开始阅读cmd
type StartReadingCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *StartReadingCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.startReading(cmd.Account, cmd.BookID)
	cmd.Response() <- err
}

// 更新阅读进度cmd
type UpdateReadingProgressCmd struct {
	core.ActorCommand
	Account     string
	BookID      string
	CurrentPage int
	Notes       string
}

func (cmd *UpdateReadingProgressCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.updateReadingProgress(cmd.Account, cmd.BookID, cmd.CurrentPage, cmd.Notes)
	cmd.Response() <- err
}

// 获取阅读记录cmd
type GetReadingRecordCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *GetReadingRecordCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	record := readingActor.getReadingRecord(cmd.Account, cmd.BookID)
	cmd.Response() <- record
}

// 添加笔记cmd
type AddBookNoteCmd struct {
	core.ActorCommand
	Account  string
	BookID   string
	NoteType string
	Chapter  string
	Content  string
	Page     int
	Tags     []string
}

func (cmd *AddBookNoteCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	note, err := readingActor.addBookNote(cmd.Account, cmd.BookID, cmd.NoteType, cmd.Chapter, cmd.Content, cmd.Page, cmd.Tags)
	cmd.Response() <- note
	cmd.Response() <- err
}

// 获取笔记cmd
type GetBookNotesCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *GetBookNotesCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	notes := readingActor.getBookNotes(cmd.Account, cmd.BookID)
	cmd.Response() <- notes
}

// 更新笔记cmd
type UpdateBookNoteCmd struct {
	core.ActorCommand
	Account string
	BookID  string
	NoteID  string
	Updates map[string]interface{}
}

func (cmd *UpdateBookNoteCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.updateBookNote(cmd.Account, cmd.BookID, cmd.NoteID, cmd.Updates)
	cmd.Response() <- err
}

// 删除笔记cmd
type DeleteBookNoteCmd struct {
	core.ActorCommand
	Account string
	BookID  string
	NoteID  string
}

func (cmd *DeleteBookNoteCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.deleteBookNote(cmd.Account, cmd.BookID, cmd.NoteID)
	cmd.Response() <- err
}

// 添加读书感悟cmd
type AddBookInsightCmd struct {
	core.ActorCommand
	Account      string
	BookID       string
	Title        string
	Content      string
	KeyTakeaways []string
	Applications []string
	Rating       int
	Tags         []string
}

func (cmd *AddBookInsightCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	insight, err := readingActor.addBookInsight(cmd.Account, cmd.BookID, cmd.Title, cmd.Content, cmd.KeyTakeaways, cmd.Applications, cmd.Rating, cmd.Tags)
	cmd.Response() <- insight
	cmd.Response() <- err
}

// 获取读书感悟cmd
type GetBookInsightsCmd struct {
	core.ActorCommand
	Account string
	BookID  string
}

func (cmd *GetBookInsightsCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	insights := readingActor.getBookInsights(cmd.Account, cmd.BookID)
	cmd.Response() <- insights
}

// 更新读书感悟cmd
type UpdateBookInsightCmd struct {
	core.ActorCommand
	Account   string
	InsightID string
	Updates   map[string]interface{}
}

func (cmd *UpdateBookInsightCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.updateBookInsight(cmd.Account, cmd.InsightID, cmd.Updates)
	cmd.Response() <- err
}

// 删除读书感悟cmd
type DeleteBookInsightCmd struct {
	core.ActorCommand
	Account   string
	InsightID string
}

func (cmd *DeleteBookInsightCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	err := readingActor.deleteBookInsight(cmd.Account, cmd.InsightID)
	cmd.Response() <- err
}

// 搜索书籍cmd
type SearchBooksCmd struct {
	core.ActorCommand
	Account string
	Keyword string
}

func (cmd *SearchBooksCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	books := readingActor.searchBooks(cmd.Keyword)
	cmd.Response() <- books
}

// 按状态筛选书籍cmd
type FilterBooksByStatusCmd struct {
	core.ActorCommand
	Account string
	Status  string
}

func (cmd *FilterBooksByStatusCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	books := readingActor.filterBooksByStatus(cmd.Status)
	cmd.Response() <- books
}

// 按分类筛选书籍cmd
type FilterBooksByCategoryCmd struct {
	core.ActorCommand
	Account  string
	Category string
}

func (cmd *FilterBooksByCategoryCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	books := readingActor.filterBooksByCategory(cmd.Category)
	cmd.Response() <- books
}

// 获取阅读统计cmd
type GetReadingStatisticsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetReadingStatisticsCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	stats := readingActor.getReadingStatistics()
	cmd.Response() <- stats
}

// 添加阅读计划cmd
type AddReadingPlanCmd struct {
	core.ActorCommand
	Account     string
	Title       string
	Description string
	StartDate   string
	EndDate     string
	TargetBooks []string
}

func (cmd *AddReadingPlanCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	plan, err := readingActor.addReadingPlan(cmd.Title, cmd.Description, cmd.StartDate, cmd.EndDate, cmd.TargetBooks)
	cmd.Response() <- plan
	cmd.Response() <- err
}

// 获取阅读计划cmd
type GetReadingPlanCmd struct {
	core.ActorCommand
	Account string
	PlanID  string
}

func (cmd *GetReadingPlanCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	plan := readingActor.getReadingPlan(cmd.PlanID)
	cmd.Response() <- plan
}

// 获取所有阅读计划cmd
type GetAllReadingPlansCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllReadingPlansCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	plans := readingActor.getAllReadingPlans()
	cmd.Response() <- plans
}
