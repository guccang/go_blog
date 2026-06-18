package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type PromptRuntimeContext struct {
	Title  string
	Fields map[string]string
	Blocks []string
}

func buildPromptRuntimeContext(ctx PromptRuntimeContext) string {
	fields := make(map[string]string)
	for k, v := range ctx.Fields {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			fields[k] = strings.TrimSpace(v)
		}
	}
	var blocks []string
	for _, block := range ctx.Blocks {
		block = strings.TrimSpace(block)
		if block != "" {
			blocks = append(blocks, block)
		}
	}
	if len(fields) == 0 && len(blocks) == 0 {
		return ""
	}

	title := strings.TrimSpace(ctx.Title)
	if title == "" {
		title = "运行时上下文"
	}

	var sb strings.Builder
	sb.WriteString("## ")
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString("以下信息是本轮动态上下文，不属于稳定系统提示词；只在本轮决策中使用。\n")

	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(fields[k])
			sb.WriteString("\n")
		}
	}

	for _, block := range blocks {
		sb.WriteString("\n")
		sb.WriteString(block)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func buildAccountRuntimeContext(account, source string, extra map[string]string) string {
	fields := make(map[string]string)
	if account = strings.TrimSpace(account); account != "" {
		fields["account"] = account
	}
	if source = strings.TrimSpace(source); source != "" {
		fields["source"] = source
	}
	for k, v := range extra {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			fields[k] = v
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return buildPromptRuntimeContext(PromptRuntimeContext{
		Title:  "请求运行时上下文",
		Fields: fields,
	})
}

func buildTurnRuntimeContext(account, source string, extra map[string]string, now time.Time) string {
	return combineRuntimeBlocks(
		buildAccountRuntimeContext(account, source, extra),
		buildLocalTimeRuntimeContext(now),
	)
}

func buildLocalTimeRuntimeContext(now time.Time) string {
	if now.IsZero() {
		return ""
	}
	_, offset := now.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	fields := map[string]string{
		"local_datetime":  now.Format("2006-01-02 15:04"),
		"time_precision":  "minute",
		"timezone":        now.Location().String(),
		"timezone_offset": fmt.Sprintf("%s%02d:%02d", sign, offset/3600, (offset%3600)/60),
		"weekday":         chineseWeekday(now.Weekday()),
	}
	return buildPromptRuntimeContext(PromptRuntimeContext{
		Title:  "当前本地时间上下文",
		Fields: fields,
	})
}

func combineRuntimeBlocks(blocks ...string) string {
	var parts []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block != "" {
			parts = append(parts, block)
		}
	}
	return strings.Join(parts, "\n\n")
}

func messagesWithRuntimeContext(systemPrompt, runtimeContext, userContent string) []Message {
	messages := []Message{{Role: "system", Content: systemPrompt}}
	if runtimeContext = strings.TrimSpace(runtimeContext); runtimeContext != "" {
		messages = append(messages, Message{Role: "user", Content: runtimeContext})
	}
	messages = append(messages, Message{Role: "user", Content: userContent})
	return messages
}
