# Grabby

<p align="center"><img src="assets/logo-512.png" width="128" alt="Grabby logo"></p>

<p align="center"><b>本地优先的浏览器采集 + AI 阅读台</b></p>

Grabby 把「用真实浏览器抓网页」和「AI 帮你筛选、总结、归档」放在同一个本地应用里：
Chrome 扩展负责在带登录态的浏览器中抓取内容，Go 服务负责存储、调度与 AI 分析，
桌面 App 提供完整的阅读与管理界面。数据默认只保存在本机 SQLite，不需要云端账号。

- **桌面 App**：[MyGO](https://github.com/egoist/mygo) v0.8.3（macOS / Windows / Linux，系统 WebView）
- **后端 / CLI**：Go（HTTP、WebSocket、MCP 与命令行）
- **扩展 / Web 界面**：JavaScript、React

同一个进程同时提供 HTTP / WebSocket / MCP 服务，所以它既是桌面 App，也是一个
**浏览器自动化 API 中转站**：外部请求进来后，由它转发给 Chrome 扩展在真实浏览器里执行。

## 功能

### 采集

- **网页提取**：任意 URL → 干净 Markdown / 截图，保留登录态，支持多浏览器路由。
- **订阅源**：RSS / API / 网页抓取源，cron 定时、启停、立即抓取、抓取日志、分组管理。
- **平台采集**：X（搜索 / 时间线 / 喜欢）、Reddit（板块 / 搜索 / 帖子）、小红书、Google Search Console。
- **浏览器扩展**：WebSocket 长连接、连接状态图标、按页面一键采集、自动重连。

### 阅读与管理

- **今天**：当日统计、扩展连接状态、今日日报摘要、最新内容。
- **内容库**：服务端搜索；全部 / 已收藏 / AI 优质；来源、内容类型、AI 分类筛选；
  分页加载；详情阅读、收藏、标记已读、单条重新分析。
- **多设备**：一个服务可同时连接多台 Chrome，按名称路由抓取任务。

### AI

- **多服务商 / 多配置档**：OpenAI、Anthropic、Google、DeepSeek、OpenRouter、LM Studio、Ollama 或自定义
  `base_url`；内置 [models.dev](https://models.dev) 模型目录（价格、上下文、能力）。
- **自动分析**：新内容自动进入队列，输出质量评分、分类、摘要、点评与标签；按阈值筛出「AI 优质」。
- **多模型策略**：单模型、轮询、故障转移，可限制每分钟请求数。
- **AI 日报**：早报 / 晚报定时生成，支持按日期与类型回看、补生成近 7 天、RSS 订阅。
- **可观测**：近 7 天调用次数、Token 用量、平均延迟；支持全量评测与单条重分析。

### 桌面体验

- macOS / Windows / Linux，跟随系统深浅色；关闭窗口后驻留托盘，服务继续运行。
- 单实例：重复启动会把已有窗口带到前台。
- 设置页集中管理服务状态、AI 模型、提示词、日报定时、API 与安全、数据与文件。
- 配置导出 / 导入（AI 配置 + 订阅源），换设备或重装后一键恢复。
- 本机 loopback 请求始终免密；需要给局域网其他设备调用时再开启 Token。

## 快速开始

### 1. 安装桌面 App

从 [Releases](https://github.com/gusibi/grabby/releases) 下载对应平台的安装包。

安装包是**单个自包含的可执行文件**：Go 服务端、HTTP / WebSocket / MCP、SQLite 和桌面界面全部打包在一起，
不需要再安装 Go、Node、数据库或单独启动后端。唯一的外部依赖是系统自带的 WebView
（Windows 的 WebView2、Linux 的 WebKitGTK 4.1）。

#### macOS（DMG，Apple Silicon + Intel）

1. 下载 `Grabby-<版本>-macOS-universal.dmg`，双击打开；
2. 把窗口里的 **Grabby** 拖到 **Applications**（窗口内有 Applications 快捷方式）；
3. 在 Finder 侧栏推出磁盘映像；
4. 第一次打开时，因为安装包没有 Apple 公证，系统可能提示
   **「无法打开 Grabby，因为 Apple 无法验证此 App 是否包含恶意软件」**。任选一种方式放行：
   - 在 Finder 里**右键点击 Grabby →「打开」**，在弹窗中再点「打开」；
   - 或打开 **系统设置 → 隐私与安全性**，在页面底部点 **「仍要打开」**；
   - 或用终端执行一次：
     ```bash
     xattr -dr com.apple.quarantine /Applications/Grabby.app
     ```
   放行一次后，之后就可以正常双击启动。

系统要求 macOS 12+；DMG 内是 arm64 + x86_64 通用二进制。

#### Windows（安装包 / 免安装 zip）

1. 下载 `Grabby-<版本>-Windows-amd64-setup.exe`（ARM 设备下载 `arm64` 版本）并双击；
2. 如果出现 **「Windows 已保护你的电脑」**（SmartScreen，因为安装包未签名）：
   点 **「更多信息」→「仍要运行」**；
3. 选择安装目录（默认 `%LOCALAPPDATA%\Programs\Grabby`，当前用户安装，**不需要管理员权限**），
   按提示完成；
4. 完成页会勾选 **「运行 Grabby」**，也可以从开始菜单或桌面快捷方式启动。

也可以下载免安装的 `Grabby-<版本>-Windows-amd64.zip`：解压后双击 `Grabby.exe`，
SmartScreen 提示的处理方式相同。建议放在一个固定目录，避免从临时下载目录运行。

- 卸载：**设置 → 应用 → 已安装的应用 → Grabby → 卸载**；
- Windows 11 自带 WebView2；部分 Windows 10 需要先安装
  [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。

#### Linux（.deb / tar.gz）

**Debian / Ubuntu**

```bash
sudo apt install ./grabby_<版本>_amd64.deb     # arm64 换成对应的包名
```

也可以在文件管理器里双击 `.deb`，用软件中心安装。安装后从应用菜单启动 Grabby。
`.deb` 会自动声明并安装 GTK3、WebKitGTK 4.1 与托盘库依赖。

**其他发行版（tar.gz）**

```bash
tar -xzf Grabby-<版本>-Linux-amd64.tar.gz
cd Grabby-<版本>-Linux-amd64
./grabby
```

需要先安装 GTK3 与 WebKitGTK 4.1，例如：

- Arch / Manjaro：`sudo pacman -S gtk3 webkit2gtk-4.1 libayatana-appindicator`
- Fedora：`sudo dnf install gtk3 webkit2gtk4.1 libayatana-appindicator-gtk3`
- openSUSE：`sudo zypper install gtk3 webkit2gtk-4_1 libayatana-appindicator3-1`

#### 首次启动

- App 会在 `127.0.0.1` 自动启动内置服务（默认端口 `5040`；被占用时自动顺延到 `5041`、`5042`…），
  不需要终端，也不需要另开进程；窗口关闭后驻留托盘，服务继续运行。
- 默认数据目录（`grabby.db`、配置、日志）：
  - macOS：`~/Library/Application Support/Grabby`
  - Windows：`%AppData%\Grabby`
  - Linux：`~/.config/Grabby`
  如果 App 是从仓库的 `dist/` 里运行，会优先复用项目里的 `go-server/` 数据目录。
- 接下来按「2. 安装并连接 Chrome 扩展」和「3. 配置 AI」完成初始化；
  没有连接扩展时仍可浏览已有内容，但采集和 AI 分析需要扩展或订阅源。

#### 从源码构建

需要 Go 1.27.3+、Node.js 22+（仅构建 Web 界面时需要）。

```bash
git clone https://github.com/gusibi/grabby.git
cd grabby
make app          # 构建 → 打包 dist/Grabby.app → 启动
```

也可以只用 `make build-desktop` 编译出 `go-server/grabby-desktop`，
或 `make run-desktop` 前台启动并在终端看日志。

系统要求：macOS 12+ / Windows 10+（WebView2）/ Linux（`libwebkit2gtk-4.1-0`）。

### 2. 安装并连接 Chrome 扩展

扩展是独立安装的（它需要在带登录态的真实浏览器里运行），有两种来源：

- **从 Release 安装**：下载 `grabby-extension-<版本>.zip` 并解压；
- **从源码安装**：直接使用仓库里的 `chrome-extension/` 目录。

1. 打开 `chrome://extensions`，开启「开发者模式」。
2. 点击「加载已解压的扩展程序」，选择解压后的扩展目录（或仓库里的 `chrome-extension/`）。
3. 右键工具栏中的 Grabby 图标 →「选项」，填写：
   - **WebSocket 服务器地址**：`ws://localhost:5040/ws_browser`
   - **API 密钥**：`browser-tools`（需与服务端 `CONNECT_ID` 一致）
   - **浏览器名称**（可选）：多浏览器场景下用于区分，如 `chrome-office`
4. 保存后扩展会自动连接；App 的「今天」页会显示「扩展已连接」，侧栏底部也会显示在线设备数。

> 端口被占用时 App 会自动顺延到 5041、5042…，实际地址以「设置 → 服务状态」为准，
> 扩展里的地址需要同步修改。

### 3. 配置 AI

打开 App →「设置 → AI 模型」：

1. 选择服务商预设（或自定义），填写 API Key、Base URL、模型名称；
   模型可以逗号分隔填写多个，配合「多模型策略」使用。
2. 点击「测试模型连通性」确认可用后保存，新任务立即生效。
3. 可在「提示词」里调整分析/日报提示词，在「日报定时」里开关早报与晚报。

### 4. 开始使用

- 在「RSS 源」添加订阅源（RSS / API / 网页），或到「新闻源」做一次平台采集；
- 新内容会自动进入 AI 队列，「AI 分析」页查看评分、分类与优质内容；
- 「今天」查看当日概览和今日日报，「内容库」按来源 / 类型 / AI 分类筛选阅读；
- 「AI 日报」按日期与类型查看历史日报，或补生成近 7 天。

### 其他运行方式

- **Web 管理界面**：`./start.sh`（macOS/Linux）或 `python start.py`，然后打开 `http://localhost:5040`。
  也可以 `make run-go` 先构建前端再启动。
- **CLI**：`go-cli/`（Go）或 `python-cli/`（Python），通过本地服务抓取网页。
- **只要 API / MCP**：不启动桌面 App，直接运行 Go 服务即可（见下文「Web 服务」）。

## Web 服务（无桌面 App 时）

不安装桌面 App 也可以只运行 Go 服务，通过浏览器使用 Web 管理界面，或把它当作纯 API / MCP 服务。
Web 版与桌面 App 使用同一套 Go 服务与 SQLite 数据库，接口完全一致。

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

## 浏览器扩展配置

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

## API 与 MCP

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

## 桌面 App（MyGO）

桌面 App 是 Grabby 的主要使用方式。它复用同一套 Go 后端接口，但使用独立的界面
`go-server/desktop/dist/`：macOS 为 Apple News 风格（沉浸式半透明侧栏、大标题、卡片流），
Windows 为 Fluent / Windows 新闻风格，两者都跟随系统深浅色，并按平台处理窗口与图标样式。
界面是运行在系统 WebView 中的 HTML/CSS/JS；进程、托盘、单实例、数据库和 API 服务则是原生 Go 二进制。

### 关于 MyGO

桌面外壳基于 [MyGO](https://github.com/egoist/mygo) v0.8.3 构建。MyGO 负责窗口、菜单栏托盘、
Dock 图标、单实例锁、系统 WebView 与 `.app` 打包等原生能力，Grabby 专注于界面与业务逻辑。
入口文件是 `go-server/main_desktop.go`，`make package-macos-app` 生成可分发的 `dist/Grabby.app`。

感谢 MyGO 作者 [@egoist](https://github.com/egoist) 及所有贡献者。

> 原有的 Web 管理页面保持原样，桌面 App 与 Web 版共用同一个数据库和 API，可以按需选择。

### APP 功能

APP 复用 Web 版同一套后端接口，侧栏分四组：

| 页面 | 功能 | 主要接口 |
|---|---|---|
| 今天 | 当日统计、扩展连接状态、今日日报摘要、最新内容 | `/open/api/health`、`/open/api/stats`、`/open/api/items`、`/open/api/ai/daily/list` |
| 内容库 | 服务端搜索；全部 / 已收藏 / AI 优质；来源、内容类型、AI 分类标签筛选；加载更多、详情、收藏、已读 | `/open/api/items`、`/open/api/ai/items`、`/api/items/:id/*` |
| RSS 源 | 订阅源增删改、启停、单条与批量立即抓取、抓取日志、分组 | `/api/sources`、`/api/sources/:id/toggle|run`、`/api/logs` |
| 新闻源 | 来源分组跳转、X / Reddit / 小红书 / 网页提取采集、提取归档 | `/api/twitter/*`、`/api/reddit/*`、`/api/xiaohongshu/*`、`/api/extract`、`/api/captures/extract` |
| AI 分析 | 「优质内容 / 概览与配置」两个 Tab；队列与用量统计、连通性测试、全量评测、优质内容（含分析时间）、单条重分析 | `/open/api/ai/stats`、`/api/ai/test`、`/api/ai/start_eval`、`/open/api/ai/*` |
| AI 日报 | 按日期 / 类型查看、生成、补生成近 7 天、历史列表（含生成时间）、RSS 订阅地址 | `/open/api/ai/daily`、`/open/api/ai/daily/list`、`/api/ai/daily/generate` |
| 浏览器设备 | 在线状态、断开 / 禁用 / 解禁、扩展连接信息 | `/api/browsers*` |
| API 接入 | Base URL、MCP、鉴权、中转链路、端点清单与 curl 示例 | — |
| 设置 | 六个分区：服务状态、AI 模型、提示词、日报定时、API 与安全、数据与文件（含配置导出 / 导入） | `/api/ai/settings`、`/api/desktop/settings`、`/api/ai/test`、`/api/config/*` |

**设置页（参考 Web 的 AISettings 布局）**用编号分区 + 顶部快速跳转，全部配置集中在一页：

1. **01 服务状态** — 进程、监听地址、本机免密状态、Token 状态、扩展 WebSocket 地址；
2. **02 AI 模型** — 启用开关、配置档选择 / 新建（对应后端的 provider profiles）、服务商预设
   （OpenAI / Anthropic / DeepSeek / Gemini / OpenRouter / LM Studio / Ollama / 自定义等）、
   模型目录（实时读取 models.dev，含价格、上下文与能力标签）、服务商、模型、Base URL、
   API Key、每分钟请求上限、优质阈值、多模型策略；
3. **03 提示词** — 系统提示词与日报提示词，标注可用变量；
4. **04 日报定时** — 早报 / 晚报开关与时间，可在本页直接测试模型连通性；
5. **05 API 与安全** — Token 开关 / 生成 / 保存、局域网访问开关、鉴权说明；
6. **06 数据与文件** — 数据目录、`grabby.db`、`desktop-settings.json`、配置导出 / 导入、curl 示例。

模型配置必须写在「配置档（profile）」里：后端会用当前配置档回填 provider / model / base_url，
所以设置页直接编辑选中的配置档并把它设为当前使用，保存后立即对新分析任务生效。
切换配置档会重新载入该档的字段；新建配置档从空白表单开始，不会复制已有的 Key。

设置页同时支持：

- **导入已有配置**：自动扫描 Claude Code、Codex、OpenCode、CC Switch 的本地配置，预览后一键导入连接、Key、端点和模型；
- **获取模型**：用当前连接调用供应商的 `/v1/models`（Anthropic / Gemini / Ollama 各自适配），勾选后写入模型列表；
- **多模型**：模型输入支持逗号分隔的多个模型，按「多模型策略」轮询或故障转移；
- **配置备份**：把 AI 配置与订阅源导出为 `grabby-config-*.json`，换设备或重装后导入；AI 配置整体覆盖，数据源按 ID 合并，内容库与抓取日志不受影响；
- **用量**：「AI 分析」页展示近 7 天的调用次数、Token 与平均延迟，数据来自 `ai_usage` 表。

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
make package-macos-app   # 生成 dist/Grabby.app（本机架构，未签名）
```

打包之后**直接双击 `dist/Grabby.app` 就能用**：服务随 APP 一起启动，不需要终端，也不需要另开进程；
仓库里打出来的这个 bundle 会自动复用 `go-server/` 的数据库与 `.env`，订阅源和内容库都在。

发布产物使用统一脚本构建（CI 也调用它）：

```bash
bash scripts/package-desktop.sh macos   0.2.0   # 通用架构 .app + .dmg
bash scripts/package-desktop.sh windows 0.2.0   # .exe + zip + NSIS 安装包（需 go-winres、makensis）
bash scripts/package-desktop.sh linux   0.2.0   # 可执行文件 + tar.gz + .deb（需 dpkg-deb）
```

桌面 App 是**单个自包含的可执行文件**：Go 服务端、HTTP / WebSocket / MCP、SQLite 与
`desktop/dist` 界面都通过 `go:embed` 打进二进制，运行时不依赖 Go、Node 或任何后端进程；
交叉编译使用 `CGO_ENABLED=0`（MyGO 是纯 Go），因此 macOS / Windows / Linux 的产物都可以在任意机器上构建。

MyGO 要求 Go 1.27.3+（`go-server/go.mod` 已升级）。若本机设置了 `GOSUMDB=off`，
Go 自动切换 toolchain 会失败，Makefile 的 desktop target 已临时指定 `GOSUMDB=sum.golang.org`。

### 测试

```bash
cd go-server && go test ./...                        # 后端单元测试
cd go-server/frontend && npm ci && npm run build    # 前端构建 + TypeScript 检查
cd chrome-extension && npm ci && npm run build      # 生成 defuddle bundle
```

### 发布（打 tag）

打包由 GitHub Actions 完成，工作流见 `.github/workflows/release.yml`：

```bash
git tag v0.2.0
git push origin v0.2.0
```

推送 `v*` tag 后会自动：

1. 构建 Web 前端与 Chrome 扩展（`grabby-extension-<version>.zip`）；
2. 交叉编译 headless server 与 CLI（darwin / linux / windows × amd64 / arm64）；
3. 构建桌面 App：
   - macOS：arm64 + amd64 合并为通用二进制，写入版本号、ad-hoc 签名，打包 `.dmg`；
   - Windows：用 go-winres 写入图标与版本信息，生成 `.exe`、免安装 zip 和 NSIS 安装包（amd64 / arm64）；
   - Linux：生成可执行文件、`.tar.gz` 和 `.deb`（amd64 / arm64，含桌面入口与图标）；
4. 运行 `go test ./...`；
5. 创建 GitHub Release，附上全部产物与 `checksums.txt`。

也可以在 Actions 页面手动触发 `workflow_dispatch`，填写版本号后只构建产物，不会创建 Release。
本地打包使用同一个脚本，命令见上文「构建与运行」。

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
- 安装包是自包含的单个可执行文件，不需要 Go / Node / 数据库；但 Linux 仍需要系统安装 GTK 3 与 WebKitGTK 4.1（`.deb` 已声明依赖）。
- macOS 与 Windows 产物**没有购买代码签名证书**：macOS 只做 ad-hoc 签名、未公证；Windows 安装包未签名。
  首次启动的处理方式（右键打开 / 「仍要运行」）见上文「安装桌面 App」。
- Chrome 扩展仍需单独安装（登录态在真实浏览器里），APP 不会替你安装。
- **对外监听时 HTTP 与 WebSocket 都会校验 Token**：远端 HTTP 请求带 `X-API-Key`（或 `Authorization: Bearer`），
  远端 WS 握手带 `?token=`。本机 loopback 请求始终免密，所以 Chrome 扩展、CLI、MCP 客户端与 APP 自身不受影响，
  Web 版的鉴权行为也没有变化。`CheckOrigin` 仍为允许，不要把 Token 长期暴露在不可信网络里。

## 项目架构

```
grabby/
├── go-server/               # Go 后端 + 桌面入口（唯一后端）
│   ├── main.go              # headless 服务入口
│   ├── main_desktop.go      # 桌面 App 入口（MyGO）
│   ├── internal/
│   │   ├── domain/          # 领域模型（capture / item / source / ai / browser）
│   │   ├── application/     # 业务逻辑（scraping / ai / scheduler / twitter / reddit / xiaohongshu / backup）
│   │   ├── infrastructure/  # 基础设施（browserws / sqlite / llm / browserregistry / catalog）
│   │   └── interfaces/      # 对外接口（http / websocket / mcp / dto）
│   ├── frontend/            # React Web 管理界面（编译后嵌入 headless 二进制）
│   └── desktop/             # 桌面 App（MyGO WebView UI + 平台图标资源）
├── assets/                  # 品牌 Logo 与各平台图标（icons/macos|windows|linux）
├── go-cli/                  # Go 版命令行客户端
├── python-cli/              # Python 版命令行客户端（免编译）
├── chrome-extension/        # Chrome 浏览器扩展
│   ├── background.js        # 后台服务脚本
│   ├── lib/                 # 提取 / WebSocket / 截图
│   ├── popup/ options/      # 弹窗与设置页
│   └── manifest.json        # 扩展配置
├── docs/                    # 使用与 API 文档
├── packaging/               # 桌面打包配置（Windows NSIS / Linux desktop entry）
├── scripts/                 # 安装与打包脚本（含 package-desktop.sh）
├── site/                    # 官网（Astro）
├── start.sh / start.py      # 启动脚本 (macOS / Linux / Windows)
└── readme.md                # 本文档
```

## 数据流

### 网页提取（API）

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

### 订阅与 AI 阅读

```
RSS / API / 平台采集（X、Reddit、小红书…）
    |
    | scheduler + task queue
    v
Chrome Extension（真实浏览器，保留登录态）
    |
    v
SQLite（scraped_items）
    |
    | AI 队列：评分 / 分类 / 摘要 / 点评
    v
┌───────────────┬──────────────────┐
│ 内容库 / 今天  │  AI 日报（早报/晚报）│
└───────────────┴──────────────────┘
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
- [x] 订阅源 / 平台采集与 AI 分析
- [x] AI 早报 / 晚报与桌面 App
- [x] 配置导入 / 导出
- [x] macOS 通用架构 DMG / Windows 安装包 / Linux 包
- [ ] 提取规则配置

## 许可证

本项目采用 MIT 许可证。
