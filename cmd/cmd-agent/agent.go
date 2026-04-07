package main

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"uap"
)

type pendingToolCall struct {
	ch chan toolCallResult
}

type toolCallResult struct {
	Success bool
	Result  string
	Error   string
}

type sessionRoute struct {
	SourceAgentID string
	Channel       string
	UserID        string
	TargetAgentID string
	Project       string
	Kind          string
	AutoDeploy    bool
}

type userCodegenSession struct {
	SessionID     string
	TargetAgentID string
	Project       string
	Backend       string
}

type gatewayAgentSnapshot struct {
	AgentID      string         `json:"agent_id"`
	AgentType    string         `json:"agent_type"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	HostPlatform string         `json:"host_platform"`
	HostIP       string         `json:"host_ip"`
	Workspace    string         `json:"workspace"`
	Tools        []string       `json:"tools"`
	Capacity     int            `json:"capacity"`
	Meta         map[string]any `json:"meta"`
}

type inboundNotify struct {
	Channel string `json:"channel"`
	To      string `json:"to"`
	Content string `json:"content"`
}

type inboundAppEnvelope struct {
	Kind        string `json:"kind"`
	UserID      string `json:"user_id"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	Scope       string `json:"scope"`
}

type codegenStreamEventPayload struct {
	SessionID string             `json:"session_id"`
	RequestID string             `json:"request_id,omitempty"`
	Account   string             `json:"account,omitempty"`
	Event     forwardedCodeEvent `json:"event"`
}

type forwardedCodeEvent struct {
	Type     string  `json:"type"`
	Text     string  `json:"text,omitempty"`
	ToolName string  `json:"tool_name,omitempty"`
	CostUSD  float64 `json:"cost_usd,omitempty"`
	Done     bool    `json:"done,omitempty"`
}

type codegenTaskCompletePayload struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id,omitempty"`
	Account   string `json:"account,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type codegenToolResult struct {
	SessionID    string `json:"session_id"`
	ProjectDir   string `json:"project_dir,omitempty"`
	Summary      string `json:"summary,omitempty"`
	FilesWritten int    `json:"files_written,omitempty"`
	FilesEdited  int    `json:"files_edited,omitempty"`
}

type deployAcceptedResult struct {
	Success    bool   `json:"success"`
	SessionID  string `json:"session_id"`
	Status     string `json:"status,omitempty"`
	Project    string `json:"project,omitempty"`
	Pipeline   string `json:"pipeline,omitempty"`
	PackOnly   bool   `json:"pack_only,omitempty"`
	Target     string `json:"deploy_target,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
	Error      string `json:"error,omitempty"`
}

type CMDAGent struct {
	cfg         *Config
	client      *uap.Client
	gatewayHTTP string
	httpClient  *http.Client

	mu               sync.Mutex
	pendingCalls     map[string]*pendingToolCall
	pendingRoutes    map[string]sessionRoute
	sessionRoutes    map[string]sessionRoute
	userCodeSessions map[string]userCodegenSession
}

func NewCMDAgent(cfg *Config) (*CMDAGent, error) {
	gatewayHTTP, err := gatewayHTTPURL(cfg.GatewayURL)
	if err != nil {
		return nil, err
	}

	client := uap.NewClient(cfg.GatewayURL, cfg.AgentID, "cmd", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Description = "统一处理 /cg 命令并分发到 acp-agent / deploy-agent"

	a := &CMDAGent{
		cfg:              cfg,
		client:           client,
		gatewayHTTP:      gatewayHTTP,
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		pendingCalls:     make(map[string]*pendingToolCall),
		pendingRoutes:    make(map[string]sessionRoute),
		sessionRoutes:    make(map[string]sessionRoute),
		userCodeSessions: make(map[string]userCodegenSession),
	}
	client.OnMessage = a.handleMessage
	return a, nil
}

func (a *CMDAGent) Run() {
	a.client.Run()
}

func (a *CMDAGent) Stop() {
	a.client.Stop()
}

func (a *CMDAGent) handleMessage(msg *uap.Message) {
	switch msg.Type {
	case uap.MsgRegister:
		a.handleRegister(msg)
	case uap.MsgHeartbeat:
		a.handleHeartbeat(msg)
	case uap.MsgNotify:
		a.handleNotify(msg)
	case uap.MsgToolResult:
		a.handleToolResult(msg)
	case uap.MsgError:
		a.handleError(msg)
	case "stream_event":
		a.handleCodegenStreamEvent(msg)
	case "task_complete":
		a.handleCodegenTaskComplete(msg)
	}
}

func (a *CMDAGent) handleRegister(msg *uap.Message) {
	_ = a.client.SendTo(msg.From, uap.MsgRegisterAck, uap.RegisterAckPayload{Success: true})
}

func (a *CMDAGent) handleHeartbeat(msg *uap.Message) {
	_ = a.client.SendTo(msg.From, uap.MsgHeartbeatAck, struct{}{})
}

func (a *CMDAGent) handleNotify(msg *uap.Message) {
	var payload inboundNotify
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid notify payload: %v", err)
		return
	}

	switch payload.Channel {
	case "app", "wechat":
		go a.handleUserCommand(msg.From, payload)
	case "acp_stream":
		a.handleACPStreamNotify(msg.From, payload)
	case "tool_progress":
		a.forwardToolProgress(payload)
	default:
		log.MessageF(log.ModuleAgent, "cmd-agent ignore notify channel=%s from=%s", payload.Channel, msg.From)
	}
}

func (a *CMDAGent) handleACPStreamNotify(sourceAgentID string, payload inboundNotify) {
	var stream codegenStreamEventPayload
	if err := json.Unmarshal([]byte(payload.Content), &stream); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid acp_stream payload: %v", err)
		return
	}

	route, ok := a.lookupRoute(firstNonEmpty(payload.To, stream.RequestID), stream.SessionID)
	if !ok {
		log.WarnF(log.ModuleAgent, "cmd-agent missing route for acp_stream request_id=%s session_id=%s", stream.RequestID, stream.SessionID)
		return
	}
	stream.Account = route.UserID
	if stream.SessionID != "" {
		a.associateSessionRoute(stream.SessionID, route)
	}
	if err := a.client.SendTo(route.SourceAgentID, "stream_event", stream); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent forward acp_stream failed to=%s err=%v", route.SourceAgentID, err)
	}
}

func (a *CMDAGent) handleToolResult(msg *uap.Message) {
	var payload uap.ToolResultPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid tool_result payload: %v", err)
		return
	}

	a.mu.Lock()
	pending := a.pendingCalls[payload.RequestID]
	delete(a.pendingCalls, payload.RequestID)
	a.mu.Unlock()
	if pending == nil {
		return
	}

	select {
	case pending.ch <- toolCallResult{
		Success: payload.Success,
		Result:  payload.Result,
		Error:   payload.Error,
	}:
	default:
	}
}

func (a *CMDAGent) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid error payload: %v", err)
		return
	}

	a.mu.Lock()
	pending := a.pendingCalls[msg.ID]
	delete(a.pendingCalls, msg.ID)
	a.mu.Unlock()
	if pending == nil {
		return
	}
	select {
	case pending.ch <- toolCallResult{
		Success: false,
		Error:   payload.Message,
	}:
	default:
	}
}

func (a *CMDAGent) handleCodegenStreamEvent(msg *uap.Message) {
	var payload codegenStreamEventPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid stream_event payload: %v", err)
		return
	}

	route, ok := a.lookupRoute(payload.RequestID, payload.SessionID)
	if !ok {
		log.WarnF(log.ModuleAgent, "cmd-agent missing route for codegen stream request_id=%s session_id=%s", payload.RequestID, payload.SessionID)
		return
	}
	payload.Account = route.UserID
	if payload.SessionID != "" {
		a.associateSessionRoute(payload.SessionID, route)
	}
	if err := a.client.SendTo(route.SourceAgentID, "stream_event", payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent forward stream_event failed to=%s err=%v", route.SourceAgentID, err)
	}
}

func (a *CMDAGent) handleCodegenTaskComplete(msg *uap.Message) {
	var payload codegenTaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent invalid task_complete payload: %v", err)
		return
	}

	route, ok := a.lookupRoute(payload.RequestID, payload.SessionID)
	if !ok {
		log.WarnF(log.ModuleAgent, "cmd-agent missing route for task_complete request_id=%s session_id=%s", payload.RequestID, payload.SessionID)
		return
	}
	payload.Account = route.UserID
	if payload.SessionID != "" {
		a.associateSessionRoute(payload.SessionID, route)
	}
	if err := a.client.SendTo(route.SourceAgentID, "task_complete", payload); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent forward task_complete failed to=%s err=%v", route.SourceAgentID, err)
	}
}

func (a *CMDAGent) forwardToolProgress(payload inboundNotify) {
	route, ok := a.lookupRoute(payload.To, payload.To)
	if !ok {
		return
	}
	if err := a.sendClientNotify(route, payload.Content); err != nil {
		log.WarnF(log.ModuleAgent, "cmd-agent forward tool progress failed: %v", err)
	}
}

func (a *CMDAGent) handleUserCommand(sourceAgentID string, payload inboundNotify) {
	userID, content := unwrapInboundCommand(payload)
	if userID == "" {
		userID = payload.To
	}
	content = normalizeCodegenCommand(content)
	log.MessageF(log.ModuleAgent, "cmd-agent inbound command from=%s user=%s channel=%s content=%s",
		sourceAgentID, userID, payload.Channel, content)

	if !isCGCommand(content) {
		_ = a.client.SendTo(sourceAgentID, uap.MsgNotify, uap.NotifyPayload{
			Channel: payload.Channel,
			To:      userID,
			Content: "⚠️ cmd-agent 只处理 /cg 命令",
		})
		return
	}

	req := commandRequest{
		SourceAgentID: sourceAgentID,
		Channel:       payload.Channel,
		UserID:        userID,
		Content:       content,
	}
	if err := a.dispatchCommand(req); err != nil {
		_ = a.sendClientNotify(sessionRoute{
			SourceAgentID: sourceAgentID,
			Channel:       payload.Channel,
			UserID:        userID,
		}, fmt.Sprintf("❌ %v", err))
	}
}

func (a *CMDAGent) callTool(agentID, requestID, toolName string, args map[string]any) (<-chan toolCallResult, error) {
	ch := make(chan toolCallResult, 1)

	a.mu.Lock()
	a.pendingCalls[requestID] = &pendingToolCall{ch: ch}
	a.mu.Unlock()

	payload := uap.ToolCallPayload{
		ToolName:  toolName,
		Arguments: mustMarshalJSON(args),
	}
	err := a.client.Send(&uap.Message{
		Type:    uap.MsgToolCall,
		ID:      requestID,
		From:    a.client.AgentID,
		To:      agentID,
		Payload: mustMarshalJSON(payload),
		Ts:      time.Now().UnixMilli(),
	})
	if err != nil {
		a.mu.Lock()
		delete(a.pendingCalls, requestID)
		a.mu.Unlock()
		return nil, err
	}
	return ch, nil
}

func (a *CMDAGent) fetchGatewayAgents() ([]gatewayAgentSnapshot, error) {
	resp, err := a.httpClient.Get(a.gatewayHTTP + "/api/gateway/agents")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool                   `json:"success"`
		Agents  []gatewayAgentSnapshot `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("gateway returned success=false")
	}
	return result.Agents, nil
}

func (a *CMDAGent) sendClientNotify(route sessionRoute, content string) error {
	if strings.TrimSpace(route.SourceAgentID) == "" || strings.TrimSpace(route.Channel) == "" || strings.TrimSpace(route.UserID) == "" {
		return fmt.Errorf("invalid notify route")
	}
	return a.client.SendTo(route.SourceAgentID, uap.MsgNotify, uap.NotifyPayload{
		Channel: route.Channel,
		To:      route.UserID,
		Content: content,
	})
}

func (a *CMDAGent) sendTaskComplete(route sessionRoute, sessionID, status, errText string) error {
	sessionID = firstNonEmpty(sessionID)
	if strings.TrimSpace(route.SourceAgentID) == "" || sessionID == "" {
		return nil
	}
	payload := codegenTaskCompletePayload{
		SessionID: sessionID,
		Account:   route.UserID,
		Status:    status,
		Error:     errText,
	}
	return a.client.SendTo(route.SourceAgentID, "task_complete", payload)
}

func (a *CMDAGent) setPendingRoute(trackID string, route sessionRoute) {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return
	}
	a.mu.Lock()
	a.pendingRoutes[trackID] = route
	a.mu.Unlock()
}

func (a *CMDAGent) associateSessionRoute(sessionID string, route sessionRoute) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	a.mu.Lock()
	a.sessionRoutes[sessionID] = route
	a.mu.Unlock()
}

func (a *CMDAGent) lookupRoute(trackID, sessionID string) (sessionRoute, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if trackID = strings.TrimSpace(trackID); trackID != "" {
		if route, ok := a.pendingRoutes[trackID]; ok {
			return route, true
		}
	}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		if route, ok := a.sessionRoutes[sessionID]; ok {
			return route, true
		}
	}
	return sessionRoute{}, false
}

func (a *CMDAGent) rememberUserCodegenSession(userID string, sess userCodegenSession) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sess.SessionID) == "" {
		return
	}
	a.mu.Lock()
	a.userCodeSessions[userID] = sess
	a.mu.Unlock()
}

func (a *CMDAGent) getUserCodegenSession(userID string) (userCodegenSession, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sess, ok := a.userCodeSessions[userID]
	return sess, ok
}

func gatewayHTTPURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported gateway url scheme: %s", u.Scheme)
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func hasTool(agent gatewayAgentSnapshot, toolName string) bool {
	for _, tool := range agent.Tools {
		if tool == toolName {
			return true
		}
	}
	return false
}

func projectNamesFromMeta(meta map[string]any) []string {
	return stringSliceFromAny(meta["projects"])
}

func modelNamesFromMeta(meta map[string]any) []string {
	out := append([]string{}, stringSliceFromAny(meta["models"])...)
	out = append(out, stringSliceFromAny(meta["claudecode_models"])...)
	out = append(out, stringSliceFromAny(meta["opencode_models"])...)
	return uniqueSorted(out)
}

func codingToolsFromMeta(meta map[string]any) []string {
	return uniqueSorted(stringSliceFromAny(meta["coding_tools"]))
}

func stringSliceFromAny(v any) []string {
	switch val := v.(type) {
	case []string:
		return append([]string{}, val...)
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func uniqueSorted(items []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
