//go:build !web

package events

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Emit 桌面模式（默认构建）：原样转发给 Wails 运行时，行为与迁移前完全一致。
func Emit(ctx context.Context, eventName string, optionalData ...any) {
	if ctx == nil || eventName == "" {
		return
	}
	runtime.EventsEmit(ctx, eventName, optionalData...)
}
