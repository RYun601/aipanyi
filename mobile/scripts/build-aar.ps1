param(
    [string]$Version = "0.1.0",
    [string]$Target = "android/arm64",
    [switch]$CleanCache
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

# gomobile/NDK 对带空格的路径处理不好；优先提示用户建一个无空格 junction。
if (-not $env:ANDROID_HOME -and -not $env:ANDROID_SDK_ROOT) {
    if (Test-Path "D:\tools\android-sdk") {
        $env:ANDROID_HOME = "D:\tools\android-sdk"
        $env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
    } else {
        throw "未设置 ANDROID_HOME / ANDROID_SDK_ROOT（需要 Android SDK Platform 34 + Build-Tools）"
    }
}
$sdk = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { $env:ANDROID_SDK_ROOT }
if ($sdk -match '\s') {
    throw "ANDROID_HOME 含空格，gomobile 的 NDK clang 路径会被截断。`n建议：New-Item -ItemType Junction -Path D:\tools\android-sdk -Target '$sdk'，然后把 ANDROID_HOME 设为 D:\tools\android-sdk"
}
if (-not $env:ANDROID_NDK_HOME) {
    $ndkRoot = Join-Path $sdk "ndk"
    $ndk = Get-ChildItem $ndkRoot -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | Select-Object -First 1
    if (-not $ndk) {
        throw "未设置 ANDROID_NDK_HOME，且 $ndkRoot 下没有 NDK（建议安装 ndk;26.1.10909125）"
    }
    $env:ANDROID_NDK_HOME = $ndk.FullName
}
if ($env:ANDROID_NDK_HOME -match '\s') {
    throw "ANDROID_NDK_HOME 含空格，gomobile 的 clang 路径会被截断。请为 SDK/NDK 建无空格 junction 后再构建。"
}

# gomobile 的 javac/jar 需要 JDK；Gradle 用的是 JDK 17/21，gomobile 本身用哪个 JDK 都可。
if (-not $env:JAVA_HOME) {
    foreach ($candidate in @("D:\tools\jdk17", "D:\tools\android-jbr")) {
        if (Test-Path (Join-Path $candidate "bin\java.exe")) {
            $env:JAVA_HOME = $candidate
            break
        }
    }
}
if (-not $env:JAVA_HOME) {
    throw "未设置 JAVA_HOME（gomobile bind 需要 JDK 17+）"
}
$env:PATH = "$env:JAVA_HOME\bin;$env:PATH"
# gomobile 生成的 Java 注释含 UTF-8 中文；JDK 17 的 javac 默认用平台编码（GBK）会报 unmappable character。
$env:JAVA_TOOL_OPTIONS = "-Dfile.encoding=UTF-8"

# 项目里如果 pin 了很旧的 x/mobile（如 2019），新版 gomobile 会生成不兼容的 bind。
$mobileVersion = ""
try { $mobileVersion = (& go list -m golang.org/x/mobile 2>$null) } catch {}
if (-not $mobileVersion -or $mobileVersion -match '2019|2020|2021|2022|2023') {
    Write-Host "==> 升级 golang.org/x/mobile（当前: $mobileVersion）"
    & go get golang.org/x/mobile@latest
    if ($LASTEXITCODE -ne 0) { throw "go get golang.org/x/mobile@latest 失败" }
}

if ($CleanCache) {
    Write-Host "==> go clean -cache（修复 D 盘掉线/中断导致的构建缓存损坏）"
    & go clean -cache
    if ($LASTEXITCODE -ne 0) { throw "go clean -cache 失败" }
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
$aar = Join-Path $outDir "mobilecore.aar"

& $gomobile bind `
    "-target=$Target" `
    -androidapi 26 `
    -tags "mobileassets" `
    -ldflags "-s -w -X aipanyi/mobile/core.buildVersion=$Version" `
    -o "$aar" `
    aipanyi/mobile/core

if ($LASTEXITCODE -ne 0) {
    throw @"
gomobile bind 失败（exit $LASTEXITCODE）。
常见原因：
  1) 构建缓存损坏（尤其 D 盘掉线后）：执行 .\mobile\scripts\build-aar.ps1 -CleanCache
  2) SDK/NDK 路径含空格：为 SDK 建无空格 junction，并设置 ANDROID_HOME/ANDROID_NDK_HOME
  3) go.mod 里 x/mobile 版本过旧：go get golang.org/x/mobile@latest
"@
}

if (-not (Test-Path $aar)) {
    throw "gomobile bind 返回成功但没有生成 $aar"
}
$sizeMB = [math]::Round((Get-Item $aar).Length / 1MB, 1)
Write-Host "==> 完成: $aar ($sizeMB MB)"
