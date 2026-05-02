package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"uap"
)

type CortanaProactiveDecision struct {
	ShouldInteract    bool             `json:"should_interact"`
	Reason            string           `json:"reason"`
	Priority          string           `json:"priority"`
	SpeechText        string           `json:"speech_text"`
	SummaryText       string           `json:"summary_text"`
	Expression        string           `json:"expression"`
	Actions           []map[string]any `json:"actions"`
	FollowUpSuggested bool             `json:"follow_up_suggested"`
	NextCooldownSec   int              `json:"next_cooldown_sec"`
}

func (b *Bridge) handleCortanaProactiveTask(taskID, sourceAgent string, payload *CortanaProactivePayload) {
	log.Printf("[CortanaProactive] task=%s account=%s trigger=%s", taskID, payload.Account, payload.TriggerReason)

	decision, err := b.generateCortanaProactiveDecision(payload)
	status := "success"
	errMsg := ""
	result := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	} else {
		body, _ := json.Marshal(decision)
		result = string(body)
	}

	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      sourceAgent,
		Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskID, Status: status, Error: errMsg, Result: result}),
		Ts:      time.Now().UnixMilli(),
	})
}

func (b *Bridge) generateCortanaProactiveDecision(payload *CortanaProactivePayload) (*CortanaProactiveDecision, error) {
	if payload == nil {
		return nil, fmt.Errorf("empty payload")
	}

	systemPrompt := strings.TrimSpace(`
你是 Cortana 数字助手的主动陪伴决策器。
职责：聊天助手、工作秘书、生活助手。目标是帮助用户完成目标，让用户感觉被照顾，但不能无意义打扰。
你要基于当前在线状态、授权状态、主动模式、生活快照、事件上下文和最近互动历史，判断现在是否值得打扰用户。
只输出一个 JSON 对象，不要输出 markdown，不要解释。

输出字段固定为：
{
  "should_interact": true,
  "reason": "为什么要/不要主动互动",
  "priority": "low|medium|high|urgent",
  "speech_text": "最终播报文本",
  "summary_text": "供客户端展示的摘要",
  "expression": "happy|sad|surprised",
  "actions": [{"motion":"IdleWave","delay":0}],
  "follow_up_suggested": false,
  "next_cooldown_sec": 1800
}

规则：
1. 如果不该打扰，should_interact=false，speech_text 和 summary_text 留空数组 actions 可为空。
2. 优先只推送“此刻对用户最有价值的一件事”。
3. 如果用户未授权全量访问或不在线，通常应该返回 should_interact=false。
4. 表情只能是 happy/sad/surprised；动作优先 Idle/IdleAlt/IdleWave/Tap。
5. 口播简短、自然、中文。
6. 当 proactive_mode=high 且用户在线、已授权全量访问时，不要把“没有待办/没有新内容”直接等同于“不该互动”。
7. 在 high 模式下，如果没有明确任务提醒，也可以主动返回轻量陪伴类消息，例如问候、陪伴式闲聊、温和鼓励、状态确认、轻提醒、生活化寒暄，但仍要自然、克制、像真人助理。
8. 当前阶段不要因为“刚刚互动过”就抑制 Cortana；除非内容明显重复、明显不合时宜、或者会造成打扰，否则优先允许继续互动。
9. 如果 snapshot 大部分为空，也不要机械地因为“无数据”就拒绝互动；可以基于时间段、在线状态和 high 模式给出一句轻陪伴消息。
10. 如果 event_context 提供了 trigger_source、trigger_reason、content 或 summary，要优先理解“为什么此刻触发”，例如用户刚打开 Cortana 页签、刚点了 Cortana 浮窗、刚发了一句聊天、刚收到 cron 提醒。
11. 如果 trigger_source 与聊天相关，可以结合 content 判断用户是在求助、表达情绪、还是只是路过，然后决定是安抚、追问、提醒还是轻陪伴。
12. 即使 recent_interactions 里存在最近消息，也不要把它当作硬性冷却规则；当前目标是先充分观察无冷却下的互动表现。
13. 陪伴类消息不要空泛鸡汤，尽量具体、像对当前在线的本人说话。`)

	body, _ := json.Marshal(payload)
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(body)},
	}

	cfg := b.activeLLM.Get()
	text, _, err := SendLLMRequestWithFallback(&cfg, b.cfg.Fallbacks, b.fallbackCooldown(), messages, nil, b.cfg.Providers)
	if err != nil {
		return nil, err
	}

	raw := extractJSONObject(text)
	if raw == "" {
		raw = strings.TrimSpace(text)
	}
	var decision CortanaProactiveDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return nil, fmt.Errorf("parse proactive decision: %w", err)
	}
	if decision.Expression == "" {
		decision.Expression = "happy"
	}
	if len(decision.Actions) == 0 && decision.ShouldInteract {
		decision.Actions = []map[string]any{{"motion": "IdleWave", "delay": 0}}
	}
	return &decision, nil
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}
