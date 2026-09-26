//go:build !web

// app_ui_wails.go — 桌面模式（默认构建）下的 UI 启动占位。
//
// 保留 useWebUI / startWebServer 两个符号的目的：让 main.go 可以无差别书写
//
//	if useWebUI { err = app.startWebServer() } else { err = wails.Run(...) }
//
// 两种模式共用同一份 main 逻辑，由构建标签决定实际分支。
package main

import "errors"

// useWebUI 桌面模式标记（Web 模式为 true，见 app_web.go）。
const useWebUI = false

// startWebServer 桌面模式不可达，仅为保持 main.go 分支可编译。
func (a *App) startWebServer() error {
	return errors.New("当前构建未启用 Web 模式（需要 -tags web）")
}
