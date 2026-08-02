# 使用指南

## 启动服务

### 方式一：启动脚本（推荐）

```bash
# macOS / Linux
./start.sh

# 跨平台（含 Windows）
python start.py
```

两个脚本都只是在 `go-server/` 里执行 `go run .`。

### 方式二：直接运行

```bash
cd go-server
./go-server
```

---

## 服务启动后

控制台会输出可用端点：

```
HTTP API:
  POST http://localhost:5040/api/extract  - 提取网页 Markdown
  GET  http://localhost:5040/api/health   - 健康检查

WebSocket:
  ws://localhost:5040/ws_browser  - 浏览器扩展连接
  ws://localhost:5040/ws_command  - 命令客户端连接

MCP Server:
  http://localhost:5040/mcp       - MCP Streamable HTTP 端点
  http://localhost:5040/mcp/sse   - MCP SSE 端点（兼容）
```

---

## HTTP API 使用

### 提取网页内容为 Markdown

```bash
# API_KEY 未配置时
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'

# API_KEY 已配置时
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{"url": "https://example.com"}'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 要提取的网页 URL |

**成功响应（200）：**

```json
{
  "success": true,
  "url": "https://example.com",
  "title": "Example Domain",
  "markdown": "# Example Domain\n\nThis domain is for use in illustrative examples..."
}
```

**错误响应：**

| 状态码 | 说明 |
|--------|------|
| `503` | 浏览器扩展未连接 |
| `502` | 浏览器扩展执行错误 |
| `504` | 提取超时或连接断开 |
| `500` | 服务器内部错误 |

### 健康检查

```bash
# API_KEY 未配置时
curl http://localhost:5040/api/health

# API_KEY 已配置时
curl -H "X-API-Key: your_api_key" http://localhost:5040/api/health
```

**响应：**

```json
{
  "status": "ok",
  "browser_connected": true,
  "timestamp": "2026-05-25T12:00:00Z"
}
```

---

## MCP 工具使用

后端在 HTTP 端口上同时提供 MCP (Model Context Protocol) 服务，Agent 可以直接连接。

### 端点

| 端点 | 传输方式 | 说明 |
|------|----------|------|
| `http://localhost:5040/mcp` | Streamable HTTP | 推荐，当前标准传输 |
| `http://localhost:5040/mcp/sse` | SSE（+ `/mcp/message`） | 旧客户端兼容 |

配置了 `GRABBY_API_TOKEN`（或 `GRABBY_ADMIN_KEY`）时，MCP 端点必须带 token，
支持三种请求头：`Authorization: Bearer <token>`、`X-API-Key`、`X-Grabby-Token`。
未配置 token 时端点不鉴权（仅建议在本机使用）。

### 可用工具

浏览器类（需要浏览器扩展在线）：

| 工具名 | 参数 | 说明 |
|--------|------|------|
| `extract` | `url`(必填), `browser` | 提取网页正文，返回带元信息头的 Markdown |
| `screenshot` | `url`(必填), `fullPage`, `browser` | 网页截图，返回 Base64 图片数据 |
| `fetch_in_page` | `url`(必填), `requestUrl`, `method`, `body`, `credentials`, `browser` | 在已登录页面上下文里发请求，复用站点 Cookie |
| `list_browsers` | 无 | 列出当前在线的浏览器，供其他工具的 `browser` 参数使用 |

平台类（复用已登录会话）：

| 工具名 | 参数 | 说明 |
|--------|------|------|
| `twitter_search` / `twitter_timeline` / `twitter_likes` | `query`/`kind`/`handle`, `limit`, `browser` | X/Twitter 结构化抓取 |
| `reddit_thread` / `reddit_subreddit` / `reddit_search` | `url`/`subreddit`/`query`, `sort`, `limit`, `browser` | Reddit 帖子与评论树 |
| `xiaohongshu_note` / `xiaohongshu_search` / `xiaohongshu_user_notes` | `url`/`query`, `limit`, `browser` | 小红书笔记与评论 |

内容库类（只读本地数据库，浏览器离线也能用）：

| 工具名 | 参数 | 说明 |
|--------|------|------|
| `library_search` | `query`, `category`, `source_category`, `origin`, `starred`, `unread_only`, `limit`, `cursor` | 检索已收集的条目（只返回元信息） |
| `library_get_item` | `id`(必填) | 按 id 取单条完整 Markdown |
| `library_list_sources` | 无 | 列出已配置的采集源及其状态 |
| `library_daily_report` | `date`, `report_type` | 取 AI 日报 Markdown，默认最新一期 |
| `library_stats` | 无 | 条目总数 / 未读 / 收藏 / 分类分布 |

### 使用示例（Claude Code）

```bash
claude mcp add --transport http grabby http://localhost:5040/mcp \
  --header "Authorization: Bearer $GRABBY_API_TOKEN"
```

### 使用示例（Claude Desktop 配置）

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json`（macOS）：

```json
{
  "mcpServers": {
    "grabby": {
      "type": "http",
      "url": "http://localhost:5040/mcp",
      "headers": {
        "Authorization": "Bearer <GRABBY_API_TOKEN>"
      }
    }
  }
}
```

---

## 命令行客户端

可以直接通过 WebSocket 发送命令：

```bash
# 使用 wscat（需 npm install -g wscat）
wscat -c "ws://localhost:5040/ws_command?conn_id=browser-tools"

# 发送提取命令
> {"command":"extract","url":"https://example.com","message_id":"1"}

# 发送截图命令
> {"command":"capture","url":"https://example.com","fullPage":true,"message_id":"2"}
```

---

## 使用流程示例

### 完整提取流程

```bash
# 1. 确认浏览器扩展已连接
curl http://localhost:5040/api/health
# 或（配置了 API_KEY 时）
curl -H "X-API-Key: your_api_key" http://localhost:5040/api/health
# → 确认 browser_connected: true

# 2. 提取网页内容
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://news.ycombinator.com"}'
# 或（配置了 API_KEY 时）
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{"url": "https://news.ycombinator.com"}'

# 3. 查看返回的 Markdown
```

### 后台行为

当你发送 extract 请求时：

1. 后端通过 WebSocket 通知浏览器扩展
2. 扩展**新建标签页**并访问目标 URL
3. 等待页面加载完成并稳定
4. 使用 defuddle 解析 DOM，输出 Markdown
5. 发送结果回后端
6. **自动关闭临时标签页**
7. 后端将 Markdown 返回给调用方

---

## 浏览器扩展使用

### 手动提取当前页面

1. 打开任意网页
2. 点击 Chrome 工具栏的 Grabby 图标
3. 点击 **"提取内容"** 按钮
4. 查看提取结果

### 手动截图

1. 打开任意网页
2. 点击 Grabby 图标
3. 选择截图模式（可见区域 / 全页面 / 选择区域）
4. 截图自动下载或发送到后端
