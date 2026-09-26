//go:build web

package events

import "context"

// Emit Web 模式（-tags web 构建）：广播给所有 WebSocket 订阅者。
// ctx 仅为保持与 Wails 签名一致，Web 模式下不使用。
func Emit(_ context.Context, eventName string, optionalData ...any) {
	var data any
	if len(optionalData) > 0 {
		data = optionalData[0]
	}
	broadcast(eventName, data)
}
