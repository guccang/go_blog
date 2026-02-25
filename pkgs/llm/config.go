package llm

import (
	"config"
	"encoding/json"
	"fmt"
	log "mylog"
	"os"
	"sync"
)

// LLMConfig LLM Configuration
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

// ModelProvider 模型提供者配置
type ModelProvider struct {
	Name    string  `json:"name"`
	APIKey  string  `json:"api_key"`
	BaseURL string  `json:"base_url"`
	Model   string  `json:"model"`
	Temp    float64 `json:"temperature"`
}

// Global configuration
var (
	llmConfig         = LLMConfig{}
	fallbackProviders []ModelProvider
	activeProvider    string // 当前活跃的 provider 名称
	configMu          sync.RWMutex
)

// Maximum number of tools that can be selected
var maxSelectedTools = 70

// Context overflow prevention constants
const (
	MaxToolResultChars    = 4000   // per tool result passed back to the model
	MaxToolArgumentsChars = 4000   // per tool-call arguments embedded in assistant message
	MaxMessageChars       = 8000   // per message content clamp
	MaxMessagesToSend     = 60     // overall message count cap
	MaxTotalCharsBudget   = 200000 // rough total-char budget for all messages
)

// GetConfig returns the current LLM configuration
func GetConfig() *LLMConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return &llmConfig
}

// GetMaxSelectedTools returns the maximum number of tools that can be selected
func GetMaxSelectedTools() int {
	return maxSelectedTools
}

// getConfigWithDefault retrieves config value, uses default if empty
func getConfigWithDefault(key, defaultValue string) string {
	value := config.GetConfigWithAccount(config.GetAdminAccount(), key)
	if value == "" {
		return defaultValue
	}
	return value
}

// InitConfig initializes the LLM configuration
func InitConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	llmConfig = LLMConfig{
		APIKey:      getConfigWithDefault("deepseek_api_key", os.Getenv("OPENAI_API_KEY")),
		BaseURL:     getConfigWithDefault("deepseek_api_url", "https://api.deepseek.com/v1/chat/completions"),
		Model:       "deepseek-chat",
		Temperature: 0.3,
	}
	activeProvider = "deepseek"

	// 加载 fallback 配置
	loadFallbackProviders()

	log.InfoF(log.ModuleLLM, "Init config: primary=%s, fallbacks=%d", activeProvider, len(fallbackProviders))
}

// loadFallbackProviders 从 sys_conf.md 加载备用模型配置
func loadFallbackProviders() {
	fallbackProviders = make([]ModelProvider, 0)

	// 尝试读取 JSON 格式的 fallback_models 配置
	fallbackJSON := config.GetConfigWithAccount(config.GetAdminAccount(), "llm_fallback_models")
	if fallbackJSON != "" {
		var providers []ModelProvider
		if err := json.Unmarshal([]byte(fallbackJSON), &providers); err == nil {
			fallbackProviders = providers
			log.MessageF(log.ModuleLLM, "Loaded %d fallback providers from config", len(providers))
			return
		}
	}

	// 单独字段方式读取备用模型
	openaiKey := config.GetConfigWithAccount(config.GetAdminAccount(), "openai_api_key")
	openaiURL := config.GetConfigWithAccount(config.GetAdminAccount(), "openai_api_url")
	if openaiKey != "" {
		if openaiURL == "" {
			openaiURL = "https://api.openai.com/v1/chat/completions"
		}
		fallbackProviders = append(fallbackProviders, ModelProvider{
			Name:    "openai",
			APIKey:  openaiKey,
			BaseURL: openaiURL,
			Model:   "gpt-4o-mini",
			Temp:    0.3,
		})
	}

	// 读取其他备用模型
	qwenKey := config.GetConfigWithAccount(config.GetAdminAccount(), "qwen_api_key")
	qwenURL := config.GetConfigWithAccount(config.GetAdminAccount(), "qwen_api_url")
	if qwenKey != "" {
		if qwenURL == "" {
			qwenURL = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
		}
		fallbackProviders = append(fallbackProviders, ModelProvider{
			Name:    "qwen",
			APIKey:  qwenKey,
			BaseURL: qwenURL,
			Model:   "qwen-plus",
			Temp:    0.3,
		})
	}
}

// GetFallbackProviders 获取备用模型列表
func GetFallbackProviders() []ModelProvider {
	configMu.RLock()
	defer configMu.RUnlock()
	result := make([]ModelProvider, len(fallbackProviders))
	copy(result, fallbackProviders)
	return result
}

// GetActiveProvider 获取当前活跃的 provider
func GetActiveProvider() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return activeProvider
}

// SwitchModel 切换到指定模型
func SwitchModel(providerName string) error {
	configMu.Lock()
	defer configMu.Unlock()

	// 切回主模型
	if providerName == "deepseek" || providerName == "primary" {
		llmConfig = LLMConfig{
			APIKey:      getConfigWithDefault("deepseek_api_key", os.Getenv("OPENAI_API_KEY")),
			BaseURL:     getConfigWithDefault("deepseek_api_url", "https://api.deepseek.com/v1/chat/completions"),
			Model:       "deepseek-chat",
			Temperature: 0.3,
		}
		activeProvider = "deepseek"
		log.MessageF(log.ModuleLLM, "Switched to primary model: deepseek")
		return nil
	}

	// 在 fallback 列表中查找
	for _, p := range fallbackProviders {
		if p.Name == providerName {
			llmConfig = LLMConfig{
				APIKey:      p.APIKey,
				BaseURL:     p.BaseURL,
				Model:       p.Model,
				Temperature: p.Temp,
			}
			activeProvider = p.Name
			log.MessageF(log.ModuleLLM, "Switched to model: %s (%s)", p.Name, p.Model)
			return nil
		}
	}

	return fmt.Errorf("provider not found: %s. Available: deepseek, %s", providerName, listProviderNames())
}

// FallbackToNext 切换到下一个可用的备用模型
func FallbackToNext() bool {
	configMu.Lock()
	defer configMu.Unlock()

	if len(fallbackProviders) == 0 {
		return false
	}

	// 找到当前 provider 的下一个
	currentIdx := -1
	for i, p := range fallbackProviders {
		if p.Name == activeProvider {
			currentIdx = i
			break
		}
	}

	// 如果当前是主模型，切到第一个 fallback
	if currentIdx == -1 {
		p := fallbackProviders[0]
		llmConfig = LLMConfig{
			APIKey:      p.APIKey,
			BaseURL:     p.BaseURL,
			Model:       p.Model,
			Temperature: p.Temp,
		}
		activeProvider = p.Name
		log.MessageF(log.ModuleLLM, "Fallback to: %s (%s)", p.Name, p.Model)
		return true
	}

	// 如果还有下一个 fallback
	nextIdx := currentIdx + 1
	if nextIdx < len(fallbackProviders) {
		p := fallbackProviders[nextIdx]
		llmConfig = LLMConfig{
			APIKey:      p.APIKey,
			BaseURL:     p.BaseURL,
			Model:       p.Model,
			Temperature: p.Temp,
		}
		activeProvider = p.Name
		log.MessageF(log.ModuleLLM, "Fallback to: %s (%s)", p.Name, p.Model)
		return true
	}

	return false
}

// GetModelInfo 获取当前模型信息
func GetModelInfo() map[string]interface{} {
	configMu.RLock()
	defer configMu.RUnlock()

	providers := make([]map[string]string, 0)
	providers = append(providers, map[string]string{
		"name":  "deepseek",
		"model": "deepseek-chat",
		"type":  "primary",
	})
	for _, p := range fallbackProviders {
		providers = append(providers, map[string]string{
			"name":  p.Name,
			"model": p.Model,
			"type":  "fallback",
		})
	}

	return map[string]interface{}{
		"active_provider": activeProvider,
		"active_model":    llmConfig.Model,
		"base_url":        llmConfig.BaseURL,
		"providers":       providers,
	}
}

func listProviderNames() string {
	names := ""
	for i, p := range fallbackProviders {
		if i > 0 {
			names += ", "
		}
		names += p.Name
	}
	return names
}
