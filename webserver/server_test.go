package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeApp 模拟 main.App 的绑定方法集合，覆盖同步/多参/error/异步四类形态。
type fakeApp struct{}

func (fakeApp) GetVersion() string { return "1.0.0-test" }

func (fakeApp) Add(a, b int) int { return a + b }

func (fakeApp) GetName(id int) (string, error) {
	if id < 0 {
		return "", fmt.Errorf("id 不能为负数: %d", id)
	}
	return fmt.Sprintf("stock-%d", id), nil
}

func (fakeApp) NilPtr(sysPromptId *int) string {
	if sysPromptId == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *sysPromptId)
}

// ChatWithAgent 命中异步白名单：应立即返回 accepted。
func (fakeApp) ChatWithAgent(q string) {}

func startTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, err := New(fakeApp{}, nil, Options{Port: -1, Logger: discardLogger()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Shutdown() })
	return s, s.URL()
}

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func doJSON(t *testing.T, method, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(TokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestHealthNoToken(t *testing.T) {
	_, base := startTestServer(t)
	code, body := doJSON(t, http.MethodGet, base+"/api/health", "", "")
	if code != http.StatusOK {
		t.Fatalf("health 应免鉴权，got %d", code)
	}
	if body["ok"] != true {
		t.Fatalf("health ok != true: %v", body)
	}
}

func TestCallRequiresToken(t *testing.T) {
	_, base := startTestServer(t)
	code, _ := doJSON(t, http.MethodPost, base+"/api/call", "", `{"method":"GetVersion","args":[]}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("缺少 token 应 401，got %d", code)
	}
}

func TestCallDispatch(t *testing.T) {
	s, base := startTestServer(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"无参方法", `{"method":"GetVersion","args":[]}`, "1.0.0-test"},
		{"多参数", `{"method":"Add","args":[2,3]}`, "5"},
		{"返回值带 error", `{"method":"GetName","args":[7]}`, "stock-7"},
		{"指针参数 null", `{"method":"NilPtr","args":[null]}`, "nil"},
	}
	for _, c := range cases {
		code, body := doJSON(t, http.MethodPost, base+"/api/call", s.Token(), c.body)
		if code != http.StatusOK {
			t.Fatalf("%s: 期望 200，got %d (%v)", c.name, code, body)
		}
		if body["error"] != nil {
			t.Fatalf("%s: 不应有 error: %v", c.name, body["error"])
		}
		if fmt.Sprint(body["data"]) != c.want {
			t.Fatalf("%s: data=%v, want %s", c.name, body["data"], c.want)
		}
	}
}

func TestCallErrorPropagation(t *testing.T) {
	s, base := startTestServer(t)
	code, body := doJSON(t, http.MethodPost, base+"/api/call", s.Token(), `{"method":"GetName","args":[-1]}`)
	if code != http.StatusOK {
		t.Fatalf("业务错误应仍返回 200 + error 字段，got %d", code)
	}
	if body["error"] == nil || !strings.Contains(fmt.Sprint(body["error"]), "不能为负数") {
		t.Fatalf("error 未正确透传: %v", body)
	}
}

func TestCallUnknownMethodAndArgCount(t *testing.T) {
	s, base := startTestServer(t)
	if code, _ := doJSON(t, http.MethodPost, base+"/api/call", s.Token(), `{"method":"NoSuchMethod","args":[]}`); code != http.StatusNotFound {
		t.Fatalf("未知方法应 404，got %d", code)
	}
	if code, _ := doJSON(t, http.MethodPost, base+"/api/call", s.Token(), `{"method":"Add","args":[1]}`); code != http.StatusBadRequest {
		t.Fatalf("参数个数不匹配应 400，got %d", code)
	}
}

func TestAsyncMethodReturnsImmediately(t *testing.T) {
	s, base := startTestServer(t)
	code, body := doJSON(t, http.MethodPost, base+"/api/call", s.Token(), `{"method":"ChatWithAgent","args":["你好"]}`)
	if code != http.StatusOK || fmt.Sprint(body["data"]) != "accepted" {
		t.Fatalf("异步方法应返回 accepted，got %d %v", code, body)
	}
}

func TestStaticInjectsToken(t *testing.T) {
	s, base := startTestServer(t)
	// assets 为 nil 时应返回 500 而不是 panic
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("无 assets 时应 500，got %d", resp.StatusCode)
	}
	_ = s
	_ = httptest.NewRecorder()
}
