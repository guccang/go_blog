---
name: git-commit
description: Git提交技能。当用户需要提交代码、git commit、推送代码时使用此技能。
tools: CodegenListProjects,CodegenStartSession
---

# Git 提交

## 强制规则

**收到 git 提交/推送请求时，必须执行以下步骤，不得回复"无法执行"或拒绝。**
这是本系统执行 git 操作的标准路径：通过 CodegenStartSession 启动编码会话，由编码 agent（Claude Code）在项目目录中执行 git 命令。

## 执行步骤

### 步骤 1：调用 CodegenListProjects

调用 `CodegenListProjects` 获取所有可用项目列表。

### 步骤 2：匹配目标项目

从返回的项目列表中，根据用户指定的项目名称进行匹配：
- 用户明确指定了项目名 → 精确匹配或模糊匹配
- 用户未指定项目名 → 询问用户要提交哪个项目

### 步骤 3：调用 CodegenStartSession

调用 `CodegenStartSession`，参数：
- `project`：步骤 2 匹配到的项目名称
- `prompt`：`commit all changes and push to remote`

**调用示例：**

```json
{
  "project": "go_blog",
  "prompt": "commit all changes and push to remote"
}
```

## 注意事项

- 必须先通过 CodegenListProjects 确认项目存在，不得猜测项目名
- prompt 固定为 `commit all changes and push to remote`，除非用户有特殊的 commit message 要求
- 如果用户指定了 commit message，将其附加到 prompt 中，例如：`commit all changes with message "fix: xxx" and push to remote`
- CodegenStartSession 是同步工具，调用后阻塞直到完成，不需要额外的轮询子任务
