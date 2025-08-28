package module

import log "mylog"

// blog权限
const (
	EAuthType_private     = 1
	EAuthType_public      = 2
	EAuthType_encrypt     = 4
	EAuthType_cooperation = 8
	EAuthType_diary       = 16 // 日记博客，需要密码保护
	EAuthType_all         = 0xffff
)

// 网页上传的数据集合
type UploadedBlogData struct {
	Title    string
	Content  string
	AuthType int
	Tags     string
	Encrypt  int
	Account  string
}

// blog数据
type Blog struct {
	Title      string
	Content    string
	CreateTime string
	ModifyTime string
	AccessTime string
	ModifyNum  int
	AccessNum  int
	AuthType   int
	Tags       string
	Encrypt    int
	Account    string
}

// 用户
type User struct {
	Account  string
	Password string
}

// 评论者用户信息
type CommentUser struct {
	UserID       string `json:"user_id"`       // 唯一用户ID
	Username     string `json:"username"`      // 显示用户名
	Email        string `json:"email"`         // 邮箱(可选)
	Avatar       string `json:"avatar"`        // 头像URL
	RegisterTime string `json:"register_time"` // 注册时间
	LastActive   string `json:"last_active"`   // 最后活跃时间
	CommentCount int    `json:"comment_count"` // 评论总数
	Reputation   int    `json:"reputation"`    // 信誉积分
	Status       int    `json:"status"`        // 用户状态: 1-正常, 2-禁言, 3-封禁
	IsVerified   bool   `json:"is_verified"`   // 是否已验证
}

// 评论者会话信息
type CommentSession struct {
	SessionID  string `json:"session_id"`  // 会话ID
	UserID     string `json:"user_id"`     // 用户ID
	IP         string `json:"ip"`          // IP地址
	UserAgent  string `json:"user_agent"`  // 浏览器信息
	CreateTime string `json:"create_time"` // 创建时间
	ExpireTime string `json:"expire_time"` // 过期时间
	IsActive   bool   `json:"is_active"`   // 是否活跃
}

// 用户名占用记录
type UsernameReservation struct {
	Username    string `json:"username"`     // 用户名
	UserID      string `json:"user_id"`      // 占用者ID
	ReserveTime string `json:"reserve_time"` // 占用时间
	IsTemporary bool   `json:"is_temporary"` // 是否临时占用
}

// 评论
type Comment struct {
	Owner      string
	Msg        string
	CreateTime string
	ModifyTime string
	Idx        int
	Pwd        string
	Mail       string
	// 新增字段
	UserID      string `json:"user_id"`      // 评论者用户ID
	SessionID   string `json:"session_id"`   // 会话ID
	IP          string `json:"ip"`           // IP地址
	UserAgent   string `json:"user_agent"`   // 浏览器信息
	IsAnonymous bool   `json:"is_anonymous"` // 是否匿名评论
	IsVerified  bool   `json:"is_verified"`  // 评论者是否已验证
}

// 博客评论
type BlogComments struct {
	Title    string
	Comments []*Comment
}

func Info() {
	log.InfoF(log.ModuleCommon, "info module v1.0")
}

// 读书相关数据结构
type Book struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	ISBN        string   `json:"isbn"`
	Publisher   string   `json:"publisher"`
	PublishDate string   `json:"publish_date"`
	CoverUrl    string   `json:"cover_url"`
	Description string   `json:"description"`
	TotalPages  int      `json:"total_pages"`
	CurrentPage int      `json:"current_page"`
	Category    []string `json:"category"`
	Tags        []string `json:"tags"`
	SourceUrl   string   `json:"source_url"`
	AddTime     string   `json:"add_time"`
	Rating      float64  `json:"rating"`
	Status      string   `json:"status"` // unstart, reading, finished, paused
}

type ReadingRecord struct {
	BookID           string           `json:"book_id"`
	Status           string           `json:"status"`
	StartDate        string           `json:"start_date"`
	EndDate          string           `json:"end_date"`
	CurrentPage      int              `json:"current_page"`
	TotalReadingTime int              `json:"total_reading_time"` // 分钟
	ReadingSessions  []ReadingSession `json:"reading_sessions"`
	LastUpdateTime   string           `json:"last_update_time"`
}

type ReadingSession struct {
	Date      string `json:"date"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
	Duration  int    `json:"duration"` // 分钟
	Notes     string `json:"notes"`
}

type BookNote struct {
	ID         string   `json:"id"`
	BookID     string   `json:"book_id"`
	Type       string   `json:"type"` // note, insight, quote
	Chapter    string   `json:"chapter"`
	Page       int      `json:"page"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	CreateTime string   `json:"create_time"`
	UpdateTime string   `json:"update_time"`
}

type BookInsight struct {
	ID           string   `json:"id"`
	BookID       string   `json:"book_id"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	KeyTakeaways []string `json:"key_takeaways"`
	Applications []string `json:"applications"`
	Rating       int      `json:"rating"`
	Tags         []string `json:"tags"`
	CreateTime   string   `json:"create_time"`
	UpdateTime   string   `json:"update_time"`
}

// 新增数据结构

// 阅读计划
type ReadingPlan struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	TargetBooks []string `json:"target_books"` // 书籍ID列表
	Status      string   `json:"status"`       // active, completed, paused
	Progress    float64  `json:"progress"`     // 完成百分比
	CreateTime  string   `json:"create_time"`
	UpdateTime  string   `json:"update_time"`
}

// 阅读目标
type ReadingGoal struct {
	ID           string `json:"id"`
	Year         int    `json:"year"`
	Month        int    `json:"month,omitempty"` // 可选，月度目标
	TargetType   string `json:"target_type"`     // books, pages, time
	TargetValue  int    `json:"target_value"`    // 目标值
	CurrentValue int    `json:"current_value"`   // 当前值
	Status       string `json:"status"`          // active, completed, failed
	CreateTime   string `json:"create_time"`
	UpdateTime   string `json:"update_time"`
}

// 书籍推荐
type BookRecommendation struct {
	ID         string   `json:"id"`
	BookID     string   `json:"book_id"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Reason     string   `json:"reason"` // 推荐理由
	Score      float64  `json:"score"`  // 推荐分数
	Tags       []string `json:"tags"`
	SourceType string   `json:"source_type"` // similar, author, category
	SourceID   string   `json:"source_id"`   // 来源书籍ID
	CreateTime string   `json:"create_time"`
}

// 阅读统计
type ReadingStats struct {
	Date          string   `json:"date"`
	BooksRead     int      `json:"books_read"`
	PagesRead     int      `json:"pages_read"`
	TimeSpent     int      `json:"time_spent"` // 分钟
	NotesCount    int      `json:"notes_count"`
	InsightsCount int      `json:"insights_count"`
	AverageRating float64  `json:"average_rating"`
	TopCategories []string `json:"top_categories"`
	MonthlyGoal   int      `json:"monthly_goal"`
	GoalProgress  float64  `json:"goal_progress"`
}

// 书籍导出配置
type ExportConfig struct {
	Format          string   `json:"format"` // pdf, markdown, txt
	IncludeNotes    bool     `json:"include_notes"`
	IncludeInsights bool     `json:"include_insights"`
	BookIDs         []string `json:"book_ids"`
	DateRange       struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"date_range"`
}

// 书籍收藏夹
type BookCollection struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BookIDs     []string `json:"book_ids"`
	IsPublic    bool     `json:"is_public"`
	Tags        []string `json:"tags"`
	CreateTime  string   `json:"create_time"`
	UpdateTime  string   `json:"update_time"`
}

// 阅读时间记录
type ReadingTimeRecord struct {
	ID         string `json:"id"`
	BookID     string `json:"book_id"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Duration   int    `json:"duration"` // 分钟
	Pages      int    `json:"pages"`    // 阅读页数
	Notes      string `json:"notes"`
	CreateTime string `json:"create_time"`
}
