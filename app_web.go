//go:build web

// app_web.go — Web 模式（-tags web）的入口实现。
//
// 由 main.go 中的 useWebUI 开关选中：Web 模式下不启动 Wails 窗口，
// 而是初始化应用上下文后启动本地 HTTP/WS 服务，并用默认浏览器打开 UI。
// 详见 docs/ADR-001-UI浏览器化与AI边车架构决策.md。
package main

import (
	"context"

	"aipanyi/backend/data"
	"aipanyi/webserver"
)

// useWebUI Web 模式标记（桌面模式为 false，见 app_ui_wails.go）。
const useWebUI = true

// startWebServer 初始化应用上下文并启动本地 Web 服务（阻塞常驻）。
//
// 与 Wails OnStartup 的差异：Web 模式下没有前端→后端的 EventsOn 通道，
// 这里只保留必要的初始化（上下文、定时任务、交易日预缓存），
// 托盘初始化待平台适配后补入（见 ADR Phase 1）。
func (a *App) startWebServer() error {
	ctx := context.Background()
	a.ctx = ctx
	data.SetAppCtx(ctx)
	a.InitCronTasks()
	preCacheTradingDays()

	return webserver.Start(a, assets, webserver.Options{
		Port:        0,    // 0 = 从 webserver.DefaultPort 开始探测
		OpenBrowser: true, // 启动后用默认浏览器打开
	})
}
