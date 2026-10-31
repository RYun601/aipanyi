//go:build mobileassets

// Package webui 提供移动端 WebView 使用的静态资源。
//
// 正式构建使用 -tags mobileassets：Vite 产物输出到 dist/，这里整体 embed。
// 日常 go test 不带头 tag 时使用 embed_stub.go 的占位页，避免 CI 因未构建前端而失败。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS 以 dist 为根，正好对应 webserver 期望的 index.html 位置。
var FS fs.FS = mustSub(embedded, "dist")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic("mobile webui: dist 不存在，请先构建 mobile/frontend")
	}
	return sub
}
