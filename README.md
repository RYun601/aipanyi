<div align="center">

# AI盘译 (aipanyi)

**基于大语言模型的本地股票分析工具**

Go + Wails + Vue 3 + Naive UI ｜ 本地优先 · 本地存储 · 桌面原生窗口

[![GitHub Release](https://img.shields.io/github/v/release/RYun601/aipanyi?link=https%3A%2F%2Fgithub.com%2FRYun601%2Faipanyi%2Freleases)](https://github.com/RYun601/aipanyi/releases)
[![GitHub Repo stars](https://img.shields.io/github/stars/RYun601/aipanyi?style=social)](https://github.com/RYun601/aipanyi)
[![License](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)

</div>

---

## 项目简介

AI盘译是一款本地运行的股票分析工具。该工具整合行情、K 线、资金流、龙虎榜、政策新闻等数据，并接入可调用 120+ 数据工具的 AI 智能体。用户以自然语言提出分析需求，模型据此选择数据接口，结合图表与指标完成分析，最终输出包含图表的分析报告。

三条设计原则：

1. **本地优先**：行情缓存、AI 配置、提示词模板与知识库均存储于本机 SQLite 或本地文件，不进行云端同步；AI 请求仅发送至用户配置的模型服务。
2. **桌面原生窗口**：基于 Wails 构建，启动后直接打开桌面窗口，不依赖系统浏览器，窗口支持自由缩放。
3. **功能完整**：所有功能均可在本地使用。

> 本项目为 [go-stock](https://github.com/ArvinLovegood/go-stock)（GNU GPLv3）的衍生版本，依据 GPLv3 要求公开全部修改。详见 [许可证](#许可证) 与 [致谢](#致谢)。

---

## 功能概览

**AI 分析**

- 内置 AI 智能体，支持 React / PlanExecute / DeepAgents 三种模式。
- 120+ AI 数据工具，覆盖行情、K 线、分时、分笔、资金流、龙虎榜、概念板块、财务报告、政策新闻、节假日及问财自然语言选股等。
- 桌面端 AI 分析支持可选的持仓上下文：默认关闭；开启后仅传入所选自选股的成本与组合，当前股票未选中时不提供其成本。
- 视觉理解：支持截图、K 线图、图片上传、粘贴及 URL 输入。
- 多轮对话、对话摘要、长期记忆、用户画像学习及知识库（chromem-go 向量库）。
- 技能系统（Skill）：支持 ZIP 技能包导入、在线编辑与渐进式提示词注入。
- MCP 扩展：支持 MCP 服务器管理、OAuth 2.1 授权，以及 DeepAgents 模式下的工具按需检索。

**行情与图表**

- 支持 A 股、港股、美股及场内 ETF。
- K 线分析：复权切换、波浪理论、测量画框、13 种绘图工具、成交量分布 VPVR、神奇九转、自动背离（RSI/MACD/KDJ）、BBI、涨跌停价位线、威斯波浪。
- 分时、分笔成交、集合竞价、个股资金流向及通达信 MAC 协议。
- 基于 lightweight-charts 的多周期 K 线图与 ECharts 指标图，窗口缩放时自动重排。

**监控与推送**

- 后台买卖点信号监控（9 路共振 / TEMA 转折），窗口最小化时持续运行，支持语音播报股票名称。
- 信号流水与收益统计（买点开仓、卖点平仓，支持多区间）。
- 飞书 / 钉钉机器人推送，飞书 AI 运行进度卡片。
- 定时预警、异动监控、涨停梯队。

**复盘策略**

- 每日复盘与盘前策略（AI 生成，交易日定时执行）。
- 提示词模板回测：历史行情重放、避免未来数据，支持胜率、回撤、夏普比率、Jaccard 稳定性等多维对比。
- 推荐回测统计：按持有期核算实际收益，并与沪深 300 指数对比。
- 交易日志、每日操作计划。

**AI 模型服务**

支持 OpenAI 兼容接口、Ollama、LMStudio、AnythingLLM、DeepSeek、智谱、豆包、阿里百炼、Moonshot、混元、讯飞星火、MiniMax、小米 MiMo、硅基流动等平台。

---

## Android 客户端（实验性）

仓库内包含独立的 Android 客户端（`mobile/`），面向移动端轻量使用场景：

- 支持自选、成本/持仓、AI 分析（可选持仓上下文，默认关闭，开启后逐只选择）、AI 配置与系统提示词、价格触发提醒（30 秒轮询）。
- 数据存储于手机本地 SQLite，不使用云端存储；AI 请求仅发送至用户配置的模型服务。
- 当前提供 debug 签名、arm64-v8a 安装包，随 Release 发布：`aipanyi-android-0.1.0-debug.apk`。桌面三平台由 CI 自动构建，Android APK 当前由本地脚本构建后手动上传。
- 构建与 Honor/MagicOS 配置见 [`mobile/android/README.md`](mobile/android/README.md)。
- 设计与真机验收记录见 [`docs/plan/2026-10-29-android-price-alert-mvp.md`](docs/plan/2026-10-29-android-price-alert-mvp.md)。

> Android 客户端不包含桌面端的 K 线、回测、MCP 等功能。Honor/MagicOS 需允许自启动与后台活动，并关闭电池优化；应用已从最近任务列表隐藏，以降低被系统清理的概率；设备重启后需重新打开应用以恢复监控。

---

## 快速开始

### 下载与安装

请前往 [Releases](https://github.com/RYun601/aipanyi/releases) 下载对应平台的文件：

| 平台 | 文件 |
| --- | --- |
| Windows x64 | `aipanyi-windows-amd64.exe` |
| Windows ARM64 | `aipanyi-windows-arm64.exe` |
| macOS（Intel 与 Apple Silicon 通用） | `aipanyi-darwin-universal.zip` |
| Android arm64-v8a | `aipanyi-android-0.1.0-debug.apk`（debug 签名，手动安装） |

Windows 提供单个可执行文件，下载后可直接运行，无需安装程序，不写入注册表。macOS 提供 zip 压缩包，解压后得到 `.app`。Linux 版本当前不在 CI 发布范围内，需自行构建，参见 [`docs/BUILD_LINUX.md`](docs/BUILD_LINUX.md)。

Android APK 为 debug 签名、arm64-v8a，手动安装前需在系统设置中允许安装未知来源应用。

首次启动：

1. 启动应用，打开桌面窗口（窗口大小自适应屏幕分辨率）。
2. 在「设置」中配置 AI 服务 API Key；如使用 Ollama 本地模型，可不填写外部服务 Key。
3. 输入股票代码或名称，开始分析。

首次启动会自动初始化 A 股、港股与美股基础数据；网络较慢时请耐心等待，避免重复启动程序。

> **Windows 使用说明**：建议新建独立目录（例如 `D:\AI盘译`），将可执行文件放入后从该目录启动。
> 程序数据（`data\stock.db`、日志及缓存）生成于**启动时的工作目录**（通常为可执行文件所在目录）。
> 更换工作目录会创建新的数据目录，原有自选股与设置不会自动迁移。升级时可替换可执行文件，数据目录保持不变。

> **首次运行安全说明**：本项目未使用代码签名证书，首次运行可能触发以下提示：
>
> - Windows SmartScreen 提示「Windows 已保护你的电脑」→ 选择「更多信息」→「仍要运行」。
> - 部分安全软件可能误报 → 将程序加入信任列表；未签名程序较易被误判。
> - macOS 提示「无法验证开发者」→ 处理办法见 [RUN_MACOS_TERMINAL.md](docs/RUN_MACOS_TERMINAL.md)。
>
> 如需校验文件完整性，请在对应版本的 Release 中下载 `SHA256SUMS.txt`，计算本地文件的 SHA-256 后比对：
>
> ```powershell
> Get-FileHash .\aipanyi-windows-amd64.exe -Algorithm SHA256
> ```
>
> 请仅从本仓库 Releases 页面下载；第三方站点转发的文件无法保证安全。

### 源码构建

环境要求：Go 1.27+、Node.js 22+、Wails CLI v2.15.0。

```bash
git clone https://github.com/RYun601/aipanyi.git
cd aipanyi

# 0. 安装 Wails CLI（一次性）
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0

# 1. 构建前端（wails build 也会自动执行）
cd frontend && npm install && npm run build && cd ..

# 2. 桌面构建（Wails 原生窗口，唯一 UI 形态）
wails build -platform windows/amd64 -o aipanyi
```

> `-o aipanyi` 不会自动添加 `.exe` 扩展名；Windows 产物为 `build/bin/aipanyi`。如需 `.exe`，可执行 `Copy-Item build/bin/aipanyi build/bin/aipanyi.exe`。
> 请勿使用 `go build` 替代 `wails build`：后者才能打包前端产物并提供 Wails 运行时。

> 数据目录 `data/` 相对于**启动时的工作目录**。从仓库根目录启动时读取 `F:\aipanyi\data`；直接运行 `build/bin/aipanyi` 时使用 `build/bin/data`，不会自动读取仓库根目录的数据。迁移或升级时请一并保留 `data/` 目录。

桌面端使用 Wails 原生窗口；浏览器网页版模式已移除，详见 [ADR-002](docs/ADR-002-移除桌面网页版模式.md)。`webserver` 包仅保留给 Android 客户端本地界面使用。

更多说明见 [`docs/快速开始指南.md`](docs/快速开始指南.md) 与 [`docs/BUILD_LINUX.md`](docs/BUILD_LINUX.md)。

---

## 许可与赞助

全部功能免费；赞助不附带任何功能。

如本项目对你有帮助，欢迎自愿赞助：

<div align="center">

  <img src="./docs/sponsor/wechat.png" alt="微信收款码" width="280" height="420" />&emsp;&emsp;<img src="./docs/sponsor/alipay.jpg" alt="支付宝收款码" width="280" height="420" />

</div>

---

## 数据来源与免责声明

- 行情与新闻数据来自东方财富、新浪财经、财联社、问财、通达信等公开接口。
- 上述接口通常禁止商业用途的批量抓取；本项目仅用于个人学习研究，并披露数据来源。
- 数据可能存在延迟、缺失或错误；AI 生成内容可能存在错误或过时。相关内容均不构成投资建议，投资决策及风险由用户自行承担。
- 本项目仅供学习研究使用。

---

## 文档

- [小白入门使用手册（零基础）](docs/小白入门使用手册.md)
- [小白设置页面使用说明](docs/小白设置页面使用说明.md)
- [快速开始指南](docs/快速开始指南.md)
- [AI盘译 使用手册（完整功能）](docs/AI盘译使用手册.md)
- [后台买卖点信号监控](docs/后台买卖点信号监控.md)
- [知识库与长期记忆功能说明](docs/知识库与长期记忆功能说明.md)
- [AI对话与视觉理解功能说明](docs/AI对话与视觉理解功能说明.md)
- [技能管理功能说明](docs/技能管理功能说明.md)
- [架构决策记录 ADR-001（已由 ADR-002 取代）](docs/ADR-001-UI浏览器化与AI边车架构决策.md)
- [架构决策记录 ADR-002：移除桌面网页版模式](docs/ADR-002-移除桌面网页版模式.md)

---

## 面向 AI 助手与贡献者

本仓库包含 [`AGENTS.md`](AGENTS.md)，其中规定构建与验证命令、分支与提交规范、发版纪律（修改说明必须手写，禁止使用 GitHub 自动生成 release notes）及代码约定。AI 编码助手在修改代码前应先阅读该文件。

---

## 致谢

本项目基于以下开源项目构建：

- [go-stock](https://github.com/ArvinLovegood/go-stock) —— 上游项目，本项目的起点
- [Wails](https://wails.io/) / [Vue](https://vuejs.org/) / [Vite](https://vitejs.org/) / [NaiveUI](https://www.naiveui.com/)
- [ECharts](https://echarts.apache.org/) / [lightweight-charts](https://tradingview.github.io/lightweight-charts/) / [md-editor-v3](https://github.com/imzbf/md-editor-v3)
- [chromem-go](https://github.com/philippgille/chromem-go) / [eino](https://github.com/cloudwego/eino)
- [Tushare](https://tushare.pro/)

感谢上游作者 [ArvinLovegood](https://github.com/ArvinLovegood) 与所有贡献者。

---

## Star 趋势

<a href="https://www.star-history.com/?repos=RYun601%2Faipanyi&type=date&legend=top-left">
  <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=RYun601/aipanyi&type=date&legend=top-left" />
</a>

---

## 许可证

[GNU GPLv3](LICENSE)

本项目为 go-stock 的衍生作品，依据 GNU General Public License v3.0 发布。你可以自由使用、研究、修改和再分发本项目的全部源码（包括本衍生版本的全部修改），但需同样以 GPLv3 授权你的衍生版本，并保留原始版权声明与修改标注。
