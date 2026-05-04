package main

import (
	"strings"
	"testing"
)

func TestBuildCortanaProactiveSystemPromptIncludesProfile(t *testing.T) {
	prompt := buildCortanaProactiveSystemPrompt(&CortanaProfile{
		Name:        "Cortana Prime",
		OwnerTitle:  "主人",
		Description: "冷静、主动、会接住用户情绪",
	})

	for _, want := range []string{
		"当前 Cortana 名称: Cortana Prime",
		"对用户固定称呼: 主人",
		"冷静、主动、会接住用户情绪",
		"语气、人设和称呼必须与以上 Cortana 设定保持一致",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, prompt)
		}
	}
}
