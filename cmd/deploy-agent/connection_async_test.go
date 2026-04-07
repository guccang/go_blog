package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigIncludesBuildOnlyProjectWithoutTargets(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(t.TempDir(), "settings")
	projectsDir := filepath.Join(settingsDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("MkdirAll projectsDir failed: %v", err)
	}

	projectJSON := mustJSONString(map[string]any{
		"build_only":   true,
		"pack_pattern": "build/app/outputs/flutter-apk/app-release-*.apk",
		"build": map[string]any{
			platformSubdir(): map[string]any{
				"project_dir": projectDir,
				"pack_script": "run-push-apk.sh",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(projectsDir, "build-flutter-apk.json"), []byte(projectJSON), 0644); err != nil {
		t.Fatalf("WriteFile project json failed: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "deploy-agent.json")
	configJSON := mustJSONString(map[string]any{
		"settings_dir": settingsDir,
		"workspaces":   []string{},
	})
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("WriteFile config failed: %v", err)
	}

	cfg, err := LoadConfig(configPath, "all")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	proj := cfg.GetProject("build-flutter-apk")
	if proj == nil {
		t.Fatalf("expected build-flutter-apk project to be loaded")
	}
	if !proj.BuildOnly {
		t.Fatalf("expected build-flutter-apk to be build_only")
	}
	if len(proj.Targets) != 0 {
		t.Fatalf("expected build-flutter-apk to have no deploy targets, got %d", len(proj.Targets))
	}
}

func TestDeployProjectBuildOnlyRunsAsyncAndRecordsArtifact(t *testing.T) {
	conn := newAsyncTestConnection(t, "flutter-apk-test", true, 1)

	start := time.Now()
	resultJSON := conn.toolDeployProject(map[string]interface{}{
		"project":   "flutter-apk-test",
		"pack_only": true,
	}, func(string, string) {})
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected async return, took %s", elapsed)
	}

	result := parseToolJSON(t, resultJSON)
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result, got %#v", result)
	}
	if accepted, _ := result["accepted"].(bool); !accepted {
		t.Fatalf("expected accepted=true, got %#v", result)
	}
	sessionID, _ := result["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("expected session_id in result, got %#v", result)
	}

	statusJSON := conn.toolDeployGetStatus(map[string]interface{}{"session_id": sessionID})
	status := parseToolJSON(t, statusJSON)
	gotStatus, _ := status["status"].(string)
	if gotStatus != deployTaskStatusQueued && gotStatus != deployTaskStatusInProgress {
		t.Fatalf("expected queued/in_progress, got %#v", status)
	}

	final := waitForTaskStatus(t, conn, sessionID, deployTaskStatusDone, 4*time.Second)
	resultData, _ := final["result"].(map[string]interface{})
	if resultData == nil {
		t.Fatalf("expected result payload, got %#v", final)
	}
	if got, _ := resultData["artifact_file"].(string); got != "app-release-1.0.10.apk" {
		t.Fatalf("expected artifact_file app-release-1.0.10.apk, got %#v", resultData["artifact_file"])
	}
	artifactPath, _ := resultData["artifact_path"].(string)
	if !strings.HasSuffix(filepath.ToSlash(artifactPath), "build/app/outputs/flutter-apk/app-release-1.0.10.apk") {
		t.Fatalf("unexpected artifact_path: %q", artifactPath)
	}
}

func TestDeployProjectBuildOnlyPropagatesPort(t *testing.T) {
	conn := newAsyncTestConnection(t, "flutter-apk-test", true, 1)

	result := parseToolJSON(t, conn.toolDeployProject(map[string]interface{}{
		"project":   "flutter-apk-test",
		"pack_only": true,
		"port":      8888,
	}, func(string, string) {}))
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result, got %#v", result)
	}
	if got, _ := result["port"].(float64); got != 8888 {
		t.Fatalf("expected accepted result port=8888, got %#v", result["port"])
	}

	sessionID, _ := result["session_id"].(string)
	final := waitForTaskStatus(t, conn, sessionID, deployTaskStatusDone, 4*time.Second)
	if got, _ := final["port"].(float64); got != 8888 {
		t.Fatalf("expected final status port=8888, got %#v", final["port"])
	}
	resultData, _ := final["result"].(map[string]interface{})
	if got, _ := resultData["port"].(float64); got != 8888 {
		t.Fatalf("expected result payload port=8888, got %#v", resultData["port"])
	}
}

func TestCloneProjectWithServicePortOverridesTarget(t *testing.T) {
	proj := &ProjectConfig{
		Name: "demo",
		Targets: []*Target{
			{Name: "local", ServicePort: 0},
			{Name: "ssh-prod", ServicePort: 0},
		},
	}

	cloned := cloneProjectWithServicePort(proj, 8888, "ssh-prod")
	if cloned == proj {
		t.Fatalf("expected cloned project when service port override is provided")
	}
	if got := cloned.Targets[0].ServicePort; got != 0 {
		t.Fatalf("expected local target port to remain 0, got %d", got)
	}
	if got := cloned.Targets[1].ServicePort; got != 8888 {
		t.Fatalf("expected ssh-prod target port=8888, got %d", got)
	}
	if got := proj.Targets[1].ServicePort; got != 0 {
		t.Fatalf("expected original project to remain unchanged, got %d", got)
	}
}

func TestDeployProjectBuildOnlyRequiresPackOnly(t *testing.T) {
	conn := newAsyncTestConnection(t, "flutter-apk-test", true, 1)

	result := parseToolJSON(t, conn.toolDeployProject(map[string]interface{}{
		"project": "flutter-apk-test",
	}, func(string, string) {}))
	if success, _ := result["success"].(bool); success {
		t.Fatalf("expected failure result, got %#v", result)
	}
	errText, _ := result["error"].(string)
	if !strings.Contains(errText, "only supports pack_only") {
		t.Fatalf("unexpected error: %q", errText)
	}
}

func TestDeployProjectAsyncRejectsWhenBusy(t *testing.T) {
	conn := newAsyncTestConnection(t, "flutter-apk-test", false, 1)

	first := parseToolJSON(t, conn.toolDeployProject(map[string]interface{}{
		"project":   "flutter-apk-test",
		"pack_only": true,
	}, func(string, string) {}))
	if success, _ := first["success"].(bool); !success {
		t.Fatalf("expected first task accepted, got %#v", first)
	}
	firstSessionID, _ := first["session_id"].(string)

	second := parseToolJSON(t, conn.toolDeployProject(map[string]interface{}{
		"project":   "flutter-apk-test",
		"pack_only": true,
	}, func(string, string) {}))
	if success, _ := second["success"].(bool); success {
		t.Fatalf("expected second task to be rejected while busy, got %#v", second)
	}
	if errText, _ := second["error"].(string); !strings.Contains(errText, "busy") {
		t.Fatalf("unexpected busy error: %#v", second)
	}

	waitForTaskStatus(t, conn, firstSessionID, deployTaskStatusDone, 4*time.Second)
}

func newAsyncTestConnection(t *testing.T, projectName string, buildOnly bool, maxConcurrent int) *Connection {
	t.Helper()

	projectDir := t.TempDir()
	scriptPath := filepath.Join(projectDir, "pack.sh")
	scriptContent := "#!/bin/bash\nset -e\nsleep 1\nmkdir -p build/app/outputs/flutter-apk\nprintf 'apk' > build/app/outputs/flutter-apk/app-release-1.0.10.apk\n"
	if !buildOnly {
		scriptContent = "#!/bin/bash\nset -e\nsleep 1\nprintf 'zip' > " + projectName + "_20260406_120000.zip\n"
	}
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("WriteFile script failed: %v", err)
	}

	packPattern := "build/app/outputs/flutter-apk/app-release-*.apk"
	if !buildOnly {
		packPattern = projectName + "_{date}.zip"
	}
	cfg := &DeployConfig{
		HostPlatform:  platformSubdir(),
		MaxConcurrent: maxConcurrent,
		Projects: map[string]*ProjectConfig{
			projectName: {
				Name:        projectName,
				ProjectDir:  projectDir,
				PackScript:  scriptPath,
				PackPattern: packPattern,
				BuildOnly:   buildOnly,
			},
		},
		ProjectOrder: []string{projectName},
	}
	return NewConnection(cfg, "", "deploy-agent-test")
}

func parseToolJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v raw=%s", err, raw)
	}
	return result
}

func waitForTaskStatus(t *testing.T, conn *Connection, sessionID, want string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := parseToolJSON(t, conn.toolDeployGetStatus(map[string]interface{}{"session_id": sessionID}))
		if got, _ := status["status"].(string); got == want {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for session %s to reach %s", sessionID, want)
	return nil
}
