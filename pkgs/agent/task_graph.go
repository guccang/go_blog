package agent

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// ============================================================================
// TaskGraph - 任务执行图
// ============================================================================

// GraphEdge 图边（节点关系）
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // parent_child / dependency
}

// TaskGraph 任务执行图
type TaskGraph struct {
	RootID string               `json:"root_id"`
	Root   *TaskNode            `json:"-"` // 根节点引用（不序列化）
	Nodes  map[string]*TaskNode `json:"-"` // 所有节点索引（不序列化）
	Edges  []GraphEdge          `json:"edges"`

	// 实时状态
	ActiveNodes []string `json:"active_nodes"` // 当前执行中的节点
	TotalNodes  int      `json:"total_nodes"`
	DoneNodes   int      `json:"done_nodes"`
	FailedNodes int      `json:"failed_nodes"`

	// 执行统计
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// 配置
	Config *ExecutionConfig `json:"config"`

	// 并发控制
	mu sync.RWMutex `json:"-"`
}

// NewTaskGraph 创建任务图
func NewTaskGraph(root *TaskNode, config *ExecutionConfig) *TaskGraph {
	if config == nil {
		config = DefaultExecutionConfig()
	}

	g := &TaskGraph{
		RootID:      root.ID,
		Root:        root,
		Nodes:       make(map[string]*TaskNode),
		Edges:       []GraphEdge{},
		ActiveNodes: []string{},
		StartTime:   time.Now(),
		Config:      config,
	}

	// 初始化节点索引
	g.indexNode(root)

	return g
}

// indexNode 递归索引节点
func (g *TaskGraph) indexNode(node *TaskNode) {
	g.Nodes[node.ID] = node
	g.TotalNodes++

	for _, child := range node.Children {
		g.Edges = append(g.Edges, GraphEdge{
			From: node.ID,
			To:   child.ID,
			Type: "parent_child",
		})
		g.indexNode(child)
	}

	// 添加依赖边
	for _, depID := range node.DependsOn {
		g.Edges = append(g.Edges, GraphEdge{
			From: depID,
			To:   node.ID,
			Type: "dependency",
		})
	}
}

// AddNode 添加节点到图
func (g *TaskGraph) AddNode(node *TaskNode) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Nodes[node.ID] = node
	g.TotalNodes++

	// 添加父子边
	if node.ParentID != "" {
		g.Edges = append(g.Edges, GraphEdge{
			From: node.ParentID,
			To:   node.ID,
			Type: "parent_child",
		})
	}

	// 添加依赖边
	for _, depID := range node.DependsOn {
		g.Edges = append(g.Edges, GraphEdge{
			From: depID,
			To:   node.ID,
			Type: "dependency",
		})
	}
}

// GetNode 获取节点
func (g *TaskGraph) GetNode(nodeID string) *TaskNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Nodes[nodeID]
}

// UpdateNodeStatus 更新节点状态
func (g *TaskGraph) UpdateNodeStatus(nodeID string, status NodeStatus) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.Nodes[nodeID]
	if !ok {
		return
	}

	oldStatus := node.Status
	node.SetStatus(status)

	// 更新统计
	if status == NodeRunning {
		g.addActiveNode(nodeID)
	} else {
		g.removeActiveNode(nodeID)
	}

	if status == NodeDone && oldStatus != NodeDone {
		g.DoneNodes++
	}
	if status == NodeFailed && oldStatus != NodeFailed {
		g.FailedNodes++
	}
}

// addActiveNode 添加活跃节点
func (g *TaskGraph) addActiveNode(nodeID string) {
	for _, id := range g.ActiveNodes {
		if id == nodeID {
			return
		}
	}
	g.ActiveNodes = append(g.ActiveNodes, nodeID)
}

// removeActiveNode 移除活跃节点
func (g *TaskGraph) removeActiveNode(nodeID string) {
	for i, id := range g.ActiveNodes {
		if id == nodeID {
			g.ActiveNodes = append(g.ActiveNodes[:i], g.ActiveNodes[i+1:]...)
			return
		}
	}
}

// GetActiveNodes 获取活跃节点
func (g *TaskGraph) GetActiveNodes() []*TaskNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*TaskNode, 0, len(g.ActiveNodes))
	for _, id := range g.ActiveNodes {
		if node, ok := g.Nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetChildren 获取子节点
func (g *TaskGraph) GetChildren(nodeID string) []*TaskNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.Nodes[nodeID]
	if !ok {
		return nil
	}
	return node.Children
}

// GetParent 获取父节点
func (g *TaskGraph) GetParent(nodeID string) *TaskNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.Nodes[nodeID]
	if !ok {
		return nil
	}
	if node.ParentID == "" {
		return nil
	}
	return g.Nodes[node.ParentID]
}

// GetSiblings 获取兄弟节点
func (g *TaskGraph) GetSiblings(nodeID string) []*TaskNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.Nodes[nodeID]
	if !ok {
		return nil
	}

	parent := g.Nodes[node.ParentID]
	if parent == nil {
		return nil
	}

	siblings := make([]*TaskNode, 0)
	for _, child := range parent.Children {
		if child.ID != nodeID {
			siblings = append(siblings, child)
		}
	}
	return siblings
}

// GetCompletedSiblings 获取已完成的兄弟节点
func (g *TaskGraph) GetCompletedSiblings(nodeID string) []*TaskNode {
	siblings := g.GetSiblings(nodeID)
	completed := make([]*TaskNode, 0)
	for _, s := range siblings {
		if s.Status == NodeDone {
			completed = append(completed, s)
		}
	}
	return completed
}

// CalculateProgress 计算整体进度
func (g *TaskGraph) CalculateProgress() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.TotalNodes == 0 {
		return 0
	}

	// 只统计叶子节点的进度
	leafNodes := g.getLeafNodes()
	if len(leafNodes) == 0 {
		return 0
	}

	var totalProgress float64
	for _, node := range leafNodes {
		switch node.Status {
		case NodeDone:
			totalProgress += 100
		case NodeFailed, NodeCanceled, NodeSkipped:
			totalProgress += 100 // 算作完成
		default:
			totalProgress += node.Progress
		}
	}

	return totalProgress / float64(len(leafNodes))
}

// getLeafNodes 获取所有叶子节点
func (g *TaskGraph) getLeafNodes() []*TaskNode {
	leaves := make([]*TaskNode, 0)
	for _, node := range g.Nodes {
		if len(node.Children) == 0 {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// IsComplete 检查是否完成
func (g *TaskGraph) IsComplete() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.Root.Status == NodeDone ||
		g.Root.Status == NodeFailed ||
		g.Root.Status == NodeCanceled
}

// MarkComplete 标记完成
func (g *TaskGraph) MarkComplete() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	g.EndTime = &now
}

// GetExecutionTime 获取执行时间
func (g *TaskGraph) GetExecutionTime() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.EndTime != nil {
		return g.EndTime.Sub(g.StartTime)
	}
	return time.Since(g.StartTime)
}

// GetAllLogs 获取所有日志（按时间排序）
func (g *TaskGraph) GetAllLogs() []ExecutionLog {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var allLogs []ExecutionLog
	for _, node := range g.Nodes {
		allLogs = append(allLogs, node.Logs...)
	}

	// 按时间排序（使用高效的标准库排序）
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Time.Before(allLogs[j].Time)
	})

	return allLogs
}

// ResetFailedNodes 重置失败/取消的节点为待执行状态（用于重试）
// 保留已完成节点的 Context 和 Result，仅重置失败节点
// 返回重置的节点数量
func (g *TaskGraph) ResetFailedNodes() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := 0
	for _, node := range g.Nodes {
		if node.Status == NodeFailed || node.Status == NodeCanceled {
			node.Status = NodePending
			node.Progress = 0
			// 保留 node.Context（上下文）和 node.Result（可能有部分结果）
			node.AddLog(LogInfo, "retry", "节点已重置，准备重试")
			count++
		}
	}

	// 更新图状态
	if count > 0 {
		if g.Root != nil {
			g.Root.Status = NodeRunning
		}
		g.EndTime = nil // 清除结束时间
		g.FailedNodes = 0
	}

	return count
}

// ============================================================================
// 可视化导出
// ============================================================================

// GraphVisualization 图可视化数据
type GraphVisualization struct {
	Nodes []VisNode   `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Stats GraphStats  `json:"stats"`
}

// VisNode 可视化节点
type VisNode struct {
	ID            string        `json:"id"`
	ParentID      string        `json:"parent_id,omitempty"`
	Title         string        `json:"title"`
	Status        NodeStatus    `json:"status"`
	Progress      float64       `json:"progress"`
	Depth         int           `json:"depth"`
	ExecutionMode ExecutionMode `json:"execution_mode"`
	HasChildren   bool          `json:"has_children"`
	Duration      string        `json:"duration,omitempty"`
	Error         string        `json:"error,omitempty"`
}

// GraphStats 图统计信息
type GraphStats struct {
	TotalNodes  int     `json:"total_nodes"`
	DoneNodes   int     `json:"done_nodes"`
	FailedNodes int     `json:"failed_nodes"`
	ActiveNodes int     `json:"active_nodes"`
	Progress    float64 `json:"progress"`
	ExecutionMs int64   `json:"execution_ms"`
	MaxDepth    int     `json:"max_depth"`
}

// ToVisualization 导出可视化数据
func (g *TaskGraph) ToVisualization() *GraphVisualization {
	g.mu.RLock()
	defer g.mu.RUnlock()

	vis := &GraphVisualization{
		Nodes: make([]VisNode, 0, len(g.Nodes)),
		Edges: g.Edges,
		Stats: GraphStats{
			TotalNodes:  g.TotalNodes,
			DoneNodes:   g.DoneNodes,
			FailedNodes: g.FailedNodes,
			ActiveNodes: len(g.ActiveNodes),
			Progress:    g.CalculateProgress(),
			ExecutionMs: g.GetExecutionTime().Milliseconds(),
		},
	}

	maxDepth := 0
	for _, node := range g.Nodes {
		vn := VisNode{
			ID:            node.ID,
			ParentID:      node.ParentID,
			Title:         node.Title,
			Status:        node.Status,
			Progress:      node.Progress,
			Depth:         node.Depth,
			ExecutionMode: node.ExecutionMode,
			HasChildren:   len(node.Children) > 0,
		}

		if node.Duration > 0 {
			vn.Duration = node.Duration.String()
		}
		if node.Result != nil && node.Result.Error != "" {
			vn.Error = node.Result.Error
		}
		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}

		vis.Nodes = append(vis.Nodes, vn)
	}

	vis.Stats.MaxDepth = maxDepth
	return vis
}

// ToJSON 导出 JSON
func (g *TaskGraph) ToJSON() string {
	vis := g.ToVisualization()
	data, _ := json.MarshalIndent(vis, "", "  ")
	return string(data)
}

// ============================================================================
// 任务通知扩展
// ============================================================================

// GraphNotification 图更新通知
type GraphNotification struct {
	Type     string              `json:"type"` // graph_update / node_started / node_progress / node_completed / node_failed
	NodeID   string              `json:"node_id,omitempty"`
	Node     *VisNode            `json:"node,omitempty"`
	Stats    *GraphStats         `json:"stats,omitempty"`
	Log      *ExecutionLog       `json:"log,omitempty"`
	FullData *GraphVisualization `json:"full_data,omitempty"`
}

// NewGraphNotification 创建图通知
func NewGraphNotification(notifType string, nodeID string) *GraphNotification {
	return &GraphNotification{
		Type:   notifType,
		NodeID: nodeID,
	}
}

// WithNode 附加节点信息
func (n *GraphNotification) WithNode(node *TaskNode) *GraphNotification {
	n.Node = &VisNode{
		ID:            node.ID,
		ParentID:      node.ParentID,
		Title:         node.Title,
		Status:        node.Status,
		Progress:      node.Progress,
		Depth:         node.Depth,
		ExecutionMode: node.ExecutionMode,
		HasChildren:   len(node.Children) > 0,
	}
	if node.Duration > 0 {
		n.Node.Duration = node.Duration.String()
	}
	return n
}

// WithStats 附加统计信息
func (n *GraphNotification) WithStats(g *TaskGraph) *GraphNotification {
	n.Stats = &GraphStats{
		TotalNodes:  g.TotalNodes,
		DoneNodes:   g.DoneNodes,
		FailedNodes: g.FailedNodes,
		ActiveNodes: len(g.ActiveNodes),
		Progress:    g.CalculateProgress(),
		ExecutionMs: g.GetExecutionTime().Milliseconds(),
	}
	return n
}

// WithLog 附加日志
func (n *GraphNotification) WithLog(log *ExecutionLog) *GraphNotification {
	n.Log = log
	return n
}

// WithFullData 附加完整图数据
func (n *GraphNotification) WithFullData(g *TaskGraph) *GraphNotification {
	n.FullData = g.ToVisualization()
	return n
}

// ToJSON 转换为 JSON
func (n *GraphNotification) ToJSON() string {
	data, _ := json.Marshal(n)
	return string(data)
}

// MarshalJSON 自定义 TaskGraph JSON 序列化
func (g *TaskGraph) MarshalJSON() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 收集所有节点为列表
	nodeList := make([]*TaskNode, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		nodeList = append(nodeList, node)
	}

	// 构建可序列化的结构
	type Alias TaskGraph
	return json.Marshal(&struct {
		Root  *TaskNode   `json:"root"`
		Nodes []*TaskNode `json:"nodes"`
		*Alias
	}{
		Root:  g.Root,
		Nodes: nodeList,
		Alias: (*Alias)(g),
	})
}
