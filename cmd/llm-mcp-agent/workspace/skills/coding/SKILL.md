---
name: coding
description: 编码开发技能。当用户需要编写代码、创建项目、修复bug、重构、开发新功能时使用此技能。
tools: CodegenStartSession,CodegenSendMessage,CodegenGetStatus,CodegenStopSession,CodegenCreateProject
---

# 编码开发

## 消息传递规则（最高优先级）

当用户消息包含编码需求时，**必须将用户的原始消息原文直接作为 prompt 传给 CodegenStartSession**，严禁修改、缩写、翻译或重新措辞。

编码 agent（Claude Code）具备完整的理解和执行能力，你不需要替它做任何预处理。

**正确做法：**
- 用户说："写一个helloworld网页" → prompt="写一个helloworld网页"
- 用户说："重构登录模块，把密码改成bcrypt加密" → prompt="重构登录模块，把密码改成bcrypt加密"

**错误做法：**
- ❌ 把用户消息改写成更详细的技术方案后传入
- ❌ 把"编码"两字去掉后传入
- ❌ 把用户消息拆成多段分别调用

## 项目创建

CodegenStartSession 会在项目不存在时自动创建项目，通常不需要单独调用 CodegenCreateProject 步骤。

## 状态查询

CodegenGetStatus 支持传入 session_id 查询指定任务状态。只有当工具明确返回"进行中"状态时，才需要后续的状态检查子任务。

## 混合任务处理

当任务同时包含编码和其他操作（如"编码XX然后部署到YY"）时：
- 第一步：调用 CodegenStartSession，prompt = 用户消息中的编码部分原文
- 第二步：编码完成后，调用部署工具部署
- 关键：编码和部署是两个独立的工具调用，不要把部署指令混入编码 prompt

## 注意事项

- 编码和部署是两个核心步骤，不要将"创建项目""发送通知"拆为独立子任务
- 工具描述中标注"同步等待完成"的工具，调用后会阻塞直到任务完成并返回完整结果，不需要创建额外的"等待完成""轮询结果"子任务
