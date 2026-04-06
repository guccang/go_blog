package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSkillDocumentsFollowConventions(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("workspace", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("glob skill files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected skill documents")
	}

	namePattern := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	requiredSections := []string{
		"## 适用场景",
		"## 必须遵守",
		"## 推荐流程",
		"## 工具选择规则",
		"## 禁止行为",
		"## 示例",
	}
	bannedSnippets := []string{
		"corn-agent",
		"root@114.115.214.86",
		"ssh root@server",
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		skill, err := parseSkillFile(string(data), path)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		if !namePattern.MatchString(skill.Name) {
			t.Fatalf("%s: invalid skill name %q", path, skill.Name)
		}
		if strings.TrimSpace(skill.Description) == "" {
			t.Fatalf("%s: missing description", path)
		}
		if strings.TrimSpace(skill.Summary) == "" {
			t.Fatalf("%s: missing summary", path)
		}
		if len(skill.Tools) == 0 {
			t.Fatalf("%s: missing tools", path)
		}
		if len(skill.Keywords) == 0 {
			t.Fatalf("%s: missing keywords", path)
		}

		for _, section := range requiredSections {
			if !strings.Contains(skill.Content, section) {
				t.Fatalf("%s: missing section %q", path, section)
			}
		}
		for _, snippet := range bannedSnippets {
			if strings.Contains(skill.Content, snippet) {
				t.Fatalf("%s: contains banned snippet %q", path, snippet)
			}
		}
	}
}
