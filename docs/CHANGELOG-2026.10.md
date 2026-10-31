# 2026.10 / v1.0.0-pre

## ✨ 新增

- **Android 手机端 0.1.0**：自选、成本/持仓、AI 分析、AI 配置与系统提示词、价格触发提醒；
  本地 SQLite，数据不上云。
- **AI 持仓开关**：分析时可选择是否带入持仓信息；开启后逐只勾选已配置成本/持仓的股票，
  AI 结合组合权重给出当前股票策略；默认关闭。
- 手机端 AI 分析采用轻量流式请求，支持 DeepSeek / OpenAI 兼容接口；AI 请求直达用户配置的服务地址。
- 设置页测试通知、电池白名单 / 启动管理引导；监控服务独立 `:monitor` 进程。

## 🐛 修复

- 手机端 Go 进程时区固定为北京时间，修正 A/HK 交易时段判断和显示时间。
- 解决 Honor/MagicOS 划掉最近任务杀进程：App 从最近任务隐藏，清空任务后监控服务仍在。
- 避免手机网络下 AI 分析卡在桌面 8 路数据源；桌面 `NewChatStream` 增加 20 秒上下文兜底。

## ⚡ 优化

- 价格提醒 30 秒轮询；通知点击直达个股；AI 建议价格提醒需用户确认后写入。
- App 隐藏最近任务，从桌面图标或常驻通知打开；重启后手动打开一次恢复。

## ⚠️ 注意事项

- Android 包为 **debug 签名、arm64-v8a 侧载包**，安装包内**不含任何 AI key**；
  请在 App 设置里填写自己的 AI 配置，key 只保存在手机本地 SQLite。
- 实盘价格触发尚未验证（发版当天为周六休市）；Honor 重启可能不自动拉起；
  强停 / 系统深度清理后需重新打开 App。
- 桌面端 1.0.0 同步构建。

## 下载

| 平台 | 文件 |
| --- | --- |
| Windows x64 | CI 产物 `aipanyi-windows-amd64.exe` |
| Windows ARM64 | CI 产物 `aipanyi-windows-arm64.exe` |
| macOS（Intel + Apple Silicon） | CI 产物 `aipanyi-darwin-universal.zip` |
| Android arm64（侧载） | `aipanyi-android-0.1.0-debug.apk` |

Android SHA256：`0132E5AFAEB47351D64952768C37FCB8E4A7F308569109BCA7FC4D2E3B710666`
