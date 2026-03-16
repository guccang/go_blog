package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"uap"
)

// ========================= 事件类别常量 =========================

const (
	EventKindMsgIn     = "msg_in"        // 收到消息
	EventKindMsgOut    = "msg_out"       // 转发消息
	EventKindMsgDrop   = "msg_drop"      // 消息丢弃（无 To 或未注册）
	EventKindAgentOn   = "agent_online"  // agent 上线
	EventKindAgentOff  = "agent_offline" // agent 下线
	EventKindHBTimeout = "hb_timeout"    // 心跳超时
	EventKindRouteErr  = "route_error"   // 路由失败（目标离线）
)

// ========================= Event 数据模型 =========================

// Event 单条追踪事件
type Event struct {
	Seq            uint64 `json:"seq"`                   // 单调递增序号
	Ts             int64  `json:"ts"`                    // unix 毫秒
	Kind           string `json:"kind"`                  // 事件类别
	TraceID        string `json:"trace_id"`              // 关联 ID
	MsgID          string `json:"msg_id,omitempty"`      // 原始 Message.ID
	From           string `json:"from,omitempty"`        // 源 agent ID
	FromName       string `json:"from_name,omitempty"`   // 源 agent 名称
	To             string `json:"to,omitempty"`          // 目标 agent ID
	ToName         string `json:"to_name,omitempty"`     // 目标 agent 名称
	MsgType        string `json:"msg_type,omitempty"`    // 消息类型
	PayloadSummary string `json:"summary,omitempty"`     // 摘要
	DurationMs     int64  `json:"duration_ms,omitempty"` // 耗时
	Error          string `json:"error,omitempty"`       // 错误信息
}

// ========================= EventQuery 查询参数 =========================

// EventQuery 事件查询条件
type EventQuery struct {
	TraceID string // 按 trace_id 过滤
	Agent   string // 按 agent ID 过滤（from 或 to）
	Kind    string // 按事件类别过滤
	MsgType string // 按消息类型过滤
	Since   int64  // 起始时间（unix 毫秒）
	Until   int64  // 截止时间（unix 毫秒）
	Limit   int    // 返回上限
	Offset  int    // 跳过前 N 条
}

// ========================= RingBuffer 环形缓冲区 =========================

// RingBuffer 固定大小的环形事件缓冲区
type RingBuffer struct {
	mu    sync.RWMutex
	buf   []Event
	size  int
	head  int    // 下一个写入位置
	count int    // 当前有效数据量（≤size）
	seq   uint64 // 单调递增计数器（atomic）
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 10000
	}
	return &RingBuffer{
		buf:  make([]Event, size),
		size: size,
	}
}

// Push 写入一条事件，返回分配的 seq
func (rb *RingBuffer) Push(e *Event) uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	seq := atomic.AddUint64(&rb.seq, 1)
	e.Seq = seq
	rb.buf[rb.head] = *e
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
	return seq
}

// Query 按条件过滤查询
func (rb *RingBuffer) Query(q *EventQuery) ([]Event, int) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	var matched []Event
	total := 0

	// 从最旧到最新遍历
	start := 0
	length := rb.count
	if rb.count == rb.size {
		start = rb.head // 环形缓冲区已满，head 指向最旧
	}

	for i := 0; i < length; i++ {
		idx := (start + i) % rb.size
		e := &rb.buf[idx]

		if !matchEvent(e, q) {
			continue
		}
		total++
		if total <= q.Offset {
			continue
		}
		if len(matched) < limit {
			matched = append(matched, *e)
		}
	}

	return matched, total
}

// Latest 返回最近 N 条事件（从新到旧）
func (rb *RingBuffer) Latest(n int) []Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || n > rb.count {
		n = rb.count
	}
	result := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.size) % rb.size
		result[i] = rb.buf[idx]
	}
	return result
}

// Stats 聚合统计
func (rb *RingBuffer) Stats() (byKind map[string]int64, byMsgType map[string]int64, byAgent map[string]int64) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	byKind = make(map[string]int64)
	byMsgType = make(map[string]int64)
	byAgent = make(map[string]int64)

	start := 0
	if rb.count == rb.size {
		start = rb.head
	}
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.size
		e := &rb.buf[idx]
		byKind[e.Kind]++
		if e.MsgType != "" {
			byMsgType[e.MsgType]++
		}
		if e.From != "" {
			byAgent[e.From]++
		}
	}
	return
}

// Used 返回缓冲区中有效事件数
func (rb *RingBuffer) Used() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// TotalRecorded 返回已记录的事件总数
func (rb *RingBuffer) TotalRecorded() uint64 {
	return atomic.LoadUint64(&rb.seq)
}

// matchEvent 判断事件是否符合查询条件
func matchEvent(e *Event, q *EventQuery) bool {
	if q.TraceID != "" && e.TraceID != q.TraceID {
		return false
	}
	if q.Agent != "" && e.From != q.Agent && e.To != q.Agent {
		return false
	}
	if q.Kind != "" && e.Kind != q.Kind {
		return false
	}
	if q.MsgType != "" && e.MsgType != q.MsgType {
		return false
	}
	if q.Since > 0 && e.Ts < q.Since {
		return false
	}
	if q.Until > 0 && e.Ts > q.Until {
		return false
	}
	return true
}

// ========================= JSONLWriter 文件写入 =========================

// JSONLWriter 按天 JSONL 日志写入
type JSONLWriter struct {
	mu      sync.Mutex
	dir     string
	current *os.File
	today   string
}

// NewJSONLWriter 创建 JSONL 写入器
func NewJSONLWriter(dir string) (*JSONLWriter, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create event log dir: %w", err)
	}
	return &JSONLWriter{dir: dir}, nil
}

// Write 写入一条事件到 JSONL 文件
func (w *JSONLWriter) Write(e *Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if w.current == nil || w.today != today {
		if w.current != nil {
			w.current.Close()
		}
		path := filepath.Join(w.dir, fmt.Sprintf("events_%s.jsonl", today))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[Tracker] open event log failed: %v", err)
			return
		}
		w.current = f
		w.today = today
	}

	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	w.current.Write(data)
	w.current.Write([]byte("\n"))
}

// Close 关闭文件
func (w *JSONLWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		w.current.Close()
		w.current = nil
	}
}

// ========================= TrackerConfig =========================

// TrackerConfig Tracker 配置
type TrackerConfig struct {
	BufferSize int    // 环形缓冲区大小
	LogDir     string // JSONL 日志目录
	LogStdout  bool   // 是否输出到终端
	SkipHB     bool   // 是否跳过 heartbeat 事件
}

// ========================= Tracker 主体 =========================

// Tracker 全量事件追踪器
type Tracker struct {
	ring       *RingBuffer
	writer     *JSONLWriter
	logStdout  bool
	skipHB     bool
	mu         sync.RWMutex
	taskTraces map[string]string // taskID → traceID
	traceStart map[string]int64  // traceID → 起始时间戳(ms)
	agentNames map[string]string // agentID → name 缓存
}

// NewTracker 创建事件追踪器
func NewTracker(cfg *TrackerConfig) (*Tracker, error) {
	writer, err := NewJSONLWriter(cfg.LogDir)
	if err != nil {
		return nil, err
	}

	t := &Tracker{
		ring:       NewRingBuffer(cfg.BufferSize),
		writer:     writer,
		logStdout:  cfg.LogStdout,
		skipHB:     cfg.SkipHB,
		taskTraces: make(map[string]string),
		traceStart: make(map[string]int64),
		agentNames: make(map[string]string),
	}

	// 启动 taskTraces 清理协程（1 小时 TTL）
	go t.cleanupLoop()

	return t, nil
}

// Record 主入口：写入 ring + jsonl + stdout
func (t *Tracker) Record(e *Event) {
	// 跳过 heartbeat
	if t.skipHB && (e.MsgType == uap.MsgHeartbeat || e.MsgType == uap.MsgHeartbeatAck) {
		return
	}

	// 填充 agent 名称
	t.mu.RLock()
	if e.FromName == "" && e.From != "" {
		e.FromName = t.agentNames[e.From]
	}
	if e.ToName == "" && e.To != "" {
		e.ToName = t.agentNames[e.To]
	}
	t.mu.RUnlock()

	// 写入环形缓冲区
	t.ring.Push(e)

	// 写入 JSONL 文件
	t.writer.Write(e)

	// 终端日志
	if t.logStdout {
		t.logToStdout(e)
	}
}

// RecordMessage 从 UAP 消息构建 Event 并记录
func (t *Tracker) RecordMessage(kind string, from *uap.AgentConn, to *uap.AgentConn, msg *uap.Message) {
	now := time.Now().UnixMilli()

	e := &Event{
		Ts:      now,
		Kind:    kind,
		MsgID:   msg.ID,
		MsgType: msg.Type,
	}

	// 填充 from
	if from != nil {
		e.From = from.ID
		e.FromName = from.Name
	} else if msg.From != "" {
		e.From = msg.From
	}

	// 填充 to
	if to != nil {
		e.To = to.ID
		e.ToName = to.Name
	} else if msg.To != "" {
		e.To = msg.To
	}

	// 解析 traceID
	e.TraceID = t.resolveTraceID(msg)

	// 提取摘要
	e.PayloadSummary = t.extractSummary(msg)

	// 计算 duration（响应消息）
	e.DurationMs = t.computeDuration(e.TraceID, msg)

	t.Record(e)
}

// RecordLifecycle 记录生命周期事件
func (t *Tracker) RecordLifecycle(kind string, agent *uap.AgentConn, detail string) {
	now := time.Now().UnixMilli()

	e := &Event{
		Ts:             now,
		Kind:           kind,
		TraceID:        agent.ID,
		From:           agent.ID,
		FromName:       agent.Name,
		PayloadSummary: detail,
	}

	// 缓存 agent 名称
	if kind == EventKindAgentOn {
		t.mu.Lock()
		t.agentNames[agent.ID] = agent.Name
		t.mu.Unlock()
	} else if kind == EventKindAgentOff {
		t.mu.Lock()
		delete(t.agentNames, agent.ID)
		t.mu.Unlock()
	}

	t.Record(e)
}

// Query 按条件查询事件
func (t *Tracker) Query(q *EventQuery) ([]Event, int) {
	return t.ring.Query(q)
}

// GetTrace 获取完整调用链
func (t *Tracker) GetTrace(traceID string) []Event {
	events, _ := t.ring.Query(&EventQuery{
		TraceID: traceID,
		Limit:   1000,
	})
	return events
}

// Stats 统计信息
func (t *Tracker) Stats() map[string]any {
	byKind, byMsgType, byAgent := t.ring.Stats()

	t.mu.RLock()
	activeTraces := len(t.traceStart)
	t.mu.RUnlock()

	return map[string]any{
		"buffer_capacity": t.ring.size,
		"buffer_used":     t.ring.Used(),
		"total_recorded":  t.ring.TotalRecorded(),
		"by_kind":         byKind,
		"by_msg_type":     byMsgType,
		"by_agent":        byAgent,
		"active_traces":   activeTraces,
	}
}

// Close 关闭追踪器
func (t *Tracker) Close() {
	t.writer.Close()
}

// ========================= Trace ID 关联 =========================

// resolveTraceID 根据消息类型解析 traceID
func (t *Tracker) resolveTraceID(msg *uap.Message) string {
	switch msg.Type {
	case uap.MsgToolCall:
		// 新调用链起点
		traceID := msg.ID
		if traceID != "" {
			t.mu.Lock()
			t.traceStart[traceID] = time.Now().UnixMilli()
			t.mu.Unlock()
		}
		return traceID

	case uap.MsgToolResult:
		// 关联到原始 tool_call
		reqID := extractRequestID(msg.Payload)
		if reqID != "" {
			return reqID
		}
		return msg.ID

	case uap.MsgTaskAssign:
		// 新任务链起点
		traceID := msg.ID
		taskID := extractTaskID(msg.Payload)
		if taskID != "" && traceID != "" {
			t.mu.Lock()
			t.taskTraces[taskID] = traceID
			t.traceStart[traceID] = time.Now().UnixMilli()
			t.mu.Unlock()
		}
		return traceID

	case uap.MsgTaskAccepted, uap.MsgTaskRejected, uap.MsgTaskEvent, uap.MsgTaskComplete, uap.MsgTaskStop:
		taskID := extractTaskID(msg.Payload)
		if taskID != "" {
			t.mu.RLock()
			traceID := t.taskTraces[taskID]
			t.mu.RUnlock()
			if traceID != "" {
				return traceID
			}
		}
		return msg.ID

	case uap.MsgRegister:
		return msg.ID

	case uap.MsgHeartbeat, uap.MsgHeartbeatAck:
		return msg.From // 按 agent 分组

	case uap.MsgError:
		// gateway error 消息保留了原始请求 ID
		return msg.ID

	case uap.MsgNotify:
		return msg.ID

	default:
		return msg.ID
	}
}

// computeDuration 计算耗时
func (t *Tracker) computeDuration(traceID string, msg *uap.Message) int64 {
	// 仅对响应类型计算 duration
	switch msg.Type {
	case uap.MsgToolResult, uap.MsgTaskComplete, uap.MsgError:
	default:
		return 0
	}

	if traceID == "" {
		return 0
	}

	t.mu.Lock()
	startTs, ok := t.traceStart[traceID]
	if ok && (msg.Type == uap.MsgToolResult || msg.Type == uap.MsgTaskComplete) {
		delete(t.traceStart, traceID)
	}
	t.mu.Unlock()

	if !ok || startTs == 0 {
		return 0
	}
	return time.Now().UnixMilli() - startTs
}

// cleanupLoop 定期清理过期的 taskTraces 和 traceStart
func (t *Tracker) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		t.mu.Lock()
		now := time.Now().UnixMilli()
		ttl := int64(3600_000) // 1 小时
		for traceID, startTs := range t.traceStart {
			if now-startTs > ttl {
				delete(t.traceStart, traceID)
			}
		}
		// taskTraces 没有时间戳，只在 traceStart 过期时顺带清理
		for taskID, traceID := range t.taskTraces {
			if _, ok := t.traceStart[traceID]; !ok {
				delete(t.taskTraces, taskID)
			}
		}
		t.mu.Unlock()
	}
}

// ========================= Payload 摘要提取 =========================

func extractRequestID(payload json.RawMessage) string {
	var p struct {
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(payload, &p) == nil {
		return p.RequestID
	}
	return ""
}

func extractTaskID(payload json.RawMessage) string {
	var p struct {
		TaskID string `json:"task_id"`
	}
	if json.Unmarshal(payload, &p) == nil {
		return p.TaskID
	}
	return ""
}

func extractToolName(payload json.RawMessage) string {
	var p struct {
		ToolName string `json:"tool_name"`
	}
	if json.Unmarshal(payload, &p) == nil {
		return p.ToolName
	}
	return ""
}

func extractToolResultSummary(payload json.RawMessage) string {
	var p uap.ToolResultPayload
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	if p.Success {
		resultLen := len(p.Result)
		if resultLen > 0 {
			return fmt.Sprintf("success (len=%d)", resultLen)
		}
		return "success"
	}
	errMsg := p.Error
	if len(errMsg) > 80 {
		errMsg = errMsg[:80] + "..."
	}
	return "error: " + errMsg
}

func extractErrorMessage(payload json.RawMessage) string {
	var p uap.ErrorPayload
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	msg := p.Code
	if p.Message != "" {
		msg += ": " + p.Message
	}
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
	return msg
}

func extractRegisterSummary(payload json.RawMessage) string {
	var p struct {
		Name      string `json:"name"`
		AgentType string `json:"agent_type"`
		AgentID   string `json:"agent_id"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	return fmt.Sprintf("%s (%s/%s)", p.Name, p.AgentType, p.AgentID)
}

func extractNotifySummary(payload json.RawMessage) string {
	var p struct {
		Channel string `json:"channel"`
		Event   string `json:"event"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	if p.Channel != "" {
		return "channel=" + p.Channel
	}
	if p.Event != "" {
		return "event=" + p.Event
	}
	return ""
}

// extractSummary 根据消息类型提取摘要
func (t *Tracker) extractSummary(msg *uap.Message) string {
	switch msg.Type {
	case uap.MsgToolCall:
		return extractToolName(msg.Payload)
	case uap.MsgToolResult:
		return extractToolResultSummary(msg.Payload)
	case uap.MsgRegister:
		return extractRegisterSummary(msg.Payload)
	case uap.MsgNotify:
		return extractNotifySummary(msg.Payload)
	case uap.MsgError:
		return extractErrorMessage(msg.Payload)
	case uap.MsgTaskAssign:
		taskID := extractTaskID(msg.Payload)
		return "task=" + taskID
	case uap.MsgTaskAccepted:
		return "accepted task=" + extractTaskID(msg.Payload)
	case uap.MsgTaskRejected:
		return "rejected task=" + extractTaskID(msg.Payload)
	case uap.MsgTaskEvent:
		return "event task=" + extractTaskID(msg.Payload)
	case uap.MsgTaskComplete:
		var p uap.TaskCompletePayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			return fmt.Sprintf("complete task=%s status=%s", p.TaskID, p.Status)
		}
		return "complete"
	default:
		return ""
	}
}

// ========================= 终端日志 =========================

// logToStdout 输出结构化终端日志
func (t *Tracker) logToStdout(e *Event) {
	var parts []string
	parts = append(parts, fmt.Sprintf("#%d", e.Seq))
	parts = append(parts, e.Kind)

	if e.TraceID != "" {
		// 截断显示
		tid := e.TraceID
		if len(tid) > 12 {
			tid = tid[:12]
		}
		parts = append(parts, "trace="+tid)
	}

	if e.MsgType != "" {
		parts = append(parts, e.MsgType)
	}

	// from→to
	if e.From != "" || e.To != "" {
		fromStr := e.FromName
		if fromStr == "" {
			fromStr = e.From
		}
		toStr := e.ToName
		if toStr == "" {
			toStr = e.To
		}
		if fromStr != "" && toStr != "" {
			parts = append(parts, fromStr+"→"+toStr)
		} else if fromStr != "" {
			parts = append(parts, fromStr)
		} else if toStr != "" {
			parts = append(parts, "→"+toStr)
		}
	}

	if e.PayloadSummary != "" {
		summary := e.PayloadSummary
		if len(summary) > 60 {
			summary = summary[:60] + "..."
		}
		parts = append(parts, "\""+summary+"\"")
	}

	if e.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("%dms", e.DurationMs))
	}

	if e.Error != "" {
		errStr := e.Error
		if len(errStr) > 60 {
			errStr = errStr[:60] + "..."
		}
		parts = append(parts, "ERR="+errStr)
	}

	log.Printf("[Event] %s", strings.Join(parts, " "))
}
