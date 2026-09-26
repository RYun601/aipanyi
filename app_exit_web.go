//go:build web

package main

import (
	"os"

	"aipanyi/webserver"
	"aipanyi/backend/logger"
)

// exitProcess 供 Web 模式托盘"退出"使用：直接结束进程。
// 桌面模式走 runtime.Quit，二者经构建标签区分。
func exitProcess() {
	os.Exit(0)
}

// openBrowserURL 跨平台打开浏览器（Web 模式下由托盘"打开界面"调用）。
func openBrowserURL(url string) {
	if err := webserver.OpenBrowser(url); err != nil {
		logger.SugaredLogger.Warnf("打开浏览器失败: %v", err)
	}
}
