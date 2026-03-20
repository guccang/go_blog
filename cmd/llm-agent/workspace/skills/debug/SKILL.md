---
name: debug
description: 问题诊断技能。当用户需要排查问题、查看日志、分析错误时使用此技能。
summary: 用 Bash 查日志和进程状态，先定位再修复
tools: Bash
keywords: 调试,debug,排查,日志,错误
---

# 问题诊断

## 执行步骤

### 步骤 1：确认目标
确认用户要排查的服务和日志位置。

### 步骤 2：查看日志
使用 Bash 查看日志：
- 本地日志：`Bash("tail -200 /data/logs/app.log")`
- 远程日志：`Bash("ssh root@114.115.214.86 'tail -200 /data/logs/app.log'")`
- 搜索错误：`Bash("ssh root@114.115.214.86 'grep -i error /data/logs/app.log | tail -50'")`
- 按时间过滤：`Bash("ssh root@114.115.214.86 'grep 2026-03-17 /data/logs/app.log | grep -i error'")`

### 步骤 3：分析定位
根据日志内容分析根因，给出修复建议。

## 注意事项
- 先用 tail 看最近日志，再用 grep 缩小范围
- 日志量大时用 grep 过滤，避免输出过多
