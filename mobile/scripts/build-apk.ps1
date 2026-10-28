param(
    [string]$Version = "0.1.0",
    [string]$Task = "assembleDebug"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")

& (Join-Path $PSScriptRoot "build-aar.ps1") -Version $Version

$android = Join-Path $root "mobile\android"
Set-Location $android

$gradlew = Join-Path $android "gradlew.bat"
if (-not (Test-Path $gradlew)) {
    throw "缺少 gradlew.bat。请用 Android Studio 打开 mobile/android，或先执行 gradle wrapper --gradle-version 8.9"
}

& $gradlew $Task
if ($LASTEXITCODE -ne 0) {
    throw "Gradle 构建失败（exit $LASTEXITCODE）"
}

Write-Host "==> APK: mobile/android/app/build/outputs/apk/debug/app-debug.apk"
