package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type CortanaProfile struct {
	Name        string `json:"name"`
	OwnerTitle  string `json:"owner_title,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedAt   int64  `json:"updated_at,omitempty"`
}

var cortanaOwnerTitlePattern = regexp.MustCompile(`(?:称呼(?:我|用户)?为|称呼)\s*[:：]?\s*[「“"]?([^，。；;\s」”"]+)[」”"]?`)

func cortanaProfilePath(baseDir, account string) string {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(account) == "" {
		return ""
	}
	return filepath.Join(GetAccountWorkspace(baseDir, account), "memory", "cortana_profile.json")
}

func loadCortanaProfile(baseDir, account string) *CortanaProfile {
	path := cortanaProfilePath(baseDir, account)
	if path == "" {
		return defaultCortanaProfile()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultCortanaProfile()
	}
	var profile CortanaProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return defaultCortanaProfile()
	}
	return normalizeCortanaProfile(&profile)
}

func defaultCortanaProfile() *CortanaProfile {
	return &CortanaProfile{
		Name:      "Cortana",
		UpdatedAt: time.Now().UnixMilli(),
	}
}

func normalizeCortanaProfile(profile *CortanaProfile) *CortanaProfile {
	if profile == nil {
		return defaultCortanaProfile()
	}
	out := *profile
	out.Name = strings.TrimSpace(out.Name)
	out.OwnerTitle = strings.TrimSpace(out.OwnerTitle)
	out.Description = strings.TrimSpace(out.Description)
	if out.Name == "" {
		out.Name = "Cortana"
	}
	if out.OwnerTitle == "" {
		out.OwnerTitle = inferOwnerTitleFromDescription(out.Description)
	}
	if out.UpdatedAt <= 0 {
		out.UpdatedAt = time.Now().UnixMilli()
	}
	return &out
}

func inferOwnerTitleFromDescription(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	matches := cortanaOwnerTitlePattern.FindStringSubmatch(description)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func buildCortanaPersonaPrompt(profile *CortanaProfile, account string) string {
	profile = normalizeCortanaProfile(profile)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("你是 %s。\n", profile.Name))
	sb.WriteString("你是用户在 Flutter 客户端中的唯一数字助手主体。用户无论在 Cortana 页面说话，还是在普通聊天页发消息，本质上都是在和你对话。\n")
	sb.WriteString("除群聊以外，不要把自己当成另一个通用 agent，也不要以第三方口吻谈论 Cortana；你自己就是 Cortana。\n")
	if profile.Description != "" {
		sb.WriteString("## Cortana 人设\n")
		sb.WriteString(profile.Description)
		sb.WriteString("\n")
	}
	if profile.OwnerTitle != "" {
		sb.WriteString(fmt.Sprintf("你对用户的固定称呼是「%s」。除非用户在当前对话中明确要求更换称呼，否则不要改用 account、用户名或其他名字称呼用户。\n", profile.OwnerTitle))
	}

	now := time.Now()
	sb.WriteString(fmt.Sprintf("account: %s\n", account))
	sb.WriteString(fmt.Sprintf("当前时间: %s %s\n", now.Format("2006-01-02 15:04"), chineseWeekday(now.Weekday())))
	sb.WriteString("## Cortana 工作边界\n")
	sb.WriteString("- `account` 仅用于权限、工作区和数据隔离标识，不是你对用户的称呼。\n")
	sb.WriteString("- 你要延续用户的长期上下文，主动承接他已有的数字资料、历史对话、规则、记忆和当前状态。\n")
	sb.WriteString("- 你的回复默认要自然、口语化、像同一个熟悉用户的数字同伴，而不是每轮重置身份的客服。\n")
	sb.WriteString("- 当任务需要调用工具、读写代码、查询资料、操作系统时，直接以 Cortana 的身份去完成，不需要切换成别的人设。\n")
	sb.WriteString("- 涉及删除数据、覆盖用户工作、推送发布、批量外部副作用时，先明确风险再执行。\n")
	sb.WriteString("- 不要编造执行结果；完成后做最小必要验证，没验证就明确说明。\n\n")
	return sb.String()
}

func (b *Bridge) buildCortanaAppSystemPrompt(account, query string, profile *CortanaProfile, state *CortanaCompanionState) (string, []PromptSection) {
	opts := b.buildAssistantPromptOptions(query, true)
	var sb strings.Builder
	var sections []PromptSection

	writeSection := func(name, content string) {
		if content == "" {
			return
		}
		sb.WriteString(content)
		sections = append(sections, PromptSection{Name: name, Chars: len([]rune(content))})
	}

	writeSection("Cortana人设", buildCortanaPersonaPrompt(profile, account))
	writeSection("Cortana陪伴", buildCortanaCompanionPrompt(state)+"\n")

	if opts.IncludeProjectInstructions {
		if cwd, err := os.Getwd(); err == nil {
			writeSection("项目指令", buildInstructionBlock(cwd))
			if opts.IncludeGitSnapshot {
				writeSection("Git快照", buildGitStatusBlock(cwd))
			}
		}
	}
	if opts.IncludeUserRules {
		if memMgr := b.GetMemoryManager(account); memMgr != nil {
			writeSection("用户规则", memMgr.BuildRulePromptBlock())
		}
	}
	if opts.IncludeAgentCapabilities {
		writeSection("Agent能力", b.getAgentDescriptionBlock())
	}
	if opts.IncludeToolCatalog {
		writeSection("工具目录", b.buildBriefToolCatalog())
	}
	if opts.IncludeSkillCatalog && b.skillMgr != nil {
		writeSection("Skill目录", b.skillMgr.BuildCatalogWithToolHint())
	}
	if opts.IncludeMemory {
		if memMgr := b.GetMemoryManager(account); memMgr != nil {
			writeSection("长期记忆", memMgr.BuildPromptBlock())
		}
	}

	return sb.String(), sections
}
