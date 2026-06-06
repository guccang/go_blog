package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type clientConfig struct {
	BaseURL      string
	UserID       string
	Password     string
	ReceiveToken string
}

type authSession struct {
	Success      bool   `json:"success"`
	SessionToken string `json:"session_token"`
	AccessToken  string `json:"access_token"`
	UserID       string `json:"user_id"`
	Error        string `json:"error"`
}

type pushEnvelope struct {
	MessageID   string         `json:"message_id"`
	Sequence    int64          `json:"sequence"`
	UserID      string         `json:"user_id"`
	Content     string         `json:"content"`
	MessageType string         `json:"message_type"`
	Channel     string         `json:"channel"`
	Timestamp   int64          `json:"timestamp"`
	Meta        map[string]any `json:"meta"`
}

type appClient struct {
	cfg          clientConfig
	sessionToken string
	httpClient   *http.Client
	conn         *websocket.Conn
	logger       *messageLogger
	writeMu      sync.Mutex
}

func newAppClient(cfg clientConfig, logger *messageLogger) *appClient {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.UserID = strings.TrimSpace(cfg.UserID)
	cfg.ReceiveToken = strings.TrimSpace(cfg.ReceiveToken)
	logger.addSecrets(cfg.Password, cfg.ReceiveToken)
	return &appClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

func (c *appClient) loginAndConnect(ctx context.Context) error {
	if c.cfg.BaseURL == "" || c.cfg.UserID == "" || strings.TrimSpace(c.cfg.Password) == "" {
		return fmt.Errorf("base URL, user ID and password are required")
	}

	payload := map[string]string{
		"user_id":  c.cfg.UserID,
		"password": c.cfg.Password,
	}
	var session authSession
	if err := c.doJSON(ctx, http.MethodPost, "/api/app/login", "", payload, &session); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if !session.Success {
		if session.Error == "" {
			session.Error = "app-agent rejected login"
		}
		return fmt.Errorf("login: %s", session.Error)
	}
	c.sessionToken = strings.TrimSpace(session.SessionToken)
	if c.sessionToken == "" {
		c.sessionToken = strings.TrimSpace(session.AccessToken)
	}
	if c.sessionToken == "" {
		return fmt.Errorf("login: response missing session token")
	}
	c.logger.addSecrets(c.sessionToken)

	wsURL, err := buildWebSocketURL(c.cfg.BaseURL, c.cfg.UserID, c.sessionToken, c.cfg.ReceiveToken)
	if err != nil {
		return err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if resp != nil {
			err = fmt.Errorf("connect websocket: HTTP %d: %w", resp.StatusCode, err)
			c.logger.log("system", "websocket", "connect", c.cfg.UserID, map[string]any{"base_url": c.cfg.BaseURL}, err)
			return err
		}
		err = fmt.Errorf("connect websocket: %w", err)
		c.logger.log("system", "websocket", "connect", c.cfg.UserID, map[string]any{"base_url": c.cfg.BaseURL}, err)
		return err
	}
	c.conn = conn
	c.logger.log("system", "websocket", "connect", c.cfg.UserID, map[string]any{"base_url": c.cfg.BaseURL}, nil)
	return nil
}

func (c *appClient) sendMessage(ctx context.Context, content string) error {
	payload := map[string]any{
		"user_id":      c.cfg.UserID,
		"content":      content,
		"message_type": "text",
	}
	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/app/message", c.sessionToken, payload, &response); err != nil {
		return err
	}
	if !response.Success {
		if response.Error == "" {
			response.Error = "app-agent rejected message"
		}
		return fmt.Errorf("%s", response.Error)
	}
	return nil
}

func (c *appClient) readMessage() (pushEnvelope, error) {
	var envelope pushEnvelope
	if c.conn == nil {
		return envelope, fmt.Errorf("websocket is not connected")
	}
	if err := c.conn.ReadJSON(&envelope); err != nil {
		c.logger.log("received", "websocket", "message", c.cfg.UserID, nil, err)
		return envelope, err
	}
	c.logger.log("received", "websocket", "message", c.cfg.UserID, envelope, nil)
	if envelope.MessageID != "" {
		ack := map[string]string{
			"type":       "ack",
			"message_id": envelope.MessageID,
		}
		if err := c.writeJSON(ack); err != nil {
			c.logger.log("sent", "websocket", "ack", c.cfg.UserID, ack, err)
			return envelope, fmt.Errorf("ack message: %w", err)
		}
		c.logger.log("sent", "websocket", "ack", c.cfg.UserID, ack, nil)
	}
	return envelope, nil
}

func (c *appClient) writeJSON(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("websocket is not connected")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(value)
}

func (c *appClient) close() {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.logger.log("system", "websocket", "close", c.cfg.UserID, nil, nil)
	}
}

func (c *appClient) doJSON(ctx context.Context, method, path, sessionToken string, payload, output any) error {
	c.logger.log("sent", "http", "request", c.cfg.UserID, map[string]any{
		"method":  method,
		"path":    path,
		"payload": payload,
	}, nil)
	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.log("received", "http", "response", c.cfg.UserID, map[string]any{"method": method, "path": path}, err)
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		c.logger.log("received", "http", "response", c.cfg.UserID, map[string]any{"method": method, "path": path}, err)
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if c.cfg.ReceiveToken != "" {
		req.Header.Set("X-App-Agent-Token", c.cfg.ReceiveToken)
	}
	if sessionToken != "" {
		req.Header.Set("X-App-Agent-Session", sessionToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.log("received", "http", "response", c.cfg.UserID, map[string]any{"method": method, "path": path}, err)
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		c.logger.log("received", "http", "response", c.cfg.UserID, map[string]any{
			"method": method, "path": path, "status_code": resp.StatusCode,
		}, err)
		return err
	}
	var responsePayload any
	if len(responseBody) > 0 {
		if json.Unmarshal(responseBody, &responsePayload) != nil {
			responsePayload = strings.TrimSpace(string(responseBody))
		}
	}
	c.logger.log("received", "http", "response", c.cfg.UserID, map[string]any{
		"method": method, "path": path, "status_code": resp.StatusCode, "payload": responsePayload,
	}, nil)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if output == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func buildWebSocketURL(baseURL, userID, sessionToken, receiveToken string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	switch base.Scheme {
	case "http":
		base.Scheme = "ws"
	case "https":
		base.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported base URL scheme %q", base.Scheme)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/ws/app"
	query := base.Query()
	query.Set("user_id", userID)
	query.Set("session_token", sessionToken)
	if receiveToken != "" {
		query.Set("token", receiveToken)
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
}
