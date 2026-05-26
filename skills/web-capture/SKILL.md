---
name: web-capture
description: 抓取网页内容并转换为 Markdown 格式。当用户说要抓取网页、提取网页内容、URL 转 Markdown、保存网页、web capture、extract content、fetch page、scrape webpage 时触发。即使没有明确说"markdown"，只要用户想把网页内容拿下来阅读或分析，就应该使用此 skill。
---

# Web Capture — 网页内容抓取

通过本地 MCP Web Capture 服务抓取网页，返回干净的 Markdown 内容。

## 工作流程

### 1. 确定目标 URL

从用户输入中提取目标 URL。如果 URL 不完整（缺少 `https://`），自动补全。

### 2. 检查服务状态

```bash
curl -s http://localhost:5040/api/health \
  -H "X-API-Key: your_api_key"
```

如果服务未运行（连接失败），告知用户先启动服务：
```bash
cd <project-dir> && go run ./go-server/...
```

如果 `browser_connected` 为 `false`，告知用户浏览器扩展未连接。

### 3. 抓取网页

```bash
curl -s -X POST http://localhost:5040/api/extract \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{"url": "<target-url>"}'
```

API Key 默认使用 `your_api_key`。如果环境变量 `API_KEY` 有值，则优先使用环境变量。

### 4. 输出结果

将返回的 `markdown` 字段内容展示给用户。同时显示 `title` 和原始 `url`。

返回格式参考：
```json
{
  "success": true,
  "url": "https://example.com",
  "title": "页面标题",
  "markdown": "# Markdown 内容..."
}
```

### 错误处理

| 错误 | 响应 | 处理 |
|------|------|------|
| 服务未启动 | 连接失败 | 提示用户启动服务 |
| 浏览器未连接 | `503` / `{"error":"Browser..."}` | 提示用户打开浏览器扩展 |
| 超时 | `504` | 提示用户页面加载超时，可重试 |
| API Key 错误 | `401` | 检查 API Key 配置 |

## 注意事项

- 抓取依赖浏览器扩展实际加载页面，动态内容可能需要较长等待时间
- 如果页面需要登录才能访问，建议用户先在浏览器中登录
- 返回的 Markdown 内容由浏览器的内容提取算法生成，复杂页面可能不完美
