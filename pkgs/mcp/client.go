package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	log "mylog"
	"os"
	"os/exec"
	"time"
)

// MCPClient represents a persistent MCP client connection
type MCPClient struct {
	config    MCPConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	reader    *bufio.Scanner
	writer    *json.Encoder
	connected bool
	requests  map[string]chan MCPClientResponse
	nextID    int
}

// MCPClientResponse represents a response from the MCP client
type MCPClientResponse struct {
	Success bool
	Data    interface{}
	Error   string
}

// NewMCPClient creates a new MCP client
func NewMCPClient(config MCPConfig) *MCPClient {
	return &MCPClient{
		config:   config,
		requests: make(map[string]chan MCPClientResponse),
		nextID:   1,
	}
}

// Connect establishes connection to the MCP server
func (c *MCPClient) Connect() error {

	if c.connected {
		return nil
	}

	// Start the MCP server process
	c.cmd = exec.Command(c.config.Command, c.config.Args...)

	// Set environment variables
	c.cmd.Env = os.Environ()
	for key, value := range c.config.Environment {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set up pipes
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %v", err)
	}

	// Set up JSON encoder/decoder
	c.writer = json.NewEncoder(c.stdin)
	c.reader = bufio.NewScanner(c.stdout)

	// Start response handler
	go c.handleResponses()

	// Initialize the connection
	if err := c.initialize(); err != nil {
		c.Close()
		return fmt.Errorf("failed to initialize MCP connection: %v", err)
	}

	c.connected = true
	log.DebugF(log.ModuleMCP, "MCP client connected to %s", c.config.Name)
	return nil
}

// initialize sends the initialization request
func (c *MCPClient) initialize() error {
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "go-blog-assistant",
				"version": "1.0.0",
			},
		},
	}

	response, err := c.sendRequest(initRequest)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("initialization failed: %s", response.Error)
	}

	// Send initialized notification
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	return c.writer.Encode(initNotification)
}

// sendRequest sends a request and waits for response
func (c *MCPClient) sendRequest(request map[string]interface{}) (MCPClientResponse, error) {
	id := fmt.Sprintf("%v", request["id"])

	// Create response channel
	responseChan := make(chan MCPClientResponse, 1)
	c.requests[id] = responseChan

	// Send request
	if err := c.writer.Encode(request); err != nil {
		delete(c.requests, id)
		return MCPClientResponse{}, fmt.Errorf("failed to send request: %v", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		delete(c.requests, id)
		return response, nil
	case <-time.After(30 * time.Second):
		delete(c.requests, id)
		return MCPClientResponse{}, fmt.Errorf("request timeout")
	}
}

// handleResponses processes incoming responses
func (c *MCPClient) handleResponses() {
	for c.reader.Scan() {
		line := c.reader.Text()
		if line == "" {
			continue
		}

		var response struct {
			JSONRPC string      `json:"jsonrpc"`
			ID      interface{} `json:"id,omitempty"`
			Result  interface{} `json:"result,omitempty"`
			Error   *MCPError   `json:"error,omitempty"`
			Method  string      `json:"method,omitempty"`
		}

		if err := json.Unmarshal([]byte(line), &response); err != nil {
			log.ErrorF(log.ModuleMCP, "Failed to parse MCP response: %v", err)
			continue
		}

		// Handle response with ID
		if response.ID != nil {
			id := fmt.Sprintf("%v", response.ID)
			if responseChan, exists := c.requests[id]; exists {
				clientResponse := MCPClientResponse{
					Success: response.Error == nil,
					Data:    response.Result,
				}

				if response.Error != nil {
					clientResponse.Error = response.Error.Message
				}

				select {
				case responseChan <- clientResponse:
				default:
					// Channel full, drop response
				}
			}
		}

		// Handle notifications
		if response.Method != "" {
			c.handleNotification(response.Method, response.Result)
		}
	}

	// Scanner ended, connection closed
	c.connected = false
}

// handleNotification handles incoming notifications
func (c *MCPClient) handleNotification(method string, params interface{}) {
	log.DebugF(log.ModuleMCP, "Received MCP notification: %s", method)
	// Handle specific notifications here
}

// ListTools retrieves available tools from the server
func (c *MCPClient) ListTools() ([]MCPTool, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "tools/list",
	}

	response, err := c.sendRequest(request)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("failed to list tools: %s", response.Error)
	}

	// Parse tools from response
	var tools []MCPTool
	if result, ok := response.Data.(map[string]interface{}); ok {
		if toolsList, ok := result["tools"].([]interface{}); ok {
			for _, toolData := range toolsList {
				if toolMap, ok := toolData.(map[string]interface{}); ok {
					tool := MCPTool{
						Name:        fmt.Sprintf("%s.%s", c.config.Name, getString(toolMap, "name")),
						Description: getString(toolMap, "description"),
						InputSchema: toolMap["inputSchema"],
					}
					tools = append(tools, tool)
				}
			}
		}
	}

	return tools, nil
}

// CallTool executes a tool call
func (c *MCPClient) CallTool(toolName string, arguments map[string]interface{}) (MCPClientResponse, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return MCPClientResponse{}, err
		}
	}

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.getNextID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	return c.sendRequest(request)
}

// Close closes the MCP client connection
func (c *MCPClient) Close() error {

	c.connected = false

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}

	// Clear pending requests
	for id, ch := range c.requests {
		close(ch)
		delete(c.requests, id)
	}

	log.DebugF(log.ModuleMCP, "MCP client disconnected from %s", c.config.Name)
	return nil
}

// getNextID generates the next request ID
func (c *MCPClient) getNextID() string {
	id := fmt.Sprintf("req_%d_%d", time.Now().Unix(), c.nextID)
	c.nextID++
	return id
}

// IsConnected returns whether the client is connected
func (c *MCPClient) IsConnected() bool {
	return c.connected
}

// GetConfig returns the client configuration
func (c *MCPClient) GetConfig() MCPConfig {
	return c.config
}
