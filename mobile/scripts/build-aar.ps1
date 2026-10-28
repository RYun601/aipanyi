param(
    [string]$Version = "0.1.0",
    [string]$Target = "android/arm64"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $root

function Require-Command($name, $hint) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if (-not $cmd) {
        throw "缺少 $name。$hint"
    }
    return $cmd.Source
}

Require-Command "go" "请先安装 Go 1.27+" | Out-Null
Require-Command "npm" "请先安装 Node.js 20+" | Out-Null
$gomobile = Require-Command "gomobile" "执行: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init"

if (-not $env:ANDROID_HOME -and -not $env:ANDROID_SDK_ROOT) {
    throw "未设置 ANDROID_HOME / ANDROID_SDK_ROOT（需要 Android SDK Platform 34 + Build-Tools）"
}
if (-not $env:ANDROID_NDK_HOME) {
    throw "未设置 ANDROID_NDK_HOME（gomobile 需要 NDK，建议 26.x）"
}

Write-Host "==> 构建移动端前端"
Push-Location (Join-Path $root "mobile\frontend")
try {
    npm install --no-audit --no-fund
    npm run build
} finally {
    Pop-Location
}

Write-Host "==> gomobile bind ($Target)"
$outDir = Join-Path $root "mobile\android\app\libs"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

& $gomobile bind `
    -target=$Target `
    -androidapi 26 `
    -tags "web mobileassets" `
    -ldflags "-X aipanyi/mobile/core.buildVersion=$Version" `
    -o (Join-Path $outDir "mobilecore.aar") `
    aipanyi/mobile/core

if ($LASTEXITCODE -ne 0) {
    throw "gomobile bind 失败（exit $LASTEXITCODE）"
}

Write-Host "==> 完成: mobile/android/app/libs/mobilecore.aar"
