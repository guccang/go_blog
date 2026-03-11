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
}

// NewRouter 创建路由器
func NewRouter(cfg *Config, registry *Registry) *Router {
	server := uap.NewServer()
	server.AuthToken = cfg.AuthToken

	// 绑定注册表
	registry.SetServer(server)

	// 处理无 To 字段的消息
	server.OnMessage = func(from *uap.AgentConn, msg *uap.Message) {
		log.Printf("[Router] message from %s with empty To (type=%s), dropping", from.ID, msg.Type)
	}

	return &Router{
		cfg:      cfg,
		registry: registry,
		server:   server,
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
