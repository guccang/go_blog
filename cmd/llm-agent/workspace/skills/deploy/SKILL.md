---
name: deploy
description: 项目部署技能。当用户需要部署项目、发布上线、执行部署流水线时使用此技能。
summary: 已配置项目用 DeployProject + deploy_target，未配置项目用 DeployAdhoc
tools: DeployListProjects,DeployProject,DeployAdhoc,DeployPipeline
agents: deploy
keywords: 部署,deploy,发布,上线
---

# 项目部署

## 部署流程（必须遵守）

1. **先调用 DeployListProjects** 查看项目列表和配置状态
2. 根据 `configured` 字段选择接口：
   - `configured=true` → 使用 **DeployProject**（只需 project + deploy_target）
   - `configured=false` → 使用 **DeployAdhoc**（需要 project_dir + ssh_host）

## 用户参数不可修改（强制规则）

用户指定的部署参数（端口、地址、环境等）**严禁擅自修改**：
- 用户指定端口 → 必须使用该端口，不得更换
- 端口被占用、权限不足等冲突 → **直接返回部署失败**并说明原因，不得自动更换端口
- 地址、域名等参数同理，用户怎么说就怎么用

## DeployProject（已配置项目）

用于部署 settings 中已有配置的项目。配置文件已定义了构建脚本、部署路径、发布脚本等。

**调用示例：**

```json
{
  "project": "llm-agent",
  "deploy_target": "ssh-prod"
}
```

- `project`：项目名称（来自 DeployListProjects）
- `deploy_target`：部署目标（来自 DeployListProjects 返回的 targets 列表，如 local, ssh-prod）
- **不要传** `ssh_host`、`project_dir` 等参数，已配置项目的路径由 settings 管理

## DeployAdhoc（未配置项目/一次性部署）

用于部署未在 settings 中配置的项目，需要手动指定源码目录和目标服务器。

**调用示例（编码后部署新项目到远程服务器）：**

```json
{
  "project": "helloworld-web",
  "project_dir": "/path/from/coding/task/result",
  "ssh_host": "root@114.115.214.86"
}
```

- `project` 和 `project_dir` **必须**从前置编码任务的结果中提取，禁止猜测
- `ssh_host` **必须**从系统提示的 agent 能力描述中获取真实地址，禁止使用别名

## DeployPipeline 使用

用于编排多步部署流水线：
- 多项目按序部署
- 自定义部署前/后脚本
- 验证 URL 健康检查

## 部署失败处理

部署失败时的标准行为：
1. 返回失败状态和具体错误原因（端口冲突、编译错误、网络不通等）
2. **不得**自行重试并更换参数（如换端口）
3. 编译错误 → 通过 CodegenSendMessage 让编码 agent 修复源代码后重新部署
4. 参数类错误（端口占用等） → 直接报告失败，由用户决定下一步

## 注意事项

- DeployProject/DeployAdhoc 是同步工具，调用后阻塞直到完成，不需要额外的"等待部署""检查状态"子任务
- 编码和部署是两个独立步骤，拆解任务时分别作为子任务处理
- **禁止**用 ExecuteCode 探索项目目录、查找 SSH 地址、读取源代码
- 部署子任务应简洁：DeployListProjects → DeployProject/DeployAdhoc，通常只需 2 次调用
