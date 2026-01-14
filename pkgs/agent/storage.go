package agent

import (
	"control"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"sync"
)

// TaskStorage 任务存储（使用 blog 系统）
type TaskStorage struct {
	account string
	cache   map[string]*AgentTask
	mu      sync.RWMutex
}

// NewTaskStorage 创建任务存储
func NewTaskStorage(account string) *TaskStorage {
	storage := &TaskStorage{
		account: account,
		cache:   make(map[string]*AgentTask),
	}
	storage.loadAllTasks()
	return storage
}

// SaveTask 保存任务
func (s *TaskStorage) SaveTask(task *AgentTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新缓存
	s.cache[task.ID] = task

	// 保存到 blog
	title := s.getTaskBlogTitle(task.ID)
	content, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     fmt.Sprintf("agent|task|%s", task.Status),
		AuthType: module.EAuthType_private,
		Account:  s.account,
	}

	if control.GetBlog(s.account, title) == nil {
		control.AddBlog(s.account, ubd)
	} else {
		control.ModifyBlog(s.account, ubd)
	}

	log.DebugF(log.ModuleAgent, "Task saved: %s", task.ID)
	return nil
}

// GetTask 获取任务
func (s *TaskStorage) GetTask(taskID string) *AgentTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 先从缓存获取
	if task, ok := s.cache[taskID]; ok {
		return task
	}

	// 从 blog 加载
	title := s.getTaskBlogTitle(taskID)
	blog := control.GetBlog(s.account, title)
	if blog == nil {
		return nil
	}

	var task AgentTask
	if err := json.Unmarshal([]byte(blog.Content), &task); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse task: %v", err)
		return nil
	}

	// 重新初始化 channels
	task.pauseCh = make(chan struct{}, 1)
	task.cancelCh = make(chan struct{}, 1)

	return &task
}

// GetTasksByAccount 获取账户的所有任务
func (s *TaskStorage) GetTasksByAccount(account string) []*AgentTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*AgentTask
	for _, task := range s.cache {
		if task.Account == account {
			tasks = append(tasks, task)
		}
	}

	// 按创建时间倒序排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// DeleteTask 删除任务
func (s *TaskStorage) DeleteTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从缓存删除
	delete(s.cache, taskID)

	// 从 blog 删除
	title := s.getTaskBlogTitle(taskID)
	control.DeleteBlog(s.account, title)

	log.DebugF(log.ModuleAgent, "Task deleted: %s", taskID)
	return nil
}

// loadAllTasks 加载所有任务到缓存
func (s *TaskStorage) loadAllTasks() {
	blogs := control.GetAll(s.account, 0, module.EAuthType_all)
	for _, blog := range blogs {
		if strings.HasPrefix(blog.Title, "agent_task_") {
			var task AgentTask
			if err := json.Unmarshal([]byte(blog.Content), &task); err == nil {
				// 重新初始化 channels
				task.pauseCh = make(chan struct{}, 1)
				task.cancelCh = make(chan struct{}, 1)
				s.cache[task.ID] = &task
			}
		}
	}
	log.MessageF(log.ModuleAgent, "Loaded %d tasks from storage", len(s.cache))
}

// getTaskBlogTitle 获取任务的 blog 标题
func (s *TaskStorage) getTaskBlogTitle(taskID string) string {
	return fmt.Sprintf("agent_task_%s", taskID)
}

// GetPendingTasks 获取待执行的任务
func (s *TaskStorage) GetPendingTasks() []*AgentTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*AgentTask
	for _, task := range s.cache {
		if task.Status == StatusPending {
			tasks = append(tasks, task)
		}
	}

	// 按优先级排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority > tasks[j].Priority
	})

	return tasks
}
