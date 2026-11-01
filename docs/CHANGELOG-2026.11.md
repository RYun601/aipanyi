# 2026.11 / v1.0.1-pre

Release: <https://github.com/RYun601/aipanyi/releases/tag/v1.0.1-pre>

## ✨ 新增

- **自选左滑删除**：手机端自选列表左滑露出「删除」，确认后删除并同步停用该股票的价格提醒。
- **模型配置一键获取**：移动端和桌面端填写 BaseUrl + API Key 后点「获取模型」即可拉取模型列表；选中模型自动填充最大输出、上下文窗口和名称。
- **高级参数默认折叠**：AI 配置常用区只保留名称、接口、密钥、模型和「深度思考」；其他参数收进「高级参数」。
- **运行时文件归拢**：手机端 SQLite、Go 日志、数据文件统一迁移到 `filesDir/aipanyi/`；首次启动自动迁移，不覆盖已有新文件。
- **按交易窗口自动启停**：后台服务按启用提醒涉及的市场运行；A 股 09:15–15:00、港股 09:00–16:10、美股 04:00–20:00（纽约时间，含盘前盘后），重叠窗口合并；无启用提醒时不常驻。
- **设置页新增精确闹钟入口和「退出应用」**：退出立即停止本次监控，下一个未来窗口仍会自动启动。
- **运行日志**：新增 Kotlin `runtime.log` 与 Go 调度日志，可回溯调度下发、闹钟触发、服务启停、手动退出和失败原因。

## 🐛 修复

- 手机端 **无法连接本地服务**：后台服务重启后 Go core 更换随机端口，WebView 仍请求旧端口；现在 core URL 变化后会保留当前页面路由重新加载。
- 老版本提醒规则 `market` 为空时按 A 股窗口调度：新增迁移补全。
- 跨进程 `SharedPreferences` 陈旧窗口可能误停后台服务：主进程不再读调度状态，只通知 UI 可见性。
- `Mobilecore.stop()` 在主线程调用可能造成 ANR：改为后台线程停止 Go core。
- 美股交易窗口可能因时区初始化丢失 DST：`marketcal` 直接内嵌 `time/tzdata`。
- 桌面端模型获取逻辑与移动端统一，绑定签名不变。

## ⚡ 优化

- 模型获取失败时保留手动输入，不阻塞保存。
- 精确闹钟未授权时自动降级为不精确闹钟，并在设置页提供授权入口。
- 后台调度日志输出下一个窗口时间，便于真机核对。

## ⚠️ 注意事项

- Android 包仍为 **debug 签名、arm64-v8a 侧载包**，安装包内**不含任何 AI key**；请在 App 设置里填写自己的 AI 配置，key 只保存在手机本地 SQLite。
- 首次启动会把旧数据从 `filesDir` 根目录迁移到 `filesDir/aipanyi/`；回滚前请备份该目录。
- 真机已验证：文件迁移、左滑删除、A 股下一窗口调度、手动退出且保留下一个闹钟、精确闹钟授权、core 换端口后页面恢复。
- 真机未验证：真实交易窗口内闹钟触发、港股/美股窗口、真实 provider `/models` 自动填充。

## 下载

| 平台 | 文件 |
| --- | --- |
| Windows x64 | `aipanyi-windows-amd64.exe` |
| Windows ARM64 | `aipanyi-windows-arm64.exe` |
| macOS（Intel + Apple Silicon） | `aipanyi-darwin-universal.zip` |
| Android arm64（debug 侧载） | `aipanyi-android-0.1.0-debug.apk` |
| 校验值 | `SHA256SUMS.txt` |

## SHA256

```text
a02449a2d067f1124fab59d530fc62d3e32923a25e1756c4ba23787a76071b06  aipanyi-darwin-universal.zip
bf9b264f0b0cdf59578fa4938bf9e4f170853bc46ad434a9da62130baab84359  aipanyi-windows-amd64.exe
7f110bcbb1fab7e1800495bedb51a71b974770b0ec8a1600a7685dbfa596448f  aipanyi-windows-arm64.exe
b20ac9fd5d132d0736024dd066c9b22167ed02a6891853308b71decae57a866e  aipanyi-android-0.1.0-debug.apk
```
