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
	storage.loadAllTaskGraphs()
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
