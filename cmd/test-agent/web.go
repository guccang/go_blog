package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type dashboardRunner interface {
	SetEventSink(RunnerEventSink)
	RunEvaluation(ctx context.Context) (*FinalEvaluationReport, error)
	RunSuite(ctx context.Context, suite *TestSuite, scenarioID string) (*SystemEvaluationReport, error)
}

type DashboardSuiteOption struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description,omitempty"`
	Path        string                    `json:"path"`
	Scenarios   []DashboardScenarioOption `json:"scenarios,omitempty"`
}

type DashboardScenarioOption struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type DashboardHistoryItem struct {
	RunID        string     `json:"run_id"`
	Mode         string     `json:"mode"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	OverallScore int        `json:"overall_score"`
}

type DashboardState struct {
	Running         bool                    `json:"running"`
	Mode            string                  `json:"mode,omitempty"`
	Status          string                  `json:"status,omitempty"`
	Error           string                  `json:"error,omitempty"`
	LastMessage     string                  `json:"last_message,omitempty"`
	LastUpdated     time.Time               `json:"last_updated"`
	Health          *GatewayHealthSnapshot  `json:"health,omitempty"`
	OnlineAgents    []GatewayAgentSnapshot  `json:"online_agents,omitempty"`
	AvailableSuites []DashboardSuiteOption  `json:"available_suites,omitempty"`
	CurrentRun      *TestRun                `json:"current_run,omitempty"`
	SuiteReport     *SystemEvaluationReport `json:"suite_report,omitempty"`
	ExecutionPlan   *EvaluationPlan         `json:"execution_plan,omitempty"`
	StaticReport    *SystemEvaluationReport `json:"static_report,omitempty"`
	DynamicReport   *SystemEvaluationReport `json:"dynamic_report,omitempty"`
	FinalReport     *FinalEvaluationReport  `json:"final_report,omitempty"`
	History         []DashboardHistoryItem  `json:"history,omitempty"`
}

type dashboardRunRequest struct {
	Mode       string `json:"mode"`
	Suite      string `json:"suite"`
	ScenarioID string `json:"scenario_id"`
}

type DashboardController struct {
	cfg    *Config
	runner dashboardRunner

	mu     sync.RWMutex
	state  DashboardState
	cancel context.CancelFunc
	suites map[string]loadedSuite
	order  []DashboardSuiteOption

	clients map[chan []byte]struct{}
}

func NewDashboardController(cfg *Config, runner dashboardRunner) *DashboardController {
	c := &DashboardController{
		cfg:     cfg,
		runner:  runner,
		suites:  make(map[string]loadedSuite),
		clients: make(map[chan []byte]struct{}),
		state: DashboardState{
			LastUpdated: time.Now(),
		},
	}
	if runner != nil {
		runner.SetEventSink(c)
	}
	c.reloadSuites()
	return c
}

func (c *DashboardController) reloadSuites() {
	suites, err := LoadSuitesFromDir(c.cfg.StaticSuiteDir)
	options := make([]DashboardSuiteOption, 0, len(suites))
	index := make(map[string]loadedSuite, len(suites))
	if err == nil {
		for _, item := range suites {
			if item.suite == nil {
				continue
			}
			option := DashboardSuiteOption{
				ID:          item.suite.ID,
				Title:       item.suite.Title,
				Description: item.suite.Description,
				Path:        item.path,
			}
			for _, scenario := range item.suite.Scenarios {
				option.Scenarios = append(option.Scenarios, DashboardScenarioOption{
					ID:    scenario.ID,
					Title: scenario.Title,
				})
			}
			options = append(options, option)
			index[item.suite.ID] = item
			index[filepath.Base(item.path)] = item
		}
	}
	c.mu.Lock()
	c.suites = index
	c.order = options
	c.state.AvailableSuites = cloneSlice(options)
	c.state.LastUpdated = time.Now()
	c.mu.Unlock()
	c.broadcast()
}

func (c *DashboardController) HandleRunnerEvent(event RunnerEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.LastUpdated = time.Now()
	if strings.TrimSpace(event.Message) != "" {
		c.state.LastMessage = strings.TrimSpace(event.Message)
	}
	if event.Health != nil {
		c.state.Health = clonePointer(event.Health)
	}
	if len(event.OnlineAgents) > 0 {
		c.state.OnlineAgents = cloneSlice(event.OnlineAgents)
	}
	switch event.Type {
	case RunnerEventEvaluationStarted:
		c.state.Running = true
		c.state.Mode = "evaluation"
		c.state.Status = RunStatusRunning
		c.state.Error = ""
		c.state.CurrentRun = nil
		c.state.SuiteReport = nil
		c.state.ExecutionPlan = nil
		c.state.StaticReport = nil
		c.state.DynamicReport = nil
		c.state.FinalReport = nil
	case RunnerEventRunUpdated:
		c.state.CurrentRun = clonePointer(event.Run)
		if event.Run != nil {
			c.state.Status = event.Run.Status
			if event.Run.EvaluationID == "" && c.state.Mode == "" {
				c.state.Mode = "suite"
			}
		}
	case RunnerEventSuiteReport:
		c.state.SuiteReport = clonePointer(event.SuiteReport)
		c.state.Mode = "suite"
		if event.SuiteReport != nil {
			c.state.Status = event.SuiteReport.Status
		}
	case RunnerEventCollectionReport:
		if event.CollectionReport != nil {
			switch event.CollectionReport.CollectionType {
			case CollectionTypeStatic:
				c.state.StaticReport = clonePointer(event.CollectionReport)
			case CollectionTypeDynamic:
				c.state.DynamicReport = clonePointer(event.CollectionReport)
			}
			c.state.Status = event.CollectionReport.Status
		}
	case RunnerEventEvaluationPlan:
		c.state.ExecutionPlan = clonePointer(event.Plan)
		c.state.Mode = "evaluation"
	case RunnerEventEvaluationDone:
		c.state.FinalReport = clonePointer(event.FinalReport)
		c.state.Running = false
		if event.FinalReport != nil {
			c.state.Status = event.FinalReport.Status
			c.state.StaticReport = clonePointer(event.FinalReport.StaticReport)
			c.state.DynamicReport = clonePointer(event.FinalReport.DynamicReport)
			c.appendHistoryLocked(DashboardHistoryItem{
				RunID:        event.FinalReport.RunID,
				Mode:         "evaluation",
				Title:        event.FinalReport.Title,
				Status:       event.FinalReport.Status,
				StartedAt:    event.FinalReport.StartedAt,
				FinishedAt:   &event.FinalReport.FinishedAt,
				OverallScore: event.FinalReport.OverallScore,
			})
		}
	case RunnerEventRunnerError:
		c.state.Running = false
		c.state.Error = firstNonEmpty(event.Message, "runner error")
		c.state.Status = RunStatusError
	}
	go c.broadcast()
}

func (c *DashboardController) appendHistoryLocked(item DashboardHistoryItem) {
	c.state.History = append([]DashboardHistoryItem{item}, c.state.History...)
	if len(c.state.History) > 10 {
		c.state.History = c.state.History[:10]
	}
}

func (c *DashboardController) snapshot() DashboardState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return DashboardState{
		Running:         c.state.Running,
		Mode:            c.state.Mode,
		Status:          c.state.Status,
		Error:           c.state.Error,
		LastMessage:     c.state.LastMessage,
		LastUpdated:     c.state.LastUpdated,
		Health:          clonePointer(c.state.Health),
		OnlineAgents:    cloneSlice(c.state.OnlineAgents),
		AvailableSuites: cloneSlice(c.state.AvailableSuites),
		CurrentRun:      clonePointer(c.state.CurrentRun),
		SuiteReport:     clonePointer(c.state.SuiteReport),
		ExecutionPlan:   clonePointer(c.state.ExecutionPlan),
		StaticReport:    clonePointer(c.state.StaticReport),
		DynamicReport:   clonePointer(c.state.DynamicReport),
		FinalReport:     clonePointer(c.state.FinalReport),
		History:         cloneSlice(c.state.History),
	}
}

func (c *DashboardController) subscribe() chan []byte {
	ch := make(chan []byte, 4)
	c.mu.Lock()
	c.clients[ch] = struct{}{}
	c.mu.Unlock()
	return ch
}

func (c *DashboardController) unsubscribe(ch chan []byte) {
	c.mu.Lock()
	delete(c.clients, ch)
	close(ch)
	c.mu.Unlock()
}

func (c *DashboardController) broadcast() {
	state := c.snapshot()
	payload, err := json.Marshal(state)
	if err != nil {
		return
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for ch := range c.clients {
		select {
		case ch <- payload:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- payload:
			default:
			}
		}
	}
}

func (c *DashboardController) StartEvaluation() error {
	c.mu.Lock()
	if c.state.Running {
		c.mu.Unlock()
		return fmt.Errorf("test-agent is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.state.Running = true
	c.state.Mode = "evaluation"
	c.state.Status = RunStatusRunning
	c.state.Error = ""
	c.state.LastMessage = "evaluation queued"
	c.state.CurrentRun = nil
	c.state.SuiteReport = nil
	c.state.ExecutionPlan = nil
	c.state.StaticReport = nil
	c.state.DynamicReport = nil
	c.state.FinalReport = nil
	c.state.LastUpdated = time.Now()
	c.mu.Unlock()
	c.broadcast()

	go func() {
		final, err := c.runner.RunEvaluation(ctx)
		if err != nil {
			c.HandleRunnerEvent(RunnerEvent{
				Type:    RunnerEventRunnerError,
				Mode:    "evaluation",
				Message: err.Error(),
			})
			return
		}
		if final != nil {
			c.HandleRunnerEvent(RunnerEvent{
				Type:        RunnerEventEvaluationDone,
				Mode:        "evaluation",
				FinalReport: final,
			})
		}
	}()
	return nil
}

func (c *DashboardController) StartSuite(suiteID, scenarioID string) error {
	c.reloadSuites()
	c.mu.Lock()
	if c.state.Running {
		c.mu.Unlock()
		return fmt.Errorf("test-agent is already running")
	}
	item, ok := c.suites[strings.TrimSpace(suiteID)]
	if !ok || item.suite == nil {
		c.mu.Unlock()
		return fmt.Errorf("suite not found: %s", suiteID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.state.Running = true
	c.state.Mode = "suite"
	c.state.Status = RunStatusRunning
	c.state.Error = ""
	c.state.LastMessage = "suite queued"
	c.state.CurrentRun = nil
	c.state.SuiteReport = nil
	c.state.ExecutionPlan = nil
	c.state.StaticReport = nil
	c.state.DynamicReport = nil
	c.state.FinalReport = nil
	c.state.LastUpdated = time.Now()
	suite := clonePointer(item.suite)
	c.mu.Unlock()
	c.broadcast()

	go func() {
		report, err := c.runner.RunSuite(ctx, suite, strings.TrimSpace(scenarioID))
		if err != nil {
			c.HandleRunnerEvent(RunnerEvent{
				Type:    RunnerEventRunnerError,
				Mode:    "suite",
				Message: err.Error(),
			})
			return
		}
		c.mu.Lock()
		c.state.Running = false
		if report != nil {
			c.state.SuiteReport = clonePointer(report)
			c.state.Status = report.Status
			finishedAt := report.FinishedAt
			c.appendHistoryLocked(DashboardHistoryItem{
				RunID:        report.RunID,
				Mode:         "suite",
				Title:        report.Title,
				Status:       report.Status,
				StartedAt:    report.StartedAt,
				FinishedAt:   &finishedAt,
				OverallScore: report.AverageScore,
			})
		}
		c.state.LastUpdated = time.Now()
		c.mu.Unlock()
		c.broadcast()
	}()
	return nil
}

func (c *DashboardController) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardPage.Execute(w, map[string]any{
		"ListenAddr": c.cfg.Web.ListenAddr,
	})
}

func (c *DashboardController) handleState(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(c.snapshot())
}

func (c *DashboardController) handleSuites(w http.ResponseWriter, _ *http.Request) {
	c.reloadSuites()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.mu.RLock()
	defer c.mu.RUnlock()
	_ = json.NewEncoder(w).Encode(c.state.AvailableSuites)
}

func (c *DashboardController) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req dashboardRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	var err error
	switch strings.TrimSpace(req.Mode) {
	case "", "evaluation":
		err = c.StartEvaluation()
	case "suite":
		err = c.StartSuite(req.Suite, req.ScenarioID)
	default:
		err = fmt.Errorf("unsupported mode: %s", req.Mode)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (c *DashboardController) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := c.subscribe()
	defer c.unsubscribe(ch)

	initial, _ := json.Marshal(c.snapshot())
	_, _ = fmt.Fprintf(w, "event: state\ndata: %s\n\n", initial)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-ch:
			_, _ = fmt.Fprintf(w, "event: state\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func StartDashboardServer(cfg *Config, runner dashboardRunner) error {
	controller := NewDashboardController(cfg, runner)
	mux := http.NewServeMux()
	mux.HandleFunc("/", controller.handleIndex)
	mux.HandleFunc("/api/state", controller.handleState)
	mux.HandleFunc("/api/suites", controller.handleSuites)
	mux.HandleFunc("/api/run", controller.handleRun)
	mux.HandleFunc("/api/stream", controller.handleStream)
	mux.Handle("/artifacts/", http.StripPrefix("/artifacts/", http.FileServer(http.Dir(cfg.OutputDir))))

	server := &http.Server{
		Addr:              cfg.Web.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

var dashboardPage = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>test-agent 控制台</title>
  <style>
    :root {
      --bg: #f4f1e8;
      --panel: #fffdf7;
      --text: #1f2421;
      --muted: #65706b;
      --line: #d8d0c1;
      --accent: #1f6f5f;
      --warn: #b35c1e;
      --bad: #b42318;
      --good: #0d7a46;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "SF Mono", "Menlo", monospace;
      color: var(--text);
      background:
        radial-gradient(circle at top right, rgba(31,111,95,0.10), transparent 28%),
        linear-gradient(180deg, #f7f3ea 0%, var(--bg) 100%);
    }
    .wrap {
      max-width: 1280px;
      margin: 0 auto;
      padding: 24px;
    }
    .hero {
      display: grid;
      gap: 12px;
      margin-bottom: 20px;
    }
    .hero h1 {
      margin: 0;
      font-size: 28px;
      letter-spacing: 0.02em;
    }
    .hero p {
      margin: 0;
      color: var(--muted);
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(12, 1fr);
      gap: 16px;
    }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 16px;
      box-shadow: 0 8px 24px rgba(31, 36, 33, 0.05);
    }
    .span-4 { grid-column: span 4; }
    .span-6 { grid-column: span 6; }
    .span-8 { grid-column: span 8; }
    .span-12 { grid-column: span 12; }
    .controls {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      align-items: center;
    }
    button, select {
      border-radius: 999px;
      border: 1px solid var(--line);
      background: white;
      padding: 10px 14px;
      font: inherit;
    }
    button.primary {
      background: var(--accent);
      color: white;
      border-color: var(--accent);
    }
    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 6px 10px;
      border-radius: 999px;
      background: rgba(31, 111, 95, 0.08);
      color: var(--accent);
    }
    .status.bad { background: rgba(180, 35, 24, 0.08); color: var(--bad); }
    .status.warn { background: rgba(179, 92, 30, 0.08); color: var(--warn); }
    .metric {
      display: grid;
      gap: 6px;
    }
    .metric .label { color: var(--muted); font-size: 12px; text-transform: uppercase; }
    .metric .value { font-size: 24px; }
    .meta { color: var(--muted); font-size: 13px; }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }
    th, td {
      text-align: left;
      padding: 8px 6px;
      border-bottom: 1px solid var(--line);
      vertical-align: top;
    }
    th { color: var(--muted); font-weight: 600; }
    pre {
      margin: 0;
      white-space: pre-wrap;
      word-break: break-word;
      font-size: 12px;
      line-height: 1.45;
    }
    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }
    .chip {
      border-radius: 999px;
      padding: 5px 10px;
      background: #f0ebe0;
      color: #31413b;
      font-size: 12px;
    }
    .section-title {
      margin: 0 0 10px 0;
      font-size: 16px;
    }
    .empty { color: var(--muted); font-size: 13px; }
    @media (max-width: 960px) {
      .span-4, .span-6, .span-8, .span-12 { grid-column: span 12; }
      .wrap { padding: 14px; }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <h1>test-agent 实时检测控制台</h1>
      <p>用于启动评估、实时查看执行路径、场景进度、静态/动态结果，以及最终评估结论。</p>
    </div>

    <div class="grid">
      <div class="card span-12">
        <div class="controls">
          <button id="runEval" class="primary">运行完整评估</button>
          <select id="suiteSelect"></select>
          <select id="scenarioSelect"></select>
          <button id="runSuite">运行选中 Suite</button>
          <span id="statusBadge" class="status">idle</span>
        </div>
        <div class="meta" id="lastMessage" style="margin-top: 10px;"></div>
      </div>

      <div class="card span-4">
        <div class="metric">
          <div class="label">模式</div>
          <div class="value" id="modeValue">-</div>
        </div>
      </div>
      <div class="card span-4">
        <div class="metric">
          <div class="label">在线 Agents</div>
          <div class="value" id="agentCount">0</div>
        </div>
      </div>
      <div class="card span-4">
        <div class="metric">
          <div class="label">综合得分</div>
          <div class="value" id="scoreValue">-</div>
        </div>
      </div>

      <div class="card span-12">
        <h2 class="section-title">在线 Agent</h2>
        <div id="agents" class="chips"></div>
      </div>

      <div class="card span-6">
        <h2 class="section-title">当前场景</h2>
        <div id="currentRun" class="empty">暂无运行中的场景。</div>
      </div>
      <div class="card span-6">
        <h2 class="section-title">执行计划</h2>
        <div id="plan" class="empty">暂无执行计划。</div>
      </div>

      <div class="card span-6">
        <h2 class="section-title">静态评估集</h2>
        <div id="staticReport" class="empty">暂无数据。</div>
      </div>
      <div class="card span-6">
        <h2 class="section-title">动态评估集</h2>
        <div id="dynamicReport" class="empty">暂无数据。</div>
      </div>

      <div class="card span-12">
        <h2 class="section-title">最终结果 / Suite 结果</h2>
        <div id="finalReport" class="empty">暂无结果。</div>
      </div>

      <div class="card span-12">
        <h2 class="section-title">最近运行历史</h2>
        <div id="history" class="empty">暂无历史。</div>
      </div>
    </div>
  </div>

  <script>
    const runEvalBtn = document.getElementById('runEval');
    const runSuiteBtn = document.getElementById('runSuite');
    const suiteSelect = document.getElementById('suiteSelect');
    const scenarioSelect = document.getElementById('scenarioSelect');

    let latestState = null;

    function fmtTime(value) {
      if (!value) return '-';
      return new Date(value).toLocaleString();
    }

    function escapeHtml(text) {
      return String(text || '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;');
    }

    function renderStatus(status) {
      const bad = ['failed', 'error', 'timeout'];
      const warn = ['running', 'skipped'];
      const cls = bad.includes(status) ? 'status bad' : (warn.includes(status) ? 'status warn' : 'status');
      return '<span class="' + cls + '">' + escapeHtml(status || 'idle') + '</span>';
    }

    function renderReport(report) {
      if (!report) return '<div class="empty">暂无数据。</div>';
      const rows = [
        ['状态', renderStatus(report.status)],
        ['总场景', report.total_scenarios || 0],
        ['已执行', report.executed_scenarios || 0],
        ['通过', report.passed_scenarios || 0],
        ['失败', report.failed_scenarios || 0],
        ['跳过', report.skipped_scenarios || 0],
        ['平均分', report.average_score || 0]
      ];
      let html = '<table><tbody>';
      rows.forEach(([k, v]) => {
        html += '<tr><th>' + escapeHtml(k) + '</th><td>' + v + '</td></tr>';
      });
      html += '</tbody></table>';
      if (report.runs && report.runs.length) {
        html += '<div style="margin-top:12px;"><table><thead><tr><th>Scenario</th><th>Status</th><th>Score</th><th>Target</th></tr></thead><tbody>';
        report.runs.forEach(run => {
          html += '<tr><td>' + escapeHtml(run.scenario_id) + '</td><td>' + renderStatus(run.status) + '</td><td>' + (run.result?.scores?.total || 0) + '</td><td>' + escapeHtml(run.target_agent) + '</td></tr>';
        });
        html += '</tbody></table></div>';
      }
      return html;
    }

    function renderPlan(plan) {
      if (!plan) return '<div class="empty">暂无执行计划。</div>';
      let html = '<table><tbody>';
      html += '<tr><th>run_id</th><td>' + escapeHtml(plan.run_id) + '</td></tr>';
      html += '<tr><th>started_at</th><td>' + escapeHtml(fmtTime(plan.started_at)) + '</td></tr>';
      html += '</tbody></table>';
      const collections = [];
      (plan.static_collections || []).forEach(item => collections.push(item));
      if (plan.dynamic_collection) collections.push(plan.dynamic_collection);
      if (collections.length) {
        html += '<div style="margin-top:12px;"><table><thead><tr><th>Collection</th><th>Scenarios</th><th>Source</th></tr></thead><tbody>';
        collections.forEach(item => {
          html += '<tr><td>' + escapeHtml(item.title || item.id) + '</td><td>' + (item.scenario_count || 0) + '</td><td>' + escapeHtml(item.source || '-') + '</td></tr>';
        });
        html += '</tbody></table></div>';
      }
      return html;
    }

    function renderCurrentRun(run) {
      if (!run) return '<div class="empty">暂无运行中的场景。</div>';
      let html = '<table><tbody>';
      html += '<tr><th>scenario</th><td>' + escapeHtml(run.scenario_id) + '</td></tr>';
      html += '<tr><th>title</th><td>' + escapeHtml(run.title) + '</td></tr>';
      html += '<tr><th>status</th><td>' + renderStatus(run.status) + '</td></tr>';
      html += '<tr><th>target</th><td>' + escapeHtml(run.target_agent) + '</td></tr>';
      html += '<tr><th>trace</th><td>' + escapeHtml(run.trace_id) + '</td></tr>';
      html += '</tbody></table>';
      if (run.steps && run.steps.length) {
        html += '<div style="margin-top:12px;"><table><thead><tr><th>Step</th><th>Status</th><th>Detail</th></tr></thead><tbody>';
        run.steps.forEach(step => {
          html += '<tr><td>' + escapeHtml(step.name) + '</td><td>' + renderStatus(step.status) + '</td><td>' + escapeHtml(step.detail || '') + '</td></tr>';
        });
        html += '</tbody></table></div>';
      }
      if (run.result) {
        html += '<div style="margin-top:12px;"><pre>' + escapeHtml(JSON.stringify(run.result, null, 2)) + '</pre></div>';
      }
      return html;
    }

    function renderFinal(state) {
      const final = state.final_report;
      if (final) {
        let html = '<table><tbody>';
        html += '<tr><th>run_id</th><td>' + escapeHtml(final.run_id) + '</td></tr>';
        html += '<tr><th>status</th><td>' + renderStatus(final.status) + '</td></tr>';
        html += '<tr><th>overall_score</th><td>' + (final.overall_score || 0) + '</td></tr>';
        html += '<tr><th>started_at</th><td>' + escapeHtml(fmtTime(final.started_at)) + '</td></tr>';
        html += '<tr><th>finished_at</th><td>' + escapeHtml(fmtTime(final.finished_at)) + '</td></tr>';
        html += '</tbody></table>';
        if (final.findings && final.findings.length) {
          html += '<div style="margin-top:12px;"><pre>' + escapeHtml(final.findings.join('\n')) + '</pre></div>';
        }
        return html;
      }
      if (state.suite_report) {
        return renderReport(state.suite_report);
      }
      return '<div class="empty">暂无结果。</div>';
    }

    function renderHistory(history) {
      if (!history || !history.length) return '<div class="empty">暂无历史。</div>';
      let html = '<table><thead><tr><th>Run</th><th>Mode</th><th>Status</th><th>Score</th><th>Started</th><th>Finished</th></tr></thead><tbody>';
      history.forEach(item => {
        html += '<tr><td>' + escapeHtml(item.run_id) + '</td><td>' + escapeHtml(item.mode) + '</td><td>' + renderStatus(item.status) + '</td><td>' + (item.overall_score || 0) + '</td><td>' + escapeHtml(fmtTime(item.started_at)) + '</td><td>' + escapeHtml(fmtTime(item.finished_at)) + '</td></tr>';
      });
      html += '</tbody></table>';
      return html;
    }

    function fillSuites(suites) {
      suiteSelect.innerHTML = '';
      const placeholder = document.createElement('option');
      placeholder.value = '';
      placeholder.textContent = '选择 suite';
      suiteSelect.appendChild(placeholder);
      (suites || []).forEach(suite => {
        const option = document.createElement('option');
        option.value = suite.id;
        option.textContent = suite.title || suite.id;
        suiteSelect.appendChild(option);
      });
      renderScenarioOptions();
    }

    function renderScenarioOptions() {
      const suites = latestState?.available_suites || [];
      const selected = suites.find(item => item.id === suiteSelect.value);
      scenarioSelect.innerHTML = '';
      const any = document.createElement('option');
      any.value = '';
      any.textContent = '全部场景';
      scenarioSelect.appendChild(any);
      (selected?.scenarios || []).forEach(s => {
        const option = document.createElement('option');
        option.value = s.id;
        option.textContent = s.title || s.id;
        scenarioSelect.appendChild(option);
      });
    }

    function renderState(state) {
      latestState = state;
      fillSuites(state.available_suites || []);
      if (suiteSelect.value) renderScenarioOptions();

      document.getElementById('statusBadge').outerHTML = renderStatus(state.status || (state.running ? 'running' : 'idle')).replace('<span', '<span id="statusBadge"');
      document.getElementById('lastMessage').textContent = state.error || state.last_message || '';
      document.getElementById('modeValue').textContent = state.mode || '-';
      document.getElementById('agentCount').textContent = String((state.online_agents || []).length);
      document.getElementById('scoreValue').textContent = state.final_report ? String(state.final_report.overall_score || 0) : (state.suite_report ? String(state.suite_report.average_score || 0) : '-');
      document.getElementById('agents').innerHTML = (state.online_agents || []).map(agent => '<span class="chip">' + escapeHtml(agent.agent_id) + '</span>').join('') || '<span class="empty">暂无在线 agent</span>';
      document.getElementById('currentRun').innerHTML = renderCurrentRun(state.current_run);
      document.getElementById('plan').innerHTML = renderPlan(state.execution_plan);
      document.getElementById('staticReport').innerHTML = renderReport(state.static_report);
      document.getElementById('dynamicReport').innerHTML = renderReport(state.dynamic_report);
      document.getElementById('finalReport').innerHTML = renderFinal(state);
      document.getElementById('history').innerHTML = renderHistory(state.history);

      runEvalBtn.disabled = !!state.running;
      runSuiteBtn.disabled = !!state.running || !suiteSelect.value;
    }

    async function fetchState() {
      const response = await fetch('/api/state');
      renderState(await response.json());
    }

    async function postRun(body) {
      const response = await fetch('/api/run', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        alert(await response.text());
        return;
      }
    }

    suiteSelect.addEventListener('change', () => {
      renderScenarioOptions();
      runSuiteBtn.disabled = !suiteSelect.value || !!latestState?.running;
    });

    runEvalBtn.addEventListener('click', () => postRun({mode: 'evaluation'}));
    runSuiteBtn.addEventListener('click', () => postRun({
      mode: 'suite',
      suite: suiteSelect.value,
      scenario_id: scenarioSelect.value
    }));

    fetchState();
    const stream = new EventSource('/api/stream');
    stream.addEventListener('state', (event) => {
      renderState(JSON.parse(event.data));
    });
    stream.onerror = () => {
      setTimeout(fetchState, 1000);
    };
  </script>
</body>
</html>`))

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
