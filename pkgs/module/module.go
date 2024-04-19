package module

import(
	"fmt"
)

// blog权限
const (
	EAuthType_private = iota
	EAuthType_public 
)

// 网页上传的数据集合
type UploadedBlogData struct {
	Title string
	Content string
	AuthType int
	Tags	string
	Encrypt int
}
	
// blog数据
type Blog struct{
	Title string
	Content string
	CreateTime string
	ModifyTime string
	AccessTime string
	ModifyNum int
	AccessNum int
	AuthType  int
	Tags	  string
	Encrypt   int
}

// 用户
type User struct {
	Account string
	Password string
}


// 评论
type Comment struct {
	Owner string
	Msg  string
	CreateTime string
	ModifyTime string
	Idx  int
	Pwd string
	Mail string
}
// 博客评论
type BlogComments struct {
	Title string
	Comments[] *Comment
}


func Info(){
	fmt.Println("info module v1.0")
}
