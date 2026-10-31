//go:build !mobileassets

package events

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Emit 桌面端（默认构建）：原样转发给 Wails 运行时。
func Emit(ctx context.Context, eventName string, optionalData ...any) {
	if ctx == nil || eventName == "" {
		return
	}
	runtime.EventsEmit(ctx, eventName, optionalData...)
}
