package main

import "strings"

// SubTaskPlan 子任务描述，当前用于 execute_skill 等隔离子任务运行。
type SubTaskPlan struct {
	ID          string
	Title       string
	Description string
	ContextMode string // fresh / fork
	ToolsHint   []string
}

func normalizeContextMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "fork":
		return "fork"
	default:
		return "fresh"
	}
}

func (st SubTaskPlan) EffectiveContextMode() string {
	return normalizeContextMode(st.ContextMode)
}
