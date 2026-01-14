package llm

import (
	"mcp"
)

// Message represents a message in conversation
type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []mcp.ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string         `json:"tool_call_id,omitempty"`
}

// Choice represents a choice in LLM response
type Choice struct {
	Index        int       `json:"index"`
	Message      Message   `json:"message"`
	LogProbs     *struct{} `json:"logprobs"`
	FinishReason string    `json:"finish_reason"`
}

// Usage represents the usage statistics in LLM response
type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
}

// LLMRequest represents request to LLM API
type LLMRequest struct {
	Model       string        `json:"model"`
	Messages    []Message     `json:"messages"`
	Tools       []mcp.LLMTool `json:"tools,omitempty"`
	Temperature float64       `json:"temperature"`
}

// LLMResponse represents response from LLM API
type LLMResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint"`
}

// TruncateString truncates a string with a marker if it exceeds max length
func TruncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "... [truncated]"
}

// SanitizeToolCall limits the size of tool call arguments
func SanitizeToolCall(tc mcp.ToolCall) mcp.ToolCall {
	if len(tc.Function.Arguments) > MaxToolArgumentsChars {
		tc.Function.Arguments = TruncateString(tc.Function.Arguments, MaxToolArgumentsChars)
	}
	return tc
}

// SanitizeMessages sanitizes/prunes messages to stay within budget (default limits)
func SanitizeMessages(original []Message) []Message {
	return SanitizeMessagesWithLimits(original, MaxMessageChars, MaxTotalCharsBudget, MaxMessagesToSend)
}

// SanitizeMessagesWithLimits sanitizes messages with adjustable limits for retry
func SanitizeMessagesWithLimits(original []Message, perMessageMax, totalBudget, maxMsgs int) []Message {
	var totalChars int
	var resultReversed []Message

	// Preserve the first system message if present
	var system *Message
	if len(original) > 0 && original[0].Role == "system" {
		sys := original[0]
		if len(sys.Content) > perMessageMax {
			sys.Content = TruncateString(sys.Content, perMessageMax)
		}
		system = &sys
	}

	// Walk from end to start so we keep the most recent turns
	for i := len(original) - 1; i >= 0; i-- {
		if system != nil && i == 0 {
			continue
		}

		msg := original[i]

		// Clamp message content
		if msg.Content != "" && len(msg.Content) > perMessageMax {
			msg.Content = TruncateString(msg.Content, perMessageMax)
		}

		// Clamp any tool calls embedded in assistant message
		if len(msg.ToolCalls) > 0 {
			sanitizedCalls := make([]mcp.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				sanitizedCalls = append(sanitizedCalls, SanitizeToolCall(tc))
			}
			msg.ToolCalls = sanitizedCalls
		}

		// Rough contribution to budget
		approx := len(msg.Content)
		for _, tc := range msg.ToolCalls {
			approx += len(tc.Function.Name) + len(tc.Function.Arguments) + len(tc.ID)
		}

		// Enforce message count cap (reserve one slot for system if any)
		if len(resultReversed) >= maxMsgs-1 {
			break
		}
		// Enforce total char budget
		if totalChars+approx > totalBudget {
			break
		}

		totalChars += approx
		resultReversed = append(resultReversed, msg)
	}

	// Reverse back to chronological order
	for i, j := 0, len(resultReversed)-1; i < j; i, j = i+1, j-1 {
		resultReversed[i], resultReversed[j] = resultReversed[j], resultReversed[i]
	}

	if system != nil {
		return append([]Message{*system}, resultReversed...)
	}
	return resultReversed
}
