package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

// ========================= 活跃 LLM 状态管理 =========================

// ActiveLLMState 线程安全的运行时 LLM 配置管理
type ActiveLLMState struct {
	mu       sync.RWMutex
	current  LLMConfig
	provider string // 当前 provider key
	modelKey string // 当前 model key（providers[provider].models 中的 key）
}

// NewActiveLLMState 从解析后的 LLMConfig 初始化活跃状态
func NewActiveLLMState(cfg LLMConfig) *ActiveLLMState {
	return &ActiveLLMState{
		current:  cfg,
		provider: cfg.Provider,
		modelKey: cfg.Model,
	}
}

// Get 返回当前活跃配置的拷贝（值类型，无并发风险）
func (a *ActiveLLMState) Get() LLMConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

// GetInfo 返回当前状态摘要
func (a *ActiveLLMState) GetInfo() (provider, modelKey, modelID string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.provider, a.modelKey, a.current.EffectiveModel()
}

// SwitchProvider 切换 provider（使用其第一个模型）
func (a *ActiveLLMState) SwitchProvider(name string, pc ProviderConfig) error {
	if len(pc.Models) == 0 {
		return fmt.Errorf("provider %q has no models", name)
	}
	// 取按字典序第一个模型作为默认
	var firstKey string
	for k := range pc.Models {
		if firstKey == "" || k < firstKey {
			firstKey = k
		}
	}
	mc := pc.Models[firstKey]
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = name
	a.modelKey = firstKey
	a.current = LLMConfig{
		Provider:    name,
		Model:       firstKey,
		APIKey:      pc.APIKey,
		BaseURL:     pc.BaseURL,
		ModelID:     mc.Model,
		MaxTokens:   mc.MaxTokens,
		Temperature: mc.Temperature,
	}
	log.Printf("[ActiveLLM] 切换 provider=%s model=%s(%s)", name, firstKey, mc.Model)
	return nil
}

// SwitchModel 切换指定模型（可跨 provider）
func (a *ActiveLLMState) SwitchModel(providerName, modelKey string, pc ProviderConfig, mc ModelConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = providerName
	a.modelKey = modelKey
	a.current = LLMConfig{
		Provider:    providerName,
		Model:       modelKey,
		APIKey:      pc.APIKey,
		BaseURL:     pc.BaseURL,
		ModelID:     mc.Model,
		MaxTokens:   mc.MaxTokens,
		Temperature: mc.Temperature,
	}
	log.Printf("[ActiveLLM] 切换模型 provider=%s model=%s(%s)", providerName, modelKey, mc.Model)
}

// ========================= 虚拟工具定义 =========================

var listProvidersTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "list_providers",
		Description: "列出当前可切换的 LLM 服务商及其模型列表。仅用于查看候选项和参数，不会修改当前模型。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
	},
}

var getCurrentModelTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_current_model",
		Description: "获取当前正在使用的服务商、模型 key 和实际 model_id。仅做只读查询，不会切换模型。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`),
	},
}

var switchProviderTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "switch_provider",
		Description: "将运行时 LLM 切换到指定服务商，并自动使用该服务商的默认模型。会影响后续对话与工具评估路径，仅在用户明确要求换服务商时使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {
					"type": "string",
					"description": "服务商名称（来自 list_providers 返回的 provider key）"
				}
			},
			"required": ["provider"]
		}`),
	},
}

var switchModelTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "switch_model",
		Description: "将运行时 LLM 切换到指定 provider 下的指定模型。会影响后续全部 LLM 请求；仅在用户明确要求切换模型时使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {
					"type": "string",
					"description": "服务商名称"
				},
				"model": {
					"type": "string",
					"description": "模型 key（来自 list_providers 返回的 model key）"
				}
			},
			"required": ["provider", "model"]
		}`),
	},
}

// ========================= Handler 方法 =========================

// handleListProviders 列出所有 provider 及其模型
func (b *Bridge) handleListProviders() string {
	var sb strings.Builder
	sb.WriteString("可用服务商及模型：\n")

	// 按名称排序
	names := make([]string, 0, len(b.cfg.Providers))
	for name := range b.cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		pc := b.cfg.Providers[name]
		sb.WriteString(fmt.Sprintf("\n[%s] %s\n", name, pc.BaseURL))
		// 模型按 key 排序
		mkeys := make([]string, 0, len(pc.Models))
		for k := range pc.Models {
			mkeys = append(mkeys, k)
		}
		sort.Strings(mkeys)
		for _, mk := range mkeys {
			mc := pc.Models[mk]
			sb.WriteString(fmt.Sprintf("  - %s → %s (max_tokens=%d, temperature=%.1f)\n",
				mk, mc.Model, mc.MaxTokens, mc.Temperature))
		}
	}
	return sb.String()
}

// handleGetCurrentModel 返回当前 provider + model
func (b *Bridge) handleGetCurrentModel() string {
	provider, modelKey, modelID := b.activeLLM.GetInfo()
	cfg := b.activeLLM.Get()
	return fmt.Sprintf("当前模型：\n  provider: %s\n  model: %s\n  model_id: %s\n  max_tokens: %d\n  temperature: %.2f",
		provider, modelKey, modelID, cfg.MaxTokens, cfg.Temperature)
}

// handleSwitchProvider 切换 provider
func (b *Bridge) handleSwitchProvider(provider string) (string, error) {
	pc, ok := b.cfg.Providers[provider]
	if !ok {
		available := make([]string, 0, len(b.cfg.Providers))
		for k := range b.cfg.Providers {
			available = append(available, k)
		}
		return "", fmt.Errorf("provider %q 不存在，可用: %v", provider, available)
	}
	if err := b.activeLLM.SwitchProvider(provider, pc); err != nil {
		return "", err
	}
	return b.handleGetCurrentModel(), nil
}

// handleSwitchModel 切换模型（可跨 provider）
func (b *Bridge) handleSwitchModel(modelKey, provider string) (string, error) {
	pc, ok := b.cfg.Providers[provider]
	if !ok {
		available := make([]string, 0, len(b.cfg.Providers))
		for k := range b.cfg.Providers {
			available = append(available, k)
		}
		return "", fmt.Errorf("provider %q 不存在，可用: %v", provider, available)
	}
	mc, ok := pc.Models[modelKey]
	if !ok {
		available := make([]string, 0, len(pc.Models))
		for k := range pc.Models {
			available = append(available, k)
		}
		return "", fmt.Errorf("model %q 不存在于 provider %q，可用: %v", modelKey, provider, available)
	}
	b.activeLLM.SwitchModel(provider, modelKey, pc, mc)
	return b.handleGetCurrentModel(), nil
}

// hasMultipleModels 判断是否有多个可选模型（决定是否注入切换工具）
func (b *Bridge) hasMultipleModels() bool {
	total := 0
	for _, pc := range b.cfg.Providers {
		total += len(pc.Models)
		if total > 1 {
			return true
		}
	}
	return false
}
