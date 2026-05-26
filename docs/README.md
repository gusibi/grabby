# Grabby 文档

## 目录

| 文档 | 说明 |
|------|------|
| [工具介绍](introduction.md) | 项目概述、核心能力、架构图 |
| [安装指南](installation.md) | 环境要求、安装步骤、常见问题 |
| [配置指南](configuration.md) | 服务端配置、扩展配置、高级配置 |
| [使用指南](usage.md) | 启动服务、HTTP API、MCP 工具、命令行客户端 |
| [API 参考](api-reference.md) | HTTP API 端点、MCP 工具定义、WebSocket 协议、错误码 |

## 快速开始

```bash
# 1. 克隆项目
git clone https://github.com/your-repo/grabby.git
cd grabby

# 2. 加载浏览器扩展（Chrome → chrome://extensions → 加载已解压的扩展程序 → 选择 chrome-extension/）

# 3. 启动后端（Python 或 Go 二选一）

# Python
cd python-server && cp .env.example .env && uv run python main.py

# 或 Go
cd go-server && cp .env.example .env && go run .

# 4. 配置扩展：右键 MCP 图标 → 选项 → 填写服务器地址和连接 ID

# 5. 测试提取
curl -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
```

## 项目结构

```
grabby/
├── docs/                    # 本文档
├── chrome-extension/        # Chrome 浏览器扩展
│   ├── manifest.json
│   ├── background.js
│   ├── content/
│   ├── lib/
│   └── ...
├── python-server/           # Python 后端（FastAPI）
│   ├── main.py
│   ├── config.py
│   └── ...
├── go-server/               # Go 后端
│   ├── main.go
│   ├── config.go
│   └── ...
├── Makefile
├── start.sh                 # 启动脚本
└── start.py
```
