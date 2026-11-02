//go:build devserver

// 本地调试用：在桌面直接启动移动端 Go 核心与静态页面，用浏览器验证接口/UI。
//
// 用法：
//
//	go run -tags "mobileassets devserver" ./mobile/devserver
//
// 然后打开终端输出的 http://127.0.0.1:<port>/ 。
// 数据默认写到 tmp/mobile-dev-data（已在 .gitignore 中忽略）。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"aipanyi/mobile/core"
)

func main() {
	dir := filepath.Join("tmp", "mobile-dev-data")
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	url, err := mobilecore.Start(dir, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("AI盘译 mobile dev server:", url)
	select {}
}
