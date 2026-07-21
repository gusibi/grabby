# Spec 04：统一数据出口与增量语义

> 仅涉及 go-server + skill/MCP 更新。前置约束（`CONTEXT.md`，不可违反）：每平台独立内容表，不建跨平台共用表；评论等大体量数据不落库。
> 实现前先确认各内容表结构：`grep -rn "CREATE TABLE" go-server/` 找到 tweets / reddit_posts / xhs_notes 等表定义。

## 目标

"个人数据源"要求消费侧有统一出口：agent 不应该每接一个平台就学一套查询方式。本方案在**不动各平台表**的前提下加一个统一读取层，并把增量抓取语义标准化。

---

## 改动 1：增量抓取标准化

所有"个人集合类"原子能力（twitter.likes、以及 Spec 05 新增的 reddit.saved、xiaohongshu.collected、bilibili.favorites、youtube.watch_later）统一支持：

1. 请求参数 `since`（可选）：上次抓取返回的游标（推荐用平台内容 ID 或时间戳，各适配器自定，但对调用方是不透明字符串）。
2. 响应统一附加：
   ```json
   { "items": [...], "new_count": 12, "cursor": "下次传入的 since 值" }
   ```
3. 行为：带 `since` 时抓到已见过的条目即停止翻页（likes incremental 的现有模式，`grep -r "incremental\|since" go-server/` 参考现有实现），只返回新增；不带 `since` 时按现状全量（受 limit 限制）。
4. 落库仍按各平台表 upsert（幂等），`since` 只影响"翻页停在哪"和"响应里返回什么"。

## 改动 2：统一查询出口 `/api/records`

新增只读 endpoint（Protected `/api` 组）：

```
GET /api/records?platform=twitter,xiaohongshu&since=2026-07-01T00:00:00Z&q=关键词&limit=100&offset=0&format=json
```

| 参数 | 说明 |
|------|------|
| `platform` | 逗号分隔，缺省=全部已有平台 |
| `since` / `until` | 按抓取时间（fetched_at）过滤，RFC3339 |
| `q` | 对标题/正文 LIKE 匹配（先不做 FTS） |
| `limit`/`offset` | 默认 100，上限 1000 |
| `format` | `json`（默认）/ `jsonl` / `markdown` |

**实现方式**：不建新表。为每个平台表写一个到统一最小字段集的映射函数（Go 代码层即可，不必建 SQL VIEW），逐表查询后合并按时间倒序：

```json
{
  "platform": "twitter",
  "id": "平台内 ID",
  "url": "原始链接",
  "author": "作者名/handle",
  "title": "标题（无标题平台为空）",
  "content": "正文/Markdown",
  "created_at": "内容发布时间（RFC3339，未知为空）",
  "fetched_at": "抓取时间（RFC3339）",
  "source": "平台内意图，如 likes/search/saved"
}
```

新增平台适配器时必须同时提供该映射函数（加入 Spec 05 的适配器清单检查项）。

`format=jsonl` 每行一个对象（Content-Type `application/x-ndjson`）；`format=markdown` 输出按平台分节的 Markdown 列表（标题链接 + 作者 + 时间 + 正文摘要 200 字）。

## 改动 3：MCP 与 skill

1. 新增 MCP 工具 `records_query`，参数与 `/api/records` 一致（format 固定 json），描述写明"查询本地已归档的所有平台内容"。
2. skill（`skills/grabby/`）：`scripts/grabby-api.sh` 加 `records` 子命令包装该 endpoint；SKILL.md 的触发词与用法表同步（"我最近点赞/收藏了什么"、"所有平台过去 24 小时的新内容"）。
3. `docs/api-reference.md` 补文档。

## 验收标准

1. 抓一轮 twitter likes 和小红书笔记后，`GET /api/records?limit=10` 返回两个平台混排、按 fetched_at 倒序的统一结构
2. `platform=twitter&q=xxx` 过滤正确
3. `format=jsonl` 可被 `jq` 逐行解析；`format=markdown` 人眼可读
4. `twitter.likes` 带上次返回的 `cursor` 作为 `since` 再抓 → 只返回新增条目，`new_count` 正确，重复条目不重复入库（表行数不变）
5. MCP client（或 skill 脚本）能通过 `records_query` 拿到与 HTTP 相同的结果
