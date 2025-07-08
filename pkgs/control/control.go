package control

import (
	"fmt"
	"module"
	"blog"
	"comment"
	"search"
	"statistics"
)

func Info(){
	fmt.Println("info control v1.0");
}

func Init(){
	comment.Init()
	blog.Init()
	statistics.Init()
}

func ImportBlogsFromPath(dir string){
	blog.ImportBlogsFromPath(dir)
}

func GetBlog(title string)*module.Blog{
	return blog.GetBlog(title)
}

func AddBlog(udb *module.UploadedBlogData) int{
	return blog.AddBlog(udb)
}

func ModifyBlog(udb *module.UploadedBlogData) int {
	return blog.ModifyBlog(udb)
}

func GetAll(cnt int,flag int) []*module.Blog {
	return blog.GetAll(cnt,flag)
}

func GetMatch(match string) []*module.Blog{
	return search.Search(match)
}


func UpdateAccessTime(b *module.Blog){
	blog.UpdateAccessTime(b)
}

func GetBlogAuthType(blogname string) int {
	return blog.GetBlogAuthType(blogname)
}

func GetBlogComments(blogname string) *module.BlogComments {
	return comment.GetComments(blogname)
}

func AddComment(title string,msg string,owner string,pwd string,mail string){
	comment.AddComment(title,msg,owner,pwd,mail)
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

func GetBlogsNum() int {
	return blog.GetBlogsNum()
}

func DeleteBlog(title string) int {
	return blog.DeleteBlog(title)
}

func GetRecentlyTimedBlog(title string) *module.Blog{
	return blog.GetRecentlyTimedBlog(title)
}

func TagReplace(from string, to string) {
	blog.TagReplace(from,to)
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
