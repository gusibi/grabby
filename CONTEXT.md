# Grabby 领域上下文（Glossary）

> 本文件只记录领域术语与 ubiquitous language，不含实现细节。实现决策见 `docs/adr/`。

## 核心定位：原子能力提供者

- **原子抓取能力（Atomic Fetch Capability）**：Grabby 是原子能力提供者，不是爬虫。每个平台每个功能暴露成一个独立的原子抓取 API（如 `/api/twitter/likes`、`/api/twitter/timeline`、`/api/twitter/search`、未来的 `/api/reddit/subreddit`）。
- **统一抓取流程**：用户请求 API → 服务端经 WebSocket 连接浏览器插件 → 插件打开页面抓取内容 → 返回结果。每个原子能力都走同一条流程。
- **浏览器端职责**：真实数据获取、登录认证都在浏览器端完成。**登录态由用户提前保证**（浏览器已登录目标平台），服务端不管登录。
- **风控不在服务职责内**：使用场景是每天请求 1~2 次，不是作为爬虫高频抓取。风控由调用方考虑，服务只管正确地把数据抓回来。不在服务里做随机延迟、失败冷却、限流等风控机制。
- **服务端职责**：调度（选浏览器连接）+ 站点适配（把请求翻译成原语调用）+ 解析。只负责正确的数据抓取与返回。

## 调用模型

- **主动选择，非意图协商**：服务端按"已实现哪些平台适配器"静态注册 MCP 工具；运行时由调用方主动选择某个平台 API 发起请求。**没有能力协商握手**——扩展是通用执行器，register 只上报设备（`connect_id`+`name`），MCP 据此知道可连接哪些设备。设备可用性已用 503 gating（无设备连接即拒绝）。
- **意图级工具（Intent-level Tool）**：面向 MCP client 的高层工具（如 `twitter_search`/`twitter_timeline`/`twitter_likes`），由服务端站点适配器翻译成底层原语调用。底层原语（`runPageScript`/`fetchInPage`/`intercept`）仅服务端内部使用，不暴露给 MCP client。

## 浏览器执行器

- **浏览器执行器（Browser Executor）**：浏览器扩展在用户真实浏览器里带登录态地取数据/调站内 API/执行操作。底层原语：`intercept`（早注入拦截 XHR/GraphQL 响应，MAIN world hook + ISOLATED bridge）、`fetchInPage`（页面上下文 fetch，带登录态）、`runPageScript`（白名单页面脚本，读页面 JS 变量，不接受任意 eval）。
- **站点适配器（Site Adapter）**：服务端把请求翻译成原语调用的层，**每个平台一个独立适配器、独立实现**——因为各平台规则、GraphQL/DOM 结构都不同，不共享逻辑。每个适配器有自己独立的 HTTP API（如 Twitter 是 `/api/twitter/*`，Reddit 是 `/api/reddit/*`）和独立的内容 record 表。统一降级链：拦截响应 > 读页面变量（白名单脚本）> DOM 解析。

## 内容归档（Record）

- **内容表按站点独立**：`tweets` 表只存 Twitter 内容（`source` 字段 `"search"/"timeline"/"likes"` 是 Twitter 内部意图区分）；Reddit 用 `reddit_posts`，小红书用 `xhs_notes`，其它站点各自独立 record 表。
- **每平台独立内容表原则（确定，不再讨论）**：每个平台一张独立内容表，不跨平台共用。评论区等大体量、低时效数据不落库，只实时返回。
