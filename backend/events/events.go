// Package events 提供向前端推送事件的统一入口，兼容桌面端与 Android 端：
//
//   - 桌面端（默认构建）：转发给 Wails runtime.EventsEmit；
//   - Android 端（-tags mobileassets 构建）：广播给本地 webserver 注册的所有 WebSocket 订阅者。
//
// 设计要点：
//   - Emit 签名与 wails runtime.EventsEmit 完全一致，既有调用点可机械替换；
//   - 桌面端无订阅者；Android 端由本地 webserver 注册订阅者；
//   - 所有导出函数对 nil / 空事件名安全，调用方无需防御；
//   - 本包不 import 任何业务包，避免循环依赖（backend/data、backend/agent 均可引用）。
package events

import "sync"

// Subscriber 事件订阅者，由 webserver 的 WebSocket 连接实现。
// 实现必须并发安全，且 SendEvent 不得阻塞（内部需有界缓冲 + 异步写出）。
type Subscriber interface {
	SendEvent(eventName string, data any)
}

var (
	subMu sync.RWMutex
	subs  = map[Subscriber]struct{}{}
)

// Subscribe 注册订阅者。Android 端由本地 webserver 在 WS 建连时调用。
func Subscribe(s Subscriber) {
	if s == nil {
		return
	}
	subMu.Lock()
	subs[s] = struct{}{}
	subMu.Unlock()
}

// Unsubscribe 注销订阅者。WS 断开时调用，必须与 Subscribe 成对。
func Unsubscribe(s Subscriber) {
	if s == nil {
		return
	}
	subMu.Lock()
	delete(subs, s)
	subMu.Unlock()
}

// SubscriberCount 当前订阅者数量，用于健康检查与日志。
func SubscriberCount() int {
	subMu.RLock()
	defer subMu.RUnlock()
	return len(subs)
}

// broadcast 向所有订阅者推送事件。
// 单个订阅者 panic 或写阻塞不影响其他订阅者与主流程。
func broadcast(eventName string, data any) {
	if eventName == "" {
		return
	}
	subMu.RLock()
	list := make([]Subscriber, 0, len(subs))
	for s := range subs {
		list = append(list, s)
	}
	subMu.RUnlock()

	for _, s := range list {
		func() {
			defer func() { _ = recover() }() // 订阅者异常不扩散
			s.SendEvent(eventName, data)
		}()
	}
}
