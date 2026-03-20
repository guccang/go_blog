---
name: env-check
description: 环境检测技能。当用户需要检查服务器环境、软件版本、系统状态时使用此技能。
summary: 用 Bash 检测软件版本和系统状态
tools: Bash
keywords: 环境,检查,env,安装,系统
---

# 环境检测

## 执行步骤

### 步骤 1：确认目标
确认要检测的是本地还是远程服务器。

### 步骤 2：执行检测
常用检测命令：
- Go 版本：`Bash("go version")` 或 `Bash("ssh root@server 'go version'")`
- Node 版本：`Bash("ssh root@server 'node --version'")`
- 磁盘空间：`Bash("ssh root@server 'df -h'")`
- 内存使用：`Bash("ssh root@server 'free -h'")`
- 进程状态：`Bash("ssh root@server 'ps aux | grep {process}'")`
- 端口监听：`Bash("ssh root@server 'ss -tlnp | grep {port}'")`

### 步骤 3：汇报结果
整理检测结果，标注异常项。
