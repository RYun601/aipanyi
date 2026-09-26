package main

// update_config.go — 自动更新源配置（fork 后必须修改）
//
// 具体坐标与 URL 构造统一定义在 backend/appinfo，这里仅做包内别名转发，
// 避免 main 与 backend/data 各存一份仓库坐标导致不一致。
//
// ⚠️ 发布前必须把 backend/appinfo 中的 Owner 改成你自己的 GitHub 用户名，
// 否则用户检查更新会拉到原作者的发行版，覆盖你的构建。

import "aipanyi/backend/appinfo"

const (
	UpdateRepoOwner = appinfo.Owner
	UpdateRepoName  = appinfo.Repo
	UpdateGHProxy   = appinfo.GHProxy

	assetWindowsAMD64 = appinfo.AssetWindowsAMD64
	assetWindowsARM64 = appinfo.AssetWindowsARM64
	assetDarwin       = appinfo.AssetDarwin
	assetLinuxAMD64   = appinfo.AssetLinuxAMD64
)

func updateReleasesPage() string      { return appinfo.ReleasesPage() }
func updateLatestAPI() string         { return appinfo.LatestReleaseAPI() }
func updateTagRefAPI(tag string) string { return appinfo.TagRefAPI(tag) }

func updateAssetURL(tag, asset string, useProxy bool) string {
	return appinfo.AssetURL(tag, asset, useProxy)
}
