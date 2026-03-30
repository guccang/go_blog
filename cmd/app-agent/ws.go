package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type appClientConn struct {
	userID  string
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (c *appClientConn) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(v)
}

func (c *appClientConn) close() {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.Close()
}

var appUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (b *Bridge) ServeWebSocket(w http.ResponseWriter, r *http.Request, userID string) error {
	conn, err := appUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("upgrade websocket: %w", err)
	}
	log.Printf("[WS] websocket upgraded user=%s remote=%s", userID, r.RemoteAddr)

	client := &appClientConn{
		userID: userID,
		conn:   conn,
	}

	oldClient := b.registerClient(client)
	if oldClient != nil {
		oldClient.close()
	}

	if err := client.writeJSON(AppPushPayload{
		Sequence:    0,
		UserID:      userID,
		Content:     "WebSocket connected.",
		MessageType: "system",
		Channel:     "app",
		Timestamp:   time.Now().UnixMilli(),
	}); err != nil {
		b.unregisterClient(client)
		client.close()
		return fmt.Errorf("write welcome message: %w", err)
	}

	if err := b.flushPendingToClient(client); err != nil {
		log.Printf("[WS] flush pending failed for %s: %v", userID, err)
	}

	log.Printf("[WS] app client connected user=%s", userID)
	defer func() {
		b.unregisterClient(client)
		client.close()
		log.Printf("[WS] app client disconnected user=%s", userID)
	}()

	conn.SetReadLimit(4096)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return nil
		}
	}
}

func (b *Bridge) enqueueAndDeliver(payload AppPushPayload) error {
	b.deliveryMu.Lock()
	b.nextSequence++
	payload.Sequence = b.nextSequence

	queue := append(b.pending[payload.UserID], payload)
	maxPending := b.cfg.MaxPendingPerUser
	if maxPending > 0 && len(queue) > maxPending {
		queue = queue[len(queue)-maxPending:]
	}
	b.pending[payload.UserID] = queue
	client := b.clients[payload.UserID]
	pendingCount := len(queue)
	b.deliveryMu.Unlock()
	log.Printf("[WS] enqueue message seq=%d user=%s pending=%d online=%v type=%s len=%d content=%q",
		payload.Sequence, payload.UserID, pendingCount, client != nil, payload.MessageType, len(payload.Content), shortText(payload.Content))
	if payload.Meta != nil {
		log.Printf("[WS] enqueue meta seq=%d user=%s group=%v from=%v origin=%v scope=%v",
			payload.Sequence,
			payload.UserID,
			payload.Meta["group_id"],
			payload.Meta["from_user"],
			payload.Meta["origin"],
			payload.Meta["scope"])
	}

	if client == nil {
		log.Printf("[WS] queued app message user=%s pending=%d", payload.UserID, pendingCount)
		return nil
	}

	return b.flushPendingToClient(client)
}

func (b *Bridge) registerClient(client *appClientConn) *appClientConn {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	oldClient := b.clients[client.userID]
	b.clients[client.userID] = client
	return oldClient
}

func (b *Bridge) unregisterClient(client *appClientConn) {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	if current := b.clients[client.userID]; current == client {
		delete(b.clients, client.userID)
	}
}

func (b *Bridge) flushPendingToClient(client *appClientConn) error {
	b.deliveryMu.Lock()
	current := b.clients[client.userID]
	if current != client {
		b.deliveryMu.Unlock()
		return nil
	}
	queue := append([]AppPushPayload(nil), b.pending[client.userID]...)
	b.deliveryMu.Unlock()

	if len(queue) == 0 {
		return nil
	}

	var lastDelivered int64
	for _, msg := range queue {
		log.Printf("[WS] push message seq=%d user=%s type=%s len=%d content=%q",
			msg.Sequence, msg.UserID, msg.MessageType, len(msg.Content), shortText(msg.Content))
		if msg.Meta != nil {
			log.Printf("[WS] push meta seq=%d user=%s group=%v from=%v origin=%v scope=%v",
				msg.Sequence,
				msg.UserID,
				msg.Meta["group_id"],
				msg.Meta["from_user"],
				msg.Meta["origin"],
				msg.Meta["scope"])
		}
		if err := client.writeJSON(msg); err != nil {
			b.unregisterClient(client)
			return fmt.Errorf("write queued message: %w", err)
		}
		lastDelivered = msg.Sequence
	}

	b.deliveryMu.Lock()
	if b.clients[client.userID] == client {
		remaining := b.pending[client.userID][:0]
		for _, msg := range b.pending[client.userID] {
			if msg.Sequence > lastDelivered {
				remaining = append(remaining, msg)
			}
		}
		if len(remaining) == 0 {
			delete(b.pending, client.userID)
		} else {
			b.pending[client.userID] = append([]AppPushPayload(nil), remaining...)
		}
	}
	b.deliveryMu.Unlock()

	log.Printf("[WS] flushed %d message(s) to %s", len(queue), client.userID)
	return nil
}

func (b *Bridge) closeAllClients() {
	b.deliveryMu.Lock()
	clients := make([]*appClientConn, 0, len(b.clients))
	for _, client := range b.clients {
		clients = append(clients, client)
	}
	b.clients = make(map[string]*appClientConn)
	b.deliveryMu.Unlock()

	for _, client := range clients {
		client.close()
	}
}
