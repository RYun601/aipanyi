// Package appinfo 集中存放应用的分发与更新源信息。
//
// 独立成包的原因：main（自动更新）与 backend/data（游资名录远程源）
// 都需要引用同一份仓库坐标，而 backend/data 无法 import package main。
//
// ⚠️ fork 后必须把 Owner 改为你自己的 GitHub 用户名，否则：
//   - 用户检查更新会拉到原作者的发行版，覆盖你的构建；
//   - 游资名录等远程资源仍从原作者仓库拉取。
package appinfo

const (
	// Owner 更新源 GitHub 组织/用户名。发布前必须修改。
	Owner = "RYun601"

	// Repo 更新源仓库名。
	Repo = "aipanyi"

	// Branch 远程资源默认分支。
	Branch = "main"

	// GHProxy GitHub 加速前缀（国内网络直连慢）；留空则直连。
	GHProxy = "https://gh.927223.xyz/https://github.com/"

	// 发行资产命名（与发布流水线产出保持一致）
	AssetWindowsAMD64 = "aipanyi-windows-amd64.exe"
	AssetWindowsARM64 = "aipanyi-windows-arm64.exe"
	AssetDarwin       = "aipanyi-darwin-universal.zip"
	AssetLinuxAMD64   = "aipanyi-linux-amd64"
)

// RepoPath 返回 "owner/repo"。
func RepoPath() string { return Owner + "/" + Repo }

// ReleasesPage 发行页地址。
func ReleasesPage() string { return "https://github.com/" + RepoPath() + "/releases" }

// LatestReleaseAPI latest release 元数据接口。
func LatestReleaseAPI() string { return "https://api.github.com/repos/" + RepoPath() + "/releases/latest" }

// TagRefAPI 按 tag 查询引用的接口。
func TagRefAPI(tag string) string {
	return "https://api.github.com/repos/" + RepoPath() + "/git/ref/tags/" + tag
}

// AssetURL 按平台返回发行资产下载地址；useProxy 为 true 时走加速前缀。
func AssetURL(tag, asset string, useProxy bool) string {
	direct := "https://github.com/" + RepoPath() + "/releases/download/" + tag + "/" + asset
	if useProxy && GHProxy != "" {
		return GHProxy + direct
	}
	return direct
}

// RawFileURL 返回仓库内某个文件的 raw 地址（用于远程资源同步）。
func RawFileURL(path string) string {
	return "https://raw.githubusercontent.com/" + RepoPath() + "/" + Branch + "/" + path
}
