---
name: coding
description: 编码开发技能。当用户需要编写代码、创建项目、修复bug、重构、开发新功能时使用此技能。
summary: CodegenStartSession 传入用户原文，严禁修改
tools: CodegenStartSession,CodegenSendMessage,CodegenGetStatus,CodegenStopSession,CodegenCreateProject
keywords: 编码,代码,开发,编写,code,coding,项目,写一个,实现,功能
---

# 编码开发

## 适用范围

本规则适用于**纯编码任务**或**已拆解后的编码子任务**。

如果用户消息同时包含编码和其他操作（如部署、查询），必须先通过 plan_and_execute 拆解，再按本规则处理编码子任务。详见 TASK_GUIDE「多技能组合」规则。

## 消息传递规则

将用户的原始消息原文直接作为 prompt 传给 CodegenStartSession，**严禁修改、缩写、翻译、重新措辞或添加额外内容**。

编码 agent（Claude Code）具备完整的理解和执行能力，不需要预处理。用户怎么说就怎么传，不得"加料"。

**示例：**

| 用户输入 | prompt 参数 | 说明 |
|----------|------------|------|
| "写一个helloworld网页" | "写一个helloworld网页" | 原文直传 |
| "编码 go语言写一个网页" | "编码 go语言写一个网页" | 保留"编码"前缀，不要去掉 |
| "重构登录模块，把密码改成bcrypt加密" | "重构登录模块，把密码改成bcrypt加密" | 原文直传 |

**禁止的做法：**
- 把用户消息改写成更详细的技术方案后传入
- 把用户消息拆成多段分别调用
- 添加用户没提到的技术细节或需求

## 系统级补充（唯一例外）

以下补充由系统自动附加到编码 prompt 末尾，不属于 LLM "加料"：
- Go 项目：追加"确保生成 go.mod 文件"（避免部署时编译失败）

仅限上述列表中的固定补充，LLM 不得自行发明新的补充内容。

## 项目创建

CodegenStartSession 会在项目不存在时自动创建，通常不需要单独调用 CodegenCreateProject。

## 项目命名规范

`project` 参数必须使用描述性名称（如 `helloworld-web`、`todo-api`），**严禁使用 account 账号名作为项目名**。

命名规则：提取核心功能关键词，小写+连字符格式。

## 状态查询

CodegenGetStatus 支持传入 session_id 查询指定任务状态。只有当工具明确返回"进行中"状态时，才需要后续的状态检查子任务。

## 注意事项

- 编码和部署是两个核心步骤，不要将"创建项目""发送通知"拆为独立子任务
- 标注"同步等待完成"的工具，调用后会阻塞直到返回结果，不需要额外的"等待完成""轮询结果"子任务
- 编译/语法错误修复：部署阶段发现编译错误时，通过 CodegenSendMessage 让编码 agent 修复，禁止用 ExecuteCode（Python）直接改写 Go/JS 等源代码文件
