package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"agentbase"
	"uap"
)

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg     *Config
	monitor *MonitorEngine
	decider *BroadcastDecider
	caller  *agentbase.RemoteCaller
	catalog *agentbase.ToolCatalog

	activeCount int32
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	tools := buildToolDefs()

	log.Printf("[CortanaAgent] 创建连接 agentID=%s type=cortana_agent tools=%d", agentID, len(tools))
	for _, t := range tools {
		log.Printf("[CortanaAgent]   ├─ tool: %s", t.Name)
	}

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "cortana_agent",
		AgentName:   cfg.AgentName,
		Description: "Cortana 数字管家代理：监控博客、待办、运动数据，智能播报，主动推送语音提醒",
		AuthToken:   cfg.AuthToken,
		Capacity:    5,
		Tools:       tools,
		Meta: map[string]any{
			"version":            "1.0.0",
			"check_interval_sec": cfg.CheckIntervalSec,
			"broadcasts_count":   len(cfg.Broadcasts),
		},
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
	}

	// 创建工具目录和远程调用器（用于 TTS 等跨 agent 调用）
	c.catalog = agentbase.NewToolCatalog(cfg.GatewayHTTPURL)
	if err := c.catalog.Discover(agentID); err != nil {
		log.Printf("[CortanaAgent] ⚠ ToolCatalog 发现失败: %v（TTS 将不可用）", err)
	} else {
		log.Printf("[CortanaAgent] ✓ ToolCatalog 就绪")
	}
	c.catalog.StartRefreshLoop(30*time.Second, agentID)
	log.Printf("[CortanaAgent] ToolCatalog 后台刷新已启动 interval=30s")
	c.caller = agentbase.NewRemoteCaller(c.AgentBase, c.catalog)
	// 注册 tool_result 分发处理器
	c.RegisterHandler(uap.MsgToolResult, func(msg *uap.Message) {
		var result uap.ToolResultPayload
		if err := json.Unmarshal(msg.Payload, &result); err != nil {
			log.Printf("[CortanaAgent] 解析 tool_result 失败: %v", err)
			return
		}
		if !c.caller.DispatchToolResult(&result) {
			log.Printf("[CortanaAgent] 未匹配的 tool_result requestID=%s from=%s", result.RequestID, msg.From)
		}
	})

	// 创建广播决策器和监控引擎
	c.decider = NewBroadcastDecider(cfg)
	c.monitor = NewMonitorEngine(cfg, c.Client, c.decider, NewAccountRegistry())
	c.monitor.OnGenerateTTS = c.generateTTSAudio

	// 注册消息处理器
	c.RegisterToolCallHandler(c.handleToolCall)

	log.Printf("[CortanaAgent] ✓ 连接管理器创建完成")

	return c
}

// Run 启动 agent
func (c *Connection) Run() {
	log.Printf("[CortanaAgent] 启动监控引擎 interval=%ds", c.cfg.CheckIntervalSec)
	c.monitor.Start()
	c.AgentBase.Client.Run()
}

// Stop 停止 agent
func (c *Connection) Stop() {
	log.Printf("[CortanaAgent] 停止监控引擎")
	c.monitor.Stop()
	if c.catalog != nil {
		c.catalog.Stop()
	}
}

// ========================= 工具定义 =========================

func buildToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "cortana.RegisterAccount",
			Description: "注册一个 app 用户账号到 Cortana 播报列表；重复注册会直接成功",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account": map[string]interface{}{"type": "string", "description": "用户账号"},
				},
				"required": []string{"account"},
			}),
		},
		{
			Name:        "cortana.UnregisterAccount",
			Description: "从 Cortana 播报列表移除一个 app 用户账号；账号不存在时直接成功",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account": map[string]interface{}{"type": "string", "description": "用户账号"},
				},
				"required": []string{"account"},
			}),
		},
		{
			Name:        "cortana.ListAccounts",
			Description: "列出当前已注册到 Cortana 的全部 app 用户账号",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
		},
		{
			Name:        "cortana.Status",
			Description: "查询 Cortana 当前状态：是否监控中、上次检查时间、可用播报列表等",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
		},
		{
			Name:        "cortana.ListBroadcasts",
			Description: "列出所有已配置的播报场景及其状态（冷却中/可用/今日已用完）",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}),
		},
		{
			Name:        "cortana.TriggerBroadcast",
			Description: "手动触发一次播报。可指定播报 ID 或直接传入播报文本",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"broadcast_id": map[string]interface{}{"type": "string", "description": "可选：播报场景 ID，不传则需要传 text"},
					"text":         map[string]interface{}{"type": "string", "description": "可选：播报文本（broadcast_id 为空时必填）"},
					"account":      map[string]interface{}{"type": "string", "description": "目标用户账号"},
					"expression":   map[string]interface{}{"type": "string", "description": "可选：Cortana 表情 (happy/sad/surprised)"},
					"motion":       map[string]interface{}{"type": "string", "description": "可选：Cortana 动作 (IdleWave/IdleAlt/Tap/Idle)"},
				},
			}),
		},
		{
			Name:        "cortana.CheckNow",
			Description: "立即执行一次数据检查（博客、待办、运动等），返回检查结果",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account": map[string]interface{}{"type": "string", "description": "用户账号"},
				},
			}),
		},
	}
}

// ========================= 消息处理 =========================

func (c *Connection) handleToolCall(msg *uap.Message) {
	atomic.AddInt32(&c.activeCount, 1)
	defer atomic.AddInt32(&c.activeCount, -1)

	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CortanaAgent] ✗ 解析 tool_call 失败: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid payload"))
		return
	}

	log.Printf("[CortanaAgent] ← tool_call 收到 from=%s msgID=%s tool=%s", msg.From, msg.ID, payload.ToolName)

	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[CortanaAgent] ✗ 解析 arguments 失败: %v", err)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid arguments"))
			return
		}
	} else {
		args = make(map[string]interface{})
	}

	var result string
	var success bool

	switch payload.ToolName {
	case "cortana.RegisterAccount":
		result, success = c.toolRegisterAccount(args)
	case "cortana.UnregisterAccount":
		result, success = c.toolUnregisterAccount(args)
	case "cortana.ListAccounts":
		result, success = c.toolListAccounts()
	case "cortana.Status":
		result, success = c.toolStatus()
	case "cortana.ListBroadcasts":
		result, success = c.toolListBroadcasts()
	case "cortana.TriggerBroadcast":
		result, success = c.toolTriggerBroadcast(args)
	case "cortana.CheckNow":
		result, success = c.toolCheckNow(args)
	default:
		log.Printf("[CortanaAgent] ✗ 未知工具: %s", payload.ToolName)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName)))
		return
	}

	if success {
		log.Printf("[CortanaAgent] → tool_result 成功 to=%s tool=%s", msg.From, payload.ToolName)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
	} else {
		log.Printf("[CortanaAgent] → tool_result 失败 to=%s tool=%s error=%s", msg.From, payload.ToolName, result)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, result))
	}
}

// ========================= 工具实现 =========================

func (c *Connection) toolStatus() (string, bool) {
	state := c.monitor.State()
	state["broadcasts_configured"] = len(c.cfg.Broadcasts)
	state["broadcasts_state"] = c.decider.AllStates()

	resp, _ := json.Marshal(state)
	return string(resp), true
}

func (c *Connection) toolRegisterAccount(args map[string]interface{}) (string, bool) {
	account, _ := args["account"].(string)
	account = strings.TrimSpace(account)
	if account == "" {
		return "account 不能为空", false
	}

	added := c.monitor.RegisterAccount(account)
	log.Printf("[CortanaAgent] 注册账号 account=%s added=%v accounts=%v", account, added, c.monitor.ListAccounts())

	resp, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"account": account,
		"added":   added,
		"total":   len(c.monitor.ListAccounts()),
	})
	return string(resp), true
}

func (c *Connection) toolUnregisterAccount(args map[string]interface{}) (string, bool) {
	account, _ := args["account"].(string)
	account = strings.TrimSpace(account)
	if account == "" {
		return "account 不能为空", false
	}

	removed := c.monitor.UnregisterAccount(account)
	log.Printf("[CortanaAgent] 注销账号 account=%s removed=%v accounts=%v", account, removed, c.monitor.ListAccounts())

	resp, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"account": account,
		"removed": removed,
		"total":   len(c.monitor.ListAccounts()),
	})
	return string(resp), true
}

func (c *Connection) toolListAccounts() (string, bool) {
	accounts := c.monitor.ListAccounts()
	resp, _ := json.Marshal(map[string]interface{}{
		"accounts": accounts,
		"total":    len(accounts),
	})
	return string(resp), true
}

func (c *Connection) toolListBroadcasts() (string, bool) {
	states := c.decider.AllStates()

	type broadcastInfo struct {
		BroadcastConfig
		State string `json:"state"`
	}

	infos := make([]broadcastInfo, 0, len(c.cfg.Broadcasts))
	for _, bc := range c.cfg.Broadcasts {
		state := "available"
		if s, ok := states[bc.ID]; ok {
			state = s
		}
		infos = append(infos, broadcastInfo{
			BroadcastConfig: bc,
			State:           state,
		})
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"broadcasts": infos,
		"total":      len(infos),
	})
	return string(resp), true
}

func (c *Connection) toolTriggerBroadcast(args map[string]interface{}) (string, bool) {
	broadcastID, _ := args["broadcast_id"].(string)
	text, _ := args["text"].(string)
	account, _ := args["account"].(string)
	expression, _ := args["expression"].(string)
	motion, _ := args["motion"].(string)

	if broadcastID == "" && text == "" {
		return "broadcast_id 或 text 必须提供一个", false
	}

	var bc *BroadcastConfig
	if broadcastID != "" {
		for i := range c.cfg.Broadcasts {
			if c.cfg.Broadcasts[i].ID == broadcastID {
				bc = &c.cfg.Broadcasts[i]
				break
			}
		}
		if bc == nil {
			return fmt.Sprintf("播报场景不存在: %s", broadcastID), false
		}
		text = bc.Template
		if expression == "" {
			expression = bc.Expression
		}
		if motion == "" {
			motion = bc.Motion
		}
	}

	if account == "" {
		return "account 不能为空", false
	}

	log.Printf("[CortanaAgent] 手动触发播报 account=%s broadcast_id=%s text=%q",
		account, broadcastID, text)

	if err := c.sendBroadcast(account, text, expression, motion); err != nil {
		return fmt.Sprintf("播报失败: %v", err), false
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"success":      true,
		"broadcast_id": broadcastID,
		"account":      account,
	})
	return string(resp), true
}

func (c *Connection) toolCheckNow(args map[string]interface{}) (string, bool) {
	account, _ := args["account"].(string)

	log.Printf("[CortanaAgent] 手动触发检查 account=%s", account)

	result := c.monitor.CheckNow(account)

	resp, _ := json.Marshal(map[string]interface{}{
		"result":  result,
		"account": account,
	})
	return string(resp), true
}

// sendBroadcast 向 app-agent 推送 Cortana 播报（含 TTS 语音）
func (c *Connection) sendBroadcast(account, text, expression, motion string) error {
	if text == "" {
		return fmt.Errorf("播报文本为空")
	}
	if account == "" {
		return fmt.Errorf("目标用户为空")
	}

	// 生成 TTS 语音
	audioBase64, audioFormat := c.generateTTSAudio(text)

	// 构建 Cortana 播报 payload
	broadcastPayload := map[string]interface{}{
		"kind":         "cortana_broadcast",
		"user_id":      account,
		"text":         text,
		"expression":   expression,
		"motion":       motion,
		"origin":       "cortana-agent",
		"audio_base64": audioBase64,
		"audio_format": audioFormat,
	}

	payloadJSON, err := json.Marshal(broadcastPayload)
	if err != nil {
		return fmt.Errorf("marshal broadcast payload: %w", err)
	}

	notify := uap.NotifyPayload{
		Channel: "app",
		To:      account,
		Content: string(payloadJSON),
	}

	log.Printf("[CortanaAgent] 发送播报到 app-agent user=%s text=%q expression=%s motion=%s audio=%dbytes",
		account, text, expression, motion, len(audioBase64))

	return c.Client.SendTo(c.cfg.AppAgentID, uap.MsgNotify, notify)
}

// generateTTSAudio 调用 audio-agent 生成 TTS 语音
// 返回 base64 编码的音频数据和格式（如 "mp3"）
// 失败时返回空字符串（非致命，播报仍会发送文本）
func (c *Connection) generateTTSAudio(text string) (audioBase64 string, format string) {
	if c.caller == nil || c.catalog == nil {
		return "", ""
	}

	args, _ := json.Marshal(map[string]string{
		"text":   text,
		"format": "mp3",
	})

	result, _, err := c.caller.CallTool("TextToAudio", args, 30*time.Second)
	if err != nil {
		log.Printf("[CortanaAgent] TTS 生成失败: %v", err)
		return "", ""
	}

	var ttsResult struct {
		AudioBase64 string `json:"audio_base64"`
		AudioFormat string `json:"audio_format"`
	}
	if err := json.Unmarshal([]byte(result), &ttsResult); err != nil {
		log.Printf("[CortanaAgent] TTS 结果解析失败: %v", err)
		return "", ""
	}

	if ttsResult.AudioBase64 == "" {
		log.Printf("[CortanaAgent] TTS 返回空音频")
		return "", ""
	}

	log.Printf("[CortanaAgent] ✓ TTS 生成成功 format=%s len=%d", ttsResult.AudioFormat, len(ttsResult.AudioBase64))
	return ttsResult.AudioBase64, ttsResult.AudioFormat
}
