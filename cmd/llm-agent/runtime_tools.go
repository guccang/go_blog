package main

import "log"

type ToolRuntimeView struct {
	AllTools        []LLMTool
	VisibleTools    []LLMTool
	DiscoveredTools map[string]LLMTool
	SourceReasons   map[string]string
}

func newToolRuntimeView(allTools, visibleTools []LLMTool) *ToolRuntimeView {
	view := &ToolRuntimeView{
		AllTools:        cloneTools(allTools),
		VisibleTools:    cloneTools(visibleTools),
		DiscoveredTools: make(map[string]LLMTool),
		SourceReasons:   make(map[string]string),
	}
	for _, tool := range allTools {
		view.SourceReasons[tool.Function.Name] = "base"
	}
	for _, tool := range visibleTools {
		if _, ok := view.SourceReasons[tool.Function.Name]; !ok {
			view.SourceReasons[tool.Function.Name] = "visible"
		}
	}
	return view
}

func cloneTools(tools []LLMTool) []LLMTool {
	if len(tools) == 0 {
		return nil
	}
	cloned := make([]LLMTool, len(tools))
	copy(cloned, tools)
	return cloned
}

func (b *Bridge) buildRootToolRuntimeView(ctx *TaskContext, query string, allTools []LLMTool) *ToolRuntimeView {
	visible := b.injectVirtualTools(cloneTools(allTools), ctx.NoTools)
	if !ctx.NoTools && query != "" && isGreeting(query) {
		visible = nil
	} else if !ctx.NoTools && query != "" {
		visible = b.filterRootToolsByMatchedSkills(query, visible)
	}
	view := newToolRuntimeView(allTools, visible)
	for _, tool := range visible {
		if _, ok := view.SourceReasons[tool.Function.Name]; !ok {
			view.SourceReasons[tool.Function.Name] = "runtime"
		}
	}
	return view
}

func (b *Bridge) filterRootToolsByMatchedSkills(query string, tools []LLMTool) []LLMTool {
	if b.skillMgr == nil || len(tools) == 0 {
		return tools
	}

	limit := 2
	if b.cfg != nil && b.cfg.MaxMatchedSkills > 0 {
		limit = b.cfg.MaxMatchedSkills
	}
	matchedSkills := b.skillMgr.MatchByQuery(query, limit)
	if len(matchedSkills) == 0 {
		return tools
	}

	allowed := make(map[string]bool)
	var matchedNames []string
	for _, skill := range matchedSkills {
		matchedNames = append(matchedNames, skill.Name)
		for _, toolName := range skill.Tools {
			allowed[toolName] = true
			allowed[sanitizeToolName(toolName)] = true
		}
	}

	var filtered []LLMTool
	for _, tool := range tools {
		name := b.resolveToolName(tool.Function.Name)
		switch {
		case isRootVirtualTool(name):
			filtered = append(filtered, tool)
		case allowed[name] || allowed[tool.Function.Name]:
			filtered = append(filtered, tool)
		}
	}

	if len(filtered) == 0 {
		return tools
	}
	log.Printf("[RootToolPolicy] matched skills=%v tools=%d→%d query=%s", matchedNames, len(tools), len(filtered), truncate(query, 120))
	return filtered
}

func isRootVirtualTool(name string) bool {
	switch name {
	case "plan_and_execute", "execute_skill",
		"get_skill_detail", "get_tool_detail", "get_agent_detail",
		"WebSearch", "WebFetch",
		"set_persona", "set_rule",
		"list_providers", "get_current_model", "switch_provider", "switch_model":
		return true
	default:
		return false
	}
}

func (b *Bridge) buildSubTaskToolRuntimeView(tools []LLMTool, hints []string) *ToolRuntimeView {
	base := excludeVirtualTools(cloneTools(tools), hints)
	visible := base
	if len(hints) > 0 {
		visible = b.ApplySubtaskPolicy(base, hints)
	}
	view := newToolRuntimeView(base, visible)
	for _, tool := range visible {
		view.SourceReasons[tool.Function.Name] = "subtask"
	}
	return view
}

func (b *Bridge) buildSkillToolRuntimeView(skill *SkillEntry, parentTools []LLMTool) *ToolRuntimeView {
	allTools := cloneTools(parentTools)
	if len(allTools) == 0 {
		allTools = b.getLLMTools()
	}
	visible := b.filterToolsForSkill(skill, allTools)
	view := newToolRuntimeView(allTools, visible)
	for _, tool := range visible {
		view.SourceReasons[tool.Function.Name] = "skill"
	}
	return view
}

func (tv *ToolRuntimeView) Visible() []LLMTool {
	return cloneTools(tv.VisibleTools)
}

func (tv *ToolRuntimeView) ExpandWithDiscoveredTools(tools []LLMTool) []string {
	if tv == nil || len(tools) == 0 {
		return nil
	}
	existing := make(map[string]bool, len(tv.VisibleTools))
	for _, tool := range tv.VisibleTools {
		existing[tool.Function.Name] = true
	}

	var added []string
	for _, tool := range tools {
		if existing[tool.Function.Name] {
			continue
		}
		existing[tool.Function.Name] = true
		tv.VisibleTools = append(tv.VisibleTools, tool)
		tv.DiscoveredTools[tool.Function.Name] = tool
		tv.SourceReasons[tool.Function.Name] = "discovered"
		added = append(added, tool.Function.Name)
	}
	return added
}

func (b *Bridge) expandSiblingToolsInView(view *ToolRuntimeView, failedTools []string) []string {
	if view == nil {
		return nil
	}

	var canonicalAdded []string
	for _, failedTool := range failedTools {
		siblings := b.getSiblingTools(failedTool)
		added := view.ExpandWithDiscoveredTools(siblings)
		for _, name := range added {
			canonicalAdded = append(canonicalAdded, b.resolveToolName(name))
		}
	}
	return canonicalAdded
}
