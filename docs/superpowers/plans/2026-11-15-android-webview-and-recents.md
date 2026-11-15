# Android 页面恢复与最近任务修复计划

**目标**　AI盘译在系统最近任务中保留可点击的卡片。冷启动、从后台返回或点击提醒时，WebView 连接当前可用的本机服务，并打开正确页面。

**范围**　Android 的 Activity、WebView 和 `:monitor` 服务。继续使用 Go 核心提供的 `127.0.0.1` 随机端口、现有 SQLite 数据目录和交易窗口调度。桌面仍使用 Wails。见 [ADR-001](../../ADR-001-UI浏览器化与AI边车架构决策.md) 和 [ADR-002](../../ADR-002-移除桌面网页版模式.md)。

## 排查结论

| 症状 | 已核实的证据 | 仍需验证 |
| --- | --- | --- |
| 切到系统多任务后卡片消失 | `MainActivity` 的 `android:excludeFromRecents="true"` 会让系统隐藏任务。用 `aapt dump xmltree` 核实两份本地 0.1.1、0.1.2 APK 都含此配置。提交 `b20f246` 记录了其用途：规避荣耀设备上划掉任务导致后台监控进程被终止。 | 恢复卡片后，在荣耀设备验证普通应用切换与主动划掉卡片分别怎样影响监控。 |
| 自选和个股详情报 `ERR_CONNECTION_REFUSED` | 两页都在 `127.0.0.1:37735`；`#` 后的页面路由共用同一个本机 HTTP 服务。交易窗口外界面隐藏时，服务可以正常停机，重启后通常会换端口。已有真机历史日志记录了 `40511 → 40361` 并成功恢复页面。0.1.2 包含 `dcf69ed` 的恢复链路：按请求 ID 接收 `CORE_READY`、探测 `/api/health`、换端口并保留路由，失败时可点击重试。第一张图的「页面加载失败」文案命中本地 0.1.1 APK，0.1.2 APK 使用新的连接提示。 | 截图未显示安装版本，历史日志未出现 `37735`。本次 ADB 设备列表为空，尚无法判断故障发生在旧版、0.1.2 的恢复边界、服务启动失败，还是系统终止了进程。 |

源码入口见 [Manifest](../../../mobile/android/app/src/main/AndroidManifest.xml#L26)、[Activity 导航与恢复](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L112)、[服务启停](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MonitorService.kt#L72) 和 [Go 核心随机端口](../../../mobile/core/core.go#L95)。系统行为见 [Android 最近任务文档](https://developer.android.com/guide/components/activities/recents.html) 和 [Activity 生命周期文档](https://developer.android.com/guide/components/activities/activity-lifecycle)。

## 1. 固定故障版本和复现步骤

- [ ] 在故障手机记录机型、系统版本、Android System WebView 版本、App versionName/versionCode、复现时间和入口。若机器仍装故障包，先取日志和任务状态，再做同签名覆盖升级对照。用测试数据验证时覆盖无提醒、窗口外有提醒、窗口内有提醒三种状态。
- [ ] 分别从桌面图标打开自选和个股详情，执行 Home、系统多任务切换、锁屏解锁以及点提醒通知回来。先测快速切回，再测服务已停止后切回。另外验证在换端口后按返回键，以及页面加载中切后台再回来。
- [ ] 使用下列命令在复现当次关联任务、进程退出、服务起停、地址变化与 WebView 主文档导航。记录报错端口、`CORE_READY` 端口、是否确实换端口、健康探测结果和错误时间。调试包可用 `run-as` 读取私有日志。

```powershell
$androidAdb = 'D:/tools/android-sdk/platform-tools/adb.exe'
& $androidAdb devices -l
& $androidAdb shell dumpsys package com.aipanyi.mobile | Select-String 'versionName|versionCode'
& $androidAdb shell dumpsys webviewupdate
& $androidAdb shell dumpsys activity recents
& $androidAdb shell dumpsys activity services com.aipanyi.mobile
& $androidAdb shell dumpsys activity exit-info com.aipanyi.mobile
& $androidAdb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/runtime.log
& $androidAdb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/info.log
& $androidAdb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/error.log
& $androidAdb logcat -d -v threadtime -s AiPanyiRuntime AndroidRuntime ActivityTaskManager chromium
```

- [ ] 用 0.1.2 同步骤对照。若旧包复现、0.1.2 恢复成功，记录已修版本并把场景留作 APK 回归。若 0.1.2 同样失败，根据下表选择后续改动。现有日志缺字段时，仅补充 PID、任务 ID、请求 ID、端口、启动阶段、错误码和耗时。

## 2. 恢复系统最近任务卡片

- [ ] 在旧包上用系统手势或三键导航进入最近任务，记录卡片缺失和 APK manifest 值。
- [x] 从 [AndroidManifest.xml](../../../mobile/android/app/src/main/AndroidManifest.xml#L26) 删除 `MainActivity` 的 `excludeFromRecents` 属性。沿用 `singleTask`、`:monitor` 和 `stopWithTask="false"`，设置页明确选择「退出应用」时继续由 `exitApp()` 移除任务。
- [ ] 重打 APK，检查 APK 内 manifest。覆盖安装后从桌面启动，连续执行 10 次「多任务 → 切换到另一应用 → 点击 AI盘译卡片」，确认每次都能找到并返回同一任务。检查从通知进详情时只出现一张卡片。
- [ ] 在故障机和另一台 Android 设备分别测试窗口内普通切换、主动划掉卡片、清空最近任务、设置页退出。窗口内普通切换时提醒仍应有效；窗口外允许服务停机。若荣耀设备在主动划卡片或清空任务时终止 `:monitor`，采集进程退出原因并验证下一个交易窗口的闹钟与重启。普通切换和主动移除各自记录结果。

## 3. 补齐 0.1.2 的页面恢复边界

本地已修复两条通过代码确认的路径。返回键校验上一条历史地址的端口，完整文档替换后清理旧历史。切后台和新连接请求都会作废正在加载的页面，保留最后成功页面及待处理的提醒目标。真机结果见下方记录。

| 触发条件与代码线索 | 修复及测试 |
| --- | --- |
| **换端口后按返回键。** [`onBackPressed`](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L451) 直接对 WebView 执行 `goBack()`；[错误回调](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L134) 主要匹配当前待加载地址或 `lastGoodUrl`。若退回历史中的旧端口，其错误可能被忽略。 | 在真机确认旧端口是否仍在历史。若确认，换端口并完成新文档后清理旧文档历史，并复测换端口后的返回和新端口内的正常返回。恢复以新服务地址拼接目标路由。 |
| **加载中切到后台，再回来或点另一条通知。** [`onStop`](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L388) 取消请求与超时，保留 `loadingUrl`；[加载路径](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L209) 相同地址直接跳过。旧导航回调也可能抢先更新新一轮页面状态。 | 若复现，取消请求时同步作废待完成导航，保留最后成功路由和未处理的通知目标。用当前请求和导航状态接收结果。若确认发生 Activity 重建，借助 `savedInstanceState` 恢复路由与未消费目标，重新握手取端口。补测旧回调晚到、相同地址重新加载、连续通知和 Activity 重建。 |
| **服务停启交错、启动受限或 Go 启动失败。** [`ensureCore`](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MonitorService.kt#L47) 的启动异常当前写入日志；Activity 的 [`onStop`](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L388) 在发送隐藏事件失败时会直接尝试停止服务。 | 若日志命中该链路，在服务端继续串行协调当前停启请求，只有当前可用的地址才回给当前请求；隐藏通知失败时由服务自身依据窗口和 UI 状态判断是否停机。启动失败带请求 ID 回传，使前台可立即进入有限重试或原生错误提示。模拟停机时立即返回、一次启动失败，以及窗口内隐藏事件失败，核对当前页面和提醒行为。 |
| **系统记录 WebView 渲染进程退出。** 当前 [WebViewClient](../../../mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt#L98) 缺少 `onRenderProcessGone`。 | 若退出记录命中，重建受影响的 WebView，重新握手并恢复目标页。测试系统回收与渲染进程崩溃路径，确认任务可返回且重建失败时有可点击重试。见 [WebViewClient 文档](https://developer.android.com/reference/android/webkit/WebViewClient#onRenderProcessGone(android.webkit.WebView,%20android.webkit.RenderProcessGoneDetail))。 |

- [x] 在 [CoreUrlTest](../../../mobile/android/app/src/test/java/com/aipanyi/mobile/CoreUrlTest.kt) 覆盖新增的 URL 判定分支。
- [ ] 用 Android 真机覆盖生命周期、系统任务和 WebView 行为。连接恢复成功时，以自选数据、详情及提醒定位能正常加载作为验收结果。

## 4. 构建与验收

本地调试包已生成于 `tmp/android-recovery-build/aipanyi-android-0.1.2-recovery-debug.apk`，复用了已有的 Go AAR。已核对 ARM64、APK 内 Manifest 和原 0.1.2 安装包签名。Android 13 项单测、`mobileassets` Go 构建及移动核心和 Web 服务测试均通过。

- [ ] 根目录执行下列命令。若仅改 Android 壳，可用 `-SkipAar` 加速调试，最终验收重建 AAR 和 APK。构建脚本的 `-Version` 用于 Go 核心；APK 版本由 `app/build.gradle.kts` 决定。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -SkipAar -Task testDebugUnitTest
go build -tags mobileassets ./...
go test -tags mobileassets ./mobile/core/... ./webserver/...
$androidApkVersion = (Select-String -Path mobile/android/app/build.gradle.kts -Pattern 'versionName = "([^"]+)"').Matches[0].Groups[1].Value
powershell -NoProfile -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version $androidApkVersion
```

- [ ] 同签名覆盖安装后，核对自选、配置、持仓与提醒。用自选和个股详情各完成至少 10 次后台返回，包含服务端口确实更换的场景。每次成功恢复原页面或最新通知目标。对持续失败场景，显示应用内错误与重试入口；服务恢复后点击重试成功。
- [ ] 保留版本、设备信息、多任务卡片截图和匿名化日志片段。提交前按仓库约定运行 `go build ./...`、`go vet ./...`、`go test ./webserver/...` 和前端构建；通过功能分支 PR 合入。

## 5. 2026-11-16 荣耀真机调试

- **设备与安装**　NTH-AN00、Android 14、华为 WebView 14.0.0.370。测试前为 0.1.1（versionCode 2）。核对设备上安装包的证书后覆盖安装修复版 0.1.2（versionCode 3）。原自选和持仓在应用中正常显示。
- **历史故障线索**　手机留存日志记录了 11 月 5 日 00:10:46 启用截图中的 `37735` 端口，00:14:06 因界面隐藏停止服务，00:14:28 改用 `40225`。这与截图保留旧地址的现象吻合。
- **最近任务与断线恢复**　旧版启动后切到最近任务，界面未显示 AI盘译卡片。覆盖安装后卡片可见。自选和赛力斯详情各从卡片返回 10 次，20 次都确认本地服务换端口、恢复原路由并渲染正文，未出现连接拒绝。正常点击详情后，在同一端口按系统返回键回到自选。
- **任务移除和通知入口**　ADB 连续投递两个个股打开 Intent，最后显示第二个个股，系统最近任务只有一个 AI盘译任务。在交易窗口外主动划掉 AI盘译卡片后任务移除，从图标启动成功回到自选。实际价格提醒、交易窗口内划卡片的后台行为、清空所有任务及第二台设备仍待验收。
