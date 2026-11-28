# 订阅文本采集与归档（第一阶段）

Grabby 现在把四类订阅的文本统一归档到同一个内容库，并为每一轮抓取保存逐条明细：

| 订阅类型 | `type` | 归档内容 |
| --- | --- | --- |
| RSS / Atom | `rss` | Feed 自带正文或摘要；可补抓公开原文正文 |
| 博客列表 / 单篇网页 | `web_scrape` | 列表页发现文章 → 文章标题、正文、来源链接、发布时间 |
| YouTube 频道 / 视频 | `youtube` | 视频链接、标题、原始简介、公开（人工/自动）字幕；无字幕时可回退到语音转写 |
| 播客 | `podcast` | 单集链接、节目简介、音频链接、公开文字稿 |

订阅抓取本身只读取公开文本；只有 YouTube 没有公开字幕时，才会在「设置 → 阅读与转写」启用 ASR 的前提下按需下载音频并转写（本地 Whisper CLI 或云端 Whisper 接口）。播客音频仍然不下载。
只采集公开内容；登录墙、付费墙、403/404 等会作为失败原因记录在抓取日志中。

## 1. 配置订阅

### 桌面端

「订阅源」→「添加数据源」：

- 类型可选 RSS / 播客 / 视频 / API / 网页 / 文章。
- RSS 可打开「抓取全文」；网页抓取始终保存文章页正文（列表订阅的用途）。
- 网页 / 文章有一个「抓取页面中的链接」开关：打开后自动把本页正文里的链接分别保存为文章，导航、分类、分页和图片链接会自动跳过，不需要写 CSS 选择器。
- 需要精确控制站点结构时，可在「高级选项」里填「列表区域」「文章链接」「链接规则」「每轮上限」。
- 保存后可在同一入口编辑、启停、立即抓取、查看日志。

### HTTP API

```bash
# RSS，开启全文
curl -X POST http://127.0.0.1:5040/api/sources \
  -H 'Content-Type: application/json' \
  -d '{"id":"hn","name":"Hacker News","type":"rss",
       "url":"https://hnrss.org/frontpage","schedule":"0 */2 * * *",
       "default_category":"article","category":"科技",
       "config":"{\"full_content\":true,\"max_backfill\":15}"}'

# 博客列表：打开 crawl_links 即可自动发现正文里的文章链接
curl -X POST http://127.0.0.1:5040/api/sources \
  -H 'Content-Type: application/json' \
  -d '{"id":"anthropic","name":"Anthropic News","type":"web_scrape",
       "url":"https://www.anthropic.com/news","schedule":"0 */4 * * *",
       "default_category":"blog",
       "config":"{\"crawl_links\":true,\"max_items\":5}"}'

# YouTube 频道（@handle 或 /channel/UC... 均可）
curl -X POST http://127.0.0.1:5040/api/sources \
  -H 'Content-Type: application/json' \
  -d '{"id":"matt","name":"Matt Pocock","type":"youtube",
       "url":"https://www.youtube.com/@mattpocockuk/videos","schedule":"0 */6 * * *",
       "default_category":"youtube","config":"{\"max_items\":10}"}'

# 播客 RSS
curl -X POST http://127.0.0.1:5040/api/sources \
  -H 'Content-Type: application/json' \
  -d '{"id":"bbc","name":"Learning English Grammar","type":"podcast",
       "url":"https://podcasts.files.bbci.co.uk/p02pc9wq.rss","schedule":"0 */6 * * *",
       "default_category":"podcast","config":"{\"max_items\":10}"}'
```

### 配置字段

RSS / 网页抓取：

- `full_content`：抓取全文。兼容旧字段 `fetch_full_via_scrape`（任一为 true 即开启）。
- 原文链接是 PDF 时：自动提取 PDF 文字层作为正文，元数据记 `content_source=pdf_text`、`pdf_url`、`pdf_pages`；扫描件等无法提取文字的 PDF 保留 Feed 简介，记 `content_source=pdf`，阅读器提供「打开 PDF 原文」入口，下一次抓取不会重复下载。
- `crawl_links`：网页 / 文章订阅的简单列表模式。打开后自动从页面正文发现文章链接，并跳过导航、分类、分页和图片链接；不需要配选择器。
- `max_items`：网页列表每轮最多处理多少篇文章。
- `max_backfill`：每轮最多补抓多少条原文（RSS 新条目与旧摘要条目共用此上限，默认 15）。
- `list_selector`：博客列表的区域（CSS 选择器，可选；配了就优先于自动发现）。
- `article_link_selector`：列表区域内文章链接的选择器（可选）。
- `article_url_pattern` / `exclude_url_pattern`：文章链接正则（可选）。
- `use_browser`：强制用浏览器执行器打开列表页（可选）。浏览器返回 Markdown，发现链接时使用 URL 规则。CSS 选择器适用于 HTTP 获取的 HTML。

YouTube：

- `max_items`：每轮最多打开多少个视频页（默认 10）。
- `transcribe_audio`：保留的旧选项；本阶段默认不下载音频、不调用转写。

播客：

- `max_items`：每轮最多处理多少集（默认 10）。音频只在用户点击播放时由阅读器拉取。

## 2. 立即抓取与定时抓取

- 立即抓取：`POST /api/sources/{id}/run`，返回 `log_id`、`status` 与新增/补全/跳过/失败计数。
- 定时抓取：沿用 `schedule` 的 cron 表达式，触发同一条抓取链路，日志 `trigger` 记为 `scheduled`。
- 重复运行不会产生副本；已入库条目会保留内容库 ID、已读和收藏状态。
- 开启全文后再次运行即可补抓旧的摘要条目；即使 Feed 返回 `304 Not Modified`，也会继续补抓已知的摘要条目。
- 原文需要浏览器执行器时，RSS 先保留 Feed 内容，浏览器在后台补全正文，抓取明细会在任务完成后更新。

## 3. 查看逐条抓取结果

### 桌面端

「订阅源」→ 某订阅的「日志」→ 某轮「明细」：

- 触发方式（手动 / 定时）、开始/结束时间、整体状态和汇总。
- 每条：标题或链接、来源链接、内容库 ID、结果和原因。
- 每种文本分别显示状态与来源，例如：
  - `content: stored · web_page · 1200 字`
  - `content: failed · feed_summary · 阶段 access · HTTP 404`
  - `description: stored · youtube_feed`
  - `transcript: no_public_text · youtube caption`
  - `transcript: stored · podcast_transcript · 8000 字`
  - `audio: stored · audio_enclosure`

### API

```bash
# 某订阅的抓取历史（含 trigger/status/计数/汇总）
GET /api/logs?source_id=hn

# 某一轮的逐条明细（含内容库 item_id 和每种文本的来源/状态/失败阶段）
GET /api/logs/{log_id}
```

状态说明：

| 层级 | 取值 |
| --- | --- |
| 整轮 | `running`、`success`、`partial`、`error`、`skipped`、`not_modified`、`no_new_items` |
| 条目 | `stored`、`updated`、`kept_existing`、`skipped`、`no_public_text`、`failed`、`pending` |
| 文本 | `stored`、`updated`、`kept_existing`、`skipped`、`not_attempted`、`no_public_text`、`failed`、`unverified`、`pending` |
| 失败阶段 | `access`（访问）、`extract`（解析/提取）、`persist`（入库） |

日志只保存状态、原因和定位信息；正文、简介、字幕和文字稿始终从内容库读取：

```bash
GET /open/api/items/{item_id}
# 或桌面端「内容库」→ 打开对应条目
```

## 4. 阅读

- RSS / 博客：内容详情与阅读器显示正文；摘要条目也会保留 Feed 摘要和原文链接。
- PDF：阅读器顶部显示 PDF 文档卡片（页数、打开原文按钮）。能提取文字时正文是 PDF 的文字版；扫描件或加密文件则说明原因并只保留链接。
- YouTube：正文中分别有「视频简介」和「视频文字稿 (Transcript)」两节；元数据记录 `caption_status`、`caption_source`、`caption_language`、`caption_languages`。没有公开字幕、且 ASR 可用时，`caption_source` 为 `asr`，文字稿来自语音转写。
- 播客：正文中分别有「节目简介 (Show Notes)」和「播客文字稿 (Transcript)」；阅读器根据 `media_url` 提供播放器，音频按需拉取。

## 5. 自动验收

```bash
# 仓库根目录
make acceptance
# 等价于
./scripts/acceptance.sh
```

验收使用本地 httptest 固定样本（RSS / 两种博客列表 / YouTube Atom+播放器响应 / 播客 RSS+SRT）和临时 SQLite，
不需要公网、浏览器、登录或模型 API。
