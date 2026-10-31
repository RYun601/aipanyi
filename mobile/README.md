# Mobile（Android 价格提醒 App）

在现有桌面前端/后端之外，新增一个**独立 Android App**：

- 自选股（关注、成本、持仓、现价/盈亏）
- 个股 AI 分析（流式，自动带入成本/持仓）
- AI 配置、系统提示词
- 价格提醒（手动 + AI 建议确认；后台 30 秒轮询；本地通知）

详细设计与里程碑见 [`docs/plan/2026-10-29-android-price-alert-mvp.md`](../docs/plan/2026-10-29-android-price-alert-mvp.md)。

## 目录

```
mobile/
├─ alerts/          纯逻辑：提醒状态机 + AI 建议解析（可本地单测）
├─ core/            Go 核心：SQLite、自选、AI、提醒引擎、MobileApp 门面
├─ webui/           前端产物 embed（build tag: mobileassets）
├─ frontend/        Vue 3 + Vant 移动端页面
├─ android/         Kotlin 壳（WebView + 前台服务 + 通知）
├─ devserver/       桌面调试：go run 起本地服务
└─ scripts/         构建 AAR / APK
```

## 运行方式

```
Android WebView
   └─ http://127.0.0.1:<随机端口>/  ← 复用 backend/webserver（token 注入）
        └─ /api/call → MobileApp（反射派发）
        └─ /ws       → 分析流 / 提醒事件
             └─ Go 核心：backend/data + backend/db + 提醒引擎
```

Go 核心**不编译**桌面 `main.App`（Wails/托盘/更新器），而是 `mobile/core` 里的独立门面。

## 本地调试（不需要 Android）

```powershell
cd mobile/frontend
npm install
npm run build

cd ../..
go run -tags "web mobileassets devserver" ./mobile/devserver
# 打开输出的 http://127.0.0.1:<port>/
```

数据写入 `tmp/mobile-dev-data`。也可以用 `-tags web` 跑占位页（只验证 Go 接口）。

## 测试

```powershell
# 纯逻辑状态机 / AI 建议解析（不依赖网络/数据库）
go test ./mobile/alerts

# 编译
go build -tags web ./mobile/...
go build -tags "web mobileassets" ./mobile/...
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -tags web ./mobile/...
```

> Windows Defender 偶尔会把 `mobile/core` 的测试二进制误判为病毒（它链接了
> chromedp/eino 等完整数据依赖）。纯逻辑测试放在 `mobile/alerts`，可正常执行；
> `mobile/core` 用 `go build`/`go vet` + 真机联调验证。

## 构建 APK

见 [`android/README.md`](android/README.md)。核心命令：

```powershell
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1
```

## 当前实现状态

- [x] `mobile/alerts`：above/below 状态机、unknown 初始化、AI 建议解析/校验 + 单测
- [x] `mobile/core`：SQLite、股票基础表导入、自选、AI 配置/提示词、AI 流、提醒 CRUD、提醒引擎、HTTP 门面
- [x] `mobile/frontend`：自选、个股、提醒、设置四个页面，Vite 构建通过
- [x] `mobile/android`：WebView、前台服务、通知渠道、开机广播、原生设置桥
- [x] gomobile AAR（12.1 MB，arm64-v8a，`-s -w`）
- [x] Debug APK（26.7 MiB，`app/build/outputs/apk/debug/app-debug.apk`）
- [ ] 真机验收（Honor 50 / MagicOS 8）

## 已知限制

- 价格提醒是分钟级（默认 30 秒轮询），不是逐笔条件单。
- 用户强行停止 App 后，本地服务/通知无法自动恢复，这是 Android 限制。
- MagicOS 需要手动允许自启动/后台活动；仍可能被清理。
- 手机端使用独立 SQLite；桌面配置迁移尚未实现（第二阶段导出/导入）。
