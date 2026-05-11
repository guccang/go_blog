package main

import "testing"

func TestClassifyAppProcessMeta(t *testing.T) {
	meta := classifyAppProcessMeta("llm-agent", "部署进度: 正在发布 blog-agent")
	if meta == nil {
		t.Fatalf("expected process meta")
	}
	if meta["origin"] != "app-process" {
		t.Fatalf("origin=%v want app-process", meta["origin"])
	}
	if meta["process_kind"] != "deploy" {
		t.Fatalf("process_kind=%v want deploy", meta["process_kind"])
	}
}

func TestClassifyAppProcessMetaForDeployAgentFlutterBuild(t *testing.T) {
	meta := classifyAppProcessMeta("deploy-agent-mac", "build-flutter-apk 打包完成: app-release.apk")
	if meta == nil {
		t.Fatalf("expected process meta")
	}
	if meta["origin"] != "app-process" {
		t.Fatalf("origin=%v want app-process", meta["origin"])
	}
	if meta["process_kind"] != "deploy" {
		t.Fatalf("process_kind=%v want deploy", meta["process_kind"])
	}
}

func TestClassifyAppProcessMetaIgnoresNormalReply(t *testing.T) {
	meta := classifyAppProcessMeta("llm-agent", "你好，我已经帮你整理好了今天的计划。")
	if meta != nil {
		t.Fatalf("expected nil meta for normal reply, got %#v", meta)
	}
}
