---
name: grabby
description: 抓取网页内容（URL转Markdown）并从本地 Grabby 服务读取 AI 智能新闻和日报。当用户说抓取网页、URL 转 Markdown、保存网页、grab page、extract content、fetch page、scrape webpage 时触发；也在用户问"今天的 AI 新闻"、"财经新闻"、"今日新闻"、"获取日报"、"早报"、"晚报"、"智能日报"、"新闻摘要"、"新闻分类"等时触发。只要用户想读取任何类别的今日资讯或日报内容，都应使用此 skill。
---

# Grabby — 网页抓取 & 智能新闻

两类用途：**抓取网页** 和 **查询本地 Grabby 服务中的新闻/日报**。

---

## 一、查询智能新闻和日报

Grabby 服务在后台持续抓取订阅源并用 AI 进行分类评分，可直接调 API 读取结果。**无需浏览器扩展**。

服务地址：`http://localhost:5040`（或环境变量 `$GRABBY_SERVER_URL`）。

使用本 skill 时优先调用脚本：`scripts/grabby-api.sh`。脚本内部调用 HTTP API，并自动读取：
- `GRABBY_SERVER_URL` — 服务地址，默认 `http://localhost:5040`
- `GRABBY_API_TOKEN` — 可选，访问 `/api/*` 认证接口时使用

### 1. 获取今日日报 / 早报 / 晚报

```bash
# 今日日报（type 可选 daily | morning | evening）
scripts/grabby-api.sh daily daily "$(date +%F)"

# 最新早报
scripts/grabby-api.sh daily morning

# 最新晚报
scripts/grabby-api.sh daily evening
```

返回字段说明：
- `title` / `date` / `editor` — 报告标题、日期和编辑模型
- `sections` — 结构化新闻分组，每组包含 `title` 和 `items`
- `sections.*.items[].title` / `summary` / `source` / `link` — 新闻标题、摘要、来源和链接
- `report_type` / `generated_at` — 报告类型和生成时间
- `total_items` / `quality_items` — 处理条数

若响应没有 `sections`，说明今日尚未生成（可告知用户或触发生成）。

### 2. 按类别获取新闻

先查可用分类：
```bash
scripts/grabby-api.sh categories | jq '.categories[] | {name, count, avg_score}'
```

然后按分类拉取文章：
```bash
# 获取 AI 相关新闻（category 填分类名，如 "AI"、"财经"、"科技"、"国际"）
scripts/grabby-api.sh items AI 10 | jq '.items[] | {title, url, ai_category, score: .quality_score, summary: .ai_summary}'
```

常用参数：
- `category` — AI 语义分类名（来自 `/open/api/ai/categories`）
- `source_category` — 数据源原始分类
- `score_min` — 质量分最低值（0-10 分制，默认 0，推荐 6+）
- `limit` — 返回条数（默认 20）
- `cursor` — 翻页游标

### 3. 获取高质量新闻（综合评分筛选）

```bash
# 最近 7 天评分 ≥ 6 的优质内容（评分为 0-10 分制）
scripts/grabby-api.sh quality 6 7 10 | jq '.items[] | {title, url, score: .quality_score, category: .ai_category, summary: .ai_summary}'
```

### 如何展示给用户

- 日报/早报/晚报：直接按 `sections` 分组展示结构化 JSON
- 新闻列表：显示标题、链接、AI 分类、评分和 AI 摘要
- 若服务不可用（脚本失败），提示用户启动本地 Grabby 服务，并检查 `$GRABBY_SERVER_URL`

---

## 二、抓取网页内容

### 1. 检查服务与浏览器连接

```bash
scripts/grabby-api.sh health
```

判断方式：
- 脚本请求失败或非 200：服务未运行或地址不对，提示用户启动本地 Grabby 服务或检查 `$GRABBY_SERVER_URL`
- `browser_connected: true`：可以继续抓取
- `browser_connected: false`：服务已运行但没有浏览器扩展连接，提示用户打开 Grabby Chrome 扩展

### 2. 抓取网页

```bash
scripts/grabby-api.sh extract "https://example.com"
```

指定浏览器时传 `browser`：

```bash
scripts/grabby-api.sh extract "https://example.com" "chrome-office"
```

返回 JSON：`{"success": true, "title": "...", "url": "...", "markdown": "..."}`

将 `markdown` 字段内容展示给用户，同时显示 `title` 和 `url`。

### 3. 截图网页

```bash
scripts/grabby-api.sh screenshot "https://example.com"
```

返回 JSON：`{"success": true, "url": "...", "imageData": "data:image/png;base64,..."}`。

### 4. 查看已连接浏览器

```bash
scripts/grabby-api.sh browsers
```

如未设置 `GRABBY_API_TOKEN` 且服务端启用了认证，`/api/*` 会返回 401。让用户设置 `GRABBY_API_TOKEN`，或先在浏览器中登录管理后台。

---

## 错误处理

| 情况 | 处理 |
|------|------|
| 服务未运行 | 启动本地 Grabby 服务，或检查 `$GRABBY_SERVER_URL` |
| 日报为 null | 提示今日尚未生成，可建议用户在 Grabby 界面触发生成 |
| 分类为空 | 可能 AI 分析未启用，告知用户在设置中开启 AI 语义分析 |
| 浏览器未连接（抓取时）| 提示打开 Grabby Chrome 扩展 |
| `/api/*` 返回 401 | 设置 `GRABBY_API_TOKEN`，或通过 Cookie 登录管理后台 |
