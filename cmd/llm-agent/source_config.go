package main

// SourceLLMConfig 来源渠道的 LLM 配置
type SourceLLMConfig struct {
	Source    string    `json:"source"`               // "wechat" | "web" | "api"
	LLM      LLMConfig `json:"llm"`                  // 该渠道使用的 LLM
	Fallbacks []LLMConfig `json:"fallbacks,omitempty"` // 该渠道的降级链
}
