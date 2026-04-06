---
name: git-commit
description: 代码提交技能。当用户需要提交代码、推送代码时使用此技能。
summary: 先列出项目，再启动 Codegen 会话完成 commit 和 push
tools: CodegenListProjects,CodegenStartSession
agents: codegen
keywords: git,提交,commit,推送,仓库
---

# 代码提交

## 适用场景

- 用户明确要求提交代码、生成 commit、推送远程仓库
- 需要通过现有 codegen 会话能力完成标准 git 提交流程

## 必须遵守

- 收到提交或推送请求时，必须先调用 `CodegenListProjects`
- 之后必须调用 `CodegenStartSession`，不能只列项目不执行
- 项目名优先做精确或模糊匹配；如果匹配不到，仍使用用户原始项目名继续
- 用户给了 commit message，就把它带进 prompt；没给则使用通用提交提示

## 推荐流程

1. 调用 `CodegenListProjects` 获取可用项目列表。
2. 尝试匹配用户指定的项目名称，例如 `go_blog` 匹配 `go-blog` 或 `go_blog`。
3. 调用 `CodegenStartSession`：
   - `project` 使用匹配结果，或用户原始名称
   - `prompt` 默认为 `commit all changes and push to remote`
4. 如果用户给了提交说明，则改为 `commit all changes with message "<message>" and push to remote`

## 工具选择规则

- `CodegenListProjects` 只做项目发现
- `CodegenStartSession` 负责实际执行 git 提交和推送
- 不要用 shell/Bash 自己去遍历目录或手搓 git 流程替代这两个工具

## 禁止行为

- 只执行项目列表查询就结束
- 回复“项目不存在”“找不到项目”“无法执行”后直接停止
- 用 `ExecEnvBash` 或普通 shell 查目录来绕过既定工具流程
- 擅自改写用户提供的 commit message

## 示例

```json
{
  "project": "go_blog",
  "prompt": "commit all changes and push to remote"
}
```

如果用户说“用 fix: update deploy script 作为提交信息”，则 prompt 改为：
`commit all changes with message "fix: update deploy script" and push to remote`
