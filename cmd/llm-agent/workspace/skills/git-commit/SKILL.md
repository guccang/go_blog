---
name: 代码提交
description: 代码提交技能。当用户需要提交代码、推送代码时使用此技能。
summary: 先 ListProjects 获取项目，再 StartSession 执行 git 操作
tools: CodegenListProjects,CodegenStartSession
agents: codegen
keywords: git,提交,commit,推送,仓库
---

# 代码提交

## 强制规则

**收到提交/推送请求时，必须完整执行以下 3 个步骤，不得中途停止、不得回复"无法执行"或拒绝。**

## 执行步骤

### 步骤 1：调用 CodegenListProjects

调用 `CodegenListProjects` 获取所有可用项目列表。

### 步骤 2：匹配目标项目

从返回的项目列表中匹配用户指定的项目名称：
- 精确匹配或模糊匹配（如 "go_blog" 匹配 "go-blog" 或 "go_blog"）
- **如果列表中没有匹配项，直接使用用户指定的原始名称**，不得停止

### 步骤 3：调用 CodegenStartSession（必须执行）

无论步骤 2 是否找到精确匹配，都**必须**调用 `CodegenStartSession`：
- `project`：匹配到的项目名，或用户指定的原始名称
- `prompt`：`commit all changes and push to remote`

```json
{
  "project": "go_blog",
  "prompt": "commit all changes and push to remote"
}
```

## 禁止行为

- 禁止只执行步骤 1 就结束，**步骤 3 是必须执行的**
- 禁止回复"项目不存在""找不到项目""无法执行"
- 禁止用 ExecEnvBash/shell 命令查找目录替代上述步骤
- 如果用户指定了 commit message，附加到 prompt 中：`commit all changes with message "fix: xxx" and push to remote`
