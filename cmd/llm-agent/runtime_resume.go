package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

type ResumeRuntime struct {
	orchestrator  *Orchestrator
	rootSessionID string
	tools         []LLMTool
	sendEvent     func(event, text string)
	start         time.Time

	rootSession      *TaskSession
	children         map[string]*TaskSession
	completedResults map[string]string
	pendingSubTasks  []SubTaskPlan
}

func (o *Orchestrator) Resume(
	rootSessionID string,
	tools []LLMTool,
	sendEvent func(event, text string),
) (string, error) {
	rt := &ResumeRuntime{
		orchestrator:     o,
		rootSessionID:    rootSessionID,
		tools:            tools,
		sendEvent:        sendEvent,
		start:            time.Now(),
		completedResults: make(map[string]string),
	}
	return rt.Run()
}

func (rt *ResumeRuntime) Run() (string, error) {
	log.Printf("[Resume] ▶ 开始恢复 rootSessionID=%s", rt.rootSessionID)
	rt.sendEvent("resume", "正在恢复任务...")

	if err := rt.loadTree(); err != nil {
		return "", err
	}
	if rt.rootSession.Plan == nil {
		return "", fmt.Errorf("root session has no plan")
	}

	rt.collectResumeState()
	rt.resumePendingSubTasks()
	return rt.summarize()
}

func (rt *ResumeRuntime) loadTree() error {
	rootSession, children, err := rt.orchestrator.store.LoadTree(rt.rootSessionID)
	if err != nil {
		return fmt.Errorf("load session tree: %v", err)
	}
	rt.rootSession = rootSession
	rt.children = children
	log.Printf("[Resume] 加载会话树 subtasks=%d children=%d", len(rootSession.ChildIDs), len(children))
	return nil
}

func (rt *ResumeRuntime) collectResumeState() {
	for _, subtask := range rt.rootSession.Plan.SubTasks {
		child, ok := rt.children[subtask.ID]
		if !ok {
			rt.pendingSubTasks = append(rt.pendingSubTasks, subtask)
			continue
		}

		switch child.Status {
		case "done":
			rt.completedResults[subtask.ID] = child.Result
			rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 已完成，跳过", subtask.ID, subtask.Title))
		case "running":
			rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 从断点恢复", subtask.ID, subtask.Title))
			siblingContext := buildSiblingContext(subtask.DependsOn, rt.completedResults)
			result := rt.orchestrator.resumeSubTask(child, subtask, siblingContext, rt.tools, rt.sendEvent)
			if result.Status == "done" {
				rt.completedResults[subtask.ID] = result.Result
			}
		case "failed":
			if rt.shouldRetryFailedSubTask(subtask.ID) {
				rt.pendingSubTasks = append(rt.pendingSubTasks, subtask)
			} else {
				rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前已失败，跳过", subtask.ID, subtask.Title))
			}
		case "pending":
			rt.pendingSubTasks = append(rt.pendingSubTasks, subtask)
		case "skipped":
			rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前已跳过", subtask.ID, subtask.Title))
		case "async":
			rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前为异步，重新评估", subtask.ID, subtask.Title))
			rt.pendingSubTasks = append(rt.pendingSubTasks, subtask)
		case "deferred":
			rt.sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前被推迟，重新评估", subtask.ID, subtask.Title))
			rt.pendingSubTasks = append(rt.pendingSubTasks, subtask)
		}
	}
}

func (rt *ResumeRuntime) shouldRetryFailedSubTask(subTaskID string) bool {
	for _, d := range rt.rootSession.FailureDecisions {
		if d.SubTaskID == subTaskID && d.Action == "retry" {
			return true
		}
	}
	return false
}

func (rt *ResumeRuntime) resumePendingSubTasks() {
	if len(rt.pendingSubTasks) == 0 {
		return
	}

	rt.sendEvent("resume_info", fmt.Sprintf("继续执行 %d 个未完成的子任务", len(rt.pendingSubTasks)))
	for _, subtask := range rt.pendingSubTasks {
		child, ok := rt.children[subtask.ID]
		if !ok {
			child = NewChildSession(rt.rootSession, subtask.Title, subtask.Description)
			child.ID = subtask.ID
			rt.children[subtask.ID] = child
			rt.rootSession.AddChildID(subtask.ID)
		}

		siblingContext := buildSiblingContext(subtask.DependsOn, rt.completedResults)
		taskIdx := indexOf(rt.rootSession.Plan.SubTasks, subtask.ID)
		rt.sendEvent("subtask_start", fmt.Sprintf("[%d/%d] %s", taskIdx+1, len(rt.rootSession.Plan.SubTasks), subtask.Title))
		result := rt.orchestrator.executeSubTask(context.Background(), "", subtask, child, siblingContext, rt.tools, rt.sendEvent, nil)
		if result.Status == "done" {
			rt.completedResults[subtask.ID] = result.Result
			rt.sendEvent("subtask_done", fmt.Sprintf("[%s] %s — 完成", subtask.ID, subtask.Title))
		}
	}
}

func (rt *ResumeRuntime) summarize() (string, error) {
	allResults := buildSubTaskResults(rt.rootSession.Plan, rt.children)
	summary := rt.orchestrator.Synthesize(rt.rootSession, rt.children, allResults, rt.rootSession.Title, rt.sendEvent)
	assistantRecord := buildPersistentAssistantRecord(AssistantRecordInput{
		Query:         rt.rootSession.Title,
		DisplayResult: summary,
		Status:        "done",
		RootSession:   rt.rootSession,
		ChildSessions: rt.children,
		Results:       allResults,
	})
	finalizeRootSession(rt.orchestrator.store, rt.rootSession, "done", summary, assistantRecord, rt.children)

	log.Printf("[Resume] ◀ 恢复完成 duration=%v summaryLen=%d", time.Since(rt.start), len(summary))
	return summary, nil
}
