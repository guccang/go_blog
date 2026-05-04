package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"uap"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPServerState 单个 MCP 服务器运行时状态
type MCPServerState struct {
	Name      string
	Config    MCPServerConfig
	Client    client.MCPClient
	Tools     []mcp.Tool // MCP 原始工具定义
	Available bool
	mu        sync.RWMutex
	cancel    context.CancelFunc // 停止管理 goroutine
}

// MCPManager 管理所有 MCP 服务器连接
type MCPManager struct {
	servers    map[string]*MCPServerState // server_name → state
	toolIndex  map[string]string          // "mcp.read_file" → "filesystem"
	toolPrefix string
	httpClient *http.Client // 共享的调优 HTTP 客户端
	mu         sync.RWMutex
}

// NewMCPManager 创建 MCPManager
func NewMCPManager(prefix string) *MCPManager {
	return &MCPManager{
		servers:    make(map[string]*MCPServerState),
		toolIndex:  make(map[string]string),
		toolPrefix: prefix,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout:  10 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
			Timeout: 60 * time.Second,
		},
	}
}

// StartServer 启动单个 MCP Server 连接
func (m *MCPManager) StartServer(name string, cfg MCPServerConfig) error {
	if !cfg.Enabled {
		log.Printf("[MCPManager] server %s disabled, skip", name)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	state := &MCPServerState{
		Name:   name,
		Config: cfg,
		cancel: cancel,
	}

	// 创建客户端并初始化
	if err := m.connectServer(ctx, state); err != nil {
		cancel()
		return fmt.Errorf("connect %s: %w", name, err)
	}

	m.mu.Lock()
	m.servers[name] = state
	m.mu.Unlock()

	// 后台监控，崩溃/断连时自动重连
	switch cfg.Transport {
	case "stdio":
		go m.watchStdioServer(ctx, state)
	case "http":
		if interval := cfg.EffectiveHealthCheckSec(); interval > 0 {
			go m.watchHttpServer(ctx, state, interval)
		}
	}

	log.Printf("[MCPManager] server %s started, %d tools discovered", name, len(state.Tools))
	return nil
}

// connectServer 连接并初始化单个 MCP Server
func (m *MCPManager) connectServer(ctx context.Context, state *MCPServerState) error {
	cfg := state.Config
	var mcpClient client.MCPClient
	var err error

	switch cfg.Transport {
	case "stdio":
		// 构建环境变量
		var env []string
		for k, v := range cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		mcpClient, err = client.NewStdioMCPClient(cfg.Command, env, cfg.Args...)
		if err != nil {
			return fmt.Errorf("create stdio client: %w", err)
		}

	case "http":
		opts := []transport.StreamableHTTPCOption{
			transport.WithHTTPBasicClient(m.httpClient),
			transport.WithHTTPTimeout(30 * time.Second),
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(cfg.Headers))
		}
		mcpClient, err = client.NewStreamableHttpClient(cfg.URL, opts...)
		if err != nil {
			return fmt.Errorf("create http client: %w", err)
		}

	default:
		return fmt.Errorf("unsupported transport: %s", cfg.Transport)
	}

	// Initialize 握手
	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "mcp-agent",
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = mcpClient.Initialize(initCtx, initReq)
	if err != nil {
		mcpClient.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	// ListTools 发现工具
	listCtx, listCancel := context.WithTimeout(ctx, 15*time.Second)
	defer listCancel()

	toolsResult, err := mcpClient.ListTools(listCtx, mcp.ListToolsRequest{})
	if err != nil {
		mcpClient.Close()
		return fmt.Errorf("list tools: %w", err)
	}

	state.mu.Lock()
	state.Client = mcpClient
	state.Tools = toolsResult.Tools
	state.Available = true
	state.mu.Unlock()

	for _, t := range toolsResult.Tools {
		log.Printf("[MCPManager] %s: discovered tool %s", state.Name, t.Name)
	}

	return nil
}

// watchStdioServer 监控 stdio 子进程，崩溃时指数退避重启
func (m *MCPManager) watchStdioServer(ctx context.Context, state *MCPServerState) {
	backoff := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}

		// 尝试 ping 检测存活
		state.mu.RLock()
		c := state.Client
		state.mu.RUnlock()

		if c == nil {
			continue
		}

		pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.Ping(pingCtx)
		pingCancel()

		if err == nil {
			backoff = 0
			continue
		}

		log.Printf("[MCPManager] %s ping failed: %v, restarting...", state.Name, err)

		state.mu.Lock()
		state.Available = false
		if state.Client != nil {
			state.Client.Close()
			state.Client = nil
		}
		state.mu.Unlock()

		// 指数退避重启
		delay := time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(backoff)), 60)) * time.Second
		if delay < time.Second {
			delay = time.Second
		}
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		backoff++

		log.Printf("[MCPManager] %s waiting %v before restart...", state.Name, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		if err := m.connectServer(ctx, state); err != nil {
			log.Printf("[MCPManager] %s restart failed: %v", state.Name, err)
		} else {
			log.Printf("[MCPManager] %s restarted successfully", state.Name)
			backoff = 0
		}
	}
}

// watchHttpServer 监控 HTTP 服务器连接，定期 ping，失败时指数退避重连
func (m *MCPManager) watchHttpServer(ctx context.Context, state *MCPServerState, intervalSec int) {
	interval := time.Duration(intervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	backoff := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		state.mu.RLock()
		c := state.Client
		state.mu.RUnlock()

		if c == nil {
			continue
		}

		pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.Ping(pingCtx)
		pingCancel()

		if err == nil {
			backoff = 0
			continue
		}

		log.Printf("[MCPManager] %s ping failed: %v, reconnecting...", state.Name, err)

		state.mu.Lock()
		state.Available = false
		if state.Client != nil {
			state.Client.Close()
			state.Client = nil
		}
		state.mu.Unlock()

		// 指数退避重连
		delay := time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(backoff)), 60)) * time.Second
		if delay < interval {
			delay = interval
		}
		backoff++

		log.Printf("[MCPManager] %s waiting %v before reconnect...", state.Name, delay)
		ticker.Reset(delay) // 延迟下次检查

		if err := m.connectServer(ctx, state); err != nil {
			log.Printf("[MCPManager] %s reconnect failed: %v", state.Name, err)
		} else {
			log.Printf("[MCPManager] %s reconnected, %d tools available", state.Name, len(state.Tools))
			backoff = 0
			ticker.Reset(interval) // 恢复正常间隔
		}
	}
}

// StopServer 停止单个 MCP Server
func (m *MCPManager) StopServer(name string) {
	m.mu.Lock()
	state, exists := m.servers[name]
	if exists {
		delete(m.servers, name)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	state.cancel()
	state.mu.Lock()
	if state.Client != nil {
		state.Client.Close()
		state.Client = nil
	}
	state.Available = false
	state.mu.Unlock()

	log.Printf("[MCPManager] server %s stopped", name)
}

// StopAll 停止所有 MCP Server
func (m *MCPManager) StopAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.StopServer(name)
	}
}

// BuildUAPTools 汇总所有工具转为 UAP 格式
func (m *MCPManager) BuildUAPTools() []uap.ToolDef {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 第一遍：统计工具名出现次数
	nameCount := make(map[string]int)
	type toolEntry struct {
		serverName string
		tool       mcp.Tool
	}
	var allTools []toolEntry

	for serverName, state := range m.servers {
		state.mu.RLock()
		for _, t := range state.Tools {
			nameCount[t.Name]++
			allTools = append(allTools, toolEntry{serverName: serverName, tool: t})
		}
		state.mu.RUnlock()
	}

	// 第二遍：构建 UAP 工具定义，同名消歧
	newIndex := make(map[string]string)
	var uapTools []uap.ToolDef

	for _, entry := range allTools {
		var prefixedName string
		if nameCount[entry.tool.Name] > 1 {
			// 冲突：mcp.{server}_{tool}
			prefixedName = fmt.Sprintf("%s.%s_%s", m.toolPrefix, entry.serverName, entry.tool.Name)
		} else {
			// 唯一：mcp.{tool}
			prefixedName = fmt.Sprintf("%s.%s", m.toolPrefix, entry.tool.Name)
		}

		uapTools = append(uapTools, mcpToolToUAPToolDef(entry.tool, prefixedName))
		newIndex[prefixedName] = entry.serverName
	}

	m.toolIndex = newIndex

	log.Printf("[MCPManager] built %d UAP tools", len(uapTools))
	return uapTools
}

// CallTool 代理工具调用
func (m *MCPManager) CallTool(ctx context.Context, prefixedName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	serverName, exists := m.toolIndex[prefixedName]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("unknown tool: %s", prefixedName)
	}

	m.mu.RLock()
	state, exists := m.servers[serverName]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("server %s not found", serverName)
	}

	state.mu.RLock()
	available := state.Available
	c := state.Client
	state.mu.RUnlock()

	if !available || c == nil {
		return "", fmt.Errorf("server %s not available", serverName)
	}

	// 剥离前缀，还原原始工具名
	originalName := stripPrefix(prefixedName, m.toolPrefix, serverName)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = originalName
	callReq.Params.Arguments = args

	// HTTP 传输：带重试机制
	if state.Config.Transport == "http" {
		return m.callToolWithRetry(ctx, c, state.Config, callReq, originalName, serverName)
	}

	result, err := c.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("call tool %s on %s: %w", originalName, serverName, err)
	}

	if result.IsError {
		return "", fmt.Errorf("tool %s returned error: %s", originalName, mcpResultToString(result))
	}

	return mcpResultToString(result), nil
}

// callToolWithRetry HTTP 传输的工具调用（指数退避重试）
func (m *MCPManager) callToolWithRetry(ctx context.Context, c client.MCPClient,
	cfg MCPServerConfig, callReq mcp.CallToolRequest, toolName, serverName string) (string, error) {

	maxRetries := cfg.EffectiveRetryCount()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(3, float64(attempt-1))) * time.Second
			log.Printf("[MCPManager] retry %d/%d for %s after %v", attempt, maxRetries, toolName, delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := c.CallTool(ctx, callReq)
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return "", fmt.Errorf("call tool %s on %s: %w", toolName, serverName, err)
		}

		if result.IsError {
			return "", fmt.Errorf("tool %s returned error: %s", toolName, mcpResultToString(result))
		}

		return mcpResultToString(result), nil
	}

	return "", fmt.Errorf("call tool %s on %s (retried %d times): %w", toolName, serverName, maxRetries, lastErr)
}

// isRetryableError 判断错误是否可重试（网络类错误，非工具逻辑错误）
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// context 超时
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// 检查 net.Error 接口
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// 包含常见网络错误关键字
	msg := err.Error()
	for _, keyword := range []string{
		"connection reset", "connection refused", "broken pipe",
		"timeout", "EOF", "no such host",
	} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// stripPrefix 从带前缀的工具名还原原始名
// "mcp.read_file" → "read_file"
// "mcp.filesystem_read_file" → "read_file"（当 serverName 为 "filesystem" 时）
func stripPrefix(prefixedName, prefix, serverName string) string {
	// 去掉 "mcp." 前缀
	name := strings.TrimPrefix(prefixedName, prefix+".")

	// 尝试去掉 "{server}_" 前缀（消歧格式）
	serverPrefix := serverName + "_"
	if strings.HasPrefix(name, serverPrefix) {
		return strings.TrimPrefix(name, serverPrefix)
	}

	return name
}

// mcpToolToUAPToolDef 将 MCP Tool 转为 UAP ToolDef
func mcpToolToUAPToolDef(tool mcp.Tool, prefixedName string) uap.ToolDef {
	// 将 InputSchema 序列化为 JSON
	var params json.RawMessage
	if tool.RawInputSchema != nil {
		params = tool.RawInputSchema
	} else {
		params, _ = json.Marshal(tool.InputSchema)
	}

	return uap.ToolDef{
		Name:        prefixedName,
		Description: tool.Description,
		Parameters:  params,
	}
}

// mcpResultToString 将 MCP CallToolResult 转为字符串
func mcpResultToString(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}

	var parts []string
	for _, content := range result.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			parts = append(parts, c.Text)
		case *mcp.TextContent:
			parts = append(parts, c.Text)
		default:
			// 非文本内容序列化为 JSON
			data, _ := json.Marshal(c)
			parts = append(parts, string(data))
		}
	}

	return strings.Join(parts, "\n")
}
