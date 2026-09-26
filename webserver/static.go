package webserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// serveStatic 提供前端静态资源，并在返回 index.html 时注入启动令牌。
//
// 注入方式：将构建产物中的 TokenPlaceholder 替换为真实 token，
// 页面脚本读取 window.__AIPANYI_TOKEN__ 后由 wailsjs shim 附加到每次请求。
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if s.assets == nil {
		http.Error(w, "assets not configured", http.StatusInternalServerError)
		return
	}

	urlPath := path.Clean(r.URL.Path)
	if urlPath == "/" || urlPath == "." {
		urlPath = "/index.html"
	}

	// SPA 兜底：找不到具体文件时回退 index.html，交由前端路由处理
	if _, err := fs.Stat(s.assets, strings.TrimPrefix(urlPath, "/")); err != nil {
		if !strings.HasPrefix(urlPath, "/api") && !strings.HasPrefix(urlPath, "/ws") {
			urlPath = "/index.html"
		}
	}

	if urlPath == "/index.html" {
		s.serveIndexHTML(w)
		return
	}
	http.FileServer(http.FS(s.assets)).ServeHTTP(w, r)
}

// serveIndexHTML 读取 index.html 并注入 token。
func (s *Server) serveIndexHTML(w http.ResponseWriter) {
	data, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		http.Error(w, "index.html not found (frontend not built?)", http.StatusInternalServerError)
		return
	}
	html := strings.ReplaceAll(string(data), TokenPlaceholder, s.token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}
