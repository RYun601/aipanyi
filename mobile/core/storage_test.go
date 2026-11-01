package mobilecore

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestMigrateLegacyDataMovesKnownFiles(t *testing.T) {
	filesDir := t.TempDir()
	rootDir := filepath.Join(filesDir, "aipanyi")
	if err := os.MkdirAll(filepath.Join(rootDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(filesDir, "aipanyi.db"), "db")
	writeTestFile(t, filepath.Join(filesDir, "logs", "info.log"), "old log")
	writeTestFile(t, filepath.Join(filesDir, "data", "hot_money_seats.json"), "seats")
	writeTestFile(t, filepath.Join(filesDir, "stock_basic.json"), "basic")
	writeTestFile(t, filepath.Join(rootDir, "logs", "runtime.log"), "runtime")

	stats := migrateLegacyData(filesDir, rootDir)
	if len(stats.errors) != 0 {
		t.Fatalf("migration errors: %v", stats.errors)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "aipanyi.db")); got != "db" {
		t.Fatalf("db = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "logs", "info.log")); got != "old log" {
		t.Fatalf("info.log = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "data", "hot_money_seats.json")); got != "seats" {
		t.Fatalf("seats = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "stock_basic.json")); got != "basic" {
		t.Fatalf("stock_basic = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "logs", "runtime.log")); got != "runtime" {
		t.Fatalf("runtime.log = %q", got)
	}
	if _, err := os.Stat(filepath.Join(filesDir, "aipanyi.db")); !os.IsNotExist(err) {
		t.Fatalf("old db still exists: %v", err)
	}
}

func TestMigrateLegacyDataDoesNotOverwrite(t *testing.T) {
	filesDir := t.TempDir()
	rootDir := filepath.Join(filesDir, "aipanyi")
	writeTestFile(t, filepath.Join(filesDir, "aipanyi.db"), "old")
	writeTestFile(t, filepath.Join(rootDir, "aipanyi.db"), "new")

	stats := migrateLegacyData(filesDir, rootDir)
	if stats.skipped == 0 {
		t.Fatal("expected skipped file")
	}
	if got := readTestFile(t, filepath.Join(rootDir, "aipanyi.db")); got != "new" {
		t.Fatalf("target overwritten: %q", got)
	}
	if got := readTestFile(t, filepath.Join(filesDir, "aipanyi.db")); got != "old" {
		t.Fatalf("source changed: %q", got)
	}
}
