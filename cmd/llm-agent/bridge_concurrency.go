package main

import (
	"log"
	"time"
)

// ========================= 并发控制 =========================

// canAccept 是否可以接受新任务
func (b *Bridge) canAccept() bool {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	return len(b.activeTasks) < b.cfg.MaxConcurrent
}

// registerTask 注册活跃任务
func (b *Bridge) registerTask(taskID, taskType string) {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	b.activeTasks[taskID] = taskType
	log.Printf("[Bridge] task registered: %s (type=%s, active=%d/%d)", taskID, taskType, len(b.activeTasks), b.cfg.MaxConcurrent)
}

// deregisterTask 注销活跃任务，并尝试从队列消费下一个
func (b *Bridge) deregisterTask(taskID string) {
	b.activeTaskMu.Lock()
	delete(b.activeTasks, taskID)
	active := len(b.activeTasks)
	b.activeTaskMu.Unlock()
	log.Printf("[Bridge] task deregistered: %s (active=%d/%d)", taskID, active, b.cfg.MaxConcurrent)
	b.drainQueue()
}

// activeCount 当前活跃任务数
func (b *Bridge) activeCount() int {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	return len(b.activeTasks)
}

// loadFactor 负载因子 0.0~1.0
func (b *Bridge) loadFactor() float64 {
	if b.cfg.MaxConcurrent <= 0 {
		return 1.0
	}
	return float64(b.activeCount()) / float64(b.cfg.MaxConcurrent)
}

// enqueueOrReject 非阻塞入队，队列满时返回 false
func (b *Bridge) enqueueOrReject(qt *queuedTask) bool {
	select {
	case b.taskQueue <- qt:
		log.Printf("[Bridge] task enqueued: %s (type=%s, queueLen=%d/%d)", qt.taskID, qt.taskType, len(b.taskQueue), b.cfg.TaskQueueSize)
		return true
	default:
		log.Printf("[Bridge] task queue full, rejecting: %s (type=%s)", qt.taskID, qt.taskType)
		return false
	}
}

// drainQueue 从队列取出一个可执行任务并启动
func (b *Bridge) drainQueue() {
	if !b.canAccept() {
		return
	}
	select {
	case qt := <-b.taskQueue:
		log.Printf("[Bridge] task dequeued: %s (type=%s, queueLen=%d)", qt.taskID, qt.taskType, len(b.taskQueue))
		b.registerTask(qt.taskID, qt.taskType)
		go func() {
			defer b.deregisterTask(qt.taskID)
			qt.handler()
		}()
	default:
		// 队列为空
	}
}

// StartQueueConsumer 后台定时消费队列（兜底，正常流程靠 deregisterTask 触发 drainQueue）
func (b *Bridge) StartQueueConsumer() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-b.queueDone:
				return
			case <-ticker.C:
				b.drainQueue()
			}
		}
	}()
	log.Printf("[Bridge] queue consumer started (MaxConcurrent=%d TaskQueueSize=%d)", b.cfg.MaxConcurrent, b.cfg.TaskQueueSize)
}
