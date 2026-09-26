# ADR-001：UI 浏览器化与 AI 边车架构决策

| 项 | 内容 |
| --- | --- |
| 状态 | **已接受** |
| 日期 | 2026-09-25 |
| 影响范围 | 启动流程、UI 承载方式、前后端通信层、AI 能力扩展方式、分发形态 |
| 相关代码 | `main.go`、`app_common.go`、`app_windows/darwin/linux.go`、`backend/agent/*`、`frontend/*`、`ai-assistant-web/*` |

---

## 1. 背景

当前 AI盘译 是 Wails v2.15 桌面应用：Go 后端 + Vue3 前端打包进原生 webview 窗口。现状盘点：

- **前端耦合面**：419 个 Wails 绑定方法（`frontend/wailsjs/go/main/App.js`）、约 40 个后端事件（`EventsEmit`）、前端 123 处 `EventsOn` 监听；前端几乎不直接使用 `runtime.*`（仅 `EventsOn/EventsOff/BrowserOpenURL`）。
- **已有效仿先例**：`ai-assistant-web` 已在同进程内以 Go HTTP 服务 + 内嵌静态页 + SSE 流式 + VIP 鉴权的方式跑通过"Go 后端直接服务浏览器前端"的完整链路。
- **痛点**：小屏幕下 UI 被遮挡。根因是布局（固定宽度表格、悬浮 AI 面板、图表不随容器 resize），但在 Wails 窗口内受 webview 缩放 quirks 与窗口尺寸约束，适配手段受限。
- **原生依赖面很窄**：托盘（`getlantern/systray`）、系统通知（`go-toast`）、文件对话框（`runtime.OpenFileDialog/SaveFileDialog`）、`BrowserOpenURL`。其中托盘与通知是纯 Go 库，与 Wails 无耦合。

## 2. 决策

采用 **「Go 单体核心 + 本地 HTTP/WS 服务承载浏览器 UI + 原生托盘 + 按需 Python AI 边车（MCP）」**，移除 Wails，作为项目的最终形态（非过渡方案）。

**启动方式**：程序启动后在**默认浏览器**打开 `http://127.0.0.1:<port>`（普通标签页形态；不采用 `--app` 无边框窗口模式，保持浏览器原生体验与 DevTools 可用性）。

## 3. 目标架构

```
┌──────────────────────────────────────────────────────┐
│  AI盘译.exe（单文件，无 webview 运行时依赖）          │
│                                                      │
│  ┌──────────────┐   ┌──────────────┐   ┌───────────┐ │
│  │ 行情/监控核心  │   │ eino Agent   │   │ 本地向量库  │ │
│  │ TDX/爬虫/DB  │   │ 编排 + 工具   │   │ + 长期记忆 │ │
│  └──────────────┘   └──────────────┘   └───────────┘ │
│           │                │                          │
│  ┌────────┴────────────────┴───────────────────────┐ │
│  │  Web 层                                        │ │
│  │   POST /api/call   反射派发 App 方法（承接 419 绑定）│ │
│  │   GET  /ws         事件总线（承接 ~40 个事件）      │ │
│  │   POST /api/upload 文件上传（替代 OpenFileDialog） │ │
│  │   GET  /api/file/* 文件下载（替代 SaveFileDialog） │ │
│  │   127.0.0.1 绑定 + 启动随机 token 鉴权             │ │
│  │   一进程一实例（端口锁），二实例唤起浏览器           │ │
│  └──────────────┬──────────────────────────────────┘ │
│  ┌──────────────┴──────────────────────────────────┐ │
│  │  systray 托盘：显示界面→打开浏览器 / 退出           │ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
                   │ MCP（按需，可选开启）
                   ▼
        Python AI 边车（可选下载，默认不随包）
        · bge-m3（bge-small-zh）：本地 embedding（唯一职责）
        · 可选扩展：bge-reranker（后置）
        · 否决项：Qlib 回测 / AKShare 数据源（见 4.5）
```

## 4. 详细设计

### 4.1 Web 服务层

| 项 | 决策 |
| --- | --- |
| 监听地址 | 仅 `127.0.0.1`；局域网访问默认关闭，设置页显式开启并强提示 |
| 端口 | 默认 7077，被占用则递增探测至 7100；最终端口写入日志、托盘 tooltip、设置页 |
| 鉴权 | 进程启动时生成随机 token，注入 `index.html`（`window.__GOSTOCK_TOKEN__`）；`/api/*` 与 `/ws` 强制校验，防止本地恶意网页/程序驱动 |
| 单实例 | 监听固定 instance 端口作为进程锁；第二实例启动时直接打开已运行实例的浏览器地址后退出（替代 `SingleInstanceLock`） |
| 自动打开浏览器 | Windows `rundll32 url.dll,FileProtocolHandler <url>`；macOS `open <url>`；Linux `xdg-open <url>`；失败仅记日志不阻断 |
| 静态资源 | 沿用 `//go:embed frontend/dist`，与现有 `assets` 一致 |

### 4.2 绑定层（承接 419 个方法）

- 新增 `POST /api/call`，请求体 `{"method": "...", "args": [...]}`；
- 服务端反射派发：`reflect.ValueOf(app).MethodByName(method)`，按声明参数类型 `reflect.New(t)` + `json.Unmarshal` 构造实参；
- 返回值约定与 Wails 对齐：最后一个返回值为 `error` 时转为 `{"data":..., "error":...}`，前端 shim 将其抛为异常，保持既有 try/catch 语义；
- **异步白名单**：`ChatWithAgent`、`ChatWithAgentKBQA`、`CheckUpdate` 等"调用后长时间不返回、结果走事件"的方法必须立即返回 `{"ok":true}`，结果经 WS 推送；
- 前端 `wailsjs/go/main/App.js` 由脚本基于 Go 方法签名重新生成：**导出函数签名不变**，内部统一走 `fetch("/api/call")`。

### 4.3 事件层（承接约 40 个事件）

- 新增 `events` 包，提供两个实现，用 build tag 切换：
  - `//go:build wails` → 转调 `runtime.EventsEmit`（迁移期桌面版）；
  - `//go:build !wails` → WS hub 广播；
- 全量替换 `runtime.EventsEmit(a.ctx, ...)` 为 `events.Emit(a.ctx, ...)`（机械替换，约百处）；
- WS hub 语义对齐 Wails 多窗口：所有连接广播同一事件流；每连接独立 goroutine + 有界缓冲，慢消费者断开清理；ping/pong 心跳；前端断线重连；
- 前端 `wailsjs/runtime` 的 `EventsOn/EventsOff` shim 为 `ws.onmessage` 按事件名分发，**`src/` 业务代码零改动**。

### 4.4 原生能力处置

| 能力 | 现状 | 目标态 |
| --- | --- | --- |
| 托盘菜单 | `getlantern/systray` | **保留**；“显示窗口”改为打开浏览器 URL |
| 系统通知 | `go-toast` | **保留**（纯 Go，与 Wails 无关） |
| 打开外链 | `runtime.BrowserOpenURL` | 前端 `window.open` |
| 选择文件（3 处） | `runtime.OpenFileDialog` | `<input type="file">` + `POST /api/upload` |
| 保存文件（5 处） | `runtime.SaveFileDialog` | 前端 blob 下载；服务端已有文件内容的场景直接前端生成 |
| 剪贴板 | `navigator.clipboard` | 不变（localhost 下可用） |
| 窗口菜单/尺寸记忆 | `wails.Run` options | 下线（含 `getScreenResolution` 等逻辑） |
| 自动更新 | `update_helper_*.go` | **保留**（下载 + 替换 exe，不依赖 Wails） |

### 4.5 Python AI 边车（经 MCP）——职责已收窄

> 初版曾将 Qlib 回测、AKShare 数据源也列入边车职责，经审查**否决**（Qlib 迁移成本高且受众小；AKShare 同为爬虫、数据 ToS 风险相同，不解决根问题）。边车职责收缩为一项。

- **唯一职责**：本地 embedding（bge-m3 或 bge-small-zh）。价值：持仓/自选股等敏感数据不出本机、零 API 成本、离线可用。可选扩展 bge-reranker（后置）。
- **接入方式**：以 MCP Server（stdio 或 SSE）形态注册，复用现有 `backend/data/mcp_server_api.go` 与 `maybeGetMCPTools` 客户端能力，Go 侧无需写专用协议。
- **生命周期**：按需拉起（首次调用 Python 系工具时）、健康检查、失败自动降级到远程 embedding；**默认不随包分发**，设置页提供一键安装/卸载。
- **红线**：Agent 编排、工具调度、数据主权、DB 永远留在 Go。Python 只是"模型外设"，不进入主请求链路关键路径。

### 4.6 数据层：维持现状

- 向量存储**继续使用 chromem-go，不迁移 sqlite-vec**。
- 否决理由：chromem-go 已在工作；“备份=拷贝一个文件”不成立（`memory/` 目录本就可整体拷贝），迁移风险大于收益。


### 4.7 移除 Wails

- 删除 `wails.Run` 及相关 options；`frontend/wailsjs/` 由 shim 实现取代生成物；窗口相关代码下线。
- 迁移期通过 build tag 双模式并存（`-tags wails` 产出旧桌面版），目标态彻底移除。

### 4.8 商业化模式（已决策：免费下载 + 付费增值服务）

**分发形态**：打包为用户下载到个人电脑使用，**不提供远程托管/SaaS**。因此 GPLv3 全义务生效：

| 义务 | 落地做法 |
| --- | --- |
| 提供完整对应源码（含修改与构建脚本） | 公开仓库放完整可构建源码 |
| 保留协议、版权声明、标注修改 | 随包带 `LICENSE`，关于页注明“基于 AI盘译（GPLv3）修改 + 日期” |
| 整体以 GPLv3 授权 | 新增代码不闭源 |
| 不得附加再分发限制 | EULA 不得含“禁止转售/再分发”条款 |

**付费点必须放在服务端**（账号 + 云资源）：GPL 赋予用户修改权，二进制内的功能锁可被合法移除，本地功能锁等于没锁。免费版必须完整可用。

**服务端数据服务的正确形态：聚合缓存 + 扇出**——服务端批量拉取全市场快照（东财等接口一次返回全市场）→ 缓存（行情 3~10s / 榜单 60s / 静态资料 1h）→ 扇出给所有用户，使**上游请求量与用户数彻底解耦**，单 IP 即可承载。

**明确否决：代理池 / 请求转发型中继。** 单台服务器只有一个出口 IP，把所有用户请求汇聚到该 IP 会导致集中封禁后全体瘫痪；而免费版用户各自直连时，家庭宽带 IP 天然分散，被限流只影响单人。购买住宅代理池则是持续的烧钱跑步机。

**付费服务清单（已清洗）**：

| 优先级 | 服务 | 说明 |
| --- | --- | --- |
| P0 | 云行情快照扇出 | 低延迟、抗网络抖动、多源交叉校验 |
| P0 | 历史数据仓库 | 离线慢速拉取 10 年日线/分钟线，提供查询与导出 |
| P0 | AI 额度托管 | 官方 API 中转计费，消除“没有 API Key”的门槛 |
| P1 | 云监控（关机盯盘）+ 手机端 H5 只读看板 | 服务端复用 signal/cron/daily 逻辑持续执行；H5 只读同步自选股，顺带解决小屏 |
| P2 | 采购授权数据源接入 | Tushare Pro 等，合规 + 质量 + 天然付费墙 |

**明确不做**：云端 embedding/rerank 付费（本地 bge 免费可替代）、AI 用量看板（用户无感）、短信/语音告警（需 SP 资质且低频，钉钉/飞书已覆盖）、云端回测与策略市场（受众小，且“售卖选股策略”涉投顾资质红线）。

**云同步边界**：仅同步自选股与设置；**绝不同步持仓**——持仓属敏感个人信息，上云触发合规与安全责任，收益与风险不对称。

## 5. 备选方案与否决理由

| 方案 | 结论 | 理由 |
| --- | --- | --- |
| 继续优化 Wails 响应式 | 否决 | 天花板低：webview 缩放 quirks、窗口最小尺寸约束、无多设备能力；治标不治本 |
| Wails v3 browser 模式 | 否决 | v3 仍在演进，生态与稳定性需长期验证；自建 Web 层模式已被 `ai-assistant-web` 验证，可控性更高 |
| Tauri v2 | 否决 | 仍是桌面 webview 窗口，不满足浏览器形态诉求；引入 Rust 侧纯增风险 |
| Electron | 否决 | 仅使壳变重、分发体积翻倍，Go 后端价值不变，零收益 |
| 整体迁 Python | 否决 | 丢失单二进制分发、TDX 二进制解析、高并发行情轮询、后台监控等 Go 主场能力 |
| Rust 重写 | 否决 | 十万行级可用代码重写，AI/量化生态弱于 Go，纯负收益 |
| 上云 Web 化 | 否决 | 持仓隐私、本地行情数据、离线可用性是产品根基 |

## 6. 代价与风险

| 风险 | 缓解措施 |
| --- | --- |
| 本地服务被本地恶意程序/网页驱动 | 仅绑 127.0.0.1 + 启动随机 token 强制校验 + 默认关闭局域网；局域网模式开启时强提示 |
| 419 绑定的反射派发存在类型边界 case（`*int`/`time.Time`/嵌套结构体/多返回值） | 编写全量绑定回归脚本，对比 Web/Wails 两种模式的返回结构 |
| 异步方法阻塞 HTTP | 异步白名单显式声明 + 集成测试覆盖 |
| 多标签/多设备并发 | 沿用 Wails 多窗口广播语义；写操作沿用 `agentCancel` 互斥串行化 |
| Python 边车环境复杂（依赖/版本/性能） | 按需拉起、健康检查、失败降级；默认不分发，opt-in |
| 分发形态变化（安装包 → 单 exe） | 更新文档与首次启动引导（自动开浏览器 + 托盘常驻） |
| **GPLv3 分发义务**（须开源修改版、不得限制再分发） | 公开完整可构建源码；随包带 LICENSE 并标注修改；付费点全部服务端，二进制内不做功能锁 |
| **数据源 ToS 风险**（东财/问财/新浪等普遍禁止商用抓取） | 免费版如实披露数据来源；付费版采购授权数据源（Tushare Pro 等）；客户端退避+缓存降低频率 |
| 商标与品牌（GPL 不授予商标权） | 自有品牌名/logo，移除原作者 VIP/赞助校验体系，注明"基于 AI盘译 修改" |
| 服务端运营成本（数据中继/云监控/账号计费） | 先以单台服务器起步；聚合扇出使上游请求量与用户数解耦；按订阅阶梯控制成本 |

## 7. 迁移路径（按依赖排序，工期另议）

1. **Phase 1｜Web 骨架**：HTTP 服务 + 内嵌静态资源 + token 鉴权 + 端口探测 + 单实例锁 + 自动打开浏览器 + 托盘"显示界面"改打开 URL。
2. **Phase 2｜绑定层**：`/api/call` 反射派发器 + `wailsjs` shim 重新生成 + 异步白名单 + 全量回归脚本。
3. **Phase 3｜事件层**：`events` 包 + WS hub + 前端 `EventsOn` shim + 心跳重连。
4. **Phase 4｜文件能力**：上传/下载接口 + 前端 `input[type=file]` 与 blob 下载改造。
5. **Phase 5｜响应式改造**（与以上各阶段并行）：路由分页、栅格与媒体查询、图表 resize、小屏 AI 对话独立成页。
6. **Phase 6｜Python AI 边车**：MCP Server 框架 + 本地 embedding（bge）+ 按需生命周期管理。
7. **Phase 7｜服务端增值**：云行情扇出 + 历史数据仓库 + AI 额度托管 + 账号计费体系。
8. **Phase 8｜移除 Wails**：删除双模式代码路径，收尾。

> 原 Phase 7「sqlite-vec 向量库收敛」已否决，见 4.6；原 Phase 6 中的 rerank/Qlib/AKShare 已否决，见 4.5。

## 8. 待决问题

- [ ] 局域网（手机/平板）访问模式：建议实现但默认关闭，需产品侧确认是否纳入发布说明。
- [ ] Python 边车分发形态：安装包内置（体积 +1~2GB）vs 首次使用时按需下载（推荐后者）。
- [ ] 云同步范围：仅自选股与设置；若未来同步更多数据，是否端到端加密。
- [ ] `ai-assistant-web` 独立服务与新主 Web 层的关系：建议合并为同一服务的路由前缀，避免双端口双进程。
- [x] 品牌命名：已定「**AI盘译**」（AI盘译 / AI解盘 / AI析股 均已被同类产品占用）；module path 已改为 `aipanyi`，`backend/appinfo` 集中管理仓库坐标。原作者署名按 GPL 要求在关于页与 LICENSE 标注。

---

*本文档记录架构决策；后续如引入 LLM 语义路由、cross-encoder 重排、边车模型分级等方案，另立 ADR 补充。*

---

## 9. 实施进度

| 阶段 | 状态 | 说明 |
| --- | --- | --- |
| Phase 1｜Web 骨架 | ✅ 已完成 | `webserver/`（server/binding/hub/auth/static/upload）+ `backend/events/` 双模式事件桥 + `main.go` 的 `useWebUI` 开关；`-tags web` 与默认构建均通过编译、vet、测试 |
| Phase 2｜前端 shim | ✅ 已完成 | `frontend/wailsjs/bridge.js` 双模式桥接层；`go/main/App.js` 352 个方法重新生成走 `call()`；`runtime/runtime.js` 67 个导出全部改造（事件走 WS、窗口类 API 在 Web 模式安全降级）；`index.html` 增加 `__AIPANYI_TOKEN_VALUE__` 占位符 |
| Phase 3｜文件能力 | ✅ 已完成 | `app_file_bridge.go` 6 个取数据方法 + `frontend/src/utils/fileBridge.js`；11 处 OpenFileDialog/SaveFileDialog 调用点全部改造完成，Web 模式走 HTTP 上传/下载，桌面模式回退 Wails 原生对话框 |
| Phase 4｜响应式改造 | ✅ 已完成 | `responsive.css` 全局兜底（dvh/防溢出/弹窗/底部菜单）；`useResponsive` 断点；`useBottomMenuInset` 实测底部导航高度动态算内容区高度（替代原写死 96px，小屏遮挡主因）；`useChartAutoResize` + `attachChartResize` 覆盖 13 个图表组件；宽表格加 `scroll-x` + `dvh` 高度；`market.vue` 的 `window.onresize` 全局赋值改为可注销监听 |
| Phase 5｜Python AI 边车 | ⬜ 未开始 | 仅保留本地 bge embedding（Qlib/AKShare 已否决） |
| Phase 6｜服务端增值 | ⬜ 未开始 | 云行情扇出、历史数据仓库、AI 额度托管、账号计费 |
| Phase 7｜移除 Wails | ⬜ 未开始 | 目标态删除双模式代码路径 |

### 9.1 验证记录（Phase 1-2）

- `go build ./...` 与 `go build -tags web ./...` 均通过；`go vet` 两种模式均通过；
- `webserver` 包 11 个测试全部通过（桌面/Web 两种模式各跑一遍）：健康检查免鉴权、无 token 401、反射派发（多参/error 透传/指针 null/数组返回）、未知方法 404、参数个数不符 400、异步方法立即返回 accepted、**WS 建连后 `events.Emit` 事件送达前端**、错误 token 拒连、**index.html token 注入闭环**；
- 进程级冒烟（真实 `frontend/dist` 资产）：`/api/health` 200、`/` 注入令牌与启动令牌一致、带令牌 `/api/call` 正确返回、无令牌 401；
- 前端 `npm run build` 通过（1035 模块）。

### 9.2 构建命令

```bash
# 桌面模式（默认，行为与迁移前一致）
go build -o aipanyi.exe .

# Web 模式（本地 HTTP 服务 + 默认浏览器）
go build -tags web -o aipanyi-web.exe .

# 前端构建（两种模式共用）
cd frontend && npm run build
```

### 9.3 发布阻塞项处理进度

| # | 事项 | 状态 | 说明 |
| --- | --- | --- | --- |
| 1 | 自动更新 URL 指向原作者仓库 | ✅ 已完成 | `backend/appinfo` 集中管理仓库坐标；`Owner=RYun601`、`Repo=aipanyi`；`stock_basic*.json` 三处硬编码 IP（`8.134.249.145:18080`）改为 `appinfo.RawFileURL`；`wails.json`、CI 资产名、`hot_money_seats*.json` 的 `remoteUrl` 全部同步；CI 已改为原生 steps（见 9.9） |
| 2 | `sparkmemory.top` 上游依赖 | ⚠️ 部分处理 | `syncNews`（VIP 资讯同步）已标注为休眠状态；`DefaultPromptPlazaApiBase`（提示词广场）仍指向上游服务器，**用户点"分享到广场"会把提示词上传到第三方**，需自建服务或下线该功能 |
| 3 | 赞助/VIP 门禁 | ✅ 已彻底移除 | 自选股 63 只上限取消；`ai-assistant-web` 的 `requireVip2` 恒真；「关于」页原作者第三方支付 iframe 已换成本地收款码；README 赞助章节重写为"免费下载 + 自愿赞助"。**2026.09.26 二次清理**：前端全部硬门禁删净（见 9.10） |
| 4 | Web 模式系统托盘 | ✅ 已实现 | `app_tray_web.go`：跨平台托盘，"打开界面"用默认浏览器访问本地服务、"退出"结束进程；`webserver.CurrentURL()` 暴露当前地址。注：macOS 裸二进制可能不显示托盘图标 |
| 5 | 文件对话框前端改造 | ✅ 已完成 | `app_file_bridge.go` + `utils/fileBridge.js`，11 处调用点改造完毕；另修复 Phase 4 遗留的 3 处模板语法错误（App.vue / FloatingAiAssistant.vue / FloatingAgentAssistant.vue 分享入口残留）与 `style.css` 的 `@import` 位置错误 |

### 9.5 验证记录（Phase 3-4）

- `go build ./...`、`go build -tags web ./...`、`go vet`（两种模式）全部通过；
- `go test ./webserver/...` 两种模式全部通过；
- 前端 `npm run build` 通过（修复 3 处模板语法错误 + `style.css` 的 `@import` 位置错误后）；
- 响应式改动均为增量：桌面端（>=1280px）不改变既有视觉，小屏仅做兜底与降级。

### 9.6 Phase 4 响应式改动清单

| 文件 | 改动 |
| --- | --- |
| `frontend/src/responsive.css` | 新增。dvh 视口、全局防溢出、宽表格横向滚动、底部导航小屏适配、悬浮面板小屏近全屏、表格字号压缩、弹窗限宽 |
| `frontend/src/composables/useResponsive.js` | 新增。768/1280 双断点，模块级单例监听，多组件共用 |
| `frontend/src/composables/useBottomMenuInset.js` | 新增。ResizeObserver 实测底部固定导航高度，内容区 max-height 动态计算，**替代原写死 96px**（小屏遮挡主因） |
| `frontend/src/composables/useChartAutoResize.js` | 新增。`useChartAutoResize(ref)` + `attachChartResize(el, chart)`，ResizeObserver + rAF 合流 |
| `frontend/src/App.vue` | 接入 `useBottomMenuInset`；底部菜单挂 `bottomMenuRef`；`contentStyle` 改为动态计算 |
| `frontend/src/style.css` | responsive.css 的 import 移到文件首行（CSS 规范要求 @import 前置） |
| `stock.vue` | 4 个 ECharts 容器各自独立观测；两张宽表加 `scroll-x`；高度 `vh` 改 `dvh` |
| `market.vue` | `window.onresize` 全局赋值改为 `addEventListener` + `onBeforeUnmount` 解绑；小屏下调低高度预留量并设下限 |
| `allStockList.vue` / `SelectStock.vue` | 宽表横向滚动 + `dvh` 高度 |
| `AnalyzeMartket.vue` / `moneyTrend.vue` / `stockSparkLine.vue` / `FuturesPositionChart.vue` / `bkFundFlowChart.vue` / `conceptFundFlowChart.vue` / `ConceptDetailModal.vue` / `DailyOperationPlan.vue` / `PromptBacktest.vue` / `promptTemplateList.vue` | 接入图表容器自适应 |

### 9.7 商业化准备改动（README / 赞助 / 第三方依赖清理）

| 项 | 改动 |
| --- | --- |
| README | 整篇重写为「AI盘译 / aipanyi」：品牌、徽章、 Releases 链接、Star History 全部指向 `RYun601/aipanyi`；移除原作者 gitee/gitcode 镜像、QQ 群、硅基流动与 Anspire 推荐返利链接、VIP 分级赞助表、QQ 付费技术支持表；新增「关于收费」「与原版 AI盘译 的差异」「数据来源与免责声明」章节。原 README 备份为 `README.AI盘译.md.bak` |
| 赞助 | 「关于」页第三方支付 iframe → 本地收款码（`//go:embed` 进二进制，`GetVersionInfo` 以 data URI 下发）；`GetImageBase` 增加 PNG 魔数探测（原来写死 `image/jpeg`，换成 PNG 会显示空白）；README 与「关于」页均可扫码 |
| 广场下线 | `promptPlaza.vue` / `promptQa.vue` / `skillPlaza.vue` / `plazaAuthModal.vue` / `plazaBindEmailModal.vue` 共 5 个组件删除；App.vue 菜单项、researchIndex Tab、settings 表单项、skill-manager 入口全部移除；researchIndex 对老链接做兜底跳转（避免书签打开空白页） |
| 开放代理 | `PromptPlazaRequest` 原实现是"前端传任意 apiBase + path，Go 侧代发"的通用 HTTP 代理。广场下线后已无调用方，但留着等于给本地留一个任意 URL 转发器，与"不上传用户数据到第三方"冲突，故改为返回错误的短路桩（签名保留以不破坏 wailsjs 生成绑定） |
| 品牌串 | 窗口标题、系统托盘 tooltip、Windows AppID/Title、`app_windows/app_darwin/app_linux` 的托盘标题、飞书/钉钉测试消息、AI 报告页脚、GitHub 按钮，全部从 AI盘译 改为 AI盘译 / aipanyi |
| 外链图标 | 4 处硬编码 `raw.githubusercontent.com/ArvinLovegood/go-stock/.../appicon.png` 改为由 `GetVersionInfo().icon` 注入本地内嵌图标 |
| 配置 | `wails.json` 的 name/outputfilename/author/info 全部改为 aipanyi；`.github/workflows/main.yml` 资产名改为 `aipanyi-*`（与 `appinfo.Asset*` 常量对齐，否则自动更新找不到文件） |

**仍未处理（需你决策）**：

- CI 仍使用 `ArvinLovegood/wails-build-action@v3.9`（原作者 fork 的构建 action）。建议换成上游 `dAppServer/wails-build-action` 或改为原生 `wails build`，消除供应链信任问题；未经实机验证，故未擅自改。
- `appinfo.AssetLinuxAMD64` 已声明但 CI 没有 Linux 构建 job，Linux 用户拿不到自动更新包。
- `docs/` 下仍有若干 AI盘译 品牌字样与历史 changelog（`docs/CHANGELOG-*.md`、`AI盘译使用手册.md` 等），不影响运行，可后续统一。
- 前端 localStorage key 仍为 `AI盘译-agent-*` / `AI盘译-ai-*` 前缀（内部键，不影响使用；改动会重置用户已保存的模型选择，故保留）。
- `app.go` 里通知 payload 的 `"source": "AI盘译"` 与前端 `data.source==="AI盘译"` 的颜色判断是配对使用的内部协议值，改动需两端同步，故保留。

### 9.8 收款码图片处理

收款码原图是微信 / 支付宝官方生成的海报，顶部自带「推荐使用微信支付」「推荐使用支付宝」营销横幅，
与开源项目赞助场景不搭，已用 PIL 处理：

- **只涂顶部横幅带**（微信 0~250 行、支付宝 0~189 行，即白色卡片起始行之上），整块填回背景色后重绘文字；
- 新文字为「感谢支持」，微软雅黑 Bold（`C:\Windows\Fonts\msyhbd.ttc`），白色，水平垂直居中于横幅带；
- **二维码区域零改动**：改前改后对卡片区域（微信 251 行起 / 支付宝 190 行起）取 MD5 比对，完全一致，扫码不受影响；
- 两张图原始宽高比不同（微信 989x1060 / 支付宝 740x928），README 与「关于」页均改为按 **height** 归一化而非 width，否则两边会一高一低；
- 收款码统一收敛到 `docs/sponsor/`：README 直接引用该目录；`main.go` 通过 `//go:embed build/sponsor/*` 内嵌同一份文件（内容与 `docs/sponsor/` 保持一致，MD5 相同），供「关于」页下发。

支付宝图底部还留有「打开支付宝[扫一扫]」一行（在卡片之外），未处理；需要一并去掉的话再说。

### 9.9 CI 去第三方化与文档品牌统一

#### 9.9.1 CI 改为原生 steps

原先 `uses: ArvinLovegood/wails-build-action@v3.9`——这是**上游作者仓库的 fork**，
CI 实际执行的是别人仓库里的代码；且它自定义了 `build-statement` / `build-key` 两个非标准输入
来注入 `main.OFFICIAL_STATEMENT` / `main.BuildKey`，换成官方 action 会丢掉这两个变量。
现改为原生步骤，只用 GitHub 官方 action + runner 自带的 `gh` CLI：

| 步骤 | 用法 |
| --- | --- |
| Checkout | `actions/checkout@v4`（原为 v2，已升级） |
| Setup Go | `actions/setup-go@v5`，go-version 1.27 |
| Setup Node | `actions/setup-node@v4`，node 22.22.2 |
| Install Wails CLI | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0` |
| Build | `wails build -platform <p> -ldflags "..." -o aipanyi` |
| Package | Windows 直接取 `build/bin/*.exe`；macOS 用 `zip` 打包 `.app` |
| Upload | `gh release upload <tag> dist-out/* --clobber` |

ldflags 每个 `-X` 值都用单引号包裹：commit message / 声明文案含空格，
不包裹会被 ldflags 按空格切碎（历史上正是因此导致 macOS 构建失败）。

顺带清理：去掉 `nsis: true`（产品定位是绿色版，无安装器，`appinfo.Asset*` 也未声明安装包资产）、
去掉 `build-tags: ${{ github.ref_name }}`（把 tag 名当 Go build tag 传入，无实际作用）、
合并三个 darwin 目标为单个 `darwin/universal`（与 `appinfo.AssetDarwin` 对齐）。

> ⚠️ **未实机验证**：CI 只能在推送 tag 时触发，本地无法跑。首次发版请先用 `*-dev` tag 试一次。

#### 9.9.2 文档品牌统一

- 重命名：`go-stock使用手册.md` → `AI盘译使用手册.md`、`go-stock帮助问答手册_v2.md` → `AI盘译帮助问答手册.md`、`go-stock项目功能问答手册.docx` → `AI盘译项目功能问答手册.docx`；`main.go` 的 `//go:embed` 与 README 链接同步更新
- 18 个 md 文档中 `go-stock` → `AI盘译`；**上游仓库地址 `ArvinLovegood/go-stock` 保留**（GPLv3 署名义务）
- 旧资产名 `go-stock-*.exe/zip` → `aipanyi-*`
- `RUN_MACOS.md` 的 bundle id `com.sparkmemory.AI盘译` → `com.ryun601.aipanyi`
- `CODE_OF_CONDUCT.md` / `SECURITY.md`：两文件随后整体删除（上游模板，含原作者邮箱 `sparkmemory@163.com`，且与 GPLv3 项目无必要关联）；`CONTRIBUTING.md` 中的「行为准则」引用节一并移除
- `BUILD_LINUX.md` 的 `Maintainer: sparkmemory` → `RYun601`
- 删除已下线功能的文档段落：使用手册 11.6/11.7（广场）、技能广场小节、14.6 社区技巧节、FAQ「广场无法访问」、快速开始指南的广场条目、设置说明 4.11 广场地址节；相应位置改写为"已下线 + 原因"说明

### 9.10 VIP 门禁彻底移除（2026.09.26）

#### 9.10.1 问题

ADR §9.3 阻塞项 3 此前标记为"已移除"，但实际只处理了自选股数量上限、`ai-assistant-web` 的 `requireVip2`、
两个悬浮助手的 VIP2 拦截三处。**前端仍残留一批硬门禁**，且因为 fork 后 `BuildKey` 与上游不同，
原赞助码无法解密 → `EffectiveSponsorVipLevel()` 恒返回 `(0, false)` → **这些功能对所有人都是锁死的**。

这与 README「全部功能免费，二进制里没有任何功能锁」及商业化决策「付费点必须放服务端、免费版必须完整可用」直接冲突。

#### 9.10.2 清理清单

| 文件 | 原门禁 | 处理 |
| --- | --- | --- |
| `App.vue` | K线分析菜单 `vipLevel < 2` 直接 return | 删 `vipLevel`/`refreshEffectiveVip`/`getDiscreteMessage`，菜单直接跳转 |
| `kline-analysis.vue` | 整页 VIP 弹窗 + 60s 轮询重弹 | 删弹窗 DOM、`startVipCheck`、`refreshEffectiveVip`、相关 state |
| `allStockList.vue` | 多周期K线 / 技术面筛选拦截 | 删 `refreshEffectiveVip` 与两处判断；`handleCheckedChange` 变为 no-op |
| `stock.vue` | 多周期K线 10 秒自动关闭 | 删判断与 `klineAutoCloseTimer`；删 `GetEffectiveSponsorVip` import |
| `SelectStock.vue` / `FundRanking.vue` / `TradingRecordManager.vue` | K线图拦截 | 改为直接打开 |
| `SignalMonitorPanel.vue` + `kline/signalMonitor.ts` | 打开面板 / 总开关 / 手动扫描 / 注册监听四处校验 | 全部删除，`togglePanel`/`onToggleMonitor` 改回同步函数 |
| `settings.vue` | 赞助码输入区 + 保存前校验 | 删表单项、`CheckSponsorCode` 调用、表单字段与样式 |
| `about.vue` | VIP 徽章 + 到期时间 | 删除；`GetSponsorInfo` 调用移除 |
| `aiRecommendStocksList.vue` | 5 处渲染降级 + `showDetail` 硬拦截 + tooltip 开关 | 降级分支全部删除（恢复增强渲染），`showDetail` 放开 |
| `FloatingAgentAssistant.vue` / `FloatingAiAssistant.vue` / `agent-chat.vue` / `agent-chat_bk.vue` | `ensureVipInfo` 死代码 + `GetSponsorInfo`/`GetEffectiveSponsorVip` import | 全部清除 |

**后端保留未动**：`App.GetSponsorInfo` / `GetEffectiveSponsorVip` / `CheckSponsorCode` / `isVip`、
`data.sponsor_vip.go`、`Settings.SponsorCode` 字段。前端已无任何调用方，属于不可达死代码而非功能锁；
`isVip` 仍被 `CheckUpdate` 用于选择下载 URL（CDN vs 直连），删掉会牵连更新链路，故保留。

**同步修正的文档**：`后台买卖点信号监控.md`、`交易日志与每日操作计划AI实战指南.md`、
`小白设置页面使用说明.md`、`快速开始指南.md`、`AI盘译帮助问答手册.md`、`AI盘译使用手册.md`
中所有 VIP2 / 赞助码 / 权限不足描述；`8月新功能介绍` 与 changelog 表格属历史记录，加注说明而非改写。

### 9.4 验证记录（补充）

- `go build` / `go build -tags web` / `go vet`（两种模式）全部通过；
- `webserver` 包测试两种模式全部通过；
- `backend/data` 包测试存在 DB 连接失败，**经 `git worktree` 对比验证为既有环境问题**（未改动代码同样失败），与本次改造无关。
