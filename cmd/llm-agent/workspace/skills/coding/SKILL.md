---
name: coding
description: 编码开发技能。当用户需要编写代码、创建项目、修复bug、重构、开发新功能时使用此技能。
summary: 纯编码任务原文直传 AcpStartSession，后续修复再用 AcpSendMessage
tools: AcpStartSession,AcpSendMessage,AcpGetStatus,AcpStopSession,AcpAnalyzeProject,AcpListProjects
agents: acp
keywords: 编码,代码,开发,编写,code,coding,项目,写一个,实现,功能
---

# 编码开发

## 适用场景

本规则适用于**纯编码任务**或**已拆解后的编码子任务**。

## 必须遵守

**纯编码任务**（用户消息只包含编码需求）：将用户原始消息原文直接作为 prompt 传给 AcpStartSession，**严禁修改、缩写、翻译、重新措辞或添加额外内容**。

**拆解后的编码子任务**（用户消息包含编码+部署等多步骤）：prompt 只传编码相关的需求描述，**剥离部署、通知等其他步骤的指令**。例如用户说"编码xx然后部署到yy"，prompt 只传"编码xx"部分。

编码 agent（Claude Code）具备完整的理解和执行能力，不需要预处理。用户怎么说就怎么传，不得"加料"。

## 推荐流程

1. 确认这是纯编码任务还是拆解后的编码子任务。
2. 选择描述性 `project` 名称；不存在时由 `AcpStartSession` 自动创建。
3. 调用 `AcpStartSession`，将 prompt 按上面的规则原文直传或只传编码部分。
4. 如果部署阶段暴露编译错误，再通过 `AcpSendMessage` 让编码 agent 在原会话中修复。
5. 只有工具明确返回“进行中”状态时，才补 `AcpGetStatus` 查询。

## 工具选择规则

- `AcpStartSession`：启动新的编码会话，创建或修改项目都优先使用它
- `AcpSendMessage`：只用于续接已有编码会话，例如修复编译错误、补充变更
- `AcpGetStatus`：只在工具返回 `in_progress` 时再查状态
- `AcpAnalyzeProject`：用于代码审查、架构分析、性能或安全分析
- `AcpListProjects`：仅当必须从现有项目列表中做匹配时使用，不能替代编码会话本身

## 项目命名规范

`project` 参数必须使用描述性名称（如 `helloworld-web`、`todo-api`），**严禁使用 account 账号名作为项目名,严禁使用中文作为项目名称**。

命名规则：提取核心功能关键词，小写+连字符格式。

## 禁止行为

- 把用户消息改写成更详细的技术方案再传入
- 把同一编码需求拆成多次 `AcpStartSession`
- 使用账号名或中文作为项目名
- 用 `ExecuteCode` 直接改写源代码文件，绕过编码 agent 的会话上下文
- 在同步工具已返回后，再额外制造“等待完成”子任务

## 示例

| 用户输入 | prompt 参数 | 说明 |
|----------|------------|------|
| "写一个helloworld网页" | "写一个helloworld网页" | 原文直传 |
| "编码 go语言写一个网页" | "编码 go语言写一个网页" | 保留原始前缀 |
| "重构登录模块，把密码改成bcrypt加密" | "重构登录模块，把密码改成bcrypt加密" | 原文直传 |
