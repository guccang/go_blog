package main

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BroadcastDecision 播报决策结果
type BroadcastDecision struct {
	ShouldBroadcast bool   `json:"should_broadcast"`
	BroadcastID     string `json:"broadcast_id"`
	Text            string `json:"text"`
	Account         string `json:"account"`
	Expression      string `json:"expression"`
	Motion          string `json:"motion"`
	SkipReason      string `json:"skip_reason,omitempty"`
}

// broadcastState 单个播报场景的运行时状态
type broadcastState struct {
	LastBroadcastAt time.Time `json:"last_broadcast_at"`
	TodayCount      int       `json:"today_count"`
	TodayDate       string    `json:"today_date"` // "2006-01-02" 格式，用于重置每日计数
}

// BroadcastDecider 播报决策器：评估数据变化是否触发播报
type BroadcastDecider struct {
	cfg    *Config
	mu     sync.Mutex
	states map[string]*broadcastState // account:broadcastID → state
}

// NewBroadcastDecider 创建播报决策器
func NewBroadcastDecider(cfg *Config) *BroadcastDecider {
	bd := &BroadcastDecider{
		cfg:    cfg,
		states: make(map[string]*broadcastState),
	}

	return bd
}

// Evaluate 评估监控结果，决定是否播报
func (bd *BroadcastDecider) Evaluate(result *MonitorResult) BroadcastDecision {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	account := strings.TrimSpace(result.Account)

	for _, bc := range bd.cfg.Broadcasts {
		stateKey := makeBroadcastStateKey(account, bc.ID)
		state := bd.states[stateKey]
		if state == nil {
			state = &broadcastState{}
			bd.states[stateKey] = state
		}

		// 重置每日计数
		if state.TodayDate != today {
			state.TodayCount = 0
			state.TodayDate = today
		}

		// 检查每日最大次数
		if bc.MaxPerDay > 0 && state.TodayCount >= bc.MaxPerDay {
			continue
		}

		// 检查冷却时间
		if !state.LastBroadcastAt.IsZero() {
			cooldown := time.Duration(bd.cfg.BroadcastCooldownSec) * time.Second
			if time.Since(state.LastBroadcastAt) < cooldown {
				continue
			}
		}

		// 根据触发类型评估
		decision := bd.evaluateBroadcast(bc, result, state)
		if decision.ShouldBroadcast {
			if decision.Account == "" {
				decision.Account = account
			}
			return decision
		}
	}

	return BroadcastDecision{
		ShouldBroadcast: false,
		SkipReason:      "no matching broadcast conditions",
	}
}

// evaluateBroadcast 评估单个播报场景
func (bd *BroadcastDecider) evaluateBroadcast(
	bc BroadcastConfig,
	result *MonitorResult,
	state *broadcastState,
) BroadcastDecision {
	switch bc.Trigger {
	case "time_range":
		return bd.evalTimeRange(bc, result)
	case "data_change":
		return bd.evalDataChange(bc, result)
	case "interval":
		return bd.evalInterval(bc, state)
	default:
		return BroadcastDecision{SkipReason: "unknown trigger type: " + bc.Trigger}
	}
}

// evalTimeRange 评估时间范围触发
func (bd *BroadcastDecider) evalTimeRange(bc BroadcastConfig, result *MonitorResult) BroadcastDecision {
	if bc.TimeStart == "" || bc.TimeEnd == "" {
		return BroadcastDecision{SkipReason: "time range not configured"}
	}

	startMin, ok1 := parseTimeMinutes(bc.TimeStart)
	endMin, ok2 := parseTimeMinutes(bc.TimeEnd)
	if !ok1 || !ok2 {
		return BroadcastDecision{SkipReason: "invalid time format"}
	}

	now := time.Now()
	curMin := now.Hour()*60 + now.Minute()

	inRange := false
	if startMin <= endMin {
		inRange = curMin >= startMin && curMin <= endMin
	} else {
		// 跨午夜
		inRange = curMin >= startMin || curMin <= endMin
	}

	if !inRange {
		return BroadcastDecision{SkipReason: "outside time range"}
	}

	// 渲染模板
	text := bd.renderTemplate(bc.Template, result)

	return BroadcastDecision{
		ShouldBroadcast: true,
		BroadcastID:     bc.ID,
		Text:            text,
		Account:         result.Account,
		Expression:      emptyDefault(bc.Expression, "happy"),
		Motion:          emptyDefault(bc.Motion, "IdleWave"),
	}
}

// evalDataChange 评估数据变化触发
func (bd *BroadcastDecider) evalDataChange(bc BroadcastConfig, result *MonitorResult) BroadcastDecision {
	if len(bc.DataSources) == 0 {
		return BroadcastDecision{SkipReason: "no data sources configured"}
	}

	// 检查每个数据源是否达到阈值
	dataValues := bd.collectDataValues(bc.DataSources, result)
	if dataValues == nil {
		return BroadcastDecision{SkipReason: "failed to collect data values"}
	}

	// 检查是否有任何一个值达到 min_count
	triggered := false
	for _, source := range bc.DataSources {
		if val, ok := dataValues[source]; ok && val >= bc.MinCount {
			triggered = true
			break
		}
	}

	if !triggered {
		return BroadcastDecision{SkipReason: "no data source reached min_count"}
	}

	// 渲染模板（注入数据值到模板变量）
	text := bd.renderTemplateWithData(bc.Template, dataValues, result)

	return BroadcastDecision{
		ShouldBroadcast: true,
		BroadcastID:     bc.ID,
		Text:            text,
		Account:         result.Account,
		Expression:      emptyDefault(bc.Expression, "surprised"),
		Motion:          emptyDefault(bc.Motion, "Tap"),
	}
}

// evalInterval 评估间隔触发
func (bd *BroadcastDecider) evalInterval(bc BroadcastConfig, state *broadcastState) BroadcastDecision {
	if state.LastBroadcastAt.IsZero() {
		// 首次检查，不触发（等待一个完整间隔）
		return BroadcastDecision{SkipReason: "first check, waiting for interval"}
	}

	interval := time.Duration(bc.IntervalSec) * time.Second
	if time.Since(state.LastBroadcastAt) >= interval {
		return BroadcastDecision{
			ShouldBroadcast: true,
			BroadcastID:     bc.ID,
			Text:            bc.Template,
			Account:         "",
			Expression:      emptyDefault(bc.Expression, "happy"),
			Motion:          emptyDefault(bc.Motion, "Idle"),
		}
	}

	return BroadcastDecision{SkipReason: "interval not reached"}
}

// collectDataValues 从监控结果中收集数据源的值
func (bd *BroadcastDecider) collectDataValues(sources []string, result *MonitorResult) map[string]int {
	values := make(map[string]int, len(sources))

	for _, source := range sources {
		switch source {
		case "blog_new_count":
			values[source] = result.NewBlogs
		case "blog_count":
			values[source] = result.BlogCount
		case "todo_count":
			values[source] = result.TodoCount
		case "todo_overdue_count":
			values[source] = result.OverdueTodo
		case "exercise_min":
			values[source] = result.ExerciseMin
		case "exercise_count":
			values[source] = result.ExerciseCnt
		case "agent_offline":
			values[source] = len(result.OfflineAgents)
		default:
			// 未知数据源，默认值为 0
			values[source] = 0
		}
	}

	return values
}

// renderTemplate 渲染播报模板，替换 {{变量}} 占位符
func (bd *BroadcastDecider) renderTemplate(template string, result *MonitorResult) string {
	text := template
	text = strings.ReplaceAll(text, "{{blog_new_count}}", itoa(result.NewBlogs))
	text = strings.ReplaceAll(text, "{{blog_count}}", itoa(result.BlogCount))
	text = strings.ReplaceAll(text, "{{todo_count}}", itoa(result.TodoCount))
	text = strings.ReplaceAll(text, "{{todo_overdue_count}}", itoa(result.OverdueTodo))
	text = strings.ReplaceAll(text, "{{exercise_min}}", itoa(result.ExerciseMin))
	text = strings.ReplaceAll(text, "{{exercise_count}}", itoa(result.ExerciseCnt))
	text = strings.ReplaceAll(text, "{{exercise_completed}}", itoa(result.ExerciseCnt))

	if len(result.OfflineAgents) > 0 {
		text = strings.ReplaceAll(text, "{{offline_agent_name}}", result.OfflineAgents[0])
	}

	return text
}

// renderTemplateWithData 使用显式数据值渲染模板
func (bd *BroadcastDecider) renderTemplateWithData(
	template string,
	data map[string]int,
	result *MonitorResult,
) string {
	text := template
	for key, val := range data {
		text = strings.ReplaceAll(text, "{{"+key+"}}", itoa(val))
	}
	// 也替换标准变量
	text = bd.renderTemplate(text, result)
	return text
}

// RecordBroadcast 记录一次播报（更新冷却和每日计数）
func (bd *BroadcastDecider) RecordBroadcast(account, broadcastID string) {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	stateKey := makeBroadcastStateKey(account, broadcastID)
	state := bd.states[stateKey]
	if state == nil {
		state = &broadcastState{}
		bd.states[stateKey] = state
	}

	now := time.Now()
	today := now.Format("2006-01-02")

	if state.TodayDate != today {
		state.TodayCount = 0
		state.TodayDate = today
	}

	state.LastBroadcastAt = now
	state.TodayCount++

	log.Printf("[Broadcast] 记录播报 account=%s broadcast_id=%s today_count=%d/%d last_at=%s",
		account, broadcastID, state.TodayCount,
		bd.getMaxPerDay(broadcastID),
		now.Format("15:04:05"))
}

// getMaxPerDay 获取某个播报场景的每日最大次数
func (bd *BroadcastDecider) getMaxPerDay(broadcastID string) int {
	for _, bc := range bd.cfg.Broadcasts {
		if bc.ID == broadcastID {
			return bc.MaxPerDay
		}
	}
	return 0
}

// AllStates 返回所有播报场景的状态信息
func (bd *BroadcastDecider) AllStates() map[string]string {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	states := make(map[string]string, len(bd.cfg.Broadcasts)+len(bd.states))
	today := time.Now().Format("2006-01-02")

	for _, bc := range bd.cfg.Broadcasts {
		states[bc.ID] = "available"
	}

	for id, state := range bd.states {
		if state == nil {
			states[id] = "uninitialized"
			continue
		}

		if state.TodayDate != today {
			states[id] = "available"
			continue
		}

		maxPerDay := bd.getMaxPerDay(id)
		if maxPerDay > 0 && state.TodayCount >= maxPerDay {
			states[id] = "maxed_out_today"
			continue
		}

		cooldown := time.Duration(bd.cfg.BroadcastCooldownSec) * time.Second
		if !state.LastBroadcastAt.IsZero() && time.Since(state.LastBroadcastAt) < cooldown {
			states[id] = "cooling_down"
			continue
		}

		states[id] = "available"
	}

	return states
}

func makeBroadcastStateKey(account, broadcastID string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return broadcastID
	}
	return account + ":" + broadcastID
}

// ========================= 辅助函数 =========================

func emptyDefault(val, def string) string {
	if strings.TrimSpace(val) == "" {
		return def
	}
	return val
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	return strconv.Itoa(n)
}
