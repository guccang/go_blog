package taskbreakdown

import (
	log "mylog"
)

// InitTaskBreakdown 初始化任务拆解功能
func InitTaskBreakdown() error {
	// 创建任务管理器
	manager := NewTaskManager()

	// 创建控制器
	controller := NewController(manager)

	// 为处理器设置控制器
	SetController(controller)

	log.InfoF(log.ModuleTaskBreakdown, "Task breakdown system initialized successfully")
	return nil
}