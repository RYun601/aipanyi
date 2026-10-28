# Android 价格提醒 App（MVP）设计与实施计划

- 日期：2026-10-29
- 分支：`feat/android-price-alert`
- 状态：执行中
- 目标设备：Honor 50（Android 14 / MagicOS 8，arm64）

## 1. 目标

在现有 AI盘译（Go + Vue）基础上，交付一个可独立安装的 Android APK，只保留手机端最需要的功能：

1. 自选股：关注/取消关注、成本价、持仓数量、现价/涨跌/盈亏。
2. 个股 AI 分析：流式输出，自动带入该股票的成本价与持仓数量。
3. AI 配置：多套 OpenAI 兼容配置，可测试连接。
4. 系统提示词：模板增删改，选择默认模板。
5. 价格提醒：手动添加 + AI 分析后建议、用户确认；后台轮询；达到价格后发本地通知。

## 2. 非目标（MVP 不做）

- 指定时间提醒（如 14:50 提醒）。
- 云同步、账号、多端同步。
- 自动交易、券商下单。
- 分组/概念/研报/龙虎榜/基金/回测/K 线图。
- 桌面端数据自动同步（第二阶段可做导出/导入 JSON）。

## 3. 总体架构

```
Android App
├─ Kotlin 壳（mobile/android）
│   ├─ MainActivity / WebView
│   ├─ MonitorService（前台服务，保活 + 通知）
│   ├─ NotificationHelper（通知渠道）
│   ├─ BootReceiver（开机自启）
│   └─ NativeBridge（通知权限/电池白名单/Honor 自启动设置）
│
├─ Go 核心（mobile/core，gomobile AAR）
│   ├─ mobilecore.Start/Stop（幂等）
│   ├─ MobileApp 门面（约 25 个方法，经 /api/call 反射派发）
│   ├─ SQLite（自选/成本/AI 配置/提示词/提醒）
│   ├─ 提醒引擎（30 秒轮询 + 穿越去重 + 落库）
│   └─ AI 分析 + 策略提取
│
├─ WebView UI（mobile/frontend，Vue 3 + Vant）
│   └─ 自选 / 个股 / 提醒 / 设置
│
└─ 本地 HTTP（复用 webserver）
    ├─ 127.0.0.1 随机端口 + token
    ├─ POST /api/call
    └─ GET /ws（事件：分析流、提醒触发）
```

复用现有代码：

- `backend/data`：行情、K 线、AI 流、AI 配置、提示词、自选股。
- `backend/db`：SQLite/GORM。
- `backend/webserver`：静态资源、`/api/call`、WebSocket、token。
- `backend/events`：事件广播（`-tags web`）。
- `frontend/wailsjs/bridge.js` 的协议思路（移动端单独实现轻量客户端）。

不编译桌面 `main.App`（依赖 Wails/托盘/更新器），移动端使用独立 `MobileApp` 门面。

## 4. 目录结构

```
mobile/
├─ core/                 Go 包（package mobilecore）
│   ├─ core.go           Start/Stop、初始化、幂等
│   ├─ app.go            MobileApp 门面（/api/call）
│   ├─ models.go         DTO + 数据表
│   ├─ watchlist.go      自选 + 实时价 + 盈亏
│   ├─ analyze.go        AI 分析流 + 自动保存
│   ├─ alert_engine.go   提醒引擎
│   ├─ alert_store.go    提醒 CRUD + 历史
│   ├─ alert_extract.go  AI 策略提取
│   ├─ chat_once.go      一次性 JSON 提取调用
│   ├─ config.go         手机端设置
│   ├─ stockseed.go      内置股票基础表导入
│   └─ notifier.go       Notifier 接口
├─ webui/                嵌入前端产物（build tag: mobileassets）
├─ frontend/             Vue 3 + Vite + Vant
├─ android/              Gradle Android 工程
└─ scripts/              构建脚本
```

## 5. 数据模型

复用：

- `followed_stock`：`stock_code / name / cost_price / volume / alarm_price / ...`
- `ai_config`：AI 服务配置
- `prompt_templates`：系统提示词模板
- `ai_response_result`：AI 分析历史
- `stock_basic`：A 股基础表（首次启动从内置 JSON 导入）

新增：

```go
// mobile_price_alert
type PriceAlert struct {
    ID              uint
    StockCode       string  // 归一化代码，如 sh600519
    StockName       string
    Market          string  // A / HK / US
    Direction       string  // above / below
    Price           float64
    Note            string
    Source          string  // manual / ai
    Enabled         bool
    LastState       string  // above / below / unknown
    LastTriggeredAt *time.Time
    TriggerCount    int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// mobile_price_alert_event
type PriceAlertEvent struct {
    ID          uint
    AlertID     uint
    StockCode   string
    StockName   string
    Price       float64
    Threshold   float64
    Direction   string
    TriggeredAt time.Time
}
```

手机端设置（单行表 `mobile_settings`）：

```go
type MobileSettings struct {
    ID                uint
    DefaultAIConfigID int64
    DefaultPromptID   int64
    AutoExtractAlerts bool
    PollIntervalSec   int  // 默认 30，范围 10-300
    EnableTools       bool
    Thinking          bool
    PushEnabled       bool
}
```

## 6. 提醒引擎

### 6.1 状态机

每条规则维护 `last_state`：

- `above` 规则（价格 ≥ 阈值触发）：
  - 上次状态不是 `above`，当前价 ≥ 阈值 → 通知并置 `above`。
  - 当前价 < 阈值 → 置 `below`，重新武装。
- `below` 规则（价格 ≤ 阈值触发）：
  - 上次状态不是 `below`，当前价 ≤ 阈值 → 通知并置 `below`。
  - 当前价 > 阈值 → 置 `above`，重新武装。
- `unknown`：首次拿到行情时只初始化状态，不通知（避免创建时误报）；如果创建时能拿到行情且已满足条件，立即通知一次并置位。

### 6.2 运行

- 仅在该股票所属市场的交易时段运行（A/HK/US）。
- 每 `PollIntervalSec`（默认 30 秒）批量拉取启用规则涉及的股票行情；一次请求多只。
- 行情失败、停牌、价格 ≤ 0：跳过，不修改状态，不通知。
- 通知后写 `mobile_price_alert_event`、`trigger_count++`、`last_triggered_at`。
- 单条规则同一方向只通知一次，价格回到另一侧后重新武装。
- 最多 50 条启用规则；超过时创建失败并提示。

### 6.3 通知

- Android 通知渠道 `price_alert`（高优先级、声音/震动）。
- 文案：`【价格提醒】贵州茅台` / `现价 1448.00，已跌破 1450.00`
- 点击打开个股详情（`#/stock/sh600519`）。
- 常驻通知：`监控中 · N 条价格提醒`，点击打开 App。

## 7. AI 分析

- 复用 `data.NewDeepSeekOpenAi(...).NewChatStream(...)`，流式事件经 WS 推送。
- 提示词模板：`MobileSettings.DefaultPromptID`；0 表示使用桌面全局提示词。
- 成本/持仓注入：
  - 现有 `{{costPrice}}` 保留。
  - 新增 `{{costVolume}}`（持仓数量，股）。
  - 未设置时注入“未设置”，避免模板出现裸占位符。
- 分析完成后服务端自动 `SaveAIResponseResult`，即使 WebView 被切走也不丢结果。
- 同时只允许一个分析任务；支持取消。

## 8. AI 策略提取（建议规则）

分析完成后（可配置开关）用同一个 AI 配置做一次低温度 JSON 调用：

```json
{"alerts":[{"direction":"below","price":1450,"note":"跌破支撑减仓"}]}
```

规则：

- 只输出 JSON，禁止 Markdown。
- 校验：
  - `direction` ∈ {above, below}
  - `price > 0`
  - 价格在现价 0.5~2 倍之间
  - 股票代码必须等于当前分析股票
- 校验失败的条目丢弃并返回原因。
- 前端弹底部确认面板：勾选、改价、编辑备注，确认后写入。
- 设置里可关闭“分析后自动提取”。
- 手动“提取提醒”按钮复用同一接口。

## 9. 接口清单（MobileApp）

自选：

- `GetWatchlist() ([]WatchItem, error)`
- `SearchStocks(keyword string) []StockBasicDTO`
- `AddStock(code string) (string, error)`
- `RemoveStock(code string) (string, error)`
- `SetPosition(code string, costPrice float64, volume int64) (string, error)`
- `GetStockDetail(code string) (*StockDetailDTO, error)`

AI 分析：

- `StartAnalysis(code string) (string, error)`
- `CancelAnalysis()`
- `GetAnalysis(code string) *AnalysisDTO`
- `ExtractAlerts(code string) ([]AlertSuggestionDTO, error)`

AI 配置：

- `ListAIConfigs() []AIConfigDTO`
- `SaveAIConfig(cfg AIConfigDTO) (int64, error)`
- `DeleteAIConfig(id int64) (string, error)`
- `TestAIConfig(cfg AIConfigDTO) (string, error)`

提示词：

- `ListPrompts() []PromptDTO`
- `SavePrompt(p PromptDTO) (int64, error)`
- `DeletePrompt(id int64) (string, error)`

提醒：

- `ListAlerts(code string) []AlertDTO`
- `SaveAlert(a AlertDTO) (int64, error)`
- `DeleteAlert(id int64) (string, error)`
- `SetAlertEnabled(id int64, enabled bool) (string, error)`
- `ListAlertEvents(limit int) []AlertEventDTO`

设置：

- `GetMobileSettings() MobileSettingsDTO`
- `SaveMobileSettings(s MobileSettingsDTO) (string, error)`
- `GetVersionInfo() VersionDTO`

事件：

- `mobileAnalysis`：`{runId, code, delta, reasoning, done, error, saved}`
- `mobileAlertSuggestions`：`{code, suggestions}`
- `mobileAlertTriggered`：`{alertId, code, name, price, threshold, direction, time}`
- `mobileWatchlistChanged`

## 10. 移动端页面

1. **自选**：列表（名称/代码/现价/涨跌/成本/盈亏）、下拉刷新、添加股票。
2. **个股**：行情卡、成本/持仓编辑、AI 分析（流式 Markdown）、提取提醒、该股提醒列表。
3. **提醒**：全部规则、开关、编辑/删除、最近触发记录。
4. **设置**：AI 配置 CRUD + 测试；提示词 CRUD + 默认选择；后台保活/通知权限引导；关于。

UI 使用 Vant 4 + 原生 CSS 变量；hash 路由；API 客户端读取 `window.__AIPANYI_TOKEN__`。

## 11. Android 壳

### 11.1 权限

- `INTERNET`、`ACCESS_NETWORK_STATE`
- `POST_NOTIFICATIONS`（Android 13+ 运行时）
- `FOREGROUND_SERVICE`、`FOREGROUND_SERVICE_DATA_SYNC`
- `RECEIVE_BOOT_COMPLETED`
- `REQUEST_IGNORE_BATTERY_OPTIMIZATIONS`
- `WAKE_LOCK`（轮询期间短时持有，可选）

### 11.2 后台

- `MonitorService`：`startForeground` 先行，再后台线程 `Mobilecore.start`。
- `START_STICKY`；`onTaskRemoved` 不停止，必要时 1 秒后重启服务。
- 开机广播：若上次监控规则数 > 0，启动服务（Android 14 允许；后续 Android 15 需适配）。
- 设置页提供：
  - 通知权限状态 + 申请按钮
  - 电池优化白名单 + 跳转
  - Honor/MagicOS 应用启动管理跳转（失败回退应用详情）
  - 最近任务加锁说明

### 11.3 WebView

- 加载 `http://127.0.0.1:<随机端口>/`，token 由服务端注入 `index.html`。
- 允许 cleartext（localhost 与本机 HTTP 行情源）。
- 外部链接交给系统浏览器。
- 返回键优先 `webView.canGoBack()`。
- `NativeBridge` 暴露：通知权限、电池优化、自启动设置、版本号、监控状态。

## 12. 对现有代码的改动

1. `backend/data/openai_stream.go`
   - 注入 `{{costVolume}}`。
   - `costPrice`/`costVolume` 未设置时注入“未设置”。
   - 修复 8 个 goroutine 并发 append `msg` 的数据竞争（mutex）。
2. `backend/marketcal`（新增）
   - 从 `app.go` 抽出 `isTradingDay / isTradingTime / IsHKTradingTime / IsUSTradingTime` 及节假日检查。
   - 桌面保留薄包装，行为不变。
3. `backend/data`
   - 导出 `NormalizeStockCode` 包装（移动端复用代码归一化）。
   - 导出 `OpenAIChatEndpoint` 包装（移动端一次性 JSON 调用复用 URL 归一化）。
4. `backend/db`
   - 新增 `InitWithOptions`（可配置 cache/mmap/连接数），桌面 `Init` 行为不变。
5. `backend/logger`
   - 支持 `SetLogDir`，移动端日志写到应用私有目录。
6. `build/assets.go`（新增）
   - `//go:embed` 三个股票基础表 JSON，供移动端首次启动导入；桌面 `main.go` 不改。
7. `mobile/*`（新增）
   - 不修改桌面构建命令；新增独立构建脚本。

## 13. 构建与验证

### 13.1 前端

```bash
cd mobile/frontend
npm install
npm run build          # 输出到 mobile/webui/dist
```

### 13.2 Go 核心

```bash
go build -tags "web mobileassets" ./mobile/core
go build -tags "web mobileassets" ./mobile/...
go test ./mobile/alerts   # 纯逻辑单测；不依赖网络/DB
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -tags "web mobileassets" ./mobile/...
```

> `mobile/core` 的测试二进制会链接完整数据栈（chromedp/eino），Windows Defender 可能误判为病毒；
> 状态机与 AI 建议解析已拆到 `mobile/alerts`，可正常执行。

### 13.3 AAR

```bash
gomobile bind -target=android/arm64 -androidapi 26 \
  -tags "web mobileassets" \
  -o mobile/android/app/libs/mobilecore.aar \
  aipanyi/mobile/core
```

> 需要 Android SDK + NDK（`ANDROID_HOME` / `ANDROID_NDK_HOME`）。当前开发机未安装，首次构建前需安装。

### 13.4 APK

```bash
cd mobile/android
./gradlew assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk
```

### 13.5 回归

```bash
go build ./... && go build -tags web ./...
go vet ./...
go test ./backend/data/... ./mobile/core/...
cd frontend && npm run build
```

## 14. 里程碑

### M0 设计落地
- [x] 需求确认：只做价格触发，不做指定时间。
- [x] 规则来源：手动 + AI 建议、人工确认。
- [x] 方案确认：Go 核心 + Android 壳 + WebView。
- [x] 设计文档落盘。

### M1 后端基础
- [x] `backend/marketcal` 抽取 + 桌面包装。
- [x] `costVolume` 注入 + 数据竞争修复。
- [x] `db.InitWithOptions`、`logger.SetLogDir`。
- [x] `build/assets.go` 股票基础表 embed。
- [x] `mobile/core` 初始化、DB、股票导入、设置。

### M2 提醒引擎
- [x] `PriceAlert` / `PriceAlertEvent` 表。
- [x] 状态机 + 30 秒轮询 + 通知接口。
- [x] 去重、重启恢复、交易时段判断。
- [x] 单元测试（above/below/unknown/重复/行情失败/非交易时段）。

### M3 AI 与门面
- [x] 自选 + 实时价 + 盈亏。
- [x] AI 分析流 + 自动保存 + 取消。
- [x] AI 配置/提示词 CRUD。
- [x] 策略提取 + 校验。
- [x] `MobileApp` 全部接口 + 本地 API 冒烟测试。

### M4 移动端 UI
- [x] Vite + Vue 3 + Vant 工程。
- [x] 自选 / 个股 / 提醒 / 设置页面。
- [x] API 客户端 + WS 事件 + token。
- [x] `npm run build` 通过。

### M5 Android 壳
- [x] Gradle 工程 + Manifest + 权限。
- [x] WebView + NativeBridge。
- [x] Foreground Service + 通知 + 开机广播。
- [x] 保活引导页。
- [ ] `assembleDebug` 构建（需本机安装 Android SDK/NDK + gomobile）。

### M6 验收
- [ ] 锁屏/后台/划掉任务触发通知（真机）。
- [ ] 重启恢复；强行停止提示（真机）。
- [x] AI 分析建议规则确认后生效（本地 API/WS 验证）。
- [x] 断网/停牌不误报（纯逻辑测试）。
- [x] 桌面构建/测试无回归。

## 15. 验收标准

1. 手动创建“跌破 1450”规则，锁屏、切后台、从最近任务划掉，价格触发后 30-60 秒内收到通知（已加白名单）。
2. 同一规则不重复刷屏；价格回升后再次跌破会再次提醒。
3. App 进程被系统回收后，前台服务重启能恢复规则状态，不重复轰炸。
4. 重启手机后，打开 App 能恢复监控；用户强行停止后，App 明确提示“需重新打开”。
5. AI 分析完成，自动带入成本价/持仓；结果落库，切走再回来仍在。
6. AI 提取出建议规则，确认后写入；不合法价格被丢弃。
7. 断网、停牌、行情接口失败时不误报。
8. 桌面端 `go build ./...`、`go build -tags web ./...`、`go test ./webserver/...`、`frontend npm run build` 全部通过。

## 16. 风险与待验证

| 风险 | 影响 | 应对 |
| --- | --- | --- |
| gomobile 与 Go 1.27 兼容性 | 无法打 AAR | 先做 hello-world spike；失败则改用 native 可执行库方式，Go 接口不变 |
| APK 体积 40-80MB | 下载/安装偏大 | 只打 arm64；后续裁剪未用包 |
| MagicOS 杀后台 | 漏提醒 | 前台服务 + 白名单 + 自启动 + 最近任务锁；文档明确不保证 100% |
| 30 秒轮询漏掉瞬时刺破 | 漏报 | 产品定位为分钟级提醒；可调至 10 秒但更费电 |
| AI 提取价格幻觉 | 错误规则 | 0.5~2 倍现价校验 + 用户确认 |
| 行情源限流 | 数据缺失 | 批量请求、失败跳过不误报；后续可换源 |
| 手机端独立数据 | 需重新配置 | MVP 手动填；第二阶段导出/导入 JSON |

## 17. 实施记录

- 2026-10-29：设计落盘，开始 M1。
- 2026-10-29：M1-M4 完成；M5 代码完成，等待 Android SDK/NDK 构建。

### 已验证（本机）

```powershell
go build .                                  # 桌面通过
go build -tags web .                        # Web 模式通过
go build -tags web ./mobile/...             # 移动核心（占位页）通过
go build -tags "web mobileassets" ./mobile/...  # 移动核心 + 真实前端通过
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -tags web ./mobile/...
go vet -tags "web mobileassets" ./mobile/...
go test ./mobile/alerts                     # 状态机/解析单测通过
go test ./backend/db                        # DSN 回归通过
go test ./webserver/...                     # Web 承载层回归通过
cd frontend && npm run build                # 桌面端前端通过
cd mobile/frontend && npm run build         # 移动端前端通过
```

本地 devserver + 假 AI 服务冒烟通过：

- 搜索 `600519` → 关注 → 设置成本/持仓 → 创建提醒 → 查询详情，盈亏计算正确。
- `TestAIConfig` 返回“连接成功”。
- `StartAnalysis` 流式事件经 WS 推送，完成后落库。
- `ExtractAlerts` 从分析中提取出 `below 1450` 建议并通过校验。

### 待完成

- 安装 Android SDK 34 / NDK 26 / gomobile，执行 `mobile/scripts/build-aar.ps1` + `build-apk.ps1`。
- Honor 50 真机验收：前台/后台/锁屏/划掉任务/重启/强行停止；MagicOS 自启动与电池白名单。
- `mobile/core` 的测试二进制在本机被 Windows Defender 误判，CI 或关闭实时防护的机器可跑；
  纯逻辑测试已放在 `mobile/alerts`。
