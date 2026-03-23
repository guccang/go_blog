package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// ToolHandler 统一工具处理接口
// 所有工具（内置、虚拟、远程）都实现此签名
type ToolHandler func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error)

// registerTool 注册工具处理器和名称映射（需在 catalogMu 写锁外调用，内部加锁）
func (b *Bridge) registerTool(canonicalName string, handler ToolHandler) {
	b.catalogMu.Lock()
	defer b.catalogMu.Unlock()
	b.registerToolLocked(canonicalName, handler)
}

// registerToolLocked 注册工具处理器和名称映射（需持有 catalogMu 写锁）
func (b *Bridge) registerToolLocked(canonicalName string, handler ToolHandler) {
	b.toolHandlers[canonicalName] = handler

	// 注册名称变体映射
	sanitized := sanitizeToolName(canonicalName)
	b.toolNameMap[sanitized] = canonicalName    // llm-agent_Bash → llm-agent.Bash
	b.toolNameMap[canonicalName] = canonicalName // llm-agent.Bash → llm-agent.Bash

	// 裸名映射（如 Bash）—— 仅在无冲突时
	if dot := strings.LastIndex(canonicalName, "."); dot >= 0 {
		bare := canonicalName[dot+1:]
		if _, exists := b.toolNameMap[bare]; !exists {
			b.toolNameMap[bare] = canonicalName
		}
	}
}

// resolveToolName 将任意格式的工具名解析为规范名（canonical name）
// 支持 sanitized（下划线）、original（点号）和裸名三种格式
func (b *Bridge) resolveToolName(name string) string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	if canonical, ok := b.toolNameMap[name]; ok {
		return canonical
	}
	return name
}

// DispatchTool 统一工具调度入口
// 所有外部调用（CallTool/CallToolCtx/CallToolCtxWithProgress）最终汇聚于此
func (b *Bridge) DispatchTool(ctx context.Context, toolName string, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	canonical := b.resolveToolName(toolName)

	b.catalogMu.RLock()
	handler, ok := b.toolHandlers[canonical]
	b.catalogMu.RUnlock()

	if ok {
		return handler(ctx, args, sink)
	}

	return nil, fmt.Errorf("tool %s not found (resolved: %s)", toolName, canonical)
}

// registerBuiltinTools 注册所有内置工具（Bash、set_persona、set_rule）
func (b *Bridge) registerBuiltinTools() {
	// Bash 工具
	if b.bashManager != nil {
		canonicalName := b.cfg.AgentID + ".Bash"
		mgr := b.bashManager
		b.registerTool(canonicalName, func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			var argsMap map[string]interface{}
			if err := json.Unmarshal(args, &argsMap); err != nil {
				return &ToolCallResult{Result: "参数解析失败: " + err.Error(), AgentID: "builtin"}, nil
			}
			command, _ := argsMap["command"].(string)
			if command == "" {
				return &ToolCallResult{Result: "错误: command 参数不能为空", AgentID: "builtin"}, nil
			}
			workDir, _ := argsMap["work_dir"].(string)
			output, err := mgr.Exec(command, workDir)
			if err != nil {
				if output != "" {
					return &ToolCallResult{Result: fmt.Sprintf("%s\n[错误] %v", output, err), AgentID: "builtin"}, nil
				}
				return &ToolCallResult{Result: fmt.Sprintf("[错误] %v", err), AgentID: "builtin"}, nil
			}
			if output == "" {
				output = "(无输出)"
			}
			return &ToolCallResult{Result: output, AgentID: "builtin"}, nil
		})
		log.Printf("[Bridge] 注册内置工具: %s", canonicalName)
	}

	// set_persona 工具
	if b.persona != nil {
		b.registerTool("set_persona", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			reply, ok := b.persona.HandleSetPersona(string(args))
			log.Printf("[ToolHandler] set_persona: success=%v result=%s", ok, reply)
			return &ToolCallResult{Result: reply, AgentID: "builtin"}, nil
		})
	}

	// set_rule 工具
	if b.memoryMgr != nil {
		b.registerTool("set_rule", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			reply, ok := b.memoryMgr.HandleSetRule(string(args))
			log.Printf("[ToolHandler] set_rule: success=%v result=%s", ok, reply)
			return &ToolCallResult{Result: reply, AgentID: "builtin"}, nil
		})
	}

	// 模型切换工具（4 个）
	b.registerTool("list_providers", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		result := b.handleListProviders()
		return &ToolCallResult{Result: result, AgentID: "builtin"}, nil
	})
	b.registerTool("get_current_model", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		result := b.handleGetCurrentModel()
		return &ToolCallResult{Result: result, AgentID: "builtin"}, nil
	})
	b.registerTool("switch_provider", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		var params struct {
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return &ToolCallResult{Result: "参数解析失败: " + err.Error(), AgentID: "builtin"}, nil
		}
		result, err := b.handleSwitchProvider(params.Provider)
		if err != nil {
			return &ToolCallResult{Result: "切换失败: " + err.Error(), AgentID: "builtin"}, nil
		}
		return &ToolCallResult{Result: result, AgentID: "builtin"}, nil
	})
	b.registerTool("switch_model", func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		var params struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return &ToolCallResult{Result: "参数解析失败: " + err.Error(), AgentID: "builtin"}, nil
		}
		result, err := b.handleSwitchModel(params.Model, params.Provider)
		if err != nil {
			return &ToolCallResult{Result: "切换失败: " + err.Error(), AgentID: "builtin"}, nil
		}
		return &ToolCallResult{Result: result, AgentID: "builtin"}, nil
	})
}

// registerRemoteTool 注册远程 agent 工具的 handler
func (b *Bridge) registerRemoteToolLocked(canonicalName, agentID string) {
	capturedAgent := agentID
	capturedName := canonicalName
	b.registerToolLocked(canonicalName, func(ctx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		return b.callRemoteAgent(ctx, capturedName, capturedAgent, args, sink)
	})
}
