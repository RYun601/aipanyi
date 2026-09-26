package webserver

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"aipanyi/backend/events"
)

// wsFrame 前端事件帧格式，与 wailsjs/runtime shim 约定一致：
//
//	{"event": "agent-message", "data": {...}}
type wsFrame struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// EventHub 实现 events.Subscriber：把事件以 JSON 帧广播给所有 WS 连接。
//
// 设计对齐 Wails 多窗口语义（所有连接收到同一事件流）：
//   - 每连接独立写 goroutine + 有界缓冲，慢消费者直接断开而非阻塞广播方；
//   - ping/pong 心跳保活，读写超时兜底；
//   - 断连时自动 Unsubscribe，无泄漏。
type EventHub struct {
	mu    sync.RWMutex
	conns map[*wsConn]struct{}
}

type wsConn struct {
	c      *websocket.Conn
	send   chan wsFrame
	closed atomic.Bool
}

// NewEventHub 创建事件中枢。
func NewEventHub() *EventHub {
	return &EventHub{conns: make(map[*wsConn]struct{})}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// 仅同源页面可建连（token 已足够强，这里再收紧一层）
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || origin == "http://"+r.Host
	},
}

// Handle 升级 HTTP 为 WebSocket 并注册为事件订阅者。
func (h *EventHub) Handle(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn := &wsConn{c: c, send: make(chan wsFrame, 256)}

	h.mu.Lock()
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
	events.Subscribe(conn)

	go conn.writeLoop()
	go conn.readLoop(func() { h.drop(conn) })
}

func (h *EventHub) drop(conn *wsConn) {
	h.mu.Lock()
	if _, ok := h.conns[conn]; ok {
		delete(h.conns, conn)
		close(conn.send)
	}
	h.mu.Unlock()
	events.Unsubscribe(conn)
	_ = conn.c.Close()
}

// Count 当前连接数。
func (h *EventHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// SendEvent 实现 events.Subscriber。永不阻塞：缓冲满则丢弃该连接的这条事件。
func (c *wsConn) SendEvent(eventName string, data any) {
	if c.closed.Load() {
		return
	}
	select {
	case c.send <- wsFrame{Event: eventName, Data: data}:
	default:
		// 慢消费者：丢弃而非阻塞广播方，与 Wails 的"尽力送达"语义一致
	}
}

func (c *wsConn) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case frame, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.c.WriteJSON(frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *wsConn) readLoop(onClose func()) {
	defer onClose()
	c.c.SetReadLimit(1 << 20)
	_ = c.c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.c.SetPongHandler(func(string) error {
		_ = c.c.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := c.c.ReadMessage(); err != nil {
			c.closed.Store(true)
			return
		}
	}
}

var _ events.Subscriber = (*wsConn)(nil)

