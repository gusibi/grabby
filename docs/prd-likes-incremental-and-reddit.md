# PRD: Reddit 原子抓取能力

> 本 PRD 由 `/to-prd` 从 grill-with-docs 会话综合而成。领域术语见根目录 `CONTEXT.md`。
>
> Grabby 是**原子能力提供者**：每个平台每个功能 = 一个独立原子抓取 API，走统一流程（用户请求 API → 服务端经 WebSocket 连浏览器插件 → 插件打开页面抓内容 → 返回）。登录态由用户提前保证浏览器已登录，风控由调用方考虑，服务只管正确抓取。本 PRD 按此极简标准设计。

## Problem Statement

Grabby 目前只实现了 Twitter 的原子抓取能力（search / timeline / likes）。Reddit 是浏览器执行器的典型适用场景——服务端直连会被 403，而 Reddit 的 `.json` 是公开 API，浏览器插件带用户上下文 fetch 即可干净拿到。当前没有 Reddit 适配器，用户无法通过 Grabby 抓取 Reddit 的 subreddit 帖子列表或单个帖子线程。

需要一个 Reddit 独立适配器，提供 Reddit 的原子抓取能力，复用现有 `fetchInPage` 原语和浏览器执行器链路。

## Solution

新建 Reddit 独立适配器（与 Twitter 适配器平级，独立实现，不共享 Twitter 代码），提供两个原子抓取能力：

1. **`reddit_subreddit`**：给定 subreddit 名，抓取该 subreddit 的最新帖子列表。
2. **`reddit_thread`**：给定帖子 URL，抓取该帖子的标题、正文、评论树。

两者都走 `fetchInPage` 原语：打开 reddit 页面提供请求上下文（Cookie/Origin），在页面上下文 fetch 对应 `.json`，解析返回。与 Twitter 适配器同链路，架构一致。

遵循「每平台独立适配器、独立 HTTP API、独立内容表，只共享底层原语」原则。不做增量同步、不做风控、不做能力协商——这些都是调用方或后续的事，服务只管正确抓取。

## User Stories

1. 作为用户，我想要给定一个 subreddit 名，抓取它的最新帖子列表，以便看到该版块有哪些新帖、拿到它们的标题/作者/URL/摘要。
2. 作为用户，我想要帖子列表返回结构化字段（id、title、author、subreddit、url、正文摘要、score、评论数、发布时间），以便后续处理。
3. 作为用户，我想要给定一个帖子 URL，抓取该帖子的标题、正文、评论树，以便拿到比通用 extract 更干净的结构化数据。
4. 作为 AI agent，我想要 `reddit_subreddit` 和 `reddit_thread` 作为 MCP 工具，以便通过 MCP 调用 Reddit 抓取。
5. 作为 http api 工作流调用方，我想要 `/api/reddit/subreddit` 和 `/api/reddit/thread` 两个 HTTP API，以便直接 HTTP 调用。
6. 作为用户，我想要 Reddit 抓取复用浏览器登录态（用户提前在浏览器登录 Reddit），以便拿到带身份的内容、绕过服务端 403。
7. 作为系统，我想要 Reddit 帖子按 id upsert 进独立 `reddit_posts` 表，以便跨多次抓取去重存档。
8. 作为用户，我想要 `reddit_thread` 的评论实时返回、不落库，以便避免大体量评论数据膨胀存储。
9. 作为用户，我想要浏览器没连接时返回 503、抓取失败时返回明确错误，以便知道是设备问题还是抓取出错。
10. 作为开发者，我想要 Reddit 适配器与 Twitter 适配器同构（独立适配器 + 独立 HTTP API + 独立内容表 + 复用 fetchInPage），以便每个新平台都按同一模式加。

## Implementation Decisions

### 架构原则（已在 `CONTEXT.md` 固化）

- **原子能力提供者**：每个平台功能 = 一个原子抓取 API，走统一流程。服务只管正确抓取，不管登录（用户提前保证）和风控（调用方考虑）。
- **每平台独立适配器、独立 HTTP API、独立内容表**，只共享底层原语（`intercept`/`fetchInPage`/`runPageScript`）。Reddit 不复用 Twitter 代码。
- 不做增量同步、不做失败冷却/随机延迟/限流等风控、不做能力协商。

### Reddit 适配器

- 新建独立适配器，与 twitter 适配器平级。
- **`reddit_subreddit`**：
  - URL：`reddit.com/r/{name}/new.json`（`new` 排序，最新帖子）。
  - 用 `fetchInPage`：打开 `reddit.com/r/{name}/new` 提供请求上下文，`requestUrl` 指向 `.../new.json`。
  - 解析返回的帖子列表为结构化字段。
  - 用 `after` 游标分页取多页（具体页数/上限实现时定，保持合理默认）。
- **`reddit_thread`**：
  - URL：`reddit.com/r/.../comments/{id}/` + `.json`。
  - 用 `fetchInPage`：打开帖子页提供上下文，`requestUrl` 指向 `.../.json`。
  - 解析返回标题、正文、评论树。评论**实时返回、不落库**。
- 链路与 api-reference 中 `fetch_in_page` 用法一致（[docs/api-reference.md](docs/api-reference.md) 的 `/api/fetch_in_page`）。
- Reddit 公开内容无登录态也能拿；登录态由用户提前保证浏览器已登录 Reddit。

### 内容表

- 新建独立内容表 `reddit_posts`（按 id upsert 去重）：`id PK / title / author / subreddit / url / body / score / num_comments / created_utc / fetched_at`。
- 评论不建表。
- 用 SQLite + GORM，随现有 `database.go` 自动迁移。

### MCP 工具

- `reddit_subreddit`：参数 subreddit 名（必填）、limit（可选）、browser（可选）。底层原语不暴露给 MCP。
- `reddit_thread`：参数 url（必填）、browser（可选）。
- 静态注册（与现有 twitter 工具同方式）。

### HTTP API

- `POST /api/reddit/subreddit`、`POST /api/reddit/thread`，与 Twitter 接口风格一致。
- 成功响应结构与 Twitter 一致风格（`success` + 结果数组/对象）。
- 错误码：503（无已连接浏览器）/ 502（浏览器执行无可用结果）/ 400（请求体非法）。
- 遵循 `apis/` 下的 Bruuno spec 约定，新增 `apis/reddit_subreddit.yml`、`apis/reddit_thread.yml`。

### 不做（明确排除）

- 不做 `sync_cursors` 增量同步表。
- 不做失败冷却 / 随机延迟 / 限流。
- 不做错误分类的复杂分级——按现有 Twitter 的错误处理方式（matched X/Y responses 信息、503/502/400 错误码）即可。
- 不做 `reddit_search`（本轮只 subreddit + thread）。
- 不做 Reddit 前端历史视图（`reddit_posts` 表建好即可，视图后续加）。

## Testing Decisions

### 测试原则

只测外部行为，不测实现细节。浏览器/`fetchInPage` 实时往返与真实 Reddit 页面保持手动验证（与现有 Twitter 测试一致——只单测解析器，实时路径靠跑起来验证）。

### 测试 seam（两个，均为现有模式，不新增 seam）

**Seam 1 — 适配器纯函数解析 seam**（沿用 `internal/application/twitter/twitter_test.go` 模式）：

喂入镜像真实 Reddit `.json` 响应的精简样本给解析函数，断言结构化输出。覆盖：
- subreddit `.json` → 帖子列表结构提取、`after` 分页游标解析。
- thread `.json` → 标题/正文/评论树解析。

**Seam 2 — 内存 SQLite 仓储 seam**（沿用 `internal/infrastructure/sqlite/cache_repo_test.go` 模式，`NewDatabase(":memory:")`）：

- `reddit_posts`：按 id upsert 去重、列表查询。

### 不新增的 seam

- 不加 HTTP handler 层单测——handler 是薄转发，行为由适配器 seam 覆盖。
- 不加浏览器/网络层单测——保持手动验证。

## Out of Scope

- **A（twitter_likes 增量同步 + 风控）**：整体不做。likes 现作为原子抓取 API 已是正确形态，增量/风控不是原子能力工具的职责。
- **`sync_cursors` 通用同步游标表**：不做（增量同步不在服务职责内）。
- **风控机制**（随机延迟 / 失败冷却 / 限流）：不做（调用方考虑）。
- **能力协商握手**：不做（register 只上报设备）。
- **`reddit_search`**：本轮不做。
- **Reddit 前端历史视图**：`reddit_posts` 表建好即可，视图后续加。
- **YouTube / Bilibili / 小红书 / web_search / GitHub 直连 API 适配器**：本轮不做。

## Further Notes

- 领域术语已写入根目录 `CONTEXT.md`（原子能力提供者、统一抓取流程、浏览器端职责、每平台独立内容表原则）。实现时遵循该 ubiquitous language。
- 本 PRD 对应单一 issue：Reddit 适配器（subreddit + thread + reddit_posts 表 + 两个 MCP 工具 + 两个 HTTP API）。
- 设计取向：尽可能简单。每个新平台按"独立适配器 + 独立 HTTP API + 独立内容表 + 复用原语"同一模式加，服务只管正确抓取。
