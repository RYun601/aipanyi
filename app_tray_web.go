//go:build web

// app_tray_web.go — Web 模式的系统托盘（跨平台）。
//
// 与桌面模式的差异：桌面托盘"显示窗口"调用 runtime.WindowShow（Wails 专有），
// Web 模式没有原生窗口，改为"打开界面"——用默认浏览器访问本地服务地址。
//
// 已知限制：getlantern/systray 在 macOS 上要求运行在打包后的 .app bundle 中
// 才能稳定显示托盘图标；以裸二进制运行时可能不显示（Windows/Linux 正常）。
package main

import (

	"github.com/getlantern/systray"

	"aipanyi/backend/logger"
	"aipanyi/webserver"
)

// initSystrayForWeb 启动托盘。必须在 goroutine 中调用（systray.Run 会阻塞）。
func initSystrayForWeb() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("托盘初始化失败（不影响主服务）: %v", r)
			}
		}()
		systray.Run(func() {
			systray.SetIcon(icon2)
			systray.SetTitle("AI盘译")
			systray.SetTooltip("AI盘译：AI 赋能股票分析")

			mOpen := systray.AddMenuItem("打开界面", "在默认浏览器中打开 AI盘译")
			mQuit := systray.AddMenuItem("退出程序", "退出应用程序")

			go func() {
				for {
					select {
					case <-mOpen.ClickedCh:
						openAppInBrowser()
					case <-mQuit.ClickedCh:
						systray.Quit()
						exitProcess()
					}
				}
			}()
		}, func() {
			// cleanup
		})
	}()
}

// openAppInBrowser 打开本地服务地址；服务未就绪时给出提示。
func openAppInBrowser() {
	url := webserver.CurrentURL()
	if url == "" {
		logger.SugaredLogger.Warnf("托盘打开界面：Web 服务尚未就绪")
		return
	}
	openBrowserURL(url)
}
