package mobilecore

import (
	"fmt"
	"os"
	"path/filepath"
)

type migrationStats struct {
	moved   int
	skipped int
	errors  []string
}

func prepareDataRoot(dataDir string) (string, migrationStats, error) {
	rootDir := filepath.Join(dataDir, "aipanyi")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return "", migrationStats{}, fmt.Errorf("创建应用数据目录失败: %w", err)
	}
	return rootDir, migrateLegacyData(dataDir, rootDir), nil
}

func migrateLegacyData(dataDir, rootDir string) migrationStats {
	var stats migrationStats
	files := []struct{ src, dst string }{
		{filepath.Join(dataDir, "aipanyi.db"), filepath.Join(rootDir, "aipanyi.db")},
		{filepath.Join(dataDir, "aipanyi.db-wal"), filepath.Join(rootDir, "aipanyi.db-wal")},
		{filepath.Join(dataDir, "aipanyi.db-shm"), filepath.Join(rootDir, "aipanyi.db-shm")},
		{filepath.Join(dataDir, "stock_basic.json"), filepath.Join(rootDir, "stock_basic.json")},
		{filepath.Join(dataDir, "hot_money_seats.json"), filepath.Join(rootDir, "data", "hot_money_seats.json")},
		{filepath.Join(dataDir, "key_departments.json"), filepath.Join(rootDir, "data", "key_departments.json")},
	}
	for _, f := range files {
		moved, skipped, err := moveLegacyFile(f.src, f.dst)
		if moved {
			stats.moved++
		}
		if skipped {
			stats.skipped++
		}
		if err != nil {
			stats.errors = append(stats.errors, err.Error())
		}
	}

	for _, d := range []struct{ src, dst string }{
		{filepath.Join(dataDir, "logs"), filepath.Join(rootDir, "logs")},
		{filepath.Join(dataDir, "data"), filepath.Join(rootDir, "data")},
	} {
		moved, skipped, errs := mergeLegacyDir(d.src, d.dst)
		stats.moved += moved
		stats.skipped += skipped
		stats.errors = append(stats.errors, errs...)
	}
	return stats
}

func moveLegacyFile(src, dst string) (bool, bool, error) {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("检查旧文件 %s 失败: %w", src, err)
	}
	if _, err := os.Stat(dst); err == nil {
		return false, true, nil
	} else if !os.IsNotExist(err) {
		return false, false, fmt.Errorf("检查目标文件 %s 失败: %w", dst, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, false, fmt.Errorf("创建目标目录 %s 失败: %w", filepath.Dir(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return false, false, fmt.Errorf("迁移 %s -> %s 失败: %w", src, dst, err)
	}
	return true, false, nil
}

func mergeLegacyDir(src, dst string) (int, int, []string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, []string{fmt.Sprintf("读取旧目录 %s 失败: %v", src, err)}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, 0, []string{fmt.Sprintf("创建目标目录 %s 失败: %v", dst, err)}
	}

	moved, skipped := 0, 0
	var errs []string
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			m, k, e := mergeLegacyDir(s, d)
			moved += m
			skipped += k
			errs = append(errs, e...)
			_ = os.Remove(s)
			continue
		}
		m, k, err := moveLegacyFile(s, d)
		if m {
			moved++
		}
		if k {
			skipped++
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	_ = os.Remove(src)
	return moved, skipped, errs
}
