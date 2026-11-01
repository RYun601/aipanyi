# 移动端体验与后台生命周期设计

- 日期：2026-11-01
- 分支：`feat/mobile-ux-config-storage-background`
- 状态：待评审
- 关联模块：`mobile/frontend`、`mobile/core`、`mobile/android`、`frontend`、`backend/data`、`backend/agent`、`backend/marketcal`

## 1. 背景与目标

当前移动端存在四类问题：

1. 自选股只能添加，不能删除；用户需要左滑露出删除按钮。
2. 模型配置需要手工填写最大输出、上下文窗口、Temperature、超时等参数；用户希望填好接口地址和 API Key 后一键获取模型列表，并自动带出通用参数。桌面端也要同步简化。
3. 移动端运行时文件散落在 Android 应用私有目录根部，用户希望统一收进一个子目录。
4. Android 前台服务 7×24 小时常驻，用户认为非交易时段没有意义；希望只在交易日、有实际提醒需求的市场交易时段运行，并且能明确退出。用户还会在交易日实际运行后连接手机检查日志，因此需要可追踪的自动启停/退出日志。

### 成功标准

- 自选列表左滑可以删除股票，删除后该股票的价格提醒同步停用。
- 移动端和桌面端都能通过「获取模型」拉取模型列表；选中模型后自动填充最大输出、上下文窗口，日常不再需要手动改这些参数。
- 移动端运行时数据库、日志、外置数据文件都位于 `filesDir/aipanyi/` 下，旧数据自动迁移且不丢失。
- 后台监控只在「已启用提醒涉及的市场」的交易窗口内运行；没有启用提醒时不常驻。
- 设置页可以手动退出应用；退出后本次停止，下一个未来监控窗口仍会自动启动。
- 自动启动、自动停止、手动退出、闹钟触发失败等关键事件都能在运行日志中按时间线查到。

## 2. 非目标

- 不改变桌面端的数据目录和版本更新机制。
- 不引入新的行情源、AI 供应商或后台推送通道。
- 不实现远程日志上传；日志只保存在手机本地。
- 不把 Android 前台服务改成 WorkManager 常驻任务。
- 不重构移动端现有的 AI 流式分析、提醒状态机。

## 3. 现状与约束

- Android 端 Go 核心由 `MonitorService`（独立 `:monitor` 进程）启动，`MainActivity` 只承载 WebView。
- `Mobilecore.start(dataDir, notifier)` 目前直接用 `filesDir` 作为数据根目录：数据库为 `filesDir/aipanyi.db`，日志为 `filesDir/logs`。
- Android 应用进程工作目录是 `/`，没有外部存储权限，代码中也没有写 `/sdcard` 的路径。用户看到的 `/sdcard` 文件无法由当前代码直接写入；本次仍会做全局审计并把所有已知运行时写入收敛到应用私有目录新根目录。
- `MonitorService` 当前返回 `START_STICKY`，`onTaskRemoved` 会在 1 秒后重新拉起；`BootReceiver` 在启用提醒数大于 0 时启动服务，因此服务实际上长期常驻。
- `marketcal` 已有 A/HK/US 交易时段和交易日判断，但只提供“当前是否在交易时段”的布尔判断，没有“下一个交易窗口”的计算。
- 移动端 AAR 使用 `gomobile bind` 生成，`Notifier` 接口变更后需要重新构建 AAR。
- Android 12+ 精确闹钟需要 `SCHEDULE_EXACT_ALARM` 权限；Android 14 新安装应用默认可能未授权，必须提供降级和授权入口。

## 4. 决策摘要

| 问题 | 决策 |
| --- | --- |
| 后台监控窗口 | 跟随当前启用提醒涉及的市场，多个市场窗口重叠时合并 |
| 调度实现 | Go 核心计算窗口，通过 `Notifier` 下发起止时间，Kotlin 用 `AlarmManager` 定时启停 |
| 无启用提醒 | 不安排后台启动；用户打开 App 时服务仅为当前页面运行，离开后停止 |
| 手动退出 | 立即停止后台服务和页面；保留下一个未来窗口的自动启动计划 |
| 模型参数 | 常用区只保留「深度思考」开关；其余参数移入默认收起的「高级参数」 |
| 模型探测 | 桌面端和移动端共用 `backend/data` 中的模型列表/模型信息实现 |
| 内置模型表 | 抽到轻量包 `backend/modelinfo`，避免移动端引入 `backend/agent` 重依赖 |
| 移动端数据根目录 | `filesDir/aipanyi/`，首次启动自动迁移旧文件 |
| 运行日志 | Kotlin 写 `filesDir/aipanyi/logs/runtime.log`；Go 写现有 `info.log`/`error.log` |

## 5. 详细设计

### 5.1 自选左滑删除

**前端**

- `mobile/frontend/src/views/Watchlist.vue` 中每个股票项用 `van-swipe-cell` 包裹。
- 右侧插槽放红色方形「删除」按钮。
- 点击删除时：
  1. 弹出 `showConfirmDialog`，提示股票名称；
  2. 确认后调用 `api.removeStock(item.code)`；
  3. 成功则从 `items` 移除并提示；
  4. 失败保留原项并提示错误。
- 股票主体点击仍然进入详情；删除按钮点击不触发详情跳转。

**后端**

- 复用现有 `RemoveStock`，它已经会：
  - 调用 `UnFollow` 取消关注；
  - 将该股票的价格提醒置为 `enabled=false`；
  - 刷新常驻通知的提醒数量。
- 取消关注后触发监控窗口重算（见 5.4）。

### 5.2 模型配置简化

#### 5.2.1 后端共用实现

新增 `backend/modelinfo`：

- 迁移 `backend/agent/builtin_models.go` 中的内置模型参数表和匹配逻辑。
- 暴露：
  - `GetBuiltinModelContextWindow(modelName string) int`
  - `GetBuiltinModelMaxOutput(modelName string) int`
- `backend/agent/builtin_models.go` 保留同名包装函数，现有调用点和测试不变。

新增 `backend/data/ai_models_api.go`：

- 定义 `AiModelInfo`：
  - `ModelName`
  - `MaxTokens`（输出上限）
  - `ContextWindow`（输入+输出总容量）
  - `Source`（`api` / `builtin`）
- `FetchAiModels(baseUrl, apiKey, extraHeaders string) []string`
  - 请求 `{baseUrl}/models`；
  - 使用 `Authorization: Bearer`；
  - 支持自定义 Header；
  - 失败返回空列表并记录日志。
- `FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders string) *AiModelInfo`
  - 先请求 `{baseUrl}/models/{modelName}`，读取 `max_context_length`、`context_length`、`max_output_tokens`、`max_tokens`；
  - 缺失字段用 `backend/modelinfo` 兜底；
  - 返回 `Source` 说明来源。

桌面端 `app.go`：

- `FetchAiModels`、`FetchAiModelInfo` 改为调用 `backend/data` 的共用实现。
- `main.AiModelInfo` 保持现有结构，内部转换，避免 Wails 绑定和前端类型大改。

移动端 `mobile/core/app.go`：

- 新增 `FetchAiModels`、`FetchAiModelInfo` 方法，返回与后端一致的结构。
- `mobile/frontend/src/api.js` 增加对应调用。

#### 5.2.2 移动端界面

`mobile/frontend/src/views/Settings.vue`：

- AI 编辑弹窗常用区域：
  - 名称
  - BaseUrl
  - API Key
  - 模型名（可手动输入，也可点「获取模型」）
  - 深度思考开关
- 「获取模型」按钮：
  - 校验 BaseUrl 和 API Key；
  - 调用 `FetchAiModels`；
  - 弹窗展示可搜索的模型列表；
  - 选中后写入模型名，并调用 `FetchAiModelInfo`；
  - 自动填充 `maxTokens`、`contextWindow`；
  - 名称为空时用模型名自动填充；
  - 提示自动填充结果和来源。
- 「高级参数」默认收起，包含：
  - Temperature
  - MaxTokens
  - 上下文窗口
  - 超时
  - HTTP 代理及开关
  - 自定义 Header
  - 视觉理解
  - 模型类型
- 已有配置的所有字段原样保留；用户展开高级参数仍可覆盖自动填充值。
- 获取模型失败时：
  - 保留手动输入模式；
  - 提示检查地址、API Key 或服务是否实现 `/models`；
  - 不阻塞保存。

#### 5.2.3 桌面端界面

`frontend/src/components/ai-config-manager.vue`：

- 模型名称旁增加显式「获取模型」按钮，不再依赖点击下拉框触发请求。
- 拉取成功后自动打开选项；当前模型名为空时选中第一项并自动查询模型信息。
- `Temperature`、`MaxTokens`、上下文窗口、`Timeout` 移入 `n-collapse` 的「高级参数」，默认收起。
- 模型类型、视觉理解、HTTP 代理、自定义 Header 同样移入高级参数。
- 「深度思考」保留在常用区域。
- 自动填充逻辑复用现有 `fetchModelInfo`，并显示来源。

### 5.3 移动端文件归拢

#### 5.3.1 新目录结构

```text
filesDir/
└─ aipanyi/
   ├─ aipanyi.db
   ├─ aipanyi.db-wal
   ├─ aipanyi.db-shm
   ├─ logs/
   │  ├─ info.log
   │  ├─ error.log
   │  └─ runtime.log
   └─ data/
      ├─ hot_money_seats.json
      └─ key_departments.json
```

#### 5.3.2 启动流程

`mobile/core/core.go` 的 `Start`：

1. 计算 `rootDir = filepath.Join(dataDir, "aipanyi")`；
2. 创建 `rootDir`；
3. 执行 `migrateLegacyData(dataDir, rootDir)`；
4. `data.SetDataDir(rootDir)`；
5. `logger.SetLogDir(filepath.Join(rootDir, "logs"))`；
6. `db.InitWithOptions(filepath.Join(rootDir, "aipanyi.db"), db.MobileOptions())`。

#### 5.3.3 旧数据迁移

迁移是幂等的，只移动已知运行时文件：

- `filesDir/aipanyi.db` → `filesDir/aipanyi/aipanyi.db`
- `filesDir/aipanyi.db-wal`、`filesDir/aipanyi.db-shm` 同理
- `filesDir/logs/` → `filesDir/aipanyi/logs/`
- `filesDir/data/` → `filesDir/aipanyi/data/`
- `filesDir/stock_basic.json` → `filesDir/aipanyi/stock_basic.json`
- `filesDir/data/hot_money_seats.json`、`filesDir/data/key_departments.json` → `filesDir/aipanyi/data/` 下对应文件
- 如果历史上直接写在根部的 `filesDir/hot_money_seats.json`、`filesDir/key_departments.json` 存在，也一并迁移到 `filesDir/aipanyi/data/`

规则：

- 目标已存在时保留目标文件，不覆盖，源文件保留并记录警告；
- `rename` 失败时记录错误，不阻塞启动；
- 迁移必须在数据库初始化之前完成；
- 迁移日志写入 `mobile: migrate legacy data ...`，便于排查。

#### 5.3.4 `backend/data` 可配置数据目录

新增：

```go
func SetDataDir(dir string)
```

`backend/data` 中所有已知外置文件写入统一走 `dataPath(name)`：

- `data/hot_money_seats.json`
- `data/key_departments.json`
- `stock_basic.json`

桌面端不调用 `SetDataDir`，默认仍是当前工作目录，行为不变。移动端在 `Start` 中设置为 `rootDir`，因此这些文件会落在 `filesDir/aipanyi/` 下。

全局审计结论：

- 当前 `mobile/core` 链接到的代码中，除 logger、SQLite、上述 `backend/data` 外置文件外，没有写外部存储的路径；
- AndroidManifest 没有 `READ/WRITE_EXTERNAL_STORAGE`、`MANAGE_EXTERNAL_STORAGE` 权限；
- 因此 `/sdcard` 根目录文件不是当前 App 代码写入的；本次仍会保证所有已知写入收敛到应用私有目录。

### 5.4 后台监控生命周期与调度

#### 5.4.1 交易窗口计算

`backend/marketcal` 新增：

```go
type MonitorWindow struct {
    Start time.Time
    End   time.Time
}

func NextTradingSession(market string, now time.Time) (start, end time.Time, ok bool)
func NextMonitorWindow(markets []string, now time.Time) (MonitorWindow, bool)
func NextMonitorWindowAfter(markets []string, after time.Time) (MonitorWindow, bool)
```

交易时段定义：

| 市场 | 交易日判断 | 监控窗口（本地市场时间） |
| --- | --- | --- |
| A | `IsTradingDay` | 09:15–15:00 北京时间 |
| HK | `IsHKTradingDay` | 09:00–16:10 北京时间 |
| US | `IsUSTradingDay` | 04:00–20:00 纽约时间（含盘前盘后） |

实现要点：

- A/HK 使用固定北京时间偏移；
- US 使用 `America/New_York`，依赖 Go 内嵌 `time/tzdata`；
- 从 `now` 开始向后扫描最多 10 天，找到每个市场“当前或下一个”交易窗口；
- 多个市场窗口按开始时间排序，重叠或首尾相接的窗口合并；
- 返回包含 `now` 的窗口；若都不包含，返回最早开始的未来窗口；
- `NextMonitorWindowAfter` 用于计算当前窗口结束后的下一个窗口。

新增导出包装：

- `IsHKTradingDay(date time.Time) bool`
- `IsUSTradingDay(date time.Time) bool`

#### 5.4.2 Go 核心下发调度

`Notifier` 扩展：

```go
type Notifier interface {
    Notify(title, body, payload string)
    UpdateMonitorStatus(active int32)
    UpdateMonitorSchedule(startMs, endMs, nextStartMs int64)
}
```

移动端新增 `refreshMonitorSchedule()`：

1. 查询所有 `enabled=true` 的价格提醒，去重得到市场集合；
2. 如果没有提醒，下发 `0, 0, 0`；
3. 计算当前窗口和下一个窗口；
4. 下发 Unix 毫秒时间戳；
5. 记录日志：
   - `mobile: monitor schedule markets=[...] start=... end=... next=...`
   - 无提醒时记录 `mobile: monitor schedule cleared (no enabled alerts)`。

触发时机：

- `Start` 完成初始化后；
- `SaveAlert`；
- `DeleteAlert`；
- `SetAlertEnabled`；
- `RemoveStock`（会停用该股票提醒）；
- 提醒数量或市场变化后。

计算放到 goroutine 中执行，并用互斥锁合并短时间内的重复请求，避免阻塞保存操作。

#### 5.4.3 Kotlin 调度

新增 `MonitorScheduler.kt`：

- 保存 `window_start_ms`、`window_end_ms`、`next_start_ms` 到 `SharedPreferences`。
- `applySchedule(context, startMs, endMs, nextStartMs)`：
  - 当前在窗口内：确保 `MonitorService` 启动；
  - 窗口在未来：安排开始闹钟；
  - 窗口已结束：取消当前闹钟，安排下一个未来窗口；
  - 全部为 0：取消闹钟。
- `scheduleStart(context, atMs)`：
  - 使用显式 `PendingIntent` 指向 `MonitorAlarmReceiver`；
  - Android 12+ 且允许精确闹钟：`setExactAndAllowWhileIdle`；
  - 否则：`setAndAllowWhileIdle`；
  - 记录日志：目标时间、精确/不精确、是否授权。
- `resumeAfterBoot(context)`：
  - 读取已存窗口；
  - 当前在窗口内：立即启动服务；
  - 否则安排下一个未来窗口；
  - 记录日志。

新增 `MonitorAlarmReceiver.kt`：

- 收到闹钟后尝试 `ContextCompat.startForegroundService`；
- 成功记录日志；
- 失败（如 Android 12+ 后台启动限制）记录错误并发送一条可点击通知，提示用户打开 App 恢复监控；
- 不静默失败。

`MonitorService` 变化：

- 增加 action：
  - `ACTION_START`：定时/开机启动；
  - `ACTION_UI_VISIBLE`：Activity 可见；
  - `ACTION_UI_HIDDEN`：Activity 不可见；
  - `ACTION_SCHEDULE_CHANGED`：Go 下发了新窗口；
  - `ACTION_EXIT`：手动退出。
- 维护 `uiVisible` 标志。
- 根据窗口结束时间安排 `Handler.postDelayed`，窗口结束时调用 `stopIfIdle`。
- `stopIfIdle(reason)`：
  - 如果 Activity 可见，保持服务，避免正在使用的 WebView 断掉；
  - 如果 Activity 不可见且当前不在监控窗口，调用 `Mobilecore.stop()`、`stopForeground(STOP_FOREGROUND_REMOVE)`、`stopSelf()`；
  - 停止前安排下一个未来窗口；
  - 记录日志：`reason=window_end|ui_hidden|manual_exit|no_schedule`。
- `onTaskRemoved`：
  - 删除原有的 1 秒自动重启；
  - 窗口内保持服务；窗口外停止。
- `onDestroy`：
  - 取消 Handler；
  - 调用 `Mobilecore.stop()`；
  - 记录日志。
- `START_STICKY` 保留，但系统重启后若不在窗口且 UI 不可见，立即停止。

`MainActivity` 变化：

- `onStart` 发送 `ACTION_UI_VISIBLE` 启动/唤醒服务。
- `onStop` 在非配置变更时发送 `ACTION_UI_HIDDEN`。
- 新增 `exitApp()`：
  - 记录手动退出日志；
  - 停止 `MonitorService`；
  - `finishAndRemoveTask()`；
  - 保存的下一个未来窗口计划保留。

`NativeBridge` 增加：

- `exitApp()`
- `canScheduleExactAlarms(): Boolean`
- `requestExactAlarmPermission()`

`mobile/frontend/src/native.js` 增加对应封装。

`Settings.vue` 的「后台与通知」增加：

- 「精确闹钟」状态行：未授权时显示「去开启」；
- 「退出应用」按钮：二次确认后调用 `native.exitApp()`。
- 说明文案：非交易时段自动停止，下个交易日按已启用提醒的市场自动启动。

`AndroidManifest.xml` 增加：

```xml
<uses-permission android:name="android.permission.SCHEDULE_EXACT_ALARM" />
<receiver android:name=".MonitorAlarmReceiver" android:exported="false" />
```

#### 5.4.4 边界行为

- 无启用提醒：Go 下发 `0,0,0`；UI 不可见后服务停止，不安排闹钟。
- 只有 A 股提醒：只在 A 股窗口运行。
- 只有美股提醒：只在纽约时间 04:00–20:00 对应北京时间窗口运行。
- A/HK/US 窗口重叠或首尾相接：合并为一个连续窗口。
- 手动退出发生在窗口中间：停止本次运行，下一个未来窗口仍会启动。
- 手动退出发生在窗口开始前：下次窗口仍会按计划启动。
- Activity 在窗口结束时仍可见：服务保持到用户离开页面，离开后停止。
- 精确闹钟未授权：使用不精确闹钟；设置页提示，日志记录 `exact=false`。

### 5.5 运行追踪日志

目标：用户交易日实际运行后连接手机，可以还原“谁在什么时间启动了服务、为什么停止、退出是否生效”。

#### 5.5.1 Kotlin 日志

新增 `RuntimeLog.kt`：

- 文件：`filesDir/aipanyi/logs/runtime.log`
- 单文件上限：512 KB
- 滚动保留：`runtime.log.1`、`runtime.log.2`，共 3 个文件
- 每行格式：`yyyy-MM-dd HH:mm:ss.SSS LEVEL/TAG message`
- 同时输出到 `Log.i/w/e`，便于 `adb logcat` 实时观察
- 写入使用同步追加，单行短消息，避免复杂线程池

新增 `AiPanyiApp.kt`（`Application`）：

- `onCreate` 初始化 `RuntimeLog`
- 记录进程启动：`process start pid=...`
- Manifest 的 `<application>` 增加 `android:name=".AiPanyiApp"`

关键日志点：

| 模块 | 事件 | 级别 | 关键字段 |
| --- | --- | --- | --- |
| AiPanyiApp | 进程启动 | I | pid |
| MainActivity | onStart/onStop/onDestroy | I | activity 生命周期 |
| MainActivity | 手动退出 | I | `manual exit requested` |
| MonitorService | onCreate/onDestroy | I | service 生命周期 |
| MonitorService | onStartCommand | I | action、uiVisible、窗口状态 |
| MonitorService | 核心启动成功/失败 | I/E | URL / 异常 |
| MonitorService | 停止 | I | reason |
| MonitorService | onTaskRemoved | I | 是否窗口内 |
| MonitorScheduler | 应用调度 | I | start/end/next、当前是否窗口内 |
| MonitorScheduler | 安排闹钟 | I | 目标时间、exact/inexact |
| MonitorScheduler | 取消闹钟 | I | reason |
| MonitorScheduler | 开机恢复 | I | 当前窗口/下一个窗口 |
| MonitorAlarmReceiver | 闹钟触发 | I | 目标时间、实际时间 |
| MonitorAlarmReceiver | 启动失败 | E | 异常类型 |
| AppNotifier | 收到 Go 调度 | I | start/end/next |
| AppNotifier | 提醒数量更新 | I | active |
| NativeBridge | 精确闹钟状态/授权跳转 | I | allowed |

不记录：API Key、完整请求头、用户自选股列表等敏感信息。

#### 5.5.2 Go 日志

继续使用 `logger.SugaredLogger`，写入 `filesDir/aipanyi/logs/info.log` / `error.log`。

新增日志点：

- `mobile: core start dataDir=...`
- `mobile: core stop`
- `mobile: migrate legacy data from=... to=... moved=... skipped=...`
- `mobile: monitor schedule markets=[...] start=... end=... next=...`
- `mobile: monitor schedule cleared (no enabled alerts)`
- `mobile: monitor schedule notify failed: ...`
- 现有 `mobile: started at ... (version=...)`

#### 5.5.3 事后分析方式

用户连接手机后：

- Kotlin 时间线：`adb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/runtime.log`（debug 包）；或通过文件管理器进入应用私有目录导出。
- Go 时间线：`files/aipanyi/logs/info.log`、`error.log`。
- 对照 `MonitorScheduler` 的窗口时间、`MonitorService` 的启动/停止原因、`MonitorAlarmReceiver` 的触发时间，即可判断是否符合预期。

如果后续需要更方便地导出，可再增加「分享运行日志」按钮；本次不实现，避免引入 FileProvider 和额外权限。

## 6. 兼容性与数据迁移

- 现有移动端数据库、日志自动迁移到 `filesDir/aipanyi/`；迁移失败时原文件保留。
- 现有 AI 配置字段全部保留；界面只是把高级字段折叠，不会清空已有值。
- 桌面端 Wails 绑定签名保持兼容；只替换内部实现。
- `Notifier` 接口新增方法属于破坏性变更，但只影响本仓库 Android 壳，随 AAR 一起更新。
- `mobile/android/README.md` 需要同步说明新的数据目录和 AAR 接口。

## 7. 错误处理

- 获取模型列表失败：返回空列表，前端提示，不抛未捕获异常。
- 获取模型信息失败：保留手动输入和已有参数，不影响保存。
- 存储迁移失败：记录错误，继续使用新目录；如果旧数据库未迁移成功，启动时可能创建空库，需在日志中明确标记。
- 本地服务启动失败：沿用现有 `MainActivity` 错误页和重试入口。
- 闹钟启动服务失败：记录错误并发送可点击通知，不静默失败。
- 精确闹钟未授权：降级为不精确闹钟，并在设置页提示。
- 没有启用提醒：不安排后台任务，不视为错误。

## 8. 测试与验收

### 8.1 自动化

- `go build ./...`
- `go vet ./...`
- `go test ./backend/marketcal/... ./backend/modelinfo/... ./backend/agent/...`
- `go test ./webserver/...`
- `go build -tags mobileassets ./mobile/...`
- `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -tags mobileassets ./mobile/...`
- `cd mobile/frontend && npm run build`
- `cd frontend && npm run build`
- 如环境允许：`mobile/scripts/build-aar.ps1`、`mobile/scripts/build-apk.ps1`，确认 Kotlin 编译通过。

### 8.2 真机验收

1. 自选列表左滑出现删除；确认后股票消失，相关提醒被停用。
2. 模型配置：填 BaseUrl + Key，点获取模型，选择模型后最大输出/上下文窗口自动填充；不展开高级参数也能保存。
3. 旧版本升级后，`filesDir/aipanyi/` 下出现数据库和日志；旧位置文件不再增长。
4. 无提醒时，退出 App 后服务停止，常驻通知消失。
5. 只有 A 股提醒时，非 A 股交易时段服务停止；A 股窗口开始后自动启动。
6. 有美股提醒时，美股窗口自动启动；窗口结束自动停止。
7. 设置页手动退出：页面关闭、服务停止；下个未来窗口仍自动启动。
8. 精确闹钟未授权时，设置页提示；授权后可正常定时启动。
9. `runtime.log` 中能按时间顺序看到：调度下发 → 闹钟安排 → 闹钟触发 → 服务启动 → 核心启动 → 窗口结束/手动退出 → 服务停止。

## 9. 交付拆分

建议按 4 个逻辑提交，最终一个 PR：

1. `feat(mobile): 自选左滑删除`
2. `feat(ai): 模型自动获取与高级参数折叠`（含桌面端）
3. `refactor(mobile): 运行时文件归拢到应用子目录`
4. `feat(mobile): 按交易窗口自动启停后台服务`（含运行日志）

## 10. 评审记录

- 2026-11-01：初稿完成，等待评审。
