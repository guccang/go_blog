package agentbase

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ========================= env.json 配置 =========================

// EnvConfig env.json 配置
type EnvConfig struct {
	GatewayHTTP  string           `json:"gateway_http,omitempty"`
	Requirements []EnvRequirement `json:"requirements"`
}

// EnvRequirement 环境检测需求
type EnvRequirement struct {
	Software   string `json:"software"`
	MinVersion string `json:"min_version,omitempty"`
}

// EnvCheckResult env-agent EnvSetup 返回的单项结果
type EnvCheckResult struct {
	Software string `json:"software"`
	Success  bool   `json:"success"`
	Version  string `json:"version,omitempty"`
	Path     string `json:"path,omitempty"`
	Method   string `json:"method,omitempty"`
	Error    string `json:"error,omitempty"`
}

// envSetupResponse EnvSetup 工具返回的完整结果
type envSetupResponse struct {
	Results []EnvCheckResult `json:"results"`
}

// LoadEnvConfig 从指定目录加载 env.json
// 不存在则返回 nil, nil（不报错）
func LoadEnvConfig(configDir string) (*EnvConfig, error) {
	path := filepath.Join(configDir, "env.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg EnvConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ========================= EnvChecker =========================

// EnvChecker 启动环境检测器
type EnvChecker struct {
	ab       *AgentBase
	catalog  *ToolCatalog
	caller   *RemoteCaller
	envCfg   *EnvConfig
	onResult func([]EnvCheckResult) // 各 agent 自定义回调（可为 nil）
}

// NewEnvChecker 创建环境检测器
func NewEnvChecker(ab *AgentBase, catalog *ToolCatalog, caller *RemoteCaller, envCfg *EnvConfig, onResult func([]EnvCheckResult)) *EnvChecker {
	return &EnvChecker{
		ab:       ab,
		catalog:  catalog,
		caller:   caller,
		envCfg:   envCfg,
		onResult: onResult,
	}
}

// Run 执行环境检测（在 goroutine 中调用，不阻塞 agent 启动）
func (ec *EnvChecker) Run() {
	// 无需求则跳过
	if len(ec.envCfg.Requirements) == 0 {
		log.Println("[EnvCheck] 无环境检测需求，跳过")
		return
	}

	log.Printf("[EnvCheck] 待检测: %d 项", len(ec.envCfg.Requirements))

	// 1. 等待 gateway 连接建立
	if !ec.waitForConnection(60 * time.Second) {
		log.Println("[EnvCheck] gateway 连接超时，跳过环境检测")
		return
	}
	log.Println("[EnvCheck] gateway 连接已建立")

	// 2. 等待 env-agent 上线（EnvSetup 工具可用）
	if !ec.waitForEnvAgent(30 * time.Second) {
		log.Println("[EnvCheck] env-agent 未在线，跳过环境检测")
		return
	}
	log.Println("[EnvCheck] env-agent 已在线")

	// 3. 调用 EnvSetup
	log.Println("[EnvCheck] 开始环境检测")
	results, err := ec.callEnvSetup()
	if err != nil {
		log.Printf("[EnvCheck] EnvSetup 调用失败: %v，跳过", err)
		return
	}

	// 4. 输出结果日志
	for _, r := range results {
		if r.Success {
			log.Printf("[EnvCheck] %s: OK version=%s path=%s", r.Software, r.Version, r.Path)
		} else {
			log.Printf("[EnvCheck] %s: FAILED error=%s", r.Software, r.Error)
		}
	}

	// 5. 调用回调
	if ec.onResult != nil {
		ec.onResult(results)
	}
}

// waitForConnection 轮询等待 gateway 连接建立
func (ec *EnvChecker) waitForConnection(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ec.ab.IsConnected() {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// waitForEnvAgent 轮询等待 EnvSetup 工具出现在 toolCatalog 中
func (ec *EnvChecker) waitForEnvAgent(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, ok := ec.catalog.GetAgentID("EnvSetup"); ok {
			return true
		}
		// 主动刷新一次工具目录
		ec.catalog.Discover(ec.ab.AgentID)
		time.Sleep(3 * time.Second)
	}
	return false
}

// callEnvSetup 调用 env-agent 的 EnvSetup 工具
func (ec *EnvChecker) callEnvSetup() ([]EnvCheckResult, error) {
	// 构建 requirements 参数
	type requirement struct {
		Software   string `json:"software"`
		MinVersion string `json:"min_version,omitempty"`
	}
	reqs := make([]requirement, len(ec.envCfg.Requirements))
	for i, r := range ec.envCfg.Requirements {
		reqs[i] = requirement{Software: r.Software, MinVersion: r.MinVersion}
	}

	args, _ := json.Marshal(map[string]interface{}{
		"target_agent": ec.ab.AgentName,
		"requirements": reqs,
	})

	result, _, err := ec.caller.CallToolWithRetry("EnvSetup", args, 120*time.Second)
	if err != nil {
		return nil, err
	}

	// 解析返回结果
	var resp envSetupResponse
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		return nil, err
	}

	return resp.Results, nil
}
