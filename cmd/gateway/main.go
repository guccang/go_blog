package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	configFile := flag.String("config", "gateway.json", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := LoadConfig(*configFile)
	if err != nil {
		// 无配置文件时使用默认值
		log.Printf("[Gateway] config file not found (%s), using defaults", *configFile)
		cfg = DefaultConfig()
	}

	log.Printf("[Gateway] starting on port %d", cfg.Port)
	log.Printf("[Gateway] go_blog upstream: %s", cfg.GoBackendURL)

	// 初始化注册表
	registry := NewRegistry()

	// 初始化事件追踪器
	var tracker *Tracker
	if cfg.EventTracking {
		var err2 error
		tracker, err2 = NewTracker(&TrackerConfig{
			BufferSize: cfg.EventBufferSize,
			LogDir:     cfg.EventLogDir,
			LogStdout:  cfg.EventLogStdout,
			SkipHB:     cfg.EventSkipHB,
		})
		if err2 != nil {
			log.Printf("[Gateway] event tracker init failed: %v, continuing without tracking", err2)
		} else {
			log.Printf("[Gateway] event tracking enabled (buffer=%d, dir=%s, stdout=%v, skip_hb=%v)",
				cfg.EventBufferSize, cfg.EventLogDir, cfg.EventLogStdout, cfg.EventSkipHB)
		}
	}

	// 初始化路由器（包含 UAP server）
	router := NewRouter(cfg, registry, tracker)

	// 注册 HTTP 路由
	mux := http.NewServeMux()

	// WebSocket 入口 — agent 连接
	mux.HandleFunc("/ws/uap", router.HandleUAP)

	// 管理 API
	mux.HandleFunc("/api/gateway/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := registry.GetAllAgents()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"agents":  agents,
		})
	})

	mux.HandleFunc("/api/gateway/tools", func(w http.ResponseWriter, r *http.Request) {
		tools := registry.GetAllTools()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"tools":   tools,
		})
	})

	mux.HandleFunc("/api/gateway/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"agents":  registry.OnlineCount(),
		})
	})

	// 事件追踪 API
	if tracker != nil {
		mux.HandleFunc("/api/gateway/events/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"stats":   tracker.Stats(),
			})
		})

		mux.HandleFunc("/api/gateway/events/trace/", func(w http.ResponseWriter, r *http.Request) {
			// 从路径中提取 traceID: /api/gateway/events/trace/{traceID}
			traceID := strings.TrimPrefix(r.URL.Path, "/api/gateway/events/trace/")
			if traceID == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "trace_id required",
				})
				return
			}
			events := tracker.GetTrace(traceID)

			// 计算 trace 总耗时和状态
			var durationMs int64
			status := "in_progress"
			for _, e := range events {
				if e.DurationMs > durationMs {
					durationMs = e.DurationMs
				}
				if e.MsgType == "tool_result" || e.MsgType == "task_complete" {
					status = "completed"
				}
				if e.Kind == EventKindRouteErr || e.Error != "" {
					status = "error"
				}
			}
			if len(events) == 0 {
				status = "not_found"
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success":     true,
				"trace_id":    traceID,
				"events":      events,
				"duration_ms": durationMs,
				"status":      status,
			})
		})

		mux.HandleFunc("/api/gateway/events", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			limit := 100
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					limit = n
					if limit > 1000 {
						limit = 1000
					}
				}
			}
			offset := 0
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 0 {
					offset = n
				}
			}
			var since, until int64
			if v := q.Get("since"); v != "" {
				since, _ = strconv.ParseInt(v, 10, 64)
			}
			if v := q.Get("until"); v != "" {
				until, _ = strconv.ParseInt(v, 10, 64)
			}

			query := &EventQuery{
				TraceID: q.Get("trace_id"),
				Agent:   q.Get("agent"),
				Kind:    q.Get("kind"),
				MsgType: q.Get("msg_type"),
				Since:   since,
				Until:   until,
				Limit:   limit,
				Offset:  offset,
			}

			events, total := tracker.Query(query)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success":         true,
				"events":          events,
				"total":           total,
				"buffer_capacity": tracker.ring.size,
				"buffer_used":     tracker.ring.Used(),
			})
		})
	}

	// HTTP 反向代理 — 将其余请求转发到 go_blog
	proxy := NewProxy(cfg.GoBackendURL)
	mux.Handle("/", proxy)

	// 启动心跳检测
	router.StartHealthCheck()

	// 启动 HTTP 服务
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[Gateway] shutting down...")
		if tracker != nil {
			tracker.Close()
		}
		server.Close()
	}()

	log.Printf("[Gateway] listening on :%d", cfg.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[Gateway] server error: %v", err)
	}
	log.Println("[Gateway] stopped")
}
