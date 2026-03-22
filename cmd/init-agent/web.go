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

	addr := fmt.Sprintf(":%d", cfg.WebPort)
	fmt.Printf("[init-agent] Web 向导已启动: http://localhost:%d\n", cfg.WebPort)
	return http.ListenAndServe(addr, mux)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
