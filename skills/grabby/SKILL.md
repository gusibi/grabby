---
name: grabby
description: 抓取网页内容（URL转Markdown）并从本地 Grabby 服务读取 AI 智能新闻和日报。当用户说抓取网页、URL 转 Markdown、保存网页、grab page、extract content、fetch page、scrape webpage 时触发；也在用户问"今天的 AI 新闻"、"财经新闻"、"今日新闻"、"获取日报"、"早报"、"晚报"、"智能日报"、"新闻摘要"、"新闻分类"等时触发。只要用户想读取任何类别的今日资讯或日报内容，都应使用此 skill。
---

# Grabby — 网页抓取 & 智能新闻

两类用途：**抓取网页** 和 **查询本地 Grabby 服务中的新闻/日报**。

---

## 一、查询智能新闻和日报

Grabby 服务在后台持续抓取订阅源并用 AI 进行分类评分，可直接调 API 读取结果。**无需浏览器扩展**。

服务地址：`http://localhost:5040`（或环境变量 `$GRABBY_SERVER_URL`）

### 1. 获取今日日报 / 早报 / 晚报

```bash
# 今日日报（type 可选 daily | morning | evening）
curl -s "http://localhost:5040/api/ai/daily?date=$(date +%F)&type=daily" | jq '{title:.report.title, content:.report.content}'

# 最新早报
curl -s "http://localhost:5040/api/ai/daily?type=morning" | jq '{title:.report.title, content:.report.content}'

# 最新晚报
curl -s "http://localhost:5040/api/ai/daily?type=evening" | jq '{title:.report.title, content:.report.content}'
```

返回字段说明：
- `report.title` — 报告标题
- `report.content` — Markdown 正文（直接展示给用户）
- `report.total_items` / `report.quality_items` — 处理条数
- `report.generated_at` — 生成时间

若 `report` 为 null，说明今日尚未生成（可告知用户或触发生成）。

### 2. 按类别获取新闻

先查可用分类：
```bash
curl -s "http://localhost:5040/api/ai/categories" | jq '.categories[] | {name, count, avg_score}'
```

然后按分类拉取文章：
```bash
# 获取 AI 相关新闻（category 填分类名，如 "AI"、"财经"、"科技"、"国际"）
curl -s "http://localhost:5040/api/ai/items?category=AI&limit=10" | jq '.items[] | {title, url, ai_category, score: .quality_score, summary: .ai_summary}'
```

常用参数：
- `category` — AI 语义分类名（来自 `/api/ai/categories`）
- `source_category` — 数据源原始分类
- `score_min` — 质量分最低值（0-10 分制，默认 0，推荐 6+）
- `limit` — 返回条数（默认 20）
- `cursor` — 翻页游标

### 3. 获取高质量新闻（综合评分筛选）

```bash
# 最近 7 天评分 ≥ 6 的优质内容（评分为 0-10 分制）
curl -s "http://localhost:5040/api/ai/quality?score_min=6&days=7&limit=10" | jq '.items[] | {title, url, score: .quality_score, category: .ai_category, summary: .ai_summary}'
```

### 如何展示给用户

- 日报/早报/晚报：直接将 `report.content`（Markdown）渲染展示
- 新闻列表：显示标题、链接、AI 分类、评分和 AI 摘要
- 若服务不可用（curl 失败），提示用户启动服务：`grabby start go` 或 `grabby start python`

---

## 二、抓取网页内容

### 1. 检查 grabby CLI

```bash
command -v grabby
```

- 找到 → 继续
- 未找到 → 安装：
  ```bash
  cd ~/.grabby/src && python3 scripts/install.py --type python
  ```
  或克隆后安装：
  ```bash
  git clone https://github.com/gusibi/mcp-web-capture.git ~/.grabby/src
  cd ~/.grabby/src && python3 scripts/install.py --type python
  ```

### 2. 检查服务与浏览器

```bash
grabby health
```

- exit 0：服务运行且浏览器已连接 → 继续
- exit 1：服务未运行 → `grabby start python`
- exit 2：服务运行但浏览器未连接 → 提示用户打开 Grabby Chrome 扩展

### 3. 抓取网页

```bash
grabby extract <url>
```

返回 JSON：`{"title": "...", "url": "...", "markdown": "..."}`

将 `markdown` 字段内容展示给用户，同时显示 `title` 和 `url`。

### 其他命令

```bash
grabby screenshot <url>     # 截图
grabby browsers list        # 查看已连接浏览器
```

---

## 错误处理

| 情况 | 处理 |
|------|------|
| 服务未运行 | `grabby start go/python` 或检查 `$GRABBY_SERVER_URL` |
| 日报为 null | 提示今日尚未生成，可建议用户在 Grabby 界面触发生成 |
| 分类为空 | 可能 AI 分析未启用，告知用户在设置中开启 AI 语义分析 |
| 浏览器未连接（抓取时）| 提示打开 Grabby Chrome 扩展 |
