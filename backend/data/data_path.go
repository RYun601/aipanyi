package data

import (
	"path/filepath"
	"sync"
)

var (
	dataDirMu sync.RWMutex
	dataDir   = "."
)

// SetDataDir 设置本包写外置数据文件的根目录。
// 桌面端不调用，保持当前工作目录；移动端设为 filesDir/aipanyi。
func SetDataDir(dir string) {
	if dir == "" {
		dir = "."
	}
	dataDirMu.Lock()
	dataDir = dir
	dataDirMu.Unlock()
}

func dataPath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	dataDirMu.RLock()
	root := dataDir
	dataDirMu.RUnlock()
	return filepath.Join(root, name)
}
