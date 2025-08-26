package user

import (
	"fmt"
	log "mylog"
	"module"
)

// Package level information
func Info() {
	log.Debug("info user v1.0 - Multi-User Actor System")
}

// 初始化用户 Actor 系统
func Init() error {
	// 初始化 UserActorManager
	config := GetDefaultUserActorConfig()
	err := InitUserActorManager(config)
	if err != nil {
		return fmt.Errorf("failed to initialize UserActorManager: %w", err)
	}
	
	// 初始化消息路由器
	err = InitMessageRouter(GlobalUserActorManager)
	if err != nil {
		return fmt.Errorf("failed to initialize MessageRouter: %w", err)
	}
	
	log.Debug("User Actor System initialized successfully")
	return nil
}

// 关闭用户 Actor 系统
func Shutdown() {
	log.Debug("Shutting down User Actor System...")
	
	// 停止消息路由器
	if GlobalMessageRouter != nil {
		GlobalMessageRouter.Stop()
	}
	
	// 停止 UserActorManager
	if GlobalUserActorManager != nil {
		GlobalUserActorManager.Stop()
	}
	
	log.Debug("User Actor System shutdown complete")
}

// === 用户 Actor 管理接口 ===

// 获取用户 Actor
func GetUserActor(userID string) (*UserActor, error) {
	if GlobalUserActorManager == nil {
		return nil, fmt.Errorf("user actor manager not initialized")
	}
	
	return GlobalUserActorManager.GetOrCreateUserActor(userID)
}

// 移除用户 Actor
func RemoveUserActor(userID string) error {
	if GlobalUserActorManager == nil {
		return fmt.Errorf("user actor manager not initialized")
	}
	
	return GlobalUserActorManager.RemoveUserActor(userID)
}

// 检查用户是否活跃
func IsUserActive(userID string) bool {
	if GlobalUserActorManager == nil {
		return false
	}
	
	return GlobalUserActorManager.IsUserActive(userID)
}

// 获取活跃用户数量
func GetActiveUserCount() int {
	if GlobalUserActorManager == nil {
		return 0
	}
	
	return GlobalUserActorManager.GetActiveUserCount()
}

// 获取总用户数量
func GetTotalUserCount() int {
	if GlobalUserActorManager == nil {
		return 0
	}
	
	return GlobalUserActorManager.GetTotalUserCount()
}

// 获取系统统计信息
func GetSystemStats() *UserActorManagerStats {
	if GlobalUserActorManager == nil {
		return nil
	}
	
	return GlobalUserActorManager.GetStats()
}

// === 消息路由接口 ===

// 发送消息给指定用户
func SendMessageToUser(userID string, message UserMessage) error {
	return RouteToUser(userID, message)
}

// 发送消息给多个用户
func SendMessageToUsers(userIDs []string, message UserMessage) map[string]error {
	return RouteToMultipleUsers(userIDs, message)
}

// 广播消息给所有活跃用户
func BroadcastMessage(message UserMessage) (int, error) {
	return BroadcastToActiveUsers(message)
}

// 获取消息路由统计信息
func GetMessageRouterStats() *RoutingStats {
	if GlobalMessageRouter == nil {
		return nil
	}
	
	return GlobalMessageRouter.GetStats()
}

// 重置消息路由统计信息
func ResetMessageRouterStats() {
	if GlobalMessageRouter != nil {
		GlobalMessageRouter.ResetStats()
	}
}

// === 用户博客操作接口 ===

// 创建用户博客
func CreateUserBlog(userID, title, content string, authType int, tags string) error {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.CreateBlog(title, content, authType, tags)
}

// 获取用户博客
func GetUserBlog(userID, title string) (*module.Blog, error) {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return nil, err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return nil, fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.GetBlog(title)
}

// 获取用户所有博客
func GetUserAllBlogs(userID string) (map[string]*module.Blog, error) {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return nil, err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return nil, fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.GetAllBlogs(), nil
}

// 更新用户博客
func UpdateUserBlog(userID, title, content string, authType int, tags string) error {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.UpdateBlog(title, content, authType, tags)
}

// 删除用户博客
func DeleteUserBlog(userID, title string) error {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.DeleteBlog(title)
}

// 搜索用户博客
func SearchUserBlogs(userID, keyword string) ([]*module.Blog, error) {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return nil, err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return nil, fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.SearchBlogs(keyword), nil
}

// 获取用户博客统计信息
func GetUserBlogStats(userID string) (map[string]interface{}, error) {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return nil, err
	}
	
	blogActor := userActor.GetBlogActor()
	if blogActor == nil {
		return nil, fmt.Errorf("blog actor not available for user: %s", userID)
	}
	
	return blogActor.GetStats(), nil
}

// === 健康检查接口 ===

// 检查用户 Actor 健康状态
func CheckUserActorHealth(userID string) (map[string]interface{}, error) {
	userActor, err := GetUserActor(userID)
	if err != nil {
		return nil, err
	}
	
	return userActor.HealthCheck(), nil
}

// 检查系统整体健康状态
func CheckSystemHealth() map[string]interface{} {
	health := map[string]interface{}{
		"system": "user_actor_system",
		"version": "1.0",
		"status": "unknown",
	}
	
	// 检查 UserActorManager
	if GlobalUserActorManager != nil {
		health["user_manager"] = map[string]interface{}{
			"status": "running",
			"active_users": GlobalUserActorManager.GetActiveUserCount(),
			"total_users": GlobalUserActorManager.GetTotalUserCount(),
		}
	} else {
		health["user_manager"] = map[string]interface{}{
			"status": "not_initialized",
		}
	}
	
	// 检查 MessageRouter
	if GlobalMessageRouter != nil {
		stats := GlobalMessageRouter.GetStats()
		health["message_router"] = map[string]interface{}{
			"status": "running",
			"total_messages": stats.TotalMessages,
			"successful_routes": stats.SuccessfulRoutes,
			"failed_routes": stats.FailedRoutes,
			"messages_per_second": stats.MessagesPerSecond,
		}
	} else {
		health["message_router"] = map[string]interface{}{
			"status": "not_initialized",
		}
	}
	
	// 确定整体状态
	if GlobalUserActorManager != nil && GlobalMessageRouter != nil {
		health["status"] = "healthy"
	} else {
		health["status"] = "unhealthy"
	}
	
	return health
}

// === 配置管理接口 ===

// 获取当前配置
func GetCurrentConfig() *module.UserActorConfig {
	if GlobalUserActorManager != nil && GlobalUserActorManager.config != nil {
		return GlobalUserActorManager.config
	}
	return GetDefaultUserActorConfig()
}

// 更新配置（注意：某些配置更改可能需要重启系统）
func UpdateConfig(newConfig *module.UserActorConfig) error {
	if GlobalUserActorManager == nil {
		return fmt.Errorf("user actor manager not initialized")
	}
	
	if newConfig == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	// 更新配置
	GlobalUserActorManager.config = newConfig
	
	log.Debug("UserActor system configuration updated")
	return nil
}