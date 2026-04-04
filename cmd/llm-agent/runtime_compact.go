package main

import "time"

type RuntimeCompactConfig struct {
	MaxMessages      int
	MaxChars         int
	TriggerMessages  int
	TriggerChars     int
	ToolResultBudget int
	RecentToolKeep   int
}

type RuntimeCompactMetadata struct {
	Reason            string    `json:"reason"`
	BeforeMessages    int       `json:"before_messages"`
	AfterMessages     int       `json:"after_messages"`
	BeforeChars       int       `json:"before_chars"`
	AfterChars        int       `json:"after_chars"`
	ToolResultsTrimed int       `json:"tool_results_trimed"`
	DroppedMessages   int       `json:"dropped_messages"`
	At                time.Time `json:"at"`
}

func defaultQueryCompactConfig() RuntimeCompactConfig {
	return RuntimeCompactConfig{
		MaxMessages:      processMaxMessages,
		MaxChars:         processMaxTotalChars,
		TriggerMessages:  20,
		TriggerChars:     processMaxTotalChars * 80 / 100,
		ToolResultBudget: 24000,
		RecentToolKeep:   2,
	}
}

func defaultSubTaskCompactConfig() RuntimeCompactConfig {
	return RuntimeCompactConfig{
		MaxMessages:      18,
		MaxChars:         120000,
		TriggerMessages:  15,
		TriggerChars:     120000,
		ToolResultBudget: 18000,
		RecentToolKeep:   2,
	}
}

func shouldCompactMessages(messages []Message, cfg RuntimeCompactConfig) bool {
	if len(messages) > cfg.TriggerMessages {
		return true
	}
	return estimateChars(messages) > cfg.TriggerChars
}

func compactRuntimeMessages(messages []Message, cfg RuntimeCompactConfig, iteration int, reason string) ([]Message, *RuntimeCompactMetadata) {
	if len(messages) == 0 {
		return messages, nil
	}

	beforeMessages := len(messages)
	beforeChars := estimateChars(messages)
	trimmedMessages, trimmedToolCount := applyToolResultBudget(messages, cfg.ToolResultBudget, cfg.RecentToolKeep, iteration)

	compacted := trimmedMessages
	if len(compacted) > cfg.MaxMessages || estimateChars(compacted) > cfg.MaxChars {
		compacted = sanitizeMessagesWithBudget(compacted, cfg.MaxMessages, cfg.MaxChars)
	}

	afterMessages := len(compacted)
	afterChars := estimateChars(compacted)
	if beforeMessages == afterMessages && beforeChars == afterChars && trimmedToolCount == 0 {
		return messages, nil
	}

	return compacted, &RuntimeCompactMetadata{
		Reason:            reason,
		BeforeMessages:    beforeMessages,
		AfterMessages:     afterMessages,
		BeforeChars:       beforeChars,
		AfterChars:        afterChars,
		ToolResultsTrimed: trimmedToolCount,
		DroppedMessages:   beforeMessages - afterMessages,
		At:                time.Now(),
	}
}

func applyToolResultBudget(messages []Message, budget int, recentKeep int, iteration int) ([]Message, int) {
	if budget <= 0 || len(messages) == 0 {
		return messages, 0
	}

	totalToolChars := 0
	toolIndexes := make([]int, 0)
	for idx, msg := range messages {
		if msg.Role != "tool" {
			continue
		}
		totalToolChars += len(msg.Content)
		toolIndexes = append(toolIndexes, idx)
	}
	if totalToolChars <= budget || len(toolIndexes) == 0 {
		return messages, 0
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	trimmed := 0
	used := 0
	recentSeen := 0
	for i := len(toolIndexes) - 1; i >= 0; i-- {
		idx := toolIndexes[i]
		msg := result[idx]
		contentLen := len(msg.Content)
		limit := 800
		if recentSeen < recentKeep {
			limit = 1600
			recentSeen++
		}
		if used+contentLen <= budget {
			used += contentLen
			continue
		}

		allowed := limit
		if remain := budget - used; remain > 0 && remain < allowed {
			allowed = remain
		}
		if allowed < 200 {
			allowed = 200
		}
		shortened := truncateToolResultWithLimit(msg.Content, allowed, iteration)
		if shortened != msg.Content {
			result[idx].Content = shortened
			trimmed++
		}
		used += len(result[idx].Content)
	}
	return result, trimmed
}

func truncateToolResultWithLimit(result string, maxLen int, iteration int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(result)
	if len(runes) <= maxLen {
		return result
	}
	if maxLen < 32 {
		return string(runes[:maxLen])
	}
	suffix := "\n...[上下文压缩]"
	contentMax := maxLen - len([]rune(suffix))
	if contentMax < 16 {
		contentMax = maxLen
		suffix = ""
	}
	return string(runes[:contentMax]) + suffix
}

func sanitizeMessagesWithBudget(messages []Message, maxMessages, maxChars int) []Message {
	if len(messages) <= 2 {
		return messages
	}

	systemMsg := messages[0]
	rest := messages[1:]
	systemChars := len(systemMsg.Content)
	charBudget := maxChars - systemChars
	msgBudget := maxMessages - 1

	var kept []Message
	totalChars := 0
	for i := len(rest) - 1; i >= 0; i-- {
		msg := rest[i]
		msgChars := len(msg.Content)
		for _, tc := range msg.ToolCalls {
			msgChars += len(tc.Function.Arguments)
		}
		if len(kept) >= msgBudget || totalChars+msgChars > charBudget {
			break
		}
		kept = append(kept, msg)
		totalChars += msgChars
	}

	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}

	result := make([]Message, 0, 1+len(kept))
	result = append(result, systemMsg)
	result = append(result, kept...)
	return result
}
