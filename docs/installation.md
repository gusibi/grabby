# 安装指南

## 前置要求

### 必需

- **Chrome 浏览器**（或基于 Chromium 的浏览器）
- **Node.js**（用于构建浏览器扩展）
- **后端运行环境**（Python 或 Go 二选一）

### 可选

- **[uv](https://docs.astral.sh/uv/)** — Python 包管理器（推荐）
- **Go 1.23+** — 如果使用 Go 后端

---

## 1. 克隆项目

```bash
git clone https://github.com/your-repo/grabby.git
cd grabby
```

---

## 2. 安装浏览器扩展

### 方式一：Chrome 网上应用店安装（推荐）

一键安装，自动更新：

[**点击安装 — Grabby - 网页内容采集助手**](https://chromewebstore.google.com/detail/mcp-网页内容采集工具/hfimnafeekedoeeflppddlkbhcbbnfab)

安装后点击 Chrome 工具栏的 Grabby 图标，打开 **选项 / Options** 页面配置服务器地址。

### 方式二：开发模式加载

如果你需要修改源码或进行开发调试：

1. 打开 Chrome，访问 `chrome://extensions`
2. 开启右上角 **"开发者模式"**
3. 点击 **"加载已解压的扩展程序"**
4. 选择项目中的 `chrome-extension/` 目录

### 方式三：打包安装

```bash
# 构建并打包
make all

# 然后在 chrome://extensions 页面拖拽 dist/grabby-v*.zip 安装
```

### 构建依赖安装

```bash
cd chrome-extension
npm install
```

---

## 3. 安装后端服务

### 选择一：Python 后端（推荐新手）

#### 使用 uv（推荐）

```bash
cd python-server

# 复制环境配置
cp .env.example .env

# uv 会自动处理：读取 pyproject.toml → 创建虚拟环境 → 安装依赖 → 运行
uv run python main.py
```

#### 使用 pip

```bash
cd python-server
cp .env.example .env
pip install -r requirements.txt
python main.py
```

### 选择二：Go 后端（推荐资源受限环境）

```bash
cd go-server

# 复制环境配置
cp .env.example .env

# 编译并运行
go build -o go-server .
./go-server
```

---

## 4. 验证安装

### 检查后端服务

```bash
# 健康检查（API_KEY 未配置）
curl http://localhost:5040/api/health

# 健康检查（API_KEY 已配置）
curl -H "X-API-Key: your_api_key" http://localhost:5040/api/health

# 预期响应
{"status":"ok","browser_connected":false,"timestamp":"..."}
```

### 检查浏览器扩展连接

1. 点击 Chrome 工具栏的 Grabby 图标
2. 打开 **选项 / Options** 页面
3. 配置服务器地址和连接 ID
4. 看到 **"连接状态：已连接"** 即表示成功

### 测试提取功能

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

---

## 常见问题

### 扩展无法连接服务器

1. 确认后端服务已启动
2. 检查 WebSocket 地址和端口是否正确
3. 确认 API 密钥与服务器 `.env` 中的 `CONNECT_ID` 一致
4. 检查防火墙是否放行端口

### Python 依赖安装失败

推荐使用 `uv` 替代 `pip`，uv 会自动创建虚拟环境并解析依赖：

```bash
pip install uv
uv run python main.py
```

### Go 编译失败

确保 Go 版本 >= 1.23：

```bash
go version
```

如果模块下载失败，设置代理：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```
