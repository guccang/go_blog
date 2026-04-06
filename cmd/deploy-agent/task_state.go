package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"uap"
)

const (
	deployTaskStatusQueued     = "queued"
	deployTaskStatusInProgress = "in_progress"
	deployTaskStatusDone       = "done"
	deployTaskStatusError      = "error"
	maxTaskHistory             = 200
	defaultAppAgentID          = "app-app-agent"
	flutterAPKNotifyUser       = "ztt"
)

type deployTaskRecord struct {
	SessionID    string         `json:"session_id"`
	ToolName     string         `json:"tool_name,omitempty"`
	Project      string         `json:"project,omitempty"`
	Pipeline     string         `json:"pipeline,omitempty"`
	ProjectDir   string         `json:"project_dir,omitempty"`
	SSHHost      string         `json:"ssh_host,omitempty"`
	DeployTarget string         `json:"deploy_target,omitempty"`
	PackOnly     bool           `json:"pack_only,omitempty"`
	Status       string         `json:"status"`
	Error        string         `json:"error,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
	CreatedAt    int64          `json:"created_at"`
	StartedAt    int64          `json:"started_at,omitempty"`
	FinishedAt   int64          `json:"finished_at,omitempty"`
	UpdatedAt    int64          `json:"updated_at"`
}

func newDeployTaskRecord(sessionID, toolName string, task TaskAssignPayload) *deployTaskRecord {
	now := time.Now().UnixMilli()
	return &deployTaskRecord{
		SessionID:    sessionID,
		ToolName:     toolName,
		Project:      task.Project,
		Pipeline:     task.Pipeline,
		ProjectDir:   task.ProjectDir,
		SSHHost:      task.SSHHost,
		DeployTarget: task.DeployTarget,
		PackOnly:     task.PackOnly,
		Status:       deployTaskStatusQueued,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func cloneResultMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (r *deployTaskRecord) snapshot() map[string]any {
	if r == nil {
		return nil
	}
	out := map[string]any{
		"success":       r.Status != deployTaskStatusError,
		"session_id":    r.SessionID,
		"tool_name":     r.ToolName,
		"project":       r.Project,
		"pipeline":      r.Pipeline,
		"project_dir":   r.ProjectDir,
		"ssh_host":      r.SSHHost,
		"deploy_target": r.DeployTarget,
		"pack_only":     r.PackOnly,
		"status":        r.Status,
		"created_at":    r.CreatedAt,
		"started_at":    r.StartedAt,
		"finished_at":   r.FinishedAt,
		"updated_at":    r.UpdatedAt,
	}
	if r.Error != "" {
		out["error"] = r.Error
	}
	if result := cloneResultMap(r.Result); len(result) > 0 {
		out["result"] = result
	}
	return out
}

func mustJSONString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"success":false,"error":"marshal result failed"}`
	}
	return string(data)
}

func buildToolErrorJSON(err error) string {
	if err == nil {
		return `{"success":false,"error":"unknown error"}`
	}
	return mustJSONString(map[string]any{
		"success": false,
		"error":   err.Error(),
	})
}

func (c *Connection) reserveTask(rec *deployTaskRecord) bool {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()

	if len(c.activeTasks) >= c.cfg.MaxConcurrent {
		return false
	}
	c.activeTasks[rec.SessionID] = true
	c.taskRecords[rec.SessionID] = rec
	c.taskOrder = append(c.taskOrder, rec.SessionID)
	return true
}

func (c *Connection) setTaskRunning(sessionID string) {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()

	rec := c.taskRecords[sessionID]
	if rec == nil {
		return
	}
	now := time.Now().UnixMilli()
	rec.Status = deployTaskStatusInProgress
	rec.StartedAt = now
	rec.UpdatedAt = now
}

func (c *Connection) completeTask(sessionID, status, errMsg string, result map[string]any) *deployTaskRecord {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()

	delete(c.activeTasks, sessionID)
	rec := c.taskRecords[sessionID]
	if rec == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	rec.Status = status
	rec.Error = errMsg
	rec.Result = cloneResultMap(result)
	rec.FinishedAt = now
	rec.UpdatedAt = now
	c.pruneTaskHistoryLocked()
	return rec
}

func (c *Connection) taskSnapshot(sessionID string) *deployTaskRecord {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()

	rec := c.taskRecords[sessionID]
	if rec == nil {
		return nil
	}
	copyRec := *rec
	copyRec.Result = cloneResultMap(rec.Result)
	return &copyRec
}

func (c *Connection) pruneTaskHistoryLocked() {
	if len(c.taskOrder) <= maxTaskHistory {
		return
	}

	keep := make([]string, 0, len(c.taskOrder))
	remainingDrop := len(c.taskOrder) - maxTaskHistory
	for _, sessionID := range c.taskOrder {
		rec := c.taskRecords[sessionID]
		if rec == nil {
			continue
		}
		if remainingDrop > 0 && rec.Status != deployTaskStatusQueued && rec.Status != deployTaskStatusInProgress {
			delete(c.taskRecords, sessionID)
			remainingDrop--
			continue
		}
		keep = append(keep, sessionID)
	}
	c.taskOrder = keep
}

func (c *Connection) submitTask(
	rec *deployTaskRecord,
	sourceAgent string,
	sendEvent func(level, text string),
	run func() (map[string]any, error),
) error {
	if rec == nil {
		return fmt.Errorf("empty task record")
	}
	if !c.reserveTask(rec) {
		return fmt.Errorf("deploy agent busy")
	}

	go func() {
		c.setTaskRunning(rec.SessionID)
		result, err := run()

		status := deployTaskStatusDone
		errMsg := ""
		if err != nil {
			status = deployTaskStatusError
			errMsg = err.Error()
		}

		finished := c.completeTask(rec.SessionID, status, errMsg, result)
		if err == nil {
			c.notifyBuildFlutterAPKSuccess(finished)
		}

		if sourceAgent != "" {
			payload := TaskCompletePayload{
				SessionID: rec.SessionID,
				Status:    SessionStatus(status),
				Error:     errMsg,
			}
			if sendErr := c.Client.SendTo(sourceAgent, MsgTaskComplete, payload); sendErr != nil {
				log.Printf("[WARN] send task_complete failed session=%s target=%s err=%v", rec.SessionID, sourceAgent, sendErr)
			}
		}
	}()

	return nil
}

func (c *Connection) sendTaskStreamEvent(sourceAgent, sessionID, level, text string) {
	if sourceAgent == "" {
		return
	}
	if err := c.Client.SendTo(sourceAgent, MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event:     StreamEvent{Type: level, Text: text},
	}); err != nil {
		log.Printf("[WARN] send stream_event failed session=%s target=%s err=%v", sessionID, sourceAgent, err)
	}
}

func (c *Connection) sendAppNotification(userID, content string) error {
	appAgentID := c.cfg.ResolveAgentID("app-agent")
	if appAgentID == "app-agent" {
		appAgentID = defaultAppAgentID
	}
	return c.Client.SendTo(appAgentID, uap.MsgNotify, uap.NotifyPayload{
		Channel: "app",
		To:      userID,
		Content: content,
	})
}

func (c *Connection) notifyBuildFlutterAPKSuccess(rec *deployTaskRecord) {
	if rec == nil || rec.Project != "build-flutter-apk" || rec.Status != deployTaskStatusDone {
		return
	}
	artifactFile := ""
	if rec.Result != nil {
		if v, _ := rec.Result["artifact_file"].(string); v != "" {
			artifactFile = v
		}
	}
	content := "build-flutter-apk 打包完成"
	if artifactFile != "" {
		content = fmt.Sprintf("build-flutter-apk 打包完成: %s", artifactFile)
	}
	if err := c.sendAppNotification(flutterAPKNotifyUser, content); err != nil {
		log.Printf("[WARN] send flutter apk completion notify failed user=%s err=%v", flutterAPKNotifyUser, err)
		return
	}
	log.Printf("[INFO] flutter apk completion notify sent user=%s content=%q", flutterAPKNotifyUser, content)
}
