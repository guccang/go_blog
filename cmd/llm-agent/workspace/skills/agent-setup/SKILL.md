---
name: agent-setup
description: Agent 发现与渐进式安装引导。当用户需要的功能对应的 agent 未上线时，自然引导安装。
summary: 查询 gateway /api/gateway/agents 获取在线 agent，推荐缺失 agent，引导 init-agent --add 安装
tools: ExecuteCode
agents: exec_code
keywords: 安装agent,setup agent,install agent,配置agent,configure agent,添加agent,enable agent,init-agent,quickstart
---

# Agent 安装引导

当用户需要某功能但对应 agent 未上线时，使用此技能引导安装。

## 可用 Agent 及能力对照表

| Agent | 能力 | 层级 | 前置依赖 |
|-------|------|------|----------|
| gateway | 消息路由、WebSocket、HTTP 反向代理 | 核心 | - |
| blog-agent | Web 后端、数据存储、Redis | 核心 | gateway |
| llm-agent | AI 对话、工具调用、任务分解、多模型 | 智能 | gateway, blog-agent |
| execute-code-agent | Python/Shell 代码执行沙箱 | 生产力 | gateway, llm-agent |
| mcp-agent | MCP 外部工具桥接 | 生产力 | gateway, llm-agent |
| corn-agent | 定时任务调度 | 生产力 | gateway, llm-agent |
| deploy-agent | SSH 自动化部署 | 专业 | gateway, blog-agent |
| codegen-agent | Claude 代码生成 | 专业 | gateway, llm-agent |
| wechat-agent | 企业微信集成 | 专业 | gateway, llm-agent |
| acp-agent | Claude 代码分析 | 专业 | gateway, llm-agent |
| log-agent | 日志聚合分析 | 专业 | gateway |
| env-agent | 远程环境检测 | 专业 | gateway, blog-agent |
| deploy-bridge-server | 远程部署接收端 | 专业 | deploy-agent |

## 执行步骤

1. **检查在线 agent**
   用 ExecuteCode 执行:
   ```bash
   curl -s http://localhost:10086/api/gateway/agents
   ```
   获取当前在线的 agent 列表。

2. **对照表确定缺失**
   根据用户需求匹配上表中的 agent，确认哪些未在线。

3. **检查依赖链**
   对照"前置依赖"列，确认所有前置 agent 是否在线。如有未上线的依赖，一并提示安装。

4. **引导安装**
   告知用户运行以下命令安装缺失 agent：
   ```bash
   # 安装单个 agent（自动解析依赖）
   init-agent --add <agent-name>

   # 安装多个 agent
   init-agent --add <agent1>,<agent2>
   ```
   说明安装过程中需要准备的配置项（如 API Key、端口等）。

5. **安装后确认**
   安装并启动后，再次检查确认 agent 上线。

## 交互示例

**用户**: "我想每天早上 9 点自动发一篇博客"
**回复思路**:
- 检查在线 agent → corn-agent 未上线
- 定时任务需要 corn-agent，它依赖 llm-agent
- 检查 llm-agent 是否在线
  - 在线 → 只需安装 corn-agent
  - 不在线 → 需要安装 llm-agent 和 corn-agent
- 引导: `init-agent --add corn-agent`
- 安装后启动 corn-agent，然后帮用户设置定时任务

**用户**: "帮我部署项目到服务器"
**回复思路**:
- 检查 deploy-agent 是否在线
- 不在线 → 引导: `init-agent --add deploy-agent`
- deploy-agent 依赖 gateway 和 blog-agent（通常已在线）

## 注意事项

- 引导语气要自然，不要像 checklist，而是融入对话
- 先说明用户想做的事需要什么能力，再说怎么获得
- 如果所有需要的 agent 都在线，直接帮用户完成任务，不要提及安装
- 快速启动核心 agent: `init-agent --quickstart`
- 查看推荐: `init-agent --recommend "你想做的事"`
