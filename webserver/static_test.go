package webserver

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaticInjectsRealToken 用真实文件系统资产验证 token 注入闭环：
// index.html 中的占位符必须被替换为本次启动的令牌。
func TestStaticInjectsRealToken(t *testing.T) {
	dir := t.TempDir()
	html := "<!DOCTYPE html><html><body><div id=\"app\"></div>" +
		"<script>window.__AIPANYI_TOKEN__ = \"" + TokenPlaceholder + "\";</script></body></html>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	s, err := New(fakeApp{}, os.DirFS(dir), Options{Port: -1, Logger: discardLogger()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Shutdown() })

	resp, err := http.Get(s.URL() + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)

	if strings.Contains(out, TokenPlaceholder) {
		t.Fatal("占位符未被替换，token 注入失败")
	}
	if !strings.Contains(out, "window.__AIPANYI_TOKEN__ = \""+s.Token()+"\"") {
		t.Fatalf("未注入正确令牌:\n%s", out)
	}

	// 注入的 token 必须能通过 /api/call 鉴权（前端实际使用路径）
	code, _ := doJSON(t, http.MethodPost, s.URL()+"/api/call", s.Token(), `{"method":"GetVersion","args":[]}`)
	if code != http.StatusOK {
		t.Fatalf("注入的 token 无法通过鉴权，got %d", code)
	}
}
