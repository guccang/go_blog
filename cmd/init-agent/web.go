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

		go func() {
			reqs := DefaultRequirements()
			for _, req := range reqs {
				result := checkOne(req)
				state.EnvResults = append(state.EnvResults, result)
				hub.broadcast("env_check_result", result)
			}
			hub.broadcast("env_check_complete", state.EnvResults)
		}()

		writeJSON(w, map[string]any{"success": true, "message": "环境检测已开始"})
	})

	// API: Get cached environment results
	mux.HandleFunc("/api/env/results", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"success": true, "results": state.EnvResults})
	})

	// API: Get all agent schemas
	mux.HandleFunc("/api/agents/schemas", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"success": true, "schemas": AllAgentSchemas()})
	})

	// API: Get existing configs
	mux.HandleFunc("/api/agents/configs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			configs := make(map[string]map[string]any)
			for _, schema := range AllAgentSchemas() {
				existing, _ := LoadExistingConfig(cfg.RootDir, &schema)
				if existing != nil {
					configs[schema.Name] = existing
				}
			}
			writeJSON(w, map[string]any{"success": true, "configs": configs})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Agents map[string]map[string]string `json:"agents"`
				Shared map[string]string            `json:"shared"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]any{"success": false, "error": err.Error()})
				return
			}

			state.SharedValues = req.Shared
			var written []string

			for agentName, vals := range req.Agents {
				schema := GetAgentSchema(agentName)
				if schema == nil {
					continue
				}
				state.AgentValues[agentName] = vals
				state.MergeAndStoreConfig(schema)

				path, err := WriteAgentConfig(cfg.RootDir, schema, state.GeneratedConfigs[agentName])
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

		go func() {
			layers := RunAvailabilityChecks(cfg.RootDir, state.GeneratedConfigs)
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
			"success":      true,
			"current_step": state.CurrentStep,
			"root_dir":     state.RootDir,
			"env_results":  state.EnvResults,
			"selected":     state.SelectedAgents,
			"availability": state.AvailabilityLayers,
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
