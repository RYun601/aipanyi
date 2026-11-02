package mobilecore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aipanyi/backend/data"
)

// requestRecorder 记录测试服务端收到的 max_tokens。
// httptest handler 在独立 goroutine 中运行，用锁避免 -race 下的数据竞争。
type requestRecorder struct {
	mu        sync.Mutex
	maxTokens []int
}

func (r *requestRecorder) record(v int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxTokens = append(r.maxTokens, v)
}

func (r *requestRecorder) snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int, len(r.maxTokens))
	copy(out, r.maxTokens)
	return out
}

func TestChatOnceUsesResolvedBudgetWhenZero(t *testing.T) {
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if v, ok := body["max_tokens"].(float64); ok {
			rec.record(int(v))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": "OK"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "gpt-5", ApiKey: "test-key"}
	if _, err := chatOnce(cfg, "sys", "user", 0); err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	got := rec.snapshot()
	if len(got) != 1 || got[0] != 32000 {
		t.Fatalf("max_tokens = %v, want [32000] (gpt-5 builtin)", got)
	}
}

func TestChatOnceEmptyContentReportsFinishReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content":           "",
					"reasoning_content": "思考了很久",
				},
				"finish_reason": "length",
			}},
		})
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "deepseek-flash", ApiKey: "test-key"}
	_, err := chatOnce(cfg, "sys", "user", 0)
	if err == nil {
		t.Fatal("chatOnce() error = nil, want actionable empty-content error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "思考") || !strings.Contains(msg, "length") {
		t.Fatalf("error = %q, want mention reasoning and finish_reason", msg)
	}
}

func TestChatStreamUsesResolvedBudgetWhenZero(t *testing.T) {
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if v, ok := body["max_tokens"].(float64); ok {
			rec.record(int(v))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "gpt-5", ApiKey: "test-key"}
	content, err := chatStream(context.Background(), cfg, "sys", "user", 0, nil, nil)
	if err != nil {
		t.Fatalf("chatStream() error = %v", err)
	}
	if content != "ok" {
		t.Fatalf("content = %q, want ok", content)
	}
	got := rec.snapshot()
	if len(got) != 1 || got[0] != 32000 {
		t.Fatalf("max_tokens = %v, want [32000] (gpt-5 builtin)", got)
	}
}

func TestChatOnceRetriesWhenMaxTokensRejected(t *testing.T) {
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if v, ok := body["max_tokens"].(float64); ok {
			rec.record(int(v))
		}
		if len(rec.snapshot()) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "Invalid max_tokens value, the valid range is [1, 393216]"},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": "OK"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "deepseek-flash", ApiKey: "test-key", MaxTokens: 128000, ContextWindow: 1000000}
	content, err := chatOnce(cfg, "sys", "user", 128000)
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if content != "OK" {
		t.Fatalf("content = %q, want OK", content)
	}
	got := rec.snapshot()
	if len(got) != 2 || got[1] != 64000 {
		t.Fatalf("budgets = %v, want two requests with second 64000", got)
	}
}

func TestChatStreamRetriesWhenMaxTokensRejected(t *testing.T) {
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if v, ok := body["max_tokens"].(float64); ok {
			rec.record(int(v))
		}
		if len(rec.snapshot()) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "Invalid max_tokens value, the valid range is [1, 393216]"},
			})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "deepseek-flash", ApiKey: "test-key", MaxTokens: 128000, ContextWindow: 1000000}
	content, err := chatStream(context.Background(), cfg, "sys", "user", 128000, nil, nil)
	if err != nil {
		t.Fatalf("chatStream() error = %v", err)
	}
	if content != "ok" {
		t.Fatalf("content = %q, want ok", content)
	}
	got := rec.snapshot()
	if len(got) != 2 || got[1] != 64000 {
		t.Fatalf("budgets = %v, want two requests with second 64000", got)
	}
}

func TestTestAIConfigUsesCappedBudget(t *testing.T) {
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if v, ok := body["max_tokens"].(float64); ok {
			rec.record(int(v))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": "OK"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	app := &mobileApp{}
	if _, err := app.TestAIConfig(aiConfigDTO{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		ModelName: "gpt-5",
	}); err != nil {
		t.Fatalf("TestAIConfig() error = %v", err)
	}
	got := rec.snapshot()
	if len(got) != 1 || got[0] != 8192 {
		t.Fatalf("max_tokens = %v, want [8192]", got)
	}
}

func TestChatStreamEmptyContentReportsFinishReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"思考了很久\"},\"finish_reason\":null}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := data.AIConfig{BaseUrl: srv.URL, ModelName: "deepseek-flash", ApiKey: "test-key"}
	_, err := chatStream(context.Background(), cfg, "sys", "user", 0, nil, nil)
	if err == nil {
		t.Fatal("chatStream() error = nil, want actionable empty-content error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "思考") || !strings.Contains(msg, "length") {
		t.Fatalf("error = %q, want mention reasoning and finish_reason", msg)
	}
}
