# 使用指南

## 启动服务

### 方式一：启动脚本（推荐）

```bash
# macOS / Linux
./start.sh

# 或跨平台 Python 脚本
python start.py
```

> 注意：`start.sh` 和 `start.py` 默认启动 **Python 后端**。如需启动 Go 后端，请直接运行二进制文件。

### 方式二：直接运行

**Python 后端：**

```bash
cd python-server
uv run python main.py
```

**Go 后端：**

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
  http://localhost:5040/mcp       - MCP SSE 端点
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

后端同时提供 MCP (Model Context Protocol) 工具，可被 AI Agent 调用。

### 可用工具

| 工具名 | 参数 | 说明 |
|--------|------|------|
| `screenshot` | `url` (string, 必填), `fullPage` (boolean, 默认 false) | 捕获网页截图，返回 Base64 图片数据 |
| `extract` | `url` (string, 必填) | 提取网页内容，返回 Markdown 文本 |
| `add` | `a` (number), `b` (number) | 计算两数之和 |
| `get_server_time` | 无 | 获取服务器当前时间 |

### MCP SSE 端点

```
http://localhost:5040/mcp
```

### 使用示例（Claude Desktop 配置）

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json`（macOS）：

```json
{
  "mcpServers": {
    "web-capture": {
      "command": "uv",
      "args": [
        "run",
        "--directory",
        "/path/to/mcp-web-capture/python-server",
        "python",
        "main.py"
      ]
    }
  }
}
```

或使用 Go 后端：

```json
{
  "mcpServers": {
    "web-capture": {
      "command": "/path/to/mcp-web-capture/go-server/go-server"
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
2. 点击 Chrome 工具栏的 MCP 图标
3. 点击 **"提取内容"** 按钮
4. 查看提取结果

### 手动截图

1. 打开任意网页
2. 点击 MCP 图标
3. 选择截图模式（可见区域 / 全页面 / 选择区域）
4. 截图自动下载或发送到后端
