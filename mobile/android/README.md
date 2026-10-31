# Android 壳

这是 AI盘译 的 Android 原生壳：WebView 加载 Go 核心启动的本地服务，前台服务负责保活与通知。

## 本机已验证的工具链（2026-10-31）

| 组件 | 路径/版本 |
| --- | --- |
| JDK 17 | `D:\tools\jdk17`（Microsoft OpenJDK 17.0.13） |
| Android Studio | `D:\Program Files (x86)\android-studio`（仅 IDE，CLI 构建不依赖） |
| Android SDK | `D:\tools\android-sdk` → junction 到 `D:\Program Files (x86)\Android` |
| Platform / Build-Tools | `android-34` / `34.0.0` |
| NDK | `26.1.10909125` |
| Gradle | `8.9`，`D:\tools\gradle-8.9` |
| Go | `1.27.0`，`D:\Program Files (x86)\golang` |
| gomobile | `C:\Users\ranyu\go\bin\gomobile.exe`（2026-10） |

> **NDK/SDK 路径不能含空格**：gomobile 调用 clang 时不会给路径加引号，
> `D:\Program Files (x86)\...` 会被截成 `D:\Program`。本机用 junction 解决：
>
> ```powershell
> New-Item -ItemType Junction -Path D:\tools\android-sdk -Target 'D:\Program Files (x86)\Android'
> New-Item -ItemType Junction -Path D:\tools\android-jbr -Target 'D:\Program Files (x86)\android-studio\jbr'
> ```

## 构建

```powershell
# 1) 前端 + AAR（约 12MB，含 arm64 libgojni.so）
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1 -Version 0.1.0

# 2) APK（脚本会自动找 JDK17 / Gradle8.9 / SDK）
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version 0.1.0

# 只重打 APK、不重打 AAR
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -SkipAar
```

产物：

- `app/libs/mobilecore.aar`（gitignore，不入库）
- `app/build/outputs/apk/debug/app-debug.apk`（约 26.7 MiB，arm64-v8a，debug 签名）

## 已验证的 gomobile API

AAR 生成的是：

```java
public abstract class mobilecore.Mobilecore {
  public static native String start(String dataDir, mobilecore.Notifier notifier) throws Exception;
  public static native void stop();
}

public interface mobilecore.Notifier {
  void notify(String title, String body, String payload);
  void updateMonitorStatus(int active);
}
```

Kotlin 侧 `AppNotifier` / `MainActivity` 已按此实现。

## 常见问题

**1. `gomobile bind` 报 `fatal error` / `morestack on g0`**
- 项目里 pin 了 2019 年的 `golang.org/x/mobile`，与 2026 版 gomobile 不兼容。
- 构建脚本会检查并执行 `go get golang.org/x/mobile@latest`。

**2. `could not import ... (not the start of an archive file ...)`**
- Go 构建缓存损坏（例如 D 盘掉线/构建中断）。
- 执行：`powershell -File mobile/scripts/build-aar.ps1 -CleanCache`

**3. Gradle 报 JDK 版本不支持**
- Gradle 8.9 不支持 Android Studio 自带的 JDK 25；用 JDK 17/21。
- 本机用 `D:\tools\jdk17`，`build-apk.ps1` 会自动探测。

**4. `ANDROID_HOME` 含空格**
- 用上面的 junction，并把 `ANDROID_HOME` 指向无空格路径。

## 首次安装后的设置（Honor / MagicOS）

1. 允许通知（Android 13+ 会弹窗）。
2. 设置 → 应用 → 应用启动管理 → AI盘译 → 手动管理：允许自启动、关联启动、后台活动。
3. 电池 → 更多电池设置 → 关闭对 AI盘译 的省电限制。
4. 最近任务里下拉锁定 AI盘译。

即使全部设置，ROM 仍可能清理后台；价格提醒无法承诺 100% 准时。
