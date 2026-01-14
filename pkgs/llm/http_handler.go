package llm

import (
	"auth"
	"encoding/json"
	"fmt"
	"io"
	log "mylog"
	"net/http"
)

// ProcessRequest handles HTTP requests for assistant chat
func ProcessRequest(r *http.Request, w http.ResponseWriter) int {
	if r.Method != http.MethodPost {
		log.WarnF(log.ModuleLLM, "Invalid method %s for assistant chat from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return http.StatusMethodNotAllowed
	}

	// Read request body
	log.Debug(log.ModuleLLM, "Reading assistant chat request body...")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Error reading assistant chat request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	defer r.Body.Close()

	log.DebugF(log.ModuleLLM, "Received assistant chat request body: %d bytes", len(body))

	// Parse request
	var request struct {
		Messages    []Message `json:"messages"`
		Stream      bool      `json:"stream"`
		Tools       []string  `json:"selected_tools,omitempty"`
		TypingSpeed string    `json:"typing_speed,omitempty"` // Typing speed setting
	}

	if err := json.Unmarshal(body, &request); err != nil {
		log.ErrorF(log.ModuleLLM, "Error parsing assistant chat request body: %v", err)
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.InfoF(log.ModuleLLM, "Assistant chat request parsed: %d messages, stream=%t, tools=%v", len(request.Messages), request.Stream, request.Tools)

	// Extract last user message as query
	var userQuery string
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role == "user" {
			userQuery = request.Messages[i].Content
			break
		}
	}

	if userQuery == "" {
		log.WarnF(log.ModuleLLM, "No user message found in conversation")
		http.Error(w, "No user query found", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.DebugF(log.ModuleLLM, "Extracted user query: %s", userQuery)

	// Save conversation to blog (commented out in original)
	//log.Debug("Starting background conversation save to blog...")
	//go SaveConversationToBlog(request.Messages)

	// Set streaming response headers
	log.Debug(log.ModuleLLM, "Setting up streaming response headers...")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.ErrorF(log.ModuleLLM, "Streaming not supported by response writer")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}

	// Use streaming to process query, directly forward LLM streaming response
	session, err := r.Cookie("session")
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Error getting session cookie: %v", err)
		http.Error(w, "Error getting session cookie", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	account := auth.GetAccountBySession(session.Value)
	log.InfoF(log.ModuleLLM, "Processing query with streaming LLM: account=%s %s", account, userQuery)
	err = ProcessQueryStreaming(account, userQuery, request.Tools, w, flusher)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Streaming ProcessQuery failed: %v", err)
		fmt.Fprintf(w, "data: Error processing query: %v\n\n", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return http.StatusInternalServerError
	}

	// Send completion signal
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	log.Debug(log.ModuleLLM, "=== Assistant Chat Request Completed (MCP Mode) ===")

	return http.StatusOK
}
