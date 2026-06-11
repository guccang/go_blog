package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"uap"
)

type CortanaUserSnapshot struct {
	Account        string         `json:"account"`
	CollectedAt    int64          `json:"collected_at"`
	LocalDatetime  string         `json:"local_datetime"`
	Weekday        string         `json:"weekday"`
	Timezone       string         `json:"timezone"`
	TimezoneOffset string         `json:"timezone_offset"`
	TriggerReason  string         `json:"trigger_reason"`
	EventContext   map[string]any `json:"event_context,omitempty"`
	DeviceContext  map[string]any `json:"device_context,omitempty"`
	Blog           map[string]any `json:"blog,omitempty"`
	Todo           map[string]any `json:"todo,omitempty"`
	Exercise       map[string]any `json:"exercise,omitempty"`
	Reading        map[string]any `json:"reading,omitempty"`
	YearPlan       map[string]any `json:"year_plan,omitempty"`
	Goals          map[string]any `json:"goals,omitempty"`
	Projects       map[string]any `json:"projects,omitempty"`
	Agents         map[string]any `json:"agents,omitempty"`
	Memory         map[string]any `json:"memory,omitempty"`
}

type CortanaProactiveTaskPayload struct {
	TaskType           string              `json:"task_type"`
	Account            string              `json:"account"`
	Online             bool                `json:"online"`
	AllowFullAccess    bool                `json:"allow_full_access"`
	ProactiveMode      string              `json:"proactive_mode"`
	TriggerReason      string              `json:"trigger_reason"`
	Snapshot           CortanaUserSnapshot `json:"snapshot"`
	EventContext       map[string]any      `json:"event_context,omitempty"`
	RecentInteractions []map[string]any    `json:"recent_interactions,omitempty"`
}

type CortanaProactiveDecision struct {
	ShouldInteract    bool             `json:"should_interact"`
	Reason            string           `json:"reason"`
	Priority          string           `json:"priority"`
	SpeechText        string           `json:"speech_text"`
	SummaryText       string           `json:"summary_text"`
	Expression        string           `json:"expression"`
	Actions           []map[string]any `json:"actions"`
	SuggestedReplies  []map[string]any `json:"suggested_replies,omitempty"`
	FollowUpSuggested bool             `json:"follow_up_suggested"`
	NextCooldownSec   int              `json:"next_cooldown_sec"`
}

func (c *Connection) handleTaskComplete(msg *uap.Message) {
	var payload uap.TaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CortanaAgent] 解析 task_complete 失败: %v", err)
		return
	}
	c.monitor.HandleTaskComplete(msg)

	c.proactiveMu.Lock()
	ch := c.proactiveTasks[payload.TaskID]
	if ch != nil {
		delete(c.proactiveTasks, payload.TaskID)
	}
	c.proactiveMu.Unlock()
	if ch != nil {
		ch <- payload
	}
}

func (c *Connection) evaluateProactive(account string, result *MonitorResult) (BroadcastDecision, bool) {
	session := c.monitor.GetSession(account)
	if session == nil || !session.Registered {
		return BroadcastDecision{SkipReason: "session not registered"}, true
	}
	if !session.Enabled {
		return BroadcastDecision{SkipReason: "cortana disabled"}, true
	}
	if !session.Online {
		return BroadcastDecision{SkipReason: "user offline"}, true
	}
	if !session.AllowFullAccess {
		return BroadcastDecision{}, false
	}
	decision, err := c.requestProactiveDecision(session, result, CortanaTriggerEventPayload{
		Account:       session.Account,
		TriggerSource: "monitor",
		TriggerReason: "monitor_cycle",
		Timestamp:     time.Now().UnixMilli(),
	})
	if err != nil {
		log.Printf("[CortanaAgent] proactive decision failed account=%s err=%v", account, err)
		return BroadcastDecision{}, false
	}
	if !decision.ShouldInteract {
		return BroadcastDecision{SkipReason: defaultString(decision.Reason, "llm declined")}, true
	}
	return BroadcastDecision{
		ShouldBroadcast: true,
		BroadcastID:     "proactive_" + strings.TrimSpace(session.ProactiveMode),
		Text:            strings.TrimSpace(decision.SpeechText),
		Account:         account,
		Expression:      sanitizeExpression(decision.Expression),
		Motion:          sanitizeMotion(extractPrimaryMotion(decision.Actions)),
		ActionPlan:      buildCortanaBroadcastActionPlan(decision),
	}, true
}

func (c *Connection) requestProactiveDecision(session *CortanaUserSession, result *MonitorResult, event CortanaTriggerEventPayload) (*CortanaProactiveDecision, error) {
	snapshot := c.collectSnapshot(session, result, event)
	taskID := fmt.Sprintf("cortana_proactive_%s_%d", session.Account, time.Now().UnixMilli())
	payload := CortanaProactiveTaskPayload{
		TaskType:           "cortana_proactive",
		Account:            session.Account,
		Online:             session.Online,
		AllowFullAccess:    session.AllowFullAccess,
		ProactiveMode:      session.ProactiveMode,
		TriggerReason:      defaultString(event.TriggerReason, "monitor_cycle"),
		Snapshot:           snapshot,
		EventContext:       snapshot.EventContext,
		RecentInteractions: cloneInteractionList(session.RecentInteractions),
	}
	body, _ := json.Marshal(payload)
	ch := make(chan uap.TaskCompletePayload, 1)

	c.proactiveMu.Lock()
	c.proactiveTasks[taskID] = ch
	c.proactiveMu.Unlock()

	defer func() {
		c.proactiveMu.Lock()
		delete(c.proactiveTasks, taskID)
		c.proactiveMu.Unlock()
	}()

	if err := c.Client.SendTo(c.cfg.LLMAgentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  taskID,
		Payload: body,
	}); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Status != "success" {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		var decision CortanaProactiveDecision
		if err := json.Unmarshal([]byte(resp.Result), &decision); err != nil {
			return nil, err
		}
		return &decision, nil
	case <-time.After(25 * time.Second):
		return nil, fmt.Errorf("proactive decision timeout")
	}
}

func (c *Connection) collectSnapshot(session *CortanaUserSession, result *MonitorResult, event CortanaTriggerEventPayload) CortanaUserSnapshot {
	now := time.Now()
	account := session.Account
	date := now.Format("2006-01-02")
	year, month, _ := now.Date()

	triggerReason := defaultString(event.TriggerReason, "monitor_cycle")
	snapshot := CortanaUserSnapshot{
		Account:        account,
		CollectedAt:    now.UnixMilli(),
		LocalDatetime:  now.Format("2006-01-02 15:04:05"),
		Weekday:        chineseWeekday(now.Weekday()),
		Timezone:       localTimezoneName(now),
		TimezoneOffset: localTimezoneOffset(now),
		TriggerReason:  triggerReason,
		EventContext: map[string]any{
			"trigger_source": strings.TrimSpace(event.TriggerSource),
			"trigger_reason": triggerReason,
			"content":        strings.TrimSpace(event.Content),
			"summary":        strings.TrimSpace(event.Summary),
			"timestamp":      event.Timestamp,
			"metadata":       cloneAnyMap(event.Metadata),
		},
		DeviceContext: cloneAnyMap(session.CurrentContext),
		Blog: map[string]any{
			"blog_count": result.BlogCount,
			"new_blogs":  result.NewBlogs,
		},
		Todo: map[string]any{
			"todo_count":      result.TodoCount,
			"todo_overdue":    result.OverdueTodo,
			"monitor_checked": true,
		},
		Exercise: map[string]any{
			"exercise_min":   result.ExerciseMin,
			"exercise_count": result.ExerciseCnt,
		},
		Agents: map[string]any{
			"online_agents":  result.OnlineAgents,
			"offline_agents": result.OfflineAgents,
		},
	}

	snapshot.Todo["today_items"] = c.safeCallJSON("RawGetTodosByDate", map[string]any{
		"account": account,
		"date":    date,
	})
	snapshot.Exercise["stats_7d"] = c.safeCallValue("RawGetExerciseStats", map[string]any{
		"account": account,
		"days":    7,
	})
	snapshot.Exercise["today"] = c.safeCallJSON("RawGetExerciseByDate", map[string]any{
		"account": account,
		"date":    date,
	})
	snapshot.Reading = map[string]any{
		"stats": c.safeCallValue("RawGetReadingStats", map[string]any{
			"account": account,
		}),
		"reading_now": c.safeCallJSON("RawGetBooksByStatus", map[string]any{
			"account": account,
			"status":  "reading",
		}),
	}
	snapshot.YearPlan = map[string]any{
		"month_goal": c.safeCallJSON("RawGetMonthGoal", map[string]any{
			"account": account,
			"year":    year,
			"month":   int(month),
		}),
	}
	snapshot.Goals = map[string]any{
		"all_current": c.safeCallJSON("RawGetCurrentGoals", map[string]any{
			"account": account,
		}),
	}
	snapshot.Projects = map[string]any{
		"summary": c.safeCallJSON("RawGetProjectSummary", map[string]any{
			"account": account,
		}),
	}
	snapshot.Memory = c.collectMemory(account, event, result)
	return snapshot
}

// collectMemory 注入用户记忆库（借鉴 MiMo：可审阅 Markdown + 关键词检索）。
// 控制 token 预算：checkpoint 全文截断、MEMORY 截断、检索 top-5、今日 journal 用于查重。
func (c *Connection) collectMemory(account string, event CortanaTriggerEventPayload, result *MonitorResult) map[string]any {
	mem := map[string]any{}

	checkpoint := c.safeCallMemoryFile(account, "checkpoint")
	if checkpoint != "" {
		mem["checkpoint"] = truncateRunes(checkpoint, 800)
	}
	longTerm := c.safeCallMemoryFile(account, "MEMORY")
	if longTerm != "" {
		mem["long_term"] = truncateRunes(longTerm, 1200)
	}

	query := buildMemoryQuery(event, result)
	if query != "" {
		hits := c.safeCallValue("RawMemorySearch", map[string]any{
			"account":             account,
			"query":               query,
			"limit":               5,
			"recent_journal_days": 7,
		})
		if hits != nil {
			mem["search"] = map[string]any{"query": query, "hits": hits}
		}
	}

	// 今日 journal：供决策器查重，避免同一件事重复播报（“管家”与“闹钟”的区别）。
	todayJournal := c.safeCallMemoryFile(account, "journal_"+time.Now().Format("2006-01-02"))
	if todayJournal != "" {
		mem["today_journal"] = truncateRunes(todayJournal, 1000)
	}

	if len(mem) == 0 {
		return nil
	}
	return mem
}

// safeCallMemoryFile 读取单个记忆文件内容，失败返回空串。
func (c *Connection) safeCallMemoryFile(account, file string) string {
	val := c.safeCallValue("RawMemoryRead", map[string]any{"account": account, "file": file})
	if m, ok := val.(map[string]any); ok {
		if content, ok := m["content"].(string); ok {
			return strings.TrimSpace(content)
		}
	}
	return ""
}

// buildMemoryQuery 根据触发事件与监控信号拼检索关键词。
func buildMemoryQuery(event CortanaTriggerEventPayload, result *MonitorResult) string {
	var terms []string
	seen := map[string]bool{}
	add := func(s string) {
		for _, t := range strings.Fields(strings.TrimSpace(s)) {
			t = strings.Trim(t, "。，,.！!？?；;：:\"'")
			if len([]rune(t)) < 2 || seen[t] {
				continue
			}
			seen[t] = true
			terms = append(terms, t)
		}
	}
	add(event.Summary)
	add(event.Content)
	add(event.TriggerReason)
	if result != nil {
		if result.OverdueTodo > 0 {
			add("待办 逾期 提醒")
		}
		if result.ExerciseCnt == 0 {
			add("锻炼 运动")
		}
	}
	if len(terms) > 12 {
		terms = terms[:12]
	}
	return strings.Join(terms, " ")
}

// truncateRunes 按字符数截断，超出加省略标记。
func truncateRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "…(已截断)"
}

func chineseWeekday(w time.Weekday) string {
	switch w {
	case time.Monday:
		return "星期一"
	case time.Tuesday:
		return "星期二"
	case time.Wednesday:
		return "星期三"
	case time.Thursday:
		return "星期四"
	case time.Friday:
		return "星期五"
	case time.Saturday:
		return "星期六"
	case time.Sunday:
		return "星期日"
	default:
		return ""
	}
}

func localTimezoneName(t time.Time) string {
	name, _ := t.Zone()
	return name
}

func localTimezoneOffset(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func (c *Connection) handleProactiveEvent(event CortanaTriggerEventPayload) (BroadcastDecision, bool, error) {
	event.Account = strings.TrimSpace(event.Account)
	event.TriggerSource = strings.TrimSpace(event.TriggerSource)
	event.TriggerReason = defaultString(event.TriggerReason, event.TriggerSource)
	event.Content = strings.TrimSpace(event.Content)
	event.Summary = strings.TrimSpace(event.Summary)
	if event.Timestamp <= 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.Account == "" || event.TriggerSource == "" {
		log.Printf("[CortanaAgent] proactive event skipped account=%q source=%q reason=%q skip_reason=%s",
			event.Account, event.TriggerSource, event.TriggerReason, "invalid proactive event")
		return BroadcastDecision{SkipReason: "invalid proactive event"}, false, nil
	}
	c.updateCurrentContextFromEvent(event)
	c.recordIncomingInteraction(event)

	session := c.monitor.GetSession(event.Account)
	if session == nil || !session.Registered {
		log.Printf("[CortanaAgent] proactive event skipped account=%s source=%s reason=%s skip_reason=%s",
			event.Account, event.TriggerSource, event.TriggerReason, "session not registered")
		return BroadcastDecision{SkipReason: "session not registered"}, true, nil
	}
	if !session.Enabled {
		log.Printf("[CortanaAgent] proactive event skipped account=%s source=%s reason=%s skip_reason=%s",
			event.Account, event.TriggerSource, event.TriggerReason, "cortana disabled")
		return BroadcastDecision{SkipReason: "cortana disabled"}, true, nil
	}
	if !session.Online {
		log.Printf("[CortanaAgent] proactive event skipped account=%s source=%s reason=%s skip_reason=%s",
			event.Account, event.TriggerSource, event.TriggerReason, "user offline")
		return BroadcastDecision{SkipReason: "user offline"}, true, nil
	}
	result := c.monitor.CheckNow(session.Account)
	if !session.AllowFullAccess {
		decision := c.decider.Evaluate(result)
		if decision.ShouldBroadcast {
			log.Printf("[CortanaAgent] event fallback broadcast account=%s source=%s reason=%s text=%q",
				session.Account, event.TriggerSource, event.TriggerReason, decision.Text)
			c.executeBroadcastAndRecord(decision)
		}
		return decision, true, nil
	}

	decision, err := c.requestProactiveDecision(session, result, event)
	if err != nil {
		return BroadcastDecision{}, false, err
	}
	if !decision.ShouldInteract {
		log.Printf("[CortanaAgent] proactive event skipped account=%s source=%s reason=%s llm_reason=%s",
			session.Account, event.TriggerSource, event.TriggerReason, defaultString(decision.Reason, "llm declined"))
		return BroadcastDecision{SkipReason: defaultString(decision.Reason, "llm declined")}, true, nil
	}

	broadcast := BroadcastDecision{
		ShouldBroadcast: true,
		BroadcastID:     "event_" + strings.TrimSpace(event.TriggerReason),
		Text:            strings.TrimSpace(decision.SpeechText),
		Account:         session.Account,
		Expression:      sanitizeExpression(decision.Expression),
		Motion:          sanitizeMotion(extractPrimaryMotion(decision.Actions)),
		ActionPlan:      buildCortanaBroadcastActionPlan(decision),
	}
	log.Printf("[CortanaAgent] proactive event trigger account=%s source=%s reason=%s text=%q",
		session.Account, event.TriggerSource, event.TriggerReason, broadcast.Text)
	c.executeBroadcastAndRecord(broadcast)
	return broadcast, true, nil
}

func (c *Connection) executeBroadcastAndRecord(decision BroadcastDecision) {
	if !decision.ShouldBroadcast {
		return
	}
	c.monitor.ExecuteBroadcast(decision)
}

func (c *Connection) safeCallValue(tool string, args map[string]any) any {
	if c == nil || c.caller == nil {
		return nil
	}
	body, _ := json.Marshal(args)
	result, _, err := c.caller.CallTool(tool, body, 8*time.Second)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var parsed any
	if err := json.Unmarshal([]byte(result), &parsed); err == nil {
		return parsed
	}
	return strings.TrimSpace(result)
}

func (c *Connection) safeCallJSON(tool string, args map[string]any) any {
	return c.safeCallValue(tool, args)
}

func extractPrimaryMotion(actions []map[string]any) string {
	for _, action := range actions {
		if motion := strings.TrimSpace(fmt.Sprint(action["motion"])); motion != "" {
			return motion
		}
	}
	return "IdleWave"
}

func buildCortanaBroadcastActionPlan(decision *CortanaProactiveDecision) map[string]any {
	if decision == nil {
		return nil
	}
	out := map[string]any{
		"speech_text":         strings.TrimSpace(decision.SpeechText),
		"expression":          sanitizeExpression(decision.Expression),
		"fallback_expression": "happy",
		"expression_hold_ms":  1800,
	}
	if mood := strings.TrimSpace(decision.Priority); mood != "" {
		out["mood"] = mood
	}
	if len(decision.Actions) > 0 {
		out["actions"] = decision.Actions
	}
	if len(decision.SuggestedReplies) > 0 {
		out["suggested_replies"] = decision.SuggestedReplies
	}
	return out
}

func sanitizeExpression(expression string) string {
	switch strings.ToLower(strings.TrimSpace(expression)) {
	case "sad":
		return "sad"
	case "surprised":
		return "surprised"
	default:
		return "happy"
	}
}

func sanitizeMotion(motion string) string {
	switch strings.TrimSpace(motion) {
	case "Idle", "IdleAlt", "IdleWave", "Tap":
		return strings.TrimSpace(motion)
	default:
		return "IdleWave"
	}
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneInteractionList(in []map[string]any) []map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		out = append(out, cloneAnyMap(item))
	}
	return out
}

func (c *Connection) recordIncomingInteraction(event CortanaTriggerEventPayload) {
	if c == nil || c.monitor == nil {
		return
	}
	c.monitor.AppendInteraction(event.Account, map[string]any{
		"role":           "user",
		"kind":           "event",
		"trigger_source": strings.TrimSpace(event.TriggerSource),
		"trigger_reason": defaultString(event.TriggerReason, event.TriggerSource),
		"summary":        shortInteractionText(event.Summary, event.Content),
		"content":        strings.TrimSpace(event.Content),
		"timestamp":      event.Timestamp,
	})
}

func (c *Connection) updateCurrentContextFromEvent(event CortanaTriggerEventPayload) {
	if c == nil || c.monitor == nil || len(event.Metadata) == 0 {
		return
	}
	raw, ok := event.Metadata["device_context"].(map[string]any)
	if !ok || len(raw) == 0 {
		return
	}
	c.monitor.UpdateCurrentContext(event.Account, raw)
}

func (c *Connection) recordBroadcastInteraction(decision BroadcastDecision) {
	if c == nil || c.monitor == nil {
		return
	}
	c.monitor.AppendInteraction(decision.Account, map[string]any{
		"role":         "assistant",
		"kind":         "broadcast",
		"broadcast_id": strings.TrimSpace(decision.BroadcastID),
		"summary":      shortInteractionText("", decision.Text),
		"content":      strings.TrimSpace(decision.Text),
		"expression":   strings.TrimSpace(decision.Expression),
		"motion":       strings.TrimSpace(decision.Motion),
		"timestamp":    time.Now().UnixMilli(),
	})
	c.writeBroadcastJournal(decision)
}

// writeBroadcastJournal 把本次播报写入当日 journal，作为后续“提醒查重”的依据。
// 异步执行，避免阻塞播报链路；测试语音不入库以免污染记忆。
func (c *Connection) writeBroadcastJournal(decision BroadcastDecision) {
	account := strings.TrimSpace(decision.Account)
	text := strings.TrimSpace(decision.Text)
	broadcastID := strings.TrimSpace(decision.BroadcastID)
	if account == "" || text == "" || broadcastID == "test_voice" {
		return
	}
	entry := fmt.Sprintf("[播报] %s", text)
	if broadcastID != "" {
		entry = fmt.Sprintf("[播报:%s] %s", broadcastID, text)
	}
	go func() {
		val := c.safeCallValue("RawMemoryJournal", map[string]any{
			"account": account,
			"content": entry,
		})
		if m, ok := val.(map[string]any); ok {
			if errMsg, ok := m["error"].(string); ok && errMsg != "" {
				log.Printf("[CortanaAgent] 写播报 journal 失败 account=%s err=%s", account, errMsg)
			}
		}
	}()
}

func shortInteractionText(summary, content string) string {
	summary = strings.TrimSpace(summary)
	if summary != "" {
		return summary
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	rs := []rune(content)
	if len(rs) <= 48 {
		return content
	}
	return string(rs[:48]) + "..."
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
