package mobilecore

import (
	"strings"
	"testing"

	"aipanyi/backend/data"
)

func TestEstimatePromptTokens(t *testing.T) {
	if got := estimatePromptTokens(""); got != 0 {
		t.Fatalf("estimatePromptTokens(\"\") = %d, want 0", got)
	}
	if got := estimatePromptTokens(strings.Repeat("中", 130)); got != 100 {
		t.Fatalf("estimatePromptTokens(130 Chinese chars) = %d, want 100", got)
	}
	if got := estimatePromptTokens(strings.Repeat("a", 400)); got != 100 {
		t.Fatalf("estimatePromptTokens(400 ASCII chars) = %d, want 100", got)
	}
}

func TestResolveOutputTokenBudgetExplicitConfigWins(t *testing.T) {
	cfg := data.AIConfig{
		ModelName:     "deepseek-flash",
		MaxTokens:     128000,
		ContextWindow: 1000000,
	}
	if got := resolveOutputTokenBudget(cfg, "短提示词"); got != 128000 {
		t.Fatalf("explicit MaxTokens: got %d, want 128000", got)
	}
}

func TestResolveOutputTokenBudgetOldSmallConfigStillWins(t *testing.T) {
	cfg := data.AIConfig{
		ModelName:     "deepseek-flash",
		MaxTokens:     4096,
		ContextWindow: 1000000,
	}
	if got := resolveOutputTokenBudget(cfg, ""); got != 4096 {
		t.Fatalf("explicit 4096: got %d, want 4096", got)
	}
}

func TestResolveOutputTokenBudgetUsesModelMaxWhenUnset(t *testing.T) {
	cases := []struct {
		name string
		cfg  data.AIConfig
		want int
	}{
		{
			name: "deepseek flash targets 128k",
			cfg:  data.AIConfig{ModelName: "deepseek-flash", ContextWindow: 1000000},
			want: 128000,
		},
		{
			name: "gpt-5 builtin 32k",
			cfg:  data.AIConfig{ModelName: "gpt-5"},
			want: 32000,
		},
		{
			name: "gpt-4o builtin 16k",
			cfg:  data.AIConfig{ModelName: "gpt-4o"},
			want: 16000,
		},
		{
			name: "explicit 384k is kept",
			cfg:  data.AIConfig{ModelName: "deepseek-flash", MaxTokens: 384000, ContextWindow: 1000000},
			want: 384000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveOutputTokenBudget(tc.cfg, ""); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestResolveOutputTokenBudgetClampsToContext(t *testing.T) {
	cfg := data.AIConfig{
		ModelName:     "deepseek-flash",
		ContextWindow: 64000,
	}
	// 64K 上下文 - 2K 预留 = 62K，仍比旧的 8K/4K 大很多。
	if got := resolveOutputTokenBudget(cfg, ""); got != 61952 {
		t.Fatalf("64k context: got %d, want 61952", got)
	}
}

func TestResolveOutputTokenBudgetUnknownModelTargets128k(t *testing.T) {
	cfg := data.AIConfig{ModelName: "some-new-model"}
	got := resolveOutputTokenBudget(cfg, "")
	if got < 120000 || got > 128000 {
		t.Fatalf("unknown model: got %d, want between 120000 and 128000", got)
	}
}

func TestResolveOutputTokenBudgetCapsExplicitOverLimit(t *testing.T) {
	cfg := data.AIConfig{
		ModelName:     "deepseek-flash",
		MaxTokens:     500000,
		ContextWindow: 1000000,
	}
	if got := resolveOutputTokenBudget(cfg, ""); got != maxOutputTokenCeiling {
		t.Fatalf("over ceiling: got %d, want %d", got, maxOutputTokenCeiling)
	}
}

func TestResolveOutputTokenBudgetNeverReturnsZero(t *testing.T) {
	cfg := data.AIConfig{
		ModelName:     "deepseek-flash",
		ContextWindow: 1000,
	}
	if got := resolveOutputTokenBudget(cfg, strings.Repeat("中", 5000)); got != minOutputTokens {
		t.Fatalf("tiny context: got %d, want %d", got, minOutputTokens)
	}
}
