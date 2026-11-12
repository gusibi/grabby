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

### 桌面 APP（一条命令）

```bash
make app          # 构建 → 打包 Grabby.app → 启动
make app-log      # 跟踪日志（~/Library/Logs/Grabby/desktop.log）
make app-stop     # 停止
```

APP 进程内自带 HTTP / WebSocket / MCP 服务，**不需要另开终端跑 server**。窗口关掉后它仍驻留在菜单栏，从菜单栏图标「退出 Grabby」才算真的退出。默认不校验 token（loopback 直接放行），需要在 APP 的「设置 → API 与安全」里自行设置。

调试时用 `make run-desktop`，日志会直接打在终端。

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

## 桌面 APP（MyGO）

Grabby 另有一个独立的桌面应用，使用 [MyGO](https://github.com/egoist/mygo) v0.8.3。
**现有 Web 管理页面保持原样**，APP 使用另一套独立界面 `go-server/desktop/dist/`：
macOS 为 Apple News 风格（沉浸式半透明侧栏、大标题、卡片流），Windows 为 Fluent / Windows 新闻风格，
两者都跟随系统深浅色。界面是运行在系统 WebView 中的 HTML/CSS/JS（CSS 仿真平台外观，
不是系统原生控件）；进程、托盘、单实例、数据库和 API 服务则是原生 Go 二进制。

### APP 功能

APP 复用 Web 版同一套后端接口，侧栏分四组：

| 页面 | 功能 | 主要接口 |
|---|---|---|
| 今天 | 统计、扩展连接状态、最新内容 | `/open/api/health`、`/open/api/stats`、`/open/api/items` |
| 内容库 | 服务端搜索、全部 / 已收藏 / AI 优质筛选、分类筛选、详情、收藏、已读 | `/open/api/items`、`/open/api/ai/items`、`/api/items/:id/*` |
| RSS 源 | 订阅源增删改、启停、单条与批量立即抓取、抓取日志、分组 | `/api/sources`、`/api/sources/:id/toggle|run`、`/api/logs` |
| 新闻源 | 来源分组跳转、X / Reddit / 小红书 / 网页提取采集、提取归档 | `/api/twitter/*`、`/api/reddit/*`、`/api/xiaohongshu/*`、`/api/extract`、`/api/captures/extract` |
| AI 分析 | 队列统计、当前分析配置摘要、连通性测试、全量评测、优质内容、单条重分析 | `/open/api/ai/stats`、`/api/ai/test`、`/api/ai/start_eval`、`/open/api/ai/*` |
| AI 日报 | 按日期 / 类型查看、生成、补生成近 7 天、历史列表、RSS 订阅地址 | `/open/api/ai/daily`、`/open/api/ai/daily/list`、`/api/ai/daily/generate` |
| 浏览器设备 | 在线状态、断开 / 禁用 / 解禁、扩展连接信息 | `/api/browsers*` |
| API 接入 | Base URL、MCP、鉴权、中转链路、端点清单与 curl 示例 | — |
| 设置 | 六个分区：服务状态、AI 模型、提示词、日报定时、API 与安全、数据与文件 | `/api/ai/settings`、`/api/desktop/settings`、`/api/ai/test` |

**设置页（参考 Web 的 AISettings 布局）**用编号分区 + 顶部快速跳转，全部配置集中在一页：

1. **01 服务状态** — 进程、监听地址、本机免密状态、Token 状态、扩展 WebSocket 地址；
2. **02 AI 模型** — 启用开关、配置档选择 / 新建（对应后端的 provider profiles）、服务商预设
   （OpenAI / DeepSeek / Gemini / OpenRouter / LM Studio / 自定义）、服务商、模型、Base URL、
   API Key、每分钟请求上限、优质阈值、多模型策略；
3. **03 提示词** — 系统提示词与日报提示词，标注可用变量；
4. **04 日报定时** — 早报 / 晚报开关与时间，可在本页直接测试模型连通性；
5. **05 API 与安全** — Token 开关 / 生成 / 保存、局域网访问开关、鉴权说明；
6. **06 数据与文件** — 数据目录、`grabby.db`、`desktop-settings.json`、curl 示例。

模型配置必须写在「配置档（profile）」里：后端会用当前配置档回填 provider / model / base_url，
所以设置页直接编辑选中的配置档并把它设为当前使用，保存后立即对新分析任务生效。
切换配置档会重新载入该档的字段；新建配置档从空白表单开始，不会复制已有的 Key。

写操作走 `/api/*`，读操作走 `/open/api/*`。**APP 内不需要任何 token**：服务与本机调用（WebView、CLI、MCP、扩展）
全部走 loopback，永远免密，也不会弹登录框；Token 只用于保护来自其他设备的请求，默认关闭。
启动到指定页面可加 `?view=sources`（`today|library|sources|news|ai|daily|devices|apihub|settings`）。

APP 不只是界面：完整的 HTTP / WebSocket / MCP 服务仍然跑在同一个 Go 进程里，
所以它本身就是一个 **API 中转站**：

```
外部调用 POST /api/extract ──> APP 内的 Go server ──> ws://localhost:5040/ws_browser ──> Chrome 扩展 ──> 目标网页
```

### 构建与运行

```bash
make build-desktop       # 生成 go-server/grabby-desktop
make run-desktop         # 构建并启动
make package-macos-app   # 生成 dist/Grabby.app（未签名，本机可用）
```

打包之后**直接双击 `dist/Grabby.app` 就能用**：服务随 APP 一起启动，不需要终端，也不需要另开进程；
仓库里打出来的这个 bundle 会自动复用 `go-server/` 的数据库与 `.env`，订阅源和内容库都在。

MyGO 要求 Go 1.27.3+（`go-server/go.mod` 已升级）。若本机设置了 `GOSUMDB=off`，
Go 自动切换 toolchain 会失败，Makefile 的 desktop target 已临时指定 `GOSUMDB=sum.golang.org`。

### 配置

**打开 APP 即自动启动服务**，不需要终端，也不需要额外的进程：HTTP / WebSocket / MCP 服务就运行在 APP 进程内。

数据目录按以下顺序确定，不需要手动配置：

1. `GRABBY_DESKTOP_DATA_DIR` 环境变量（显式指定，优先级最高）；
2. 上次启动记住的目录（`~/Library/Application Support/Grabby/data-dir.txt`）；
3. 可执行文件附近的 `go-server/` 目录 —— 所以仓库里 `make package-macos-app` 产出的
   `dist/Grabby.app` 双击后会**直接复用项目现有的 `grabby.db` 和 `.env`**，订阅源、内容库都在；
4. 系统用户数据目录（macOS: `~/Library/Application Support/Grabby/`），用于安装到 `/Applications` 的独立副本。

实际使用的目录会在「设置 → 数据与文件」里显示。

| 环境变量 | 默认 | 说明 |
|---|---|---|
| `GRABBY_DESKTOP_DATA_DIR` | 自动识别 | 强制指定数据目录 |
| `GRABBY_DESKTOP_HOST` | `127.0.0.1` | 设为 `0.0.0.0` 可让其他设备访问 API；无 Token 时自动回退为本机 |
| `PORT` | `5040` | 被占用时自动顺延，实际地址显示在「设置 → 服务状态」与托盘菜单 |

Token 与对外访问**不读环境变量，也不读 `.env`**，只在 APP 的「设置 → API 与安全」里配置并保存到
`desktop-settings.json`（与 `grabby.db` 同级，`0600` 权限）。Token 保存后立即生效；
监听范围改变需要重启 APP。开启对外访问时必须先设置 Token，否则保存会被拒绝。

`.env` 里遗留的 `GRABBY_ADMIN_KEY` / `GRABBY_API_TOKEN` 对 APP 无效（启动时被清空），
所以不会出现「一打开就要求登录」的情况；Web 版与命令行的鉴权行为完全不变。

### 行为

- 关闭窗口**不会退出**：APP 留在托盘，API 服务继续运行；从托盘选择「退出」才会优雅关闭调度器、任务队列和数据库。
- 单实例：重复启动会把已运行的窗口带到前台。
- 浏览器扩展连接到 APP 显示的端口（默认 `ws://localhost:5040/ws_browser`）；端口顺延时需在扩展设置里同步修改。
- 端口顺延时 APP 仍然提供自己的 UI，不会退回 Web 页面。

### 已知限制

- MyGO 只支持桌面（macOS 12+ / Windows 10+ / Linux），不支持 iOS / Android；依赖系统 WebView，Linux 需 `libwebkit2gtk-4.1-0`，Windows 10 需 WebView2 Runtime。
- `dist/Grabby.app` 未签名公证，对外分发需走 MyGO 的打包签名流程或自行签名。
- Chrome 扩展仍需单独安装（登录态在真实浏览器里），APP 不会替你安装。
- **对外监听时 HTTP 与 WebSocket 都会校验 Token**：远端 HTTP 请求带 `X-API-Key`（或 `Authorization: Bearer`），
  远端 WS 握手带 `?token=`。本机 loopback 请求始终免密，所以 Chrome 扩展、CLI、MCP 客户端与 APP 自身不受影响，
  Web 版的鉴权行为也没有变化。`CheckOrigin` 仍为允许，不要把 Token 长期暴露在不可信网络里。

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
