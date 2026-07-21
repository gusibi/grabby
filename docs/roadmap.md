# Grabby Roadmap

> 定位：**把用户自己已登录的浏览器变成 AI 可调用的个人数据 API。**
> 三根支柱：真实所以安全（真实浏览器 + 真实登录态，天然免风控）；零成本自主（本地部署、自己的账号和数据）；一次部署控制多个浏览器。
>
> 纪律（见 `CONTEXT.md`，不再讨论）：原子能力提供者而非爬虫；低频个人使用；每平台独立适配器/独立 API/独立内容表；不做风控机制。
>
> 各阶段的详细技术方案在 `docs/specs/`，可直接交给执行 agent 实现。

总周期约 8–10 周。阶段 0/3/4 是推广的硬前置，阶段 1/2 是差异化卖点的实体化。

---

## 阶段 0：收尾与冻结（第 1 周）

目标：钉住正在漂移的部分，为后面腾出精力。

- [ ] 提交当前前端与小红书改动，此后 **Web UI 冻结**：维持"能看数据、能调试"，只修 bug 不加功能
- [ ] 清理 README/docs 过时内容（python-server 双后端叙事、MCP 示例工具 `add(a,b)` 等占位），统一以 go-server 为主线
- [ ] 清理仓库杂物：根目录 `grabby-v2-*.html` 原型、`grabby.db*` 数据库文件移出 git（加 .gitignore）

## 阶段 1：扩展可靠性 + 多浏览器调度（第 2–4 周）

目标：让扩展从"用户手动用的工具"变成"无人值守的节点"，并把"能连多个浏览器"升级成"会用多个浏览器"。

**扩展可靠性**（方案：[specs/01-extension-reliability.md](specs/01-extension-reliability.md)）
- [ ] 连接自愈：去掉重连次数上限，`chrome.alarms` 每分钟兜底检查，`onStartup` 挂钩
- [ ] WS 握手认证：WebSocket 连接 URL 携带 token，服务端握手校验
- [ ] 统一错误码：`NAV_TIMEOUT` / `NOT_LOGGED_IN` / `EMPTY_RESULT` 等，消灭"假成功"
- [ ] 结果补发缓冲：任务结果发送失败时缓存，重连后按 `message_id` 补发
- [ ] 任务串行队列 + 取消命令
- [ ] register 上报扩展版本号

**多浏览器调度**（方案：[specs/02-multi-browser-routing.md](specs/02-multi-browser-routing.md)）
- [ ] 扩展 options 配置平台标签，register 上报
- [ ] 平台 API 按标签自动路由，同平台多设备故障切换
- [ ] 503 细化为"该平台无可用设备"，`/api/browsers` 返回标签/版本/健康状态

## 阶段 2：缓存体系 + 统一数据出口（第 4–7 周）

目标：把"个人数据源"从各平台散落的表变成 agent 可依赖的统一资产；缓存 + 在途合并是"低频免风控"纪律的机器保障（agent 重试循环不会打穿浏览器）。

**缓存与在途合并**（方案：[specs/03-cache-and-coalescing.md](specs/03-cache-and-coalescing.md)）
- [ ] 缓存只做在服务端，扩展保持无状态执行器（已定，不再讨论）
- [ ] 按内容类型分默认 TTL：稳定详情 24h / 时效列表 15min / 个人增量不缓存走 `since`
- [ ] 请求级覆盖：`refresh=true` 强制刷新，`max_age=<秒>` 声明可接受的旧度
- [ ] 在途请求合并（singleflight）：并发相同请求共享一次浏览器任务
- [ ] 缓存容量上限 + LRU 清理

**统一数据出口**（方案：[specs/04-records-and-incremental.md](specs/04-records-and-incremental.md)；时间紧可整体推迟到 launch 后）
- [ ] 增量抓取标准化：原子能力支持 `since` 游标，返回"新增了什么"
- [ ] 跨平台查询出口 `/api/records`，统一最小字段集，各平台表不动只加读取视图
- [ ] JSONL / Markdown 批量导出
- [ ] MCP 工具与 skill 支持跨平台增量查询

## 阶段 3：可靠性与可观测（第 7–8 周，之后常态化）

目标：推广后用户遇到的第一个问题一定是"某平台抓不到了"，必须让失败透明、可自助诊断。

- [ ] **适配器自检**：每平台一个轻量 health check，`/health` 与 UI 点名坏了哪个平台
- [ ] **错误分类贯通**：扩展端错误码（阶段 1）映射到服务端 HTTP 错误响应，agent 可自助判断
- [ ] **解析回归测试**：每个适配器录制脱敏响应 fixture
- [ ] **远程日志**：失败 response 附带扩展端最近日志，远程排障不用跑到那台机器前

## 阶段 4：新平台适配器 + 上手体验（第 8–10 周，推广前置）

**平台范围已定，只加以下四项，其他不加**（方案：[specs/05-new-adapters.md](specs/05-new-adapters.md)）：

| 优先级 | 平台/能力 | 理由 |
|------|------|------|
| 高 | **YouTube**：订阅流、稍后观看、视频字幕 | `ytInitialPlayerResponse` 已在白名单，成本低；海外 launch 演示效果最好 |
| 高 | **B 站**：动态、收藏夹 | `__INITIAL_STATE__` 已在白名单；国内受众大 |
| 中 | **Reddit saved** | 补齐"收藏"维度，成本低 |
| 中 | **小红书收藏** | 同上，复用 userNotes 的 initial state 模式 |

统一叙事：**"你的点赞和收藏，是你自己的数据"**——五个平台的点赞/收藏每天自动汇入自己的数据库，agent 随时可查。明确不做：Hacker News / RSS（有公开 API，不需要登录态浏览器）、微信公众号专门适配器（文章页用通用 extract 已覆盖）、知乎/微博（暂缓）。

**上手体验**
- [ ] 一条命令安装：GitHub Releases 多平台二进制 + `brew install`（或安装脚本）
- [ ] README 双语，quickstart 带 30 秒 GIF
- [ ] 部署前提文档：无人值守浏览器的防休眠设置指引

---

# 推广计划

## 定位一句话

> **Grabby — Turn your own logged-in browser into a personal data API for AI agents.**
> Local-first. No scraping service, no API tax, no bot detection — because it's just your browser.

中文："把你自己已登录的浏览器变成 AI 可调用的个人数据 API。本地部署，免费，不被风控。"

## 三个信息支柱（所有内容围绕这三点）

1. **真实所以安全**：数据从你真实浏览器、真实登录态出来，平台看到的就是你本人在浏览
2. **一次部署，多浏览器**：多地点、多账号的浏览器注册到一个服务，按平台自动路由
3. **为 agent 而生**：MCP 原生 + Claude skill + 结构化增量 API；日报只是它顺手能干的事之一

反式声明（统一口径，尤其 HN）："这不是爬虫，不做规模化抓取。它是给你自己的账号、每天一两次频率设计的。"

## 时间线

### T-3 周起：Build in public（X + 即刻/V2EX）

- 每周 2–3 条带演示 GIF 的推文，各讲一个具体场景（抓小红书不用付费 API / 一个服务控制两地浏览器 / agent 每早汇总全平台点赞收藏）
- 讲原始动机故事：工具收费/被风控 → 干脆借浏览器
- 目标：launch 前积累关注者和 GitHub star 作为社会证明

### T-2 周：分发渠道铺设

- 提交 MCP 目录：mcp.so、Smithery、PulseMCP、各 Awesome-MCP 列表（受众最精准、成本最低）
- Chrome 商店**改名为 Grabby**（现名"MCP 网页内容采集工具"），商店描述重写双语
- README 置顶 30 秒演示 GIF；准备 2–3 个 good first issue

### Launch 周（选周二–周四）

- **Product Hunt**：tagline 用定位一句话；画廊 4–5 张图（架构图、多浏览器控制、agent 对话演示、日报截图）+ 40 秒视频；首评讲动机故事；提前约 10–20 个朋友当天真实使用后支持
- **Hacker News「Show HN」**（与 PH 错开一天）：技术向正文——三原语设计、为什么每平台独立适配器、为什么故意不做限流；正面回应 ToS 质疑（用反式声明）
- **Reddit**：r/selfhosted、r/LocalLLaMA、r/SideProject、r/ClaudeAI，各写符合版规的原生帖
- **X launch thread**：把前三周的演示串成一条线

### Launch 后（常态化）

- 每个新平台适配器 = 一篇内容："How I read [平台] without the API tax"
- 每周一条 changelog 推文；GitHub issue 快速响应（早期响应速度就是口碑）
- 关注指标：GitHub stars、扩展安装数、PH 排名、issue 质量

## 风险预案

| 风险 | 预案 |
|------|------|
| ToS 质疑 | 统一口径："你自己的浏览器、你自己的账号、个人阅读频率，本质是更聪明的书签/剪藏工具"；不使用"绕过/对抗/规模化"等词汇 |
| "某平台坏了"差评 | 阶段 3 必须 launch 前完成：错误点名原因 + 适配器自检，把"坏了"变成"已知且透明" |
| 竞品比较 | README 主动放对比表（vs Firecrawl / browser-use / RSSHub），各自适用场景写公道 |
