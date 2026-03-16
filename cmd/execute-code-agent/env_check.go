package main

import (
	"log"

	"agentbase"
)

// startEnvCheck 启动环境检测（在独立 goroutine 中运行，不阻塞 agent 启动）
func startEnvCheck(conn *Connection, envCfg *agentbase.EnvConfig) {
	checker := agentbase.NewEnvChecker(
		conn.AgentBase,
		conn.toolCatalog,
		conn.remoteCaller,
		envCfg,
		func(results []agentbase.EnvCheckResult) {
			applyEnvResults(conn, results)
		},
	)
	checker.Run()
}

// applyEnvResults 应用环境检测结果，更新配置
func applyEnvResults(conn *Connection, results []agentbase.EnvCheckResult) {
	for _, r := range results {
		// Python 检测成功 → 更新路径
		if r.Software == "python" && r.Success && r.Path != "" {
			oldPath := conn.cfg.PythonPath
			conn.cfg.PythonPath = r.Path
			conn.executor.cfg.PythonPath = r.Path
			log.Printf("[EnvCheck] Python 路径已更新: %s → %s", oldPath, r.Path)
		}
	}
}
