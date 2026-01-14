package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mcp"
	log "mylog"
	"net/http"
	"strings"
	"time"
)

// SendStreamingLLMRequest sends streaming LLM request and detects tool calls
func SendStreamingLLMRequest(messages []Message, availableTools []mcp.LLMTool, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	// Prepare sanitized messages to fit context budget
	sanitizedMessages := SanitizeMessages(messages)

	attempts := []struct {
		perMessageMax int
		totalBudget   int
		maxMsgs       int
	}{
		{MaxMessageChars, MaxTotalCharsBudget, MaxMessagesToSend},
		{4000, 100000, 40}, // stricter fallback
		{2000, 60000, 30},  // most strict fallback
	}

	config := GetConfig()

	for idx, lim := range attempts {
		if idx > 0 {
			// recompute with stricter limits
			sanitizedMessages = SanitizeMessagesWithLimits(messages, lim.perMessageMax, lim.totalBudget, lim.maxMsgs)
		}

		// Create LLM request with streaming enabled
		requestBody := map[string]interface{}{
			"model":       config.Model,
			"messages":    sanitizedMessages,
			"tools":       availableTools,
			"temperature": config.Temperature,
			"stream":      true, // Enable streaming response
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "Error marshaling LLM request: %v", err)
			return "", nil, fmt.Errorf("error marshaling request: %v", err)
		}

		log.DebugF(log.ModuleLLM, "Sending streaming request to LLM API: %s (attempt %d)", config.BaseURL, idx+1)

		// Create HTTP request to LLM API
		req, err := http.NewRequest("POST", config.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.ErrorF(log.ModuleLLM, "Error creating LLM request: %v", err)
			return "", nil, fmt.Errorf("error creating request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
		req.Header.Set("Accept", "text/event-stream")

		// Send request with streaming support
		client := &http.Client{
			Timeout: 300 * time.Second, // 5 minutes timeout
		}

		resp, err := client.Do(req)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "Error sending request to LLM API: %v", err)
			return "", nil, fmt.Errorf("error sending request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.ErrorF(log.ModuleLLM, "LLM API returned status %d: %s", resp.StatusCode, string(body))
			// retry only on context-length style errors, otherwise fail fast
			if resp.StatusCode == http.StatusBadRequest && (strings.Contains(strings.ToLower(string(body)), "maximum context length") || strings.Contains(strings.ToLower(string(body)), "context")) {
				if idx < len(attempts)-1 {
					log.WarnF(log.ModuleLLM, "Retrying with stricter message limits due to context length error")
					continue
				}
			}
			return "", nil, fmt.Errorf("LLM API error: %d", resp.StatusCode)
		}

		// Ensure body is closed after processing
		defer resp.Body.Close()

		log.DebugF(log.ModuleLLM, "Received streaming response from LLM API, processing...")

		// Process the streaming response
		return ProcessStreamingResponseWithToolDetection(resp.Body, w, flusher, fullResponse)
	}

	return "", nil, fmt.Errorf("failed to send request after retries")
}
