//go:build mobileassets

package events

import "context"

// Emit Android 端（-tags mobileassets 构建）：广播给本地 webserver 的
// WebSocket 订阅者。ctx 仅为保持与 Wails 签名一致，移动端不使用。
func Emit(_ context.Context, eventName string, optionalData ...any) {
	var data any
	if len(optionalData) > 0 {
		data = optionalData[0]
	}
	broadcast(eventName, data)
}
