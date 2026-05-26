# Grabby 工具介绍

## 概述

Grabby 是一个**分布式网页内容采集系统**，由 Chrome 浏览器扩展和后端服务组成，用于将任意网页内容转换为结构化的 Markdown 格式。

## Chrome 插件下载

[![Chrome Web Store](https://developer.chrome.com/webstore/images/ChromeWebStore_Badge_v2_206x58.png)](https://chromewebstore.google.com/detail/mcp-网页内容采集工具/hfimnafeekedoeeflppddlkbhcbbnfab)

**[Chrome 网上应用店安装](https://chromewebstore.google.com/detail/mcp-网页内容采集工具/hfimnafeekedoeeflppddlkbhcbbnfab)** — 推荐方式，一键安装，自动更新。

如果你需要修改扩展源码或进行开发调试，也可以从 [GitHub 仓库](https://github.com/gusibi/grabby) 手动加载。

## 核心能力

### 1. 网页内容提取

通过浏览器扩展访问目标网页，使用 [defuddle](https://github.com/kepano/defuddle) 智能解析引擎：

- **自动识别**文章主内容区域（优于传统选择器匹配）
- **清理干扰元素**（广告、导航、页脚、社交按钮等）
- **标准化输出**（代码块、脚注、数学公式、Callout 等）
- **直接输出 Markdown**，无需服务端二次转换

### 2. 网页截图

支持多种截图模式：

- **可见区域截图** — 当前视口内容
- **全页面截图** — 完整滚动页面
- **指定区域截图** — 自定义矩形区域

### 3. 双语言后端

项目提供两种后端实现，功能完全一致：

| 后端 | 技术栈 | 适用场景 |
|------|--------|---------|
| **python-server** | FastAPI + WebSocket | Python 生态、快速原型 |
| **go-server** | Go + gorilla/websocket | 资源受限环境、高并发 |

### 4. MCP 协议支持

作为 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) 服务器运行，可被 AI Agent 直接调用：

- `screenshot(url, fullPage)` — 网页截图
- `extract(url)` — 提取网页 Markdown
- `add(a, b)` — 计算工具（示例）
- `get_server_time()` — 获取服务器时间

## 架构图

```
                    ┌─────────────────────────────────────────┐
                    │              AI Agent / User            │
                    └─────────────────┬───────────────────────┘
                                      │
                    ┌─────────────────▼───────────────────────┐
                    │         python-server / go-server       │
                    │   ┌──────────┐        ┌──────────┐     │
                    │   │  MCP SSE │        │ HTTP API │     │
                    │   │  /mcp    │        │ /api/... │     │
                    │   └────┬─────┘        └────┬─────┘     │
                    │        └───────────────────┘           │
                    │                   │                     │
                    │            WebSocket Manager            │
                    └─────────────────┬───────────────────────┘
                                      │ WebSocket
                    ┌─────────────────▼───────────────────────┐
                    │        Chrome Extension                 │
                    │   ┌──────────────┐  ┌──────────────┐   │
                    │   │  defuddle    │  │   capture    │   │
                    │   │  内容解析    │  │   截图模块   │   │
                    │   └──────────────┘  └──────────────┘   │
                    └─────────────────────────────────────────┘
```

## 数据流

```
用户请求 extract(url)
       │
       ▼
后端服务接收请求 ──→ 通过 WebSocket 发送命令到浏览器扩展
                           │
                           ▼
                   浏览器打开新标签页 → 访问 URL
                           │
                           ▼
                   defuddle 解析 DOM → 输出 Markdown
                           │
                           ▼
                   自动关闭标签页 → 返回 Markdown
                           │
       ▼                   │
后端服务直接返回 Markdown ◄┘
```

## 技术亮点

1. **浏览器端解析** — defuddle 在真实浏览器环境中运行，对动态渲染、懒加载、SPA 等现代网页有更好的兼容性
2. **自动清理标签页** — 提取完成后自动关闭临时标签，避免浏览器标签堆积
3. **无状态服务端** — 服务端只做消息路由，解析工作完全在浏览器端完成，服务端无 HTML 解析逻辑
4. **双后端支持** — Python 和 Go 两套实现，按场景选择
