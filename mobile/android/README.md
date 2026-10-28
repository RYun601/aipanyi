# Android 壳

这是 AI盘译 的 Android 原生壳：WebView 加载 Go 核心启动的本地服务，前台服务负责保活与通知。

## 前置

- JDK 17
- Android SDK Platform 34 + Build-Tools 34
- NDK 26.x（gomobile 需要）
- Go 1.27+
- Node.js 20+

环境变量：

```powershell
$env:ANDROID_HOME = "C:\Users\<you>\AppData\Local\Android\Sdk"
$env:ANDROID_NDK_HOME = "$env:ANDROID_HOME\ndk\26.1.10909125"
```

安装 gomobile：

```powershell
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

## 构建

```powershell
# 1) 前端 + AAR
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1 -Version 0.1.0

# 2) APK
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version 0.1.0
```

产物：`app/build/outputs/apk/debug/app-debug.apk`。

> 没有 `gradlew.bat` 时，用 Android Studio 打开 `mobile/android`，或先执行
> `gradle wrapper --gradle-version 8.9`。

## gomobile 生成物命名

Go 包名是 `mobilecore`，gomobile 会生成：

- Java 类 `mobilecore.Mobilecore`，静态方法 `start(String, Notifier)` / `stop()`
- Java 接口 `mobilecore.Notifier`，方法 `notify(...)` / `updateMonitorStatus(int)`

如果升级 gomobile 后命名发生变化，按 `build/generated` 里的实际 Java 接口调整
`AppNotifier.kt` 与 `MainActivity.kt`。

## 首次安装后的设置（Honor / MagicOS）

1. 允许通知（Android 13+ 会弹窗）。
2. 设置 → 应用 → 应用启动管理 → AI盘译 → 手动管理：允许自启动、关联启动、后台活动。
3. 电池 → 更多电池设置 → 关闭对 AI盘译 的省电限制。
4. 最近任务里下拉锁定 AI盘译。

即使全部设置，ROM 仍可能清理后台；价格提醒无法承诺 100% 准时。
