---
name: deploy
description: 项目部署技能。当用户需要部署项目、发布上线、执行部署流水线时使用此技能。
summary: 独立部署先 ListProjects；configured=true 用 DeployProject，新项目直用 DeployAdhoc
tools: DeployListProjects,DeployProject,DeployAdhoc,DeployListPipelines,DeployPipeline
agents: deploy
keywords: 部署,deploy,发布,上线
---

# 项目部署

## 适用场景

- 用户直接要求部署、发布、上线某个项目
- 编码任务完成后，需要把产物部署到本地或远程目标
- 需要执行预配置的部署 pipeline

## 必须遵守

- 用户直接说“部署 xxx”时，先调用 `DeployListProjects` 看项目是否已配置
- `configured=true` 只能用 `DeployProject`
- `configured=false` 或编码任务刚产出的新项目，使用 `DeployAdhoc`
- 用户指定的端口、域名、地址、环境参数不得擅自修改；冲突时直接返回失败原因
- `DeployProject` / `DeployAdhoc` / `DeployPipeline` 是同步工具，返回后即可汇报结果，不要额外制造等待子任务

## 推荐流程

1. 独立部署任务：
   - 先调 `DeployListProjects`
   - 根据 `configured` 选择 `DeployProject` 或 `DeployAdhoc`
2. 编码后部署的新项目：
   - 直接使用 `DeployAdhoc`
   - `project` 和 `project_dir` 从前置编码结果中提取，不能猜
3. 多项目编排：
   - 先看 `DeployListPipelines`
   - 再执行 `DeployPipeline`
4. 部署失败时，返回具体错误；如果是编译错误，回到 `coding` 修代码后再重部署

## 工具选择规则

- `DeployListProjects`：只做发现，不执行部署
- `DeployProject`：用于 settings 中已配置的项目，只传 `project` 和可选 `deploy_target`
- `DeployAdhoc`：用于未配置项目或一次性部署，必须传 `project_dir` 和 `ssh_host`
- `DeployListPipelines` / `DeployPipeline`：用于预配置的多步骤流水线
- `ssh_host` 必须来自 agent 能力信息或用户明确指定的真实地址，例如 `deploy@prod-host`

## 禁止行为

- 对已配置项目继续传 `project_dir`、`ssh_host` 给 `DeployProject`
- 端口冲突后擅自换端口、改域名或改部署目标
- 用 `ExecuteCode` 去猜项目目录、SSH 地址或发布参数
- 对编码任务刚创建的新项目先调 `DeployListProjects` 再空转

## 示例

- “部署 llm-agent 到 ssh-prod”
  先 `DeployListProjects`，确认 `llm-agent` 为 `configured=true`，再调用 `DeployProject`
- “把刚写好的 helloworld-web 部署到生产机”
  直接从前置编码结果拿 `project` 和 `project_dir`，再调用 `DeployAdhoc`
