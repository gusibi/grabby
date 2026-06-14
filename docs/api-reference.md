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

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://example.com",
  "title": "Example Domain",
  "markdown": "# Example Domain\n\nThis domain is for use in illustrative examples..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际提取的 URL |
| `title` | string | 网页标题 |
| `markdown` | string | 提取的 Markdown 内容 |

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
| `command` | string | 命令类型：`extract` / `capture` / `navigate` / `open` / `intercept` |
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
| `result.text` / `result.json` | string / object | **新增**：原始文本 / 结构化对象承载（预留给 `fetchInPage` / `runPageScript`） |
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
