package main

import (
	"strings"
	"testing"
)

func TestBuildSubTaskResultTextPutsKeyDataBeforeSummary(t *testing.T) {
	keyData := keyToolDataHeader + "\n- AcpStartSession: project_dir=/tmp/webcalc, session_id=acp_123"
	summary := "已完成 go web 计算器编码，并创建了入口文件与监听端口。"

	got := buildSubTaskResultText(summary, keyData)
	if !strings.HasPrefix(got, keyData) {
		t.Fatalf("expected key data to appear first, got: %s", got)
	}
	if !strings.Contains(got, "结果摘要:\n"+summary) {
		t.Fatalf("expected summary section after key data, got: %s", got)
	}
}

func TestBuildSiblingContextKeepsProjectDirWhenSummaryIsLong(t *testing.T) {
	keyData := keyToolDataHeader + "\n- AcpStartSession: project_dir=/tmp/webcalc, session_id=acp_123"
	longSummary := strings.Repeat("编码结果摘要。", 800)
	result := buildSubTaskResultText(longSummary, keyData)

	ctx := buildSiblingContext([]string{"t1"}, map[string]string{
		"t1": result,
	})

	if !strings.Contains(ctx, "project_dir=/tmp/webcalc") {
		t.Fatalf("expected sibling context to keep project_dir, got: %s", ctx)
	}
}
