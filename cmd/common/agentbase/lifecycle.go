package agentbase

import (
	"fmt"
	"log"
	"sync"
)

// 生命周期状态常量
const (
	StateStarting = "starting"
	StateRunning  = "running"
	StateDraining = "draining"
	StateStopped  = "stopped"
)

// Lifecycle Agent 生命周期状态机
//
// 状态转换:
//
//	starting → running   (Gateway 注册成功)
//	running  → draining  (收到 ctrl_shutdown / SIGINT/SIGTERM)
//	draining → stopped   (活跃任务完成 / drain 超时)
//	running  → stopped   (force shutdown)
//	starting → stopped   (启动失败)
type Lifecycle struct {
	mu    sync.RWMutex
	state string
}

// NewLifecycle 创建生命周期状态机（初始 starting 状态）
func NewLifecycle() *Lifecycle {
	return &Lifecycle{state: StateStarting}
}

// State 返回当前状态
func (lc *Lifecycle) State() string {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.state
}

// TransitionTo 状态转换，非法转换返回 error
func (lc *Lifecycle) TransitionTo(target string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if !isValidTransition(lc.state, target) {
		return fmt.Errorf("invalid lifecycle transition: %s → %s", lc.state, target)
	}

	log.Printf("[Lifecycle] %s → %s", lc.state, target)
	lc.state = target
	return nil
}

// IsAcceptingWork 当前状态是否接受新任务
func (lc *Lifecycle) IsAcceptingWork() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.state == StateRunning
}

// isValidTransition 检查状态转换是否合法
func isValidTransition(from, to string) bool {
	switch from {
	case StateStarting:
		return to == StateRunning || to == StateStopped
	case StateRunning:
		return to == StateDraining || to == StateStopped
	case StateDraining:
		return to == StateStopped
	case StateStopped:
		return false
	default:
		return false
	}
}
