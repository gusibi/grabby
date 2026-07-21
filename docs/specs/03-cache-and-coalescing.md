# Spec 03：服务端缓存体系与在途请求合并

> 仅涉及 go-server。实现前先定位现有 extract 缓存：`grep -r "extract_cache" go-server/`，新缓存层沿用其存储方式（SQLite）。

## 目标与原则

1. **缓存只做在服务端**（已定，不再讨论）：扩展是无状态执行器。理由：多浏览器下缓存命中不能取决于路由到哪台设备；MV3 SW 随时被杀且 storage 有配额；失效逻辑只写一次。
2. 缓存 + 在途合并是"低频免风控"纪律的机器保障：agent 重试循环、多 agent 并发问同一问题时，不会转化成对平台的重复真实访问。
3. record 表（tweets / reddit_posts / xhs_notes 等）是**永久归档**，不属于缓存，不参与清理。

## 非目标

- 不做分布式缓存、不引入外部依赖（Redis 等），SQLite 单表即可
- 个人增量类数据（likes/saved/收藏）不走本缓存，走 `since` 增量语义（Spec 04）

---

## 改动 1：通用缓存表

新建表 `api_cache`（现有 `extract_cache` 迁移并入或保留并存，实现者选改动小的方案，但新平台 API 一律用 `api_cache`）：

```sql
CREATE TABLE IF NOT EXISTS api_cache (
  cache_key   TEXT PRIMARY KEY,   -- endpoint + ":" + sha256(规范化参数JSON)
  endpoint    TEXT NOT NULL,       -- 如 "twitter.search"、"extract"
  response    TEXT NOT NULL,       -- 完整响应 JSON
  size_bytes  INTEGER NOT NULL,
  created_at  INTEGER NOT NULL,    -- unix 秒
  expires_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_api_cache_endpoint ON api_cache(endpoint);
CREATE INDEX IF NOT EXISTS idx_api_cache_created ON api_cache(created_at);
```

**cache_key 规范化**：参数 JSON 按 key 排序、去掉与内容无关的字段（`browser`、`refresh`、`max_age`）后取 sha256。同一逻辑请求无论指定哪台浏览器，缓存共享。

## 改动 2：按内容类型的默认 TTL

配置文件（现有 env/config 机制）新增，单位秒：

| 配置项 | 默认值 | 适用 endpoint |
|------|------|------|
| `CACHE_TTL_DETAIL` | 86400 (24h) | extract、xiaohongshu.note、reddit.thread、youtube.transcript |
| `CACHE_TTL_LIST` | 900 (15min) | twitter.search、twitter.timeline、reddit.subreddit、reddit.search、xiaohongshu.search、xiaohongshu.user_notes、youtube.subscriptions、bilibili.dynamics |
| `CACHE_TTL_NONE` | 0（不缓存） | twitter.likes、reddit.saved、xiaohongshu.collected、bilibili.favorites、youtube.watch_later（个人增量类） |

每个适配器 handler 声明自己属于哪类。TTL=0 的 endpoint 完全绕过缓存层。

## 改动 3：请求级覆盖参数

所有走缓存的 endpoint 统一支持两个查询参数：

- `refresh=true`：跳过缓存，强制抓取，成功后更新缓存（extract 已有此语义，推广到全部）
- `max_age=<秒>`：调用方可接受的最大旧度。命中条件：`now - created_at <= min(max_age, 默认TTL对应年龄)`。`max_age=0` 等价于 `refresh=true`

响应统一附加：`"cached": true|false`，命中时再附加 `"cache_age_seconds": N`。

**写入条件**：仅当抓取成功且结果非空（`success && !EMPTY_RESULT`）才写缓存。失败不缓存（不做负缓存）。

## 改动 4：在途请求合并（singleflight）

1. 引入 `golang.org/x/sync/singleflight`，key 与 cache_key 相同。
2. 请求流程统一为：
   ```
   查缓存（除非 refresh/TTL=0）→ 命中即返回
   → singleflight.Do(cache_key, 实际抓取)   // 并发相同请求共享同一次浏览器任务
   → 成功写缓存 → 返回
   ```
3. 实现为一个装饰器/中间层函数，例如 `CachedFetch(endpoint, params, ttl, fetchFn)`，各适配器 handler 调用它，避免每个适配器复制粘贴。

## 改动 5：容量上限与清理

1. 配置 `CACHE_MAX_MB`，默认 500。
2. 写入后若 `SUM(size_bytes)` 超限，按 `created_at` 升序删除最旧行直到低于上限的 90%。
3. 后台 goroutine 每小时删除 `expires_at < now` 的行。
4. `/health` 或 `/api/stats`（如已存在）暴露缓存条数与总字节数。

## 验收标准

1. 连续两次相同的 `POST /api/xiaohongshu/note` → 第二次 `cached: true` 且浏览器端无第二次任务（看扩展日志）
2. 第二次带 `?refresh=true` → 重新走浏览器，缓存更新
3. 第二次带 `?max_age=1`（隔 2 秒发）→ 重新抓取
4. 并发 5 个相同 extract 请求（`curl` 并发）→ 浏览器只执行 1 次任务，5 个请求都拿到结果
5. `twitter.likes` 请求不产生 `api_cache` 行
6. 把 `CACHE_MAX_MB` 设为 1，塞入多条大结果 → 最旧的被清除，总量不超限
7. 抓取失败（如 `NAV_TIMEOUT`）不写缓存
