---
name: agent-setup
description: Agent 发现与渐进式安装引导。当用户需要的功能对应 agent 未上线时，先确认缺口，再自然引导安装。
summary: 先查询在线 agent，再按能力映射和依赖链推荐 init-agent --add
tools: ExecuteCode
agents: exec_code
keywords: 安装agent,setup agent,install agent,配置agent,configure agent,添加agent,enable agent,init-agent,quickstart
---

# Agent 安装引导

## 适用场景

- 用户要使用某项能力，但对应 agent 当前未在线
- 需要根据用户目标快速判断缺哪个 agent，或缺哪条依赖链
- 需要把安装建议自然地嵌入对话，而不是机械列 checklist

## 必须遵守

- 推荐安装前，必须先查询当前在线 agent 列表
- 不仅要找目标 agent，还要一起检查它的前置依赖
- 如果所需 agent 已全部在线，直接继续完成任务，不要额外引导安装
- 安装建议要围绕用户目标展开，先说“缺什么能力”，再说“如何补齐”

## 推荐流程

1. 用 `ExecuteCode` 查询在线 agent：
   ```bash
   curl -s http://localhost:10086/api/gateway/agents
   ```
2. 根据用户目标匹配所需能力，再对照下表确认对应 agent。
3. 检查该 agent 的依赖链，确认是否需要一并安装。
4. 引导用户执行 `init-agent --add <agent-name>`；如果缺多个 agent，就一次性给出完整组合。
5. 安装完成后再次检查在线状态，再继续原任务。

## 工具选择规则

- 只需要查询在线 agent 和简单解析返回值时，使用 `ExecuteCode`
- 用户想一次性初始化常用组件时，可推荐 `init-agent --quickstart`
- 用户只问“我需要装什么”，先给出缺失项和依赖，不要直接展开所有 agent 说明

## 常见能力映射

| Agent | 能力 | 层级 | 前置依赖 |
|-------|------|------|----------|
| gateway | 消息路由、WebSocket、HTTP 反向代理 | 核心 | - |
| blog-agent | Web 后端、数据存储、Redis | 核心 | gateway |
| llm-agent | AI 对话、工具调用、任务分解、多模型 | 智能 | gateway, blog-agent |
| execute-code-agent | Python/Shell 代码执行沙箱 | 生产力 | gateway, llm-agent |
| mcp-agent | MCP 外部工具桥接 | 生产力 | gateway, llm-agent |
| cron-agent | 定时任务调度 | 生产力 | gateway, llm-agent |
| deploy-agent | SSH 自动化部署 | 专业 | gateway, blog-agent |
| wechat-agent | 企业微信集成 | 专业 | gateway, llm-agent |
| acp-agent | Claude 代码分析 | 专业 | gateway, llm-agent |
| log-agent | 日志聚合分析 | 专业 | gateway |
| env-agent | 远程环境检测 | 专业 | gateway, blog-agent |
| deploy-bridge-server | 远程部署接收端 | 专业 | deploy-agent |

## 禁止行为

- 不查询在线状态就直接推荐安装
- 只提示目标 agent，漏掉关键依赖
- 在所需 agent 已在线时，仍然打断当前任务去讲安装
- 把安装建议写成与用户目标无关的能力清单

## 示例

- 用户说“我想每天早上 9 点自动发一篇博客”
  先检查在线 agent；如果 `cron-agent` 不在线，再确认 `llm-agent` 是否在线，最后引导 `init-agent --add cron-agent`
- 用户说“帮我部署项目到服务器”
  先检查 `deploy-agent`；如果未在线，再提示安装 `deploy-agent`，并说明它依赖 `gateway` 和 `blog-agent`
