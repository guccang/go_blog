package core

import (
	"context"
	"sync"
	"time"
)

// ========== 泛型 Actor 框架 V2 ==========
// 使用闭包+泛型简化异步调用，消除命令结构体和类型断言

// ActorV2 简化版 Actor，直接执行闭包
type ActorV2 struct {
	mailbox chan func()
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewActorV2 创建并启动 Actor
func NewActorV2() *ActorV2 {
	ctx, cancel := context.WithCancel(context.Background())
	a := &ActorV2{
		mailbox: make(chan func(), 100),
		ctx:     ctx,
		cancel:  cancel,
	}
	a.start()
	return a
}

func (a *ActorV2) start() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case fn := <-a.mailbox:
				fn() // 直接执行闭包
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

// Stop 停止 Actor
func (a *ActorV2) Stop() {
	a.cancel()
	a.wg.Wait()
	close(a.mailbox)
}

// ========== 泛型执行函数 ==========

// Execute 同步执行，返回单一类型，类型安全
func Execute[T any](a *ActorV2, fn func() T) T {
	result := make(chan T, 1)
	a.mailbox <- func() {
		result <- fn()
	}
	return <-result
}

// ExecuteWithTimeout 带超时的同步执行
func ExecuteWithTimeout[T any](a *ActorV2, timeout time.Duration, fn func() T) (T, bool) {
	result := make(chan T, 1)
	a.mailbox <- func() {
		result <- fn()
	}

	select {
	case r := <-result:
		return r, true
	case <-time.After(timeout):
		var zero T
		return zero, false
	}
}

// ExecuteAsync 异步执行，返回 channel
func ExecuteAsync[T any](a *ActorV2, fn func() T) <-chan T {
	result := make(chan T, 1)
	a.mailbox <- func() {
		result <- fn()
	}
	return result
}

// Fire 不关心返回值的执行（fire-and-forget）
func Fire(a *ActorV2, fn func()) {
	a.mailbox <- fn
}

// ========== 多返回值支持 ==========

// Result2 两个返回值的包装
type Result2[T1, T2 any] struct {
	V1 T1
	V2 T2
}

// Execute2 执行返回两个值的函数
func Execute2[T1, T2 any](a *ActorV2, fn func() (T1, T2)) (T1, T2) {
	result := make(chan Result2[T1, T2], 1)
	a.mailbox <- func() {
		v1, v2 := fn()
		result <- Result2[T1, T2]{V1: v1, V2: v2}
	}
	r := <-result
	return r.V1, r.V2
}

// Result3 三个返回值的包装
type Result3[T1, T2, T3 any] struct {
	V1 T1
	V2 T2
	V3 T3
}

// Execute3 执行返回三个值的函数
func Execute3[T1, T2, T3 any](a *ActorV2, fn func() (T1, T2, T3)) (T1, T2, T3) {
	result := make(chan Result3[T1, T2, T3], 1)
	a.mailbox <- func() {
		v1, v2, v3 := fn()
		result <- Result3[T1, T2, T3]{V1: v1, V2: v2, V3: v3}
	}
	r := <-result
	return r.V1, r.V2, r.V3
}
