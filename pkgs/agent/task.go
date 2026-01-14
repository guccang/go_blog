package agent

import (
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending  TaskStatus = "pending"
	StatusRunning  TaskStatus = "running"
	StatusPaused   TaskStatus = "paused"
	StatusDone     TaskStatus = "done"
	StatusFailed   TaskStatus = "failed"
	StatusCanceled TaskStatus = "canceled"
)

// AgentTask 后台任务
type AgentTask struct {
	ID          string     `json:"id"`
	Account     string     `json:"account"`     // 所属账户
	Title       string     `json:"title"`       // 任务标题
	Description string     `json:"description"` // 原始用户输入
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"` // 优先级 1-10
	Progress    float64    `json:"progress"` // 0-100

	// 任务分解
	SubTasks    []SubTask `json:"subtasks"`
	CurrentStep int       `json:"current_step"`

	// 时间信息
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// 执行上下文
	Context          map[string]interface{} `json:"context,omitempty"`
	Logs             []TaskLog              `json:"logs,omitempty"`
	Result           string                 `json:"result,omitempty"`
	Error            string                 `json:"error,omitempty"`
	LinkedReminderID string                 `json:"linked_reminder_id,omitempty"` // 关联的提醒ID

	// 内部控制
	pauseCh  chan struct{} `json:"-"`
	cancelCh chan struct{} `json:"-"`
	mu       sync.RWMutex  `json:"-"`
}

// SubTask 子任务
type SubTask struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	ToolCalls   []string `json:"tool_calls,omitempty"`
	Status      string   `json:"status"` // pending/running/done/failed
	Result      string   `json:"result,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// TaskLog 任务日志
type TaskLog struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"` // info/warn/error
	Message string    `json:"message"`
}

// NewTask 创建新任务
func NewTask(account, title, description string, priority int) *AgentTask {
	return &AgentTask{
		ID:          generateTaskID(),
		Account:     account,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		Priority:    priority,
		Progress:    0,
		SubTasks:    []SubTask{},
		CreatedAt:   time.Now(),
		Context:     make(map[string]interface{}),
		Logs:        []TaskLog{},
		pauseCh:     make(chan struct{}, 1),
		cancelCh:    make(chan struct{}, 1),
	}
}

// AddLog 添加日志
func (t *AgentTask) AddLog(level, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = append(t.Logs, TaskLog{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
}

// SetStatus 设置状态
func (t *AgentTask) SetStatus(status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
	if status == StatusRunning && t.StartedAt == nil {
		now := time.Now()
		t.StartedAt = &now
	}
	if status == StatusDone || status == StatusFailed || status == StatusCanceled {
		now := time.Now()
		t.FinishedAt = &now
	}
}

// SetProgress 设置进度
func (t *AgentTask) SetProgress(progress float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Progress = progress
}

// IsCanceled 检查是否已取消
func (t *AgentTask) IsCanceled() bool {
	select {
	case <-t.cancelCh:
		return true
	default:
		return false
	}
}

// IsPaused 检查是否已暂停
func (t *AgentTask) IsPaused() bool {
	select {
	case <-t.pauseCh:
		return true
	default:
		return false
	}
}

// Pause 暂停任务
func (t *AgentTask) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == StatusRunning {
		t.Status = StatusPaused
		select {
		case t.pauseCh <- struct{}{}:
		default:
		}
	}
}

// Resume 恢复任务
func (t *AgentTask) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == StatusPaused {
		t.Status = StatusRunning
		select {
		case <-t.pauseCh:
		default:
		}
	}
}

// Cancel 取消任务
func (t *AgentTask) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == StatusPending || t.Status == StatusRunning || t.Status == StatusPaused {
		t.Status = StatusCanceled
		close(t.cancelCh)
	}
}

// GetStatus 获取状态（线程安全）
func (t *AgentTask) GetStatus() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// GetProgress 获取进度（线程安全）
func (t *AgentTask) GetProgress() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Progress
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return time.Now().Format("20060102150405") + "_" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
