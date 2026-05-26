# API 参考

## 概述

MCP Web Capture 提供两套接口：

1. **HTTP REST API** — 用于外部系统直接调用
2. **MCP (Model Context Protocol)** — 用于 AI Agent 集成
3. **WebSocket** — 浏览器扩展与后端的实时通信

---

## HTTP API

### 基础信息

| 项目 | 值 |
|------|-----|
| 基础 URL | `http://{HOST}:{PORT}` |
| 默认地址 | `http://localhost:5040` |
| 内容类型 | `application/json` |

### 认证

当后端配置了 `API_KEY` 时，所有 HTTP API 和 WebSocket 请求需在 header 中携带 `X-API-Key`：

```
X-API-Key: your_api_key
```

`API_KEY` 留空时关闭认证，允许所有请求。

---

### POST /api/extract

提取指定 URL 的网页内容并返回 Markdown。

#### 请求

```http
POST /api/extract HTTP/1.1
Content-Type: application/json
X-API-Key: your_api_key

{
  "url": "https://example.com"
}
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 要提取的网页 URL |

#### 成功响应 (200)

```json
{
  "success": true,
  "url": "https://example.com",
  "title": "Example Domain",
  "markdown": "# Example Domain\n\nThis domain is for use in illustrative examples..."
}
```

#### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否成功 |
| `url` | string | 实际提取的 URL |
| `title` | string | 网页标题 |
| `markdown` | string | 提取的 Markdown 内容 |

#### 错误响应

**503 Service Unavailable**
```json
{"error": "Browser extension not connected"}
```

**502 Bad Gateway**
```json
{"error": "Browser extension error: ..."}
```

**504 Gateway Timeout**
```json
{"error": "Extract timeout or connection lost"}
```

---

### GET /api/browsers

获取当前已连接的浏览器列表。

#### 请求

```http
GET /api/browsers HTTP/1.1
X-API-Key: your_api_key
```

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

#### 响应字段

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
X-API-Key: your_api_key

{
  "connect_id": "browser-tools",
  "name": "chrome-office"
}
```

#### 请求参数

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

#### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | boolean | 是否注册成功 |
| `browser.connect_id` | string | 注册的浏览器连接 ID |
| `browser.name` | string | 注册的浏览器名称 |

#### 错误响应

**400 Bad Request**
```json
{"error": "connect_id is required"}
```
```json
{"error": "name is required"}
```

**409 Conflict** — connect_id 已注册但名称不同，或名称已被其他 connect_id 占用
```json
{"error": "browser registration conflict"}
```

---

### GET /api/health

健康检查端点。

#### 请求

```http
GET /api/health HTTP/1.1
X-API-Key: your_api_key
```

#### 成功响应 (200)

```json
{
  "status": "ok",
  "browser_connected": true,
  "timestamp": "2026-05-25T12:00:00Z"
}
```

#### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 服务状态，`ok` 表示正常 |
| `browser_connected` | boolean | 浏览器扩展是否已连接 |
| `timestamp` | string | ISO 8601 格式时间戳 |

---

## MCP 工具

MCP Server 挂载在 `http://localhost:5040/mcp`，使用 SSE (Server-Sent Events) 传输。

### tool: screenshot

捕获指定网页的截图。

#### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要截图的网页 URL |
| `fullPage` | boolean | 否 | false | 是否截取整个页面 |

#### 返回

Base64 编码的 PNG 图片数据（data URL 格式）。

#### 示例

```json
{
  "url": "https://example.com",
  "fullPage": true
}
```

---

### tool: extract

提取指定网页的内容并返回 Markdown。

#### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 要提取的网页 URL |

#### 返回

Markdown 格式的网页内容。

#### 示例

```json
{
  "url": "https://example.com/article"
}
```

#### 返回示例

```markdown
# Article Title

This is the main content of the article...

## Section 1

Some content here.

```python
code block example
```
```

---

### tool: add

计算两个数字的和。

#### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `a` | number | 是 | - | 第一个数字 |
| `b` | number | 是 | - | 第二个数字 |

#### 返回

两数之和。

---

### tool: get_server_time

获取服务器当前时间。

#### 参数

无。

#### 返回

ISO 8601 格式的时间字符串。

---

## WebSocket 协议

### 连接端点

| 端点 | 用途 |
|------|------|
| `ws://{host}:{port}/ws_browser?conn_id={CONNECT_ID}` | 浏览器扩展连接 |
| `ws://{host}:{port}/ws_command?conn_id={CONNECT_ID}` | 命令客户端连接 |

### 认证

**1. Connect ID 认证**

连接时必须携带 `conn_id` 查询参数，值必须与后端配置的 `CONNECT_ID` 一致。

**2. API Key 认证**

当后端配置了 `API_KEY` 时，WebSocket 握手请求还需在 header 中携带 `X-API-Key`。校验失败时连接会被拒绝（HTTP 401），关闭码为 `1002`。

### 消息格式

#### 请求消息（服务端 → 浏览器）

```json
{
  "source": "mcp_client",
  "action": "mcp_request",
  "command": "extract",
  "url": "https://example.com",
  "fullPage": false,
  "message_id": "msg-xxx"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `source` | string | 消息来源标识 |
| `action` | string | 动作类型 |
| `command` | string | 命令类型：`extract` / `capture` / `navigate` |
| `url` | string | 目标 URL |
| `fullPage` | boolean | 截图时是否截取全页面 |
| `message_id` | string | 消息唯一 ID，用于匹配响应 |

#### 响应消息（浏览器 → 服务端）

```json
{
  "type": "response",
  "message_id": "msg-xxx",
  "command": "extract",
  "success": true,
  "result": {
    "url": "https://example.com",
    "title": "Example Domain",
    "content": {
      "content": "# Markdown content...",
      "author": "",
      "published": "",
      "wordCount": 42
    }
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
| `502` | 浏览器扩展执行错误 |
| `503` | 浏览器扩展未连接 |
| `504` | 请求超时 |

### WebSocket 关闭码

| 状态码 | 说明 |
|--------|------|
| `1000` | 正常关闭 |
| `1002` | API key 校验失败 |
| `4001` | connect_id 验证失败 |
