package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mcp"
	log "mylog"
	"net/http"
	"net/url"
	"strings"
)

// StreamProcessor handles streaming response processing
type StreamProcessor struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(w http.ResponseWriter, flusher http.Flusher) *StreamProcessor {
	return &StreamProcessor{
		writer:  w,
		flusher: flusher,
	}
}

// WriteContent writes content to the response and flushes
func (sp *StreamProcessor) WriteContent(content string) {
	fmt.Fprintf(sp.writer, "data: %s\n\n", url.QueryEscape(content))
	sp.flusher.Flush()
}

// WriteToolStatus writes tool call status to the response
func (sp *StreamProcessor) WriteToolStatus(toolName, args string) {
	fmt.Fprintf(sp.writer, "data: %s\n\n", url.QueryEscape(fmt.Sprintf("[Calling tool %s with args %s]", toolName, args)))
	sp.flusher.Flush()
}

// WriteDone writes the completion signal
func (sp *StreamProcessor) WriteDone() {
	fmt.Fprintf(sp.writer, "data: [DONE]\n\n")
	sp.flusher.Flush()
}

// ProcessStreamingResponseWithToolDetection processes streaming response and detects tool calls
func ProcessStreamingResponseWithToolDetection(responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	log.DebugF(log.ModuleLLM, "Starting streaming response processing with tool detection")
	scanner := bufio.NewScanner(responseBody)
	var responseContent strings.Builder
	var toolCalls []mcp.ToolCall
	var currentToolCall *mcp.ToolCall

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle SSE format: "data: ..."
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle completion signal
			if data == "[DONE]" {
				log.DebugF(log.ModuleLLM, "LLM streaming completed")
				break
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF(log.ModuleLLM, "Failed to parse streaming chunk: %v", err)
				continue
			}

			// Extract content from chunk
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {

						// Handle regular content
						if content, ok := delta["content"].(string); ok && content != "" {
							log.DebugF(log.ModuleLLM, "Tool-aware streaming: forwarding content chunk: %s", content)
							// Forward content to client immediately
							fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(content))
							flusher.Flush()

							// Accumulate for processing and saving
							responseContent.WriteString(content)
							fullResponse.WriteString(content)
						}

						// Handle tool calls
						if toolCallsRaw, ok := delta["tool_calls"].([]interface{}); ok {
							for _, toolCallRaw := range toolCallsRaw {
								if toolCallMap, ok := toolCallRaw.(map[string]interface{}); ok {
									// Parse tool call
									if err := ParseToolCallFromDelta(toolCallMap, &currentToolCall, &toolCalls); err != nil {
										log.ErrorF(log.ModuleLLM, "Failed to parse tool call: %v", err)
									}
								}
							}
						}

						// Check for finish reason
						if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" && finishReason != "null" {
							log.DebugF(log.ModuleLLM, "Finish reason: %s", finishReason)
							if finishReason == "tool_calls" {
								log.DebugF(log.ModuleLLM, "Tool calls detected, finishing content streaming")
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF(log.ModuleLLM, "Error reading streaming response: %v", err)
		return "", nil, fmt.Errorf("error reading stream: %v", err)
	}

	log.DebugF(log.ModuleLLM, "Streaming response processed. Content length: %d, Tool calls: %d", responseContent.Len(), len(toolCalls))
	return responseContent.String(), toolCalls, nil
}

// ParseToolCallFromDelta parses incremental tool call data
func ParseToolCallFromDelta(toolCallMap map[string]interface{}, currentToolCall **mcp.ToolCall, toolCalls *[]mcp.ToolCall) error {
	index, hasIndex := toolCallMap["index"].(float64)
	if !hasIndex {
		log.WarnF(log.ModuleLLM, "Tool call chunk missing index, skipping")
		return nil
	}

	// Initialize new tool call if needed
	if *currentToolCall == nil || int(index) != len(*toolCalls) {
		*currentToolCall = &mcp.ToolCall{}
		if id, ok := toolCallMap["id"].(string); ok {
			(*currentToolCall).ID = id
		}
		if typeStr, ok := toolCallMap["type"].(string); ok && typeStr == "function" {
			(*currentToolCall).Type = typeStr
		}
	}

	// Parse function details
	if function, ok := toolCallMap["function"].(map[string]interface{}); ok {
		if name, ok := function["name"].(string); ok {
			(*currentToolCall).Function.Name = name
		}
		if arguments, ok := function["arguments"].(string); ok {
			(*currentToolCall).Function.Arguments += arguments
		}
	}

	// If this tool call seems complete, add it to the list
	if (*currentToolCall).ID != "" && (*currentToolCall).Function.Name != "" && (*currentToolCall).Function.Arguments != "" {
		// Validate that arguments is valid JSON before adding to list
		if IsValidJSON((*currentToolCall).Function.Arguments) {
			// Check if this tool call is already in the list
			found := false
			for i, tc := range *toolCalls {
				if tc.ID == (*currentToolCall).ID {
					(*toolCalls)[i] = **currentToolCall // Update existing
					found = true
					break
				}
			}
			if !found {
				*toolCalls = append(*toolCalls, **currentToolCall)
			}
		} else {
			log.DebugF(log.ModuleLLM, "Tool call arguments not yet complete: %s", (*currentToolCall).Function.Arguments)
		}
	}

	return nil
}

// IsValidJSON checks if a string is valid JSON
func IsValidJSON(str string) bool {
	var js interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// ForwardStreamingResponse forwards LLM streaming response to client
func ForwardStreamingResponse(account string, responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, originalQuery string) error {
	scanner := bufio.NewScanner(responseBody)
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle SSE format: "data: ..."
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle completion signal
			if data == "[DONE]" {
				log.DebugF(log.ModuleLLM, "LLM streaming completed")
				// Save full response to diary
				go SaveLLMResponseToDiary(account, originalQuery, fullResponse.String())
				return nil
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF(log.ModuleLLM, "Failed to parse streaming chunk: %v", err)
				continue
			}

			// Extract content from chunk
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							// Forward content to client immediately
							fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(content))
							flusher.Flush()

							// Accumulate for saving
							fullResponse.WriteString(content)
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF(log.ModuleLLM, "Error reading streaming response: %v", err)
		return fmt.Errorf("error reading stream: %v", err)
	}

	// Save final response
	go SaveLLMResponseToDiary(account, originalQuery, fullResponse.String())
	return nil
}
