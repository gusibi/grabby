# 配置指南

## 配置文件位置

| 后端 | 配置文件 |
|------|---------|
| Python | `python-server/.env` |
| Go | `go-server/.env` |
| 浏览器扩展 | 扩展选项页面（chrome-extension/options/） |

---

## 服务端配置

复制 `.env.example` 为 `.env` 后按需修改：

```bash
# Python 后端
cd python-server && cp .env.example .env

# Go 后端
cd go-server && cp .env.example .env
```

### 配置项说明

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `HOST` | `0.0.0.0` | 服务器监听地址，`0.0.0.0` 表示接受所有网络接口 |
| `PORT` | `5040` | 服务器监听端口 |
| `CONNECT_ID` | `browser-tools` | 浏览器扩展连接 ID，**两端必须一致** |
| `API_KEY` | `''` | HTTP/WebSocket 端点保护密钥。留空表示关闭校验，配置后请求需携带 `X-API-Key` header |
| `DEBUG` | `false` | 调试模式，开启后输出更多日志 |
| `WEBSOCKET_TIMEOUT` | `5.0` | WebSocket 请求默认超时（秒） |
| `API_EXTRACT_TIMEOUT` | `60.0` | HTTP API extract 端点超时（秒） |

### 配置示例

```bash
# 监听所有接口，端口 5040
HOST=0.0.0.0
PORT=5040

# 连接 ID（与浏览器扩展设置一致）
CONNECT_ID=browser-tools

# 关闭调试模式
DEBUG=false

# 提取操作可能较慢，设为 60 秒
API_EXTRACT_TIMEOUT=60.0
```

### 配置优先级（高到低）

1. 系统环境变量
2. `.env` 文件
3. 代码中的默认值

---

## 浏览器扩展配置

### 打开设置页面

右键点击 Chrome 工具栏中的 Grabby 图标 → **选项 / Options**

### 配置项

| 字段 | 示例值 | 说明 |
|------|--------|------|
| **WebSocket 服务器地址** | `ws://localhost:5040/ws_browser` | 后端 WebSocket 端点 |
| **API 密钥** | `browser-tools` | 必须与后端 `CONNECT_ID` 一致 |
| **启动时自动连接** | 勾选 | 推荐开启 |

### 认证原理

扩展连接时会将 API 密钥作为 `conn_id` 参数附加到 WebSocket URL：

```
ws://localhost:5040/ws_browser?conn_id=browser-tools
```

后端验证 `conn_id` 是否匹配 `CONNECT_ID`。不一致则拒绝连接（返回 403）。

---

## 高级配置

### Python 后端：使用 pyproject.toml

Python 后端的依赖和项目元数据定义在 `python-server/pyproject.toml`：

```toml
[project]
name = "grabby"
version = "1.0.0"
dependencies = [
    "fastapi",
    "fastapi-mcp",
    "uvicorn",
]
```

### Go 后端：交叉编译

```bash
# macOS ARM64
cd go-server && GOOS=darwin GOARCH=arm64 go build -o go-server-darwin .

# Linux AMD64
cd go-server && GOOS=linux GOARCH=amd64 go build -o go-server-linux .

# Windows
cd go-server && GOOS=windows GOARCH=amd64 go build -o go-server.exe .
```

### 多环境配置

可以通过环境变量覆盖 `.env` 配置：

```bash
# 临时修改端口运行
PORT=9000 ./go-server

# 或
PORT=9000 uv run python main.py
```
