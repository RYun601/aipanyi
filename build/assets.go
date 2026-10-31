// Package buildassets 把构建期股票基础表打进二进制，供桌面端与 Android 端共用。
//
// 文件仍放在 build/ 下（Wails/CI 既有路径不变），这里只做 embed 出口，
// 避免移动端再复制一份 7MB 的 JSON。
package buildassets

import _ "embed"

//go:embed stock_basic.json
var StockBasicJSON []byte

//go:embed stock_base_info_hk.json
var StockBaseInfoHKJSON []byte

//go:embed stock_base_info_us.json
var StockBaseInfoUSJSON []byte
