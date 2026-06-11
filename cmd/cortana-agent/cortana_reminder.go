package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"uap"
)

// handleInboundReminder 处理来自 hermes cron 的提醒投递（plan §6：cron 到点 → cortana 表达层）。
// 经 cortana 的打扰控制（注册/启用/在线 + 冷却 + 记忆查重）后，由主动决策链路决定语气与表情再播报；
// 若 cortana 未托管该账号，回退为直推 app-agent，避免提醒丢失。
func (c *Connection) handleInboundReminder(msg *uap.Message) {
	var notify uap.NotifyPayload
	if err := json.Unmarshal(msg.Payload, &notify); err != nil {
		log.Printf("[CortanaAgent] 解析入站 notify 失败: %v", err)
		return
	}
	if strings.ToLower(strings.TrimSpace(notify.Channel)) != "app" {
		return
	}
	account := strings.TrimSpace(notify.To)
	content := strings.TrimSpace(notify.Content)
	if account == "" || content == "" {
		return
	}
	reason := defaultString(metaString(notify.Meta, "trigger_reason"), "cron_reminder")

	session := c.monitor.GetSession(account)
	if session == nil || !session.Registered || !session.Enabled || !session.Online {
		log.Printf("[CortanaAgent] cron 提醒回退直推 account=%s reason=%s（cortana 未托管/离线）", account, reason)
		c.forwardReminderToApp(notify)
		return
	}

	event := CortanaTriggerEventPayload{
		Account:       account,
		TriggerSource: "cron",
		TriggerReason: reason,
		Content:       content,
		Summary:       shortInteractionText(metaString(notify.Meta, "summary"), content),
		Metadata:      notify.Meta,
		Timestamp:     time.Now().UnixMilli(),
	}
	log.Printf("[CortanaAgent] cron 提醒进入打扰控制链路 account=%s reason=%s len=%d", account, reason, len(content))
	if _, _, err := c.handleProactiveEvent(event); err != nil {
		log.Printf("[CortanaAgent] cron 提醒主动链路失败 account=%s err=%v，回退直推", account, err)
		c.forwardReminderToApp(notify)
	}
}

// forwardReminderToApp 把提醒原样转发给 app-agent 推送（打扰控制不可用时的兜底）。
func (c *Connection) forwardReminderToApp(notify uap.NotifyPayload) {
	notify.Channel = "app"
	if err := c.Client.SendTo(c.cfg.AppAgentID, uap.MsgNotify, notify); err != nil {
		log.Printf("[CortanaAgent] 回退转发提醒失败: %v", err)
	}
}

func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key]; ok {
		return strings.TrimSpace(toStringValue(v))
	}
	return ""
}

func toStringValue(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return ""
	}
}
