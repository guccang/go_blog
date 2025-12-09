package taskbreakdown

import (
	"fmt"
	"sort"
	"strings"
	"time"

	log "mylog"
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
		DailyTime:     req.DailyTime,
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
		// 检查父任务ID是否等于任务自身ID（防止循环引用）
		if req.ParentID == taskID {
			return nil, fmt.Errorf("task cannot be its own parent")
		}

		parentTask, err := tm.GetTask(account, req.ParentID)
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
		// 过滤已删除的子任务
		var activeSubtasks []*ComplexTask
		for _, subtask := range subtasks {
			if !subtask.Deleted {
				activeSubtasks = append(activeSubtasks, subtask)
			}
		}
		task.Order = len(activeSubtasks)
	} else {
		// 根任务
		rootTasks, err := tm.GetRootTasks(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get root tasks: %w", err)
		}
		task.Order = len(rootTasks)
	}

	// 自动计算预估时间（如果预估时间为0但每天分配时间和日期范围有效）
	if task.EstimatedTime == 0 && task.DailyTime > 0 && task.StartDate != "" && task.EndDate != "" {
		log.DebugF(log.ModuleTaskBreakdown, "任务 %s 预估时间为0，开始自动计算预估时间", task.ID)
		log.DebugF(log.ModuleTaskBreakdown, "开始日期: %s, 结束日期: %s, 每天分配时间: %d分钟", task.StartDate, task.EndDate, task.DailyTime)
		calculatedEstimatedTime := calculateEstimatedTimeFromDates(task.StartDate, task.EndDate, task.DailyTime)
		log.DebugF(log.ModuleTaskBreakdown, "计算得到的预估时间: %d分钟", calculatedEstimatedTime)
		if calculatedEstimatedTime > 0 {
			task.EstimatedTime = calculatedEstimatedTime
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的预估时间已设置为 %d分钟", task.ID, task.EstimatedTime)
		}
	}

	// 如果创建时状态就是已完成，自动计算实际时间
	if task.Status == StatusCompleted {
		log.DebugF(log.ModuleTaskBreakdown, "任务 %s 创建时状态为已完成，开始自动计算实际时间", task.ID)
		log.DebugF(log.ModuleTaskBreakdown, "开始日期: %s, 结束日期: %s, 每天分配时间: %d分钟", task.StartDate, task.EndDate, task.DailyTime)
		calculatedActualTime := calculateActualTimeSmart(task)
		log.DebugF(log.ModuleTaskBreakdown, "计算得到的实际时间: %d分钟", calculatedActualTime)
		if calculatedActualTime > 0 {
			task.ActualTime = calculatedActualTime
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的实际时间已设置为 %d分钟", task.ID, task.ActualTime)
		}
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

	if updates.DailyTime != nil {
		task.DailyTime = *updates.DailyTime
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
			// 检查父任务ID是否等于任务自身ID（防止循环引用）
			if *updates.ParentID == task.ID {
				return nil, fmt.Errorf("task cannot be its own parent")
			}

			parentTask, err := tm.GetTask(account, *updates.ParentID)
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

	// 自动计算预估时间（如果用户未手动设置预估时间，且相关字段有效）
	if updates.EstimatedTime == nil {
		// 检查是否需要重新计算预估时间
		shouldRecalculate := task.EstimatedTime == 0 || updates.DailyTime != nil || updates.StartDate != nil || updates.EndDate != nil

		if shouldRecalculate && task.DailyTime > 0 && task.StartDate != "" && task.EndDate != "" {
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 需要重新计算预估时间", task.ID)
			log.DebugF(log.ModuleTaskBreakdown, "开始日期: %s, 结束日期: %s, 每天分配时间: %d分钟", task.StartDate, task.EndDate, task.DailyTime)
			calculatedEstimatedTime := calculateEstimatedTimeFromDates(task.StartDate, task.EndDate, task.DailyTime)
			log.DebugF(log.ModuleTaskBreakdown, "计算得到的预估时间: %d分钟", calculatedEstimatedTime)
			if calculatedEstimatedTime > 0 {
				task.EstimatedTime = calculatedEstimatedTime
				log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的预估时间已更新为 %d分钟", task.ID, task.EstimatedTime)
			} else {
				log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的预估时间计算失败或为0，保持原值: %d分钟", task.ID, task.EstimatedTime)
			}
		} else {
			if shouldRecalculate {
				log.DebugF(log.ModuleTaskBreakdown, "任务 %s 需要重新计算预估时间，但计算条件不满足: dailyTime=%d, startDate=%q, endDate=%q", task.ID, task.DailyTime, task.StartDate, task.EndDate)
			}
		}
	}

	// 自动计算实际时间（如果任务完成且用户未手动设置实际时间）
	if task.Status == StatusCompleted && updates.ActualTime == nil {
		log.DebugF(log.ModuleTaskBreakdown, "任务 %s 标记为完成，开始自动计算实际时间", task.ID)
		log.DebugF(log.ModuleTaskBreakdown, "开始日期: %s, 结束日期: %s, 每天分配时间: %d分钟", task.StartDate, task.EndDate, task.DailyTime)
		// 计算实际时间（智能计算，处理dailyTime <= 0的情况）
		calculatedActualTime := calculateActualTimeSmart(task)
		log.DebugF(log.ModuleTaskBreakdown, "计算得到的实际时间: %d分钟", calculatedActualTime)
		if calculatedActualTime > 0 {
			task.ActualTime = calculatedActualTime
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的实际时间已更新为 %d分钟", task.ID, task.ActualTime)
		} else {
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 的实际时间计算失败或为0，保持原值: %d分钟", task.ID, task.ActualTime)
		}
	} else {
		if task.Status == StatusCompleted {
			log.DebugF(log.ModuleTaskBreakdown, "任务 %s 标记为完成，但updates.ActualTime不为nil，跳过自动计算", task.ID)
		}
	}

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
	if err := tm.storage.SaveTask(account, task); err != nil {
		return err
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

	return nil
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
	return tm.buildTaskTreeWithVisited(account, task, make(map[string]bool))
}

// buildTaskTreeWithVisited 递归构建任务树，带已访问集合防止循环引用
func (tm *TaskManager) buildTaskTreeWithVisited(account string, task *ComplexTask, visited map[string]bool) (*ComplexTask, error) {
	// 检查循环引用：如果任务的parent_id等于自身id，这是一个错误状态
	// 为了安全，我们清空parent_id并记录错误
	if task.ParentID == task.ID {
		fmt.Printf("WARNING: Task %s has parent_id equal to its own id, clearing parent_id to prevent infinite recursion\n", task.ID)
		task.ParentID = ""
	}

	// 检查是否已访问过此任务（防止循环引用）
	if visited[task.ID] {
		fmt.Printf("WARNING: Detected circular reference at task %s, breaking the cycle\n", task.ID)
		return task, nil // 返回当前任务但不继续递归
	}
	visited[task.ID] = true

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
		// 创建新的visited map副本，每个分支独立
		branchVisited := make(map[string]bool)
		for k, v := range visited {
			branchVisited[k] = v
		}
		subtree, err := tm.buildTaskTreeWithVisited(account, subtask, branchVisited)
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

// GetCompletedRootTasks 获取已完成的根任务
func (tm *TaskManager) GetCompletedRootTasks(account string) ([]*ComplexTask, error) {
	tasks, err := tm.storage.GetCompletedRootTasks(account)
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
	parentTask, err := tm.GetTask(account, parentID)
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
func (tm *TaskManager) GetTimelineData(account, rootID string) (*TimelineData, error) {
	var tasks []*ComplexTask
	var err error

	if rootID == "" {
		// 获取所有任务
		tasks, err = tm.ListTasks(account)
		if err != nil {
			return nil, err
		}
	} else {
		// 获取指定根任务的子树
		rootTask, getErr := tm.GetTaskTree(account, rootID)
		if getErr != nil {
			return nil, getErr
		}
		// 扁平化任务树
		tasks = tm.flattenTaskTree(rootTask, []*ComplexTask{})
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

// calculateDaysBetween 计算两个日期之间的天数差
func calculateDaysBetween(startDateStr, endDateStr string) (int, error) {
	if startDateStr == "" || endDateStr == "" {
		log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 空日期 startDateStr=%q, endDateStr=%q", startDateStr, endDateStr)
		return 0, nil
	}

	// 解析日期
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 开始日期解析失败 startDateStr=%q, err=%v", startDateStr, err)
		return 0, fmt.Errorf("invalid start date format: %s", startDateStr)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 结束日期解析失败 endDateStr=%q, err=%v", endDateStr, err)
		return 0, fmt.Errorf("invalid end date format: %s", endDateStr)
	}

	// 计算天数差（包含开始和结束日）
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 计算天数 startDate=%v, endDate=%v, rawDays=%d", startDate, endDate, days)

	// 确保天数至少为1
	if days < 1 {
		days = 1
		log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 天数小于1，调整为1")
	}

	log.DebugF(log.ModuleTaskBreakdown, "calculateDaysBetween: 最终天数=%d", days)
	return days, nil
}

// GetStatistics 获取统计信息
func (tm *TaskManager) GetStatistics(account, rootID string) (*StatisticsData, error) {
	var tasks []*ComplexTask
	var err error

	if rootID == "" {
		// 获取所有任务
		tasks, err = tm.ListTasks(account)
		if err != nil {
			return nil, err
		}
	} else {
		// 获取指定根任务的子树
		rootTask, getErr := tm.GetTaskTree(account, rootID)
		if getErr != nil {
			return nil, getErr
		}
		// 扁平化任务树
		tasks = tm.flattenTaskTree(rootTask, []*ComplexTask{})
	}

	// 计算总预估时间（基于根任务的开始和结束时间）
	totalTime := 0
	if rootID != "" {
		// 如果有指定根任务，使用根任务的时间
		rootTask, err := tm.GetTask(account, rootID)
		if err == nil && rootTask.StartDate != "" && rootTask.EndDate != "" {
			days, err := calculateDaysBetween(rootTask.StartDate, rootTask.EndDate)
			if err == nil && days > 0 {
				// 假设每天工作8小时，转换为分钟
				totalTime = days * 8 * 60
			}
		}
	} else {
		// 如果没有指定根任务，计算所有根任务的时间
		rootTasks, err := tm.GetRootTasks(account)
		if err == nil {
			for _, rootTask := range rootTasks {
				if rootTask.StartDate != "" && rootTask.EndDate != "" {
					days, err := calculateDaysBetween(rootTask.StartDate, rootTask.EndDate)
					if err == nil && days > 0 {
						// 假设每天工作8小时，转换为分钟
						totalTime += days * 8 * 60
					}
				}
			}
		}
	}

	// 计算时间分析数据（只计入未删除且未完成的任务）
	totalEstimatedTime := 0
	totalDailyTime := 0
	for _, task := range tasks {
		// 跳过已删除或已完成的任务
		if !ShouldIncludeInTimeCalculation(task) {
			continue
		}
		totalEstimatedTime += task.EstimatedTime
		totalDailyTime += task.DailyTime
	}

	// 默认每天可用时间：14小时 = 840分钟
	dailyAvailableTime := 840
	requiredDays := 0.0
	timeMargin := 0
	timeUtilization := 0.0
	timeStatus := "sufficient"

	if dailyAvailableTime > 0 {
		requiredDays = float64(totalEstimatedTime) / float64(dailyAvailableTime)
		timeMargin = totalTime - totalEstimatedTime
		if totalTime > 0 {
			timeUtilization = float64(totalEstimatedTime) / float64(totalTime) * 100
		}

		// 判断时间状态
		if timeMargin < 0 {
			timeStatus = "insufficient"
		} else if timeMargin < dailyAvailableTime { // 余量小于一天
			timeStatus = "warning"
		}
	}

	stats := &StatisticsData{
		TotalTasks:          len(tasks),
		CompletedTasks:      0,
		InProgressTasks:     0,
		BlockedTasks:        0,
		TotalTime:           totalTime,
		StatusDistribution:  make(map[string]int),
		PriorityDistribution: make(map[int]int),
		// 时间分析字段
		DailyAvailableTime: dailyAvailableTime,
		TotalDailyTime:     totalDailyTime,
		RequiredDays:       requiredDays,
		TimeMargin:         timeMargin,
		TimeUtilization:    timeUtilization,
		TimeStatus:         timeStatus,
	}

	for _, task := range tasks {
		// 规范化状态值（转换为小写，去除空格）
		normalizedStatus := strings.ToLower(strings.TrimSpace(task.Status))

		// 统计状态
		stats.StatusDistribution[normalizedStatus]++

		// 统计特定状态数量
		// 首先检查进度是否为100%，如果是则计入已完成
		if task.Progress == 100 {
			stats.CompletedTasks++
		} else {
			// 否则按状态统计
			switch normalizedStatus {
			case StatusCompleted:
				stats.CompletedTasks++
			case StatusInProgress, StatusPlanning:
				// 将planning状态也计入进行中
				stats.InProgressTasks++
			case StatusBlocked:
				stats.BlockedTasks++
			}
		}

		// 统计优先级
		stats.PriorityDistribution[task.Priority]++
	}

	return stats, nil
}

// GetTaskGraph 获取任务网络图数据
func (tm *TaskManager) GetTaskGraph(account, rootID string) (*GraphData, error) {
	tasks, err := tm.ListTasks(account)
	if err != nil {
		return nil, err
	}

	// 如果有根任务ID，只获取该子树
	var rootTask *ComplexTask
	if rootID != "" {
		rootTask, err = tm.GetTaskTree(account, rootID)
		if err != nil {
			return nil, err
		}
		// 将树扁平化
		tasks = []*ComplexTask{rootTask}
		// 递归获取所有子任务（通过递归遍历）
		tasks = tm.flattenTaskTree(rootTask, tasks)
	}

	// 构建节点和边
	nodes := []GraphNode{}
	edges := []GraphEdge{}

	// 用于跟踪已添加的节点和边，避免重复
	nodeMap := make(map[string]bool)
	edgeMap := make(map[string]bool)

	// 递归遍历任务树构建图
	var buildGraph func(task *ComplexTask, level int)
	buildGraph = func(task *ComplexTask, level int) {
		if task == nil {
			return
		}

		// 添加节点
		nodeID := task.ID
		if !nodeMap[nodeID] {
			// 确定分组（基于状态）
			group := task.Status
			if group == "" {
				group = "planning"
			}

			// 节点大小基于优先级（优先级越高，值越大）
			value := 10
			if task.Priority >= 1 && task.Priority <= 5 {
				value = 15 - task.Priority * 2 // 优先级1 -> 13, 优先级5 -> 5
			}

			node := GraphNode{
				ID:       nodeID,
				Label:    task.Title,
				Title:    task.Title,
				Status:   task.Status,
				Priority: task.Priority,
				Progress: task.Progress,
				Level:    level,
				Group:    group,
				Value:    value,
			}
			nodes = append(nodes, node)
			nodeMap[nodeID] = true
		}

		// 处理子任务边
		for i := range task.Subtasks {
			subtask := &task.Subtasks[i]
			subtaskID := subtask.ID
			edgeKey := nodeID + "->" + subtaskID
			if !edgeMap[edgeKey] {
				edge := GraphEdge{
					From:   nodeID,
					To:     subtaskID,
					Type:   "parent-child",
					Arrows: "to",
					Dashes: false,
				}
				edges = append(edges, edge)
				edgeMap[edgeKey] = true
			}
			// 递归处理子任务
			buildGraph(subtask, level+1)
		}

		// 处理依赖关系边
		for _, depID := range task.Dependencies {
			edgeKey := nodeID + "->" + depID
			if !edgeMap[edgeKey] {
				edge := GraphEdge{
					From:   nodeID,
					To:     depID,
					Type:   "dependency",
					Arrows: "to",
					Dashes: true,
				}
				edges = append(edges, edge)
				edgeMap[edgeKey] = true
			}
		}
	}

	// 如果有根任务，从根开始构建；否则从所有根任务开始
	if rootTask != nil {
		// 检查是否有子任务但Subtasks字段为空的情况
		// 如果flattenTaskTree收集了多个任务，但rootTask.Subtasks为空，说明Subtasks字段未正确填充
		// 这种情况下，我们直接使用扁平化的任务列表构建图
		if len(rootTask.Subtasks) == 0 && len(tasks) > 1 {
			// 使用扁平化的任务列表构建图（基于parent_id）

			// 构建任务映射和父子关系映射
			taskMap := make(map[string]*ComplexTask)
			parentToChildren := make(map[string][]string)

			for _, task := range tasks {
				taskMap[task.ID] = task
				if task.ParentID != "" {
					parentToChildren[task.ParentID] = append(parentToChildren[task.ParentID], task.ID)
				}
			}

			// 计算节点层级
			levelMap := make(map[string]int)
			var calculateLevel func(taskID string) int
			calculateLevel = func(taskID string) int {
				if level, exists := levelMap[taskID]; exists {
					return level
				}

				task, exists := taskMap[taskID]
				if !exists {
					levelMap[taskID] = 0
					return 0
				}

				if task.ParentID == "" {
					levelMap[taskID] = 0
					return 0
				}

				// 递归计算父节点层级
				parentLevel := calculateLevel(task.ParentID)
				level := parentLevel + 1
				levelMap[taskID] = level
				return level
			}

			// 计算所有任务的层级
			for _, task := range tasks {
				calculateLevel(task.ID)
			}

			// 添加节点和边
			for _, task := range tasks {
				// 添加节点
				nodeID := task.ID
				if !nodeMap[nodeID] {
					// 确定分组（基于状态）
					group := task.Status
					if group == "" {
						group = "planning"
					}
					// 节点大小基于优先级（优先级越高，值越大）
					value := 10
					if task.Priority >= 1 && task.Priority <= 5 {
						value = 15 - task.Priority * 2 // 优先级1 -> 13, 优先级5 -> 5
					}

					// 获取计算好的层级
					level := levelMap[nodeID]

					node := GraphNode{
						ID:       nodeID,
						Label:    task.Title,
						Title:    task.Title,
						Status:   task.Status,
						Priority: task.Priority,
						Progress: task.Progress,
						Level:    level,
						Group:    group,
						Value:    value,
					}
					nodes = append(nodes, node)
					nodeMap[nodeID] = true
				}

				// 根据ParentID添加边
				if task.ParentID != "" {
					edgeKey := task.ParentID + "->" + nodeID
					if !edgeMap[edgeKey] {
						edge := GraphEdge{
							From:   task.ParentID,
							To:     nodeID,
							Type:   "parent-child",
							Arrows: "to",
							Dashes: false,
						}
						edges = append(edges, edge)
						edgeMap[edgeKey] = true
					}
				}

				// 添加依赖关系边
				for _, depID := range task.Dependencies {
					edgeKey := nodeID + "->" + depID
					if !edgeMap[edgeKey] {
						edge := GraphEdge{
							From:   nodeID,
							To:     depID,
							Type:   "dependency",
							Arrows: "to",
							Dashes: true,
						}
						edges = append(edges, edge)
						edgeMap[edgeKey] = true
					}
				}
			}
		} else {
			// 正常递归构建
			buildGraph(rootTask, 0)
		}
	} else {
		// 检查是否有任务具有Subtasks数据
		hasSubtasksData := false
		for _, task := range tasks {
			if len(task.Subtasks) > 0 {
				hasSubtasksData = true
				break
			}
		}

		if hasSubtasksData {
			// 使用Subtasks递归构建
			for _, task := range tasks {
				if task.ParentID == "" {
					buildGraph(task, 0)
				}
			}
		} else {
			// 使用扁平化的任务列表直接构建图（基于parent_id）

			// 构建任务映射和父子关系映射
			taskMap := make(map[string]*ComplexTask)
			parentToChildren := make(map[string][]string)

			for _, task := range tasks {
				taskMap[task.ID] = task
				if task.ParentID != "" {
					parentToChildren[task.ParentID] = append(parentToChildren[task.ParentID], task.ID)
				}
			}

			// 计算节点层级
			levelMap := make(map[string]int)
			var calculateLevel func(taskID string) int
			calculateLevel = func(taskID string) int {
				if level, exists := levelMap[taskID]; exists {
					return level
				}

				task, exists := taskMap[taskID]
				if !exists {
					levelMap[taskID] = 0
					return 0
				}

				if task.ParentID == "" {
					levelMap[taskID] = 0
					return 0
				}

				// 递归计算父节点层级
				parentLevel := calculateLevel(task.ParentID)
				level := parentLevel + 1
				levelMap[taskID] = level
				return level
			}

			// 计算所有任务的层级
			for _, task := range tasks {
				calculateLevel(task.ID)
			}

			// 添加节点和边
			for _, task := range tasks {
				// 添加节点
				nodeID := task.ID
				if !nodeMap[nodeID] {
					// 确定分组（基于状态）
					group := task.Status
					if group == "" {
						group = "planning"
					}
					// 节点大小基于优先级（优先级越高，值越大）
					value := 10
					if task.Priority >= 1 && task.Priority <= 5 {
						value = 15 - task.Priority * 2 // 优先级1 -> 13, 优先级5 -> 5
					}

					// 获取计算好的层级
					level := levelMap[nodeID]

					node := GraphNode{
						ID:       nodeID,
						Label:    task.Title,
						Title:    task.Title,
						Status:   task.Status,
						Priority: task.Priority,
						Progress: task.Progress,
						Level:    level,
						Group:    group,
						Value:    value,
					}
					nodes = append(nodes, node)
					nodeMap[nodeID] = true
				}

				// 根据ParentID添加边
				if task.ParentID != "" {
					edgeKey := task.ParentID + "->" + nodeID
					if !edgeMap[edgeKey] {
						edge := GraphEdge{
							From:   task.ParentID,
							To:     nodeID,
							Type:   "parent-child",
							Arrows: "to",
							Dashes: false,
						}
						edges = append(edges, edge)
						edgeMap[edgeKey] = true
					}
				}

				// 添加依赖关系边
				for _, depID := range task.Dependencies {
					edgeKey := nodeID + "->" + depID
					if !edgeMap[edgeKey] {
						edge := GraphEdge{
							From:   nodeID,
							To:     depID,
							Type:   "dependency",
							Arrows: "to",
							Dashes: true,
						}
						edges = append(edges, edge)
						edgeMap[edgeKey] = true
					}
				}
			}
		}
	}

	return &GraphData{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// GetTimeTrends 获取时间趋势数据
func (tm *TaskManager) GetTimeTrends(account, rootID, timeRange string) (*TimeTrendsResponse, error) {
	var tasks []*ComplexTask
	var err error

	if rootID == "" {
		// 获取所有任务
		tasks, err = tm.ListTasks(account)
		if err != nil {
			return nil, err
		}
	} else {
		// 获取指定根任务的子树
		rootTask, getErr := tm.GetTaskTree(account, rootID)
		if getErr != nil {
			return nil, getErr
		}
		// 扁平化子树
		tasks = tm.flattenTaskTree(rootTask, []*ComplexTask{})
	}

	// 确定时间范围
	var startDate, endDate time.Time
	now := time.Now()
	endDate = now

	switch timeRange {
	case "7d":
		startDate = now.AddDate(0, 0, -7)
	case "30d":
		startDate = now.AddDate(0, 0, -30)
	case "90d":
		startDate = now.AddDate(0, 0, -90)
	case "1y":
		startDate = now.AddDate(-1, 0, 0)
	default:
		// 默认30天
		startDate = now.AddDate(0, 0, -30)
		timeRange = "30d"
	}

	// 初始化数据点映射
	creationMap := make(map[string]int)
	completionMap := make(map[string]int)
	progressMap := make(map[string]int)
	progressCountMap := make(map[string]int)

	// 填充日期范围（确保所有日期都有数据点）
	current := startDate
	for !current.After(endDate) {
		dateStr := current.Format("2006-01-02")
		creationMap[dateStr] = 0
		completionMap[dateStr] = 0
		progressMap[dateStr] = 0
		progressCountMap[dateStr] = 0
		current = current.AddDate(0, 0, 1)
	}

	// 分析任务数据
	for _, task := range tasks {
		// 解析创建日期
		createdAt, err := time.Parse(time.RFC3339, task.CreatedAt)
		if err == nil && !createdAt.Before(startDate) && !createdAt.After(endDate) {
			dateStr := createdAt.Format("2006-01-02")
			creationMap[dateStr]++
		}

		// 解析完成日期（状态为completed且更新时间在范围内）
		if task.Status == StatusCompleted {
			updatedAt, err := time.Parse(time.RFC3339, task.UpdatedAt)
			if err == nil && !updatedAt.Before(startDate) && !updatedAt.After(endDate) {
				dateStr := updatedAt.Format("2006-01-02")
				completionMap[dateStr]++
			}
		}

		// 进度数据（基于更新日期）
		updatedAt, err := time.Parse(time.RFC3339, task.UpdatedAt)
		if err == nil && !updatedAt.Before(startDate) && !updatedAt.After(endDate) {
			dateStr := updatedAt.Format("2006-01-02")
			progressMap[dateStr] += task.Progress
			progressCountMap[dateStr]++
		}
	}

	// 计算平均进度
	for date := range progressMap {
		if progressCountMap[date] > 0 {
			progressMap[date] = progressMap[date] / progressCountMap[date]
		}
	}

	// 转换为排序的数据点
	creationPoints := mapToTimePoints(creationMap)
	completionPoints := mapToTimePoints(completionMap)
	progressPoints := mapToTimePoints(progressMap)

	// 计算统计信息
	creationTrend := calculateTrendData(creationPoints, "任务创建趋势", "个")
	completionTrend := calculateTrendData(completionPoints, "任务完成趋势", "个")
	progressTrend := calculateTrendData(progressPoints, "平均进度趋势", "%")

	return &TimeTrendsResponse{
		CreationTrend:   creationTrend,
		CompletionTrend: completionTrend,
		ProgressTrend:   progressTrend,
		TimeRange:       timeRange,
		StartDate:       startDate.Format("2006-01-02"),
		EndDate:         endDate.Format("2006-01-02"),
	}, nil
}

// mapToTimePoints 将映射转换为排序的时间点
func mapToTimePoints(data map[string]int) []TimePoint {
	// 提取日期并排序
	dates := make([]string, 0, len(data))
	for date := range data {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// 构建时间点
	points := make([]TimePoint, 0, len(dates))
	for _, date := range dates {
		points = append(points, TimePoint{
			Date:  date,
			Value: data[date],
		})
	}
	return points
}

// calculateTrendData 计算趋势数据
func calculateTrendData(points []TimePoint, title, unit string) *TimeTrendData {
	if len(points) == 0 {
		return &TimeTrendData{
			Title:      title,
			Unit:       unit,
			DataPoints: points,
			Total:      0,
			Average:    0,
			Max:        0,
			Min:        0,
			Trend:      "stable",
		}
	}

	// 计算统计信息
	total := 0
	max := points[0].Value
	min := points[0].Value
	for _, point := range points {
		total += point.Value
		if point.Value > max {
			max = point.Value
		}
		if point.Value < min {
			min = point.Value
		}
	}
	average := float64(total) / float64(len(points))

	// 计算趋势（简单线性趋势）
	trend := "stable"
	if len(points) >= 2 {
		firstHalf := 0
		secondHalf := 0
		midpoint := len(points) / 2

		for i := 0; i < midpoint; i++ {
			firstHalf += points[i].Value
		}
		for i := midpoint; i < len(points); i++ {
			secondHalf += points[i].Value
		}

		firstAvg := float64(firstHalf) / float64(midpoint)
		secondAvg := float64(secondHalf) / float64(len(points)-midpoint)

		if secondAvg > firstAvg*1.1 {
			trend = "up"
		} else if secondAvg < firstAvg*0.9 {
			trend = "down"
		} else {
			trend = "stable"
		}
	}

	return &TimeTrendData{
		Title:      title,
		Unit:       unit,
		DataPoints: points,
		Total:      total,
		Average:    average,
		Max:        max,
		Min:        min,
		Trend:      trend,
	}
}

// flattenTaskTree 递归扁平化任务树（辅助函数）
func (tm *TaskManager) flattenTaskTree(root *ComplexTask, result []*ComplexTask) []*ComplexTask {
	result = append(result, root)
	for i := range root.Subtasks {
		result = tm.flattenTaskTree(&root.Subtasks[i], result)
	}
	return result
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

// AnalyzeTaskTime 分析任务时间（父子任务时间关联分析）
func (tm *TaskManager) AnalyzeTaskTime(account, taskID string) (*TaskTimeAnalysis, error) {
	// 获取任务树
	taskTree, err := tm.GetTaskTree(account, taskID)
	if err != nil {
		return nil, err
	}

	// 计算子任务时间总和（只计入未删除且未完成的子任务），并收集子任务详细信息
	subtasksEstimatedTime := 0
	subtasksDailyTime := 0
	subtasksActualTime := 0
	validSubtasksCount := 0
	subtaskDetails := make([]SubtaskTimeDetail, 0, len(taskTree.Subtasks))

	for _, subtask := range taskTree.Subtasks {
		// 收集子任务详细信息（包括所有子任务，无论状态如何）
		detail := SubtaskTimeDetail{
			ID:            subtask.ID,
			Title:         subtask.Title,
			Status:        subtask.Status,
			EstimatedTime: subtask.EstimatedTime,
			DailyTime:     subtask.DailyTime,
			ActualTime:    subtask.ActualTime,
			Progress:      subtask.Progress,
			Deleted:       subtask.Deleted,
			Completed:     IsTaskCompleted(&subtask),
		}
		subtaskDetails = append(subtaskDetails, detail)

		// 跳过已删除或已完成的任务（不计入时间总和）
		if subtask.Deleted || IsTaskCompleted(&subtask) {
			continue
		}
		subtasksEstimatedTime += subtask.EstimatedTime
		subtasksDailyTime += subtask.DailyTime
		subtasksActualTime += subtask.ActualTime
		validSubtasksCount++
	}
	subtasksCount := validSubtasksCount

	// 计算时间差异
	estimatedTimeDiff := subtasksEstimatedTime - taskTree.EstimatedTime
	dailyTimeDiff := subtasksDailyTime - taskTree.DailyTime
	actualTimeDiff := subtasksActualTime - taskTree.ActualTime

	// 判断时间状态
	estimatedTimeStatus := "sufficient"
	if subtasksCount > 0 {
		// 只有在有子任务时才进行时间分析
		if estimatedTimeDiff > 0 {
			// 子任务预估时间总和大于父任务预估时间
			estimatedTimeStatus = "insufficient"
		} else if !IsTaskCompleted(taskTree) && taskTree.EstimatedTime > 0 {
			// 只有父任务未完成且父任务有预估时间时才检查是否时间分配过多
			// 使用更合理的阈值：30%（而不是50%）
			excessiveThreshold := -taskTree.EstimatedTime * 30 / 100
			if estimatedTimeDiff < excessiveThreshold {
				// 子任务预估时间总和远小于父任务预估时间（小于30%）
				estimatedTimeStatus = "excessive"
			}
		}
		// 如果父任务已完成，即使子任务时间很少也不标记为excessive（这是正常的）
	} else {
		// 没有子任务，标记为叶子任务
		estimatedTimeStatus = "leaf"
	}

	dailyTimeStatus := "sufficient"
	if subtasksCount > 0 {
		// 只有在有子任务时才进行时间分析
		if dailyTimeDiff > 0 {
			// 子任务每天分配时间总和大于父任务每天分配时间
			dailyTimeStatus = "insufficient"
		} else if !IsTaskCompleted(taskTree) && taskTree.DailyTime > 0 {
			// 只有父任务未完成且父任务有每天分配时间时才检查是否时间分配过多
			// 使用更合理的阈值：30%（而不是50%）
			excessiveThreshold := -taskTree.DailyTime * 30 / 100
			if dailyTimeDiff < excessiveThreshold {
				// 子任务每天分配时间总和远小于父任务每天分配时间（小于30%）
				dailyTimeStatus = "excessive"
			}
		}
		// 如果父任务已完成，即使子任务时间很少也不标记为excessive（这是正常的）
		// 如果父任务DailyTime为0，也不标记为excessive
	} else {
		// 没有子任务，标记为叶子任务
		dailyTimeStatus = "leaf"
	}

	analysis := &TaskTimeAnalysis{
		TaskID:                taskTree.ID,
		TaskTitle:             taskTree.Title,
		ParentID:              taskTree.ParentID,

		SelfEstimatedTime:     taskTree.EstimatedTime,
		SelfDailyTime:         taskTree.DailyTime,
		SelfActualTime:        taskTree.ActualTime,

		SubtasksEstimatedTime: subtasksEstimatedTime,
		SubtasksDailyTime:     subtasksDailyTime,
		SubtasksActualTime:    subtasksActualTime,

		EstimatedTimeDiff:     estimatedTimeDiff,
		DailyTimeDiff:         dailyTimeDiff,
		ActualTimeDiff:        actualTimeDiff,

		EstimatedTimeStatus:   estimatedTimeStatus,
		DailyTimeStatus:       dailyTimeStatus,

		SubtasksCount:         subtasksCount,
		HasSubtasks:           subtasksCount > 0,
		SubtaskDetails:        subtaskDetails,
	}

	return analysis, nil
}

// CalculateDailyTimeOverlap 计算每天的时间重叠（多个子任务在同一天的时间分配）
// 返回每天的时间分配总和，以及是否超过父任务的每天分配时间
func (tm *TaskManager) CalculateDailyTimeOverlap(account, taskID string) (map[string]int, bool, error) {
	// 获取任务树
	taskTree, err := tm.GetTaskTree(account, taskID)
	if err != nil {
		return nil, false, err
	}

	// 如果没有子任务，返回空
	if len(taskTree.Subtasks) == 0 {
		return map[string]int{}, false, nil
	}

	// 按天统计子任务的时间分配
	dailyTimeMap := make(map[string]int)
	hasOverlap := false

	// 遍历所有子任务（只检查未删除且未完成的子任务）
	for _, subtask := range taskTree.Subtasks {
		// 跳过已删除或已完成的任务
		if subtask.Deleted || IsTaskCompleted(&subtask) {
			continue
		}
		// 如果子任务有开始和结束日期
		if subtask.StartDate != "" && subtask.EndDate != "" {
			// 计算日期范围内的天数
			days, err := calculateDaysBetween(subtask.StartDate, subtask.EndDate)
			if err == nil && days > 0 && subtask.DailyTime > 0 {
				// 获取日期范围内的每一天
				start, err := time.Parse("2006-01-02", subtask.StartDate)
				if err != nil {
					continue
				}
				end, err := time.Parse("2006-01-02", subtask.EndDate)
				if err != nil {
					continue
				}

				// 遍历每一天
				for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
					dateStr := d.Format("2006-01-02")
					dailyTimeMap[dateStr] += subtask.DailyTime
				}
			}
		}
	}

	// 检查是否有任何一天的时间总和超过父任务的每天分配时间
	parentDailyTime := taskTree.DailyTime
	// 默认警告阈值：8小时 = 480分钟（如果父任务没有设置每天分配时间）
	defaultWarningThreshold := 480

	for date, totalTime := range dailyTimeMap {
		if parentDailyTime > 0 {
			// 父任务设置了每天分配时间：检查是否超过
			if totalTime > parentDailyTime {
				hasOverlap = true
				fmt.Printf("日期 %s: 子任务时间总和 %d 分钟 > 父任务每天分配时间 %d 分钟\n", date, totalTime, parentDailyTime)
				break
			}
		} else {
			// 父任务没有设置每天分配时间：检查是否超过默认警告阈值
			if totalTime > defaultWarningThreshold {
				hasOverlap = true
				fmt.Printf("日期 %s: 子任务时间总和 %d 分钟 > 建议每天最大时间 %d 分钟（父任务未设置每天分配时间）\n", date, totalTime, defaultWarningThreshold)
				break
			}
		}
	}

	return dailyTimeMap, hasOverlap, nil
}

// calculateActualTimeFromDates 根据开始日期、结束日期和每天分配时间计算实际时间
// 如果任何字段无效，返回0
func calculateActualTimeFromDates(startDate, endDate string, dailyTime int) int {
	if startDate == "" || endDate == "" || dailyTime <= 0 {
		log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeFromDates: 无效参数 startDate=%q, endDate=%q, dailyTime=%d", startDate, endDate, dailyTime)
		return 0
	}

	days, err := calculateDaysBetween(startDate, endDate)
	if err != nil || days <= 0 {
		log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeFromDates: 天数计算失败或无效 days=%d, err=%v", days, err)
		return 0
	}

	result := days * dailyTime
	log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeFromDates: 计算成功 days=%d, dailyTime=%d, result=%d", days, dailyTime, result)
	return result
}

// calculateActualTimeSmart 智能计算实际时间，处理dailyTime <= 0的情况
func calculateActualTimeSmart(task *ComplexTask) int {
	if task.StartDate == "" || task.EndDate == "" {
		log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeSmart: 开始日期或结束日期为空 startDate=%q, endDate=%q", task.StartDate, task.EndDate)
		return 0
	}

	// 计算天数
	days, err := calculateDaysBetween(task.StartDate, task.EndDate)
	if err != nil || days <= 0 {
		log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeSmart: 天数计算失败或无效 days=%d, err=%v", days, err)
		return 0
	}

	// 确定每天分配时间
	dailyTime := task.DailyTime
	if dailyTime <= 0 {
		// 如果每天分配时间为0或负数，尝试使用预估时间计算
		if task.EstimatedTime > 0 && days > 0 {
			// 计算平均每天分配时间（向上取整）
			dailyTime = (task.EstimatedTime + days - 1) / days // 向上取整除法
			log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeSmart: dailyTime<=0，使用预估时间计算 dailyTime=%d (estimatedTime=%d, days=%d)", dailyTime, task.EstimatedTime, days)
		} else {
			log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeSmart: dailyTime=%d 且无法计算替代值，返回0", task.DailyTime)
			return 0
		}
	}

	// 计算实际时间
	result := days * dailyTime
	log.DebugF(log.ModuleTaskBreakdown, "calculateActualTimeSmart: 计算成功 days=%d, dailyTime=%d, result=%d", days, dailyTime, result)
	return result
}

// calculateEstimatedTimeFromDates 根据开始日期、结束日期和每天分配时间计算预估时间
// 如果任何字段无效，返回0
func calculateEstimatedTimeFromDates(startDate, endDate string, dailyTime int) int {
	if startDate == "" || endDate == "" || dailyTime <= 0 {
		log.DebugF(log.ModuleTaskBreakdown, "calculateEstimatedTimeFromDates: 无效参数 startDate=%q, endDate=%q, dailyTime=%d", startDate, endDate, dailyTime)
		return 0
	}

	days, err := calculateDaysBetween(startDate, endDate)
	if err != nil || days <= 0 {
		log.DebugF(log.ModuleTaskBreakdown, "calculateEstimatedTimeFromDates: 天数计算失败或无效 days=%d, err=%v", days, err)
		return 0
	}

	result := days * dailyTime
	log.DebugF(log.ModuleTaskBreakdown, "calculateEstimatedTimeFromDates: 计算成功 days=%d, dailyTime=%d, result=%d", days, dailyTime, result)
	return result
}