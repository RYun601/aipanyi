package webserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"os/exec"
	"strings"
	"time"
)

// OpenBrowser 用系统默认浏览器打开指定 URL（跨平台，可复用）。
// 失败时返回错误，由调用方决定是否记日志。
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// 经 rundll32 调用系统默认浏览器，避免 start 在某些环境的引号问题
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("未支持的操作系统: %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openBrowser 服务启动时自动打开本机地址；失败仅记日志，不阻断启动。
func (s *Server) openBrowser() {
	if err := OpenBrowser(s.URL()); err != nil {
		s.logger.Printf("自动打开浏览器失败，请手动访问: %s (%v)", s.URL(), err)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}
	// 限制 64MB，覆盖知识库文档/导入包等场景
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "parse multipart failed: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	dir := filepath.Join("data", "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir failed: "+err.Error())
		return
	}
	// 仅取文件名部分，防目录穿越
	name := filepath.Base(filepath.Clean(header.Filename))
	if name == "." || name == string(os.PathSeparator) || name == "" {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	dst := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), name))
	out, err := os.Create(dst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create file failed: "+err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, "write file failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": dst, "name": name})
}

// handleFileDownload 替代 Wails 的 SaveFileDialog：
// 前端改为 blob 下载即可，本接口仅用于服务端已有文件的回传场景。
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/file/"))
	if name == "" || name == "." {
		writeError(w, http.StatusBadRequest, "missing filename")
		return
	}
	fp := filepath.Join("data", "uploads", name)
	if _, err := os.Stat(fp); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, fp)
}
