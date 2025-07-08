package module

import(
	"fmt"
)

// blog权限
const (
	EAuthType_private = 1
	EAuthType_public  = 2
	EAuthType_encrypt = 4
	EAuthType_cooperation = 8
	EAuthType_all     = 0xffff
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

// 协作用户
type Cooperation struct {
	Account  string
	Password string
	CreateTime string
	Blogs	 string 
    Tags	 string
}

// 评论者用户信息
type CommentUser struct {
	UserID       string    `json:"user_id"`        // 唯一用户ID
	Username     string    `json:"username"`       // 显示用户名
	Email        string    `json:"email"`          // 邮箱(可选)
	Avatar       string    `json:"avatar"`         // 头像URL
	RegisterTime string    `json:"register_time"`  // 注册时间
	LastActive   string    `json:"last_active"`    // 最后活跃时间
	CommentCount int       `json:"comment_count"`  // 评论总数
	Reputation   int       `json:"reputation"`     // 信誉积分
	Status       int       `json:"status"`         // 用户状态: 1-正常, 2-禁言, 3-封禁
	IsVerified   bool      `json:"is_verified"`    // 是否已验证
}

// 评论者会话信息
type CommentSession struct {
	SessionID    string `json:"session_id"`     // 会话ID
	UserID       string `json:"user_id"`        // 用户ID
	IP           string `json:"ip"`             // IP地址
	UserAgent    string `json:"user_agent"`     // 浏览器信息
	CreateTime   string `json:"create_time"`    // 创建时间
	ExpireTime   string `json:"expire_time"`    // 过期时间
	IsActive     bool   `json:"is_active"`      // 是否活跃
}

// 用户名占用记录
type UsernameReservation struct {
	Username     string `json:"username"`       // 用户名
	UserID       string `json:"user_id"`        // 占用者ID
	ReserveTime  string `json:"reserve_time"`   // 占用时间
	IsTemporary  bool   `json:"is_temporary"`   // 是否临时占用
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
	// 新增字段
	UserID       string `json:"user_id"`        // 评论者用户ID
	SessionID    string `json:"session_id"`     // 会话ID
	IP           string `json:"ip"`             // IP地址
	UserAgent    string `json:"user_agent"`     // 浏览器信息
	IsAnonymous  bool   `json:"is_anonymous"`   // 是否匿名评论
	IsVerified   bool   `json:"is_verified"`    // 评论者是否已验证
}

// 博客评论
type BlogComments struct {
	Title string
	Comments[] *Comment
}

func Info(){
	fmt.Println("info module v1.0")
}
