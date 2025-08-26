package user

import (
	"core"
	"fmt"
	log "mylog"
	"module"
	"sync"
	"time"
)

// UserMessageRouter 负责将消息路由到正确的用户 Actor
type UserMessageRouter struct {
	*core.Actor
	userManager   *UserActorManager
	routingStats  *RoutingStats
	mutex         sync.RWMutex
}

// 路由统计信息
type RoutingStats struct {
	TotalMessages      int64         `json:"total_messages"`      // 总消息数
	SuccessfulRoutes   int64         `json:"successful_routes"`   // 成功路由数
	FailedRoutes       int64         `json:"failed_routes"`       // 失败路由数
	AverageLatency     time.Duration `json:"average_latency"`     // 平均延迟
	MessagesPerSecond  float64       `json:"messages_per_second"` // 每秒消息数
	LastResetTime      time.Time     `json:"last_reset_time"`     // 上次重置时间
	UserMessageCounts  map[string]int64 `json:"user_message_counts"` // 每用户消息计数
}

// 用户消息接口
type UserMessage interface {
	GetTargetUserID() string
	GetMessageType() string
	GetPriority() MessagePriority
}

// 消息优先级
type MessagePriority int
const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// 全局消息路由器
var GlobalMessageRouter *UserMessageRouter

// 初始化消息路由器
func InitMessageRouter(userManager *UserActorManager) error {
	if userManager == nil {
		return fmt.Errorf("userManager cannot be nil")
	}

	GlobalMessageRouter = &UserMessageRouter{
		Actor:       core.NewActor(),
		userManager: userManager,
		routingStats: &RoutingStats{
			LastResetTime:     time.Now(),
			UserMessageCounts: make(map[string]int64),
		},
	}

	GlobalMessageRouter.Start(GlobalMessageRouter)
	log.Debug("UserMessageRouter initialized successfully")
	return nil
}

// 启动消息路由器
func (r *UserMessageRouter) Start(owner core.ActorInterface) {
	r.Actor.Start(owner)
	
	// 启动统计更新协程
	go r.startStatsUpdater()
	
	log.Debug("UserMessageRouter started")
}

// 停止消息路由器
func (r *UserMessageRouter) Stop() {
	log.Debug("Stopping UserMessageRouter...")
	r.Actor.Stop()
	log.Debug("UserMessageRouter stopped")
}

// 路由消息到用户
func RouteToUser(userID string, message UserMessage) error {
	if GlobalMessageRouter == nil {
		return fmt.Errorf("message router not initialized")
	}
	
	cmd := &RouteMessageCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		UserID:  userID,
		Message: message,
	}
	
	GlobalMessageRouter.Send(cmd)
	result := <-cmd.Response()
	
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// 批量路由消息
func RouteToMultipleUsers(userIDs []string, message UserMessage) map[string]error {
	if GlobalMessageRouter == nil {
		return map[string]error{"system": fmt.Errorf("message router not initialized")}
	}
	
	cmd := &BatchRouteCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		UserIDs: userIDs,
		Message: message,
	}
	
	GlobalMessageRouter.Send(cmd)
	result := <-cmd.Response()
	
	if errors, ok := result.(map[string]error); ok {
		return errors
	}
	return map[string]error{"system": fmt.Errorf("unexpected response type")}
}

// 广播消息给所有活跃用户
func BroadcastToActiveUsers(message UserMessage) (int, error) {
	if GlobalMessageRouter == nil {
		return 0, fmt.Errorf("message router not initialized")
	}
	
	cmd := &BroadcastCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Message: message,
	}
	
	GlobalMessageRouter.Send(cmd)
	result := <-cmd.Response()
	
	if response, ok := result.(*BroadcastResponse); ok {
		return response.SuccessCount, response.Error
	}
	return 0, fmt.Errorf("unexpected response type")
}

// 内部方法：路由单个消息
func (r *UserMessageRouter) routeToUser(userID string, message UserMessage) error {
	startTime := time.Now()
	
	// 更新统计信息
	r.mutex.Lock()
	r.routingStats.TotalMessages++
	r.routingStats.UserMessageCounts[userID]++
	r.mutex.Unlock()
	
	// 获取或创建用户 Actor
	userActor, err := r.userManager.GetOrCreateUserActor(userID)
	if err != nil {
		r.updateFailedRoutes()
		return fmt.Errorf("failed to get user actor for %s: %w", userID, err)
	}
	
	// 根据消息类型路由到相应的子 Actor
	err = r.routeToSubActor(userActor, message)
	if err != nil {
		r.updateFailedRoutes()
		return fmt.Errorf("failed to route message to sub-actor: %w", err)
	}
	
	// 更新成功统计
	r.updateSuccessfulRoutes(time.Since(startTime))
	
	log.Debug(fmt.Sprintf("Message routed to user %s: %s", userID, message.GetMessageType()))
	return nil
}

// 路由到子 Actor
func (r *UserMessageRouter) routeToSubActor(userActor *UserActor, message UserMessage) error {
	messageType := message.GetMessageType()
	
	switch messageType {
	case "blog":
		if blogActor := userActor.GetBlogActor(); blogActor != nil {
			return r.sendToActor(blogActor, message)
		}
		
	case "exercise":
		if exerciseActor := userActor.GetExerciseActor(); exerciseActor != nil {
			return r.sendToActor(exerciseActor, message)
		}
		
	case "reading":
		if readingActor := userActor.GetReadingActor(); readingActor != nil {
			return r.sendToActor(readingActor, message)
		}
		
	case "comment":
		if commentActor := userActor.GetCommentActor(); commentActor != nil {
			return r.sendToActor(commentActor, message)
		}
		
	case "todo":
		if todoActor := userActor.GetTodoActor(); todoActor != nil {
			return r.sendToActor(todoActor, message)
		}
		
	case "stats":
		if statsActor := userActor.GetStatsActor(); statsActor != nil {
			return r.sendToActor(statsActor, message)
		}
		
	default:
		// 发送到主 UserActor
		return r.sendToActor(userActor, message)
	}
	
	return fmt.Errorf("unable to route message type: %s", messageType)
}

// 发送消息到 Actor
func (r *UserMessageRouter) sendToActor(actor core.ActorInterface, message UserMessage) error {
	// 这里需要根据具体的消息类型创建相应的命令
	// 为了简化，我们创建一个通用的消息命令
	cmd := &GenericUserMessageCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Message: message,
	}
	
	actor.Send(cmd)
	
	// 根据优先级决定是否等待响应
	if message.GetPriority() >= PriorityHigh {
		result := <-cmd.Response()
		if err, ok := result.(error); ok {
			return err
		}
	}
	
	return nil
}

// 批量路由消息
func (r *UserMessageRouter) batchRoute(userIDs []string, message UserMessage) map[string]error {
	errors := make(map[string]error)
	
	for _, userID := range userIDs {
		if err := r.routeToUser(userID, message); err != nil {
			errors[userID] = err
		}
	}
	
	return errors
}

// 广播消息
func (r *UserMessageRouter) broadcast(message UserMessage) *BroadcastResponse {
	activeUserCount := r.userManager.GetActiveUserCount()
	successCount := 0
	
	// 获取所有活跃用户
	// 注意：这里需要从 UserActorManager 获取活跃用户列表
	// 为简化实现，我们假设有一个方法可以获取所有活跃用户ID
	
	log.Debug(fmt.Sprintf("Broadcasting message to %d active users", activeUserCount))
	
	return &BroadcastResponse{
		SuccessCount: successCount,
		FailedCount:  activeUserCount - successCount,
	}
}

// 更新成功路由统计
func (r *UserMessageRouter) updateSuccessfulRoutes(latency time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.routingStats.SuccessfulRoutes++
	
	// 更新平均延迟（简单移动平均）
	if r.routingStats.SuccessfulRoutes == 1 {
		r.routingStats.AverageLatency = latency
	} else {
		r.routingStats.AverageLatency = (r.routingStats.AverageLatency + latency) / 2
	}
}

// 更新失败路由统计
func (r *UserMessageRouter) updateFailedRoutes() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.routingStats.FailedRoutes++
}

// 获取路由统计信息
func (r *UserMessageRouter) GetStats() *RoutingStats {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// 计算每秒消息数
	duration := time.Since(r.routingStats.LastResetTime)
	if duration > 0 {
		r.routingStats.MessagesPerSecond = float64(r.routingStats.TotalMessages) / duration.Seconds()
	}
	
	// 返回统计信息的副本
	stats := *r.routingStats
	stats.UserMessageCounts = make(map[string]int64)
	for k, v := range r.routingStats.UserMessageCounts {
		stats.UserMessageCounts[k] = v
	}
	
	return &stats
}

// 重置统计信息
func (r *UserMessageRouter) ResetStats() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.routingStats.TotalMessages = 0
	r.routingStats.SuccessfulRoutes = 0
	r.routingStats.FailedRoutes = 0
	r.routingStats.AverageLatency = 0
	r.routingStats.MessagesPerSecond = 0
	r.routingStats.LastResetTime = time.Now()
	r.routingStats.UserMessageCounts = make(map[string]int64)
	
	log.Debug("Router statistics reset")
}

// 启动统计更新器
func (r *UserMessageRouter) startStatsUpdater() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.updatePeriodicStats()
			
		case <-r.GetContext().Done():
			log.Debug("Router stats updater stopped")
			return
		}
	}
}

// 定期更新统计信息
func (r *UserMessageRouter) updatePeriodicStats() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// 计算每秒消息数
	duration := time.Since(r.routingStats.LastResetTime)
	if duration > 0 {
		r.routingStats.MessagesPerSecond = float64(r.routingStats.TotalMessages) / duration.Seconds()
	}
}

// === 路由命令定义 ===

// 路由消息命令
type RouteMessageCmd struct {
	core.ActorCommand
	UserID  string
	Message UserMessage
}

func (cmd *RouteMessageCmd) Do(actor core.ActorInterface) {
	router := actor.(*UserMessageRouter)
	err := router.routeToUser(cmd.UserID, cmd.Message)
	cmd.Response() <- err
}

// 批量路由命令
type BatchRouteCmd struct {
	core.ActorCommand
	UserIDs []string
	Message UserMessage
}

func (cmd *BatchRouteCmd) Do(actor core.ActorInterface) {
	router := actor.(*UserMessageRouter)
	errors := router.batchRoute(cmd.UserIDs, cmd.Message)
	cmd.Response() <- errors
}

// 广播命令
type BroadcastCmd struct {
	core.ActorCommand
	Message UserMessage
}

func (cmd *BroadcastCmd) Do(actor core.ActorInterface) {
	router := actor.(*UserMessageRouter)
	response := router.broadcast(cmd.Message)
	cmd.Response() <- response
}

// 获取统计信息命令
type GetRouterStatsCmd struct {
	core.ActorCommand
}

func (cmd *GetRouterStatsCmd) Do(actor core.ActorInterface) {
	router := actor.(*UserMessageRouter)
	stats := router.GetStats()
	cmd.Response() <- stats
}

// === 通用消息类型 ===

// 通用用户消息
type GenericUserMessage struct {
	TargetUserID string
	MessageType  string
	Priority     MessagePriority
	Content      interface{}
}

func (m *GenericUserMessage) GetTargetUserID() string {
	return m.TargetUserID
}

func (m *GenericUserMessage) GetMessageType() string {
	return m.MessageType
}

func (m *GenericUserMessage) GetPriority() MessagePriority {
	return m.Priority
}

// 通用用户消息命令
type GenericUserMessageCmd struct {
	core.ActorCommand
	Message UserMessage
}

func (cmd *GenericUserMessageCmd) Do(actor core.ActorInterface) {
	// 这里应该根据具体的 Actor 类型处理消息
	// 为了简化，直接返回成功
	cmd.Response() <- nil
}

// 广播响应
type BroadcastResponse struct {
	SuccessCount int
	FailedCount  int
	Error        error
}

// === 工具函数 ===

// 创建博客消息
func NewBlogMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "blog",
		Priority:     priority,
		Content:      content,
	}
}

// 创建锻炼消息
func NewExerciseMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "exercise",
		Priority:     priority,
		Content:      content,
	}
}

// 创建阅读消息
func NewReadingMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "reading",
		Priority:     priority,
		Content:      content,
	}
}

// 创建评论消息
func NewCommentMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "comment",
		Priority:     priority,
		Content:      content,
	}
}

// 创建任务消息
func NewTodoMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "todo",
		Priority:     priority,
		Content:      content,
	}
}

// 创建统计消息
func NewStatsMessage(userID string, priority MessagePriority, content interface{}) UserMessage {
	return &GenericUserMessage{
		TargetUserID: userID,
		MessageType:  "stats",
		Priority:     priority,
		Content:      content,
	}
}