package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"uap"
)

// ========================= 任务生命周期 Hook =========================

// TaskHook 任务生命周期钩子
type TaskHook interface {
	OnTaskStart(ctx *TaskContext)
	OnToolCall(ctx *TaskContext, record ToolCallRecord)
	OnSubTaskDone(ctx *TaskContext, result SubTaskResult)
	OnTaskEnd(ctx *TaskContext, result string, toolCalls []ToolCallRecord, err error)
}

// BaseTaskHook 空实现基类，具体 hook 嵌入后只需覆盖关心的方法
type BaseTaskHook struct{}

func (BaseTaskHook) OnTaskStart(_ *TaskContext)                                      {}
func (BaseTaskHook) OnToolCall(_ *TaskContext, _ ToolCallRecord)                     {}
func (BaseTaskHook) OnSubTaskDone(_ *TaskContext, _ SubTaskResult)                   {}
func (BaseTaskHook) OnTaskEnd(_ *TaskContext, _ string, _ []ToolCallRecord, _ error) {}

// ========================= HookManager =========================

// HookManager 管理所有注册的 hook
type HookManager struct {
	mu    sync.RWMutex
	hooks []TaskHook
}

// NewHookManager 创建 HookManager
func NewHookManager() *HookManager {
	return &HookManager{}
}

// Register 注册 hook
func (m *HookManager) Register(h TaskHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, h)
}

// FireTaskStart 触发任务开始事件
func (m *HookManager) FireTaskStart(ctx *TaskContext) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, h := range m.hooks {
		func() {
			defer recoverHook("OnTaskStart")
			h.OnTaskStart(ctx)
		}()
	}
}

// FireToolCall 触发工具调用事件
func (m *HookManager) FireToolCall(ctx *TaskContext, record ToolCallRecord) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, h := range m.hooks {
		func() {
			defer recoverHook("OnToolCall")
			h.OnToolCall(ctx, record)
		}()
	}
}

// FireSubTaskDone 触发子任务完成事件
func (m *HookManager) FireSubTaskDone(ctx *TaskContext, result SubTaskResult) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, h := range m.hooks {
		func() {
			defer recoverHook("OnSubTaskDone")
			h.OnSubTaskDone(ctx, result)
		}()
	}
}

// FireTaskEnd 触发任务结束事件
func (m *HookManager) FireTaskEnd(ctx *TaskContext, result string, toolCalls []ToolCallRecord, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, h := range m.hooks {
		func() {
			defer recoverHook("OnTaskEnd")
			h.OnTaskEnd(ctx, result, toolCalls, err)
		}()
	}
}

// recoverHook 防止单个 hook panic 影响其他 hook
func recoverHook(hookName string) {
	if r := recover(); r != nil {
		log.Printf("[HookManager] %s panic: %v", hookName, r)
	}
}

// ========================= WechatUsageSummaryHook =========================

// WechatUsageSummaryHook 微信 Agent/Tool 使用摘要推送
type WechatUsageSummaryHook struct {
	BaseTaskHook
	bridge *Bridge
}

// OnTaskEnd 任务完成时推送工具使用摘要到微信
func (h *WechatUsageSummaryHook) OnTaskEnd(ctx *TaskContext, result string, toolCalls []ToolCallRecord, err error) {
	// 仅微信来源且有工具调用时推送
	if ctx.Source != "wechat" || len(toolCalls) == 0 {
		return
	}

	// 从 Sink 获取微信推送信息
	wechatSink, ok := ctx.Sink.(*WechatSink)
	if !ok {
		return
	}

	summary := h.buildSummary(toolCalls)
	if summary == "" {
		return
	}

	if err := h.bridge.client.SendTo(wechatSink.fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatSink.wechatUser,
		Content: summary,
	}); err != nil {
		log.Printf("[WechatUsageSummaryHook] send failed: %v", err)
	}
}

// agentCallGroup 按 agent 分组的工具调用详情
type agentCallGroup struct {
	AgentID   string
	AgentName string
	AgentDesc string
	Calls     []ToolCallRecord // 保持调用顺序
}

// buildSummary 构建 Agent/Tool 使用详细摘要
func (h *WechatUsageSummaryHook) buildSummary(toolCalls []ToolCallRecord) string {
	// 快照 catalog 和 agentInfo
	h.bridge.catalogMu.RLock()
	catalogCopy := make(map[string]string, len(h.bridge.toolCatalog))
	for k, v := range h.bridge.toolCatalog {
		catalogCopy[k] = v
	}
	agentInfoCopy := make(map[string]AgentInfo, len(h.bridge.agentInfo))
	for k, v := range h.bridge.agentInfo {
		agentInfoCopy[k] = v
	}
	h.bridge.catalogMu.RUnlock()

	// 按 agent 分组，保持出现顺序
	agentMap := make(map[string]*agentCallGroup)
	var agentOrder []string
	var totalDurationMs int64
	successCount, failCount := 0, 0

	for _, tc := range toolCalls {
		totalDurationMs += tc.DurationMs
		if tc.Success {
			successCount++
		} else {
			failCount++
		}

		agentID := catalogCopy[tc.ToolName]
		if agentID == "" {
			agentID = "unknown"
		}

		g, exists := agentMap[agentID]
		if !exists {
			info := agentInfoCopy[agentID]
			g = &agentCallGroup{
				AgentID:   agentID,
				AgentName: info.Name,
				AgentDesc: info.Description,
			}
			agentMap[agentID] = g
			agentOrder = append(agentOrder, agentID)
		}
		g.Calls = append(g.Calls, tc)
	}

	// 格式化
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 任务摘要: %d 个 Agent / %d 次工具调用 (✅%d ❌%d)\n",
		len(agentMap), len(toolCalls), successCount, failCount))

	for _, agentID := range agentOrder {
		g := agentMap[agentID]

		// agent 标题
		if g.AgentName != "" && g.AgentDesc != "" {
			sb.WriteString(fmt.Sprintf("\n🤖 %s (%s)\n", g.AgentName, g.AgentDesc))
		} else if g.AgentName != "" {
			sb.WriteString(fmt.Sprintf("\n🤖 %s\n", g.AgentName))
		} else {
			sb.WriteString(fmt.Sprintf("\n🤖 %s\n", agentID))
		}

		// 每次调用的详情
		for i, tc := range g.Calls {
			status := "✅"
			if !tc.Success {
				status = "❌"
			}
			dur := fmtDuration(time.Duration(tc.DurationMs) * time.Millisecond)

			sb.WriteString(fmt.Sprintf("  %d. %s %s %s\n", i+1, tc.ToolName, status, dur))

			// 参数摘要
			args := summarizeArgs(tc.Arguments)
			if args != "" {
				sb.WriteString(fmt.Sprintf("     参数: %s\n", args))
			}

			// 结果摘要
			if !tc.Success {
				sb.WriteString(fmt.Sprintf("     错误: %s\n", truncateRunes(tc.Result, 120)))
			} else if tc.Result != "" {
				sb.WriteString(fmt.Sprintf("     结果: %s\n", truncateRunes(tc.Result, 120)))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\n⏱ 总工具耗时: %s", fmtDuration(time.Duration(totalDurationMs)*time.Millisecond)))

	// 附加 token 用量统计
	if globalTokenStats != nil {
		if tokenSummary := globalTokenStats.Summary(); tokenSummary != "" {
			sb.WriteString("\n")
			sb.WriteString(tokenSummary)
		}
	}

	return sb.String()
}

// summarizeArgs 从 JSON 参数中提取关键字段的简短摘要
func summarizeArgs(argsJSON string) string {
	if argsJSON == "" || argsJSON == "{}" {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return truncateRunes(argsJSON, 80)
	}

	// 提取关键字段（按优先级）
	var parts []string
	keyFields := []string{"title", "name", "date", "query", "keyword", "description", "code", "content", "path", "url"}
	for _, key := range keyFields {
		val, ok := args[key]
		if !ok {
			continue
		}
		s := truncateRunes(fmt.Sprintf("%v", val), 60)
		parts = append(parts, fmt.Sprintf("%s=%s", key, s))
		if len(parts) >= 3 {
			break
		}
	}

	// 如果没命中关键字段，取前 2 个任意字段
	if len(parts) == 0 {
		for k, v := range args {
			s := truncateRunes(fmt.Sprintf("%v", v), 60)
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
			if len(parts) >= 2 {
				break
			}
		}
	}

	return strings.Join(parts, ", ")
}

// truncateRunes UTF-8 安全截断
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
