# 参与 AI盘译 开发

感谢你愿意为 [AI盘译](https://github.com/RYun601/aipanyi)（aipanyi）贡献代码、文档或想法。

本项目是 [go-stock](https://github.com/ArvinLovegood/go-stock)（GNU GPLv3）的衍生版本。
在动手之前，请先读一遍 [`AGENTS.md`](AGENTS.md)——那里有构建命令、验证清单、代码约定和发版纪律，
**其中"二进制内不得做功能锁""不得把用户数据上传到第三方服务器"两条是硬约束**，PR 若违反会被打回。

> 面向 AI 编码助手的说明：本文件与 `AGENTS.md` 分工是——
> `CONTRIBUTING.md` 面向人类贡献者讲"怎么参与"，`AGENTS.md` 面向所有协作者（含 AI）讲"这个项目怎么构建、怎么发版、有哪些约定"。
> 两者对分支与提交的约定一致，以 `AGENTS.md` §3 为准。

---

## 贡献类型

### 报告问题

在 [GitHub Issues](https://github.com/RYun601/aipanyi/issues) 提交。请尽量提供：

- **问题描述**：清晰地描述你遇到的问题或建议的功能
- **重现步骤**：如果是 bug，提供可复现的具体步骤
- **环境信息**：操作系统、软件版本（「关于」页可查看）
- **日志**：如有可能，附上相关日志或错误信息

涉及行情数据异常的 issue，请同时说明**股票代码 + 市场（A股/港股/美股/场内基金）+ 复现时间**，
不同数据源的延迟与字段口径差异很大，缺了这三项很难定位。

### 提交代码

#### 1. Fork 并克隆

```bash
git clone https://github.com/RYun601/aipanyi.git
cd aipanyi
git remote add upstream https://github.com/RYun601/aipanyi.git
```

#### 2. 创建分支

```bash
git checkout -b feat/xxx     # 新功能
git checkout -b fix/xxx      # 缺陷修复
git checkout -b docs/xxx     # 文档
git checkout -b refactor/xxx # 重构
```

分支名用描述性名称，例如 `fix/kline-resize`、`feat/agent-tool-xxx`。

#### 3. 编写代码

遵循项目的代码约定（详见 `AGENTS.md` §5）。几条最容易踩的：

- **不要用 `window.onresize = ...`**——全局唯一槽位，会覆盖别的组件且不注销。用 `addEventListener` + `onBeforeUnmount` 里 `removeEventListener`
- **图表必须做容器自适应**——用 `frontend/src/composables/useChartAutoResize.js`
- **仓库坐标统一走 `backend/appinfo`**——不要在业务代码里硬编码 GitHub 地址或 IP
- **事件用 `backend/events` 的 `events.Emit`**——不要直接用 `runtime.EventsEmit`，Web 模式会崩
- **前端调后端一律走 `frontend/wailsjs/go/main/App.js`**——不要直接 `fetch` 远程接口

#### 4. 验证

提交前必须全部通过（详细命令见 `AGENTS.md` §2）：

```bash
go build ./... && go build -tags web ./...
go vet ./...   && go vet -tags web ./...
go test ./webserver/...
cd frontend && npm run build
```

**两种构建模式都要验**。另外注意：

- `gofmt -l` 报全仓库需格式化是既有状态（仓库为 CRLF 行尾），**不要**顺手全量格式化，否则会产生巨大的无关 diff
- `backend/data` 包测试报 DB 连接失败是既有环境问题，不要试图"修复"

#### 5. 提交

```bash
git add <具体文件>
git commit -m "fix: 修复多周期K线在窄屏下被裁切的问题"
```

提交信息用中文或英文均可，但**必须说清"为什么"**。一个提交只做一件事。

#### 6. 同步并推送

```bash
git fetch upstream
git rebase upstream/main
git push origin feat/xxx
```

#### 7. 创建 Pull Request

PR 标题遵循 `<type>: <简述>`，type 取 `feat` / `fix` / `docs` / `refactor` / `perf` / `test` / `chore`。

PR 描述请写清：

- **改了什么**
- **为什么**（关联的 issue 或背景）
- **影响面**（涉及哪些页面 / 接口 / 数据）
- **验证方式**（跑了哪些命令、手工验证了什么）
- 涉及 UI 的**附截图**，特别是窄屏效果

`main` 是受保护分支，**禁止直接 push**，所有改动必须通过 PR 合入。

### 改进文档

文档和代码同样重要。发现文档有错、过时或说不清楚，直接提 PR。

需要特别注意：**功能下线或行为变更时，必须同步更新 `docs/` 下对应的说明文档**，
不要只改代码留下过期文档——这个仓库里已经因为这类问题返工过好几次。

---

## 代码风格

- **Go**：`gofmt` 基础风格即可，不引入额外的 linter 强制；导出函数写注释，注释说"为什么"而不是"是什么"
- **Vue/JS**：与所在文件既有风格保持一致，不做全仓格式化
- **命名**：有意义的变量名 / 函数名；布尔量用 `isXxx` / `hasXxx` / `enableXxx`
- **注释**：解释意图、约束、踩过的坑，不要复述代码

## 用户可见文案

- 统一用「**AI盘译**」，不要出现 `go-stock`
- 上游仓库地址 `ArvinLovegood/go-stock`、原作者署名**必须保留**（GPLv3 义务）
- 面向用户的提示语写清楚"发生了什么、该怎么办"，不要只报错误码

## 许可证

通过贡献代码，你同意你的贡献将根据项目的 [GNU GPLv3 许可证](LICENSE) 进行分发。

也就是说：任何人可以自由使用、研究、修改、再分发本项目（包括你的贡献），
但衍生版本必须同样以 GPLv3 开源并保留版权声明与修改标注。**请不要提交你无权以 GPLv3 授权的代码。**

---

再次感谢你的贡献。有问题直接在 issue 里提。
