package taskbreakdown

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	"sort"
	"strings"
	"time"
)

// TaskStorage 任务存储接口
type TaskStorage struct {
	// 使用现有的blog系统进行存储
}

// NewTaskStorage 创建新的任务存储实例
func NewTaskStorage() *TaskStorage {
	return &TaskStorage{}
}

// 生成任务博客标题
func generateTaskBlogTitle(taskID string) string {
	return fmt.Sprintf("taskbreakdown-%s", taskID)
}

// 生成任务索引博客标题
func generateIndexBlogTitle() string {
	return "taskbreakdown-index"
}

// 从博客标题提取任务ID
func extractTaskIDFromTitle(title string) string {
	if strings.HasPrefix(title, "taskbreakdown-") {
		return strings.TrimPrefix(title, "taskbreakdown-")
	}
	return ""
}

// SaveTask 保存任务到存储
func (ts *TaskStorage) SaveTask(account string, task *ComplexTask) error {
	// 检查循环引用：如果任务的parent_id等于自身id，这是一个错误状态
	// 为了安全，我们清空parent_id并记录错误
	if task.ParentID == task.ID {
		fmt.Printf("WARNING: Task %s has parent_id equal to its own id, clearing parent_id before saving\n", task.ID)
		task.ParentID = ""
	}

	// 生成博客标题
	title := generateTaskBlogTitle(task.ID)

	// 转换为JSON
	content, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert task to JSON: %w", err)
	}

	// 创建或更新博客
	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     "taskbreakdown",
		AuthType: module.EAuthType_private, // 私有任务
		Account:  account,
	}

	// 检查是否已存在
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		// 创建新博客
		blog.AddBlogWithAccount(account, ubd)
	} else {
		// 更新现有博客
		blog.ModifyBlogWithAccount(account, ubd)
	}

	// 更新索引
	return ts.updateTaskIndex(account, task)
}

// GetTask 从存储获取任务
func (ts *TaskStorage) GetTask(account, taskID string) (*ComplexTask, error) {
	title := generateTaskBlogTitle(taskID)
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// 解析JSON内容
	var task ComplexTask
	if err := json.Unmarshal([]byte(b.Content), &task); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}

	return &task, nil
}

// DeleteTask 从存储删除任务
func (ts *TaskStorage) DeleteTask(account, taskID string) error {
	// 注意：blog包没有直接的删除方法
	// 我们可以将内容清空或标记为删除
	// 这里我们采用标记删除的方式

	title := generateTaskBlogTitle(taskID)
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// 创建标记为删除的任务
	deletedTask := &ComplexTask{
		ID:        taskID,
		Title:     fmt.Sprintf("[DELETED] %s", b.Title),
		Status:    StatusCancelled,
		Deleted:   true,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	// 保存标记删除的任务
	return ts.SaveTask(account, deletedTask)
}

// GetAllTasks 获取所有任务
func (ts *TaskStorage) GetAllTasks(account string) ([]*ComplexTask, error) {
	// 获取所有博客
	blogs := blog.GetBlogsWithAccount(account)
	if blogs == nil {
		return []*ComplexTask{}, nil
	}

	var tasks []*ComplexTask
	for title, b := range blogs {
		// 只处理taskbreakdown开头的博客
		if !strings.HasPrefix(title, "taskbreakdown-") {
			continue
		}

		// 跳过索引博客
		if title == "taskbreakdown-index" {
			continue
		}

		// 解析任务
		var task ComplexTask
		if err := json.Unmarshal([]byte(b.Content), &task); err != nil {
			// 解析失败，跳过
			continue
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// GetTaskIndex 获取任务索引
func (ts *TaskStorage) GetTaskIndex(account string) ([]TaskIndex, error) {
	title := generateIndexBlogTitle()
	b := blog.GetBlogWithAccount(account, title)
	if b == nil {
		// 索引不存在，返回空数组
		return []TaskIndex{}, nil
	}

	// 解析索引
	var index []TaskIndex
	if err := json.Unmarshal([]byte(b.Content), &index); err != nil {
		return nil, fmt.Errorf("failed to parse task index: %w", err)
	}

	return index, nil
}

// updateTaskIndex 更新任务索引
func (ts *TaskStorage) updateTaskIndex(account string, task *ComplexTask) error {
	// 获取现有索引
	index, err := ts.GetTaskIndex(account)
	if err != nil {
		return err
	}

	// 如果任务已删除，从索引中移除
	if task.Deleted {
		// 从索引中移除已删除的任务
		var newIndex []TaskIndex
		for _, item := range index {
			if item.ID != task.ID {
				newIndex = append(newIndex, item)
			}
		}
		index = newIndex
	} else {
		// 创建或更新索引项
		taskIndex := TaskIndex{
			ID:        task.ID,
			Title:     task.Title,
			Status:    task.Status,
			Priority:  task.Priority,
			Progress:  task.Progress,
			StartDate: task.StartDate,
			EndDate:   task.EndDate,
			ParentID:  task.ParentID,
			Order:     task.Order,
		}

		// 查找是否已存在
		found := false
		for i, item := range index {
			if item.ID == task.ID {
				index[i] = taskIndex
				found = true
				break
			}
		}

		// 如果不存在，添加到索引
		if !found {
			index = append(index, taskIndex)
		}
	}

	// 按Order排序
	sort.Slice(index, func(i, j int) bool {
		return index[i].Order < index[j].Order
	})

	// 保存索引
	content, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert index to JSON: %w", err)
	}

	ubd := &module.UploadedBlogData{
		Title:    generateIndexBlogTitle(),
		Content:  string(content),
		Tags:     "taskbreakdown-index",
		AuthType: module.EAuthType_private,
		Account:  account,
	}

	// 保存索引
	b := blog.GetBlogWithAccount(account, generateIndexBlogTitle())
	if b == nil {
		blog.AddBlogWithAccount(account, ubd)
	} else {
		blog.ModifyBlogWithAccount(account, ubd)
	}

	return nil
}

// removeFromTaskIndex 从索引中移除任务
func (ts *TaskStorage) removeFromTaskIndex(account, taskID string) error {
	index, err := ts.GetTaskIndex(account)
	if err != nil {
		return err
	}

	// 过滤掉要删除的任务
	var newIndex []TaskIndex
	for _, item := range index {
		if item.ID != taskID {
			newIndex = append(newIndex, item)
		}
	}

	// 保存更新后的索引
	content, err := json.MarshalIndent(newIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert index to JSON: %w", err)
	}

	ubd := &module.UploadedBlogData{
		Title:    generateIndexBlogTitle(),
		Content:  string(content),
		Tags:     "taskbreakdown-index",
		AuthType: module.EAuthType_private,
		Account:  account,
	}

	b := blog.GetBlogWithAccount(account, generateIndexBlogTitle())
	if b == nil {
		blog.AddBlogWithAccount(account, ubd)
	} else {
		blog.ModifyBlogWithAccount(account, ubd)
	}

	return nil
}

// GetTasksByParent 根据父任务ID获取子任务
func (ts *TaskStorage) GetTasksByParent(account, parentID string) ([]*ComplexTask, error) {
	allTasks, err := ts.GetAllTasks(account)
	if err != nil {
		return nil, err
	}

	var subtasks []*ComplexTask
	for _, task := range allTasks {
		if task.ParentID == parentID {
			subtasks = append(subtasks, task)
		}
	}

	// 按Order排序
	sort.Slice(subtasks, func(i, j int) bool {
		return subtasks[i].Order < subtasks[j].Order
	})

	return subtasks, nil
}

// GetRootTasks 获取根任务(没有父任务的任务)
func (ts *TaskStorage) GetRootTasks(account string) ([]*ComplexTask, error) {
	allTasks, err := ts.GetAllTasks(account)
	if err != nil {
		return nil, err
	}

	var rootTasks []*ComplexTask
	for _, task := range allTasks {
		// 只显示未完成的根任务（状态不是completed或cancelled，且进度小于100，且未删除）
		if task.ParentID == "" && task.Status != "completed" && task.Status != "cancelled" && task.Progress < 100 && !task.Deleted {
			rootTasks = append(rootTasks, task)
		}
	}

	// 按Order排序
	sort.Slice(rootTasks, func(i, j int) bool {
		return rootTasks[i].Order < rootTasks[j].Order
	})

	return rootTasks, nil
}

// GetCompletedRootTasks 获取已完成的根任务
func (ts *TaskStorage) GetCompletedRootTasks(account string) ([]*ComplexTask, error) {
	allTasks, err := ts.GetAllTasks(account)
	if err != nil {
		return nil, err
	}

	var completedRootTasks []*ComplexTask
	for _, task := range allTasks {
		if task.ParentID == "" && (task.Status == "completed" || task.Progress == 100) && !task.Deleted {
			completedRootTasks = append(completedRootTasks, task)
		}
	}

	// 按更新时间倒序排序，最新的在前面
	sort.Slice(completedRootTasks, func(i, j int) bool {
		return completedRootTasks[i].UpdatedAt > completedRootTasks[j].UpdatedAt
	})

	return completedRootTasks, nil
}