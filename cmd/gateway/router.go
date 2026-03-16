package main

import (
	"log"
	"net/http"
	"time"

	"uap"
)

// Router 消息路由器
type Router struct {
	cfg      *Config
	registry *Registry
	server   *uap.Server
	tracker  *Tracker
}

// NewRouter 创建路由器
func NewRouter(cfg *Config, registry *Registry, tracker *Tracker) *Router {
	server := uap.NewServer()
	server.AuthToken = cfg.AuthToken

	// 绑定注册表（含 tracker）
	registry.SetServer(server, tracker)

	// 处理无 To 字段的消息
	server.OnMessage = func(from *uap.AgentConn, msg *uap.Message) {
		log.Printf("[Router] message from %s with empty To (type=%s), dropping", from.ID, msg.Type)
	}

	// 绑定事件追踪回调
	if tracker != nil {
		server.OnMessageReceived = func(from *uap.AgentConn, msg *uap.Message) {
			tracker.RecordMessage(EventKindMsgIn, from, nil, msg)
		}
		server.OnMessageForwarded = func(from *uap.AgentConn, to *uap.AgentConn, msg *uap.Message) {
			tracker.RecordMessage(EventKindMsgOut, from, to, msg)
		}
		server.OnRouteError = func(from *uap.AgentConn, msg *uap.Message) {
			tracker.RecordMessage(EventKindRouteErr, from, nil, msg)
		}
		server.OnHeartbeatTimeout = func(agent *uap.AgentConn) {
			tracker.RecordLifecycle(EventKindHBTimeout, agent, "heartbeat timeout")
		}
	}

	return &Router{
		cfg:      cfg,
		registry: registry,
		server:   server,
		tracker:  tracker,
	}
}

// HandleUAP WebSocket 入口
func (r *Router) HandleUAP(w http.ResponseWriter, req *http.Request) {
	r.server.HandleWebSocket(w, req)
}

// StartHealthCheck 启动心跳检测
func (r *Router) StartHealthCheck() {
	r.registry.StartHealthCheck(120 * time.Second)
}
