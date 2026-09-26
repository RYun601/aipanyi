package webserver

import (
	"net/http"
	"strings"
)

// withToken 用启动令牌保护 /api/* 处理器。
// 令牌由进程启动时生成并注入 index.html，前端 shim 通过 Header 自动携带，
// 因此本机其他网页/程序无法驱动本服务（即使它们知道端口）。
func (s *Server) withToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) {
			writeError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}
		next(w, r)
	}
}

// withTokenWS 保护 WebSocket 升级请求（浏览器 API 无法自定义 WS 头，故兼容 query 参数）。
func (s *Server) withTokenWS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) && r.URL.Query().Get("token") != s.token {
			writeError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}
		next(w, r)
	}
}

func (s *Server) checkToken(r *http.Request) bool {
	got := r.Header.Get(TokenHeader)
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	return got != "" && subtleEqual(got, s.token)
}

// subtleEqual 常量时间比较，避免时序侧信道。
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// clientIP 取请求来源 IP（仅 127.0.0.1 场景，用于日志）。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
