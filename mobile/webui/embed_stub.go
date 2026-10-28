//go:build !mobileassets

package webui

import (
	"io/fs"
	"testing/fstest"
)

// FS 是未构建移动端前端时的占位页。
// 正式 APK 必须用 -tags mobileassets 构建，否则 WebView 只会看到这张提示页。
var FS fs.FS = fstest.MapFS{
	"index.html": &fstest.MapFile{
		Data: []byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<script>window.__AIPANYI_TOKEN__="__AIPANYI_TOKEN_VALUE__"</script>
<title>AI盘译</title></head>
<body style="font-family:sans-serif;padding:24px;line-height:1.7">
<h2>移动端页面尚未构建</h2>
<p>请先执行：</p><pre>cd mobile/frontend &amp;&amp; npm install &amp;&amp; npm run build</pre>
<p>然后使用 <code>-tags "web mobileassets"</code> 重新构建 Go 核心。</p>
</body></html>`),
	},
}
