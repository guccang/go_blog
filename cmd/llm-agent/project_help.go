package main

import "strings"

func isProjectManagementHelpCommand(content string) bool {
	content = strings.TrimSpace(content)
	commands := []string{"/help", "help", "帮助"}
	for _, cmd := range commands {
		if strings.EqualFold(content, cmd) {
			return true
		}
	}
	return false
}

func buildProjectManagementHelp() string {
	return `项目管理操作建议

可以直接这样对我说：
1. 创建项目
“帮我创建一个项目：博客系统升级，状态 active，优先级 high，结束日期 2026-04-30”

2. 查看项目
“列出我当前所有项目”
“列出状态为 active 的项目”
“查看项目 xxx 的详情”

3. 管理目标 Goal
“给项目 xxx 新增目标：完成登录改造，优先级 high，进度 20”
“把项目 xxx 的目标 yyy 更新为已完成”

4. 管理 OKR
“给项目 xxx 新增 OKR：提升博客发布效率，周期 2026Q2”
“把项目 xxx 的 OKR yyy 状态改成 at_risk”

5. 管理关键结果 KR
“给项目 xxx 的 OKR yyy 添加关键结果：单篇发布时间降到 5 分钟”
“把关键结果 zzz 当前值更新到 3”

6. 查看汇总
“给我看项目管理摘要”
“统计一下项目数、逾期项目和活跃 OKR”

建议写法：
- 尽量提供项目名、状态、优先级、截止日期
- 更新时最好带项目ID、goalID、okrID 或关键结果ID
- 项目状态：planning、active、on_hold、completed、cancelled
- Goal 状态：pending、in_progress、completed、cancelled
- OKR 状态：draft、active、at_risk、completed、cancelled

如果你不想记格式，直接说自然语言也可以，比如：
“帮我建一个项目，用来管理 blog-agent 的项目管理功能开发，并补两个目标和一个 OKR”`
}
