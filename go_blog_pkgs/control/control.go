package control

import (
	"fmt"
	"module"
	"blog"
	"comment"
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

func GetAll() []*module.Blog {
	return blog.GetAll()
}

func GetMatch(match string) []*module.Blog{
	return blog.GetMatch(match)
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
