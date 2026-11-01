package data

import (
	"path/filepath"
	"testing"
)

func TestSetDataDir(t *testing.T) {
	old := dataPath("stock_basic.json")
	defer SetDataDir(".")
	SetDataDir(filepath.Join("tmp", "aipanyi"))
	got := dataPath("stock_basic.json")
	want := filepath.Join("tmp", "aipanyi", "stock_basic.json")
	if got != want {
		t.Fatalf("dataPath() = %q, want %q (old=%q)", got, want, old)
	}
}
