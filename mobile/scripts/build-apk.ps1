param(
    [string]$Version = "0.1.0",
    [string]$Task = "assembleDebug",
    [switch]$SkipAar
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$android = Join-Path $root "mobile\android"

function Find-Gradle {
    $gradlew = Join-Path $android "gradlew.bat"
    if (Test-Path $gradlew) { return $gradlew }
    $cmd = Get-Command gradle.bat -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    foreach ($p in @(
        "D:\tools\gradle-8.9\bin\gradle.bat",
        "D:\tools\gradle-8.14\bin\gradle.bat",
        "C:\Gradle\gradle-8.9\bin\gradle.bat"
    )) {
        if (Test-Path $p) { return $p }
    }
    throw "找不到 Gradle。请用 Android Studio 打开 mobile/android，或安装 Gradle 8.9 并加入 PATH。"
}

function Find-Jdk17 {
    $candidates = @(
        "D:\tools\jdk17",
        "D:\tools\jdk21",
        $env:JAVA_HOME,
        "C:\Program Files\Java\jdk-17",
        "C:\Program Files\Java\jdk-21",
        "C:\Program Files\Eclipse Adoptium\jdk-17*",
        "C:\Program Files\Eclipse Adoptium\jdk-21*"
    )
    foreach ($c in $candidates) {
        if (-not $c) { continue }
        $dirs = Get-Item $c -ErrorAction SilentlyContinue
        foreach ($d in $dirs) {
            $java = Join-Path $d.FullName "bin\java.exe"
            if (-not (Test-Path $java)) { continue }
            # 读 JDK 的 release 文件，避免 java -version 走 stderr 触发 PowerShell 的 Stop 行为。
            $release = Join-Path $d.FullName "release"
            if (-not (Test-Path $release)) { continue }
            $content = Get-Content $release -Raw -ErrorAction SilentlyContinue
            if ($content -match 'JAVA_VERSION="(\d+)') {
                $major = [int]$Matches[1]
                if ($major -ge 17 -and $major -le 21) { return $d.FullName }
            }
        }
    }
    throw "找不到 JDK 17/21。Gradle 8.9 不支持 Android Studio 自带的 JDK 25；请安装 JDK 17 并设置 JAVA_HOME。"
}

$env:JAVA_HOME = Find-Jdk17
$env:PATH = "$env:JAVA_HOME\bin;$env:PATH"

if (-not $env:ANDROID_HOME -and -not $env:ANDROID_SDK_ROOT) {
    if (Test-Path "D:\tools\android-sdk") {
        $env:ANDROID_HOME = "D:\tools\android-sdk"
        $env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
    }
}
if (-not $env:ANDROID_HOME -and -not $env:ANDROID_SDK_ROOT) {
    throw "未设置 ANDROID_HOME / ANDROID_SDK_ROOT"
}

if (-not $SkipAar) {
    & (Join-Path $PSScriptRoot "build-aar.ps1") -Version $Version
}

$gradle = Find-Gradle
$sdkForLog = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { $env:ANDROID_SDK_ROOT }
Write-Host "==> JAVA_HOME=$env:JAVA_HOME"
Write-Host "==> ANDROID_HOME=$sdkForLog"
Write-Host "==> Gradle=$gradle"

Push-Location $android
try {
    & $gradle $Task --no-daemon
    if ($LASTEXITCODE -ne 0) {
        throw "Gradle 构建失败（exit $LASTEXITCODE）"
    }
} finally {
    Pop-Location
}

$apk = Join-Path $android "app\build\outputs\apk\debug\app-debug.apk"
if (Test-Path $apk) {
    $sizeMB = [math]::Round((Get-Item $apk).Length / 1MB, 1)
    Write-Host "==> APK: $apk ($sizeMB MB)"
} else {
    Write-Host "==> 构建完成，但未找到 debug APK（Task=$Task）"
}
