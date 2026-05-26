# Grabby

MCP is a distributed web content harvesting system consisting of a Chrome extension and Python backend service, designed for automated web content collection and processing.

## 项目背景

Grabby 旨在解决大规模网页内容采集的自动化需求，特别适用于数据分析、内容聚合和自动化工作流程场景。通过浏览器扩展与后端服务的协同工作，实现了高效可靠的网页内容获取解决方案。

## 快速开始

### 前置要求

- [uv](https://docs.astral.sh/uv/) (Python 包管理器)
- Chrome 浏览器

安装 uv:
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

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

# 或跨平台 Python 脚本
python start.py
```

**方式二：直接用 uv 运行**

```bash
cd python-server
uv run python main.py
```

`uv run` 会自动处理：读取 `pyproject.toml` → 创建虚拟环境 → 安装依赖 → 运行。
无需手动创建 venv 或安装依赖。

### 环境配置

所有配置集中在 `python-server/.env` 文件中，无需修改 Python 代码。

```bash
cd python-server
cp .env.example .env   # 首次使用，复制模板
# 编辑 .env 修改配置
```

`python-server/.env` 示例：

```bash
# 服务器监听地址
HOST=0.0.0.0

# 服务器监听端口
PORT=5040

# 浏览器扩展连接ID（与浏览器端 API 密钥必须一致）
CONNECT_ID=browser-tools

# API 密钥（用于 HTTP/WebSocket 端点保护，可选）
# 留空表示关闭 API key 校验（允许所有请求）
# 配置后，客户端需在请求头中携带 X-API-Key: <your_api_key>
API_KEY=

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
2. `python-server/.env` 文件
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
  http://localhost:5040/mcp
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

后端同时提供了 MCP (Model Context Protocol) 工具，可被 AI Agent 调用：

- `screenshot(url, fullPage=False, browser="")` - 捕获网页截图
- `extract(url, browser="")` - 提取网页内容
- `add(a, b)` - 计算两数之和
- `get_server_time()` - 获取服务器时间

## 项目架构

```
grabby/
├── python-server/       # Python 后端服务
│   ├── main.py          # FastAPI 主应用 + HTTP API
│   ├── websocket_manager.py  # WebSocket 连接管理
│   ├── config.py        # 配置文件
│   ├── handlers/        # 指令处理器
│   └── requirements.txt # Python 依赖
├── go-server/           # Go 后端服务
│   ├── main.go          # HTTP 入口 + MCP Server
│   ├── websocket_manager.go  # WebSocket 连接管理
│   ├── config.go        # 配置文件
│   └── types.go         # 类型定义
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
FastAPI Server
    |
    | WebSocket 发送 extract 命令
    v
Chrome Extension
    |
    | 打开 URL → 提取 HTML → 返回结果
    v
FastAPI Server
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

### 2. Python Backend Service

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
