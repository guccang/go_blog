---
name: deploy
description: 项目部署技能。当用户需要部署项目、发布上线、执行部署流水线时使用此技能。
tools: DeployProject,DeployPipeline
---

# 项目部署

## DeployProject 使用

DeployProject 支持三种部署模式：
- **指定项目名**：直接部署已有项目
- **指定仓库 URL**：从 Git 仓库拉取并部署
- **自动检测**：根据编码会话的项目自动部署

## DeployPipeline 使用

DeployPipeline 用于编排多步部署流水线，支持：
- 多项目按序部署
- 自定义部署前/后脚本
- 验证 URL 健康检查

## 部署前检查

在执行部署前，确认以下信息：
- 项目名或仓库地址
- 目标环境（如有多环境）
- 部署端口（如需指定）

## verify_url 使用

部署完成后如需验证，可使用 verify_url 参数指定健康检查地址。部署工具会自动请求该 URL 确认服务正常。

## 注意事项

- DeployProject 是同步工具，调用后会阻塞直到部署完成，不需要额外的"等待部署""检查状态"子任务
- 编码和部署是两个独立步骤，拆解任务时分别作为子任务处理
