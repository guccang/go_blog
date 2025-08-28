package control

import (
	"blog"
	"comment"
	"errors"
	"module"
	log "mylog"
	"reading"
	"search"
	"statistics"
)

func Info() {
	log.InfoF(log.ModuleControl, "info control v1.0")
}

func Init() {
}

func ImportBlogsFromPath(account, dir string) {
	blog.ImportBlogsFromPathWithAccount(account, dir)
}

func GetBlog(account, title string) *module.Blog {
	return blog.GetBlogWithAccount(account, title)
}

func AddBlog(account string, udb *module.UploadedBlogData) int {
	return blog.AddBlogWithAccount(account, udb)
}

func ModifyBlog(account string, udb *module.UploadedBlogData) int {
	return blog.ModifyBlogWithAccount(account, udb)
}

func GetAll(account string, cnt int, flag int) []*module.Blog {
	return blog.GetAllWithAccount(account, cnt, flag)
}

func GetBlogs(account string) map[string]*module.Blog {
	return blog.GetBlogsWithAccount(account)
}

func GetMatch(account, match string) []*module.Blog {
	return search.Search(account, match)
}

func UpdateAccessTime(account string, b *module.Blog) {
	blog.UpdateAccessTimeWithAccount(account, b)
}

func GetBlogAuthType(account, blogname string) int {
	return blog.GetBlogAuthTypeWithAccount(account, blogname)
}

func GetBlogComments(account, blogname string) *module.BlogComments {
	return comment.GetComments(account, blogname)
}

func AddComment(account, title string, msg string, owner string, pwd string, mail string) {
	comment.AddComment(account, title, msg, owner, pwd, mail)
}

func AddCommentWithAuth(account, title, msg, sessionID, ip, userAgent string) (int, string) {
	return comment.AddCommentWithAuth(account, title, msg, sessionID, ip, userAgent)
}

func AddAnonymousComment(account, title, msg, username, email, ip, userAgent string) (int, string) {
	return comment.AddAnonymousComment(account, title, msg, username, email, ip, userAgent)
}

func AddCommentWithPassword(account, title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	return comment.AddCommentWithPassword(account, title, msg, username, email, password, ip, userAgent)
}

// 读书功能控制层接口
func AddBook(account, title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	return reading.AddBookWithAccount(account, title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl, totalPages, category, tags)
}

func GetBook(account, bookID string) *module.Book {
	return reading.GetBookWithAccount(account, bookID)
}

func GetAllBooks(account string) map[string]*module.Book {
	return reading.GetAllBooksWithAccount(account)
}

func UpdateBook(account, bookID string, updates map[string]interface{}) error {
	return reading.UpdateBookWithAccount(account, bookID, updates)
}

func DeleteBook(account, bookID string) error {
	return reading.DeleteBookWithAccount(account, bookID)
}

func StartReading(account, bookID string) error {
	return reading.StartReadingWithAccount(account, bookID)
}

func UpdateReadingProgress(account, bookID string, currentPage int, notes string) error {
	return reading.UpdateReadingProgressWithAccount(account, bookID, currentPage, notes)
}

func GetReadingRecord(account, bookID string) *module.ReadingRecord {
	return reading.GetReadingRecordWithAccount(account, bookID)
}

func AddBookNote(account, bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	return reading.AddBookNoteWithAccount(account, bookID, noteType, chapter, content, page, tags)
}

func GetBookNotes(account, bookID string) []*module.BookNote {
	return reading.GetBookNotesWithAccount(account, bookID)
}

func UpdateBookNote(account, bookID, noteID string, updates map[string]interface{}) error {
	return reading.UpdateBookNoteWithAccount(account, bookID, noteID, updates)
}

func DeleteBookNote(account, bookID, noteID string) error {
	return reading.DeleteBookNoteWithAccount(account, bookID, noteID)
}

func AddBookInsight(account, bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	return reading.AddBookInsightWithAccount(account, bookID, title, content, keyTakeaways, applications, rating, tags)
}

func GetBookInsights(account, bookID string) []*module.BookInsight {
	return reading.GetBookInsightsWithAccount(account, bookID)
}

func UpdateBookInsight(account, insightID string, updates map[string]interface{}) error {
	return reading.UpdateBookInsightWithAccount(account, insightID, updates)
}

func DeleteBookInsight(account, insightID string) error {
	return reading.DeleteBookInsightWithAccount(account, insightID)
}

func SearchBooks(account, keyword string) []*module.Book {
	return reading.SearchBooksWithAccount(account, keyword)
}

func FilterBooksByStatus(account, status string) []*module.Book {
	return reading.FilterBooksByStatusWithAccount(account, status)
}

func FilterBooksByCategory(account, category string) []*module.Book {
	return reading.FilterBooksByCategoryWithAccount(account, category)
}

func GetReadingStatistics(account string) map[string]interface{} {
	return reading.GetReadingStatisticsWithAccount(account)
}

// 新增功能接口

// 阅读计划相关
func AddReadingPlan(account, title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	return reading.AddReadingPlanWithAccount(account, title, description, startDate, endDate, targetBooks)
}

func GetReadingPlan(account, planID string) *module.ReadingPlan {
	return reading.GetReadingPlanWithAccount(account, planID)
}

func GetAllReadingPlans(account string) []*module.ReadingPlan {
	return reading.GetAllReadingPlansWithAccount(account)
}

func UpdateReadingPlanProgress(planID string) error {
	return reading.UpdateReadingPlanProgress(planID)
}

// 阅读目标相关
func AddReadingGoal(year, month int, targetType string, targetValue int) (*module.ReadingGoal, error) {
	return reading.AddReadingGoal(year, month, targetType, targetValue)
}

func GetReadingGoals(year, month int) []*module.ReadingGoal {
	return reading.GetReadingGoals(year, month)
}

func UpdateReadingGoalProgress(goalID string) error {
	return reading.UpdateReadingGoalProgress(goalID)
}

// 书籍推荐相关
func GenerateBookRecommendations(bookID string) ([]*module.BookRecommendation, error) {
	return reading.GenerateBookRecommendations(bookID)
}

// 阅读时间记录相关
func StartReadingSession(bookID string) (*module.ReadingTimeRecord, error) {
	return reading.StartReadingSession(bookID)
}

func EndReadingSession(recordID string, pages int, notes string) error {
	return reading.EndReadingSession(recordID, pages, notes)
}

// 书籍收藏夹相关
func AddBookCollection(name, description string, bookIDs []string, isPublic bool) (*module.BookCollection, error) {
	return reading.AddBookCollection(name, description, bookIDs, isPublic)
}

func GetBookCollection(collectionID string) *module.BookCollection {
	return reading.GetBookCollection(collectionID)
}

func GetAllBookCollections() []*module.BookCollection {
	return reading.GetAllBookCollections()
}

func AddBookToCollection(collectionID, bookID string) error {
	return reading.AddBookToCollection(collectionID, bookID)
}

// 高级统计和导出
func GetAdvancedReadingStatistics() map[string]interface{} {
	return reading.GetAdvancedReadingStatistics()
}

func ExportReadingData(config *module.ExportConfig) (string, error) {
	return reading.ExportReadingData(config)
}

// 便捷函数
func FinishBook(account, bookID string) error {
	book := reading.GetBookWithAccount(account, bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}
	return reading.UpdateReadingProgressWithAccount(account, bookID, book.TotalPages, "")
}

func GetBlogsNum(account string) int {
	return blog.GetBlogsNumWithAccount(account)
}

func DeleteBlog(account, title string) int {
	return blog.DeleteBlogWithAccount(account, title)
}

func GetRecentlyTimedBlog(account, title string) *module.Blog {
	return blog.GetRecentlyTimedBlogWithAccount(account, title)
}

func TagReplace(account, from string, to string) {
	blog.TagReplaceWithAccount(account, from, to)
}

// 统计相关功能
func GetStatistics(account string) *statistics.Statistics {
	return statistics.GetStatistics(account)
}

func RecordBlogAccess(blogTitle, ip, userAgent string) {
	statistics.RecordBlogAccess(blogTitle, ip, userAgent)
}

func RecordUserLogin(account, ip string, success bool) {
	statistics.RecordUserLogin(account, ip, success)
}

func ClearStatisticsCache() {
	statistics.ClearCache()
}
