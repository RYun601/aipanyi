# AGENTS.md

本文件面向在本仓库中工作的 **AI 编码助手 / 自动化 Agent**，以及所有贡献者。
目标是让任何人（或任何模型）在动手前就知道本项目的构建方式、代码约定与发版纪律。

---

## 1. 项目概况

| 项 | 值 |
| --- | --- |
| 产品名 | **AI盘译**（aipanyi） |
| 上游 | [go-stock](https://github.com/ArvinLovegood/go-stock)，GNU GPLv3 衍生项目 |
| 技术栈 | Go 1.27 + Wails v2 + Vue 3 + Naive UI + Vite |
| 形态 | 本地桌面应用；UI 走「本地 HTTP 服务 + 默认浏览器」（`-tags web`）或 Wails 窗口（默认） |
| 模块路径 | `aipanyi`（**不要**改回 `go-stock`） |
| 数据存储 | 本地 SQLite + 本地文件，**不上云** |

关键架构决策见 [`docs/ADR-001-UI浏览器化与AI边车架构决策.md`](docs/ADR-001-UI浏览器化与AI边车架构决策.md)，
改动涉及 UI 形态、边车、数据流、商业化边界时**必须先读该文档**。

---

> 本文件面向所有协作者（含 AI 编码助手）。面向人类贡献者的参与指南见 [`CONTRIBUTING.md`](CONTRIBUTING.md)，
> 两者对分支与提交的约定一致，以本文件 §3 为准。

## 2. 构建与验证

```bash
# 前端（两种模式共用，改任何 .vue/.js 后都要跑）
cd frontend && npm install && npm run build && cd ..

# 桌面模式（默认构建）—— 必须用 wails，不能用 go build
# （go build 不会打包前端产物，也拿不到 Wails 运行时）
wails build -platform windows/amd64 -o aipanyi

# Web 模式（本地 HTTP 服务 + 默认浏览器）
wails build -tags web -platform windows/amd64 -o aipanyi-web

# 验证（提交前必须全部通过）
go build ./... && go build -tags web ./...
go vet ./...   && go vet -tags web ./...
go test ./webserver/...
cd frontend && npm run build
```

> ⚠️ **`-o aipanyi` 不会自动补 `.exe`**。wails 只在未传 `-o` 走自动命名时才补后缀，
> 所以产物是 `build/bin/aipanyi`（无扩展名）。CI 里已按此处理，本地自用可自行
> `Copy-Item build/bin/aipanyi build/bin/aipanyi.exe`。

**约定**：
- 构建标签：**默认 = 桌面模式（Wails）**，`-tags web` = Web 模式，`-tags custom` = 定制版。
- 任何改动都要保证**两种模式都能编译**，不能只验一种。
- 分发形态是**绿色版（便携版）**：只发单个 exe / zip，**没有安装器**，不要引入 NSIS 安装包
  （`build/windows/installer/` 脚手架已删除，理由见 ADR-001 §9.9.1 与 README「下载即用」）。
- `gofmt -l` 报全仓库需格式化是**既有状态**（仓库为 CRLF 行尾），**不要**顺手全量格式化，
  否则会产生巨大的无关 diff。只保证自己新增/修改的文件风格一致。
- `backend/data` 包测试报 DB 连接失败是**既有环境问题**，与改动无关，不要试图"修复"。

### 2.1 双模式前端桥接层（改动前必读）

前端要在**桌面（Wails）**和**浏览器（`-tags web`）**两种形态下跑同一份代码，
全部差异收敛在 `frontend/wailsjs/bridge.js` 一个文件里。改前端运行时相关代码前，
必须先理解这条边界，否则很容易把 Web 模式改坏：

| 文件 | 谁生成 | 能不能手改 |
|---|---|---|
| `frontend/wailsjs/bridge.js` | 手工维护 | ✅ **唯一**该放双模式逻辑的地方 |
| `frontend/wailsjs/go/main/App.js` | wails 生成 → `scripts/gen-bindings.js` 改写 | ❌ 不要手改，改生成脚本 |
| `frontend/wailsjs/runtime/runtime.js` | wails 原生生成 | ❌ **严禁手改** |
| `frontend/wailsjs/runtime/*.d.ts`、`package.json` | wails 原生生成 | ❌ 不要手改 |

**为什么**：`wails build` 每次都会重新生成 `runtime/runtime.js`（没有任何开关可以跳过），
也会重新生成 `go/main/App.js`（除非加 `-skipbindings`）。历史上双模式适配是手写在这两个
文件里的，结果每次构建都会把 Web 模式静默改坏。现已全部迁入 `bridge.js`。

**当前流程是自愈的**：
- `npm run build` 会先跑 `scripts/gen-bindings.js`，把 wails 刚生成的原生 `App.js`
  自动改写为走 `call()` 的桥接版（幂等，重复执行无副作用）；
- 该脚本带失配断言：若 wails 改了生成格式导致正则失配，会以非零码退出、
  让构建失败，而不会安静地产出一个 Web 模式不可用的包。

**因此**：
- 新增后端绑定方法后**不需要**手动同步 `App.js`，`wails build` 会自动带上；
- 不要给构建命令加 `-skipbindings`，那反而会让绑定过期；
- 前端 import 一律从 `@/wailsjs/bridge` 取，**不要**再从 `wailsjs/runtime` 取。

---

## 3. 分支与提交规范

### 3.1 分支模型

- **`main` 是受保护分支，禁止直接 `git push` 到 main。**
- 该约束由 GitHub ruleset（Settings → Rules → Rulesets → `main`）**实际强制执行**，
  不只是文档约定：直推会收到 `GH013: Repository rule violations`。
  ruleset 配置：Bypass list 为空（无人可绕过，含仓库所有者）、
  `Require a pull request before merging` 已开启、`Required approvals = 0`
  （单人开发下自己无法批准自己的 PR，0 是唯一可行值）。
- 所有代码变更必须通过 **Pull Request** 合入 `main`。
- 日常开发在功能分支上进行，例如：
  ```bash
  git checkout -b feat/xxx        # 新功能
  git checkout -b fix/xxx         # 缺陷修复
  git checkout -b docs/xxx        # 文档
  git checkout -b refactor/xxx    # 重构
  ```
- PR 标题遵循 `<type>: <简述>`，type 取 `feat` / `fix` / `docs` / `refactor` / `perf` / `test` / `chore`。
- PR 需说明：改了什么、为什么、影响面、验证方式。涉及 UI 的附截图。
- 合入后删除功能分支。

### 3.2 提交信息

- 使用中文或英文均可，但**必须能说清"为什么"**，不要只写"修复bug"。
- 一个提交只做一件事；不要把无关改动混在一起。
- 不要提交：`frontend/dist/`、`node_modules/`、本地数据库、临时脚本、个人配置文件。

---

## 4. 发版规范（重要）

### 4.1 发版流程

1. 确认 `main` 上所有改动已通过 PR 合入且验证全绿。
2. 按语义打 tag，**tag 必须以渠道后缀结尾**（CI 依此后缀判定发布渠道）：

   | Tag 形式 | 渠道 | 是否预发布 |
   | --- | --- | --- |
   | `v1.2.3-release` | Release 稳定版 | 否 |
   | `v1.2.3-pre` | Pre-release 预发布版 | 是 |
   | `v1.2.3-dev` | Dev 开发版 | 是 |

3. 推送 tag：`git push origin v1.2.3-release`
4. CI（`.github/workflows/main.yml`）自动构建三个产物（Windows x64、Windows ARM64、macOS universal，均为绿色版/便携版）并上传到该 tag 的 Release。
5. 发布后：把 CI 产物下载回来计算 SHA256，写成 `SHA256SUMS.txt` 上传到同一 Release，并把校验值补进 Release 正文
   （程序没有代码签名证书，用户靠校验值判断文件是否被篡改，见 README「首次运行的安全提示」）。

### 4.2 修改说明必须手写，禁用 GitHub 自动生成

> **每次发版都必须人工撰写 Release 的修改说明（changelog / release notes）。**
> **严禁**使用 GitHub Release 页面那个「Auto-generate release notes」按钮，
> **严禁**直接沿用 GitHub 自动生成的 commit 列表。

原因：
- 自动生成的内容是 commit 流水，混入大量 `chore:` / 格式化 / 合并提交，用户读不出重点；
- 本项目的用户是股民不是开发者，需要的是"这次更新我能用到什么"，不是开发过程；
- 修复类改动如果不写清楚"之前是什么症状"，用户根本不知道自己遇到的问题已经解决。

**修改说明格式要求**：

```markdown
## ✨ 新增
- <功能名>：<一句话说明解决什么问题>

## 🐛 修复
- < symptom（用户可感知的症状）> → <现在的行为>

## ⚡ 优化
- <改了什么，用户能感受到什么变化>

## ⚠️ 注意事项 / 不兼容变更
- <如有则写，没有就删掉这一节>
```

写作要求：
- **面向用户写，不面向 git 写**。禁止出现"重构了 xxx 模块的内部实现"这类用户无感的描述。
- 每条说清**可感知的变化**：修了什么症状、新功能怎么用、哪个入口。
- 涉及数据/配置不兼容时，必须在「注意事项」里写清楚迁移方式。
- 历史版本归档到 `docs/CHANGELOG-<年月>.md`，不要只留在 GitHub Release 页面。

---

## 5. 代码约定

### 5.1 前端

- 组件放在 `frontend/src/components/`，一个功能一个文件。
- **禁止使用 `window.onresize = ...` 这类全局赋值**——它是唯一槽位，会覆盖其它组件的 resize 处理，
  且组件销毁时不注销。一律用 `addEventListener` + `onBeforeUnmount` 里 `removeEventListener`。
- 图表（ECharts / lightweight-charts）必须用 `frontend/src/composables/useChartAutoResize.js`
  做容器自适应；容器通过 `getElementById` 动态取的用 `attachChartResize(el, chart)`。
- 响应式统一走 `frontend/src/composables/useResponsive.js`（断点）与 `responsive.css`（全局兜底）。
- 调后端方法一律走 `frontend/wailsjs/go/main/App.js`（双模式桥接层），**不要**在前端直接 `fetch` 远程接口。
- 收款码、图标等静态资源通过 Go 侧 `//go:embed` 进二进制，再由 `GetVersionInfo()` 下发；
  不要在前端硬编码外链图片地址。

### 5.2 后端

- 仓库坐标、更新源 URL 统一走 `backend/appinfo` 包，**不要**在业务代码里硬编码 GitHub 地址或 IP。
- 给前端发事件用 `backend/events` 包的 `events.Emit`，**不要**直接用 `runtime.EventsEmit`
  （Web 模式下会崩）。
- 新增后端方法若要给前端调用，需要同步重新生成 `frontend/wailsjs/` 绑定。

### 5.3 品牌与合规

- 用户可见文案统一用「**AI盘译**」，不要出现 `go-stock`。
- 上游仓库地址 `ArvinLovegood/go-stock`、原作者署名**必须保留**（GPLv3 义务）。
- **不得在二进制内做功能锁**。付费能力只能放服务端；本地功能必须完整可用。
- **不得把用户数据上传到第三方服务器**。任何"上传/分享/同步到云端"的功能，
  要么自建服务端，要么不做——不要指向第三方域名。

---

## 6. 已知待办

- `backend/appinfo.Owner` 已配为 `RYun601`；若更换 GitHub 账号需同步修改。
- CI 使用 `gh release upload` 上传产物，首次发版建议先用 `*-dev` tag 试跑。
- `appinfo.AssetLinuxAMD64` 已声明但 CI 无 Linux 构建 job。