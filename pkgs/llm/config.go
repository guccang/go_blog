package llm

import (
	"config"
	log "mylog"
	"os"
)

// LLMConfig LLM Configuration
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

// Global configuration instance
var llmConfig = LLMConfig{}

// Maximum number of tools that can be selected
var maxSelectedTools = 50

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
	llmConfig = LLMConfig{
		APIKey:      getConfigWithDefault("deepseek_api_key", os.Getenv("OPENAI_API_KEY")),
		BaseURL:     getConfigWithDefault("deepseek_api_url", "https://api.deepseek.com/v1/chat/completions"),
		Model:       "deepseek-chat",
		Temperature: 0.3,
	}
	log.InfoF(log.ModuleLLM, "Init config %v", llmConfig)
}
