package agent

import (
	"context"
	"fmt"
	"mcp"
	log "mylog"
	"strings"
	"sync"
	"time"
)

// 输出长度限制常量
const (
	MaxOutputLength           = 5000 // 超过此长度保存为博客
	MaxSummaryLength          = 2000 // 摘要最大长度
	DefaultMaxParallelRetries = 4    // 并行执行最大重试轮数（初次执行 + 3次重试）
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
	root.AddLog(LogInfo, "starting", fmt.Sprintf("开始执行任务: %s", root.Title))

	// 通知图更新
	e.notifyGraphUpdate("graph_started", root)

	// 执行根节点
	err := e.executeNode(root)

	// 标记完成
	e.graph.MarkComplete()

	// 生成任务索引博客
	if err == nil {
		e.generateTaskIndex()
	}

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

	// 重试时跳过已完成的节点
	if node.Status == NodeDone {
		node.AddLog(LogDebug, "skip", "节点已完成，跳过执行")
		return nil
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
	result, err := e.planner.PlanNode(e.ctx, node)
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

	// 检测循环依赖（包括跨层级循环）
	if err := e.detectCyclicDependencies(node, createdNodes); err != nil {
		node.AddLog(LogError, "planning", fmt.Sprintf("检测到循环依赖: %v", err))
		// 清除已创建的子节点，避免死锁
		for _, child := range createdNodes {
			delete(e.graph.Nodes, child.ID)
		}
		node.Children = nil
		node.CanDecompose = false
		return err
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

// executeParallel 并行执行子节点（带重试）
func (e *TaskExecutor) executeParallel(node *TaskNode) error {
	node.AddLog(LogInfo, "executing", fmt.Sprintf("并行执行 %d 个子任务", len(node.Children)))

	for attempt := 0; attempt < DefaultMaxParallelRetries; attempt++ {
		if attempt > 0 {
			node.AddLog(LogInfo, "retry_round", fmt.Sprintf("并行执行第 %d 轮重试", attempt))
			log.MessageF(log.ModuleAgent, "Parallel execution retry round %d for node: %s", attempt, node.Title)
		}

		// 收集需要执行的节点（pending 或 failed 且可重试）
		var toExecute []*TaskNode
		for _, child := range node.Children {
			if child.Status == NodePending || (child.Status == NodeFailed && child.CanRetry()) {
				if child.Status == NodeFailed {
					child.IncrementRetry()
					child.AddLog(LogWarn, "retry", fmt.Sprintf("重试第 %d/%d 次 (MaxRetries=%d)", child.RetryCount, child.MaxRetries, child.MaxRetries))
					log.MessageF(log.ModuleAgent, "Retrying node '%s': attempt %d/%d", child.Title, child.RetryCount, child.MaxRetries)
					child.SetStatus(NodePending)
				}
				toExecute = append(toExecute, child)
			}
		}

		if len(toExecute) == 0 {
			break // 没有需要执行的节点
		}

		node.AddLog(LogDebug, "executing", fmt.Sprintf("本轮执行 %d 个节点", len(toExecute)))

		var wg sync.WaitGroup
		doneChan := make(chan string, len(toExecute))

		for _, child := range toExecute {
			wg.Add(1)
			go func(c *TaskNode) {
				defer wg.Done()

				// 等待依赖
				if err := e.waitForDependencies(c); err != nil {
					c.SetStatus(NodeFailed)
					c.Result = NewTaskResultError(err.Error())
					// 记录依赖等待失败的详细日志
					c.AddLog(LogError, "dependency_failed", fmt.Sprintf("依赖等待失败: %v (当前重试次数: %d/%d)", err, c.RetryCount, c.MaxRetries))
					log.MessageF(log.ModuleAgent, "Node '%s' dependency wait failed: %v (retry %d/%d)", c.Title, err, c.RetryCount, c.MaxRetries)
					return
				}

				// 执行
				if err := e.executeNode(c); err != nil {
					// executeNode 已经设置了状态
					log.MessageF(log.ModuleAgent, "Node '%s' execution failed: %v (retry %d/%d)", c.Title, err, c.RetryCount, c.MaxRetries)
					return
				}

				doneChan <- c.ID
			}(child)
		}

		// 等待本轮完成并更新进度
		var progressWg sync.WaitGroup
		progressWg.Add(1)
		go func() {
			defer progressWg.Done()
			done := 0
			total := len(node.Children)
			for range doneChan {
				done++
				// 计算已完成的总数
				completedCount := 0
				for _, c := range node.Children {
					if c.Status == NodeDone {
						completedCount++
					}
				}
				progress := float64(completedCount) / float64(total) * 100
				node.SetProgress(progress)
				e.notifyNodeUpdate("node_progress", node)
			}
		}()

		wg.Wait()
		close(doneChan)
		progressWg.Wait() // 等待进度 goroutine 退出
	}

	// 检查最终结果
	var failedNodes []string
	for _, child := range node.Children {
		if child.Status == NodeFailed {
			failedNodes = append(failedNodes, child.Title)
		}
	}

	if len(failedNodes) > 0 {
		return fmt.Errorf("parallel execution failed, failed nodes: %v", failedNodes)
	}

	return nil
}

// waitForDependencies 等待依赖完成
func (e *TaskExecutor) waitForDependencies(node *TaskNode) error {
	if len(node.DependsOn) == 0 {
		return nil
	}

	node.AddLog(LogDebug, "waiting", fmt.Sprintf("等待 %d 个依赖完成: %v", len(node.DependsOn), node.DependsOn))

	timeout := time.After(e.config.ExecutionTimeout)
	ticker := time.NewTicker(500 * time.Millisecond) // 增加检查间隔以减少 CPU 开销
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return fmt.Errorf("context canceled while waiting for dependencies")
		case <-timeout:
			// 超时时提供详细信息
			var pendingDeps []string
			for _, depID := range node.DependsOn {
				dep := e.findDependencyNode(depID)
				if dep != nil && dep.Status != NodeDone {
					pendingDeps = append(pendingDeps, fmt.Sprintf("%s(%s)", dep.Title, dep.Status))
				}
			}
			return fmt.Errorf("timeout waiting for dependencies: %v", pendingDeps)
		case <-ticker.C:
			allDone := true
			for _, depID := range node.DependsOn {
				dep := e.findDependencyNode(depID)
				if dep == nil {
					node.AddLog(LogWarn, "waiting", fmt.Sprintf("依赖节点未找到: %s", depID))
					continue // 跳过未找到的依赖，而不是立即失败
				}
				// 检查依赖是否已失败或被取消
				if dep.Status == NodeFailed || dep.Status == NodeCanceled {
					return fmt.Errorf("dependency '%s' failed with status: %s", dep.Title, dep.Status)
				}
				if dep.Status != NodeDone {
					allDone = false
					break
				}
			}
			if allDone {
				node.AddLog(LogDebug, "waiting", "所有依赖已完成")
				return nil
			}
		}
	}
}

// detectCyclicDependencies 检测循环依赖（使用 DFS）
// 检测两种循环：1. 显式依赖形成的循环 2. 子节点依赖祖先节点的跨层级循环
func (e *TaskExecutor) detectCyclicDependencies(parentNode *TaskNode, createdNodes []*TaskNode) error {
	// 收集所有祖先节点 ID
	ancestorIDs := make(map[string]bool)
	current := parentNode
	for current != nil {
		ancestorIDs[current.ID] = true
		if current.ParentID == "" {
			break
		}
		current = e.graph.GetNode(current.ParentID)
	}

	// 检查是否有子节点依赖祖先节点（跨层级循环）
	for _, child := range createdNodes {
		for _, depID := range child.DependsOn {
			if ancestorIDs[depID] {
				depNode := e.graph.GetNode(depID)
				depTitle := depID
				if depNode != nil {
					depTitle = depNode.Title
				}
				return fmt.Errorf("子任务 '%s' 依赖祖先任务 '%s'，形成跨层级循环", child.Title, depTitle)
			}
		}
	}

	// 使用 DFS 检测兄弟节点之间的循环依赖
	visited := make(map[string]int) // 0=未访问, 1=访问中, 2=完成
	var cyclePath []string

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		if visited[nodeID] == 1 {
			cyclePath = append(cyclePath, nodeID)
			return true
		}
		if visited[nodeID] == 2 {
			return false
		}

		visited[nodeID] = 1
		node := e.graph.GetNode(nodeID)
		if node == nil {
			visited[nodeID] = 2
			return false
		}

		for _, depID := range node.DependsOn {
			if dfs(depID) {
				cyclePath = append(cyclePath, nodeID)
				return true
			}
		}

		visited[nodeID] = 2
		return false
	}

	for _, node := range createdNodes {
		if visited[node.ID] == 0 {
			if dfs(node.ID) {
				// 构建循环路径描述
				var pathTitles []string
				for _, id := range cyclePath {
					if n := e.graph.GetNode(id); n != nil {
						pathTitles = append(pathTitles, n.Title)
					}
				}
				return fmt.Errorf("检测到循环依赖链: %v", pathTitles)
			}
		}
	}

	return nil
}

// findDependencyNode 查找依赖节点（支持按ID或标题查找）
func (e *TaskExecutor) findDependencyNode(idOrTitle string) *TaskNode {
	// 首先尝试按 ID 查找
	if node := e.graph.GetNode(idOrTitle); node != nil {
		return node
	}

	// 按 ID 未找到，尝试按标题查找（兼容旧数据）
	for _, node := range e.graph.Nodes {
		if node.Title == idOrTitle {
			return node
		}
	}

	return nil
}

// executeLeafNode 执行叶子节点
func (e *TaskExecutor) executeLeafNode(node *TaskNode) error {
	node.AddLog(LogInfo, "executing", fmt.Sprintf("执行叶子节点: %s", node.Title))

	// 构建上下文
	e.buildNodeContext(node)

	// 调用 planner 执行
	result, err := e.planner.ExecuteNode(e.ctx, node)
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

// aggregateChildResults 汇总子节点结果（LLM 智能整合版 + 博客引用）
func (e *TaskExecutor) aggregateChildResults(node *TaskNode) {
	var summaries []string
	var detailedOutputs []string
	var allArtifacts []string
	var allSuccess = true

	for _, child := range node.Children {
		if child.Result != nil {
			// 检查输出长度，过长则保存为博客
			childSummary := child.Result.Summary
			if len(child.Result.Output) > MaxOutputLength {
				blogLink, err := e.saveOutputAsBlog(child, child.Result.Output)
				if err != nil {
					node.AddLog(LogWarn, "artifact", fmt.Sprintf("保存博客失败: %v", err))
				} else {
					node.AddLog(LogInfo, "artifact", fmt.Sprintf("输出已保存为博客: %s", blogLink))
					allArtifacts = append(allArtifacts, blogLink)
					// 在摘要中添加博客链接
					childSummary = fmt.Sprintf("%s (详情: %s)", child.Result.Summary, blogLink)
					// 更新子节点的 Artifacts
					if child.Result.Artifacts == nil {
						child.Result.Artifacts = []string{}
					}
					child.Result.Artifacts = append(child.Result.Artifacts, blogLink)
				}
			}

			summaries = append(summaries, fmt.Sprintf("%s: %s", child.Title, childSummary))
			// 只保留较短的输出内容用于父任务参考
			if child.Result.Output != "" && len(child.Result.Output) <= MaxOutputLength {
				detailedOutputs = append(detailedOutputs, fmt.Sprintf("=== %s ===\n%s", child.Title, child.Result.Output))
			}
			if !child.Result.Success {
				allSuccess = false
			}
		}
	}

	// 原始拼接结果
	rawOutput := joinStrings(detailedOutputs, "\n\n")
	rawSummary := fmt.Sprintf("完成 %d 个子任务: %s", len(node.Children), joinStrings(summaries, "; "))

	// 尝试使用 LLM 整合结果
	var synthesizedSummary string
	if e.planner != nil && len(node.Children) > 0 {
		childResultsText := joinStrings(summaries, "\n")
		result, err := e.planner.SynthesizeResults(e.ctx, node, childResultsText)
		if err == nil && result != "" {
			synthesizedSummary = result
			node.AddLog(LogInfo, "synthesis", "LLM 结果整合完成")
		} else {
			node.AddLog(LogWarn, "synthesis", fmt.Sprintf("LLM 整合失败，使用原始汇总: %v", err))
			synthesizedSummary = rawSummary
		}
	} else {
		synthesizedSummary = rawSummary
	}

	node.Result = &TaskResult{
		Success:    allSuccess,
		Summary:    synthesizedSummary,
		RawSummary: rawSummary,
		Output:     fmt.Sprintf("子任务详细结果:\n\n%s", rawOutput),
		Artifacts:  allArtifacts,
	}

	// 更新父节点上下文，包含子任务结果供后续 LLM 调用参考
	if node.Context != nil {
		for _, child := range node.Children {
			if child.Result != nil {
				node.Context.AddSiblingResult(child.ID, child.Title, child.Status, child.Result.Summary)
			}
		}
	}
}

// saveOutputAsBlog 将过长的输出保存为博客
func (e *TaskExecutor) saveOutputAsBlog(node *TaskNode, content string) (string, error) {
	title := e.generateBlogTitle(node)

	args := map[string]interface{}{
		"account":  node.Account,
		"title":    title,
		"content":  content,
		"tags":     "Agent|任务输出|自动生成",
		"authType": float64(1), // 私有
	}

	result := mcp.CallMCPTool("RawCreateBlog", args)
	if !result.Success {
		return "", fmt.Errorf("保存博客失败: %s", result.Error)
	}

	// 返回链接格式
	link := fmt.Sprintf("[%s](/get?blogname=%s)", title, title)
	log.MessageF(log.ModuleAgent, "[博客保存] 任务 '%s' 输出已保存: %s (原长度: %d 字符)", node.Title, link, len(content))
	return link, nil
}

// generateBlogTitle 根据任务树构建层级路径
// 例如: agent_tasks/20260225_中国2026年中产如何规划现金流/制定现金流规划方法论/制定储蓄投资模块
func (e *TaskExecutor) generateBlogTitle(node *TaskNode) string {
	date := time.Now().Format("20060102")

	// 从当前节点向上遍历到根节点，收集路径
	path := []string{}
	current := node
	for current != nil {
		title := sanitizeFolderName(truncateTitle(current.Title, 20))
		path = append([]string{title}, path...)
		if current.ParentID == "" {
			break
		}
		current = e.graph.GetNode(current.ParentID)
	}

	// 根目录加上日期前缀
	if len(path) > 0 {
		path[0] = fmt.Sprintf("%s_%s", date, path[0])
	}

	return fmt.Sprintf("agent_tasks/%s", strings.Join(path, "/"))
}

// truncateTitle 截断标题（使用 rune 避免中文截断乱码）
func truncateTitle(title string, maxLen int) string {
	runes := []rune(title)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return title
}

// sanitizeFolderName 清理文件夹名中的不安全字符
func sanitizeFolderName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_",
		"*", "_", "?", "_", "\"", "_",
		"<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}

// handleNodeError 处理节点错误
func (e *TaskExecutor) handleNodeError(node *TaskNode, err error) error {
	node.SetStatus(NodeFailed)
	e.graph.UpdateNodeStatus(node.ID, NodeFailed)
	node.Result = NewTaskResultError(err.Error())

	// 详细错误分类日志
	errorType := classifyError(err)
	node.AddLog(LogError, "failed", fmt.Sprintf("[节点: %s] 执行失败 [%s]: %v", node.Title, errorType, err))
	log.MessageF(log.ModuleAgent, "[执行失败] 节点: '%s', 错误类型: %s, 详情: %v", node.Title, errorType, err)

	e.notifyNodeUpdate("node_failed", node)
	return err
}

// classifyError 错误分类
func classifyError(err error) string {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "context deadline exceeded"):
		return "超时"
	case strings.Contains(errStr, "Client.Timeout"):
		return "HTTP超时"
	case strings.Contains(errStr, "connection refused"):
		return "连接拒绝"
	case strings.Contains(errStr, "no such host"):
		return "DNS解析失败"
	case strings.Contains(errStr, "EOF"):
		return "连接中断"
	case strings.Contains(errStr, "LLM"):
		return "LLM调用失败"
	case strings.Contains(errStr, "dependency"):
		return "依赖失败"
	default:
		return "未知错误"
	}
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

// ============================================================================
// 用户输入等待支持
// ============================================================================

// notifyInputRequest 通知前端需要用户输入
func (e *TaskExecutor) notifyInputRequest(node *TaskNode, req *InputRequest) {
	if e.hub == nil {
		return
	}

	node.AddLog(LogInfo, "waiting_input", fmt.Sprintf("等待用户输入: %s", req.Title))

	e.hub.Broadcast(TaskNotification{
		TaskID:  e.graph.RootID,
		Type:    "input_required",
		Message: req.Title,
		Data: map[string]interface{}{
			"request": req,
			"node_id": node.ID,
			"node":    node.Title,
		},
	})
}

// RequestUserInput 请求用户输入并等待响应
// 这是从执行器内部请求用户输入的主方法
func (e *TaskExecutor) RequestUserInput(node *TaskNode, title, message string, inputType InputType) (*InputResponse, error) {
	// 创建输入请求
	req := NewInputRequest(node.ID, e.graph.RootID, node.Account, title, message, inputType)

	// 通知前端
	e.notifyInputRequest(node, req)

	// 等待用户输入（会阻塞直到用户响应）
	resp, cancelled := node.WaitForInput(req)
	if cancelled {
		node.AddLog(LogWarn, "input_cancelled", "用户取消了输入")
		return nil, fmt.Errorf("user cancelled input")
	}

	node.AddLog(LogInfo, "input_received", fmt.Sprintf("收到用户输入: %v", resp.Value))
	return resp, nil
}

// RequestUserConfirmation 请求用户确认（是/否）
func (e *TaskExecutor) RequestUserConfirmation(node *TaskNode, title, message string) (bool, error) {
	req := NewInputRequest(node.ID, e.graph.RootID, node.Account, title, message, InputTypeConfirm)
	req.Options = []InputOption{
		{Value: "yes", Label: "是"},
		{Value: "no", Label: "否"},
	}

	e.notifyInputRequest(node, req)

	resp, cancelled := node.WaitForInput(req)
	if cancelled {
		return false, fmt.Errorf("user cancelled confirmation")
	}

	// 解析响应
	value, ok := resp.Value.(string)
	if !ok {
		return false, fmt.Errorf("invalid confirmation response type")
	}
	return value == "yes" || value == "true", nil
}

// RequestUserSelection 请求用户从选项中选择
func (e *TaskExecutor) RequestUserSelection(node *TaskNode, title, message string, options []InputOption) (string, error) {
	req := NewInputRequest(node.ID, e.graph.RootID, node.Account, title, message, InputTypeSelect)
	req.Options = options

	e.notifyInputRequest(node, req)

	resp, cancelled := node.WaitForInput(req)
	if cancelled {
		return "", fmt.Errorf("user cancelled selection")
	}

	value, ok := resp.Value.(string)
	if !ok {
		return "", fmt.Errorf("invalid selection response type")
	}
	return value, nil
}

// joinStrings 连接字符串（使用标准库）
func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}

// ============================================================================
// 任务索引生成
// ============================================================================

// generateTaskIndex 生成任务文档索引博客
func (e *TaskExecutor) generateTaskIndex() {
	root := e.graph.Root
	if root == nil {
		return
	}

	// 构建 Markdown 索引内容
	content := e.buildIndexContent()

	// 生成索引标题
	title := e.generateIndexTitle()

	// 保存为私有博客
	args := map[string]interface{}{
		"account":  root.Account,
		"title":    title,
		"content":  content,
		"tags":     "Agent|任务索引|自动生成",
		"authType": float64(1), // 私有
	}

	result := mcp.CallMCPTool("RawCreateBlog", args)
	if result.Success {
		log.MessageF(log.ModuleAgent, "[索引生成] 任务 '%s' 索引已保存: %s", root.Title, title)
		// 将索引链接加入根节点 Artifacts
		if root.Result != nil {
			indexLink := fmt.Sprintf("[📚 任务索引](/get?blogname=%s)", title)
			root.Result.Artifacts = append([]string{indexLink}, root.Result.Artifacts...)
		}
	} else {
		log.WarnF(log.ModuleAgent, "[索引生成] 保存索引博客失败: %s", result.Error)
	}
}

// buildIndexContent 构建 Markdown 格式的索引内容
func (e *TaskExecutor) buildIndexContent() string {
	var sb strings.Builder
	sb.Grow(4096)

	root := e.graph.Root

	// 标题和元信息
	sb.WriteString(fmt.Sprintf("# 📋 任务索引: %s\n\n", root.Title))
	sb.WriteString(fmt.Sprintf("- **任务ID**: `%s`\n", root.ID))
	sb.WriteString(fmt.Sprintf("- **创建时间**: %s\n", root.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- **执行耗时**: %s\n", e.graph.GetExecutionTime().Round(time.Second)))
	sb.WriteString(fmt.Sprintf("- **节点总数**: %d\n", e.graph.TotalNodes))
	sb.WriteString(fmt.Sprintf("- **完成/失败**: %d / %d\n\n", e.graph.DoneNodes, e.graph.FailedNodes))

	// 树形结构
	sb.WriteString("## 📂 任务结构\n\n")
	e.writeNodeTree(&sb, root, 0)

	// 所有生成的文档列表
	sb.WriteString("\n## 📄 生成的文档\n\n")
	e.writeArtifactsList(&sb)

	return sb.String()
}

// writeNodeTree 递归写入节点树
func (e *TaskExecutor) writeNodeTree(sb *strings.Builder, node *TaskNode, depth int) {
	indent := strings.Repeat("  ", depth)

	// 状态图标
	statusIcon := getStatusIcon(node.Status)

	// 节点行
	sb.WriteString(fmt.Sprintf("%s- %s **%s**", indent, statusIcon, node.Title))

	// 执行时间
	if node.Duration > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", node.Duration.Round(time.Millisecond)))
	}

	// 生成的文档链接（排除索引本身）
	if node.Result != nil && len(node.Result.Artifacts) > 0 {
		var links []string
		for _, link := range node.Result.Artifacts {
			if !strings.Contains(link, "任务索引") {
				links = append(links, link)
			}
		}
		if len(links) > 0 {
			sb.WriteString(" 📎 ")
			sb.WriteString(strings.Join(links, " | "))
		}
	}

	sb.WriteString("\n")

	// 递归子节点
	for _, child := range node.Children {
		e.writeNodeTree(sb, child, depth+1)
	}
}

// writeArtifactsList 写入所有文档列表
func (e *TaskExecutor) writeArtifactsList(sb *strings.Builder) {
	type artifactInfo struct {
		NodeTitle string
		Link      string
	}
	var artifacts []artifactInfo

	// 收集所有节点的 Artifacts（排除索引本身）
	for _, node := range e.graph.Nodes {
		if node.Result != nil {
			for _, link := range node.Result.Artifacts {
				if !strings.Contains(link, "任务索引") {
					artifacts = append(artifacts, artifactInfo{node.Title, link})
				}
			}
		}
	}

	if len(artifacts) == 0 {
		sb.WriteString("*无生成文档*\n")
		return
	}

	sb.WriteString("| 来源节点 | 文档链接 |\n")
	sb.WriteString("|----------|----------|\n")
	for _, a := range artifacts {
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", a.NodeTitle, a.Link))
	}
}

// getStatusIcon 获取状态图标
func getStatusIcon(status NodeStatus) string {
	switch status {
	case NodeDone:
		return "✅"
	case NodeFailed:
		return "❌"
	case NodeRunning:
		return "🔄"
	case NodeCanceled:
		return "⏹️"
	default:
		return "⏳"
	}
}

// generateIndexTitle 生成索引博客标题（存储到根任务文件夹下）
func (e *TaskExecutor) generateIndexTitle() string {
	root := e.graph.Root
	date := time.Now().Format("20060102")
	rootTitle := sanitizeFolderName(truncateTitle(root.Title, 20))
	return fmt.Sprintf("agent_tasks/%s_%s/index", date, rootTitle)
}
