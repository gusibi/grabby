# API 参考

## 概述

Grabby 提供四类接口：

1. **Open HTTP API** — 以 `/open/api` 开头，不需要登录，用于内容浏览、AI 日报与 RSS 等公开读取。
2. **Protected HTTP API** — 以 `/api` 开头，用于管理、配置、浏览器交互和写操作。
3. **MCP (Model Context Protocol)** — 用于 AI Agent 集成，包含网页抓取、截图与 X/Twitter 读取工具。
4. **WebSocket** — 浏览器扩展与后端之间的实时通信协议。

> 说明：Grabby 借用你已登录的真实浏览器执行抓取，因此能拿到带登录态、带真实指纹的内容（如 X/Twitter 的搜索、时间线、点赞）。这部分能力通过新增的 `intercept` WebSocket 命令实现，详见下文「WebSocket 协议」。

---

## HTTP API

### 基础信息

| 项目 | 值 |
|------|-----|
| 基础 URL | `http://{HOST}:{PORT}` |
| 默认地址 | `http://localhost:5040` |
| 内容类型 | `application/json` |

### 认证

`/open/api` 接口不需要认证。

`/api` 接口使用 Echo middleware 统一鉴权，支持两种方式：

- Cookie 登录：配置 `GRABBY_ADMIN_KEY` 后，通过 `/api/auth/login` 登录，服务端写入 `grabby_admin_session` cookie。
- 固定 Token：配置 `GRABBY_API_TOKEN` 后，请求携带以下任一 header：

```
Authorization: Bearer your_token
X-API-Key: your_token
X-Grabby-Token: your_token
```

`GRABBY_ADMIN_KEY` 和 `GRABBY_API_TOKEN` 都留空时关闭认证，允许所有 `/api` 请求。

> 注意：浏览器数据接口的错误响应体统一为 `{"detail": "..."}` 格式。

---

## 浏览器数据接口

这组接口会把请求转发给浏览器扩展执行。若没有已连接的浏览器，返回 `503`。

### POST /api/extract

提取指定 URL 的网页内容并返回 Markdown。

#### 请求

```http
POST /api/extract HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://example.com",
  "browser": "chrome-office"
}
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 要提取的网页 URL |
| `browser` | string | 否 | 指定使用的浏览器名称，省略时使用默认浏览器 |

#### 查询参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `refresh` | boolean | false | `true`/`1` 时跳过缓存强制重新抓取并更新缓存 |

> 缓存：提取结果按 URL 存入 `extract_cache` 表。命中缓存直接返回（`cached: true`），不再走浏览器抓取；只有抓取成功（markdown 非空）才会写入缓存。需要最新内容时带 `?refresh=true`。

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://example.com",
  "title": "Example Domain",
  "markdown": "# Example Domain\n\nThis domain is for use in illustrative examples...",
  "cached": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际提取的 URL |
| `title` | string | 网页标题 |
| `markdown` | string | 提取的 Markdown 内容 |
| `cached` | boolean | 是否来自缓存（`true`=命中 `extract_cache`，`false`=本次新抓取） |

#### 错误响应

| 状态码 | 含义 | 示例 |
|--------|------|------|
| `503` | 浏览器扩展未连接 / 指定浏览器未找到 | `{"detail":"no browser connections available"}` |
| `502` | 浏览器扩展执行返回错误 | `{"detail":"Browser extension error: ..."}` |
| `504` | 超时或连接丢失 | `{"detail":"waiting for response timed out"}` |

---

### POST /api/screenshot

对指定 URL 截图，返回 Base64 图片数据。

#### 请求

```http
POST /api/screenshot HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://example.com",
  "fullPage": true,
  "browser": "chrome-office"
}
```

#### 请求参数

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要截图的网页 URL |
| `fullPage` | boolean | 否 | false | 是否截取整个页面 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://example.com",
  "imageData": "data:image/png;base64,iVBORw0KGgo..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际截图的 URL |
| `imageData` | string | Base64 图片数据（data URL 格式） |

#### 错误响应

与 `/api/extract` 相同（`503` / `502` / `504`）。

---

### Twitter/X 接口

这三个接口是 X 站点适配器的 HTTP 入口（见 `docs/browser-executor-plan.md` §7），与同名 MCP 工具 `twitter_search` / `twitter_timeline` / `twitter_likes` **共用同一套适配器与底层 `intercept` 原语**，仅是把能力额外暴露成 HTTP，便于 `http api -> 浏览器` 的工作流直接调用。

均依赖**用户已登录**的浏览器；返回空或报错通常意味着未登录或 X 页面结构变化（错误信息会带 `matched X/Y responses`），而非真的没有内容。

> 存储：`search` / `timeline` / `likes` 抓到的推文都会按推文 ID upsert 进同一张 `tweets` 表（含 `source` 字段标记来源），跨多次抓取自动去重存档。存储为尽力而为，失败只记日志、不影响接口返回。

所有 Twitter 接口的成功响应结构一致：

```json
{
  "success": true,
  "count": 2,
  "tweets": [ /* 单条结构见「推文结构」 */ ]
}
```

错误响应：`503`（无已连接浏览器）/ `502`（浏览器执行无可用结果，如未登录 / 页面结构变化）/ `400`（请求体非法）。

#### POST /api/twitter/search

在已登录浏览器中搜索 X/Twitter，返回结构化推文。

```http
POST /api/twitter/search HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "query": "golang",
  "limit": 40,
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `limit` | number | 否 | 40 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### POST /api/twitter/timeline

读取用户主页时间线（需已登录）。

```http
POST /api/twitter/timeline HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "kind": "for_you",
  "limit": 40,
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `kind` | string | 否 | `for_you` | `for_you`（推荐）或 `following`（关注） |
| `limit` | number | 否 | 40 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### POST /api/twitter/likes

读取 `x.com/<handle>/likes` 的点赞推文（读取自己的点赞需已登录）。

```http
POST /api/twitter/likes HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "handle": "jack",
  "limit": 60,
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `handle` | string | 是 | - | X 用户名（不含 `@`），如 `jack` |
| `limit` | number | 否 | 60 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

推文字段结构见下方 MCP 工具的「推文结构」。

---

### Reddit 接口

Reddit 适配器是独立站点适配器（见 `docs/prd-likes-incremental-and-reddit.md`），与同名 MCP 工具 `reddit_thread` 共用同一套适配器与底层 `fetchInPage` 原语，额外暴露成 HTTP 便于 `http api -> 浏览器` 工作流直接调用。

Reddit 的 `.json` 端点是公开 API；适配器在已登录浏览器中打开帖子页（提供 Cookie/Origin 上下文），再 fetch 同一 URL 加 `.json` 后缀。**登录态由用户提前保证**（浏览器已登录 Reddit），服务只管正确抓取。

错误响应：`503`（无已连接浏览器）/ `502`（浏览器执行无可用结果，如未登录 / 页面不可访问 / Reddit 返回错误）/ `400`（请求体非法）。

#### POST /api/reddit/thread

抓取一个 Reddit 帖子及其评论树，返回结构化的帖子与嵌套评论。

```http
POST /api/reddit/thread HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://www.reddit.com/r/golang/comments/abc123/go_124_released/",
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | Reddit 帖子 permalink |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://www.reddit.com/r/golang/comments/abc123/go_124_released/",
  "post": {
    "id": "abc123",
    "title": "Go 1.24 released",
    "author": "gopher",
    "subreddit": "golang",
    "url": "https://www.reddit.com/r/golang/comments/abc123/go_124_released/",
    "content_url": "https://go.dev/blog/go1.24",
    "body": "Discussion thread for the new release.",
    "score": 542,
    "num_comments": 31,
    "created_utc": 1718000000.5
  },
  "comments": [
    {
      "id": "c1",
      "author": "alice",
      "body": "Finally generics are stable.",
      "score": 12,
      "created_utc": 1718000100.0,
      "replies": [
        { "id": "c1a", "author": "bob", "body": "they have been since 1.18", "score": 3, "created_utc": 1718000200.0 }
      ]
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 帖子 permalink |
| `post` | object | 帖子（见下） |
| `comments` | array | 顶层评论树，每条可嵌套 `replies` |

帖子字段：`id` / `title` / `author` / `subreddit` / `url`（permalink）/ `content_url`（帖子指向的外链）/ `body`（selftext）/ `score` / `num_comments` / `created_utc`。

> 评论实时返回、不落库。

#### POST /api/reddit/subreddit

抓取一个 subreddit 的最新帖子列表（`/new` 排序），返回结构化帖子。

```http
POST /api/reddit/subreddit HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "subreddit": "golang",
  "limit": 100,
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `subreddit` | string | 是 | - | 版块名，不含 `r/`，如 `golang` |
| `limit` | number | 否 | 100 | 返回帖子上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### 成功响应 (200)

```json
{
  "success": true,
  "count": 2,
  "posts": [
    {
      "id": "p1",
      "title": "First post",
      "author": "alice",
      "subreddit": "golang",
      "url": "https://www.reddit.com/r/golang/comments/p1/first_post/",
      "content_url": "https://example.com/article1",
      "body": "",
      "score": 100,
      "num_comments": 5,
      "created_utc": 1718000000.0
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `count` | integer | 返回帖子数 |
| `posts` | array | 帖子数组，字段同上方「帖子字段」 |

> 用 `after` 游标分页取多页直到 `limit`；每条帖子按 id upsert 进 `reddit_posts` 表，跨多次抓取去重存档。

#### POST /api/reddit/search

在 Reddit 搜索帖子，返回结构化帖子。可限定在单个 subreddit 内搜索。

```http
POST /api/reddit/search HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "query": "ai agent",
  "subreddit": "golang",
  "sort": "relevance",
  "limit": 100,
  "browser": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `subreddit` | string | 否 | - | 限定版块内搜索（不含 `r/`）；省略则全站搜索 |
| `sort` | string | 否 | Reddit 默认 | `relevance` / `new` / `top` / `comments` |
| `limit` | number | 否 | 100 | 返回帖子上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### 成功响应 (200)

```json
{
  "success": true,
  "count": 2,
  "posts": [ /* 帖子数组，字段同「帖子字段」 */ ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `count` | integer | 返回帖子数 |
| `posts` | array | 帖子数组，字段同上方「帖子字段」 |

> 全站搜索走 `/search.json`，限定版块走 `/r/{name}/search.json?restrict_sr=1`。用 `after` 游标分页取多页直到 `limit`；每条帖子按 id upsert 进 `reddit_posts` 表。

---

### 小红书接口

小红书是独立站点适配器，与同名 MCP 工具 `xiaohongshu_note`/`xiaohongshu_search`/`xiaohongshu_user_notes` 共用同一套适配器。与 Reddit（fetchInPage .json）不同，小红书混合两种原语：

- **笔记详情**：走 `runPageScript` 读取页面 SSR 的 `window.__INITIAL_STATE__.note.noteDetailMap`（笔记数据服务端渲染，非 XHR）。
- **搜索 / 用户主页笔记**：走 `intercept` 早注入拦截 `/api/sns/web/v1/search/notes`、`/api/sns/web/v1/user_posted` 的 XHR 响应。

**登录态由用户提前保证**（浏览器已登录小红书），服务只管正确抓取。风控由调用方考虑（使用场景低频，非爬虫）。

错误响应：`503`（无已连接浏览器）/ `502`（浏览器执行无可用结果，如未登录 / 页面结构变化 / 接口未捕获）/ `400`（请求体非法）。

#### POST /api/xiaohongshu/note

抓取一个小红书笔记详情（标题、正文、图片、作者、互动数）和前 100 条顶层评论。评论通过浏览器拦截 `/api/sns/web/v2/comment/page`，滚动页面触发分页；每条顶层评论会保留接口随带的 `sub_comments`。

```http
POST /api/xiaohongshu/note HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://www.xiaohongshu.com/explore/691345f70000000003010c2b?xsec_token=...&xsec_source=pc_search"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 笔记 explore URL（含 xsec_token） |
| `browser` | string | 否 | 指定使用的浏览器名称 |

成功响应：

```json
{
  "success": true,
  "note": {
    "id": "691345f70000000003010c2b",
    "title": "Java→Go无痛入门",
    "desc": "Java工程师转Go实战系列-01/07",
    "type": "normal",
    "author": "执一｜AI与云计算",
    "author_id": "5f938394000000000100b307",
    "liked_count": "23",
    "collected_count": "13",
    "comment_count": "0",
    "share_count": "1",
    "images": ["http://.../img1.jpg", "http://.../img2.jpg"],
    "url": "https://www.xiaohongshu.com/explore/...",
    "comments": [
      {
        "id": "6a2adcb3000000002b029634",
        "note_id": "691345f70000000003010c2b",
        "content": "MacBook 能用吗[偷笑R]",
        "author": "闲散产品人",
        "author_id": "6970ffd900000000190352f8",
        "ip_location": "四川",
        "like_count": "0",
        "liked": false,
        "create_time": 1781193907000,
        "sub_comment_count": "5",
        "sub_comments": []
      }
    ]
  }
}
```

#### POST /api/xiaohongshu/search

在小红书搜索笔记，返回结构化笔记列表。

```http
POST /api/xiaohongshu/search HTTP/1.1
Content-Type: application/json

{ "query": "golang", "limit": 50 }
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `limit` | number | 否 | 50 | 返回笔记上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

成功响应：`{ "success": true, "count": N, "notes": [...] }`，每条 note 含 `id`/`title`/`author`/`author_id`/`liked_count`/`url`/`xsec_token` 等。

#### POST /api/xiaohongshu/user_notes

抓取一个小红书用户发布的笔记列表。

```http
POST /api/xiaohongshu/user_notes HTTP/1.1
Content-Type: application/json

{ "url": "https://www.xiaohongshu.com/user/profile/5f938394000000000100b307", "limit": 50 }
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 用户主页 URL |
| `limit` | number | 否 | 50 | 返回笔记上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

成功响应：`{ "success": true, "count": N, "notes": [...] }`，note 字段同 search。

> search 与 user_notes 抓到的笔记按 id upsert 进 `xhs_notes` 表，跨多次抓取去重存档。

---

### POST /api/run_page_script

在目标页面里执行**白名单页面脚本**（浏览器执行器原语，见 `docs/browser-executor-plan.md` §4.1），用于读取页面 JS 变量、JSON-LD、meta 等不在可见 DOM 里的数据。只接受内置脚本名，不接受任意 JS。

#### 请求

```http
POST /api/run_page_script HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "script": "youtube.initialPlayerResponse",
  "params": {},
  "visible": false,
  "browser": "chrome-office"
}
```

#### 请求参数

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要打开的页面 URL（脚本执行上下文） |
| `script` | string | 是 | - | 白名单脚本名，见下表 |
| `params` | object | 否 | `{}` | 传给脚本的结构化参数（不会拼进脚本体） |
| `visible` | boolean | 否 | false | 是否短暂激活标签页，结束后恢复原标签页 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**当前白名单脚本**：

| 脚本名 | 返回 |
|--------|------|
| `youtube.initialPlayerResponse` | YouTube `window.ytInitialPlayerResponse`（含字幕轨等） |
| `bilibili.initialState` | B站 `window.__INITIAL_STATE__` |
| `page.extractJsonLd` | 页面所有 `application/ld+json` 解析结果（`{items:[...]}`） |
| `page.readMeta` | 页面 `<meta property/name>` 键值对 |

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "json": { "...": "脚本返回的对象（对象类结果）" }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际执行的页面 URL |
| `json` | object | 脚本返回的对象（对象类结果时出现） |
| `text` | string | 脚本返回的非对象结果（数组/标量，JSON 编码后字符串；可选） |

> 脚本名不在白名单时返回 `502`，错误信息为 `script not allowed: <name>`。

#### 错误响应

与 `/api/extract` 相同（`400` / `503` / `502` / `504`）。

---

### POST /api/fetch_in_page

在目标页面的上下文里发起 `fetch`（浏览器执行器原语，见 `docs/browser-executor-plan.md` §4.2）。请求运行在页面 MAIN world，**共享页面 Cookie 与 Origin**，因此带登录态、更接近真实请求。适合 Reddit `.json`、YouTube timedtext 等同源简单 GET。

> 注意：不万能。CORS、SameSite Cookie、CSRF token、站点自定义 header 仍需调用方按站点显式处理。

#### 请求

```http
POST /api/fetch_in_page HTTP/1.1
Content-Type: application/json
Authorization: Bearer your_token

{
  "url": "https://www.reddit.com/r/golang/comments/xxxx/",
  "requestUrl": "https://www.reddit.com/r/golang/comments/xxxx/.json",
  "method": "GET",
  "headers": { "Accept": "application/json" },
  "credentials": "include",
  "browser": "chrome-office"
}
```

#### 请求参数

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要打开的页面（提供请求上下文与 Cookie/Origin） |
| `requestUrl` | string | 否 | 同 `url` | 实际 fetch 的地址（可与页面不同） |
| `method` | string | 否 | `GET` | HTTP 方法 |
| `headers` | object | 否 | - | 自定义请求头 |
| `body` | string | 否 | - | 请求体 |
| `credentials` | string | 否 | `include` | fetch credentials 模式 |
| `visible` | boolean | 否 | false | 是否短暂激活标签页 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://www.reddit.com/r/golang/comments/xxxx/.json",
  "status": 200,
  "text": "{...响应体原文...}"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际请求的 URL |
| `status` | number | HTTP 响应状态码 |
| `text` | string | 响应体原文（调用方自行解析 JSON/文本） |

#### 错误响应

与 `/api/extract` 相同（`400` / `503` / `502` / `504`）。页面内 fetch 抛错（CORS/网络等）时返回 `502`，错误信息为 `fetchInPage 请求失败: ...`。

---

### GET /open/api/health

健康检查端点。

#### 成功响应 (200)

```json
{
  "status": "ok",
  "browser_connected": true,
  "browser_count": 2,
  "browsers": [
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-home"}
  ],
  "timestamp": "2026-06-13T12:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 服务状态，`ok` 表示正常 |
| `browser_connected` | boolean | 是否有浏览器扩展已连接 |
| `browser_count` | integer | 已连接浏览器数量 |
| `browsers` | array | 已连接浏览器列表（`conn_id` / `name`） |
| `timestamp` | string | ISO 8601 格式时间戳 |

---

## 浏览器管理接口

### GET /api/browsers

获取当前已连接的浏览器列表。

#### 成功响应 (200)

```json
{
  "browsers": [
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-home"},
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-office"}
  ],
  "count": 2
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `browsers` | array | 已连接的浏览器列表 |
| `browsers[].conn_id` | string | 浏览器连接 ID |
| `browsers[].name` | string | 浏览器名称 |
| `count` | integer | 已连接浏览器数量 |

---

### POST /api/browsers/register

注册一个新的浏览器实例。浏览器在通过 WebSocket 连接之前需要先注册。

#### 请求

```http
POST /api/browsers/register HTTP/1.1
Content-Type: application/json
X-Grabby-Token: your_token

{
  "connect_id": "browser-tools",
  "name": "chrome-office"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `connect_id` | string | 是 | 浏览器的连接标识，需与浏览器扩展的 API 密钥一致 |
| `name` | string | 是 | 浏览器名称，用于多浏览器场景下的标识，不可重复 |

#### 成功响应 (200)

```json
{
  "success": true,
  "browser": {
    "connect_id": "browser-tools",
    "name": "chrome-office"
  }
}
```

#### 错误响应

| 状态码 | 含义 |
|--------|------|
| `400` | 请求体非法 / 缺少必填字段 |
| `409` | connect_id 已注册但名称不同，或名称已被其他 connect_id 占用 |

---

### POST /api/browsers/kick

主动断开一个已连接的浏览器。

#### 请求

```json
{
  "conn_id": "ws_browser:browser-tools"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `conn_id` | string | 是 | 要断开的浏览器连接 ID |

#### 成功响应 (200)

```json
{"success": true}
```

#### 错误响应

| 状态码 | 含义 |
|--------|------|
| `400` | 请求体非法 / 缺少 `conn_id` |
| `404` | 连接不存在 |

---

## 采集与内容管理接口

这组接口服务于 Grabby 的 RSS/定时采集与 Web 控制台，操作本地数据库中的内容（items）、订阅源（sources）与运行日志。响应体为 JSON，失败时返回相应 4xx/5xx 状态码与错误信息。

### 内容（Items）

| 方法 | 路径 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/open/api/items` | 分页查询内容列表（默认每页 20 条，游标分页） | query: `category`、`source_category`、`origin`、`q`、`starred`(0/1)、`read_status`、`cursor`、`limit` |
| GET | `/open/api/items/{id}` | 获取单条内容详情（返回 `item` 与渲染后的 `html_content`） | path: `id` |
| POST | `/api/items/{id}/read` | 设置已读状态 | body: `{"read_status": 0/1}` |
| POST | `/api/items/{id}/star` | 设置收藏状态 | body: `{"starred": 0/1}` |

`GET /open/api/items` 响应：

```json
{ "items": [ /* ... */ ], "cursor": "下一页游标，空表示到底" }
```

### 订阅源（Sources）

| 方法 | 路径 | 说明 | 请求体/参数 |
|------|------|------|------------|
| GET | `/api/sources` | 列出所有订阅源 | - |
| POST | `/api/sources` | 新建订阅源 | body: `{id, name, type, url, schedule, default_category?, config?, category?}`（前 5 项必填） |
| PUT | `/api/sources/{id}` | 更新订阅源 | body: 同上（不含 `id`） |
| DELETE | `/api/sources/{id}` | 删除订阅源 | - |
| POST | `/api/sources/{id}/toggle` | 启用/停用 | body: `{"enabled": 0/1}` |
| POST | `/api/sources/{id}/run` | 立即抓取一次（最长 5 分钟） | 返回 `{"success": true, "items_added": N}` |

### 日志与统计

| 方法 | 路径 | 说明 | 参数 |
|------|------|------|------|
| GET | `/api/logs` | 最近 50 条抓取日志 | query: `source_id`（可选，按源过滤） |
| GET | `/open/api/stats` | 内容统计 | 返回 `total_count`、`unread_count`、`starred_count`、`categories`、`source_categories`、`source_category_unread` |

---

## AI 接口

AI 读取接口挂载在 `/open/api/ai/` 下；生成、设置和评估类操作挂载在 `/api/ai/` 下。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/open/api/ai/quality` | 按质量分/分类/天数筛选内容（游标分页） |
| GET | `/open/api/ai/categories` | AI 分类列表 |
| GET | `/open/api/ai/items` | AI 维度的内容列表 |
| GET | `/open/api/ai/analysis/{id}` | 单条内容的 AI 分析结果 |
| GET | `/open/api/ai/daily` | 获取某日早/晚报（query: `date`、`type`） |
| GET | `/open/api/ai/daily/list` | 早晚报列表（query: `limit`、`type`） |
| POST | `/api/ai/daily/generate` | 异步生成早/晚报（body/query: `date`、`type`） |
| GET | `/open/api/ai/daily/rss` | 早晚报 RSS 输出（query: `limit`） |
| POST | `/api/ai/reanalyze/{id}` | 重新分析指定内容 |
| GET | `/open/api/ai/stats` | AI 分析统计 |
| GET / POST | `/api/ai/settings` | 读取 / 保存 AI 设置 |
| POST | `/api/ai/test` | 测试 AI 服务连接 |
| POST | `/api/ai/start_eval` | 启动评估任务 |

---

## MCP 工具

MCP Server 挂载在 `http://localhost:5040/mcp`，使用 SSE (Server-Sent Events) 传输。

### tool: screenshot

捕获指定网页的截图。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要截图的网页 URL |
| `fullPage` | boolean | 否 | false | 是否截取整个页面 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：Base64 编码的图片数据（data URL 格式）。

---

### tool: extract

提取指定网页的内容并返回 Markdown。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要提取的网页 URL |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：Markdown 格式的网页内容。

---

### tool: twitter_search

在用户已登录的浏览器中搜索 X/Twitter，返回结构化推文。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `limit` | number | 否 | 40 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：推文数组的 JSON 字符串，单条结构见下方「推文结构」。

---

### tool: twitter_timeline

读取用户的 X/Twitter 主页时间线（需已登录），用于挑选「今天值得看」的内容。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `kind` | string | 否 | `for_you` | `for_you`（推荐）或 `following`（关注） |
| `limit` | number | 否 | 40 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：推文数组的 JSON 字符串。

---

### tool: twitter_likes

读取 `x.com/<handle>/likes` 中的点赞推文（读取自己的点赞需已登录），用于同步/归档点赞。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `handle` | string | 是 | - | X 用户名（不含 `@`），如 `jack` |
| `limit` | number | 否 | 60 | 返回推文上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：推文数组的 JSON 字符串。

> 注意：X 的点赞页可见性与接口结构变动频繁。若返回为空或报错（错误信息会带 `matched X/Y responses`），通常意味着未登录或页面结构已变化，而非真的没有点赞。

---

### tool: reddit_thread

抓取一个 Reddit 帖子及其评论树（需已登录浏览器）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | Reddit 帖子 permalink URL |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：帖子的 JSON 字符串（`post` 对象 + 嵌套 `comments` 数组）。字段结构见上方「Reddit 接口」。

---

### tool: reddit_subreddit

抓取一个 Reddit subreddit 的最新帖子列表（需已登录浏览器）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `subreddit` | string | 是 | - | 版块名，不含 `r/`，如 `golang` |
| `limit` | number | 否 | 100 | 返回帖子上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：帖子数组的 JSON 字符串。字段结构见上方「Reddit 接口」。

---

### tool: reddit_search

在 Reddit 搜索帖子（需已登录浏览器），可限定在单个 subreddit 内。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `subreddit` | string | 否 | - | 限定版块内搜索（不含 `r/`） |
| `sort` | string | 否 | Reddit 默认 | `relevance` / `new` / `top` / `comments` |
| `limit` | number | 否 | 100 | 返回帖子上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：帖子数组的 JSON 字符串。字段结构见上方「Reddit 接口」。

---

### tool: xiaohongshu_note

抓取一个小红书笔记详情（需已登录浏览器）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 笔记 explore URL |
| `browser` | string | 否 | 指定使用的浏览器名称 |

**返回**：笔记对象的 JSON 字符串。字段结构见上方「小红书接口」。

---

### tool: xiaohongshu_search

在小红书搜索笔记（需已登录浏览器）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `query` | string | 是 | - | 搜索关键词 |
| `limit` | number | 否 | 50 | 返回笔记上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：笔记数组的 JSON 字符串。

---

### tool: xiaohongshu_user_notes

抓取一个小红书用户发布的笔记列表（需已登录浏览器）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 用户主页 URL |
| `limit` | number | 否 | 50 | 返回笔记上限 |
| `browser` | string | 否 | - | 指定使用的浏览器名称 |

**返回**：笔记数组的 JSON 字符串。

#### 推文结构

`twitter_*` 工具返回的单条推文字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 推文 ID |
| `text` | string | 推文正文 |
| `author` | string | 作者用户名（@screen_name） |
| `author_name` | string | 作者显示名 |
| `created_at` | string | 发布时间 |
| `favorite_count` | number | 点赞数 |
| `retweet_count` | number | 转推数 |
| `reply_count` | number | 回复数 |
| `quote_count` | number | 引用数 |
| `url` | string | 推文链接 |
| `media` | string[] | 媒体图片 URL（可选） |

---

## WebSocket 协议

### 连接端点

| 端点 | 用途 |
|------|------|
| `ws://{host}:{port}/ws_browser?conn_id={CONNECT_ID}&name={NAME}` | 浏览器扩展连接 |
| `ws://{host}:{port}/ws_command?conn_id={CONNECT_ID}` | 命令客户端连接 |

### 认证

**1. Connect ID 认证**

连接时必须携带 `conn_id` 查询参数，值必须与后端注册的 `CONNECT_ID` 一致。浏览器扩展端还需携带 `name`。

**2. 注册接口认证**

浏览器扩展建立 WebSocket 前会调用 `/api/browsers/register` 注册实例；该注册接口使用 `/api` middleware，支持 cookie 或 `GRABBY_API_TOKEN`。

### 命令一览

| 命令 | 说明 | 主要字段 |
|------|------|----------|
| `extract` | 提取网页正文为 Markdown | `url` |
| `capture` | 网页截图 | `url`、`fullPage` |
| `navigate` / `open` | 打开/导航到 URL | `url` |
| `intercept` | **新增**：早注入捕获页面 XHR/GraphQL 响应（X/Twitter 等） | `url`、`visible`、`closeTab`、`params` |
| `runPageScript` | **新增**：执行白名单页面脚本，读取页面 JS 变量/JSON-LD/meta | `url`、`params.script`、`params.params`、`visible`、`closeTab` |
| `fetchInPage` | **新增**：在页面上下文发 fetch（带登录态） | `url`、`params.requestUrl`、`params.method/headers/body/credentials`、`visible`、`closeTab` |

### 消息格式

#### 请求消息（服务端 → 浏览器）

```json
{
  "source": "mcp_client",
  "action": "mcp_request",
  "command": "intercept",
  "url": "https://x.com/search?q=...&f=top",
  "message_id": "msg-xxx",
  "visible": true,
  "closeTab": true,
  "params": { "scrollRounds": 6, "maxCaptures": 200 }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `source` | string | 消息来源标识 |
| `action` | string | 动作类型 |
| `command` | string | 命令类型：`extract` / `capture` / `navigate` / `open` / `intercept` / `runPageScript` / `fetchInPage` |
| `url` | string | 目标 URL |
| `fullPage` | boolean | 截图时是否截取全页面 |
| `message_id` | string | 消息唯一 ID，用于匹配响应 |
| `params` | object | **新增**：命令专属参数（如 `intercept` 的 `scrollRounds`、`maxCaptures`、`scrollDelayMs`） |
| `visible` | boolean | **新增**：是否短暂激活标签页（无限滚动页面需要），结束后恢复用户原标签页 |
| `closeTab` | boolean | **新增**：完成后是否关闭临时标签页 |
| `timeoutMs` | number | **新增**：单命令超时提示 |

> 兼容性：`params` / `visible` / `closeTab` / `timeoutMs` 为新增字段，仅新命令使用；旧命令与旧版扩展忽略它们，协议向后兼容。详见 `docs/browser-executor-plan.md` §6。

#### 响应消息（浏览器 → 服务端）

提取类（`extract`/`capture`）响应：

```json
{
  "type": "response",
  "message_id": "msg-xxx",
  "command": "extract",
  "success": true,
  "result": {
    "url": "https://example.com",
    "title": "Example Domain",
    "content": { "content": "# Markdown content...", "wordCount": 42 }
  }
}
```

捕获类（`intercept`）响应：

```json
{
  "type": "response",
  "message_id": "msg-xxx",
  "command": "intercept",
  "success": true,
  "result": {
    "url": "https://x.com/search?q=...",
    "timestamp": "2026-06-13T12:00:00Z",
    "items": [
      { "url": ".../SearchTimeline?...", "status": 200, "method": "fetch", "body": "{...graphql json...}", "ts": 1718000000000 }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 消息类型：`response` |
| `message_id` | string | 对应请求的 message_id |
| `command` | string | 对应请求的命令 |
| `success` | boolean | 是否成功 |
| `result` | object | 结果数据 |
| `result.content` | object | 提取类命令的正文（Markdown 等） |
| `result.imageData` | string | 截图类命令的 Base64 图片 |
| `result.items` | array | **新增**：`intercept` 捕获到的原始响应记录列表（服务端适配器再按 operation 过滤、解析） |
| `result.text` / `result.json` | string / object | **新增**：原始文本 / 结构化对象承载（`fetchInPage` 回响应体 `text` + `json.status`；`runPageScript` 对象结果回 `json`、非对象回 `text`） |
| `error` | string | 错误信息（失败时） |

---

## 错误码

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| `200` | 请求成功 |
| `400` | 请求参数错误 |
| `401` | API key 校验失败（未提供或值不匹配） |
| `403` | connect_id 验证失败 |
| `404` | 资源不存在（如 kick 的连接、item/source 未找到） |
| `409` | 浏览器注册冲突 |
| `500` | 服务端内部错误（数据库等） |
| `502` | 浏览器扩展执行错误 |
| `503` | 浏览器扩展未连接 |
| `504` | 请求超时 |

### WebSocket 关闭码

| 状态码 | 说明 |
|--------|------|
| `1000` | 正常关闭 |
| `1002` | API key 校验失败 |
| `4001` | connect_id 验证失败 |
