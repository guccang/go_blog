package taskbreakdown

import (
	"time"
)

// 任务状态常量
const (
	StatusPlanning    = "planning"     // 规划中
	StatusInProgress  = "in-progress"  // 进行中
	StatusCompleted   = "completed"    // 已完成
	StatusBlocked     = "blocked"      // 阻塞中
	StatusCancelled   = "cancelled"    // 已取消
)

// 优先级常量
const (
	PriorityHighest = 1 // 最高优先级
	PriorityHigh    = 2 // 高优先级
	PriorityMedium  = 3 // 中等优先级
	PriorityLow     = 4 // 低优先级
	PriorityLowest  = 5 // 最低优先级
)

// ComplexTask 复杂任务模型
type ComplexTask struct {
	ID            string        `json:"id"`              // 任务唯一ID (UUID)
	Title         string        `json:"title"`           // 任务标题
	Description   string        `json:"description"`     // 任务描述
	Status        string        `json:"status"`          // 状态: planning/in-progress/completed/blocked/cancelled
	Priority      int           `json:"priority"`        // 优先级 1-5 (1最高)
	StartDate     string        `json:"start_date"`      // 开始日期 (YYYY-MM-DD)
	EndDate       string        `json:"end_date"`        // 结束日期 (YYYY-MM-DD)
	EstimatedTime int           `json:"estimated_time"`  // 预估时间(分钟)
	ActualTime    int           `json:"actual_time"`     // 实际耗时(分钟)
	DailyTime     int           `json:"daily_time"`      // 每天分配时间(分钟)
	Progress      int           `json:"progress"`        // 进度百分比 0-100
	Subtasks      []ComplexTask `json:"subtasks"`        // 子任务列表
	Dependencies  []string      `json:"dependencies"`    // 依赖任务ID列表
	CreatedAt     string        `json:"created_at"`      // 创建时间 (RFC3339)
	UpdatedAt     string        `json:"updated_at"`      // 更新时间 (RFC3339)
	ParentID      string        `json:"parent_id"`       // 父任务ID (空表示根任务)
	Order         int           `json:"order"`           // 显示顺序
	Tags          []string      `json:"tags"`            // 标签分类
	Deleted       bool          `json:"deleted"`         // 是否已删除
}

// TaskIndex 任务索引 - 用于快速查询
type TaskIndex struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	Progress  int    `json:"progress"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	ParentID  string `json:"parent_id"`
	Order     int    `json:"order"`
}

// TaskUpdateRequest 任务更新请求
type TaskUpdateRequest struct {
	Title         *string   `json:"title,omitempty"`
	Description   *string   `json:"description,omitempty"`
	Status        *string   `json:"status,omitempty"`
	Priority      *int      `json:"priority,omitempty"`
	StartDate     *string   `json:"start_date,omitempty"`
	EndDate       *string   `json:"end_date,omitempty"`
	EstimatedTime *int      `json:"estimated_time,omitempty"`
	ActualTime    *int      `json:"actual_time,omitempty"`
	DailyTime     *int      `json:"daily_time,omitempty"`
	Progress      *int      `json:"progress,omitempty"`
	ParentID      *string   `json:"parent_id,omitempty"`
	Order         *int      `json:"order,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
}

// TaskCreateRequest 任务创建请求
type TaskCreateRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Status        string   `json:"status,omitempty"`
	Priority      int      `json:"priority,omitempty"`
	StartDate     string   `json:"start_date,omitempty"`
	EndDate       string   `json:"end_date,omitempty"`
	EstimatedTime int      `json:"estimated_time,omitempty"`
	DailyTime     int      `json:"daily_time,omitempty"`
	ParentID      string   `json:"parent_id,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// TimelineData 时间线数据
type TimelineData struct {
	Tasks []TimelineTask `json:"tasks"`
}

// TimelineTask 时间线任务
type TimelineTask struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Progress  int    `json:"progress"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	ParentID  string `json:"parent_id,omitempty"`
}

// StatisticsData 统计数据
type StatisticsData struct {
	TotalTasks      int            `json:"total_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	InProgressTasks int            `json:"in_progress_tasks"`
	BlockedTasks    int            `json:"blocked_tasks"`
	TotalTime       int            `json:"total_time"` // 总预估时间(分钟)
	StatusDistribution map[string]int `json:"status_distribution"`
	PriorityDistribution map[int]int  `json:"priority_distribution"`
	// 时间分析字段
	DailyAvailableTime int     `json:"daily_available_time"` // 每天可用时间(分钟)
	TotalDailyTime     int     `json:"total_daily_time"`     // 总每日分配时间(分钟)
	RequiredDays       float64 `json:"required_days"`        // 所需天数
	TimeMargin         int     `json:"time_margin"`          // 时间余量(分钟)
	TimeUtilization    float64 `json:"time_utilization"`     // 时间利用率(%)
	TimeStatus         string  `json:"time_status"`          // 时间状态: "sufficient", "insufficient", "warning"
}

// GraphNode 网络图节点
type GraphNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	Progress  int    `json:"progress"`
	Level     int    `json:"level"`
	Group     string `json:"group"` // 状态分组
	Value     int    `json:"value"` // 节点大小基于优先级
}

// GraphEdge 网络图边
type GraphEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`      // "parent-child" 或 "dependency"
	Arrows    string `json:"arrows"`    // "to" 表示方向
	Dashes    bool   `json:"dashes"`    // true 表示依赖关系为虚线
}

// GraphData 网络图数据
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// 生成任务ID
func GenerateTaskID() string {
	return "task-" + time.Now().Format("20060102150405") + "-" + randomString(8)
}

// 生成随机字符串(简化版，实际应该使用更安全的随机生成)
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		// 简化实现，实际应该使用crypto/rand
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

// 验证任务状态是否有效
func IsValidStatus(status string) bool {
	switch status {
	case StatusPlanning, StatusInProgress, StatusCompleted, StatusBlocked, StatusCancelled:
		return true
	default:
		return false
	}
}

// 验证优先级是否有效
func IsValidPriority(priority int) bool {
	return priority >= PriorityHighest && priority <= PriorityLowest
}

// 计算任务深度
func (t *ComplexTask) CalculateDepth() int {
	maxDepth := 0
	for _, subtask := range t.Subtasks {
		depth := subtask.CalculateDepth()
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth + 1
}

// 检查是否超过最大深度限制
func (t *ComplexTask) ExceedsMaxDepth(maxDepth int) bool {
	return t.CalculateDepth() > maxDepth
}

// ==================== 时间趋势数据结构 ====================

// TimePoint 时间点数据
type TimePoint struct {
	Date  string `json:"date"`  // 日期 (YYYY-MM-DD)
	Value int    `json:"value"` // 数值
}

// TimeTrendData 时间趋势数据
type TimeTrendData struct {
	Title      string      `json:"title"`       // 图表标题
	Unit       string      `json:"unit"`        // 单位 (如 "个", "%", "分钟")
	DataPoints []TimePoint `json:"data_points"` // 数据点
	Total      int         `json:"total"`       // 总计
	Average    float64     `json:"average"`     // 平均值
	Max        int         `json:"max"`         // 最大值
	Min        int         `json:"min"`         // 最小值
	Trend      string      `json:"trend"`       // 趋势: "up", "down", "stable"
}

// TimeTrendsResponse 时间趋势响应
type TimeTrendsResponse struct {
	CreationTrend   *TimeTrendData `json:"creation_trend,omitempty"`   // 任务创建趋势
	CompletionTrend *TimeTrendData `json:"completion_trend,omitempty"` // 任务完成趋势
	ProgressTrend   *TimeTrendData `json:"progress_trend,omitempty"`   // 进度趋势
	TimeRange       string         `json:"time_range"`                 // 时间范围: "7d", "30d", "90d", "1y"
	StartDate       string         `json:"start_date"`                 // 开始日期
	EndDate         string         `json:"end_date"`                   // 结束日期
}