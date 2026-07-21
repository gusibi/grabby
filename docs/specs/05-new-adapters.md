# Spec 05：新平台适配器——YouTube、B 站、Reddit saved、小红书收藏

> 平台范围已定，**只做本文四项，不再加其他平台**。
> 前置约束（`CONTEXT.md`）：每平台独立适配器/独立 HTTP API/独立内容表，只共享底层原语；不做风控；大体量低时效数据（评论、字幕）实时返回不落库。
> 实现模板：Reddit 适配器（`grep -rn "reddit" go-server/ --include="*.go" -l` 定位全部文件；PRD 见 `docs/prd-likes-incremental-and-reddit.md`）。每个新适配器交付物 = HTTP API + record 表 + MCP 工具 + skill 脚本 + `apis/*.yml` 请求示例 + `/api/records` 映射函数（Spec 04）+ 文档。

## 统一叙事

这批适配器围绕"**你的点赞和收藏，是你自己的数据**"：Twitter likes（已有）+ Reddit saved + 小红书收藏 + B 站收藏夹 + YouTube 稍后观看，全部支持 Spec 04 的 `since` 增量。

## 实现顺序

按风险从低到高：1. Reddit saved → 2. 小红书收藏 → 3. B 站 → 4. YouTube。前两个几乎全是复用，先做建立模式；YouTube 的 ytInitialData 解析最繁琐，放最后。

---

## 1. Reddit saved

**API**：`POST /api/reddit/saved`，body `{"limit": 100, "since": "..."}`（browser 参数与其他接口一致，下同）。

**抓取链路**（全部复用 `fetchInPage` 原语，与 reddit.subreddit 同构）：
1. 打开 `https://www.reddit.com/`，页面内 fetch `https://www.reddit.com/api/me.json` → `data.name` 得到当前用户名。拿不到 name → 返回 `NOT_LOGGED_IN`。
2. 页面内 fetch `https://www.reddit.com/user/{name}/saved.json?limit=100`，用 `after` 游标翻页（每页 100，上限受 `limit`）。
3. 解析字段与 reddit.subreddit 相同（id/title/author/subreddit/url/score/num_comments/created_utc/selftext 摘要）；saved 里可能混有评论类型（`kind: "t1"`），一并返回但标注 `"kind": "comment"`，其 title 用评论所属帖子标题（`link_title` 字段）。
4. `since` 增量：翻页遇到 `since` 对应的条目 ID 即停。

**落库**：upsert 进现有 `reddit_posts` 表，`source` 置 `"saved"`（若表无 source 列则加列，默认 `'subreddit'`；确认现有表结构后做迁移）。

**MCP 工具**：`reddit_saved`。**skill**：`scripts/reddit.sh` 加 `saved` 子命令。

## 2. 小红书收藏

**API**：`POST /api/xiaohongshu/collected`，body `{"profile_url": "...", "limit": 50, "since": "..."}`。`profile_url` 为用户主页地址（与现有 user-notes 相同格式）；省略时返回 400 提示需要用户主页 URL（不猜测）。

**抓取链路**（复用现有 `xiaohongshu.userNotesInitialState` 的模式，`chrome-extension/background.js` 中该白名单脚本是模板）：
1. 打开 `{profile_url}?tab=fav`（收藏 tab）。
2. 新增白名单脚本 `xiaohongshu.collectedInitialState`：与 userNotes 版逻辑相同（轮询 `window.__INITIAL_STATE__`，失败则从 `<script>` 文本解析），但读取收藏数据的 key。**实现时需实测确认 key 路径**（预期在 `user.collect` 或 `user.collectedNotes` 附近；打开收藏 tab 后在 console 里 `Object.keys(window.__INITIAL_STATE__.user)` 确认）。找不到数据且页面正常 → `EMPTY_RESULT`；页面跳登录 → `NOT_LOGGED_IN`。
3. 解析字段与 user-notes 相同（noteId/title/type/xsecToken/author/likedCount + 拼出带 xsec_token 的笔记 URL）。

**落库**：upsert 进现有 `xhs_notes` 表，`source` 置 `"collected"`（同样确认/添加 source 列）。

**MCP 工具**：`xiaohongshu_collected`。**skill**：`scripts/xiaohongshu.sh` 加 `collected` 子命令（必传 `profile-url`）。

## 3. B 站（bilibili）

新建独立适配器，三个原子能力，全部走 `fetchInPage`（打开 `https://www.bilibili.com/` 提供 Cookie/Origin 上下文，requestUrl 指向 `api.bilibili.com`）。B 站 web API 返回 `{"code":0,"data":{...}}`，`code!=0` 时：`code=-101` → `NOT_LOGGED_IN`，其他 → `FETCH_FAILED` 并附 message。

### 3a. `POST /api/bilibili/dynamics` — 关注动态

- 页面内 fetch `https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?type=all&page=1`，用响应里 `data.offset` 翻页，`limit` 默认 50。
- 解析 `data.items[]`：`id_str`、`modules.module_author.name`/`pub_ts`、类型（`type`：视频/图文/转发）、标题与文字内容（视频在 `modules.module_dynamic.major.archive.title`，图文在 `module_dynamic.desc.text`——字段路径以实测为准，fixture 落地后写死）、跳转 URL。
- `since` 增量：遇到已见 `id_str` 停止。

### 3b. `POST /api/bilibili/favorites` — 收藏夹

1. 页面内 fetch `https://api.bilibili.com/x/web-interface/nav` → `data.mid` 得到当前用户 mid（`isLogin:false` → `NOT_LOGGED_IN`）。
2. fetch `https://api.bilibili.com/x/v3/fav/folder/created/list-all?up_mid={mid}` → 收藏夹列表。body 可选 `folder`（收藏夹名），缺省用第一个（默认收藏夹）。
3. fetch `https://api.bilibili.com/x/v3/fav/resource/list?media_id={id}&pn=1&ps=20` 翻页取内容：`bvid`、`title`、`upper.name`、`intro` 摘要、`ctime`/`fav_time`、封面、`https://www.bilibili.com/video/{bvid}` 链接。
4. `since` 增量：遇到已见 bvid 停止。

**落库**：新表 `bilibili_items`（id TEXT 主键=id_str 或 bvid、type、title、author、content、url、published_at、source `"dynamics"|"favorites"`、raw JSON、fetched_at）。字段对齐其他平台表的既有风格（先看 tweets 表怎么建的，保持一致）。

**MCP 工具**：`bilibili_dynamics`、`bilibili_favorites`。**skill**：新建 `scripts/bilibili.sh`（`dynamics` / `favorites` 子命令），SKILL.md 表格与触发词同步。

## 4. YouTube

新建独立适配器，三个原子能力。

### 4a. `POST /api/youtube/subscriptions` — 订阅流

- 新增白名单脚本 `youtube.initialData`：返回 `window.ytInitialData ?? null`（一行，加在 `background.js` 白名单 switch）。
- 打开 `https://www.youtube.com/feed/subscriptions`，跑该脚本。
- 解析（路径繁琐，务必先存一份真实 ytInitialData 做 fixture 再写解析）：`contents.twoColumnBrowseResultsRenderer.tabs[0].…richGridRenderer.contents[]` 里的 `richItemRenderer.content.videoRenderer`：`videoId`、`title.runs[0].text`、频道 `ownerText.runs[0].text`、`publishedTimeText`（相对时间字符串，原样存）、`viewCountText`、`lengthText`。跳到登录页/consent 页 → `NOT_LOGGED_IN`。
- `limit` 默认 50（首屏即有几十条，不做滚动翻页，够用）。

### 4b. `POST /api/youtube/watch_later` — 稍后观看

- 打开 `https://www.youtube.com/playlist?list=WL`，跑 `youtube.initialData`。
- 解析 `…playlistVideoListRenderer.contents[]` 的 `playlistVideoRenderer`：videoId/title/频道/时长/index。
- `since` 增量：遇到已见 videoId 停止。

### 4c. `POST /api/youtube/transcript` — 视频字幕

- body `{"url": "https://www.youtube.com/watch?v=..."}`，两步链路：
  1. `runPageScript`：打开视频页跑已有白名单脚本 `youtube.initialPlayerResponse`，取 `captions.playerCaptionsTracklistRenderer.captionTracks[]`；选择规则：优先 `languageCode` 前缀 `zh` → `en` → 第一条；无 captions → `EMPTY_RESULT`（错误信息注明"该视频无字幕"）。
  2. `fetchInPage`：页面 `https://www.youtube.com/watch?v=...`，requestUrl 为所选 track 的 `baseUrl` 追加 `&fmt=json3`；解析 `events[].segs[].utf8` 拼接为纯文本，按 `tStartMs` 生成 `[mm:ss]` 段落时间戳。
- **字幕实时返回不落库**（大体量低时效，同评论规则）。

**落库**：新表 `youtube_videos`（video_id 主键、title、channel、url、published_text、duration、source `"subscriptions"|"watch_later"`、raw、fetched_at）。

**MCP 工具**：`youtube_subscriptions`、`youtube_watch_later`、`youtube_transcript`。**skill**：新建 `scripts/youtube.sh`。

---

## 每个适配器的统一检查清单（DoD）

- [ ] HTTP API（错误响应 `{"detail","error_code"}`；无浏览器 503；未登录 `NOT_LOGGED_IN`）
- [ ] 缓存分类注册（Spec 03：dynamics/subscriptions 属 LIST；transcript 属 DETAIL；favorites/saved/collected/watch_later 属 NONE+增量）
- [ ] `since` 增量（个人集合类）
- [ ] record 表 + upsert 幂等（同 ID 重复抓不增行）
- [ ] `/api/records` 映射函数（Spec 04）
- [ ] MCP 工具注册
- [ ] skill 脚本 + SKILL.md 用法表/触发词
- [ ] `apis/{name}.yml` 请求示例（格式照抄 `apis/twitter_likes.yml`）
- [ ] 脱敏响应 fixture + 解析单元测试
- [ ] `docs/api-reference.md` 文档

## 验收标准（人工端到端）

1. 浏览器登录对应平台后，四组 API 各能真实抓回数据；未登录时返回 `NOT_LOGGED_IN` 而非空成功
2. Reddit saved / 小红书收藏 / B 站收藏 / 稍后观看均支持 `since`：第二次带 cursor 只返回新增
3. `GET /api/records?platform=bilibili,youtube` 能查到新平台归档
4. skill 脚本不带参数打印用法；带参数端到端可用
