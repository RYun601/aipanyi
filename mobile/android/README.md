# Android 壳

这是 AI盘译 的 Android 原生壳：`MonitorService` 在独立 `:monitor` 进程里持有 Go 核心并启动本地 HTTP 服务；Activity/WebView 通过 `CORE_READY` 广播拿到本地 URL 后加载页面，前台服务负责保活与通知。

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

Kotlin 侧 `AppNotifier` / `MonitorService` 已按此实现；`MainActivity` 不再直接启动 Go 核心。

## 真机验收（2026-10-31，Honor 50 / MagicOS 8.0.0.221）

- **已通过**：安装/启动、自选与成本持仓、价格提醒 CRUD、AI 建议确认写入、
  **持仓开关（默认关闭、开启后逐只选择、组合权重）**、DeepSeek 真实分析（上一次，约 10.5 秒，
  带成本/持仓/30 日 K 线）、测试通知、前台服务、Home/锁屏存活、电池白名单、强行停止后重启恢复。
- **持仓开关**：详情页「带入持仓信息」默认关闭；开启后列出已配置成本/持仓的股票，默认全选、
  逐只可关；只把勾选股票传给 AI，组合表/权重按勾选持仓计算；当前股票未勾选时不给当前成本；
  分析中开关禁用；重新进入详情页恢复关闭；全不选会阻止启动。
- **本轮真实模型**：临时 DeepSeek key 已失效（401），真实模型分析未能复测；持仓上下文用假 AI
  在真机上验证了关闭/全选/部分/当前不选/全不选五种请求。
- **进程模型**：Go 核心在 `:monitor` 前台服务进程；Activity 只负责 WebView 与原生桥，
  通过 `CORE_READY` 广播拿本地 URL。Go 进程时区固定为 `Asia/Shanghai`，避免交易时段判断偏 8 小时。
- **Honor 限制**：早期版本划掉任务会杀掉整个 App（包括 `:monitor` 进程）；已用
  `excludeFromRecents` 把 App 从最近任务隐藏，实测“清理全部”后服务仍在，从桌面/通知打开即可。
  重启后 Honor 可能不自动拉起，需手动打开一次 App。数据（自选/提醒/分析）均保留。
- **实盘触发**：验收日为周六，未测真实价格穿越；状态机单测和通知链路已通过，下一个交易日补测。
- 详细记录见 [`docs/plan/2026-10-29-android-price-alert-mvp.md`](../../docs/plan/2026-10-29-android-price-alert-mvp.md)。

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
4. App 已从最近任务隐藏，避免被划掉；从桌面图标或常驻通知打开。
5. 重启手机后如果通知没有自动恢复，打开一次 AI盘译；自选、提醒、分析结果都会保留。
6. 设置页「测试通知」可检查通知权限和渠道是否正常。

即使全部设置，ROM 仍可能清理后台；价格提醒无法承诺 100% 准时。
