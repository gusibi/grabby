# Grabby

Grabby is a distributed web content harvesting system consisting of a Chrome extension and a Go backend service, designed for automated web content collection and processing.

## 项目背景

Grabby 旨在解决大规模网页内容采集的自动化需求，特别适用于数据分析、内容聚合和自动化工作流程场景。通过浏览器扩展与后端服务的协同工作，实现了高效可靠的网页内容获取解决方案。

## 快速开始

### 前置要求

- [Go](https://go.dev/doc/install) 1.21+
- Node.js（构建扩展与前端）
- Chrome 浏览器

### 安装

1. 克隆仓库

```bash
git clone https://github.com/your-repo/grabby.git
cd grabby
```

2. 加载浏览器扩展

- 打开 Chrome，访问 `chrome://extensions`
- 启用"开发者模式"
- 点击"加载已解压的扩展程序"，选择 `chrome-extension` 目录

## 使用说明

### 启动后端服务

**方式一：使用启动脚本（推荐）**

```bash
# macOS / Linux
./start.sh

# 跨平台（Windows 可用）
python start.py
```

两个脚本都只是在 `go-server/` 里执行 `go run .`。

**方式二：编译后运行**

```bash
make run-go      # 构建前端 + Go 二进制并启动
```

### 环境配置

所有配置集中在 `go-server/.env` 文件中，无需修改代码。

```bash
cd go-server
cp .env.example .env   # 首次使用，复制模板
# 编辑 .env 修改配置
```

`go-server/.env` 示例：

```bash
# 服务器监听地址
HOST=0.0.0.0

# 服务器监听端口
PORT=5040

# 管理后台登录密钥（留空表示不需要登录）
GRABBY_ADMIN_KEY=

# API / MCP 访问 token（留空表示不鉴权）
# 配置后，客户端需携带 X-API-Key / X-Grabby-Token / Authorization: Bearer <token>
GRABBY_API_TOKEN=

# 是否开启调试模式
DEBUG=false

# WebSocket 默认超时时间（秒）
WEBSOCKET_TIMEOUT=5.0

# HTTP API extract 端点超时时间（秒）
API_EXTRACT_TIMEOUT=60.0

# 默认浏览器名称（用于多浏览器场景，留空使用第一个连接）
DEFAULT_BROWSER=
```

配置加载优先级（高到低）：
1. 系统环境变量
2. `go-server/.env` 文件
3. 代码中的默认值

服务启动后，控制台会输出可用的端点地址：

```
HTTP API:
  POST http://localhost:5040/api/extract           - 提取网页 Markdown
  POST http://localhost:5040/api/browsers/register  - 注册浏览器实例
  GET  http://localhost:5040/api/browsers          - 查看已连接的浏览器列表
  GET  http://localhost:5040/api/health            - 健康检查

WebSocket:
  ws://localhost:5040/ws_browser  - 浏览器扩展连接
  ws://localhost:5040/ws_command  - 命令客户端连接

MCP Server:
  http://localhost:5040/mcp      - Streamable HTTP（需 GRABBY_API_TOKEN）
  http://localhost:5040/mcp/sse  - SSE（兼容旧客户端）
```

### 配置浏览器扩展

**打开设置：**

右键点击 Chrome 工具栏中的 Grabby 图标 → **选项 / Options**，打开扩展设置页面。

**填写连接信息：**

| 字段 | 值 | 说明 |
|------|-----|------|
| **WebSocket 服务器地址** | `ws://localhost:5040/ws_browser` | 服务器 WebSocket 端点 |
| **API 密钥** | `browser-tools` | 必须与服务器 `.env` 中的 `CONNECT_ID` 一致 |
| **浏览器名称** | `chrome-office` | 多浏览器场景下的标识名称（可选） |
| **启动时自动连接** | 勾选 | 推荐开启 |

![扩展设置示例](docs/extension-settings.png)

**认证原理：**

扩展连接时会把 **API 密钥** 作为 `conn_id` 参数附加到 WebSocket URL：

```
ws://localhost:5040/ws_browser?conn_id=browser-tools
```

服务器会验证 `conn_id` 是否匹配 `CONNECT_ID`。两端必须一致，否则连接会被拒绝（403）。

**保存并连接：**

1. 填写上述字段
2. 点击页面底部的 **保存设置**
3. 扩展自动尝试连接
4. 看到"连接状态：已连接"即表示成功

> 如果连接失败，检查：服务器是否已启动、WebSocket 地址和端口是否正确、API 密钥是否与服务器配置一致。

### HTTP API 使用

#### 提取网页内容为 Markdown

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

# 指定浏览器（多浏览器场景）
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{"url": "https://example.com", "browser": "chrome-office"}'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | 是 | 要提取的网页 URL |
| browser | string | 否 | 浏览器名称，为空时使用默认浏览器 |

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

- `503` - 浏览器扩展未连接
- `502` - 浏览器扩展执行错误
- `504` - 提取超时或连接断开
- `500` - 服务器内部错误

#### 注册浏览器实例

在浏览器扩展连接之前，需要先注册浏览器。

```bash
# API_KEY 未配置时
curl -X POST http://localhost:5040/api/browsers/register \
  -H "Content-Type: application/json" \
  -d '{"connect_id": "browser-tools", "name": "chrome-office"}'

# API_KEY 已配置时
curl -X POST http://localhost:5040/api/browsers/register \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{"connect_id": "browser-tools", "name": "chrome-office"}'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| connect_id | string | 是 | 浏览器连接标识，需与扩展的 API 密钥一致 |
| name | string | 是 | 浏览器名称，不可重复 |

**成功响应（200）：**

```json
{
  "success": true,
  "browser": {
    "connect_id": "browser-tools",
    "name": "chrome-office"
  }
}
```

**错误响应：**

- `400` - 缺少 connect_id 或 name
- `409` - connect_id 已注册但名称不同，或名称已被占用

#### 查看已连接的浏览器列表

```bash
curl http://localhost:5040/api/browsers
```

**响应：**

```json
{
  "browsers": [
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-home"},
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-office"}
  ],
  "count": 2
}
```

#### 健康检查

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
  "browser_count": 2,
  "browsers": [
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-home"},
    {"conn_id": "ws_browser:browser-tools", "name": "chrome-office"}
  ],
  "timestamp": "2026-05-25T12:00:00"
}
```

### 多浏览器并发

一台服务器可以同时连接多个 Chrome 实例，实现并行抓取：

1. 在每台 Chrome 的扩展设置中填入不同的**浏览器名称**（如 `chrome-home`、`chrome-office`）
2. 请求时通过 `browser` 参数指定使用哪个浏览器
3. 未指定时使用默认浏览器（可通过 `DEFAULT_BROWSER` 配置）

```bash
# 同时向两个浏览器发送请求（在终端中并行执行）
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://news.ycombinator.com", "browser": "chrome-home"}' &

curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "browser": "chrome-office"}' &

wait
```

### MCP 工具使用

后端在同一个 HTTP 端口上提供 MCP (Model Context Protocol) 服务，可被 AI Agent 直接连接：

- 浏览器类：`extract`、`screenshot`、`fetch_in_page`、`list_browsers`
- 平台类：`twitter_*`、`reddit_*`、`xiaohongshu_*`
- 内容库类：`library_search`、`library_get_item`、`library_list_sources`、`library_daily_report`、`library_stats`

完整参数与连接方式见 [docs/usage.md](docs/usage.md#mcp-工具使用)。

## 项目架构

```
grabby/
├── go-server/           # Go 后端服务（唯一后端）
│   ├── main.go          # 进程入口
│   ├── internal/
│   │   ├── domain/          # 领域模型（capture / item / source / ai / browser）
│   │   ├── application/     # 业务逻辑（scraping / ai / scheduler / twitter / reddit / xiaohongshu）
│   │   ├── infrastructure/  # 基础设施（browserws / sqlite / llm / browserregistry）
│   │   └── interfaces/      # 对外接口（http / websocket / mcp / dto）
│   └── frontend/        # React 管理界面（编译后嵌入二进制）
├── go-cli/              # Go 版命令行客户端
├── python-cli/          # Python 版命令行客户端（免编译）
├── chrome-extension/    # Chrome 浏览器扩展
│   ├── background.js    # 后台服务脚本
│   ├── lib/
│   │   ├── extractor.js # 内容提取逻辑
│   │   ├── websocket.js # WebSocket 客户端
│   │   └── capture.js   # 截图逻辑
│   └── manifest.json    # 扩展配置
├── start.sh             # 启动脚本 (macOS/Linux)
├── start.py             # 启动脚本 (跨平台)
└── readme.md            # 本文档
```

## 数据流

```
用户/Agent
    |
    | POST /api/extract {url}
    v
Go Server
    |
    | WebSocket 发送 extract 命令
    v
Chrome Extension
    |
    | 打开 URL → 提取 HTML → 返回结果
    v
Go Server
    |
    | HTML → Markdown 转换
    v
JSON 响应 {url, title, markdown}
```

## 贡献指南

我们欢迎各种形式的贡献，包括但不限于：
- 报告问题
- 提交功能请求
- 代码贡献

请遵循以下步骤：
1. Fork本项目
2. 创建您的功能分支
3. 提交您的修改
4. 推送分支并创建Pull Request

## TODO

- [x] HTTP API 提取网页 Markdown
- [x] 启动脚本
- [ ] 提取规则配置

## 许可证

本项目采用MIT许可证。

## Components

### 1. Chrome Browser Extension

**Core Features:**
- **WebSocket Connection Management**
  - Persistent WebSocket connection with remote service
  - Real-time command and target URL reception
  - Visual connection status display

- **Web Content Processing**
  - Automated URL navigation
  - Full-page or selective area screenshots
  - Intelligent content extraction (similar to Web Clipper)
  - Result transmission via WebSocket

- **Configuration Management**
  - User-friendly settings interface
  - WebSocket server address configuration
  - Security key setup
  - Local image storage path configuration
  - Custom content extraction rules

### 2. Go Backend Service

**Core Features:**
- **Communication Protocol**
  - WebSocket server implementation (multi-client support)
  - Custom MCP protocol for request/response handling
  - HTTP REST API for external integration

- **Command Management**
  - `capture` command: Takes URL, returns screenshot file path
  - `extract` command: Takes URL, returns structured content
  - Task queue and status tracking

- **HTTP API**
  - `POST /api/extract` - Extract webpage as Markdown
  - `GET /api/health` - Health check endpoint

- **Data Processing**
  - Image storage management
  - Content parsing and formatting
  - HTML to Markdown conversion
  - Extensible plugin system for custom processing

- **Security**
  - Client authentication
  - Data transmission encryption
  - Access control and rate limiting

This project provides an efficient, reliable solution for web content collection, ideal for data analysis, content aggregation, and workflow automation.
