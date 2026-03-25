package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"uap"
)

// ========================= UAP 消息处理 =========================

// handleMessage 处理来自 gateway 的消息
func (b *Bridge) handleMessage(msg *uap.Message) {
	switch msg.Type {
	case uap.MsgNotify:
		var payload uap.NotifyPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid notify payload: %v", err)
			return
		}
		if payload.Channel == "wechat" {
			go b.handleWechatMessage(msg.From, payload.To, payload.Content)
		} else if payload.Channel == "acp_stream" {
			// Claude Mode: acp-agent 发来的流式事件
			b.handleACPStreamEvent(payload)
		} else if payload.Channel == "tool_progress" {
			// deploy-agent 等发送的工具执行进度，payload.To 是工具调用 msgID
			b.toolProgressMu.Lock()
			sink, ok := b.toolProgressSinks[payload.To]
			b.toolProgressMu.Unlock()
			if ok {
				sink.OnEvent("tool_progress", payload.Content)
			} else {
				log.Printf("[Bridge] tool_progress for unknown msgID=%s: %s", payload.To, payload.Content)
			}
		} else {
			log.Printf("[Bridge] unhandled notify channel: %s", payload.Channel)
		}

	case uap.MsgToolResult:
		var payload uap.ToolResultPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_result payload: %v", err)
			return
		}
		b.pendMu.Lock()
		ch, ok := b.pending[payload.RequestID]
		b.pendMu.Unlock()
		if ok {
			ch <- &toolResultWithFrom{ToolResultPayload: payload, FromID: msg.From}
		} else {
			log.Printf("[Bridge] no pending request for %s (from=%s)", payload.RequestID, msg.From)
		}

	case uap.MsgPermissionRequest:
		// Claude Mode: acp-agent 发来的权限请求
		var payload uap.PermissionRequestPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid permission_request payload: %v", err)
			return
		}
		b.handlePermissionRequest(msg.From, payload)

	case uap.MsgError:
		var payload uap.ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			log.Printf("[Bridge] error: %s - %s (msg_id=%s)", payload.Code, payload.Message, msg.ID)
			// 如果是 agent_offline 错误，也需要释放 pending
			b.pendMu.Lock()
			ch, ok := b.pending[msg.ID]
			b.pendMu.Unlock()
			if ok {
				ch <- &toolResultWithFrom{
					ToolResultPayload: uap.ToolResultPayload{
						RequestID: msg.ID,
						Success:   false,
						Error:     payload.Message,
					},
					FromID: msg.From,
				}
			}
		}

	case uap.MsgTaskAssign:
		var taskPayload uap.TaskAssignPayload
		if err := json.Unmarshal(msg.Payload, &taskPayload); err != nil {
			log.Printf("[Bridge] invalid task_assign payload: %v", err)
			return
		}
		// 先探测 task_type 字段
		var taskType struct {
			TaskType string `json:"task_type"`
		}
		json.Unmarshal(taskPayload.Payload, &taskType)

		log.Printf("[Bridge] task_assign received: taskID=%s task_type=%s from=%s", taskPayload.TaskID, taskType.TaskType, msg.From)

		// 构建 handler（根据 task_type 解析 payload）
		var handler func()
		switch taskType.TaskType {
		case "assistant_chat":
			var assistantPayload AssistantTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &assistantPayload); err != nil {
				log.Printf("[Bridge] invalid assistant task payload: %v", err)
				return
			}
			handler = func() { b.handleAssistantTask(taskPayload.TaskID, &assistantPayload) }
		case "llm_request":
			var llmPayload LLMRequestPayload
			if err := json.Unmarshal(taskPayload.Payload, &llmPayload); err != nil {
				log.Printf("[Bridge] invalid llm_request payload: %v", err)
				return
			}
			handler = func() { b.handleLLMRequestTask(taskPayload.TaskID, &llmPayload) }
		case "resume_task":
			var resumePayload ResumeTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &resumePayload); err != nil {
				log.Printf("[Bridge] invalid resume_task payload: %v", err)
				return
			}
			handler = func() { b.handleResumeTask(taskPayload.TaskID, &resumePayload) }
		case "cron_reminder":
			var wrapper struct {
				Payload  json.RawMessage `json:"payload"`
				Provider string          `json:"provider,omitempty"`
				Model    string          `json:"model,omitempty"`
			}
			if err := json.Unmarshal(taskPayload.Payload, &wrapper); err != nil {
				log.Printf("[Bridge] invalid cron_reminder payload: %v", err)
				return
			}
			var reminderPayload CronReminderPayload
			if err := json.Unmarshal(wrapper.Payload, &reminderPayload); err != nil {
				log.Printf("[Bridge] invalid cron_reminder inner payload: %v", err)
				return
			}
			sourceAgent := msg.From
			handler = func() { b.handleCronReminder(taskPayload.TaskID, sourceAgent, &reminderPayload) }
		case "cron_query":
			var wrapper struct {
				Payload  json.RawMessage `json:"payload"`
				Provider string          `json:"provider,omitempty"`
				Model    string          `json:"model,omitempty"`
			}
			if err := json.Unmarshal(taskPayload.Payload, &wrapper); err != nil {
				log.Printf("[Bridge] invalid cron_query payload: %v", err)
				return
			}
			var queryPayload CronQueryPayload
			if err := json.Unmarshal(wrapper.Payload, &queryPayload); err != nil {
				log.Printf("[Bridge] invalid cron_query inner payload: %v", err)
				return
			}
			queryPayload.Provider = wrapper.Provider
			queryPayload.Model = wrapper.Model
			sourceAgent := msg.From
			handler = func() { b.handleCronQuery(taskPayload.TaskID, sourceAgent, &queryPayload) }
		default:
			log.Printf("[Bridge] unknown task_type: %s, sending task_complete failure to %s", taskType.TaskType, msg.From)
			b.client.Send(&uap.Message{
				Type:    uap.MsgTaskComplete,
				ID:      uap.NewMsgID(),
				From:    b.cfg.AgentID,
				To:      msg.From,
				Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskPayload.TaskID, Status: "failed", Error: fmt.Sprintf("unknown task_type: %s", taskType.TaskType)}),
				Ts:      time.Now().UnixMilli(),
			})
			return
		}

		// 统一发送 task_accepted（无论直接执行还是入队，都告知 gateway 已收到）
		b.client.Send(&uap.Message{
			Type:    uap.MsgTaskAccepted,
			ID:      uap.NewMsgID(),
			From:    b.cfg.AgentID,
			To:      "blog-agent",
			Payload: mustMarshal(uap.TaskAcceptedPayload{TaskID: taskPayload.TaskID}),
			Ts:      time.Now().UnixMilli(),
		})

		// 准入控制：直接执行 / 入队 / 拒绝
		if b.canAccept() {
			b.registerTask(taskPayload.TaskID, taskType.TaskType)
			go func() {
				defer b.deregisterTask(taskPayload.TaskID)
				handler()
			}()
		} else if b.enqueueOrReject(&queuedTask{
			taskID:    taskPayload.TaskID,
			taskType:  taskType.TaskType,
			handler:   handler,
			createdAt: time.Now(),
		}) {
			// 入队成功，等待 drainQueue 触发执行
		} else {
			// 队列也满了，发送 task_rejected
			b.client.Send(&uap.Message{
				Type: uap.MsgTaskRejected,
				ID:   uap.NewMsgID(),
				From: b.cfg.AgentID,
				To:   "blog-agent",
				Payload: mustMarshal(uap.TaskRejectedPayload{
					TaskID: taskPayload.TaskID,
					Reason: fmt.Sprintf("agent at max capacity (active=%d/%d, queue=%d/%d)",
						b.activeCount(), b.cfg.MaxConcurrent, len(b.taskQueue), b.cfg.TaskQueueSize),
				}),
				Ts: time.Now().UnixMilli(),
			})
		}

	default:
		log.Printf("[Bridge] unhandled message type: %s from %s", msg.Type, msg.From)
	}
}

// ========================= Claude Mode 事件处理 =========================

// handleACPStreamEvent 处理 acp-agent 发来的流式事件（Claude Mode）
func (b *Bridge) handleACPStreamEvent(payload uap.NotifyPayload) {
	// payload.Content 是 JSON 序列化的 StreamEventPayload
	var evt StreamEventPayload
	if err := json.Unmarshal([]byte(payload.Content), &evt); err != nil {
		log.Printf("[Bridge] invalid acp_stream payload: %v", err)
		return
	}

	// 通过 ClaudeSessionID 反查对应的 wechat user session key
	var sinkKey string
	b.sessionMgr.mu.RLock()
	for key, session := range b.sessionMgr.sessions {
		if session.ClaudeMode && session.ClaudeSessionID == evt.SessionID {
			sinkKey = key
			break
		}
	}
	b.sessionMgr.mu.RUnlock()

	// 查找对应的 claude stream sink
	b.claudeSinksMu.Lock()
	sink, ok := b.claudeSinks[sinkKey]
	if !ok {
		// fallback: 尝试任意一个 sink
		for _, s := range b.claudeSinks {
			sink = s
			break
		}
	}
	b.claudeSinksMu.Unlock()

	if sink == nil {
		log.Printf("[Bridge] no claude sink for acp_stream event session=%s", evt.SessionID)
		return
	}

	sink.onStreamEvent(evt)
}

// handlePermissionRequest 处理 acp-agent 发来的权限请求（Claude Mode 交互模式）
func (b *Bridge) handlePermissionRequest(acpAgentID string, payload uap.PermissionRequestPayload) {
	log.Printf("[Bridge] permission_request: session=%s title=%s options=%d", payload.SessionID, payload.Title, len(payload.Options))

	// 通过 sessionID 反查 wechat user
	var targetSession *ChatSession
	var fromAgent, wechatUser string

	b.sessionMgr.mu.RLock()
	for _, session := range b.sessionMgr.sessions {
		if session.ClaudeMode && session.ClaudeSessionID == payload.SessionID {
			targetSession = session
			fromAgent = session.ClaudeFromAgent
			wechatUser = session.UserID
			break
		}
	}
	b.sessionMgr.mu.RUnlock()

	if targetSession == nil {
		log.Printf("[Bridge] no session found for permission request session=%s", payload.SessionID)
		return
	}

	// 构建权限选项信息
	options := make([]PermOptionInfo, len(payload.Options))
	for i, opt := range payload.Options {
		options[i] = PermOptionInfo{
			Index:    opt.Index,
			OptionID: opt.OptionID,
			Name:     opt.Name,
			Kind:     opt.Kind,
		}
	}

	// 设置 pending permission
	targetSession.SetPendingPermission(&PendingPermission{
		RequestID:  payload.RequestID,
		SessionID:  payload.SessionID,
		ACPAgentID: acpAgentID,
		Options:    options,
	})

	// 构建可读消息发给微信用户
	var sb strings.Builder
	sb.WriteString("🔒 请求授权\n")
	sb.WriteString(fmt.Sprintf("操作: %s\n", payload.Title))
	if payload.Content != "" {
		content := payload.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(content + "\n")
	}
	sb.WriteString("\n")
	for _, opt := range payload.Options {
		sb.WriteString(fmt.Sprintf("%d. %s\n", opt.Index, opt.Name))
	}
	sb.WriteString("\n回复数字或 y/n")

	b.sendWechat(fromAgent, wechatUser, sb.String())
}

// ========================= 工具评估 =========================

// EvaluateTools 执行工具评估并输出报告
func (b *Bridge) EvaluateTools() {
	evaluator := NewToolEvaluator(b)
	report := evaluator.Evaluate()

	// 日志输出每条 issue
	for _, issue := range report.Issues {
		var prefix string
		switch issue.Severity {
		case SeverityCritical:
			prefix = "✗"
		case SeverityWarning:
			prefix = "⚠"
		case SeverityInfo:
			prefix = "ℹ"
		}
		if issue.RelatedTo != "" {
			log.Printf("[ToolEvaluator] %s [%s] %s: %s → %s",
				prefix, issue.Category, issue.Subject, issue.Description, issue.Suggestion)
		} else {
			log.Printf("[ToolEvaluator] %s [%s] %s: %s → %s",
				prefix, issue.Category, issue.Subject, issue.Description, issue.Suggestion)
		}
	}

	// 写入 JSON 报告文件
	if b.cfg.ToolEvalReportPath != "" {
		reportDir := filepath.Dir(b.cfg.ToolEvalReportPath)
		if err := os.MkdirAll(reportDir, 0755); err != nil {
			log.Printf("[ToolEvaluator] 创建报告目录失败: %v", err)
			return
		}

		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Printf("[ToolEvaluator] 序列化报告失败: %v", err)
			return
		}
		if err := os.WriteFile(b.cfg.ToolEvalReportPath, data, 0644); err != nil {
			log.Printf("[ToolEvaluator] 写入报告失败: %v", err)
			return
		}
		log.Printf("[ToolEvaluator] 报告已写入: %s", b.cfg.ToolEvalReportPath)
	}
}

// ========================= 后台刷新 =========================

// StartRefreshLoop 后台定时刷新工具目录和 agent 信息
func (b *Bridge) StartRefreshLoop() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := b.DiscoverTools(); err != nil {
				log.Printf("[Bridge] refresh tools failed: %v", err)
			}
			if err := b.DiscoverAgents(); err != nil {
				log.Printf("[Bridge] refresh agents failed: %v", err)
			}
		}
	}()
}

// StartSessionCleanupLoop 后台定时清理过期会话（替代 StartWechatCleanupLoop）
func (b *Bridge) StartSessionCleanupLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			b.sessionMgr.CleanupExpired()
		}
	}()
}

// RecoverInProgressTasks 启动时扫描并恢复中断的任务
func (b *Bridge) RecoverInProgressTasks() {
	store := NewSessionStore(b.cfg.SessionDir)
	runningIDs, err := store.ListRunningSessions()
	if err != nil {
		log.Printf("[Bridge] recover: scan failed: %v", err)
		return
	}
	if len(runningIDs) == 0 {
		log.Printf("[Bridge] recover: no interrupted tasks found")
		return
	}

	log.Printf("[Bridge] recover: found %d interrupted tasks: %v", len(runningIDs), runningIDs)
	for _, rootID := range runningIDs {
		rid := rootID
		if b.canAccept() {
			b.registerTask(rid, "resume_task")
			go func() {
				defer b.deregisterTask(rid)
				b.handleResumeTask(rid, &ResumeTaskPayload{RootSessionID: rid})
			}()
		} else if b.enqueueOrReject(&queuedTask{
			taskID:    rid,
			taskType:  "resume_task",
			handler:   func() { b.handleResumeTask(rid, &ResumeTaskPayload{RootSessionID: rid}) },
			createdAt: time.Now(),
		}) {
			log.Printf("[Bridge] recover: enqueued %s", rid)
		} else {
			log.Printf("[Bridge] recover: skipped %s (queue full)", rid)
		}
	}
}
