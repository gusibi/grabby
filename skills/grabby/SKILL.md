---
name: grabby
description: 抓取网页内容（URL转Markdown）、截图，借用已登录浏览器读取 X/Twitter、Reddit、小红书等平台内容，并从本地 Grabby 服务读取 AI 智能新闻和日报。当用户说抓取网页、URL 转 Markdown、保存网页、grab page、extract content、fetch page、scrape webpage、网页截图、搜索推特/Twitter/X、读取时间线、Reddit 帖子/评论/版块、抓小红书笔记/搜索小红书时触发；也在用户问"今天的 AI 新闻"、"财经新闻"、"今日新闻"、"获取日报"、"早报"、"晚报"、"智能日报"、"新闻摘要"、"新闻分类"等时触发。只要用户想抓取网页/平台内容或读取任何类别的今日资讯或日报，都应使用此 skill。
---

# Grabby — 网页抓取 & 平台读取 & 智能新闻

三类用途：**抓取网页/平台内容**、**查询本地 Grabby 服务的新闻/日报**。Grabby 借用你**已登录**的真实浏览器执行抓取，因此能拿到带登录态的内容。

服务地址：`http://localhost:5040`（或 `$GRABBY_SERVER_URL`）。完整 API 文档见 [docs/api-reference.md](../../docs/api-reference.md)。

## 脚本组织

每个脚本对应一类来源，**新增平台就加一个脚本**：

| 脚本 | 覆盖 |
|------|------|
| `scripts/grabby-api.sh` | Grabby 服务的 AI 新闻/日报（无需浏览器）：`daily` `categories` `items` `quality` `stats` |
| `scripts/browser.sh` | 浏览器通用：`health` `browsers` `extract` `screenshot` `run-script` `fetch` |
| `scripts/twitter.sh` | X/Twitter：`search` `timeline` `likes` |
| `scripts/reddit.sh` | Reddit：`thread` `subreddit` `search` |
| `scripts/xiaohongshu.sh` | 小红书：`note` `search` `user-notes` |

所有脚本共用 `scripts/_lib.sh`，自动读取环境变量：
- `GRABBY_SERVER_URL` — 服务地址，默认 `http://localhost:5040`
- `GRABBY_API_TOKEN` — 可选，访问 `/api/*` 认证接口时用（不设且服务端开了认证时 `/api/*` 返回 401/"admin login required"）

任意脚本不带参数运行可打印用法（`--help`）。

## 参数规则（重要）

**必传参数缺失时不要猜，停下来让用户提供。** 不要为了凑一次请求而编造 URL、query、subreddit、handle 等关键值——猜错只会浪费一次请求并返回无意义结果。

- 下表标「必传」的参数，如果用户没给、上下文里也拿不到确切值，就**直接向用户提问**（例如"要抓哪个帖子的 URL？""搜索关键词是什么？"），不要用占位符或臆测值发请求。
- 标「有默认值」的参数（如 `twitter.sh likes` 的 handle 默认 `gusibix`）按默认值走即可——默认值是明确约定，不算猜测。

| 命令 | 必传（缺失须询问） | 有默认值 |
|------|------|------|
| `browser.sh extract / screenshot` | `url` | — |
| `browser.sh run-script` | `url`、`script`（须白名单内） | — |
| `browser.sh fetch` | `page-url`、`request-url` | — |
| `twitter.sh search` | `query` | `limit=40` |
| `twitter.sh timeline` | — | `kind=for_you`、`limit=40` |
| `twitter.sh likes` | — | `handle=$GRABBY_TWITTER_HANDLE` 否则 `gusibix`、`limit=60` |
| `reddit.sh thread` | `url` | — |
| `reddit.sh subreddit` | `name` | `limit=100` |
| `reddit.sh search` | `query` | `limit=100`（subreddit/sort 可空） |
| `xiaohongshu.sh note` | `url`（须带 xsec_token） | — |
| `xiaohongshu.sh search` | `query` | `limit=50` |
| `xiaohongshu.sh user-notes` | `profile-url` | `limit=50` |
| `grabby-api.sh items` | `category` | `limit=10` |

---

## 一、抓取网页内容（browser.sh）

### 1. 检查服务与浏览器连接

```bash
scripts/browser.sh health
```

- 请求失败或非 200：服务未运行或地址不对，提示用户启动 Grabby 服务或检查 `$GRABBY_SERVER_URL`
- `browser_connected: true`：可继续抓取
- `browser_connected: false`：服务在跑但无浏览器扩展连接，提示用户打开 Grabby Chrome 扩展

### 2. 抓取网页为 Markdown

```bash
scripts/browser.sh extract "https://example.com"
scripts/browser.sh extract "https://example.com" "chrome-office"   # 指定浏览器
```

返回 `{"success": true, "title": "...", "url": "...", "markdown": "...", "cached": false}`。把 `markdown` 展示给用户，并显示 `title`/`url`。结果按 URL 缓存（`cached` 标识命中），需要最新内容时由服务端 `?refresh=true` 控制。

### 3. 网页截图

```bash
scripts/browser.sh screenshot "https://example.com"
```

返回 `{"success": true, "url": "...", "imageData": "data:image/png;base64,..."}`（默认整页）。

### 4. 读取页面 JS 变量 / JSON-LD / meta（白名单脚本）

```bash
scripts/browser.sh run-script "https://www.youtube.com/watch?v=xxxx" youtube.initialPlayerResponse
```

白名单脚本名：`youtube.initialPlayerResponse`、`bilibili.initialState`、`page.extractJsonLd`、`page.readMeta`。返回 `{"success": true, "json": {...}}`（非对象结果走 `text`）。脚本名不在白名单返回 502。

### 5. 在页面上下文发 fetch（带登录态）

```bash
scripts/browser.sh fetch "https://www.reddit.com/r/golang/comments/xxxx/" \
  "https://www.reddit.com/r/golang/comments/xxxx/.json"
```

第一个参数是打开的页面（提供 Cookie/Origin），第二个是实际 fetch 的地址。返回 `{"success": true, "status": 200, "text": "...响应体原文..."}`，调用方自行解析。

### 6. 查看已连接浏览器

```bash
scripts/browser.sh browsers
```

---

## 二、平台读取（需浏览器已登录对应站点）

这些接口把请求转发给已登录的浏览器执行。返回空或报错通常意味着**未登录**或页面结构变化，而非真的没内容。无浏览器连接返回 503。

### Twitter / X（twitter.sh）

```bash
scripts/twitter.sh search "golang" 40                 # 搜索
scripts/twitter.sh timeline for_you 40                # 主页时间线（for_you|following）
scripts/twitter.sh likes                              # 自己的点赞（handle 默认 $GRABBY_TWITTER_HANDLE，否则 gusibix）
scripts/twitter.sh likes jack 60                      # 指定 handle（不含 @）和上限
```

均返回 `{"success": true, "count": N, "tweets": [...]}`。单条推文字段：`id` `text` `author`(@) `author_name` `created_at` `favorite_count` `retweet_count` `reply_count` `quote_count` `url` `media[]`。

### Reddit（reddit.sh）

```bash
scripts/reddit.sh thread "https://www.reddit.com/r/golang/comments/abc123/xxx/"   # 帖子+评论树
scripts/reddit.sh subreddit golang 100                                            # 版块最新帖（/new）
scripts/reddit.sh search "ai agent" golang top 100                                # 搜索（subreddit/sort 可空字符串跳过）
```

`thread` 返回 `{success, url, post, comments}`，`comments` 可嵌套 `replies`。`subreddit`/`search` 返回 `{success, count, posts}`。帖子字段：`id` `title` `author` `subreddit` `url`(permalink) `content_url`(外链) `body`(selftext) `score` `num_comments` `created_utc`。search 的 sort：`relevance`/`new`/`top`/`comments`。

### 小红书（xiaohongshu.sh）

```bash
scripts/xiaohongshu.sh note "https://www.xiaohongshu.com/explore/xxxx?xsec_token=..."   # 笔记详情+前100评论
scripts/xiaohongshu.sh search "golang" 50                                               # 搜索笔记
scripts/xiaohongshu.sh user-notes "https://www.xiaohongshu.com/user/profile/xxxx" 50    # 用户发布的笔记
```

`note` 返回 `{success, note}`，note 含 `id` `title` `desc` `author` `liked_count` `collected_count` `comment_count` `images[]` `comments[]`（每条评论含 `content` `author` `ip_location` `like_count` `sub_comments[]`）。`note` URL **必须带 xsec_token**。`search`/`user-notes` 返回 `{success, count, notes}`。

---

## 三、查询智能新闻和日报（grabby-api.sh，无需浏览器）

Grabby 服务在后台持续抓取订阅源并用 AI 分类评分，直接读 API 即可。

### 1. 今日日报 / 早报 / 晚报

```bash
scripts/grabby-api.sh daily daily "$(date +%F)"   # 今日日报（type: daily|morning|evening）
scripts/grabby-api.sh daily morning               # 最新早报
scripts/grabby-api.sh daily evening               # 最新晚报
```

返回字段：`title`/`date`/`editor`；`sections` 结构化分组，每组含 `title` 和 `items`，`items[]` 含 `title`/`summary`/`source`/`link`；`report_type`/`generated_at`/`total_items`/`quality_items`。无 `sections` 说明今日尚未生成。

### 2. 按类别获取新闻

```bash
scripts/grabby-api.sh categories | jq '.categories[] | {name, count, avg_score}'
scripts/grabby-api.sh items AI 10 | jq '.items[] | {title, url, ai_category, score: .quality_score, summary: .ai_summary}'
```

`category` 填分类名（如 `AI`、`财经`、`科技`、`国际`）。

### 3. 高质量新闻（综合评分筛选）

```bash
# 最近 7 天评分 ≥ 6（0-10 分制）
scripts/grabby-api.sh quality 6 7 10 | jq '.items[] | {title, url, score: .quality_score, category: .ai_category, summary: .ai_summary}'
```

### 4. 内容统计

```bash
scripts/grabby-api.sh stats
```

### 如何展示

- 日报/早报/晚报：按 `sections` 分组展示结构化内容
- 新闻列表：显示标题、链接、AI 分类、评分、AI 摘要
- 服务不可用时提示用户启动本地 Grabby 服务并检查 `$GRABBY_SERVER_URL`

---

## 错误处理

| 情况 | 处理 |
|------|------|
| 服务未运行 / 脚本请求失败 | 启动本地 Grabby 服务，或检查 `$GRABBY_SERVER_URL` |
| `browser_connected: false` 或 503 | 提示打开 Grabby Chrome 扩展，并登录对应站点 |
| 平台返回空 / 502（`matched X/Y responses`）| 通常是未登录或页面结构变化，让用户确认浏览器已登录该站点 |
| 小红书 note 报错 | 确认 URL 带 `xsec_token` |
| `/api/*` 返回 401 / "admin login required" | 设置 `GRABBY_API_TOKEN`，或通过 Cookie 登录管理后台 |
| 日报为 null / 无 sections | 今日尚未生成，可建议用户在 Grabby 界面触发生成 |
| 分类为空 | AI 分析可能未启用，提示在设置中开启 AI 语义分析 |
