package main

import (
	"encoding/json"
	"strings"
)

func unwrapInboundCommand(payload inboundNotify) (userID string, content string) {
	raw := strings.TrimSpace(payload.Content)
	_, raw = stripDelegationPrefix(raw)
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, "APP_MESSAGE_JSON:") {
		body := strings.TrimSpace(strings.TrimPrefix(raw, "APP_MESSAGE_JSON:"))
		var env inboundAppEnvelope
		if err := json.Unmarshal([]byte(body), &env); err == nil {
			return firstNonEmpty(env.UserID, payload.To), strings.TrimSpace(env.Content)
		}
	}
	return payload.To, raw
}

func stripDelegationPrefix(content string) (token string, rest string) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "[delegation:") {
		return "", content
	}
	endIdx := strings.Index(content, "]")
	if endIdx <= 12 {
		return "", content
	}
	return content[12:endIdx], strings.TrimSpace(content[endIdx+1:])
}

func normalizeCodegenCommand(message string) string {
	message = strings.TrimSpace(message)
	if message == "/cg" {
		return "cg"
	}
	if strings.HasPrefix(message, "/cg ") {
		return "cg " + strings.TrimSpace(strings.TrimPrefix(message, "/cg "))
	}
	return message
}

func isCGCommand(content string) bool {
	content = strings.TrimSpace(content)
	return content == "cg" || strings.HasPrefix(content, "cg ")
}

func parseProjectAgent(s string) (project, agentName string) {
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func normalizeTool(tool string) string {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "oc", "opencode":
		return "opencode"
	case "cc", "claude", "claudecode":
		return "claudecode"
	default:
		return strings.ToLower(strings.TrimSpace(tool))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func getCodegenHelpText() string {
	return "💻 CodeGen 编码助手命令\n\n" +
		"cg list — 列出所有项目\n" +
		"cg create <名称[@agent]> — 创建项目\n" +
		"cg start <项目[@agent]> <需求> — 启动编码\n" +
		"cg start <项目[@agent]> #<模型> <需求> — 指定模型\n" +
		"cg start <项目[@agent]> @oc <需求> — 用OpenCode\n" +
		"cg start <项目[@agent]> !deploy <需求> — 编码后自动部署\n" +
		"cg deploy <项目[@agent]> [#目标] [!pack] [--version/-v 版本] [--desc/-d 描述] — 部署已配置项目\n" +
		"cg deploy list — 列出 deploy 项目\n" +
		"cg deploy adhoc <项目> <目录> <ssh_host> — 一次性部署\n" +
		"cg deploy pipelines — 列出可用编排\n" +
		"cg deploy pipeline <编排名[@agent]> — 执行部署编排\n" +
		"cg deploy read <项目[@agent]> <路径> — 读取部署项目文件\n" +
		"cg deploy write <项目[@agent]> <路径> <内容> — 写入部署项目文件\n" +
		"cg deploy exec <项目[@agent]> <命令> — 执行部署项目命令\n" +
		"cg deploy env <命令> — 执行环境命令\n" +
		"cg deploy agent-status <agent_id> — 查询 agent 状态\n" +
		"cg deploy agent-stop <agent_id> [原因] — 关闭 agent\n" +
		"cg send <消息> — 追加指令\n" +
		"cg status — 查看进度\n" +
		"cg stop — 停止编码\n" +
		"cg models — 查看可用模型配置\n" +
		"cg tools — 查看可用编码工具\n" +
		"cg agents — 查看在线agent\n\n" +
		"@agent 语法: 多agent同名项目时用 项目@agent 指定目标\n" +
		"工具别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode\n" +
		"示例: cg start myapp@win #sonnet !deploy 写个HTTP服务"
}
