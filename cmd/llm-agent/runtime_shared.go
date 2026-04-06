package main

func childSessionList(children map[string]*TaskSession) []*TaskSession {
	childList := make([]*TaskSession, 0, len(children))
	for _, c := range children {
		childList = append(childList, c)
	}
	return childList
}

func buildSubTaskResults(plan *TaskPlan, children map[string]*TaskSession) []SubTaskResult {
	if plan == nil {
		return nil
	}

	results := make([]SubTaskResult, 0, len(plan.SubTasks))
	for _, subtask := range plan.SubTasks {
		child, ok := children[subtask.ID]
		if !ok {
			continue
		}
		results = append(results, SubTaskResult{
			SubTaskID:     subtask.ID,
			Title:         subtask.Title,
			Status:        child.Status,
			Result:        child.Result,
			Error:         child.Error,
			AsyncSessions: dedupeAsyncSessions(detectAsyncResults(child)),
		})
	}
	return results
}

func finalizeRootSession(store *SessionStore, root *TaskSession, status, summary, assistantRecord string, children map[string]*TaskSession) {
	root.SetStatus(status)
	root.SetResult(summary)
	root.Summary = summary
	appendFinalAssistantRecord(root, assistantRecord)
	if store != nil {
		store.Save(root)
		store.SaveIndex(root, childSessionList(children))
	}
}
