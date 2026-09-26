//go:build web

package webserver

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"aipanyi/backend/events"
)

// TestWebSocketEventBridge 验证：WS 建连后 events.Emit 的事件能到达前端。
// 这是"承接原 EventsEmit"链路的最小闭环验证。
func TestWebSocketEventBridge(t *testing.T) {
	s, err := New(fakeApp{}, nil, Options{Port: -1, Logger: discardLogger()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Shutdown() })

	u := url.URL{Scheme: "ws", Host: s.Addr(), Path: "/ws", RawQuery: "token=" + s.Token()}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// 等待订阅注册完成
	deadline := time.Now().Add(2 * time.Second)
	for events.SubscriberCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if events.SubscriberCount() == 0 {
		t.Fatal("WS 建连后未注册为事件订阅者")
	}

	// 经事件桥推送（Web 模式下等价于原 runtime.EventsEmit）
	events.Emit(context.Background(), "test-event", map[string]any{"msg": "hello"})

	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var frame wsFrame
	if err := c.ReadJSON(&frame); err != nil {
		t.Fatalf("读取事件帧失败: %v", err)
	}
	if frame.Event != "test-event" {
		t.Fatalf("事件名不符: %q", frame.Event)
	}
	m, ok := frame.Data.(map[string]any)
	if !ok || m["msg"] != "hello" {
		t.Fatalf("事件载荷不符: %v", frame.Data)
	}
}

// TestWebSocketRejectsBadToken 验证无 token 的 WS 连接被拒绝。
func TestWebSocketRejectsBadToken(t *testing.T) {
	s, err := New(fakeApp{}, nil, Options{Port: -1, Logger: discardLogger()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Shutdown() })

	u := url.URL{Scheme: "ws", Host: s.Addr(), Path: "/ws", RawQuery: "token=wrong"}
	_, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err == nil {
		t.Fatal("错误 token 不应允许建连")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("应返回 401，got %v", resp)
	}
	_ = strings.TrimSpace("")
}
