package main

// FieldType represents the type of a configuration field.
type FieldType int

const (
	FieldString FieldType = iota
	FieldInt
	FieldBool
	FieldURL
	FieldPort
	FieldPath
	FieldStringSlice
	FieldMap
)

func (ft FieldType) String() string {
	switch ft {
	case FieldString:
		return "string"
	case FieldInt:
		return "int"
	case FieldBool:
		return "bool"
	case FieldURL:
		return "url"
	case FieldPort:
		return "port"
	case FieldPath:
		return "path"
	case FieldStringSlice:
		return "[]string"
	case FieldMap:
		return "map"
	default:
		return "unknown"
	}
}

// ConfigField defines a single configuration field for an agent.
type ConfigField struct {
	Key          string    `json:"key"`
	Label        string    `json:"label"`
	Description  string    `json:"description"`
	Type         FieldType `json:"type"`
	Required     bool      `json:"required"`
	DefaultValue any       `json:"default_value"`
	Validate     string    `json:"validate"`
	Group        string    `json:"group"` // "gateway", "agent", "custom"
	Shared       bool      `json:"shared"`
}

// AgentSchema defines the configuration schema for one agent.
type AgentSchema struct {
	Name           string                `json:"name"`
	ConfigFileName string                `json:"config_file_name"`
	Dir            string                `json:"dir"`
	Description    string                `json:"description"`
	Fields         []ConfigField         `json:"fields"`
	DefaultPort    int                   `json:"default_port"`
	ConfigFormat   string                `json:"config_format"` // "json" or "keyvalue"
	Dependencies   []SoftwareRequirement `json:"dependencies"`  // agent 运行所需软件
}

// SharedFields returns fields common to most agents.
func SharedFields() []ConfigField {
	return []ConfigField{
		{
			Key: "server_url", Label: "Gateway WebSocket URL",
			Description: "UAP WebSocket 连接地址", Type: FieldURL,
			Required: true, DefaultValue: "ws://127.0.0.1:10086/ws/uap",
			Group: "gateway", Shared: true,
		},
		{
			Key: "auth_token", Label: "Auth Token",
			Description: "Gateway 认证令牌", Type: FieldString,
			Required: false, DefaultValue: "",
			Group: "gateway", Shared: true,
		},
		{
			Key: "agent_name", Label: "Agent Name",
			Description: "Agent 显示名称（留空则使用默认值）", Type: FieldString,
			Required: false, DefaultValue: "",
			Group: "agent", Shared: true,
		},
		{
			Key: "max_concurrent", Label: "最大并发数",
			Description: "最大并发任务数", Type: FieldInt,
			Required: false, DefaultValue: 3,
			Group: "agent", Shared: true,
		},
	}
}

// AllAgentSchemas returns the schema definitions for all agents in the monorepo.
func AllAgentSchemas() []AgentSchema {
	shared := SharedFields()

	return []AgentSchema{
		{
			Name: "gateway", ConfigFileName: "gateway.json",
			Dir: "cmd/gateway", Description: "中央网关，WebSocket 路由和 HTTP 反向代理",
			DefaultPort: 10086,
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
				{Software: "node", MinVersion: "18.0.0"},
			},
			Fields: []ConfigField{
				{Key: "port", Label: "监听端口", Description: "Gateway 监听端口", Type: FieldPort, Required: true, DefaultValue: 10086, Group: "gateway"},
				{Key: "go_backend_url", Label: "后端 URL", Description: "Go 后端上游地址", Type: FieldURL, Required: true, DefaultValue: "http://127.0.0.1:8080", Group: "gateway"},
				{Key: "auth_token", Label: "Auth Token", Description: "Agent 认证令牌", Type: FieldString, Required: false, DefaultValue: "", Group: "gateway"},
				{Key: "event_tracking", Label: "事件追踪", Description: "是否启用事件追踪", Type: FieldBool, Required: false, DefaultValue: true, Group: "custom"},
				{Key: "event_buffer_size", Label: "事件缓冲区", Description: "事件缓冲容量", Type: FieldInt, Required: false, DefaultValue: 10000, Group: "custom"},
				{Key: "event_log_dir", Label: "事件日志目录", Description: "JSONL 事件日志目录", Type: FieldPath, Required: false, DefaultValue: "logs", Group: "custom"},
				{Key: "event_log_stdout", Label: "日志到标准输出", Description: "事件日志输出到 stdout", Type: FieldBool, Required: false, DefaultValue: true, Group: "custom"},
			},
		},
		{
			Name: "blog-agent", ConfigFileName: "sys_conf.md",
			Dir: "cmd/blog-agent", Description: "博客系统后端（key=value 格式配置）",
			DefaultPort: 8080, ConfigFormat: "keyvalue",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
				{Software: "redis", MinVersion: "6.0.0"},
			},
			Fields: []ConfigField{
				{Key: "admin", Label: "管理员账号", Description: "管理员用户名", Type: FieldString, Required: true, DefaultValue: "admin", Group: "custom"},
				{Key: "port", Label: "HTTP 端口", Description: "博客服务监听端口", Type: FieldPort, Required: true, DefaultValue: "8080", Group: "custom"},
				{Key: "redis_ip", Label: "Redis 地址", Description: "Redis 服务器 IP", Type: FieldString, Required: true, DefaultValue: "127.0.0.1", Group: "custom"},
				{Key: "redis_port", Label: "Redis 端口", Description: "Redis 端口号", Type: FieldPort, Required: true, DefaultValue: "6379", Group: "custom"},
				{Key: "redis_pwd", Label: "Redis 密码", Description: "Redis 密码（可空）", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "gateway_url", Label: "Gateway WS URL", Description: "Gateway WebSocket 地址", Type: FieldURL, Required: false, DefaultValue: "", Group: "gateway"},
				{Key: "gateway_token", Label: "Gateway Token", Description: "Gateway 认证令牌", Type: FieldString, Required: false, DefaultValue: "", Group: "gateway"},
				{Key: "logs_dir", Label: "日志目录", Description: "日志存储目录", Type: FieldPath, Required: false, DefaultValue: "", Group: "custom"},
			},
		},
		{
			Name: "env-agent", ConfigFileName: "env-agent.json",
			Dir: "cmd/env-agent", Description: "远程环境检测与软件安装 Agent",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "go_backend_agent_id", Label: "后端 Agent ID", Description: "Go 后端 agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
			}...),
		},
		{
			Name: "acp-agent", ConfigFileName: "acp-agent.json",
			Dir: "cmd/acp-agent", Description: "ACP (Anthropic Claude Protocol) 代码分析 Agent",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
				{Software: "claude"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "agent_type", Label: "Agent 类型", Description: "Agent 类型标识", Type: FieldString, Required: false, DefaultValue: "acp", Group: "agent"},
				{Key: "workspaces", Label: "工作区目录", Description: "监控的工作区目录列表（逗号分隔）", Type: FieldStringSlice, Required: true, DefaultValue: nil, Group: "custom"},
				{Key: "acp_agent_cmd", Label: "ACP 命令", Description: "ACP agent 启动命令", Type: FieldString, Required: false, DefaultValue: "npx", Group: "custom"},
				{Key: "claude_path", Label: "Claude 路径", Description: "Claude 可执行文件路径", Type: FieldPath, Required: false, DefaultValue: "claude", Group: "custom"},
				{Key: "max_turns", Label: "最大对话轮数", Description: "单次任务最大对话轮数", Type: FieldInt, Required: false, DefaultValue: 20, Group: "custom"},
				{Key: "analysis_timeout", Label: "分析超时(秒)", Description: "分析任务超时时间", Type: FieldInt, Required: false, DefaultValue: 3600, Group: "custom"},
				{Key: "go_backend_agent_id", Label: "后端 Agent ID", Description: "Go 后端 agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
			}...),
		},
		{
			Name: "codegen-agent", ConfigFileName: "codegen-agent.json",
			Dir: "cmd/codegen-agent", Description: "代码生成与部署 Agent",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
				{Software: "claude"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "agent_type", Label: "Agent 类型", Description: "Agent 类型", Type: FieldString, Required: false, DefaultValue: "codegen", Group: "agent"},
				{Key: "workspaces", Label: "工作区目录", Description: "工作区目录列表（逗号分隔）", Type: FieldStringSlice, Required: true, DefaultValue: nil, Group: "custom"},
				{Key: "claude_path", Label: "Claude 路径", Description: "Claude 可执行文件路径", Type: FieldPath, Required: false, DefaultValue: "claude", Group: "custom"},
				{Key: "opencode_path", Label: "OpenCode 路径", Description: "OpenCode 可执行文件路径", Type: FieldPath, Required: false, DefaultValue: "opencode", Group: "custom"},
				{Key: "max_turns", Label: "最大对话轮数", Description: "最大对话轮数", Type: FieldInt, Required: false, DefaultValue: 20, Group: "custom"},
				{Key: "go_backend_agent_id", Label: "后端 Agent ID", Description: "Go 后端 agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
			}...),
		},
		{
			Name: "deploy-agent", ConfigFileName: "deploy-agent.json",
			Dir: "cmd/deploy-agent", Description: "自动化部署 Agent（SSH/Bridge）",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "ssh_key", Label: "SSH 密钥路径", Description: "SSH 私钥文件路径", Type: FieldPath, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "ssh_password", Label: "SSH 密码", Description: "SSH 密码（不推荐）", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "settings_dir", Label: "配置目录", Description: "部署配置目录", Type: FieldPath, Required: false, DefaultValue: "./settings", Group: "custom"},
				{Key: "workspaces", Label: "工作区目录", Description: "工作区目录列表（逗号分隔）", Type: FieldStringSlice, Required: false, DefaultValue: nil, Group: "custom"},
				{Key: "go_backend_agent_id", Label: "后端 Agent ID", Description: "Go 后端 agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
			}...),
		},
		{
			Name: "deploy-bridge-server", ConfigFileName: "deploy-bridge-server.json",
			Dir: "cmd/deploy-bridge-server", Description: "部署桥接服务器（接收远程部署指令）",
			DefaultPort: 9090,
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: []ConfigField{
				{Key: "listen", Label: "监听地址", Description: "监听地址 (格式: :port)", Type: FieldString, Required: true, DefaultValue: ":9090", Group: "gateway"},
				{Key: "auth_token", Label: "Auth Token", Description: "认证令牌（不可为空）", Type: FieldString, Required: true, DefaultValue: "", Group: "gateway"},
				{Key: "upload_dir", Label: "上传目录", Description: "包上传存储目录", Type: FieldPath, Required: false, DefaultValue: "./uploads", Group: "custom"},
				{Key: "max_upload_size_mb", Label: "最大上传(MB)", Description: "最大上传文件大小", Type: FieldInt, Required: false, DefaultValue: 200, Group: "custom"},
				{Key: "deploy_timeout_sec", Label: "部署超时(秒)", Description: "部署操作超时", Type: FieldInt, Required: false, DefaultValue: 120, Group: "custom"},
				{Key: "log_retain_count", Label: "日志保留数", Description: "保留的日志条数", Type: FieldInt, Required: false, DefaultValue: 50, Group: "custom"},
			},
		},
		{
			Name: "execute-code-agent", ConfigFileName: "execute-code-agent.json",
			Dir: "cmd/execute-code-agent", Description: "代码执行 Agent（Python/Shell）",
			Dependencies: []SoftwareRequirement{
				{Software: "python", MinVersion: "3.6.0"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "gateway_http", Label: "Gateway HTTP URL", Description: "Gateway HTTP 地址", Type: FieldURL, Required: false, DefaultValue: "http://127.0.0.1:10086", Group: "gateway"},
				{Key: "go_backend_agent_id", Label: "后端 Agent ID", Description: "Go 后端 agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
				{Key: "python_path", Label: "Python 路径", Description: "Python 可执行文件路径（留空自动检测）", Type: FieldPath, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "max_exec_time_sec", Label: "执行超时(秒)", Description: "代码执行超时时间", Type: FieldInt, Required: false, DefaultValue: 120, Group: "custom"},
				{Key: "max_output_size", Label: "最大输出字符数", Description: "最大输出大小", Type: FieldInt, Required: false, DefaultValue: 50000, Group: "custom"},
			}...),
		},
		{
			Name: "llm-agent", ConfigFileName: "llm-agent.json",
			Dir: "cmd/llm-agent", Description: "LLM 智能 Agent（多模型、工具调用、任务分解）",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: []ConfigField{
				{Key: "gateway_url", Label: "Gateway WS URL", Description: "Gateway WebSocket 地址", Type: FieldURL, Required: true, DefaultValue: "ws://127.0.0.1:10086/ws/uap", Shared: true, Group: "gateway"},
				{Key: "gateway_http", Label: "Gateway HTTP URL", Description: "Gateway HTTP 地址", Type: FieldURL, Required: true, DefaultValue: "http://127.0.0.1:10086", Shared: true, Group: "gateway"},
				{Key: "auth_token", Label: "Auth Token", Description: "Gateway 认证令牌", Type: FieldString, Required: false, DefaultValue: "", Shared: true, Group: "gateway"},
				{Key: "agent_id", Label: "Agent ID", Description: "Agent 唯一标识", Type: FieldString, Required: false, DefaultValue: "llm-agent", Group: "agent"},
				{Key: "agent_name", Label: "Agent 名称", Description: "Agent 显示名称", Type: FieldString, Required: false, DefaultValue: "LLM MCP Agent", Group: "agent"},
				{Key: "llm.model", Label: "LLM 模型", Description: "主 LLM 模型名", Type: FieldString, Required: true, DefaultValue: "deepseek-chat", Group: "custom"},
				{Key: "llm.base_url", Label: "LLM API URL", Description: "LLM API Base URL", Type: FieldURL, Required: true, DefaultValue: "https://api.deepseek.com/v1", Group: "custom"},
				{Key: "llm.api_key", Label: "LLM API Key", Description: "LLM API 密钥", Type: FieldString, Required: true, DefaultValue: "", Group: "custom"},
				{Key: "llm.max_tokens", Label: "最大 Token 数", Description: "LLM 最大输出 token", Type: FieldInt, Required: false, DefaultValue: 8192, Group: "custom"},
				{Key: "max_concurrent", Label: "最大并发数", Description: "最大并发任务数", Type: FieldInt, Required: false, DefaultValue: 3, Shared: true, Group: "agent"},
				{Key: "max_tool_iterations", Label: "工具迭代上限", Description: "工具调用最大迭代次数", Type: FieldInt, Required: false, DefaultValue: 32, Group: "custom"},
				{Key: "session_dir", Label: "会话目录", Description: "会话持久化目录", Type: FieldPath, Required: false, DefaultValue: "agent_sessions", Group: "custom"},
				{Key: "workspace_dir", Label: "工作区目录", Description: "工作区目录", Type: FieldPath, Required: false, DefaultValue: "workspace", Group: "custom"},
			},
		},
		{
			Name: "log-agent", ConfigFileName: "log-agent.json",
			Dir: "cmd/log-agent", Description: "日志收集与分析 Agent",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "log_sources", Label: "日志源", Description: "日志源配置（JSON map，键为名称，值含 path 和 description）", Type: FieldMap, Required: false, DefaultValue: nil, Group: "custom"},
			}...),
		},
		{
			Name: "mcp-agent", ConfigFileName: "mcp-agent.json",
			Dir: "cmd/mcp-agent", Description: "MCP (Model Context Protocol) 工具桥接 Agent",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
				{Software: "node", MinVersion: "18.0.0"},
			},
			Fields: append(cloneFields(shared), []ConfigField{
				{Key: "gateway_http", Label: "Gateway HTTP URL", Description: "Gateway HTTP 地址", Type: FieldURL, Required: false, DefaultValue: "http://127.0.0.1:10086", Group: "gateway"},
				{Key: "tool_prefix", Label: "工具前缀", Description: "MCP 工具名前缀", Type: FieldString, Required: false, DefaultValue: "mcp", Group: "custom"},
				{Key: "tool_call_timeout_sec", Label: "工具超时(秒)", Description: "工具调用超时", Type: FieldInt, Required: false, DefaultValue: 30, Group: "custom"},
				{Key: "mcp_servers", Label: "MCP 服务器", Description: "MCP 服务器配置（JSON map）", Type: FieldMap, Required: false, DefaultValue: nil, Group: "custom"},
			}...),
		},
		{
			Name: "wechat-agent", ConfigFileName: "wechat-agent.json",
			Dir: "cmd/wechat-agent", Description: "微信集成 Agent（企业微信消息收发）",
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: []ConfigField{
				{Key: "http_port", Label: "HTTP 端口", Description: "微信回调监听端口", Type: FieldPort, Required: true, DefaultValue: 8884, Group: "custom"},
				{Key: "gateway_url", Label: "Gateway WS URL", Description: "Gateway WebSocket 地址", Type: FieldURL, Required: true, DefaultValue: "ws://127.0.0.1:10086/ws/uap", Shared: true, Group: "gateway"},
				{Key: "auth_token", Label: "Auth Token", Description: "Gateway 认证令牌", Type: FieldString, Required: false, DefaultValue: "", Shared: true, Group: "gateway"},
				{Key: "agent_name", Label: "Agent 名称", Description: "Agent 显示名称", Type: FieldString, Required: false, DefaultValue: "wechat-agent", Group: "agent"},
				{Key: "llm_agent_id", Label: "LLM Agent ID", Description: "LLM Agent 路由 ID", Type: FieldString, Required: false, DefaultValue: "", Group: "agent"},
				{Key: "backend_agent_id", Label: "后端 Agent ID", Description: "后端 Agent ID", Type: FieldString, Required: false, DefaultValue: "go_blog", Group: "agent"},
				{Key: "corp_id", Label: "企业ID", Description: "企业微信 Corp ID", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "agent_id", Label: "应用 Agent ID", Description: "企业微信应用 Agent ID", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "secret", Label: "应用 Secret", Description: "企业微信应用密钥", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "token", Label: "回调 Token", Description: "微信回调验证 Token", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "encoding_aes_key", Label: "AES Key", Description: "消息加密 AES Key", Type: FieldString, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "webhook_url", Label: "Webhook URL", Description: "群机器人 Webhook URL（可选）", Type: FieldURL, Required: false, DefaultValue: "", Group: "custom"},
			},
		},
		{
			Name: "init-agent", ConfigFileName: "init-agent.json",
			Dir: "cmd/init-agent", Description: "初始化向导（环境检测、配置生成、可用性面板）",
			DefaultPort: 9090,
			Dependencies: []SoftwareRequirement{
				{Software: "go", MinVersion: "1.23.0"},
			},
			Fields: []ConfigField{
				{Key: "mode", Label: "运行模式", Description: "运行模式: cli 或 web", Type: FieldString, Required: false, DefaultValue: "cli", Group: "custom"},
				{Key: "web_port", Label: "Web 端口", Description: "Web 模式监听端口", Type: FieldPort, Required: false, DefaultValue: 9090, Group: "custom"},
				{Key: "root_dir", Label: "Monorepo 根目录", Description: "monorepo 根目录（留空自动检测）", Type: FieldPath, Required: false, DefaultValue: "", Group: "custom"},
				{Key: "check_only", Label: "仅环境检测", Description: "仅运行环境检测", Type: FieldBool, Required: false, DefaultValue: false, Group: "custom"},
				{Key: "dashboard_only", Label: "仅可用性面板", Description: "仅显示可用性面板", Type: FieldBool, Required: false, DefaultValue: false, Group: "custom"},
				{Key: "non_interactive", Label: "非交互模式", Description: "接受所有默认值", Type: FieldBool, Required: false, DefaultValue: false, Group: "custom"},
				{Key: "server_url", Label: "Gateway WebSocket URL", Description: "向导默认 Gateway WebSocket 地址", Type: FieldURL, Required: false, DefaultValue: "ws://127.0.0.1:10086/ws/uap", Group: "gateway"},
				{Key: "gateway_http", Label: "Gateway HTTP URL", Description: "向导默认 Gateway HTTP 地址", Type: FieldURL, Required: false, DefaultValue: "http://127.0.0.1:10086", Group: "gateway"},
				{Key: "auth_token", Label: "Auth Token", Description: "向导默认 Gateway 认证令牌", Type: FieldString, Required: false, DefaultValue: "", Group: "gateway"},
			},
		},
	}
}

// GetAgentSchema finds a schema by agent name.
func GetAgentSchema(name string) *AgentSchema {
	for _, s := range AllAgentSchemas() {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

// GetNonSharedFields returns fields that are specific to this agent (not shared).
func GetNonSharedFields(schema *AgentSchema) []ConfigField {
	var fields []ConfigField
	for _, f := range schema.Fields {
		if !f.Shared {
			fields = append(fields, f)
		}
	}
	return fields
}

// GetSharedFieldKeys returns the keys of shared fields across agent schemas.
func GetSharedFieldKeys() []string {
	return []string{"server_url", "gateway_url", "auth_token", "agent_name", "max_concurrent"}
}

func cloneFields(fields []ConfigField) []ConfigField {
	out := make([]ConfigField, len(fields))
	copy(out, fields)
	return out
}

// --- Progressive Deployment: Agent Tier & Meta ---

// AgentTier represents the deployment tier of an agent.
type AgentTier int

const (
	TierCore         AgentTier = 0 // 基础设施（必须）
	TierIntelligence AgentTier = 1 // 智能层（推荐）
	TierProductivity AgentTier = 2 // 生产力（按需）
	TierSpecialized  AgentTier = 3 // 专业化（可选）
)

func (t AgentTier) String() string {
	switch t {
	case TierCore:
		return "核心"
	case TierIntelligence:
		return "智能"
	case TierProductivity:
		return "生产力"
	case TierSpecialized:
		return "专业"
	default:
		return "未知"
	}
}

// AgentMeta holds progressive-deployment metadata for an agent.
type AgentMeta struct {
	Name            string    `json:"name"`
	Tier            AgentTier `json:"tier"`
	AgentDeps       []string  `json:"agent_deps"`       // 依赖的其他 agent
	FeatureKeywords []string  `json:"feature_keywords"` // 功能关键词（用于推荐匹配）
	ShortPitch      string    `json:"short_pitch"`      // 一句话描述
}

// AgentMetaRegistry returns the progressive-deployment metadata for all agents.
func AgentMetaRegistry() map[string]AgentMeta {
	return map[string]AgentMeta{
		"gateway": {
			Name: "gateway", Tier: TierCore,
			AgentDeps:       nil,
			FeatureKeywords: []string{"网关", "路由", "gateway", "proxy", "websocket"},
			ShortPitch:      "中央路由，所有 agent 通过它通信",
		},
		"blog-agent": {
			Name: "blog-agent", Tier: TierCore,
			AgentDeps:       []string{"gateway"},
			FeatureKeywords: []string{"博客", "blog", "后端", "backend", "web", "数据", "存储", "redis"},
			ShortPitch:      "核心后端，数据存储与 Web UI",
		},
		"llm-agent": {
			Name: "llm-agent", Tier: TierIntelligence,
			AgentDeps:       []string{"gateway", "blog-agent"},
			FeatureKeywords: []string{"AI", "llm", "智能", "对话", "chat", "模型", "工具调用", "tool", "任务分解"},
			ShortPitch:      "AI 大脑，多模型对话、工具调用、任务分解",
		},
		"execute-code-agent": {
			Name: "execute-code-agent", Tier: TierProductivity,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"代码执行", "execute", "python", "shell", "沙箱", "sandbox", "运行代码"},
			ShortPitch:      "Python/Shell 代码执行沙箱",
		},
		"mcp-agent": {
			Name: "mcp-agent", Tier: TierProductivity,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"MCP", "外部工具", "桥接", "tool", "bridge", "扩展"},
			ShortPitch:      "MCP 外部工具桥接，连接第三方服务",
		},
		"corn-agent": {
			Name: "corn-agent", Tier: TierProductivity,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"定时", "cron", "schedule", "计划任务", "自动", "定时发布", "定时任务"},
			ShortPitch:      "定时任务调度，自动化周期性工作",
		},
		"deploy-agent": {
			Name: "deploy-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway", "blog-agent"},
			FeatureKeywords: []string{"部署", "deploy", "发布", "上线", "SSH", "远程"},
			ShortPitch:      "SSH 自动化部署，一键发布上线",
		},
		"codegen-agent": {
			Name: "codegen-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"代码生成", "codegen", "claude", "编码", "coding", "自动编程"},
			ShortPitch:      "Claude 代码生成，AI 辅助编程",
		},
		"wechat-agent": {
			Name: "wechat-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"微信", "wechat", "企业微信", "消息", "通知", "webhook"},
			ShortPitch:      "企业微信集成，消息收发与通知",
		},
		"acp-agent": {
			Name: "acp-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway", "llm-agent"},
			FeatureKeywords: []string{"ACP", "代码分析", "analysis", "claude", "审查", "review"},
			ShortPitch:      "Claude 代码分析，智能代码审查",
		},
		"log-agent": {
			Name: "log-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway"},
			FeatureKeywords: []string{"日志", "log", "监控", "分析", "聚合"},
			ShortPitch:      "日志聚合分析，集中管理运行日志",
		},
		"env-agent": {
			Name: "env-agent", Tier: TierSpecialized,
			AgentDeps:       []string{"gateway", "blog-agent"},
			FeatureKeywords: []string{"环境", "env", "检测", "远程", "软件安装"},
			ShortPitch:      "远程环境检测与软件安装",
		},
		"deploy-bridge-server": {
			Name: "deploy-bridge-server", Tier: TierSpecialized,
			AgentDeps:       []string{"deploy-agent"},
			FeatureKeywords: []string{"桥接", "bridge", "远程部署", "接收端"},
			ShortPitch:      "远程部署接收端，配合 deploy-agent 使用",
		},
	}
}

// GetAgentMeta returns the meta for a specific agent, or nil if not found.
func GetAgentMeta(name string) *AgentMeta {
	registry := AgentMetaRegistry()
	if m, ok := registry[name]; ok {
		return &m
	}
	return nil
}

// GetAgentsByTier returns agents grouped by their tier.
func GetAgentsByTier() map[AgentTier][]AgentMeta {
	result := make(map[AgentTier][]AgentMeta)
	for _, m := range AgentMetaRegistry() {
		result[m.Tier] = append(result[m.Tier], m)
	}
	return result
}
