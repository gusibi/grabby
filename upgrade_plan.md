# Grabby 架构升级与信息流看板实现方案

本方案旨在升级已有的 `grabby` 项目，将其从一个点对点的网页内容提取系统，升级为一个**动态数据源管理、定时调度抓取、内容智能分类、支持双源追踪（抓取渠道 vs 原始出处）的个人 AI 资讯聚合面板**。

## 核心架构设计

```
┌─────────────────────────────────────────────────────────────┐
│              Web 看板与管理面板 (HTML + HTMX)                 │
│   ┌──────────┐  ┌──────────────┐  ┌───────────────────┐     │
│   │  主看板   │  │ 数据源管理面板 │  │ 抓取日志 / 状态面板 │     │
│   └──────────┘  └──────────────┘  └───────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                            │ (HTTP APIs / HTMX Partial / Template)
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                Go 数据聚合引擎 (go-server)                    │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐      │
│  │  RSS Scraper │  │  API Scraper │  │ Cron Scheduler│      │
│  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘      │
│         │                 │                  │               │
│         │    ┌─────────────────────────┐     │               │
│         │    │  Web Scrape Task Queue  │     │               │
│         │    │  (并发控制 + 排队)        │     │               │
│         │    └────────────┬────────────┘     │               │
│         │                 │                  │               │
│         ▼                 ▼                  │               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │            SQLite 数据库 (grabby.db, WAL Mode)        │   │
│  │  - sources 表          - scraped_items 表              │   │
│  │  - fetch_logs 表       (PRAGMA user_version 管理迁移)   │   │
│  └──────────────────────────────────────────────────────┘   │
│         │                 │                                  │
│  ┌──────┴─────┐   ┌──────┴──────┐                           │
│  │ Classifier │   │ Goldmark    │                           │
│  │ (分类+出处) │   │ (MD→HTML)  │                            │
│  └────────────┘   └─────────────┘                           │
└─────────────────────────────────────────────────────────────┘
        │                                  │
        ▼ (直接 HTTP/XML 抓取)              ▼ (通过 WebSocket 调用插件)
    RSS Feed / API                     Chrome Extension
                                      (ws://localhost:5040/ws_browser)
```

---

## User Review Required

> [!IMPORTANT]
> **新引入的 Go 依赖**：
> 为了支持 SQLite、RSS 订阅解析、Cron 调度和 Markdown 渲染，我们需要在 `go-server` 中引入以下第三方依赖：
> - `modernc.org/sqlite`: **纯 Go 版 SQLite 驱动**。相比 `go-sqlite3` 不需要启用 CGO，避免了跨平台编译（Cross-Compilation）时的 C 编译器依赖问题。
> - `github.com/robfig/cron/v3`: 社区最稳定的 Cron 调度库，用于解析标准 crontab 规则。
> - `github.com/mmcdole/gofeed`: 用于高容错解析不同版本的 RSS/Atom 订阅源。
> - `github.com/yuin/goldmark`: 纯 Go 的 Markdown 渲染器，用于将抓取到的 Markdown 正文在看板中渲染为 HTML。
>
> 请确认在开始开发时，我们是否可以直接运行 `go get` 来拉取这些依赖。

> [!WARNING]
> **关于 Chrome Extension 并发抓取能力（待验证）**：
> 调度引擎在执行 `web_scrape` 类型的批量抓取时，需要通过 WebSocket 向 Chrome 浏览器扩展连续发送多条 `extract` 指令。以下并发风险**尚未验证**，需要在实现前确认：
>
> 1. **Extension 端是否支持并发处理多个 `extract` 指令？** — 当前 Extension 收到 `extract` 后会开新 Tab、等待页面稳定（最多 20s）、注入 defuddle 提取内容。如果同时收到多条指令，是逐条排队处理？还是同时开多个 Tab 并行处理？
> 2. **浏览器资源限制** — 如果并行开多个 Tab（如 10 个），可能导致内存爆炸、页面加载超时或 Chrome 不稳定。
> 3. **WebSocket 消息顺序** — 多条消息并发发送时，`message_id` 匹配机制是否能正确路由返回结果？（从代码看 `pendingResponses` 是 map 按 ID 查找，理论上支持，但需要实际验证。）
>
> **应对策略（不依赖 Extension 端支持并发）**：
> Go 服务端将实现一个 **Web Scrape Task Queue**，强制串行或限制并发数（默认 1），逐条向 Extension 发送指令并等待返回后再发下一条。这样即使 Extension 不支持并发，也能正常工作。后续如果验证了 Extension 支持并发，只需调大队列的并发数即可。

> [!NOTE]
> **关于 `web_scrape` 类型的执行前提**：
> 调度引擎在执行 `web_scrape` 类型抓取时，将直接调用现有的 `wsManager.SendMessage` 以通知本机的 Chrome 浏览器扩展。这意味着**必须启动浏览器扩展且处于已连接状态**，抓取才会成功；如果浏览器未连接，此项调度将记录失败日志并跳过（不阻塞其他数据源的调度）。

---

## 数据库设计 (SQLite)

在 `go-server` 中集成内置 SQLite，使用 `PRAGMA journal_mode=WAL` 和 `PRAGMA busy_timeout=5000` 保障并发读写安全。通过 `PRAGMA user_version` 管理 Schema 版本迁移。

### 数据库初始化与迁移策略

```go
// db.go 中的迁移逻辑示意
func migrate(db *sql.DB) error {
    version := getUserVersion(db)

    if version < 1 {
        // V1: 初始表结构
        exec(db, createSourcesSQL)
        exec(db, createScrapedItemsSQL)
        exec(db, createFetchLogsSQL)
        setUserVersion(db, 1)
    }
    if version < 2 {
        // V2: 未来扩展示例 — 比如新增 scraped_items.importance 字段
        exec(db, "ALTER TABLE scraped_items ADD COLUMN importance INTEGER DEFAULT 0")
        setUserVersion(db, 2)
    }
    return nil
}
```

### 1. 数据源配置表 (`sources`)

存储所有定时抓取任务的配置：

```sql
CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,                 -- 唯一ID，如 "aihot", "hackernews"
    name TEXT NOT NULL,                  -- 数据源展示名称，如 "AI HOT"
    type TEXT NOT NULL,                  -- 抓取类型: "api", "rss", "web_scrape"
    url TEXT NOT NULL,                   -- 抓取入口 URL / API 接口
    schedule TEXT NOT NULL,              -- Cron 调度规则，如 "0 8 * * *" 或 "*/30 * * * *"
    enabled INTEGER DEFAULT 1,           -- 是否启用: 1-启用, 0-禁用
    default_category TEXT DEFAULT 'auto',-- 默认分类: "article", "tweet", "paper", "project", "auto"
    config TEXT DEFAULT '{}',            -- 数据源特定配置 (JSON)，详见下方说明
    last_etag TEXT,                      -- HTTP ETag 缓存（用于 RSS 条件请求）
    last_modified TEXT,                  -- HTTP Last-Modified 缓存
    last_fetch_at DATETIME,              -- 上次抓取时间
    last_fetch_status TEXT,              -- 上次抓取结果: "success", "error", "partial"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**`config` 字段说明** — 每种类型的数据源可以在此存储特定配置，避免为新 API 源硬编码解析逻辑：

```jsonc
// type = "api" 时的 config 示例（AI HOT 接口）
{
    "response_path": "data",           // 响应 JSON 中数据数组的路径
    "title_field": "title",            // 标题字段名
    "url_field": "url",                // 链接字段名
    "summary_field": "summary",        // 摘要字段名（可选）
    "source_field": "source",          // 原始出处字段名（可选，聚合器模式）
    "published_field": "published_at", // 发布时间字段名（可选）
    "headers": {                       // 自定义请求头（可选）
        "Authorization": "Bearer xxx"
    }
}

// type = "rss" 时的 config 示例
{
    "full_content": false,             // Feed 是否包含全文，false 表示仅摘要
    "fetch_full_via_scrape": true      // 是否对仅摘要的条目进一步调用 web_scrape 抓取全文
}

// type = "web_scrape" 时的 config 示例
{
    "list_selector": "article.post h2 a",  // 列表页中链接的 CSS 选择器（两阶段抓取）
    "max_items": 20,                       // 单次调度最多抓取条数
    "concurrency": 1                       // 并发数（默认 1，待验证 Extension 并发能力后可调）
}
```

### 2. 抓取内容表 (`scraped_items`)

存储抓取到的统一格式内容：

```sql
CREATE TABLE IF NOT EXISTS scraped_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT, -- 自增 ID（SQLite rowid，性能最优）
    source_id TEXT NOT NULL,              -- 抓取渠道 (外键 -> sources.id)
    origin_source TEXT,                   -- 最初原始来源，如 "Twitter/X", "OpenAI", "Hacker News"
    title TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,             -- 原始文章/帖子直达链接（同时用作去重依据）
    summary TEXT,                         -- 简短摘要
    content TEXT,                         -- 抓取到的网页 Markdown 正文内容
    category TEXT NOT NULL,               -- 归类: "article", "tweet", "paper", "project"
    published_at DATETIME,                -- 文章/推特发布时间
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 用户交互字段
    read_status INTEGER DEFAULT 0,        -- 0-未读, 1-已读, 2-稍后阅读
    starred INTEGER DEFAULT 0,            -- 收藏标记: 0-未收藏, 1-已收藏
    tags TEXT DEFAULT '',                 -- 用户自定义标签（逗号分隔，如 "AI,LLM,重要"）
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

-- 复合索引：支持分类筛选 + 时间排序
CREATE INDEX IF NOT EXISTS idx_items_filter ON scraped_items(category, published_at DESC);
-- 索引：支持按来源筛选
CREATE INDEX IF NOT EXISTS idx_items_origin ON scraped_items(origin_source, published_at DESC);
-- 索引：支持按阅读状态筛选（未读优先）
CREATE INDEX IF NOT EXISTS idx_items_read ON scraped_items(read_status, fetched_at DESC);
-- 索引：支持收藏列表
CREATE INDEX IF NOT EXISTS idx_items_starred ON scraped_items(starred, fetched_at DESC) WHERE starred = 1;
```

### 3. 抓取执行日志表 (`fetch_logs`)

记录每次调度执行的详细结果，用于运维排查和状态展示：

```sql
CREATE TABLE IF NOT EXISTS fetch_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id TEXT NOT NULL,              -- 关联数据源
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    status TEXT NOT NULL,                 -- "success", "partial", "error", "skipped"
    items_found INTEGER DEFAULT 0,        -- 本次发现的条目总数
    items_added INTEGER DEFAULT 0,        -- 本次新增入库的条目数（去重后）
    error_message TEXT,                   -- 错误信息（如有）
    FOREIGN KEY(source_id) REFERENCES sources(id)
);

-- 索引：按数据源查看历史日志
CREATE INDEX IF NOT EXISTS idx_logs_source ON fetch_logs(source_id, started_at DESC);
```

---

## 核心机制设计

### 1. 分类识别与出处提取算法 (`classifier.go`)

对抓取到的每条数据进行自动化清洗和分类：

- **原始出处 `origin_source` 的提取**：
  - **聚合器模式 (如 AI HOT)**：解析其返回结果中自带的 `source` 字符串（如 `"X：宝玉 (@dotey)"` 提取为 `"X (Twitter)"`；`"公众号：xxx"` 提取为 `"微信公众号"`）。
  - **直抓模式**：如果数据源无附加 `source` 字段，则提取 `url` 的域名（Domain）作为 `origin_source`。
- **内容分类 `category` 的划分**：
  - 若 `sources` 中配置了明确的 `default_category` (且不为 `'auto'`)，直接采用该值。
  - 若为 `'auto'`，通过规则链自动匹配（按优先级从高到低）：
    1. URL 包含 `x.com` / `twitter.com` → `"tweet"`
    2. URL 包含 `github.com` / `gitlab.com` → `"project"`
    3. URL 包含 `arxiv.org` / `biorxiv.org` / `.pdf` → `"paper"`
    4. URL 包含 `mp.weixin.qq.com` → `"article"`（微信公众号文章）
    5. 其余默认为 `"article"`

### 2. 定时调度管理 (`scheduler.go`)

集成 `cron/v3`，管理后台定时抓取队列：

- **初始化**：服务器启动时，读取 `sources` 表中所有 `enabled = 1` 的数据源，在 cron 引擎中注册定时调度任务。同时在内存中维护 `entryMap map[string]cron.EntryID`（`source_id → entryID`）映射。
- **增量热重载（Incremental Hot Reload）**：当用户在 Web UI 中编辑/新增/删除/启用/禁用数据源后，**不再全量重建** cron 任务，而是做增量更新：
  - 新增数据源 → 调用 `cron.AddFunc()` 并记录 `entryID`
  - 删除数据源 → 调用 `cron.Remove(entryID)` 并移除映射
  - 修改调度规则 → `Remove` 旧任务 + `AddFunc` 新任务
  - 启用/禁用 → 同上述删除/新增逻辑
- **任务执行器（Job Executor）**：
  - 每个 Job 执行时，先在 `fetch_logs` 中插入一条 `started_at` 记录
  - 执行完成后更新 `finished_at`、`status`、`items_added`、`error_message`
  - 同时更新 `sources` 表的 `last_fetch_at` 和 `last_fetch_status`

### 3. 抓取器实现 (`scrapers.go`)

三种类型的具体抓取逻辑：

#### 3.1 `ScrapeRSS(src Source)` — RSS/Atom 订阅抓取

```
1. 构造 HTTP 请求，附带 If-None-Match (ETag) 和 If-Modified-Since (Last-Modified) 头
2. 如果收到 304 Not Modified → 直接跳过，返回 "skipped"
3. 使用 gofeed 解析 Feed 内容
4. 遍历 Feed.Items：
   a. 对每条 Item 调用 Classifier 获取 category 和 origin_source
   b. 使用 INSERT OR IGNORE 入库（url UNIQUE 自动去重）
5. 更新 sources 表的 last_etag、last_modified
6. 如果 config.fetch_full_via_scrape = true 且 Feed 只有摘要：
   → 将新增条目的 URL 放入 Web Scrape Task Queue 异步补抓全文
```

#### 3.2 `ScrapeAPI(src Source)` — JSON API 抓取

```
1. 发送 HTTP GET/POST 请求到 src.url
2. 根据 config 中的字段映射规则（response_path, title_field, url_field 等）
   从 JSON 响应中提取数据列表
3. 遍历数据列表：
   a. 映射为 ScrapedItem 结构
   b. 调用 Classifier 获取 category 和 origin_source
   c. 使用 INSERT OR IGNORE 入库
4. 返回新增条目数
```

#### 3.3 `ScrapeWeb(src Source)` — 浏览器插件抓取

> [!WARNING]
> **此模块依赖 Chrome Extension 的并发能力，该能力尚未验证。实现时默认按串行处理。**

```
1. 检查 wsManager 是否有已连接的浏览器实例
   → 如果没有 → 记录 "skipped"，返回错误
2. 两阶段抓取流程：
   阶段一（提取链接列表）：
     - 如果 config.list_selector 存在 → 先抓取 src.url 列表页
       → 使用 wsManager.SendMessage 发送 extract 命令
       → 从返回的 Markdown/HTML 中解析出子链接列表
     - 如果 config.list_selector 不存在 → 将 src.url 本身作为唯一待抓取目标
   阶段二（逐个抓取详情页）：
     - 按 config.max_items 限制最大条数（默认 20）
     - 过滤已存在于数据库中的 URL（SELECT url FROM scraped_items WHERE url IN (...)）
     - 将剩余 URL 推入 Web Scrape Task Queue
3. Web Scrape Task Queue 处理：
   - 并发数 = config.concurrency（默认 1，串行）
   - 逐条发送 extract 命令 → 等待返回（超时 60s）
   - 成功 → 调用 Classifier → INSERT 入库
   - 失败 → 记录错误日志，继续下一条（不阻塞）
   - 每条抓取间增加 2s 间隔，避免过于频繁
```

### 4. Web Scrape Task Queue（抓取任务队列）

独立于 Scheduler 的内部任务队列，专门处理 `web_scrape` 类型的批量抓取：

```go
// task_queue.go 核心结构示意
type ScrapeTask struct {
    SourceID string
    URL      string
    LogID    int64   // 关联的 fetch_logs.id
}

type TaskQueue struct {
    mu          sync.Mutex
    queue       []ScrapeTask
    concurrency int           // 并发 worker 数量，默认 1
    wsManager   *WebSocketManager
    db          *Database
    running     bool
}

// Enqueue 将任务加入队列
func (tq *TaskQueue) Enqueue(tasks []ScrapeTask) { ... }

// Start 启动 worker goroutine
func (tq *TaskQueue) Start(ctx context.Context) { ... }

// processTask 处理单条抓取任务
func (tq *TaskQueue) processTask(ctx context.Context, task ScrapeTask) error {
    // 1. wsManager.SendMessage(extract command)
    // 2. 等待返回（60s 超时）
    // 3. Classify + INSERT into scraped_items
    // 4. time.Sleep(2 * time.Second) — 抓取间隔
    return nil
}
```

---

## 前端设计 (HTMX + Go Template)

使用 `go:embed` 将前端静态文件嵌入 Go 二进制，通过 Go 的 `html/template` 渲染页面，HTMX 处理局部更新。

### 页面结构

#### 主看板页面 (`templates/index.html`)

```
┌──────────────────────────────────────────────────────────────┐
│  Grabby 🔍[搜索框]                            [设置⚙️] [状态●] │
├────────────┬─────────────────────────────────────────────────┤
│            │                                                 │
│  📋 全部    │  ┌─────────────────────────────────────────┐   │
│  📰 文章    │  │ [卡片] 标题 ...                         │   │
│  🐦 推特    │  │   来源: X (Twitter) · 2h ago            │   │
│  📄 论文    │  │   摘要文本前两行...                       │   │
│  🔧 项目    │  │   [文章] [已读✓] [收藏☆]                │   │
│            │  └─────────────────────────────────────────┘   │
│  ──────── │  ┌─────────────────────────────────────────┐   │
│  来源筛选   │  │ [卡片] 标题 ...                         │   │
│  ☐ AI HOT  │  │   来源: GitHub · 5h ago                 │   │
│  ☐ HN      │  │   摘要文本前两行...                       │   │
│  ☐ Twitter │  │   [项目] [未读] [收藏☆]                  │   │
│            │  └─────────────────────────────────────────┘   │
│  ──────── │                                                 │
│  ⭐ 收藏    │  ┌─────────────────────────────────────────┐   │
│  🕐 稍后读  │  │ [卡片] ...                               │   │
│  📊 日志    │  └─────────────────────────────────────────┘   │
│            │                                                 │
│            │  [加载更多... ▼] (HTMX hx-trigger="revealed")  │
└────────────┴─────────────────────────────────────────────────┘
```

**交互设计：**

| 交互 | 实现方式 |
|------|---------|
| **分类筛选** | 左侧边栏点击分类 → `hx-get="/api/items?category=tweet"` → 替换右侧内容区域 |
| **来源筛选** | 勾选来源 Checkbox → `hx-get="/api/items?origin=aihot,hn"` → 动态过滤 |
| **无限滚动** | 内容区底部占位元素 → `hx-trigger="revealed"` → `hx-get="/api/items?cursor=xxx"` → 追加内容 |
| **搜索** | 搜索框输入 → `hx-get="/api/items?q=keyword"` → `hx-trigger="keyup changed delay:300ms"` 防抖搜索 |
| **标记已读** | 点击卡片 → `hx-post="/api/items/{id}/read"` → `hx-swap="outerHTML"` 更新卡片状态 |
| **收藏** | 点击星标 → `hx-post="/api/items/{id}/star"` → `hx-swap="outerHTML"` 切换星标状态 |
| **阅读弹窗** | 点击卡片标题 → 弹出 Modal → 展示 Markdown 正文（服务端用 goldmark 渲染为 HTML）→ 同时标记已读 |
| **连接状态** | 右上角状态指示灯，定时轮询 `GET /api/health` 检查浏览器插件连接状态 |

**搜索实现**：初期使用 `WHERE title LIKE '%keyword%' OR summary LIKE '%keyword%'` 模糊匹配。数据量超过 10 万条后可以升级为 SQLite FTS5 全文搜索。

**分页策略**：基于 `fetched_at` 的 Cursor 分页（而非 OFFSET），每页 20 条，性能稳定不随数据增长退化。

#### 数据源管理页面 (`templates/settings.html`)

```
┌──────────────────────────────────────────────────────────┐
│  ← 返回看板    数据源管理                                  │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  [+ 新增数据源]                                           │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 🟢 AI HOT 热点         类型: API    Cron: 0 */4 * * * │
│  │ 上次抓取: 2h ago ✅ 成功 (新增 12 条)                  │
│  │ [编辑] [禁用] [立即执行] [删除]                         │
│  └────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 🟢 Hacker News         类型: RSS   Cron: 0 */2 * * * │
│  │ 上次抓取: 30min ago ✅ 成功 (新增 3 条)                │
│  │ [编辑] [禁用] [立即执行] [删除]                         │
│  └────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 🔴 TechCrunch (已禁用)  类型: RSS   Cron: 0 8 * * *  │
│  │ 上次抓取: 3d ago ⚠️ 304 跳过                          │
│  │ [编辑] [启用] [删除]                                   │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

**编辑/新增表单**（HTMX Modal 或内联展开）：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | text | 数据源唯一标识（英文，新增时填写，不可修改） |
| 名称 | text | 数据源展示名称 |
| 类型 | select | `api` / `rss` / `web_scrape` |
| URL | text | 抓取入口地址 |
| 调度规则 | text | Cron 表达式，附带常用模板下拉 |
| 默认分类 | select | `auto` / `article` / `tweet` / `paper` / `project` |
| 高级配置 | textarea | JSON 格式的 `config` 字段 |

**"立即执行"按钮**：`hx-post="/api/sources/{id}/run"` → 立即触发一次该数据源的抓取任务（不等待 Cron 调度），返回执行结果摘要。

#### 抓取日志页面 (`templates/logs.html`)

展示 `fetch_logs` 表的历史记录，支持按数据源筛选：

```
┌──────────────────────────────────────────────────────────┐
│  数据源: [全部 ▼]    状态: [全部 ▼]                        │
├──────────────────────────────────────────────────────────┤
│  时间           数据源      状态    发现  新增  耗时  错误  │
│  ─────────────────────────────────────────────────────── │
│  06-07 08:00   AI HOT      ✅成功   25    12   3.2s       │
│  06-07 08:00   HN RSS      ⏭️跳过   -     -    0.1s  304  │
│  06-07 06:00   AI HOT      ✅成功   25    5    2.8s       │
│  06-07 04:00   Twitter     ❌失败   -     -    60s   超时  │
│  ...                                                      │
└──────────────────────────────────────────────────────────┘
```

---

## API 路由设计

在 `main.go` 中新增以下路由（与现有 `/api/extract`、`/api/screenshot` 等并存）：

### 页面路由

| Method | Path | 说明 |
|--------|------|------|
| GET | `/` | 主看板页面 |
| GET | `/settings` | 数据源管理页面 |
| GET | `/logs` | 抓取日志页面 |

### 数据接口（HTMX Partial + JSON）

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/items` | 获取抓取内容列表（支持 `?category=`, `?origin=`, `?q=`, `?cursor=`, `?starred=1`, `?read_status=0`） |
| GET | `/api/items/{id}` | 获取单条内容详情（返回渲染后的 HTML 正文） |
| POST | `/api/items/{id}/read` | 标记已读/未读 |
| POST | `/api/items/{id}/star` | 切换收藏状态 |
| GET | `/api/sources` | 获取所有数据源列表 |
| POST | `/api/sources` | 新增数据源 |
| PUT | `/api/sources/{id}` | 更新数据源配置 |
| DELETE | `/api/sources/{id}` | 删除数据源 |
| POST | `/api/sources/{id}/toggle` | 启用/禁用数据源 |
| POST | `/api/sources/{id}/run` | 立即执行一次抓取 |
| GET | `/api/logs` | 获取抓取日志（支持 `?source_id=`, `?status=`） |
| GET | `/api/stats` | 看板统计数据（各分类数量、今日新增、未读数等） |

---

## 预设种子数据

`db.go` 初始化时，如果 `sources` 表为空，则插入以下默认数据源：

```go
var defaultSources = []Source{
    {
        ID:       "aihot",
        Name:     "AI HOT 热点",
        Type:     "api",
        URL:      "https://api.aihot.cn/list",  // 需确认实际 API 地址
        Schedule: "0 8,12,18,22 * * *",          // 每天 8:00, 12:00, 18:00, 22:00
        Category: "auto",
        Config:   `{"response_path":"data","title_field":"title","url_field":"url","summary_field":"summary","source_field":"source"}`,
    },
    {
        ID:       "hn",
        Name:     "Hacker News",
        Type:     "rss",
        URL:      "https://hnrss.org/frontpage",
        Schedule: "0 */2 * * *",                  // 每 2 小时
        Category: "auto",
        Config:   `{"full_content":false,"fetch_full_via_scrape":false}`,
    },
    {
        ID:       "hn_best",
        Name:     "Hacker News Best",
        Type:     "rss",
        URL:      "https://hnrss.org/best",
        Schedule: "0 9 * * *",                    // 每天 9:00
        Category: "auto",
        Config:   `{"full_content":false}`,
    },
}
```

---

## 运维与可靠性设计

### 1. 错误处理与重试策略

```go
// scrapers.go 中的重试逻辑
const maxRetries = 3

func scrapeWithRetry(src Source, scrapeFunc func(Source) (int, error)) (int, error) {
    var lastErr error
    for attempt := 1; attempt <= maxRetries; attempt++ {
        count, err := scrapeFunc(src)
        if err == nil {
            return count, nil
        }
        lastErr = err
        // 指数退避: 5s, 15s, 45s
        backoff := time.Duration(5*math.Pow(3, float64(attempt-1))) * time.Second
        logger.Warn("scrape failed, retrying",
            zap.String("source", src.ID),
            zap.Int("attempt", attempt),
            zap.Duration("backoff", backoff),
            zap.Error(err))
        time.Sleep(backoff)
    }
    return 0, fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}
```

### 2. SQLite 性能配置

在 `db.go` 初始化时设置：

```go
func initDB(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }
    // WAL 模式：允许并发读写，显著提升多 goroutine 场景性能
    db.Exec("PRAGMA journal_mode=WAL")
    // 繁忙超时：写锁等待最多 5 秒，避免 "database is locked" 错误
    db.Exec("PRAGMA busy_timeout=5000")
    // 外键约束
    db.Exec("PRAGMA foreign_keys=ON")
    // 连接池配置
    db.SetMaxOpenConns(1)   // SQLite 写操作只能单连接
    db.SetMaxIdleConns(2)
    return db, nil
}
```

### 3. 优雅关闭 (Graceful Shutdown)

在 `main.go` 中捕获系统信号，按顺序关闭各组件：

```go
func main() {
    // ... 初始化 ...

    // 捕获 SIGINT / SIGTERM
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-quit
        logger.Info("shutting down gracefully...")

        // 1. 停止接收新的 HTTP 请求
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        httpServer.Shutdown(ctx)

        // 2. 停止 Cron 调度器（等待正在执行的 Job 完成）
        cronCtx := scheduler.Stop()
        <-cronCtx.Done()

        // 3. 等待 Task Queue 中正在处理的任务完成
        taskQueue.Shutdown()

        // 4. 关闭 WebSocket 连接
        wsManager.CloseAll()

        // 5. 关闭数据库连接
        database.Close()

        logger.Info("shutdown complete")
        os.Exit(0)
    }()

    // ... 启动 HTTP 服务器 ...
}
```

### 4. 数据清理策略

定期清理过期数据，避免数据库无限增长：

```go
// 在 scheduler 中注册清理任务（每天凌晨 3 点执行）
scheduler.AddFunc("0 3 * * *", func() {
    // 清理 90 天前的已读、非收藏条目
    db.Exec(`DELETE FROM scraped_items
             WHERE fetched_at < datetime('now', '-90 days')
             AND read_status = 1
             AND starred = 0`)

    // 清理 30 天前的抓取日志
    db.Exec(`DELETE FROM fetch_logs
             WHERE started_at < datetime('now', '-30 days')`)

    // 回收空间
    db.Exec("VACUUM")

    logger.Info("data cleanup completed")
})
```

---

## Proposed Changes

我们将对 `go-server` 组件进行扩展，新增数据库、调度程序、任务队列、分类处理器和前端页面模板。

### go-server

#### [NEW] [db.go](file:///Users/gusi/Github/grabby/go-server/db.go)
- 初始化 SQLite 数据库（WAL 模式 + busy_timeout）。
- 使用 `PRAGMA user_version` 管理 Schema 版本，实现自动迁移。
- 提供 `sources`、`scraped_items`、`fetch_logs` 三张表的完整 CRUD 接口。
- 首次初始化时插入预设种子数据。

#### [NEW] [classifier.go](file:///Users/gusi/Github/grabby/go-server/classifier.go)
- 实现对内容的 `category`（分类）与 `origin_source`（出处提取）的核心匹配算法。
- 包含聚合器模式（解析 source 字段）和直抓模式（提取域名）两种出处提取策略。

#### [NEW] [scrapers.go](file:///Users/gusi/Github/grabby/go-server/scrapers.go)
- 实现三种类型的具体抓取逻辑：
  - `ScrapeRSS(src Source)`: 使用 `gofeed` 解析，支持 ETag/Last-Modified 条件请求减少无用拉取。
  - `ScrapeAPI(src Source)`: 基于 `config` 字段的通用 JSON API 抓取，字段映射可配置。
  - `ScrapeWeb(src Source)`: 两阶段抓取（列表页 → 详情页），通过 Task Queue 管理并发。
- 包含重试逻辑（指数退避，最多 3 次）。

#### [NEW] [task_queue.go](file:///Users/gusi/Github/grabby/go-server/task_queue.go)
- Web Scrape 专用任务队列，控制并发数（默认 1，串行），避免浏览器 Tab 爆炸。
- 支持 2 秒间隔、超时处理、错误隔离（单条失败不阻塞后续）。

#### [NEW] [scheduler.go](file:///Users/gusi/Github/grabby/go-server/scheduler.go)
- 使用 `cron/v3` 管理后台定时抓取队列。
- 维护 `entryMap` 实现增量热重载（而非全量重建）。
- 集成数据清理定时任务。

#### [NEW] [web/](file:///Users/gusi/Github/grabby/go-server/web)
- 新增 `web` 目录并使用 `go:embed` 嵌入 Go 二进制文件，包含：
  - `templates/index.html`: 主看板页面 — 侧边栏分类/来源过滤 + 卡片流 + 无限滚动 + 搜索 + 阅读弹窗。
  - `templates/settings.html`: 数据源管理面板 — 增删改查 + 启用/禁用 + 立即执行 + 状态展示。
  - `templates/logs.html`: 抓取日志面板 — 历史执行记录、错误排查。
  - `templates/partials/`: HTMX 局部渲染片段（卡片列表、单条卡片、日志行等）。
  - `static/css/style.css`: 全局样式。
  - `static/js/htmx.min.js`: HTMX 库（本地托管，无外部 CDN 依赖）。

#### [MODIFY] [types.go](file:///Users/gusi/Github/grabby/go-server/types.go)
- 新增 `Source`、`ScrapedItem`、`FetchLog` 结构体，作为内存和数据库交互的模型。
- 新增 API 请求/响应类型（`ItemsQuery`、`SourceForm` 等）。

#### [MODIFY] [main.go](file:///Users/gusi/Github/grabby/go-server/main.go)
- 在启动时连接数据库并初始化调度引擎和任务队列。
- 注入 HTML 页面路由（`/`、`/settings`、`/logs`）。
- 注入 HTMX 数据接口（`/api/items`、`/api/sources`、`/api/logs` 等完整 API 路由）。
- 集成优雅关闭逻辑（按序停止 HTTP → Scheduler → TaskQueue → WebSocket → DB）。

---

## Verification Plan

### Automated Tests

- **`db_test.go`**：使用 Go 的 `testing` 框架 + 内存 SQLite（`:memory:`）验证：
  - Schema 迁移正确性（版本号递增 + 表/索引创建）
  - `sources` 和 `scraped_items` 的 CRUD 操作
  - URL 唯一约束冲突的 `INSERT OR IGNORE` 行为
  - `fetch_logs` 的插入和查询
  - Cursor 分页查询的正确性
- **`classifier_test.go`**：验证分类器对各类输入的处理：
  - GitHub URL → `"project"`
  - X.com / Twitter URL → `"tweet"`
  - arXiv URL → `"paper"`
  - 普通 URL → `"article"`
  - 聚合器 source 字段解析（`"X：宝玉 (@dotey)"` → `"X (Twitter)"`）
  - 域名提取（`https://openai.com/blog/xxx` → `"openai.com"`）
- **`scrapers_test.go`**：验证 API scraper 的 JSON 字段映射逻辑（使用 mock HTTP server）

### Manual Verification

1. **启动测试**：
   - 启动 `go-server`，检查 `grabby.db` 是否自动生成
   - 确认 `sources` 预设种子数据加载成功
   - 确认 WAL 模式已启用（`PRAGMA journal_mode` 返回 `wal`）
2. **调度测试**：
   - 将测试数据源的 Cron 改为 `*/1 * * * *`（每分钟抓取）
   - 观察控制台日志是否定时输出抓取和入库信息
   - 检查 `fetch_logs` 表是否正确记录每次执行结果
3. **热重载测试**：
   - 在设置页新增一个数据源（如 Hacker News RSS）
   - 保存后确认 Scheduler 立即注册新任务（无需重启服务）
   - 禁用一个数据源，确认对应 Cron 任务被移除
4. **Web 功能测试**：
   - 浏览器打开 `http://localhost:5040/` 访问看板
   - 点击侧边栏不同分类进行筛选，确认 HTMX 局部更新正常
   - 测试搜索功能（输入关键词，验证防抖和结果过滤）
   - 点击卡片查看阅读弹窗，确认 Markdown 正文渲染
   - 测试已读/收藏/稍后阅读状态切换
5. **Web Scrape 测试（需浏览器扩展在线）**：
   - 在设置页配置一个 `web_scrape` 类型的数据源
   - 点击"立即执行"，观察浏览器是否自动开 Tab 并抓取
   - 确认抓取结果正确入库
   - 测试浏览器扩展未连接时的优雅降级（应记录 "skipped" 日志）
6. **错误恢复测试**：
   - 模拟网络断开，确认重试逻辑工作正常
   - 发送 `SIGTERM` 信号，确认优雅关闭流程（等待进行中的任务完成后再退出）

---

## 分阶段实施计划

### Phase 1 — 数据基座（约 2 天）
- `db.go` — SQLite 初始化 + Migration + CRUD + 种子数据
- `types.go` — 新增 `Source` / `ScrapedItem` / `FetchLog` 结构体
- `db_test.go` — 数据层单元测试

### Phase 2 — 抓取引擎（约 2-3 天）
- `classifier.go` — 分类器 + 出处提取
- `classifier_test.go` — 分类器测试
- `scrapers.go` — RSS 抓取（含 ETag 支持）+ API 抓取（含字段映射）
- `scheduler.go` — Cron 调度 + 增量热重载 + fetch_logs 记录

### Phase 3 — Web 看板（约 2-3 天）
- `web/` — 完整前端页面（主看板 + 设置 + 日志）
- `main.go` — 路由注入 + 静态资源服务 + API 接口
- 前端交互调试（HTMX 局部更新、搜索、分页）

### Phase 4 — Web Scrape 集成（约 1-2 天）
- `task_queue.go` — 抓取任务队列（并发控制）
- `scrapers.go` — `ScrapeWeb` 实现（两阶段抓取）
- **验证 Chrome Extension 并发能力**（根据结果调整队列并发数）

### Phase 5 — 打磨与运维（约 1 天）
- 优雅关闭逻辑
- 重试策略集成
- 数据清理定时任务
- 端到端集成测试
