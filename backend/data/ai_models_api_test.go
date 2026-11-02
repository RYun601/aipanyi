package data

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAiModelsAndInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "m1"}, {"id": "m2"}, {"id": "m1"}},
			})
		case "/v1/models/m1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                "m1",
				"context_length":    128000,
				"max_output_tokens": 8192,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got := FetchAiModels(srv.URL+"/v1", "test-key", "")
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("FetchAiModels() = %#v, want [m1 m2]", got)
	}

	info := FetchAiModelInfo(srv.URL+"/v1", "test-key", "m1", "")
	if info == nil {
		t.Fatal("FetchAiModelInfo() = nil")
	}
	if info.ContextWindow != 128000 || info.MaxTokens != 8192 || info.Source != "api" {
		t.Fatalf("FetchAiModelInfo() = %#v", info)
	}
}

func TestFetchAiModelInfoUsesContextWindowField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                "deepseek-flash",
			"context_window":    1048576,
			"max_output_tokens": 393216,
		})
	}))
	defer srv.Close()

	info := FetchAiModelInfo(srv.URL+"/v1", "test-key", "deepseek-flash", "")
	if info == nil {
		t.Fatal("FetchAiModelInfo() = nil")
	}
	if info.ContextWindow != 1048576 {
		t.Fatalf("ContextWindow = %d, want 1048576", info.ContextWindow)
	}
	if info.MaxTokens != 393216 {
		t.Fatalf("MaxTokens = %d, want 393216", info.MaxTokens)
	}
}

func TestFetchAiModelInfoFallsBackToBuiltin(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	info := FetchAiModelInfo(srv.URL+"/v1", "test-key", "deepseek-chat", "")
	if info == nil {
		t.Fatal("FetchAiModelInfo() = nil")
	}
	if info.ContextWindow != 1000000 {
		t.Fatalf("ContextWindow = %d, want 1000000", info.ContextWindow)
	}
	if info.MaxTokens != 384000 {
		t.Fatalf("MaxTokens = %d, want 384000", info.MaxTokens)
	}
	if info.Source != "builtin" {
		t.Fatalf("Source = %q, want builtin", info.Source)
	}
}
