package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type appClientConn struct {
	userID  string
	conn    *websocket.Conn
	writeMu sync.Mutex
}

type pendingDelivery struct {
	UserID      string
	Sequence    int64
	AckedAt     time.Time
	LastSentAt  time.Time
	LastAttempt time.Time
}

type pendingMessage struct {
	MessageID   string
	Content     string
	MessageType string
	Channel     string
	Timestamp   int64
	Meta        map[string]any
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Deliveries  map[string]*pendingDelivery
}

type clientEnvelope struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
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

	b.registerClient(client)

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

	conn.SetReadLimit(16 * 1024)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil
		}
		b.handleClientEnvelope(client.userID, data)
	}
}

func (b *Bridge) enqueueAndDeliver(payload AppPushPayload) error {
	return b.enqueueAndDeliverMany([]string{payload.UserID}, payload)
}

func (b *Bridge) enqueueAndDeliverMany(users []string, payload AppPushPayload) error {
	users = uniqueNonEmptyUsers(users)
	if len(users) == 0 {
		return fmt.Errorf("empty users")
	}

	now := time.Now()
	b.deliveryMu.Lock()

	messageID := payload.MessageID
	if messageID == "" {
		messageID = buildPushMessageID(strings.Join(users, "_"))
	}
	ttl := time.Duration(b.cfg.PendingMessageTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	msg := &pendingMessage{
		MessageID:   messageID,
		Content:     payload.Content,
		MessageType: payload.MessageType,
		Channel:     payload.Channel,
		Timestamp:   payload.Timestamp,
		Meta:        cloneMeta(payload.Meta),
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		Deliveries:  make(map[string]*pendingDelivery, len(users)),
	}

	for _, user := range users {
		b.nextSequence++
		msg.Deliveries[user] = &pendingDelivery{
			UserID:   user,
			Sequence: b.nextSequence,
		}
		b.pendingByUser[user] = append(b.pendingByUser[user], messageID)
		b.trimPendingForUserLocked(user)
	}
	b.pendingMessages[messageID] = msg

	clients := make([]*appClientConn, 0, len(users))
	for _, user := range users {
		for client := range b.clients[user] {
			clients = append(clients, client)
		}
	}
	b.deliveryMu.Unlock()

	log.Printf("[WS] enqueue message id=%s users=%v type=%s len=%d",
		messageID, users, payload.MessageType, len(payload.Content))

	for _, client := range clients {
		if err := b.flushPendingToClient(client); err != nil {
			log.Printf("[WS] flush pending failed for %s: %v", client.userID, err)
		}
	}
	return nil
}

func uniqueNonEmptyUsers(users []string) []string {
	seen := make(map[string]struct{}, len(users))
	out := make([]string, 0, len(users))
	for _, user := range users {
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		if _, ok := seen[user]; ok {
			continue
		}
		seen[user] = struct{}{}
		out = append(out, user)
	}
	return out
}

func (b *Bridge) registerClient(client *appClientConn) {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	if b.clients[client.userID] == nil {
		b.clients[client.userID] = make(map[*appClientConn]struct{})
	}
	b.clients[client.userID][client] = struct{}{}
}

func (b *Bridge) unregisterClient(client *appClientConn) {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	userClients := b.clients[client.userID]
	if len(userClients) == 0 {
		return
	}
	delete(userClients, client)
	if len(userClients) == 0 {
		delete(b.clients, client.userID)
	}
}

func (b *Bridge) flushPendingToClient(client *appClientConn) error {
	queue := b.pendingForUser(client.userID)
	if len(queue) == 0 {
		return nil
	}

	for _, msg := range queue {
		if err := client.writeJSON(msg); err != nil {
			b.unregisterClient(client)
			return fmt.Errorf("write queued message: %w", err)
		}
		b.markSent(client.userID, msg.MessageID)
		log.Printf("[WS] pushed message id=%s seq=%d user=%s type=%s",
			msg.MessageID, msg.Sequence, client.userID, msg.MessageType)
	}
	return nil
}

func (b *Bridge) pendingForUser(userID string) []AppPushPayload {
	now := time.Now()
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	b.cleanupExpiredLocked(now)

	messageIDs := append([]string(nil), b.pendingByUser[userID]...)
	queue := make([]AppPushPayload, 0, len(messageIDs))
	for _, messageID := range messageIDs {
		msg := b.pendingMessages[messageID]
		if msg == nil {
			continue
		}
		delivery := msg.Deliveries[userID]
		if delivery == nil || !delivery.AckedAt.IsZero() {
			continue
		}
		queue = append(queue, AppPushPayload{
			MessageID:   msg.MessageID,
			Sequence:    delivery.Sequence,
			UserID:      userID,
			Content:     msg.Content,
			MessageType: msg.MessageType,
			Channel:     msg.Channel,
			Timestamp:   msg.Timestamp,
			Meta:        cloneMeta(msg.Meta),
		})
	}
	return queue
}

func (b *Bridge) markSent(userID, messageID string) {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	msg := b.pendingMessages[messageID]
	if msg == nil {
		return
	}
	if delivery := msg.Deliveries[userID]; delivery != nil {
		now := time.Now()
		delivery.LastSentAt = now
		delivery.LastAttempt = now
	}
}

func (b *Bridge) handleClientEnvelope(userID string, data []byte) {
	var envelope clientEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return
	}
	if envelope.Type != "ack" || strings.TrimSpace(envelope.MessageID) == "" {
		return
	}
	b.ackMessage(userID, envelope.MessageID)
}

func (b *Bridge) ackMessage(userID, messageID string) {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	msg := b.pendingMessages[messageID]
	if msg == nil {
		return
	}
	delivery := msg.Deliveries[userID]
	if delivery == nil || !delivery.AckedAt.IsZero() {
		return
	}
	delivery.AckedAt = time.Now()
	log.Printf("[WS] ack message id=%s user=%s", messageID, userID)
	b.removeAckedMessagesLocked()
}

func (b *Bridge) trimPendingForUserLocked(userID string) {
	maxPending := b.cfg.MaxPendingPerUser
	if maxPending <= 0 {
		return
	}
	queue := b.pendingByUser[userID]
	for len(queue) > maxPending {
		dropID := queue[0]
		queue = queue[1:]
		if msg := b.pendingMessages[dropID]; msg != nil {
			delete(msg.Deliveries, userID)
			if len(msg.Deliveries) == 0 {
				delete(b.pendingMessages, dropID)
			}
		}
	}
	if len(queue) == 0 {
		delete(b.pendingByUser, userID)
		return
	}
	b.pendingByUser[userID] = queue
}

func (b *Bridge) removeAckedMessagesLocked() {
	for messageID, msg := range b.pendingMessages {
		allAcked := true
		for _, delivery := range msg.Deliveries {
			if delivery.AckedAt.IsZero() {
				allAcked = false
				break
			}
		}
		if !allAcked {
			continue
		}
		b.removeMessageLocked(messageID, msg)
	}
}

func (b *Bridge) cleanupExpiredMessages() {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()
	b.cleanupExpiredLocked(time.Now())
}

func (b *Bridge) cleanupExpiredLocked(now time.Time) {
	for messageID, msg := range b.pendingMessages {
		if now.Before(msg.ExpiresAt) {
			continue
		}
		log.Printf("[WS] expired pending message id=%s created_at=%s", messageID, msg.CreatedAt.Format(time.RFC3339))
		b.removeMessageLocked(messageID, msg)
	}
}

func (b *Bridge) removeMessageLocked(messageID string, msg *pendingMessage) {
	if msg == nil {
		msg = b.pendingMessages[messageID]
		if msg == nil {
			return
		}
	}
	for userID := range msg.Deliveries {
		queue := b.pendingByUser[userID]
		if len(queue) == 0 {
			continue
		}
		filtered := queue[:0]
		for _, id := range queue {
			if id != messageID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) == 0 {
			delete(b.pendingByUser, userID)
		} else {
			b.pendingByUser[userID] = append([]string(nil), filtered...)
		}
	}
	delete(b.pendingMessages, messageID)
}

func (b *Bridge) closeAllClients() {
	b.deliveryMu.Lock()
	clients := make([]*appClientConn, 0)
	for _, userClients := range b.clients {
		for client := range userClients {
			clients = append(clients, client)
		}
	}
	b.clients = make(map[string]map[*appClientConn]struct{})
	b.deliveryMu.Unlock()

	for _, client := range clients {
		client.close()
	}
}
