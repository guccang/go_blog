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
}

// 用户
type User struct {
	Account string
	Password string
}

func Info(){
	fmt.Println("info module v1.0")
}



