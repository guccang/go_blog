package taskbreakdown

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// TaskManager 任务管理器
type TaskManager struct {
	storage *TaskStorage
}

// NewTaskManager 创建新的任务管理器
func NewTaskManager() *TaskManager {
	return &TaskManager{
		storage: NewTaskStorage(),
	}
}

// CreateTask 创建新任务
func (tm *TaskManager) CreateTask(account string, req *TaskCreateRequest) (*ComplexTask, error) {
	// 验证请求
	if req.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	// 设置默认值
	if req.Status == "" {
		req.Status = StatusPlanning
	}
	if !IsValidStatus(req.Status) {
		return nil, fmt.Errorf("invalid task status: %s", req.Status)
	}

	if req.Priority == 0 {
		req.Priority = PriorityMedium
	}
	if !IsValidPriority(req.Priority) {
		return nil, fmt.Errorf("invalid task priority: %d", req.Priority)
	}

	// 生成任务ID
	taskID := GenerateTaskID()

	// 创建任务对象
	now := time.Now().Format(time.RFC3339)
	task := &ComplexTask{
		ID:            taskID,
		Title:         req.Title,
		Description:   req.Description,
		Status:        req.Status,
		Priority:      req.Priority,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		EstimatedTime: req.EstimatedTime,
		ActualTime:    0,
		Progress:      0,
		Subtasks:      []ComplexTask{},
		Dependencies:  []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
		ParentID:      req.ParentID,
		Order:         0, // 将在保存时计算
		Tags:          req.Tags,
		Deleted:       false,
	}

	// 如果指定了父任务，验证父任务存在
	if req.ParentID != "" {
		parentTask, err := tm.storage.GetTask(account, req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent task not found: %s", req.ParentID)
		}

		// 检查层级深度限制
		if parentTask.ExceedsMaxDepth(9) { // 父任务已经是9层，加上新任务就是10层
			return nil, fmt.Errorf("maximum task depth (10) exceeded")
		}
	}

	// 计算Order（如果是子任务，放在最后）
	if req.ParentID != "" {
		subtasks, err := tm.storage.GetTasksByParent(account, req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get subtasks: %w", err)
		}
		task.Order = len(subtasks)
	} else {
		// 根任务
		rootTasks, err := tm.storage.GetRootTasks(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get root tasks: %w", err)
		}
		task.Order = len(rootTasks)
	}

	// 保存任务
	if err := tm.storage.SaveTask(account, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	return task, nil
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(account, taskID string) (*ComplexTask, error) {
	task, err := tm.storage.GetTask(account, taskID)
	if err != nil {
		return nil, err
	}

	// 检查是否已删除
	if task.Deleted {
		return nil, fmt.Errorf("task has been deleted")
	}

	return task, nil
}

// UpdateTask 更新任务
func (tm *TaskManager) UpdateTask(account, taskID string, updates *TaskUpdateRequest) (*ComplexTask, error) {
	// 获取现有任务
	task, err := tm.storage.GetTask(account, taskID)
	if err != nil {
		return nil, err
	}

	// 检查是否已删除
	if task.Deleted {
		return nil, fmt.Errorf("cannot update deleted task")
	}

	// 应用更新
	updated := false

	if updates.Title != nil && *updates.Title != "" && *updates.Title != task.Title {
		task.Title = *updates.Title
		updated = true
	}

	if updates.Description != nil {
		task.Description = *updates.Description
		updated = true
	}

	if updates.Status != nil && *updates.Status != task.Status {
		if !IsValidStatus(*updates.Status) {
			return nil, fmt.Errorf("invalid task status: %s", *updates.Status)
		}
		task.Status = *updates.Status
		updated = true
	}

	if updates.Priority != nil && *updates.Priority != task.Priority {
		if !IsValidPriority(*updates.Priority) {
			return nil, fmt.Errorf("invalid task priority: %d", *updates.Priority)
		}
		task.Priority = *updates.Priority
		updated = true
	}

	if updates.StartDate != nil {
		task.StartDate = *updates.StartDate
		updated = true
	}

	if updates.EndDate != nil {
		task.EndDate = *updates.EndDate
		updated = true
	}

	if updates.EstimatedTime != nil {
		task.EstimatedTime = *updates.EstimatedTime
		updated = true
	}

	if updates.ActualTime != nil {
		task.ActualTime = *updates.ActualTime
		updated = true
	}

	if updates.Progress != nil && *updates.Progress != task.Progress {
		if *updates.Progress < 0 || *updates.Progress > 100 {
			return nil, fmt.Errorf("progress must be between 0 and 100")
		}
		task.Progress = *updates.Progress
		updated = true
	}

	if updates.ParentID != nil && *updates.ParentID != task.ParentID {
		// 验证新父任务存在
		if *updates.ParentID != "" {
			parentTask, err := tm.storage.GetTask(account, *updates.ParentID)
			if err != nil {
				return nil, fmt.Errorf("new parent task not found: %s", *updates.ParentID)
			}

			// 检查层级深度限制
			if parentTask.ExceedsMaxDepth(9) { // 父任务已经是9层，加上移动的任务就是10层
				return nil, fmt.Errorf("maximum task depth (10) exceeded")
			}
		}
		task.ParentID = *updates.ParentID
		updated = true
	}

	if updates.Order != nil && *updates.Order != task.Order {
		task.Order = *updates.Order
		updated = true
	}

	if updates.Tags != nil {
		task.Tags = *updates.Tags
		updated = true
	}

	// 如果没有更新，直接返回
	if !updated {
		return task, nil
	}

	// 更新更新时间
	task.UpdatedAt = time.Now().Format(time.RFC3339)

	// 保存更新后的任务
	if err := tm.storage.SaveTask(account, task); err != nil {
		return nil, fmt.Errorf("failed to save updated task: %w", err)
	}

	// 如果任务有父任务，更新父任务的进度
	if task.ParentID != "" {
		go func() {
			// 异步更新父任务进度
			if parentProgress, err := tm.CalculateOverallProgress(account, task.ParentID); err == nil {
				// 只更新进度，不改变其他字段
				progressUpdate := &TaskUpdateRequest{
					Progress: &parentProgress,
				}
				if _, updateErr := tm.UpdateTask(account, task.ParentID, progressUpdate); updateErr != nil {
					// 记录错误但不影响当前操作
					fmt.Printf("Failed to update parent task progress: %v\n", updateErr)
				}
			}
		}()
	}

	return task, nil
}

// DeleteTask 删除任务
func (tm *TaskManager) DeleteTask(account, taskID string) error {
	// 获取任务
	task, err := tm.storage.GetTask(account, taskID)
	if err != nil {
		return err
	}

	// 检查是否已删除
	if task.Deleted {
		return fmt.Errorf("task already deleted")
	}

	// 标记为删除
	task.Deleted = true
	task.Status = StatusCancelled
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	task.Title = fmt.Sprintf("[DELETED] %s", task.Title)

	// 保存标记删除的任务
	return tm.storage.SaveTask(account, task)
}

// ListTasks 列出所有任务
func (tm *TaskManager) ListTasks(account string) ([]*ComplexTask, error) {
	tasks, err := tm.storage.GetAllTasks(account)
	if err != nil {
		return nil, err
	}

	// 过滤掉已删除的任务
	var activeTasks []*ComplexTask
	for _, task := range tasks {
		if !task.Deleted {
			activeTasks = append(activeTasks, task)
		}
	}

	return activeTasks, nil
}

// GetTaskTree 获取任务树
func (tm *TaskManager) GetTaskTree(account, rootID string) (*ComplexTask, error) {
	// 获取根任务
	rootTask, err := tm.storage.GetTask(account, rootID)
	if err != nil {
		return nil, err
	}

	// 递归获取子任务
	return tm.buildTaskTree(account, rootTask)
}

// buildTaskTree 递归构建任务树
func (tm *TaskManager) buildTaskTree(account string, task *ComplexTask) (*ComplexTask, error) {
	// 获取子任务
	subtasks, err := tm.storage.GetTasksByParent(account, task.ID)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的子任务
	var activeSubtasks []*ComplexTask
	for _, subtask := range subtasks {
		if !subtask.Deleted {
			activeSubtasks = append(activeSubtasks, subtask)
		}
	}

	// 递归构建子任务树
	task.Subtasks = []ComplexTask{}
	for _, subtask := range activeSubtasks {
		subtree, err := tm.buildTaskTree(account, subtask)
		if err != nil {
			return nil, err
		}
		task.Subtasks = append(task.Subtasks, *subtree)
	}

	// 按Order排序子任务
	sort.Slice(task.Subtasks, func(i, j int) bool {
		return task.Subtasks[i].Order < task.Subtasks[j].Order
	})

	return task, nil
}

// GetRootTasks 获取根任务列表
func (tm *TaskManager) GetRootTasks(account string) ([]*ComplexTask, error) {
	tasks, err := tm.storage.GetRootTasks(account)
	if err != nil {
		return nil, err
	}

	// 过滤已删除的任务
	var activeTasks []*ComplexTask
	for _, task := range tasks {
		if !task.Deleted {
			activeTasks = append(activeTasks, task)
		}
	}

	return activeTasks, nil
}

// AddSubtask 添加子任务
func (tm *TaskManager) AddSubtask(account, parentID string, req *TaskCreateRequest) (*ComplexTask, error) {
	// 验证父任务存在
	parentTask, err := tm.storage.GetTask(account, parentID)
	if err != nil {
		return nil, fmt.Errorf("parent task not found: %s", parentID)
	}

	// 检查层级深度限制
	if parentTask.ExceedsMaxDepth(9) { // 父任务已经是9层，加上新子任务就是10层
		return nil, fmt.Errorf("maximum task depth (10) exceeded")
	}

	// 设置父任务ID
	req.ParentID = parentID

	// 创建子任务
	return tm.CreateTask(account, req)
}

// RemoveSubtask 移除子任务
func (tm *TaskManager) RemoveSubtask(account, parentID, subtaskID string) error {
	// 验证父任务存在
	_, err := tm.storage.GetTask(account, parentID)
	if err != nil {
		return fmt.Errorf("parent task not found: %s", parentID)
	}

	// 获取子任务
	subtask, err := tm.storage.GetTask(account, subtaskID)
	if err != nil {
		return fmt.Errorf("subtask not found: %s", subtaskID)
	}

	// 验证确实是子任务
	if subtask.ParentID != parentID {
		return fmt.Errorf("task %s is not a subtask of %s", subtaskID, parentID)
	}

	// 删除子任务
	return tm.DeleteTask(account, subtaskID)
}

// UpdateTaskOrder 更新任务顺序
func (tm *TaskManager) UpdateTaskOrder(account, taskID string, newOrder int) error {
	// 验证任务存在
	_, err := tm.storage.GetTask(account, taskID)
	if err != nil {
		return err
	}

	// 更新顺序
	updates := &TaskUpdateRequest{
		Order: &newOrder,
	}

	_, err = tm.UpdateTask(account, taskID, updates)
	return err
}

// CalculateOverallProgress 计算总体进度
func (tm *TaskManager) CalculateOverallProgress(account, taskID string) (int, error) {
	task, err := tm.GetTaskTree(account, taskID)
	if err != nil {
		return 0, err
	}

	return tm.calculateTaskProgress(task), nil
}

// calculateTaskProgress 递归计算任务进度
func (tm *TaskManager) calculateTaskProgress(task *ComplexTask) int {
	// 如果没有子任务，返回自己的进度
	if len(task.Subtasks) == 0 {
		return task.Progress
	}

	// 计算子任务进度平均值
	totalProgress := 0
	for _, subtask := range task.Subtasks {
		subtaskPtr := subtask // 创建指针副本
		totalProgress += tm.calculateTaskProgress(&subtaskPtr)
	}

	return totalProgress / len(task.Subtasks)
}

// GetTimelineData 获取时间线数据
func (tm *TaskManager) GetTimelineData(account string) (*TimelineData, error) {
	tasks, err := tm.ListTasks(account)
	if err != nil {
		return nil, err
	}

	var timelineTasks []TimelineTask
	for _, task := range tasks {
		// 只包含有开始和结束日期的任务
		if task.StartDate != "" && task.EndDate != "" {
			timelineTasks = append(timelineTasks, TimelineTask{
				ID:        task.ID,
				Title:     task.Title,
				StartDate: task.StartDate,
				EndDate:   task.EndDate,
				Progress:  task.Progress,
				Status:    task.Status,
				Priority:  task.Priority,
				ParentID:  task.ParentID,
			})
		}
	}

	// 排序：父任务优先，然后按开始时间，最后按结束时间
	sort.Slice(timelineTasks, func(i, j int) bool {
		// 1. 首先，父任务优先于子任务
		// 如果i是父任务（ParentID为空）而j是子任务，i应该排在前面
		if timelineTasks[i].ParentID == "" && timelineTasks[j].ParentID != "" {
			return true
		}
		// 如果i是子任务而j是父任务，j应该排在前面
		if timelineTasks[i].ParentID != "" && timelineTasks[j].ParentID == "" {
			return false
		}

		// 2. 都是父任务或都是子任务，按开始时间排序（最早的在前）
		if timelineTasks[i].StartDate != timelineTasks[j].StartDate {
			return timelineTasks[i].StartDate < timelineTasks[j].StartDate
		}

		// 3. 开始时间相同，按结束时间排序
		return timelineTasks[i].EndDate < timelineTasks[j].EndDate
	})

	return &TimelineData{Tasks: timelineTasks}, nil
}

// GetStatistics 获取统计信息
func (tm *TaskManager) GetStatistics(account string) (*StatisticsData, error) {
	tasks, err := tm.ListTasks(account)
	if err != nil {
		return nil, err
	}

	stats := &StatisticsData{
		TotalTasks:          len(tasks),
		CompletedTasks:      0,
		InProgressTasks:     0,
		BlockedTasks:        0,
		TotalTime:           0,
		StatusDistribution:  make(map[string]int),
		PriorityDistribution: make(map[int]int),
	}

	for _, task := range tasks {
		// 规范化状态值（转换为小写，去除空格）
		normalizedStatus := strings.ToLower(strings.TrimSpace(task.Status))

		// 统计状态
		stats.StatusDistribution[normalizedStatus]++

		// 统计特定状态数量
		switch normalizedStatus {
		case StatusCompleted:
			stats.CompletedTasks++
		case StatusInProgress:
			stats.InProgressTasks++
		case StatusBlocked:
			stats.BlockedTasks++
		}

		// 统计优先级
		stats.PriorityDistribution[task.Priority]++

		// 累加预估时间
		stats.TotalTime += task.EstimatedTime
	}

	return stats, nil
}

// SearchTasks 搜索任务
func (tm *TaskManager) SearchTasks(account, query string) ([]*ComplexTask, error) {
	tasks, err := tm.ListTasks(account)
	if err != nil {
		return nil, err
	}

	var results []*ComplexTask
	query = strings.ToLower(query)

	for _, task := range tasks {
		// 搜索标题和描述
		if strings.Contains(strings.ToLower(task.Title), query) ||
			strings.Contains(strings.ToLower(task.Description), query) {
			results = append(results, task)
			continue
		}

		// 搜索标签
		for _, tag := range task.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, task)
				break
			}
		}
	}

	return results, nil
}