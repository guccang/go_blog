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
// 注意：实际缓存使用下方的全局变量 graphCache，支持多账户共享
type TaskStorage struct {
	account string
}

// NewTaskStorage 创建任务存储
func NewTaskStorage(account string) *TaskStorage {
	storage := &TaskStorage{
		account: account,
	}
	storage.loadAllTaskGraphs()
	return storage
}

// ============================================================================
// TaskGraph 存储
// ============================================================================

// graphCache 任务图缓存
var graphCache = make(map[string]*TaskGraph)
var graphCacheMu sync.RWMutex

// loadAllTaskGraphs 加载所有任务图到缓存
func (s *TaskStorage) loadAllTaskGraphs() {
	blogs := control.GetAll(s.account, 0, module.EAuthType_all)
	count := 0
	for _, blog := range blogs {
		if strings.HasPrefix(blog.Title, "agent_graph_") {
			rootID := strings.TrimPrefix(blog.Title, "agent_graph_")
			// 加载任务图
			graph := s.GetTaskGraph(rootID)
			if graph != nil {
				count++
			}
		}
	}
	log.MessageF(log.ModuleAgent, "Loaded %d task graphs from storage", count)
}

// SaveTaskGraph 保存任务图
func (s *TaskStorage) SaveTaskGraph(graph *TaskGraph) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}

	// 更新缓存
	graphCacheMu.Lock()
	graphCache[graph.RootID] = graph
	graphCacheMu.Unlock()

	// 序列化图数据（不包含 Root 和 Nodes 引用）
	graphData := struct {
		RootID      string           `json:"root_id"`
		Edges       []GraphEdge      `json:"edges"`
		TotalNodes  int              `json:"total_nodes"`
		DoneNodes   int              `json:"done_nodes"`
		FailedNodes int              `json:"failed_nodes"`
		Config      *ExecutionConfig `json:"config"`
		Nodes       []*TaskNode      `json:"nodes"`
	}{
		RootID:      graph.RootID,
		Edges:       graph.Edges,
		TotalNodes:  graph.TotalNodes,
		DoneNodes:   graph.DoneNodes,
		FailedNodes: graph.FailedNodes,
		Config:      graph.Config,
	}

	// 收集所有节点
	for _, node := range graph.Nodes {
		graphData.Nodes = append(graphData.Nodes, node)
	}

	content, err := json.MarshalIndent(graphData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化任务图失败: %w", err)
	}

	title := s.getGraphBlogTitle(graph.RootID)
	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     "agent|graph",
		AuthType: module.EAuthType_private,
		Account:  s.account,
	}

	if control.GetBlog(s.account, title) == nil {
		control.AddBlog(s.account, ubd)
	} else {
		control.ModifyBlog(s.account, ubd)
	}

	log.DebugF(log.ModuleAgent, "TaskGraph saved: %s", graph.RootID)
	return nil
}

// GetTaskGraph 获取任务图
func (s *TaskStorage) GetTaskGraph(rootID string) *TaskGraph {
	// 先从缓存获取
	graphCacheMu.RLock()
	if graph, ok := graphCache[rootID]; ok {
		graphCacheMu.RUnlock()
		return graph
	}
	graphCacheMu.RUnlock()

	// 从 blog 加载
	title := s.getGraphBlogTitle(rootID)
	blog := control.GetBlog(s.account, title)
	if blog == nil {
		return nil
	}

	// 解析数据
	var graphData struct {
		RootID      string           `json:"root_id"`
		Edges       []GraphEdge      `json:"edges"`
		TotalNodes  int              `json:"total_nodes"`
		DoneNodes   int              `json:"done_nodes"`
		FailedNodes int              `json:"failed_nodes"`
		Config      *ExecutionConfig `json:"config"`
		Nodes       []*TaskNode      `json:"nodes"`
	}

	if err := json.Unmarshal([]byte(blog.Content), &graphData); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse task graph: %v", err)
		return nil
	}

	// 重建图结构
	var root *TaskNode
	nodes := make(map[string]*TaskNode)
	for _, node := range graphData.Nodes {
		// 重新初始化 channels
		node.pauseCh = make(chan struct{}, 1)
		node.cancelCh = make(chan struct{}, 1)
		nodes[node.ID] = node
		if node.ID == graphData.RootID {
			root = node
		}
	}

	if root == nil {
		return nil
	}

	// 重建父子关系
	for _, node := range nodes {
		if node.ParentID != "" {
			if parent, ok := nodes[node.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			}
		}
	}

	graph := &TaskGraph{
		RootID:      graphData.RootID,
		Root:        root,
		Nodes:       nodes,
		Edges:       graphData.Edges,
		TotalNodes:  graphData.TotalNodes,
		DoneNodes:   graphData.DoneNodes,
		FailedNodes: graphData.FailedNodes,
		Config:      graphData.Config,
	}

	// 更新缓存
	graphCacheMu.Lock()
	graphCache[rootID] = graph
	graphCacheMu.Unlock()

	return graph
}

// DeleteTaskGraph 删除任务图
func (s *TaskStorage) DeleteTaskGraph(rootID string) error {
	// 从缓存删除
	graphCacheMu.Lock()
	delete(graphCache, rootID)
	graphCacheMu.Unlock()

	// 从 blog 删除
	title := s.getGraphBlogTitle(rootID)
	control.DeleteBlog(s.account, title)

	log.DebugF(log.ModuleAgent, "TaskGraph deleted: %s", rootID)
	return nil
}

// getGraphBlogTitle 获取任务图的 blog 标题
func (s *TaskStorage) getGraphBlogTitle(rootID string) string {
	return fmt.Sprintf("agent_graph_%s", rootID)
}

// GetAllGraphs 获取所有任务图
func (s *TaskStorage) GetAllGraphs() []*TaskGraph {
	graphCacheMu.RLock()
	defer graphCacheMu.RUnlock()

	var graphs []*TaskGraph
	for _, graph := range graphCache {
		graphs = append(graphs, graph)
	}
	return graphs
}

// GetTaskGraphsByAccount 获取指定账户的所有任务图
func (s *TaskStorage) GetTaskGraphsByAccount(account string) []*TaskGraph {
	graphCacheMu.RLock()
	defer graphCacheMu.RUnlock()

	var graphs []*TaskGraph
	for _, graph := range graphCache {
		if graph.Root != nil && graph.Root.Account == account {
			graphs = append(graphs, graph)
		}
	}

	// 按创建时间降序排序（最新的在前面）
	sort.Slice(graphs, func(i, j int) bool {
		ti := graphs[i].Root.CreatedAt
		tj := graphs[j].Root.CreatedAt
		return ti.After(tj)
	})

	return graphs
}

// GetPendingTaskGraphs 获取待处理的任务图（用于恢复）
func (s *TaskStorage) GetPendingTaskGraphs() []*TaskGraph {
	graphCacheMu.RLock()
	defer graphCacheMu.RUnlock()

	var pending []*TaskGraph
	for _, graph := range graphCache {
		if graph.Root != nil {
			status := graph.Root.GetStatus()
			if status == NodePending || status == NodeRunning {
				pending = append(pending, graph)
			}
		}
	}
	return pending
}
