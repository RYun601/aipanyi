<div align="center">

# AI盘译 (aipanyi)

**基于大语言模型的 AI 赋能股票分析工具**

Go + Wails + Vue3/NaiveUI ｜ 本地优先 · 数据不出本机 · 浏览器即界面

[![GitHub Release](https://img.shields.io/github/v/release/RYun601/aipanyi?link=https%3A%2F%2Fgithub.com%2FRYun601%2Faipanyi%2Freleases)](https://github.com/RYun601/aipanyi/releases)
[![GitHub Repo stars](https://img.shields.io/github/stars/RYun601/aipanyi?style=social)](https://github.com/RYun601/aipanyi)
[![License](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)

</div>

---

## 📖 这个项目是什么

AI盘译是一个跑在你自己电脑上的股票分析工具。它把行情、K 线、资金流、龙虎榜、政策新闻等数据，
和一个可调用 150+ 数据工具的 AI 智能体接在一起——你用大白话提问，它自己决定调哪些接口、看图、算指标，
最后给你一份带图的分析。

三条设计底线：

1. **本地优先**。所有行情缓存、AI 配置、提示词模板、知识库都存在你本机的 SQLite / 本地文件里，不上云。
2. **界面就是浏览器**。启动后自动打开系统默认浏览器访问本地服务，不再弹一个桌面窗口——小屏、分屏、多显示器都不会被裁切。
3. **功能不设锁**。见下文「关于收费」。

> 本项目是 [AI盘译](https://github.com/ArvinLovegood/go-stock)（GNU GPLv3）的衍生版本，
> 依据 GPLv3 义务开源全部修改。详见 [License](#-license) 与 [致谢](#-致谢)。

---

## ✨ 功能一览

**AI 分析**
- 内置 AI 智能体，支持 React / PlanExecute / DeepAgents 三种模式
- 150+ AI 数据工具：行情、K 线、分时、分笔、资金流、龙虎榜、概念板块、财务报告、政策新闻、节假日、问财自然语言选股……
- 视觉理解：截图问股、K 线图分析、图片上传/粘贴/URL 多方式输入
- 多轮对话、对话摘要、长期记忆、用户画像学习、知识库（chromem-go 向量库）
- 技能系统（Skill）：ZIP 技能包导入、在线编辑、渐进式提示词注入
- MCP 扩展：MCP 服务器管理、OAuth 2.1 授权、DeepAgents 模式下工具按需检索

**行情与图表**
- A股 / 港股 / 美股 / 场内 ETF
- K 线分析：复权切换、波浪理论、测量画框、13 种绘图工具、成交量分布 VPVR、神奇九转、自动背离（RSI/MACD/KDJ）、BBI、涨跌停价位线、威斯波浪
- 分时、分笔成交、集合竞价、个股资金流向、通达信 MAC 协议
-  lightweight-charts 多周期K线 + ECharts 指标图，窗口缩放自动重排

**监控与推送**
- 后台买卖点信号监控（9 路共振 / TEMA 转折），最小化也持续提醒，支持语音播报股票名称
- 信号流水、收益统计（买点开仓/卖点平仓，多区间）
- 飞书 / 钉钉机器人推送，飞书 AI 运行进度卡片
- 定时预警、异动监控、涨停梯队

**复盘策略**
- 每日复盘、盘前策略（AI 生成，交易日定时执行）
- 提示词模板回测：历史行情重放 + 防未来数据，胜率/回撤/夏普/Jaccard 稳定性多维对比
- 推荐回测统计：按持有期核算实际收益并与沪深300对比
- 交易日志、每日操作计划

**数据源**
OpenAI 兼容接口 / Ollama / LMStudio / AnythingLLM / DeepSeek / 智谱 / 豆包 / 阿里百炼 / Moonshot / 混元 / 讯飞星火 / MiniMax / 小米 MiMo / 硅基流动等聚合平台

---

## 🚀 快速开始

### 下载即用

到 [Releases](https://github.com/RYun601/aipanyi/releases) 下载对应平台的文件：

| 平台 | 文件 |
| --- | --- |
| Windows x64 | `aipanyi-windows-amd64.exe` |
| Windows ARM64 | `aipanyi-windows-arm64.exe` |
| macOS（Intel + Apple Silicon 通用） | `aipanyi-darwin-universal.zip` |

**绿色版，免安装**：Windows 是单个 exe，双击即用，没有安装程序、也不写注册表；macOS 是 zip，解压得到 `.app`。
Linux 版目前不在 CI 发布范围内，需要自行构建，见 [`docs/BUILD_LINUX.md`](docs/BUILD_LINUX.md)。

首次启动后：

1. 打开应用窗口（桌面模式为原生窗口，大小自适应屏幕分辨率）
2. 进入「设置」填入你的 AI 服务 API Key（没有 Key 也可以用 Ollama 跑本地模型）
3. 输入股票代码或名称，开始提问

首次启动会自动初始化 A 股/港股/美股基础数据，网络较慢时请耐心等待，不要连续重复打开程序。

> **Windows 用户请注意**：先新建一个专属文件夹（例如 `D:\AI盘译`），把 exe 放进去再双击运行。
> 程序的数据（`data\stock.db`、日志、缓存）都生成在**程序所在目录**，之后请不要再挪动 exe ——
> 换目录启动等于换一套空白数据，自选股和设置会“消失”。升级时用新 exe 覆盖旧的即可，数据保留。

> **首次运行的安全提示**：本项目的可执行文件**没有代码签名证书**，因此从网络下载后首次运行可能遇到：
>
> - Windows SmartScreen 提示「Windows 已保护你的电脑」→ 点「更多信息」→「仍要运行」
> - 个别杀毒软件 / 安全卫士误报 → 加入信任区即可（未签名的新程序容易被误判）
> - macOS 提示「无法验证开发者」→ 处理办法见 [RUN_MACOS_TERMINAL.md](docs/RUN_MACOS_TERMINAL.md)
>
> 想确认文件没被篡改：在 [Releases](https://github.com/RYun601/aipanyi/releases) 对应版本里找到 `SHA256SUMS.txt`，
> 本地算一遍比对：
>
> ```powershell
> Get-FileHash .\aipanyi-windows-amd64.exe -Algorithm SHA256
> ```
>
> ⚠️ **只从本仓库的 Releases 页面下载**，第三方站点转发的同名文件无法保证安全。

### 从源码构建

环境要求：Go **1.27+**、Node.js 22+、Wails CLI v2.15.0

```bash
git clone https://github.com/RYun601/aipanyi.git
cd aipanyi

# 0. 安装 Wails CLI（一次性）
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0

# 1. 构建前端（两种模式共用；wails build 也会自动跑一遍）
cd frontend && npm install && npm run build && cd ..

# 2a. 桌面模式（默认，Wails 原生窗口）
wails build -platform windows/amd64 -o aipanyi

# 2b. Web 模式（本地 HTTP 服务 + 默认浏览器）
wails build -tags web -platform windows/amd64 -o aipanyi-web
```

> ⚠️ `-o aipanyi` **不会自动补 `.exe`**：Windows 上产物是 `build/bin/aipanyi`（无扩展名），
> 自行 `Copy-Item build/bin/aipanyi build/bin/aipanyi.exe` 即可。
> 也不要用 `go build` 代替 `wails build` —— 拿不到 Wails 运行时，前端产物也不保证被打包进去。

Web 模式下用系统默认浏览器打开本地界面（地址类似 `http://127.0.0.1:<随机端口>`），端口每次启动随机生成，
并带一次性 token 校验，本机之外的设备默认无法访问；桌面模式则是原生窗口。

更多细节见 [`docs/快速开始指南.md`](docs/快速开始指南.md) 与 [`docs/BUILD_LINUX.md`](docs/BUILD_LINUX.md)。

---

## 💖 关于收费：免费下载 + 自愿赞助

**全部功能免费，二进制里没有任何功能锁。**

本项目以 GNU GPLv3 发布，你拥有运行、研究、修改、再分发的完整权利。
GPL 也意味着任何"本地功能锁"都能被合法移除——所以本项目干脆不做锁。

如果你觉得它有用，欢迎自愿赞助：

<div align="center">

  <img src="./docs/sponsor/wechat.png" alt="微信收款码" width="280" height="420" />&emsp;&emsp;<img src="./docs/sponsor/alipay.jpg" alt="支付宝收款码" width="280" height="420" />

</div>

> 赞助**不会**解锁任何功能，纯粹是给作者续命的咖啡钱。☕
> 不赞助也能一直用下去，自动更新同样可用。

---

## ⚠️ 数据来源与免责声明

- 行情与新闻数据来自东方财富、新浪财经、财联社、问财、通达信等公开接口。
- 这些接口大多**禁止商业用途的批量抓取**，本项目按"个人学习研究"场景使用并如实披露来源。
- 数据可能延迟、缺失或出错，**不构成任何投资建议**。
- AI 的分析结果是模型生成内容，可能幻觉、可能过时。投资有风险，决策请自负。
- 本项目仅供学习研究使用。

---


## 📚 文档

- [小白入门使用手册（零基础）](docs/小白入门使用手册.md)
- [小白设置页面使用说明](docs/小白设置页面使用说明.md)
- [快速开始指南](docs/快速开始指南.md)
- [AI盘译 使用手册（完整功能）](docs/AI盘译使用手册.md)
- [后台买卖点信号监控](docs/后台买卖点信号监控.md)
- [知识库与长期记忆功能说明](docs/知识库与长期记忆功能说明.md)
- [AI对话与视觉理解功能说明](docs/AI对话与视觉理解功能说明.md)
- [技能管理功能说明](docs/技能管理功能说明.md)
- [架构决策记录 ADR-001](docs/ADR-001-UI浏览器化与AI边车架构决策.md)

---

## 🤖 给 AI 助手 / 贡献者

本仓库带 [`AGENTS.md`](AGENTS.md)，规定了构建验证命令、分支与提交规范、
发版纪律（**修改说明必须手写，禁用 GitHub 自动生成 release notes**）、以及代码约定。
AI 编码助手在动手前应先读它。

---

## 🦄 致谢

本项目基于以下开源项目构建：

- [AI盘译](https://github.com/ArvinLovegood/go-stock) —— 上游项目，本项目的起点
- [Wails](https://wails.io/) / [Vue](https://vuejs.org/) / [Vite](https://vitejs.org/) / [NaiveUI](https://www.naiveui.com/)
- [ECharts](https://echarts.apache.org/) / [lightweight-charts](https://tradingview.github.io/lightweight-charts/) / [md-editor-v3](https://github.com/imzbf/md-editor-v3)
- [chromem-go](https://github.com/philippgille/chromem-go) / [eino](https://github.com/cloudwego/eino)
- [Tushare](https://tushare.pro/)

感谢上游作者 [ArvinLovegood](https://github.com/ArvinLovegood) 与所有贡献者。

---

## ⭐ Star History

<a href="https://www.star-history.com/?repos=RYun601%2Faipanyi&type=date&legend=top-left">
  <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=RYun601/aipanyi&type=date&legend=top-left" />
</a>

---

## 📄 License

[GNU GPLv3](LICENSE)

本项目是 AI盘译 的衍生作品，依据 GNU General Public License v3.0 发布。
你可以自由使用、研究、修改和再分发本项目的全部源码（包括本 fork 的所有修改），
但必须同样以 GPLv3 授权你的衍生版本，并保留原始版权声明与修改标注。

**简单说：欢迎白嫖，欢迎改，欢迎二次分发，但别闭源。**
