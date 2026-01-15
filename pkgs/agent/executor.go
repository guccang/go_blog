package agent

import (
	"context"
	"fmt"
	log "mylog"
	"sync"
	"time"
)

// ============================================================================
// TaskExecutor - 任务执行器
// ============================================================================

// TaskExecutor 任务执行器
type TaskExecutor struct {
	graph   *TaskGraph
	planner *TaskPlanner
	hub     *NotificationHub
	storage *TaskStorage
	config  *ExecutionConfig
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
}

// NewTaskExecutor 创建任务执行器
func NewTaskExecutor(graph *TaskGraph, planner *TaskPlanner, hub *NotificationHub, storage *TaskStorage) *TaskExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskExecutor{
		graph:   graph,
		planner: planner,
		hub:     hub,
		storage: storage,
		config:  graph.Config,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Execute 执行任务图
func (e *TaskExecutor) Execute() error {
	root := e.graph.Root
	if root == nil {
		return fmt.Errorf("no root node in graph")
	}

	log.MessageF(log.ModuleAgent, "Starting execution of task: %s", root.Title)
	root.AddLog(LogInfo, "starting", "开始执行任务")

	// 通知图更新
	e.notifyGraphUpdate("graph_started", root)

	// 执行根节点
	err := e.executeNode(root)

	// 标记完成
	e.graph.MarkComplete()

	// 通知完成
	if err != nil {
		e.notifyGraphUpdate("graph_failed", root)
	} else {
		e.notifyGraphUpdate("graph_completed", root)
	}

	log.MessageF(log.ModuleAgent, "Execution completed for task: %s, success: %v", root.Title, err == nil)
	return err
}

// executeNode 执行单个节点
func (e *TaskExecutor) executeNode(node *TaskNode) error {
	// 检查取消
	select {
	case <-e.ctx.Done():
		node.SetStatus(NodeCanceled)
		return fmt.Errorf("execution canceled")
	default:
	}

	// 检查节点取消
	if node.IsCanceled() {
		return fmt.Errorf("node canceled")
	}

	// 设置运行状态
	node.SetStatus(NodeRunning)
	e.graph.UpdateNodeStatus(node.ID, NodeRunning)
	node.AddLog(LogInfo, "executing", fmt.Sprintf("开始执行: %s", node.Title))
	e.notifyNodeUpdate("node_started", node)

	// 检查是否需要拆解
	if e.shouldDecompose(node) {
		// 任务拆解
		if err := e.decomposeNode(node); err != nil {
			node.AddLog(LogError, "planning", fmt.Sprintf("任务拆解失败: %v", err))
			return e.handleNodeError(node, err)
		}
	}

	// 如果有子节点，执行子节点
	if len(node.Children) > 0 {
		var err error
		switch node.ExecutionMode {
		case ModeParallel:
			err = e.executeParallel(node)
		default:
			err = e.executeSequential(node)
		}

		if err != nil {
			return e.handleNodeError(node, err)
		}

		// 汇总子节点结果
		e.aggregateChildResults(node)
	} else {
		// 叶子节点，直接执行
		if err := e.executeLeafNode(node); err != nil {
			return e.handleNodeError(node, err)
		}
	}

	// 标记完成
	node.SetStatus(NodeDone)
	node.SetProgress(100)
	e.graph.UpdateNodeStatus(node.ID, NodeDone)
	node.AddLog(LogInfo, "completed", fmt.Sprintf("执行完成: %s", node.Title))
	e.notifyNodeUpdate("node_completed", node)

	return nil
}

// shouldDecompose 判断是否需要拆解
func (e *TaskExecutor) shouldDecompose(node *TaskNode) bool {
	// 已有子节点，不需要再拆解
	if len(node.Children) > 0 {
		return false
	}

	// 不可拆解
	if !node.CanDecompose {
		return false
	}

	// 达到最大深度，不再拆解
	if node.Depth >= e.config.MaxDepth {
		node.AddLog(LogInfo, "planning", fmt.Sprintf("达到最大深度 %d，不再拆解", e.config.MaxDepth))
		return false
	}

	return true
}

// decomposeNode 拆解节点（使用 LLM）
func (e *TaskExecutor) decomposeNode(node *TaskNode) error {
	node.AddLog(LogInfo, "planning", "开始任务拆解")

	// 构建上下文
	e.buildNodeContext(node)

	// 调用 planner 进行拆解
	result, err := e.planner.PlanNode(node)
	if err != nil {
		return err
	}

	// 如果没有子任务，标记为不可拆解
	if len(result.SubTasks) == 0 {
		node.CanDecompose = false
		node.AddLog(LogInfo, "planning", "无需拆解，直接执行")
		return nil
	}

	// 创建子节点
	node.ExecutionMode = result.ExecutionMode

	// 先创建所有子节点并构建标题到ID的映射
	titleToID := make(map[string]string)
	createdNodes := make([]*TaskNode, 0, len(result.SubTasks))

	for _, st := range result.SubTasks {
		child := node.NewChildNode(st.Title, st.Description, st.Goal)
		child.ToolCalls = st.Tools
		child.CanDecompose = st.CanDecompose
		// 先不设置 DependsOn，等所有节点创建完再处理
		e.graph.AddNode(child)

		titleToID[st.Title] = child.ID
		createdNodes = append(createdNodes, child)
	}

	// 将 DependsOn 中的标题转换为节点 ID
	for i, st := range result.SubTasks {
		if len(st.DependsOn) > 0 {
			var depIDs []string
			for _, depTitle := range st.DependsOn {
				if depID, ok := titleToID[depTitle]; ok {
					depIDs = append(depIDs, depID)
				} else {
					node.AddLog(LogWarn, "planning", fmt.Sprintf("依赖节点未找到: %s", depTitle))
				}
			}
			createdNodes[i].DependsOn = depIDs
		}
	}

	node.AddLog(LogInfo, "planning", fmt.Sprintf("任务拆解完成: %d 个子任务，模式: %s", len(node.Children), node.ExecutionMode))
	e.notifyGraphUpdate("graph_update", node)

	return nil
}

// executeSequential 串行执行子节点
func (e *TaskExecutor) executeSequential(node *TaskNode) error {
	node.AddLog(LogInfo, "executing", fmt.Sprintf("串行执行 %d 个子任务", len(node.Children)))

	for i, child := range node.Children {
		// 检查依赖
		if err := e.waitForDependencies(child); err != nil {
			return err
		}

		// 执行子节点
		if err := e.executeNode(child); err != nil {
			// 检查是否可重试
			if child.CanRetry() {
				child.IncrementRetry()
				child.AddLog(LogWarn, "retry", fmt.Sprintf("重试第 %d 次", child.RetryCount))
				child.SetStatus(NodePending)
				i-- // 重试当前节点
				continue
			}
			return err
		}

		// 更新父节点进度
		progress := float64(i+1) / float64(len(node.Children)) * 100
		node.SetProgress(progress)
		e.notifyNodeUpdate("node_progress", node)

		// 添加兄弟结果到上下文
		e.propagateSiblingResult(child)
	}

	return nil
}

// executeParallel 并行执行子节点
func (e *TaskExecutor) executeParallel(node *TaskNode) error {
	node.AddLog(LogInfo, "executing", fmt.Sprintf("并行执行 %d 个子任务", len(node.Children)))

	var wg sync.WaitGroup
	errChan := make(chan error, len(node.Children))
	doneChan := make(chan string, len(node.Children))

	for _, child := range node.Children {
		wg.Add(1)
		go func(c *TaskNode) {
			defer wg.Done()

			// 等待依赖
			if err := e.waitForDependencies(c); err != nil {
				errChan <- err
				return
			}

			// 执行
			if err := e.executeNode(c); err != nil {
				errChan <- err
				return
			}

			doneChan <- c.ID
		}(child)
	}

	// 等待完成并更新进度
	go func() {
		done := 0
		total := len(node.Children)
		for range doneChan {
			done++
			progress := float64(done) / float64(total) * 100
			node.SetProgress(progress)
			e.notifyNodeUpdate("node_progress", node)
		}
	}()

	wg.Wait()
	close(errChan)
	close(doneChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(errs), errs[0])
	}

	return nil
}

// waitForDependencies 等待依赖完成
func (e *TaskExecutor) waitForDependencies(node *TaskNode) error {
	if len(node.DependsOn) == 0 {
		return nil
	}

	node.AddLog(LogDebug, "waiting", fmt.Sprintf("等待 %d 个依赖完成", len(node.DependsOn)))

	timeout := time.After(e.config.ExecutionTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return fmt.Errorf("context canceled while waiting for dependencies")
		case <-timeout:
			return fmt.Errorf("timeout waiting for dependencies")
		case <-ticker.C:
			allDone := true
			for _, depID := range node.DependsOn {
				dep := e.graph.GetNode(depID)
				if dep == nil {
					return fmt.Errorf("dependency node not found: %s", depID)
				}
				if dep.Status != NodeDone {
					allDone = false
					break
				}
			}
			if allDone {
				return nil
			}
		}
	}
}

// executeLeafNode 执行叶子节点
func (e *TaskExecutor) executeLeafNode(node *TaskNode) error {
	node.AddLog(LogInfo, "executing", "执行叶子节点")

	// 构建上下文
	e.buildNodeContext(node)

	// 调用 planner 执行
	result, err := e.planner.ExecuteNode(node)
	if err != nil {
		node.Result = NewTaskResultError(err.Error())
		return err
	}

	node.Result = result
	node.AddLog(LogInfo, "completed", fmt.Sprintf("执行结果: %s", result.Summary))

	return nil
}

// buildNodeContext 构建节点上下文
func (e *TaskExecutor) buildNodeContext(node *TaskNode) {
	// 添加父任务结果
	parent := e.graph.GetParent(node.ID)
	for parent != nil {
		if parent.Result != nil {
			node.Context.AddParentResult(parent.ID, parent.Title, parent.Result.Summary)
		}
		parent = e.graph.GetParent(parent.ID)
	}

	// 添加已完成的兄弟任务结果
	siblings := e.graph.GetCompletedSiblings(node.ID)
	for _, s := range siblings {
		if s.Result != nil {
			node.Context.AddSiblingResult(s.ID, s.Title, s.Status, s.Result.Summary)
		}
	}
}

// propagateSiblingResult 传播兄弟结果到后续节点
func (e *TaskExecutor) propagateSiblingResult(node *TaskNode) {
	if node.Result == nil {
		return
	}

	// 获取未执行的兄弟节点
	parent := e.graph.GetParent(node.ID)
	if parent == nil {
		return
	}

	for _, sibling := range parent.Children {
		if sibling.ID != node.ID && sibling.Status == NodePending {
			sibling.Context.AddSiblingResult(node.ID, node.Title, node.Status, node.Result.Summary)
		}
	}
}

// aggregateChildResults 汇总子节点结果
func (e *TaskExecutor) aggregateChildResults(node *TaskNode) {
	var summaries []string
	var allSuccess = true

	for _, child := range node.Children {
		if child.Result != nil {
			summaries = append(summaries, fmt.Sprintf("%s: %s", child.Title, child.Result.Summary))
			if !child.Result.Success {
				allSuccess = false
			}
		}
	}

	node.Result = &TaskResult{
		Success: allSuccess,
		Summary: fmt.Sprintf("完成 %d 个子任务", len(node.Children)),
		Output:  fmt.Sprintf("子任务结果:\n%s", joinStrings(summaries, "\n")),
	}
}

// handleNodeError 处理节点错误
func (e *TaskExecutor) handleNodeError(node *TaskNode, err error) error {
	node.SetStatus(NodeFailed)
	e.graph.UpdateNodeStatus(node.ID, NodeFailed)
	node.Result = NewTaskResultError(err.Error())
	node.AddLog(LogError, "failed", fmt.Sprintf("执行失败: %v", err))
	e.notifyNodeUpdate("node_failed", node)
	return err
}

// notifyGraphUpdate 通知图更新
func (e *TaskExecutor) notifyGraphUpdate(notifType string, node *TaskNode) {
	if e.hub == nil {
		return
	}

	notif := NewGraphNotification(notifType, node.ID).
		WithNode(node).
		WithStats(e.graph)

	// 对于完整更新，附加全部数据
	if notifType == "graph_update" || notifType == "graph_started" {
		notif = notif.WithFullData(e.graph)
	}

	e.hub.Broadcast(TaskNotification{
		TaskID:  e.graph.RootID,
		Type:    notifType,
		Message: node.Title,
		Data:    notif,
	})
}

// notifyNodeUpdate 通知节点更新
func (e *TaskExecutor) notifyNodeUpdate(notifType string, node *TaskNode) {
	if e.hub == nil {
		return
	}

	notif := NewGraphNotification(notifType, node.ID).
		WithNode(node).
		WithStats(e.graph)

	// 如果有最新日志，附加
	if len(node.Logs) > 0 {
		lastLog := node.Logs[len(node.Logs)-1]
		notif = notif.WithLog(&lastLog)
	}

	e.hub.Broadcast(TaskNotification{
		TaskID:   e.graph.RootID,
		Type:     notifType,
		Progress: node.Progress,
		Message:  node.Title,
		Data:     notif,
	})
}

// Cancel 取消执行
func (e *TaskExecutor) Cancel() {
	e.cancel()
	e.graph.Root.Cancel()
}

// joinStrings 连接字符串
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
