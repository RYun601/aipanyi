// Package webserver 承载 AI盘译 的浏览器 UI。
//
// 定位：替代 Wails 原生窗口的"UI 承载层"。Go 核心（行情/监控/Agent）不变，
// 前端仍为 Vue 单页应用，通过本包在 127.0.0.1 上提供服务。
//
// 职责：
//   - 托管前端静态资源（main 传入的 go:embed assets）；
//   - POST /api/call 以反射方式派发到 App 的绑定方法（承接原 Wails 的 419 个绑定）；
//   - GET /ws 建立 WebSocket，把 backend/events 的事件广播给前端（承接原 EventsEmit）；
//   - 启动时生成随机 token 并注入页面，/api 与 /ws 强制校验；
//   - 自动打开默认浏览器；以固定 instance 端口实现单实例。
//
// 安全边界：默认仅监听 127.0.0.1，不对外暴露；局域网访问为显式可选项（见 ADR-001）。
package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"aipanyi/backend/events"
)

const (
	// DefaultPort 首选端口，被占用时向上探测。
	DefaultPort = 7077
	// MaxPortAttempts 端口探测次数上限。
	MaxPortAttempts = 20
	// InstancePort 单实例锁端口：抢注成功者为主实例；失败者打开浏览器后退出。
	InstancePort = 7078
	// TokenHeader 鉴权头，前端 wailsjs shim 自动携带。
	TokenHeader = "X-Aipanyi-Token"
	// TokenPlaceholder index.html 中的 token 占位符，由 serveStatic 注入替换。
	TokenPlaceholder = "__AIPANYI_TOKEN_VALUE__"
)

// Server 本地 Web 服务实例。
type Server struct {
	app    any
	assets fs.FS
	addr   string
	token  string
	hub    *EventHub
	mux    *http.ServeMux
	logger *log.Logger
	httpd  *http.Server
}

// Options 启动参数。
type Options struct {
	Port        int          // <=0 时从 DefaultPort 开始探测
	OpenBrowser bool         // 是否自动打开默认浏览器
	Logger      *log.Logger  // 为空时输出到 stdout
}

// Start 创建并启动本地 Web 服务，成功后自动打开浏览器并阻塞常驻。
//
// app 传 *main.App（以 any 传入避免对 package main 的依赖）；
// assets 传 main 中 //go:embed frontend/dist 得到的 fs.FS。
func Start(app any, assets fs.FS, opts Options) error {
	s, err := New(app, assets, opts)
	if err != nil {
		return err
	}
	if opts.OpenBrowser {
		s.openBrowser()
	}
	return s.block()
}

// New 创建服务并开始监听（不打开浏览器、不阻塞），便于测试与嵌入。
func New(app any, assets fs.FS, opts Options) (*Server, error) {
	if app == nil {
		return nil, errors.New("webserver: app 不能为空")
	}
	port := opts.Port
	switch {
	case port == 0:
		port = DefaultPort // 0 = 使用首选端口
	case port < 0:
		port = 0 // 负数 = 交由系统分配临时端口（测试/嵌入场景）
	}
	ln, err := listenLocal(port, MaxPortAttempts)
	if err != nil {
		return nil, err
	}

	lg := opts.Logger
	if lg == nil {
		lg = log.New(os.Stdout, "[aipanyi-web] ", log.LstdFlags)
	}

	s := &Server{
		app:    app,
		assets: assets,
		addr:   ln.Addr().String(),
		token:  newToken(),
		hub:    NewEventHub(),
		logger: lg,
	}
	s.mux = s.buildMux()
	s.httpd = &http.Server{Handler: s.mux}

	// 注册为事件订阅者：此后 events.Emit 会把事件推送到所有 WS 连接。

	go func() {
		if err := s.httpd.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Printf("http serve error: %v", err)
		}
	}()

	currentURLMu.Lock()
	currentURL = s.URL()
	currentURLMu.Unlock()
	s.logger.Printf("AI盘译 已启动: %s", s.addr)
	return s, nil
}

// URL 返回本机访问地址。
func (s *Server) URL() string { return "http://" + s.addr }

// Addr 返回监听地址（host:port）。
func (s *Server) Addr() string { return s.addr }

// Token 返回本次启动生成的访问令牌。
func (s *Server) Token() string { return s.token }

// Shutdown 优雅关闭服务。
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpd.Shutdown(ctx)
}

// block 阻塞当前 goroutine，保持进程常驻（托盘退出时直接 os.Exit）。
func (s *Server) block() error {
	select {}
}

// buildMux 组装路由。静态资源与 /api/health 免鉴权，其余全部校验 token。
func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/call", s.withToken(s.handleCall))
	mux.HandleFunc("/ws", s.withTokenWS(s.hub.Handle))
	mux.HandleFunc("/api/upload", s.withToken(s.handleUpload))
	mux.HandleFunc("/api/file/", s.withToken(s.handleFileDownload))
	mux.HandleFunc("/", s.serveStatic)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"time":         time.Now().Format("2006-01-02 15:04:05"),
		"subscribers":  events.SubscriberCount(),
	})
}

// listenLocal 在 127.0.0.1 上探测可用端口。
func listenLocal(port, maxAttempts int) (net.Listener, error) {
	for i := 0; i < maxAttempts; i++ {
		p := port + i
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("webserver: 端口 %d~%d 均被占用", port, port+maxAttempts-1)
}

// newToken 生成 32 字节随机令牌（hex 编码），防止本机其他程序/网页驱动本服务。
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// 极端情况：熵源不可用，退化为时间戳派生值（仍强于固定值）
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// currentURL 记录最近一次成功启动的服务地址，供托盘"打开界面"等场景使用。
var (
	currentURLMu sync.RWMutex
	currentURL   string
)

// CurrentURL 返回当前运行中的 Web 服务地址；未启动时返回空串。
func CurrentURL() string {
	currentURLMu.RLock()
	defer currentURLMu.RUnlock()
	return currentURL
}
