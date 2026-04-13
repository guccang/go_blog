package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type codexSettingsFile struct {
	Env                                  map[string]interface{} `json:"env"`
	Model                                string                 `json:"model"`
	Profile                              string                 `json:"profile"`
	SandboxMode                          string                 `json:"sandbox_mode"`
	FullAuto                             bool                   `json:"full_auto"`
	DangerouslyBypassApprovalsAndSandbox bool                   `json:"dangerously_bypass_approvals_and_sandbox"`
	SkipGitRepoCheck                     *bool                  `json:"skip_git_repo_check"`
	ConfigHome                           string                 `json:"config_home"`
	ConfigOverrides                      []string               `json:"config_overrides"`
}

type codexExecutionPlan struct {
	Args     []string
	Env      []string
	Model    string
	Warnings []string
}

type codexThreadEvent struct {
	Type     string           `json:"type"`
	ThreadID string           `json:"thread_id,omitempty"`
	Item     *codexThreadItem `json:"item,omitempty"`
	Error    *codexErrorItem  `json:"error,omitempty"`
	Message  string           `json:"message,omitempty"`
}

type codexThreadItem struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Text             string            `json:"text,omitempty"`
	Message          string            `json:"message,omitempty"`
	Command          string            `json:"command,omitempty"`
	AggregatedOutput string            `json:"aggregated_output,omitempty"`
	ExitCode         *int              `json:"exit_code,omitempty"`
	Status           string            `json:"status,omitempty"`
	Changes          []codexFileChange `json:"changes,omitempty"`
	Items            []codexTodoItem   `json:"items,omitempty"`
	Server           string            `json:"server,omitempty"`
	Tool             string            `json:"tool,omitempty"`
	Error            *codexErrorItem   `json:"error,omitempty"`
	Query            string            `json:"query,omitempty"`
}

type codexFileChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type codexTodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type codexErrorItem struct {
	Message string `json:"message"`
}

type codexRunState struct {
	threadID     string
	lastMessage  string
	lastError    string
	filesWritten map[string]bool
	filesEdited  map[string]bool
}

func newCodexRunState() *codexRunState {
	return &codexRunState{
		filesWritten: make(map[string]bool),
		filesEdited:  make(map[string]bool),
	}
}

func (s *codexRunState) handleJSONLine(line string, emit func(StreamEvent)) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var evt codexThreadEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return fmt.Errorf("parse codex jsonl: %v", err)
	}

	switch evt.Type {
	case "thread.started":
		s.threadID = evt.ThreadID
	case "turn.failed":
		if evt.Error != nil && strings.TrimSpace(evt.Error.Message) != "" {
			s.lastError = evt.Error.Message
		} else if strings.TrimSpace(evt.Message) != "" {
			s.lastError = evt.Message
		}
	case "error":
		if evt.Error != nil && strings.TrimSpace(evt.Error.Message) != "" {
			s.lastError = evt.Error.Message
		} else if strings.TrimSpace(evt.Message) != "" {
			s.lastError = evt.Message
		}
	case "item.started", "item.updated", "item.completed":
		if evt.Item != nil {
			s.handleItemEvent(evt.Type, evt.Item, emit)
		}
	}

	return nil
}

func (s *codexRunState) handleItemEvent(phase string, item *codexThreadItem, emit func(StreamEvent)) {
	switch item.Type {
	case "agent_message":
		if phase == "item.completed" && strings.TrimSpace(item.Text) != "" {
			s.lastMessage = item.Text
			emit(StreamEvent{Type: "assistant", Text: item.Text})
		}
	case "reasoning":
		if phase == "item.completed" && strings.TrimSpace(item.Text) != "" {
			emit(StreamEvent{Type: "thought", Text: item.Text})
		}
	case "command_execution":
		title := previewText(strings.TrimSpace(item.Command), 160)
		if title == "" {
			title = "command"
		}
		switch phase {
		case "item.started":
			emit(StreamEvent{Type: "tool", ToolName: title, Text: fmt.Sprintf("🔧 %s", title)})
		case "item.updated", "item.completed":
			status := strings.TrimSpace(item.Status)
			if status == "" {
				status = "updated"
			}
			text := fmt.Sprintf("🔧 %s [%s]", title, status)
			if item.ExitCode != nil {
				text += fmt.Sprintf(" (exit=%d)", *item.ExitCode)
			}
			emit(StreamEvent{Type: "tool_update", Text: text})
		}
	case "mcp_tool_call":
		title := strings.TrimSpace(item.Server + "/" + item.Tool)
		if title == "/" {
			title = ""
		}
		if title == "" {
			title = strings.TrimSpace(item.Tool)
		}
		if title == "" {
			title = "mcp_tool"
		}
		switch phase {
		case "item.started":
			emit(StreamEvent{Type: "tool", ToolName: title, Text: fmt.Sprintf("🔧 %s", title)})
		case "item.updated", "item.completed":
			status := strings.TrimSpace(item.Status)
			if status == "" {
				status = "updated"
			}
			emit(StreamEvent{Type: "tool_update", Text: fmt.Sprintf("🔧 %s [%s]", title, status)})
		}
	case "todo_list":
		if len(item.Items) > 0 {
			emit(StreamEvent{Type: "plan", Text: formatCodexTodoPlan(item.Items)})
		}
	case "file_change":
		for _, change := range item.Changes {
			switch change.Kind {
			case "add":
				s.filesWritten[change.Path] = true
			case "update":
				s.filesEdited[change.Path] = true
			}
		}
		if phase == "item.completed" && len(item.Changes) > 0 {
			files := make([]string, 0, len(item.Changes))
			for _, change := range item.Changes {
				files = append(files, change.Path)
			}
			emit(StreamEvent{
				Type: "tool_update",
				Text: fmt.Sprintf("🔧 patch [%s] (%s)", firstNonEmpty(strings.TrimSpace(item.Status), "completed"), strings.Join(files, ", ")),
			})
		}
	case "error":
		msg := strings.TrimSpace(item.Message)
		if msg == "" && item.Error != nil {
			msg = strings.TrimSpace(item.Error.Message)
		}
		if msg != "" {
			s.lastError = msg
			emit(StreamEvent{Type: "system", Text: "⚠️ Codex: " + msg})
		}
	case "web_search":
		if phase == "item.started" && strings.TrimSpace(item.Query) != "" {
			emit(StreamEvent{Type: "tool", ToolName: "web_search", Text: "🔧 web_search"})
		}
	}
}

func (s *codexRunState) summary() string {
	return strings.TrimSpace(s.lastMessage)
}

func formatCodexTodoPlan(items []codexTodoItem) string {
	if len(items) == 0 {
		return "📋 执行计划: (空)"
	}
	var sb strings.Builder
	sb.WriteString("📋 执行计划:\n")
	for i, item := range items {
		icon := "⏳"
		if item.Completed {
			icon = "✅"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, icon, item.Text))
	}
	return sb.String()
}

func buildCodexExecutionPlan(cfg *AgentConfig, extraArgs []string) (codexExecutionPlan, error) {
	plan := codexExecutionPlan{
		Args: append([]string{}, cfg.CodexArgs...),
		Env:  append([]string{}, os.Environ()...),
	}
	ensureCodexExecDefaults(&plan.Args)

	resolvedExtra := resolveSettingsArgs(extraArgs, cfg.CodexSettingsDir)
	settingsPath := extractSettingsPath(resolvedExtra)
	if settingsPath == "" && cfg.DefaultSettings != "" {
		name := cfg.DefaultSettings
		if !strings.HasSuffix(name, ".json") {
			name += ".json"
		}
		settingsPath = filepath.Join(cfg.CodexSettingsDir, name)
		if _, err := os.Stat(settingsPath); err != nil {
			return codexExecutionPlan{}, fmt.Errorf("default settings file not found: %s", settingsPath)
		}
	}

	if settingsPath != "" {
		settings, err := loadCodexSettingsFile(settingsPath)
		if err != nil {
			return codexExecutionPlan{}, err
		}
		applyCodexSettings(&plan, settings, settingsPath)
	}

	args, warnings := translateCodexExtraArgs(resolvedExtra)
	plan.Args = append(plan.Args, args...)
	plan.Warnings = append(plan.Warnings, warnings...)
	plan.Model = extractFlagValue(plan.Args, "--model")

	return plan, nil
}

func ensureCodexExecDefaults(args *[]string) {
	if len(*args) == 0 || (*args)[0] != "exec" {
		*args = append([]string{"exec"}, (*args)...)
	}
	if !hasAnyArg(*args, "--json", "--experimental-json") {
		*args = append(*args, "--json")
	}
}

func hasAnyArg(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name {
				return true
			}
		}
	}
	return false
}

func extractFlagValue(args []string, name string) string {
	var value string
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			value = args[i+1]
		}
	}
	return value
}

func loadCodexSettingsFile(path string) (*codexSettingsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read codex settings %s: %v", path, err)
	}

	var settings codexSettingsFile
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse codex settings %s: %v", path, err)
	}
	return &settings, nil
}

func applyCodexSettings(plan *codexExecutionPlan, settings *codexSettingsFile, settingsPath string) {
	if settings == nil {
		return
	}

	if settings.Model != "" {
		plan.Args = append(plan.Args, "--model", settings.Model)
		plan.Model = settings.Model
		log.Printf("[Codex] settings model: %s", settings.Model)
	}
	if settings.Profile != "" {
		plan.Args = append(plan.Args, "--profile", settings.Profile)
	}
	if settings.SandboxMode != "" {
		plan.Args = append(plan.Args, "--sandbox", settings.SandboxMode)
	}
	if settings.FullAuto {
		plan.Args = append(plan.Args, "--full-auto")
	}
	if settings.DangerouslyBypassApprovalsAndSandbox {
		plan.Args = append(plan.Args, "--dangerously-bypass-approvals-and-sandbox")
	}
	if settings.SkipGitRepoCheck != nil && *settings.SkipGitRepoCheck {
		plan.Args = append(plan.Args, "--skip-git-repo-check")
	}
	for _, override := range settings.ConfigOverrides {
		override = strings.TrimSpace(override)
		if override == "" {
			continue
		}
		plan.Args = append(plan.Args, "-c", override)
	}
	if strings.TrimSpace(settings.ConfigHome) != "" {
		configHome := settings.ConfigHome
		if !filepath.IsAbs(configHome) {
			configHome = filepath.Join(filepath.Dir(settingsPath), configHome)
		}
		plan.Env = append(plan.Env, "CODEX_HOME="+configHome)
	}
	for k, v := range settings.Env {
		plan.Env = append(plan.Env, fmt.Sprintf("%s=%v", k, v))
	}
}

func translateCodexExtraArgs(args []string) ([]string, []string) {
	var translated []string
	var warnings []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--settings":
			if i+1 < len(args) {
				i++
			}
		case "--dangerously-skip-permissions":
			translated = append(translated, "--dangerously-bypass-approvals-and-sandbox")
		case "--max-turns":
			if i+1 < len(args) {
				i++
			}
			warnings = append(warnings, "Codex backend ignores --max-turns")
		default:
			translated = append(translated, arg)
		}
	}
	return translated, warnings
}

func (a *Agent) executeCodexExec(conn *Connection, sessionID, requestID, project, prompt string, extraArgs []string, interactive bool, callerAgentID string, keepSession bool) (taskResult, error) {
	projectPath := a.resolveProject(project)
	if projectPath == "" {
		return taskResult{Status: "error"}, fmt.Errorf("project not found in workspaces: %s", project)
	}
	if interactive {
		return taskResult{Status: "error"}, fmt.Errorf("Codex backend does not support interactive approval mode yet")
	}
	if strings.TrimSpace(prompt) == "" {
		summary := "Codex backend 已就绪；当前不会创建空会话，发送下一条消息时会直接执行。"
		a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
			SessionID: sessionID,
			RequestID: requestID,
			Event: StreamEvent{
				Type: "system",
				Text: "✅ Codex 已就绪（空 prompt 不保留会话）",
				Done: true,
			},
		})
		return taskResult{
			Status:     "done",
			SessionID:  "",
			Summary:    summary,
			ProjectDir: projectPath,
		}, nil
	}

	plan, err := buildCodexExecutionPlan(a.cfg, extraArgs)
	if err != nil {
		return taskResult{Status: "error"}, err
	}

	log.Printf("[Codex] execute start: session=%s project=%s keep_session=%v prompt_len=%d", sessionID, project, keepSession, len(prompt))

	a.sessionsMu.Lock()
	a.sessions[sessionID] = &sessionRecord{
		Project:   project,
		Backend:   BackendCodexExec,
		Active:    true,
		Status:    "in_progress",
		KeepAlive: false,
	}
	a.sessionsMu.Unlock()

	a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
		SessionID: sessionID,
		RequestID: requestID,
		Event: StreamEvent{
			Type: "system",
			Text: fmt.Sprintf("🔍 Codex 会话开始... (项目: %s, Agent: %s)", project, a.cfg.AgentName),
		},
	})
	for _, warning := range plan.Warnings {
		a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
			SessionID: sessionID,
			RequestID: requestID,
			Event: StreamEvent{
				Type: "system",
				Text: "⚠️ " + warning,
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.AnalysisTimeout)*time.Second)
	defer cancel()

	cmdArgs := append([]string{}, plan.Args...)
	cmdArgs = append(cmdArgs, prompt)
	cmd := exec.CommandContext(ctx, a.cfg.CodexCmd, cmdArgs...)
	cmd.Dir = projectPath
	cmd.Env = plan.Env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		a.cleanupSessionRecord(sessionID)
		return taskResult{Status: "error"}, fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		a.cleanupSessionRecord(sessionID)
		return taskResult{Status: "error"}, fmt.Errorf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		a.completeSession(sessionID, "failed", "")
		a.cleanupSessionRecord(sessionID)
		return taskResult{Status: "error"}, fmt.Errorf("start codex exec: %v", err)
	}
	log.Printf("[Codex] started %s %s (pid=%d, dir=%s)", a.cfg.CodexCmd, strings.Join(cmdArgs, " "), cmd.Process.Pid, projectPath)

	a.sessionsMu.Lock()
	if rec, ok := a.sessions[sessionID]; ok {
		rec.ACPSession = &ACPSession{
			cmd:    cmd,
			cancel: cancel,
		}
	}
	a.sessionsMu.Unlock()

	var stderrMu sync.Mutex
	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			stderrMu.Lock()
			if stderrBuf.Len() > 0 {
				stderrBuf.WriteByte('\n')
			}
			stderrBuf.WriteString(line)
			stderrMu.Unlock()
			log.Printf("[Codex] stderr: %s", line)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[Codex] stderr scan error: %v", err)
		}
	}()

	emit := func(evt StreamEvent) {
		evt.SessionID = sessionID
		a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
			SessionID: sessionID,
			RequestID: requestID,
			Event:     evt,
		})
	}

	state := newCodexRunState()
	stdoutScanner := bufio.NewScanner(stdout)
	stdoutScanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		if err := state.handleJSONLine(line, emit); err != nil {
			stderrMu.Lock()
			if stderrBuf.Len() > 0 {
				stderrBuf.WriteByte('\n')
			}
			stderrBuf.WriteString(err.Error())
			stderrMu.Unlock()
			log.Printf("[Codex] event parse error: %v", err)
		}
	}
	stdoutErr := stdoutScanner.Err()
	waitErr := cmd.Wait()
	<-stderrDone

	if stdoutErr != nil {
		waitErr = fmt.Errorf("stdout scan: %v", stdoutErr)
	}

	stderrMu.Lock()
	stderrText := strings.TrimSpace(stderrBuf.String())
	stderrMu.Unlock()

	if waitErr != nil || ctx.Err() != nil || state.lastError != "" {
		errText := strings.TrimSpace(state.lastError)
		if errText == "" && waitErr != nil {
			errText = waitErr.Error()
		}
		if errText == "" && ctx.Err() != nil {
			errText = ctx.Err().Error()
		}
		if errText == "" && stderrText != "" {
			errText = stderrText
		}
		if errText == "" {
			errText = "codex exec failed"
		}
		a.completeSession(sessionID, "failed", "")
		a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
			SessionID: sessionID,
			RequestID: requestID,
			Event: StreamEvent{
				Type: "system",
				Text: "❌ Codex 会话失败: " + errText,
				Done: true,
			},
		})
		a.cleanupSessionRecord(sessionID)
		return taskResult{Status: "error"}, fmt.Errorf("%s", errText)
	}

	summary := state.summary()
	if summary == "" {
		summary = "Codex 执行完成"
	}
	if len(summary) > 3000 {
		summary = summary[:3000] + "\n..."
	}

	a.completeSession(sessionID, "completed", summary)
	a.cleanupSessionRecord(sessionID)
	a.sendStreamEvent(conn, callerAgentID, StreamEventPayload{
		SessionID: sessionID,
		RequestID: requestID,
		Event: StreamEvent{
			Type: "system",
			Text: "✅ Codex 会话完成",
			Done: true,
		},
	})

	return taskResult{
		Status:       "done",
		SessionID:    "",
		Summary:      summary,
		ProjectDir:   projectPath,
		FilesWritten: len(state.filesWritten),
		FilesEdited:  len(state.filesEdited),
		Model:        plan.Model,
	}, nil
}
