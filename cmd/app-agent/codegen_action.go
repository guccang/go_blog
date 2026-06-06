package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const codegenActionProtocolVersion = 1

// codegenActionRequest 是 Flutter 与 app-agent 之间的稳定适配协议。
// app-agent 在边界内将其转换为当前 cmd-agent 支持的命令，避免客户端依赖命令语法。
type codegenActionRequest struct {
	Version      int      `json:"version"`
	Kind         string   `json:"kind"`
	UserID       string   `json:"user_id"`
	HistoryID    string   `json:"history_id,omitempty"`
	Project      string   `json:"project"`
	Agent        string   `json:"agent,omitempty"`
	Tool         string   `json:"tool,omitempty"`
	Settings     string   `json:"settings,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
	Resume       bool     `json:"resume,omitempty"`
	AutoDeploy   bool     `json:"auto_deploy,omitempty"`
	DebugID      string   `json:"debug_id,omitempty"`
	DebugPath    string   `json:"debug_path,omitempty"`
	DeployTarget string   `json:"deploy_target,omitempty"`
	PackOnly     bool     `json:"pack_only,omitempty"`
	ExtraArgs    []string `json:"extra_args,omitempty"`
}

func (h *Handler) HandleCodegenAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req codegenActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" || !h.validateAppSession(r, req.UserID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	command, err := buildCodegenActionCommand(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	meta := map[string]any{
		"codegen_protocol_version": codegenActionProtocolVersion,
		"codegen_action_kind":      strings.ToLower(strings.TrimSpace(req.Kind)),
	}
	if historyID := strings.TrimSpace(req.HistoryID); historyID != "" {
		meta["codegen_history_id"] = historyID
	}
	msg := &AppMessage{
		UserID:          req.UserID,
		Content:         command,
		MessageType:     "text",
		Meta:            meta,
		DelegationToken: h.bridge.GetDelegationToken(req.UserID),
	}
	go h.bridge.HandleAppMessage(msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"version":  codegenActionProtocolVersion,
		"accepted": true,
	})
}

func buildCodegenActionCommand(req codegenActionRequest) (string, error) {
	if req.Version != 0 && req.Version != codegenActionProtocolVersion {
		return "", fmt.Errorf("unsupported codegen action version: %d", req.Version)
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	project := strings.TrimSpace(req.Project)
	if project == "" {
		return "", fmt.Errorf("project is required")
	}
	if err := validateCodegenActionToken("project", project); err != nil {
		return "", err
	}
	if agent := strings.TrimSpace(req.Agent); agent != "" {
		if err := validateCodegenActionToken("agent", agent); err != nil {
			return "", err
		}
		project += "@" + agent
	}

	switch kind {
	case "start", "debug":
		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			return "", fmt.Errorf("prompt is required")
		}
		parts := []string{"/cg", kind, project}
		if tool := strings.TrimSpace(req.Tool); tool != "" {
			if err := validateCodegenActionToken("tool", tool); err != nil {
				return "", err
			}
			parts = append(parts, "@"+tool)
		}
		if settings := strings.TrimSpace(req.Settings); settings != "" {
			if err := validateCodegenActionToken("settings", settings); err != nil {
				return "", err
			}
			parts = append(parts, "--settings", settings)
		}
		if req.Resume {
			parts = append(parts, "!resume")
		}
		if kind == "debug" {
			if debugID := strings.TrimSpace(req.DebugID); debugID != "" {
				if err := validateCodegenActionToken("debug_id", debugID); err != nil {
					return "", err
				}
				parts = append(parts, "--debug-id", debugID)
			}
			if debugPath := strings.TrimSpace(req.DebugPath); debugPath != "" {
				if err := validateCodegenActionToken("debug_path", debugPath); err != nil {
					return "", err
				}
				parts = append(parts, "--debug-path", debugPath)
			}
		} else if req.AutoDeploy {
			parts = append(parts, "!deploy")
		}
		parts = append(parts, prompt)
		return strings.Join(parts, " "), nil
	case "deploy":
		parts := []string{"/cg", "deploy", project}
		if target := strings.TrimSpace(req.DeployTarget); target != "" {
			if err := validateCodegenActionToken("deploy_target", target); err != nil {
				return "", err
			}
			parts = append(parts, "#"+target)
		}
		if req.PackOnly {
			parts = append(parts, "!pack")
		}
		for _, arg := range req.ExtraArgs {
			if arg = strings.TrimSpace(arg); arg != "" {
				parts = append(parts, arg)
			}
		}
		return strings.Join(parts, " "), nil
	default:
		return "", fmt.Errorf("unsupported codegen action kind: %s", kind)
	}
}

func validateCodegenActionToken(name, value string) error {
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s must not contain whitespace", name)
	}
	return nil
}
