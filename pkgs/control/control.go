package control

import (
	"fmt"
	"module"
	"blog"
	"comment"
	"search"
)

func Info(){
	fmt.Println("info control v1.0");
}

func Init(){
	comment.Init()
	blog.Init()
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
