package main

import (
	"sort"
	"strings"
)

func normalizeScenarioAgentRefs(scenario *TestScenario, online map[string]GatewayAgentSnapshot) {
	if scenario == nil {
		return
	}
	scenario.Entry.ToAgent = resolveOnlineAgentRef(scenario.Entry.ToAgent, online)
	if scenario.Entry.Notify != nil {
		scenario.Entry.Notify.To = resolveOnlineAgentRef(scenario.Entry.Notify.To, online)
	}
	for i, agentID := range scenario.Assertions.RequireAgents {
		scenario.Assertions.RequireAgents[i] = resolveOnlineAgentRef(agentID, online)
	}
	for i, item := range scenario.Assertions.ExpectedPath {
		scenario.Assertions.ExpectedPath[i] = resolveOnlineAgentRef(item, online)
	}
}

func resolveOnlineAgentRef(agentRef string, online map[string]GatewayAgentSnapshot) string {
	agentRef = strings.TrimSpace(agentRef)
	if agentRef == "" {
		return ""
	}
	if _, ok := online[agentRef]; ok {
		return agentRef
	}
	normalized := normalizeAgentAlias(agentRef)
	if normalized == "" {
		return agentRef
	}
	for _, agentID := range sortedOnlineAgentIDs(online) {
		agent := online[agentID]
		for _, alias := range collectAgentAliases(agentID, agent) {
			if normalizeAgentAlias(alias) == normalized {
				return agentID
			}
		}
	}
	return agentRef
}

func normalizeAgentAlias(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer("_", "-", " ", "-", "\t", "-").Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func collectAgentAliases(agentID string, agent GatewayAgentSnapshot) []string {
	seen := make(map[string]bool)
	var aliases []string
	appendAlias := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		aliases = append(aliases, value)
	}
	appendAlias(agentID)
	appendAlias(agent.AgentID)
	appendAlias(agent.Name)
	appendAlias(agent.AgentType)

	if stripped := trimNumericAgentSuffix(agentID); stripped != agentID {
		appendAlias(stripped)
	}
	if stripped := trimNumericAgentSuffix(agent.AgentID); stripped != agent.AgentID {
		appendAlias(stripped)
	}

	typeAlias := normalizeAgentAlias(agent.AgentType)
	if typeAlias != "" && !strings.HasSuffix(typeAlias, "-agent") {
		appendAlias(typeAlias + "-agent")
	}
	return aliases
}

func trimNumericAgentSuffix(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ""
	}
	idx := strings.LastIndexAny(agentID, "_-")
	if idx < 0 || idx == len(agentID)-1 {
		return agentID
	}
	suffix := agentID[idx+1:]
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return agentID
		}
	}
	return agentID[:idx]
}

func sortedOnlineAgentIDs(online map[string]GatewayAgentSnapshot) []string {
	ids := make([]string, 0, len(online))
	for agentID := range online {
		ids = append(ids, agentID)
	}
	sort.Strings(ids)
	return ids
}
