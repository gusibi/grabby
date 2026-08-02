# 安装指南

## 前置要求

### 必需

- **Chrome 浏览器**（或基于 Chromium 的浏览器）
- **Node.js**（用于构建浏览器扩展和管理界面）
- **Go 1.23+**（后端运行环境）

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

```bash
cd go-server

# 复制环境配置
cp .env.example .env

# 编译并运行
go build -o go-server .
./go-server
```

也可以在项目根目录用启动脚本或 Makefile：

```bash
./start.sh        # macOS / Linux
python start.py   # 跨平台（含 Windows）
make run-go       # 构建前端 + 二进制后启动
```

---

## 4. 验证安装

### 检查后端服务

```bash
# 健康检查（GRABBY_API_TOKEN 未配置）
curl http://localhost:5040/open/api/health

# 健康检查（GRABBY_API_TOKEN 已配置）
curl -H "X-API-Key: your_token" http://localhost:5040/open/api/health

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
# GRABBY_API_TOKEN 未配置时
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'

# GRABBY_API_TOKEN 已配置时
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_token" \
  -d '{"url": "https://example.com"}'
```

---

## 常见问题

### 扩展无法连接服务器

1. 确认后端服务已启动
2. 检查 WebSocket 地址和端口是否正确
3. 确认扩展里的浏览器名称/连接标识已设置，且 token 与服务器 `.env` 中的 `GRABBY_API_TOKEN` 一致
4. 检查防火墙是否放行端口

### Go 编译失败

确保 Go 版本 >= 1.23：

```bash
go version
```

如果模块下载失败，设置代理：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```
