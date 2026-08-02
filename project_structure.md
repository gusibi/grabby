# Grabby 项目结构

## 项目概述
Grabby 是一个分布式网页内容采集系统，由 Chrome 浏览器扩展和 Go 后端服务组成，用于自动化网页内容获取和处理。

## 目录结构

```
grabby/
├── README.md                     # 项目说明文档
├── chrome-extension/            # Chrome 浏览器扩展目录
│   ├── manifest.json            # 扩展配置文件
│   ├── background.js            # 后台脚本
│   ├── popup/                   # 弹出窗口
│   │   ├── popup.html           # 弹出窗口 HTML
│   │   ├── popup.css            # 弹出窗口样式
│   │   └── popup.js             # 弹出窗口脚本
│   ├── options/                 # 选项页面
│   │   ├── options.html         # 选项页面 HTML
│   │   ├── options.css          # 选项页面样式
│   │   └── options.js           # 选项页面脚本
│   ├── content/                 # 内容脚本
│   │   ├── content.js           # 内容处理脚本
│   │   └── content.css          # 内容样式
│   ├── lib/                     # 库文件
│   │   ├── websocket.js         # WebSocket 连接管理
│   │   ├── capture.js           # 截图功能
│   │   ├── extractor.js         # 内容提取功能
│   │   └── logger.js           # 日志记录功能
│   ├── logs/                    # 日志模块
│   │   ├── logs.html            # 日志页面
│   │   └── logs.js              # 日志处理脚本
│   ├── offscreen/               # 离屏渲染模块
│   │   ├── offscreen.html       # 离屏页面
│   │   └── offscreen.js         # 离屏脚本
│   └── icons/                   # 图标资源
│       ├── icon16.png           # 16x16 图标
│       ├── icon48.png           # 48x48 图标
│       └── icon128.png          # 128x128 图标
├── go-server/                   # Go 后端服务目录（唯一后端）
│   ├── main.go                  # 进程入口
│   ├── internal/
│   │   ├── bootstrap/           # 依赖装配与启动
│   │   ├── config/              # 配置加载（.env / 环境变量）
│   │   ├── domain/              # 领域模型 capture / item / source / ai / browser
│   │   ├── application/         # 业务逻辑 scraping / ai / scheduler / twitter / reddit / xiaohongshu
│   │   ├── infrastructure/      # browserws / sqlite / llm / browserregistry
│   │   ├── interfaces/          # http / websocket / mcp / dto
│   │   └── logging/             # 日志
│   └── frontend/                # React 管理界面（编译后嵌入二进制）
├── go-cli/                      # Go 版命令行客户端
├── python-cli/                  # Python 版命令行客户端（免编译）
├── scripts/                     # 安装脚本
├── docs/                        # 文档
├── start.sh / start.py          # 启动脚本
└── Makefile                     # 构建与打包
```

## 技术栈

### Chrome 浏览器扩展
- JavaScript (ES6+)
- HTML5 / CSS3
- Chrome Extension API
- WebSocket API

### Go 后端服务
- Go 1.23+
- Echo (HTTP 框架)
- gorilla/websocket (WebSocket)
- mark3labs/mcp-go (MCP Server)
- SQLite (数据存储)
- 内嵌 React 前端（embed.FS）