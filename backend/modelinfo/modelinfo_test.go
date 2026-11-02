package modelinfo

import "testing"

func TestDeepSeekFlashExactModelInfo(t *testing.T) {
	if got := GetBuiltinModelContextWindow("deepseek-flash"); got != 1000000 {
		t.Fatalf("deepseek-flash context = %d, want 1000000", got)
	}
	if got := GetBuiltinModelMaxOutput("deepseek-flash"); got != 384000 {
		t.Fatalf("deepseek-flash max output = %d, want 384000", got)
	}
}
