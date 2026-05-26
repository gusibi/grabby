# 架构说明

## 系统架构

MCP Web Capture 采用三层架构：

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端层                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ HTTP Client │  │ MCP Client  │  │ WebSocket Command   │ │
│  │ (curl等)    │  │ (AI Agent)  │  │ (命令行客户端)      │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
└─────────┼────────────────┼────────────────────┼────────────┘
          │                │                    │
          ▼                ▼                    ▼
┌─────────────────────────────────────────────────────────────┐
│                        服务端层                              │
│              python-server  或  go-server                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  HTTP API   │  │  MCP SSE    │  │  WebSocket Manager  │ │
│  │  /api/*     │  │  /mcp       │  │  /ws_browser        │ │
│  │             │  │             │  │  /ws_command        │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
│         │                │                    │            │
│         └────────────────┴────────────────────┘            │
│                          │                                 │
│                   ┌──────┴──────┐                          │
│                   │ Message Bus │                          │
│                   │ (请求-响应) │                          │
│                   └──────┬──────┘                          │
└──────────────────────────┼──────────────────────────────────┘
                           │ WebSocket
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      浏览器扩展层                            │
│                    Chrome Extension                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  WebSocket  │  │  defuddle   │  │  Capture Manager    │ │
│  │  Client     │  │  内容解析   │  │  截图模块           │ │
│  │             │  │             │  │                     │ │
│  │ 连接服务端  │  │ 在真实 DOM  │  │ Chrome APIs         │ │
│  │ 收发消息    │  │ 中解析内容  │  │ tabs.captureVisible │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 各层职责

### 客户端层

- **HTTP Client** — 通过 REST API 调用服务，适合脚本和外部系统集成
- **MCP Client** — AI Agent 通过 MCP 协议调用工具
- **WebSocket Command** — 直接通过 WebSocket 发送命令，适合调试

### 服务端层

服务端是**无状态的消息路由层**，不执行任何内容解析：

- **HTTP API** — 接收 HTTP 请求，转换为 WebSocket 消息发给浏览器
- **MCP SSE** — 接收 MCP 工具调用，转换为 WebSocket 消息
- **WebSocket Manager** — 管理浏览器和命令客户端的连接，维护请求-响应映射

**关键设计：** 服务端只做消息转发，所有解析工作都在浏览器端完成。

### 浏览器扩展层

- **WebSocket Client** — 与服务端保持长连接，接收命令
- **defuddle** — 智能内容提取引擎，直接在真实 DOM 上运行
- **Capture Manager** — 通过 Chrome API 执行截图

## 数据流详解

### extract 数据流

```
1. 用户/Agent 调用 extract(url)
        │
        ▼
2. 服务端收到请求
   ├─ HTTP API: POST /api/extract
   └─ MCP: tool/extract
        │
        ▼
3. 服务端构造 BrowserRequest
   {
     command: "extract",
     url: "https://...",
     message_id: "msg-xxx"
   }
        │
        ▼
4. 通过 WebSocket 发送到浏览器扩展
        │
        ▼
5. 浏览器扩展处理
   ├─ 创建新标签页
   ├─ 访问目标 URL
   ├─ 等待页面稳定
   ├─ defuddle 解析 DOM → Markdown
   └─ 关闭临时标签页
        │
        ▼
6. 浏览器返回 BrowserResponse
   {
     message_id: "msg-xxx",
     success: true,
     result: {
       content: {
         content: "# Markdown..."
       }
     }
   }
        │
        ▼
7. 服务端将 Markdown 返回给用户
```

### screenshot 数据流

```
1. 用户/Agent 调用 screenshot(url, fullPage)
        │
        ▼
2. 服务端通过 WebSocket 发送 capture 命令
        │
        ▼
3. 浏览器扩展处理
   ├─ 创建新标签页
   ├─ 访问目标 URL
   ├─ 等待页面稳定
   ├─ 执行截图（可见区域/全页面/区域）
   └─ 关闭临时标签页
        │
        ▼
4. 浏览器返回 Base64 图片数据
        │
        ▼
5. 服务端将图片数据返回给用户
```

## 关键技术决策

### 为什么解析在浏览器端？

1. **真实 DOM** — 现代网页大量依赖 JS 渲染，服务端静态 HTML 解析会遗漏内容
2. **CSS 样式** — defuddle 利用浏览器计算样式判断隐藏元素、识别内容区域
3. **懒加载** — 浏览器端可以滚动触发图片/内容的懒加载
4. **无服务端依赖** — 服务端无需 HTML 解析库（已移除 markdownify）

### 为什么提供双后端？

| 后端 | 优势 | 适用场景 |
|------|------|----------|
| Python | 生态成熟、开发快速、MCP 库完善 | 开发调试、Python 团队 |
| Go | 内存小、启动快、并发高、单二进制 | 生产部署、资源受限 |

两套实现功能完全一致，共享同一套浏览器扩展。

### 请求-响应匹配机制

服务端使用 `message_id` 关联请求和响应：

```
服务端发送: { message_id: "A", command: "extract" }
                                    │
                                    ▼
                           浏览器处理中...
                                    │
                                    ▼
浏览器返回: { message_id: "A", success: true }
```

**Python 实现：** `asyncio.Future` + 字典映射
**Go 实现：** `chan *BrowserResponse` + `sync.Map`

## 扩展点

### 添加新的 MCP 工具

**Python：**
```python
@mcp_server.tool()
async def my_tool(param: str) -> str:
    """工具描述"""
    # 构造消息发送到浏览器
    response = await ws_manager.send_message({
        "command": "my_command",
        "url": url,
    })
    return response["result"]["content"]
```

**Go：**
```go
tool := mcp.NewTool("my_tool", mcp.WithDescription("..."))
mcpSvr.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    resp, err := wsManager.SendMessage(ctx, &BrowserRequest{
        Command: "my_command",
        URL:     url,
    }, browserConnID)
    // ...
})
```

### 自定义 defuddle 解析选项

在 `chrome-extension/lib/extractor.js` 中修改 defuddle 选项：

```javascript
const result = window.__defuddleExtract({
    markdown: true,
    contentSelector: 'article.post-content',  // 指定内容区域
    removeImages: false,                       // 保留图片
    debug: true,                               // 调试模式
});
```
