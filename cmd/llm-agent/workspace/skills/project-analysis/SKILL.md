---
name: project-analysis
description: 项目分析技能。分析项目代码质量、架构设计、性能、安全等，给出优化建议。
summary: 先列项目，再调用 AcpAnalyzeProject，prompt 传用户原文
tools: AcpAnalyzeProject,AcpListProjects
agents: acp
keywords: 分析,审查,review,架构,性能,安全,项目
---

# 项目分析

## 适用场景

当用户需要分析项目代码质量、架构设计、性能瓶颈、安全隐患等，给出优化建议时使用此技能。

## 必须遵守

将用户的原始消息原文直接作为 prompt 传给 AcpAnalyzeProject，**严禁修改、缩写、翻译、重新措辞或添加额外内容**。

ACP Agent（Claude Code）具备完整的理解和执行能力，不需要预处理。用户怎么说就怎么传，不得"加料"。

## 推荐流程

1. 调用 `AcpListProjects` 获取可分析项目列表。
2. 匹配用户指定的项目名称；匹配不到时，仍然使用用户原始名称继续。
3. 调用 `AcpAnalyzeProject`：
   - `project` 使用匹配结果或用户原始名称
   - `prompt` 直接传用户原始消息
4. 返回分析结论、风险点和建议，不要在分析前擅自改写需求。

## 工具选择规则

- `AcpListProjects`：只做项目发现和匹配
- `AcpAnalyzeProject`：执行实际项目分析
- 不需要开启编码会话或额外拆分 prompt

## 禁止行为

- 禁止只执行步骤 1 就结束，**步骤 3 是必须执行的**
- 禁止修改用户的分析需求原文
- 禁止回复"项目不存在""找不到项目""无法执行"

## 示例

- 用户说“分析 go_blog 的登录模块有没有安全问题”
  先列项目并匹配 `go_blog`，再把这句原话直接传给 `AcpAnalyzeProject`
