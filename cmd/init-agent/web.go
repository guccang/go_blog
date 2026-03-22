package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// wsHub manages WebSocket connections for broadcasting events.
type wsHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func newWSHub() *wsHub {
	return &wsHub{clients: make(map[*websocket.Conn]bool)}
}

func (h *wsHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

func (h *wsHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *wsHub) broadcast(msgType string, data any) {
	msg := map[string]any{"type": msgType, "data": data}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			conn.Close()
			go h.remove(conn)
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// RunWebServer starts the web-based wizard UI.
func RunWebServer(cfg *InitConfig) error {
	state := NewWizardState(cfg.RootDir)
	hub := newWSHub()

	mux := http.NewServeMux()

	// Serve static files (SPA)
	mux.Handle("/", http.FileServer(staticFS()))

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}
		hub.add(conn)
		defer func() {
			hub.remove(conn)
			conn.Close()
		}()

		// Read loop (keep alive, handle client messages)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	})

	// API: Trigger environment check
	mux.HandleFunc("/api/env/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		ds := state.DeployState
		hasPipelines := ds != nil && ds.Available && len(ds.Pipelines) > 0

		if hasPipelines {
			// 基于 pipeline 的按 target 远程检测
			go func() {
				plans := DeriveTargetRequirements(ds.Pipelines, ds.Targets, ds.Projects)
				sshPassword := ds.SSHPassword
				sshKeyPath := GetSSHKeyPath(cfg.RootDir)

				results := RunTargetChecks(plans, sshPassword, sshKeyPath)
				state.TargetEnvResults = results

				// 逐个 target 广播
				for _, r := range results {
					hub.broadcast("target_env_result", r)
				}
				hub.broadcast("target_env_complete", results)
			}()
			writeJSON(w, map[string]any{"success": true, "message": "目标机器环境检测已开始", "mode": "target"})
		} else {
			// 无 deploy 配置，本机通用检测
			go func() {
				state.EnvResults = nil
				reqs := DefaultRequirements()
				for _, req := range reqs {
					result := checkOne(req)
					state.EnvResults = append(state.EnvResults, result)
					hub.broadcast("env_check_result", result)
				}
				hub.broadcast("env_check_complete", state.EnvResults)
			}()
			writeJSON(w, map[string]any{"success": true, "message": "环境检测已开始", "mode": "local"})
		}
	})

	// API: Get cached environment results
	mux.HandleFunc("/api/env/results", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"success":        true,
			"results":        state.EnvResults,
			"target_results": state.TargetEnvResults,
		})
	})

	// API: Get all agent schemas (kept for availability)
	mux.HandleFunc("/api/agents/schemas", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"success": true, "schemas": AllAgentSchemas()})
	})

	// API: Get discovered agent configs (dynamic JSON-based)
	mux.HandleFunc("/api/agents/discovered", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"success": true,
			"agents":  state.DiscoveredConfigs,
		})
	})

	// API: Get existing configs (now uses discovered configs)
	mux.HandleFunc("/api/agents/configs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			configs := make(map[string]map[string]any)
			for _, dc := range state.DiscoveredConfigs {
				if dc.Values != nil {
					configs[dc.Name] = dc.Values
				}
			}
			writeJSON(w, map[string]any{"success": true, "configs": configs})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Agents map[string]map[string]any `json:"agents"`
				Shared map[string]string         `json:"shared"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]any{"success": false, "error": err.Error()})
				return
			}

			state.SharedValues = req.Shared
			var written []string

			for agentName, vals := range req.Agents {
				// Find discovered config for this agent
				info := state.GetDiscoveredConfig(agentName)
				if info == nil {
					continue
				}

				state.GeneratedConfigs[agentName] = vals

				path, err := WriteDiscoveredConfig(cfg.RootDir, *info, vals)
				if err != nil {
					writeJSON(w, map[string]any{"success": false, "error": err.Error()})
					return
				}
				written = append(written, path)
				hub.broadcast("config_written", map[string]any{
					"agent": agentName,
					"path":  path,
				})
			}

			writeJSON(w, map[string]any{"success": true, "written": written})
			return
		}

		http.Error(w, "Method not allowed", 405)
	})

	// API: Trigger availability check
	mux.HandleFunc("/api/availability/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		ds := state.DeployState
		hasPipelines := ds != nil && ds.Available && len(ds.Pipelines) > 0

		go func() {
			// 有 pipeline 数据时先做 pipeline 分组检测
			if hasPipelines {
				sshPassword := ds.SSHPassword
				sshKeyPath := GetSSHKeyPath(cfg.RootDir)
				state.PipelineAvailResults = RunPipelineAvailChecks(ds, state.TargetEnvResults, sshPassword, sshKeyPath)
				hub.broadcast("pipeline_avail_result", state.PipelineAvailResults)
			}

			layers := RunAvailabilityChecks(cfg.RootDir, state.GeneratedConfigs, state.PipelineAvailResults)
			for _, layer := range layers {
				hub.broadcast("avail_layer_result", layer)
			}
			hub.broadcast("avail_complete", layers)
			state.AvailabilityLayers = layers
		}()

		writeJSON(w, map[string]any{"success": true, "message": "可用性检测已开始"})
	})

	// API: Get wizard state
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"success":                true,
			"current_step":           state.CurrentStep,
			"root_dir":               state.RootDir,
			"env_results":            state.EnvResults,
			"target_env_results":     state.TargetEnvResults,
			"selected":               state.SelectedAgents,
			"availability":           state.AvailabilityLayers,
			"pipeline_avail_results": state.PipelineAvailResults,
		})
	})

	// API: Deploy status — check if deploy settings are available
	mux.HandleFunc("/api/deploy/status", func(w http.ResponseWriter, r *http.Request) {
		ds := state.DeployState
		writeJSON(w, map[string]any{
			"success":   true,
			"available": ds != nil && ds.Available,
		})
	})

	// API: Deploy targets — GET reads, POST saves
	mux.HandleFunc("/api/deploy/targets", func(w http.ResponseWriter, r *http.Request) {
		ds := state.DeployState
		if ds == nil || !ds.Available {
			writeJSON(w, map[string]any{"success": false, "error": "deploy settings not available"})
			return
		}

		if r.Method == http.MethodGet {
			writeJSON(w, map[string]any{
				"success":      true,
				"targets":      ds.Targets,
				"ssh_password": ds.SSHPassword,
			})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Targets     map[string]DeployTarget `json:"targets"`
				SSHPassword string                  `json:"ssh_password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]any{"success": false, "error": err.Error()})
				return
			}
			ds.Targets = req.Targets
			ds.SSHPassword = req.SSHPassword
			writeJSON(w, map[string]any{"success": true})
			return
		}

		http.Error(w, "Method not allowed", 405)
	})

	// API: Deploy projects — GET reads, POST saves
	mux.HandleFunc("/api/deploy/projects", func(w http.ResponseWriter, r *http.Request) {
		ds := state.DeployState
		if ds == nil || !ds.Available {
			writeJSON(w, map[string]any{"success": false, "error": "deploy settings not available"})
			return
		}

		if r.Method == http.MethodGet {
			writeJSON(w, map[string]any{
				"success":  true,
				"projects": ds.Projects,
				"order":    ds.ProjectOrder,
			})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Projects map[string]*DeployProject `json:"projects"`
				Order    []string                  `json:"order"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]any{"success": false, "error": err.Error()})
				return
			}
			// Restore Name field from key
			for name, proj := range req.Projects {
				proj.Name = name
			}
			ds.Projects = req.Projects
			if len(req.Order) > 0 {
				ds.ProjectOrder = req.Order
			}
			writeJSON(w, map[string]any{"success": true})
			return
		}

		http.Error(w, "Method not allowed", 405)
	})

	// API: Deploy pipelines — GET reads, POST saves
	mux.HandleFunc("/api/deploy/pipelines", func(w http.ResponseWriter, r *http.Request) {
		ds := state.DeployState
		if ds == nil || !ds.Available {
			writeJSON(w, map[string]any{"success": false, "error": "deploy settings not available"})
			return
		}

		if r.Method == http.MethodGet {
			writeJSON(w, map[string]any{
				"success":   true,
				"pipelines": ds.Pipelines,
			})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Pipelines []DeployPipeline `json:"pipelines"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]any{"success": false, "error": err.Error()})
				return
			}
			ds.Pipelines = req.Pipelines
			writeJSON(w, map[string]any{"success": true})
			return
		}

		http.Error(w, "Method not allowed", 405)
	})

	// --- Progressive Deployment APIs ---

	// API: Get agent tier metadata
	mux.HandleFunc("/api/agents/tiers", func(w http.ResponseWriter, r *http.Request) {
		registry := AgentMetaRegistry()
		byTier := GetAgentsByTier()

		tierList := make([]map[string]any, 0)
		tierNames := map[AgentTier]string{
			TierCore:         "基础设施（必须）",
			TierIntelligence: "智能层（推荐）",
			TierProductivity: "生产力（按需）",
			TierSpecialized:  "专业化（可选）",
		}

		for tier := TierCore; tier <= TierSpecialized; tier++ {
			agents := byTier[tier]
			agentList := make([]map[string]any, 0, len(agents))
			for _, a := range agents {
				installed := agentHasConfig(cfg.RootDir, a.Name)
				agentList = append(agentList, map[string]any{
					"name":        a.Name,
					"tier":        a.Tier,
					"deps":        a.AgentDeps,
					"keywords":    a.FeatureKeywords,
					"short_pitch": a.ShortPitch,
					"installed":   installed,
				})
			}
			tierList = append(tierList, map[string]any{
				"tier":   tier,
				"label":  tierNames[tier],
				"agents": agentList,
			})
		}

		_ = registry
		writeJSON(w, map[string]any{"success": true, "tiers": tierList})
	})

	// API: Recommend agents based on intent
	mux.HandleFunc("/api/agents/recommend", func(w http.ResponseWriter, r *http.Request) {
		intent := r.URL.Query().Get("intent")
		if intent == "" {
			writeJSON(w, map[string]any{
				"success": false,
				"error":   "missing 'intent' query parameter",
			})
			return
		}

		results := RecommendAgents(intent)
		// Enrich with installed status
		for i := range results {
			results[i].Installed = agentHasConfig(cfg.RootDir, results[i].Agent.Name)
		}

		writeJSON(w, map[string]any{"success": true, "results": results})
	})

	// API: Add agents (incremental install)
	mux.HandleFunc("/api/agents/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		var req struct {
			Agents []string               `json:"agents"`
			Values map[string]map[string]any `json:"values"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}

		if len(req.Agents) == 0 {
			writeJSON(w, map[string]any{"success": false, "error": "no agents specified"})
			return
		}

		// Resolve dependencies
		allNeeded, err := resolveAllDeps(req.Agents)
		if err != nil {
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}

		// Filter already configured
		var needConfig []string
		var alreadyConfigured []string
		for _, name := range allNeeded {
			if agentHasConfig(cfg.RootDir, name) {
				alreadyConfigured = append(alreadyConfigured, name)
			} else {
				needConfig = append(needConfig, name)
			}
		}

		// Write configs for agents that have values provided
		var written []string
		for _, agentName := range needConfig {
			vals, ok := req.Values[agentName]
			if !ok {
				continue
			}

			info := state.GetDiscoveredConfig(agentName)
			if info != nil {
				state.GeneratedConfigs[agentName] = vals
				path, err := WriteDiscoveredConfig(cfg.RootDir, *info, vals)
				if err != nil {
					writeJSON(w, map[string]any{"success": false, "error": err.Error()})
					return
				}
				written = append(written, path)
				hub.broadcast("config_written", map[string]any{"agent": agentName, "path": path})
			} else if schema := GetAgentSchema(agentName); schema != nil {
				state.GeneratedConfigs[agentName] = vals
				path, err := WriteAgentConfig(cfg.RootDir, schema, vals)
				if err != nil {
					writeJSON(w, map[string]any{"success": false, "error": err.Error()})
					return
				}
				written = append(written, path)
				hub.broadcast("config_written", map[string]any{"agent": agentName, "path": path})
			}
		}

		writeJSON(w, map[string]any{
			"success":            true,
			"resolved":           allNeeded,
			"already_configured": alreadyConfigured,
			"need_config":        needConfig,
			"written":            written,
		})
	})

	// API: Quick start (configure core agents)
	mux.HandleFunc("/api/quickstart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		var req struct {
			GatewayPort int    `json:"gateway_port"`
			RedisIP     string `json:"redis_ip"`
			RedisPort   string `json:"redis_port"`
			Admin       string `json:"admin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}

		if req.GatewayPort == 0 {
			req.GatewayPort = 10086
		}
		if req.RedisIP == "" {
			req.RedisIP = "127.0.0.1"
		}
		if req.RedisPort == "" {
			req.RedisPort = "6379"
		}
		if req.Admin == "" {
			req.Admin = "admin"
		}

		serverURL := fmt.Sprintf("ws://127.0.0.1:%d/ws/uap", req.GatewayPort)

		gatewayValues := map[string]any{
			"port":              req.GatewayPort,
			"go_backend_url":    "http://127.0.0.1:8080",
			"auth_token":        "",
			"event_tracking":    true,
			"event_buffer_size": 10000,
			"event_log_dir":     "logs",
			"event_log_stdout":  true,
		}

		blogValues := map[string]any{
			"admin":         req.Admin,
			"port":          "8080",
			"redis_ip":      req.RedisIP,
			"redis_port":    req.RedisPort,
			"redis_pwd":     "",
			"gateway_url":   serverURL,
			"gateway_token": "",
			"logs_dir":      "",
		}

		state.GeneratedConfigs["gateway"] = gatewayValues
		state.GeneratedConfigs["blog-agent"] = blogValues
		state.SelectedAgents = []string{"gateway", "blog-agent"}

		if err := state.WriteAllConfigs(); err != nil {
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}

		hub.broadcast("quickstart_complete", map[string]any{
			"agents":  []string{"gateway", "blog-agent"},
			"written": state.WrittenFiles,
		})

		writeJSON(w, map[string]any{
			"success": true,
			"written": state.WrittenFiles,
			"message": "核心 agent 配置完成",
		})
	})

	// API: Check online agents via gateway
	mux.HandleFunc("/api/agents/online", func(w http.ResponseWriter, r *http.Request) {
		gatewayHTTP := cfg.GatewayHTTP
		if gatewayHTTP == "" {
			gatewayHTTP = "http://127.0.0.1:10086"
		}

		agentsURL := gatewayHTTP + "/api/gateway/agents"
		resp, err := http.Get(agentsURL)
		if err != nil {
			writeJSON(w, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("无法连接 Gateway: %v", err),
				"online":  []string{},
			})
			return
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			writeJSON(w, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("解析 Gateway 响应失败: %v", err),
				"online":  []string{},
			})
			return
		}

		writeJSON(w, map[string]any{
			"success":         true,
			"gateway_response": result,
		})
	})

	addr := fmt.Sprintf(":%d", cfg.WebPort)
	fmt.Printf("[init-agent] Web 向导已启动: http://localhost:%d\n", cfg.WebPort)
	return http.ListenAndServe(addr, mux)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
