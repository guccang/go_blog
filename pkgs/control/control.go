package control

import (
	"blog"
	"comment"
	"errors"
	"fmt"
	"module"
	"reading"
	"search"
	"statistics"
)

func Info() {
	fmt.Println("info control v1.0")
}

func Init() {
}

func ImportBlogsFromPath(dir string) {
	blog.ImportBlogsFromPath(dir)
}

func GetBlog(title string) *module.Blog {
	return blog.GetBlog(title)
}

func AddBlog(udb *module.UploadedBlogData) int {
	return blog.AddBlog(udb)
}

func ModifyBlog(udb *module.UploadedBlogData) int {
	return blog.ModifyBlog(udb)
}

func GetAll(cnt int, flag int) []*module.Blog {
	return blog.GetAll(cnt, flag)
}

func GetMatch(match string) []*module.Blog {
	return search.Search(match)
}

func UpdateAccessTime(b *module.Blog) {
	blog.UpdateAccessTime(b)
}

func GetBlogAuthType(blogname string) int {
	return blog.GetBlogAuthType(blogname)
}

func GetBlogComments(blogname string) *module.BlogComments {
	return comment.GetComments(blogname)
}

func AddComment(title string, msg string, owner string, pwd string, mail string) {
	comment.AddComment(title, msg, owner, pwd, mail)
}

func AddCommentWithAuth(title, msg, sessionID, ip, userAgent string) (int, string) {
	return comment.AddCommentWithAuth(title, msg, sessionID, ip, userAgent)
}

func AddAnonymousComment(title, msg, username, email, ip, userAgent string) (int, string) {
	return comment.AddAnonymousComment(title, msg, username, email, ip, userAgent)
}

func AddCommentWithPassword(title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	return comment.AddCommentWithPassword(title, msg, username, email, password, ip, userAgent)
}

// 读书功能控制层接口
func AddBook(title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl string, totalPages int, category, tags []string) (*module.Book, error) {
	return reading.AddBook(title, author, isbn, publisher, publishDate, coverUrl, description, sourceUrl, totalPages, category, tags)
}

func GetBook(bookID string) *module.Book {
	return reading.GetBook(bookID)
}

func GetAllBooks() map[string]*module.Book {
	return reading.GetAllBooks()
}

func UpdateBook(bookID string, updates map[string]interface{}) error {
	return reading.UpdateBook(bookID, updates)
}

func DeleteBook(bookID string) error {
	return reading.DeleteBook(bookID)
}

func StartReading(bookID string) error {
	return reading.StartReading(bookID)
}

func UpdateReadingProgress(bookID string, currentPage int, notes string) error {
	return reading.UpdateReadingProgress(bookID, currentPage, notes)
}

func GetReadingRecord(bookID string) *module.ReadingRecord {
	return reading.GetReadingRecord(bookID)
}

func AddBookNote(bookID, noteType, chapter, content string, page int, tags []string) (*module.BookNote, error) {
	return reading.AddBookNote(bookID, noteType, chapter, content, page, tags)
}

func GetBookNotes(bookID string) []*module.BookNote {
	return reading.GetBookNotes(bookID)
}

func UpdateBookNote(bookID, noteID string, updates map[string]interface{}) error {
	return reading.UpdateBookNote(bookID, noteID, updates)
}

func DeleteBookNote(bookID, noteID string) error {
	return reading.DeleteBookNote(bookID, noteID)
}

func AddBookInsight(bookID, title, content string, keyTakeaways, applications []string, rating int, tags []string) (*module.BookInsight, error) {
	return reading.AddBookInsight(bookID, title, content, keyTakeaways, applications, rating, tags)
}

func GetBookInsights(bookID string) []*module.BookInsight {
	return reading.GetBookInsights(bookID)
}

func UpdateBookInsight(insightID string, updates map[string]interface{}) error {
	return reading.UpdateBookInsight(insightID, updates)
}

func DeleteBookInsight(insightID string) error {
	return reading.DeleteBookInsight(insightID)
}

func SearchBooks(keyword string) []*module.Book {
	return reading.SearchBooks(keyword)
}

func FilterBooksByStatus(status string) []*module.Book {
	return reading.FilterBooksByStatus(status)
}

func FilterBooksByCategory(category string) []*module.Book {
	return reading.FilterBooksByCategory(category)
}

func GetReadingStatistics() map[string]interface{} {
	return reading.GetReadingStatistics()
}

// 新增功能接口

// 阅读计划相关
func AddReadingPlan(title, description, startDate, endDate string, targetBooks []string) (*module.ReadingPlan, error) {
	return reading.AddReadingPlan(title, description, startDate, endDate, targetBooks)
}

func GetReadingPlan(planID string) *module.ReadingPlan {
	return reading.GetReadingPlan(planID)
}

func GetAllReadingPlans() []*module.ReadingPlan {
	return reading.GetAllReadingPlans()
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
func FinishBook(bookID string) error {
	book := reading.GetBook(bookID)
	if book == nil {
		return errors.New("书籍不存在")
	}
	return reading.UpdateReadingProgress(bookID, book.TotalPages, "")
}

func UpdateBookProgress(bookID string, currentPage int) error {
	return reading.UpdateReadingProgress(bookID, currentPage, "")
}

func AddSimpleBookNote(bookID, chapter, content string, page int) (*module.BookNote, error) {
	return reading.AddBookNote(bookID, "note", chapter, content, page, []string{})
}

func AddSimpleBookInsight(bookID, title, content, takeaway string, rating int) (*module.BookInsight, error) {
	keyTakeaways := []string{}
	if takeaway != "" {
		keyTakeaways = append(keyTakeaways, takeaway)
	}
	return reading.AddBookInsight(bookID, title, content, keyTakeaways, []string{}, rating, []string{})
}

func GetBlogsNum() int {
	return blog.GetBlogsNum()
}

func DeleteBlog(title string) int {
	return blog.DeleteBlog(title)
}

func GetRecentlyTimedBlog(title string) *module.Blog {
	return blog.GetRecentlyTimedBlog(title)
}

func TagReplace(from string, to string) {
	blog.TagReplace(from, to)
}

// 统计相关功能
func GetStatistics() *statistics.Statistics {
	return statistics.GetStatistics()
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
