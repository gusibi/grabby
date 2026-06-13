# Grabby 升级方案：从「HTML→Markdown」到「浏览器执行器」（v2.1）

> 目标：让 Grabby 不只是把网页转 Markdown，而是能在用户真实浏览器里**带登录态地取数据、调站内 API，乃至执行操作**，从而覆盖 YouTube 字幕、Twitter 搜索、Reddit、小红书、B站、网页搜索等当前做不到的场景。
>
> **v2 说明**：本版已合并评审意见（见 `browser-executor-plan-review.md`）。主要变化：① 任意 `eval` 改为白名单脚本；② `intercept` 明确早注入；③ `fetchInPage` 不再假设万能；④ 后台 tab 增加 `visible` 声明与恢复原 tab；⑤ 协议采用**新旧共存、不改现有协议**的策略（与评审「先升级协议」的措辞不同，理由见 §6）。
>
> **v2.1 变更**：① 根据产品确认，Twitter **timeline** 与 **likes 同步** 是两个高频刚需场景，已从 backlog 提升为一等 MVP 场景（见 §7）。② 合并第二轮 MV3 实现细节评审：`runPageScript` 注入函数不能访问外部闭包，改为内联静态 `switch`（§4.1）；`intercept` 补 MAIN world→bridge 回传通道与动态配置注入机制（§4.3）；协议新增能力协商握手（§6.1）；`twitter_likes` 加可访问性验收前置项（§7.2）。

## 1. 核心洞察：浏览器插件是「降维打击」的位置

当前失败的场景，失败原因只有三类：

1. **反爬 / 风控**（Reddit 403、B站拦截、Twitter 收费）—— 本质是服务器 IP + 无浏览器指纹 + 无登录态被识别。
2. **登录墙**（小红书、Twitter 搜索）—— 服务端没有用户身份。
3. **内容解析**（HTML 转不干净、字幕拿不到）—— 解析能力问题。

浏览器插件天然同时解决前两类：它运行在**用户真实浏览器**里，带着用户的登录态、Cookie、真实指纹、住宅 IP。Reddit 不会 403 用户自己的浏览器，小红书用户本来就登着，Twitter 用户能免费看。这是任何服务端爬虫（包括付费 API）都做不到的位置优势。

因此正确架构不是「插件抓 HTML → 服务端解析」，而是把插件升级成**浏览器执行器**，服务端做**调度 + 站点适配 + 解析**。

```
MCP Client (Claude 等)
   │ MCP
服务端（调度器 + 站点适配层 + 解析层）
   │ WebSocket（现有连接）
浏览器插件（执行器：导航 / 取数据 / 操作 / 截图）
```

## 2. 现状盘点（基于实际代码）

**现有链路**（`go-server/internal/interfaces/mcp/server.go:79` → `chrome-extension/background.js:344`）：

MCP `extract` 工具 → `wm.SendMessage({command:"extract", url})` → background.js `handleExtractCommand` → `navigateToUrl`（`chrome.tabs.create`，**`active:true` 会抢用户焦点**）→ `waitForPageStable`（滚动 + 轮询文本长度处理懒加载，已写得不错）→ defuddle 抽正文 → Markdown → 关 tab。

**已具备的底子（比预期好）：**

- 命令分发器已有 `capture` / `extract` / `navigate` / `open` 四个 case（`background.js:258`）。
- 已有 `scripting`、`offscreen`、`tabs`、`<all_urls>` 权限（`manifest.json:39`）。
- 已有 WebSocket 双向通信、`message_id` 请求-响应配对、稳定等待懒加载逻辑。

**缺口：** 所有取数据都走「DOM → defuddle → Markdown」一条路。但目标场景的数据大多**不在可见 DOM 里**——它们在页面 JS 变量、XHR/GraphQL 响应，或需要调带登录态的站内 API。

**协议现状（重要）：** 现有 `BrowserRequest` 只能表达 `command/url/fullPage/browser`，`BrowserResponse.Result` 主要服务截图和 Markdown，缺通用 JSON/raw text/结构化承载。**但本方案不修改它**（见 §6）。

**待清理：** MCP 层还挂着 `add`（`server.go:100`）和 `get_server_time`（`server.go:117`）两个模板残留工具。

## 3. 关键决策（已确认）

| 决策 | 选择 | 说明 |
|---|---|---|
| 拦截 XHR/GraphQL 响应的方式 | **注入 fetch hook** | MAIN world 覆写 `window.fetch` / `XMLHttpRequest`，无「正在调试此浏览器」黄条。`chrome.debugger`/CDP 作为可选高级模式留接口。 |
| 写类操作（发帖/点赞/关注）本期是否做 | **本期只做读** | 先打通读取场景；写类原语与确认闸门先设计、放下一期。 |
| 抓取时后台 tab 是否抢焦点 | **默认后台加载** | `navigateToUrl` 走 `active:false`；个别站点适配器可声明 `visible` 短暂激活并恢复原 tab。 |
| 页面脚本执行方式（评审修正） | **白名单脚本，不接受任意 JS** | 见 §4.1。 |
| 拦截 hook 注入时机（评审修正） | **早注入（document_start）** | 见 §4.3。 |
| 协议演进策略（本轮新增约束） | **新旧共存，不改现有协议** | 见 §6。 |

## 4. 核心改造：加底层原语（新命令，旧命令不动）

在 `background.js` 的 `switch(command)`（`background.js:258`）里**新增** case，复用现有 `navigateToUrl`。现有 `capture`/`extract`/`navigate`/`open` 四个 case 保持不变。

### 4.1 `runPageScript` —— 白名单页面脚本（替代任意 eval）

> **评审修正**：原 v1 方案在用户登录态 MAIN world 跑任意字符串 JS（`func:(expr)=>eval(expr)`），一旦服务端适配器输入被污染即为任意页面脚本执行。**改为内置脚本白名单**。

扩展侧只允许执行内置、静态的脚本（不接受任意 JS 字符串）：

```json
{
  "command": "runPageScript",
  "url": "https://www.youtube.com/watch?v=...",
  "script": "youtube.initialPlayerResponse",
  "params": { "videoId": "..." }
}
```

初始白名单示例：`youtube.initialPlayerResponse`、`bilibili.initialState`、`page.extractJsonLd`、`page.readMeta`。

实现要点（`world:'MAIN'` 仍是必须，否则拿不到页面 `window` 变量）：

> **MV3 陷阱（评审修正）**：`chrome.scripting.executeScript` 的 `func` 会被**序列化后注入**，注入时丢失定义时的闭包/外部作用域——它**访问不到 service worker 侧定义的 `SCRIPTS` 表或任何外部变量**，`self.__grabbyScripts` 这种全局表在页面 MAIN world 里也并不存在。正确做法是把白名单内联在注入函数体里用静态 `switch(name)` 分发（或注入固定脚本文件），参数只走 `args`。参考：[chrome.scripting func/args 文档](https://developer.chrome.com/docs/extensions/reference/api/scripting)。

```js
async function handleRunPageScript(data) {
  const tab = await navigateToUrl(data.url, /*active=*/ data.visible === true);
  const [{ result }] = await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    world: 'MAIN',
    // 注意：func 被序列化注入，访问不到外部闭包；白名单必须内联在函数体内
    func: (name, params) => {
      switch (name) {
        case 'youtube.initialPlayerResponse':
          return window.ytInitialPlayerResponse ?? null;
        case 'bilibili.initialState':
          return window.__INITIAL_STATE__ ?? null;
        // ...其余白名单脚本
        default:
          return { __error: 'script not allowed' };
      }
    },
    args: [data.script, data.params || {}]  // name/params 作为结构化 args 传入
  });
}
```

若白名单脚本较多、不便内联，替代方案是把它们打包成固定脚本文件，用 `executeScript({ files: ['scripts/pageScripts.js'] })` 注入，再通过约定的入口分发——但**仍不能接受任意 JS 字符串**。

**关键安全细节**：白名单脚本接收 `params` 时**必须作为 `executeScript` 的结构化 `args` 传入，脚本体保持静态常量**，绝不把 `params` 拼接进脚本字符串——否则等于换个地方重新引入注入，allowlist 形同虚设。

### 4.2 `fetchInPage` —— 用页面的 fetch 发请求（带登录态，但不万能）

在 MAIN world 里 `await fetch(url, opts).then(r => r.text())`，在页面上下文中发请求，**更接近站点真实请求**（同源、共享页面 Cookie 与 Origin）——但**仍需显式处理 `credentials`、CORS、CSRF token 和动态 header**，不能假设自动搞定。

> **评审修正**：不能假设它解决所有站内 API。仍可能遇到 CORS、SameSite Cookie、`credentials` 默认值、CSRF token、站点自定义 header、GraphQL bearer token、Referer/Origin/transaction-id 等动态参数。

适用边界：

- **够用**：Reddit `.json`、YouTube timedtext 这类简单 GET。
- **不够用**：Twitter/X、小红书等——更多时候要靠 §4.3 `intercept` **复用页面自己发出的真实请求/响应的 header/token**，而不是手写请求。

### 4.3 `intercept` —— 拦截页面 XHR/GraphQL 响应（注入 fetch hook，必须早注入）

> **评审修正（最关键）**：若页面加载完成后再注入 hook，首批关键请求（Twitter 搜索首屏 GraphQL、小红书首屏接口）可能已发完，必然漏掉。**必须早注入。**

设计流程：

1. 在创建 tab 前，把本次 intercept 配置（patterns / limit / sessionId）写入 `chrome.storage.session`（或后台内存），按 `tabId` 或随机 `nonce` 为 key。
2. 创建 tab。
3. 在尽可能早的阶段（`document_start`）注入**通用捕获 hook**，覆写 `window.fetch` / `XMLHttpRequest`，缓存所有响应。
4. 再导航 / reload 到目标 URL。
5. 等待匹配响应；按需滚动触发后续分页请求。
6. bridge 按配置过滤、回传匹配结果。

#### MV3 关键实现细节（评审修正）

**(a) 回传通道：MAIN world hook 不能直接调扩展 API。** MAIN world 与页面共享执行环境，能覆写页面 `fetch/XHR`，但**拿不到 `chrome.runtime`** 等扩展 API。因此必须双层结构经 ISOLATED world 的 bridge 中转：

```text
MAIN world hook（覆写 fetch/XHR，捕获响应）
  -> window.postMessage(捕获到的数据)
  -> ISOLATED bridge content script（监听 message，可访问 chrome.runtime）
  -> chrome.runtime.sendMessage / 经 background WebSocket 回传服务端
```

content scripts 默认运行在隔离世界、可用扩展 API；MAIN world 则与页面共享、不可用——这正是需要 bridge 的原因。参考：[content scripts isolated worlds](https://developer.chrome.com/docs/extensions/develop/concepts/content-scripts)。

**(b) 动态配置注入：`document_start` 脚本是静态的，不能像函数调用那样直接带每次命令的 patterns/limit。** 解决：hook 只装**通用捕获器**（不含业务过滤逻辑），具体的 patterns/limit/sessionId 由 step 1 预写入 `chrome.storage.session`（按 `tabId`/`nonce` 隔离），bridge 读取配置后再做过滤与回传。`RegisteredContentScript` 支持 `runAt:'document_start'` 与 `world` 字段，但**配置如何传进去要自行设计**。参考：[RegisteredContentScript runAt/world](https://developer.chrome.com/docs/extensions/reference/api/scripting#type-RegisteredContentScript)。

MV3 落地手段（按优先级）：

- 动态 content script（`chrome.scripting.registerContentScripts`）+ `runAt:'document_start'` + `world:'MAIN'`（hook）配合 ISOLATED bridge（首选）。
- `chrome.scripting.executeScript` 注入后立即 `chrome.tabs.reload`。
- 仅高级模式考虑 `chrome.debugger` / CDP（有调试提示、权限感知更强）。

### 4.4 `navigateToUrl` 后台加载 + 站点可见性声明

> 现有 `navigateToUrl`（`background.js:466`）写死 `active:true`。**注意：直接改它会影响现有 `extract`/`capture`/`navigate`/`open` 行为。** 因此加参数而非改默认行为——现有命令仍传 `active:true`（或不传，保持旧行为），新命令默认传 `active:false`。

> **评审修正**：后台不可见 tab 对无限流页面（Twitter timeline、小红书搜索结果、可见性检测站点）可能不触发懒加载。站点适配器可声明：

```json
{ "visible": true, "restorePreviousTab": true, "scrollPlan": "timeline" }
```

即默认后台执行，但适配器可要求短暂激活，结束后恢复用户原 tab。

## 5. 站点适配器：每个场景一个策略

适配器放在服务端（新增 `go-server/internal/.../siteadapter/`，每站一个文件），把「业务意图」翻译成原语调用。统一降级链：**拦截响应 > 读页面变量（白名单脚本）> DOM 解析**。

| 场景 | 适配器策略 | 用到的原语 |
|---|---|---|
| YouTube 字幕 | 读 `ytInitialPlayerResponse` 拿字幕轨 URL，再 fetch timedtext | `runPageScript` + `fetchInPage` |
| Reddit | URL 加 `.json` 后缀直接请求 | `fetchInPage` |
| Twitter 搜索 | 早注入拦截 `SearchTimeline` GraphQL 响应；DOM 兜底 | `intercept` |
| Twitter timeline（刷内容） | 早注入拦截 `HomeTimeline`/`HomeLatestTimeline`；滚动分页 + 时间去重 | `intercept` + 滚动分页 |
| Twitter likes（同步收藏） | 打开 `x.com/<handle>/likes`，拦截 `Likes` GraphQL；增量 `since` 语义 | `intercept` + 滚动分页 |
| 小红书 | 早注入拦截 `/api/sns/web/`；控频 + 随机延迟 + 失败冷却 | `intercept` |
| B站视频 | 读 `__INITIAL_STATE__` + 调字幕 API（站内 JS 算 WBI 签名） | `runPageScript` + `fetchInPage` |
| 网页搜索 | 打开 Google/Bing/DuckDuckGo 结果页，解析或拦截结果 | `runPageScript` / DOM |
| 通用网页 → Markdown | 现有 defuddle 链路（可叠加 Readability.js） | 现状（`extract`） |
| GitHub | **不走浏览器**，服务端直连 REST/GraphQL API | 服务端 HTTP |

## 6. 协议策略：新旧共存，不修改现有协议

> **本轮新增约束（与评审措辞分歧点）**：评审建议「先把 `BrowserRequest`/`BrowserResponse` 升级成通用结构再做适配」。我们**采纳"需要通用承载"的实质，但不采纳"修改现有结构"的做法**——修改现有协议会破坏 MCP client、服务端、浏览器端三方的前向兼容，使老版本扩展与新服务端无法继续交互。

原则：

- **现有命令（`capture`/`extract`/`navigate`/`open`/`screenshot`）的请求与响应结构完全不动。** 老扩展、老客户端继续按旧协议工作。
- **新命令（`runPageScript`/`fetchInPage`/`intercept`）使用新增字段承载**，新增字段一律 `omitempty`，对旧端不可见、不影响解析。
- 通过 `command` 取值区分新旧路径；服务端与扩展对未知 `command` 已有「未知指令」兜底（`background.js:278`），天然向后兼容。

### 6.1 能力协商（评审修正，必做）

> 仅靠「未知指令兜底」不够：老扩展遇到 `runPageScript`/`intercept` 只会回「未知指令」，但服务端若**盲目**向 MCP client 注册 `twitter_timeline` 等工具，用户调用时才发现扩展不支持，体验是「工具存在但永远失败」。

机制：扩展在**连接/注册时上报 `capabilities` 与 `version`**——支持哪些 command、哪些站点适配能力。服务端据此：

- 按能力**动态注册/启用** MCP 工具（扩展不支持就不暴露对应工具）；
- 或在调用时返回**明确错误**：「扩展版本过低，请升级到 ≥ x.y 以使用 `twitter_timeline`」。

示例握手（扩展 → 服务端）：

```json
{
  "type": "register",
  "version": "2.1.0",
  "capabilities": ["capture", "extract", "navigate", "open",
                   "runPageScript", "fetchInPage", "intercept"]
}
```

这样新旧扩展可与同一服务端共存，且工具可用性对用户透明。

新增字段（**追加**到现有 `BrowserRequest`，不删不改既有字段）：

```go
// 既有字段保持原样：Source/Action/Command/URL/Browser/FullPage/MessageID ...
// 以下为新增，仅新命令使用：
Params    map[string]any `json:"params,omitempty"`
TimeoutMs int            `json:"timeoutMs,omitempty"`
Visible   bool           `json:"visible,omitempty"`
CloseTab  bool           `json:"closeTab,omitempty"`
```

响应侧同理，**在现有 `BrowserResult` 上追加**可选承载字段，旧字段（`ImageData`、`Content` 等）不动：

```go
// 既有：URL/Title/Timestamp/Content/ImageData ...
// 新增（omitempty，仅新命令填充）：
Text  string         `json:"text,omitempty"`
JSON  map[string]any `json:"json,omitempty"`
Items []any          `json:"items,omitempty"`
```

> 实现 nit：Go `encoding/json` 中 struct 类型字段的 `omitempty` 不生效（struct 非 empty-able）。若 `Result`/`Content` 这类要真正省略，需用指针类型或自定义编组——但因为我们不改既有字段，这点仅影响新加的嵌套结构。

配套需同时考虑：单次响应大小限制、超时控制、tab 生命周期（是否关闭临时 tab）、是否允许跨域 fetch、域名 allowlist、站点级限流。

## 7. Twitter timeline 与 likes 同步（一等 MVP 场景）

> **产品确认**：这是两个高频刚需，不是可选 backlog。与 `twitter_search` 共享底层原语（早注入 `intercept` + 滚动分页），但意图、解析路径与返回语义各不相同，**作为三个独立的意图级工具**。

### 7.1 `twitter_timeline` —— 刷今天值得看的内容

- 用途：读取用户 home timeline，挑选当天值得看的内容。
- 入口：`x.com/home`；拦截 `HomeTimeline`（推荐流）或 `HomeLatestTimeline`（最新流）GraphQL 响应。
- 强依赖登录态。
- 需要**滚动分页**触发后续 `cursor` 请求，按时间/页面顺序聚合。
- 返回需**去重**（同一 tweet 多次出现）、**截断**（`limit`）、结构化（作者、正文、时间、媒体、链接、互动数）。

```text
twitter_timeline(kind: "for_you" | "following", limit?, since?, browser?)
```

### 7.2 `twitter_likes` —— 同步我的 Like 用于收藏存档

- 用途：把用户（常在手机上）点的 Like 拉取下来，由另一任务存档/收藏。属**增量同步**，不是一次性抓取。
- 入口：`x.com/<handle>/likes`；拦截 `Likes` GraphQL 响应。
- 强依赖登录态（自己的 likes 页需登录）。
- 滚动分页直到**命中上次同步位置即停**——这是与 timeline 的关键差异：

```text
twitter_likes(handle?, since?, max_items?, browser?)
```

- `since`：上次同步到的最新 tweet id / 时间戳。适配器滚动到遇到已知项即停，只回传增量，避免每次全量重刷（也降低风控）。
- `handle` 省略时默认当前登录用户。
- **验收前置项**：X 的 likes 页可见性与页面行为变动频繁，**不要把 `/<handle>/likes` 稳定可访问写成强假设**。实现时先验证当前 X 页面结构与 `Likes` GraphQL 接口仍可访问，并在适配器里对「页面结构变化 / 接口改名 / likes 不可见」给出明确错误分类，而非静默返回空。

### 7.3 三者关系

| 工具 | 入口 | GraphQL | 分页停止条件 | 语义 |
|---|---|---|---|---|
| `twitter_search` | search 页 | `SearchTimeline` | 到 `limit` | 一次性查询 |
| `twitter_timeline` | `/home` | `HomeTimeline` / `HomeLatestTimeline` | 到 `limit` | 一次性快照 |
| `twitter_likes` | `/<handle>/likes` | `Likes` | 命中 `since` 或到 `max_items` | 增量同步 |

三者都需：登录态检测、滚动分页、去重、明确错误分类（未登录 / 页面结构变化 / 请求未捕获 / 超时）、站点级限流与随机延迟。由于都依赖无限流，建议默认 `visible:true` 短暂激活并在结束后恢复用户原 tab（§4.4）。

## 8. 改造清单（按文件）

### A. `chrome-extension/background.js` —— 加原语
- `switch(command)`（`background.js:258`）**新增** `runPageScript` / `fetchInPage` / `intercept` 三个 case；现有四个 case 不动。
- `navigateToUrl`（`background.js:466`）加 `active` 参数；现有命令保持旧行为，新命令默认后台。
- `intercept` 走 `document_start` 早注入（动态 content script 或注入后 reload）。
- 处理后台 tab 下 `waitForPageStable` 懒加载失效（按 §4.4 的 `visible` 声明）。

### B. `go-server/internal/interfaces/mcp/server.go` —— 加意图级工具、删残留
- 删除 `add`（`server.go:100`）和 `get_server_time`（`server.go:117`）。
- **不把底层原语直接暴露给 MCP client。** 暴露意图级工具：`youtube_transcript(url)`、`reddit_thread(url)`、`twitter_search(query)`、`twitter_timeline(...)`、`twitter_likes(...)`（§7）、`web_search(query)`、`xiaohongshu_note(url)`、`bilibili_video(url)`。
- 保留通用 `extract` / `screenshot`。

### C. 协议（按 §6，新增不改旧）
- `BrowserRequest` 追加 `Params/TimeoutMs/Visible/CloseTab`（`omitempty`）。
- `BrowserResult` 追加 `Text/JSON/Items`（`omitempty`）。
- 现有字段与现有命令的序列化结构零改动。

### D. 新增站点适配器层
- `go-server/internal/.../siteadapter/`，每站一个文件，按降级链实现。
- 例：`reddit.go` = URL 加 `.json` → 发 `fetchInPage` → 解析 JSON → 转 Markdown。

### E. GitHub 特例
- 不进适配器、不走浏览器，服务端直连 `api.github.com`。

## 9. 落地顺序（每步端到端可验证）

1. **协议最小追加 + `fetchInPage` + Reddit 适配器**（最小闭环）。协议字段与 Reddit 试点**一起做、互相验证**——用最简单场景反推协议真正需要的字段，避免先凭空设计完整协议。验收：老 `extract`/`screenshot` 不破坏；`reddit_thread(url)` 返回标题/正文/评论。
2. **`runPageScript`（白名单）+ YouTube 字幕**。验收：`youtube.initialPlayerResponse` 能读字幕轨，再 fetch timedtext 返回字幕文本。
3. **`runPageScript` + B站字幕**（页面内算 WBI 签名）。验收：`bilibili.initialState` 读视频基础信息。
4. **早注入 `intercept` + Twitter 搜索**。验收：捕获 `SearchTimeline` 响应、解析 tweet 列表、支持 limit/去重/超时。
5. **`twitter_timeline` + `twitter_likes`**（复用第 4 步的早注入 intercept + 滚动分页）。验收：登录态下 `twitter_timeline` 能返回 home 内容并去重截断；`twitter_likes` 能滚动到 `since` 即停、只回传增量。
6. **小红书**（intercept + 站点级限流 + 随机延迟 + 失败冷却 + 结果上限 + 账号风险提示，风控最严，最后做）。
7. **web_search**（解析 Google/Bing 结果页）。

## 10. 风险与注意事项

**安全**
- 任意 JS 执行风险 → 已用 §4.1 白名单脚本消解；底层原语仅服务端内部使用，不暴露给 MCP client。
- 白名单脚本的 `params` 必须结构化传参，禁止字符串拼接。
- 限制可 fetch 的域名 allowlist；对敏感站点加显式配置开关。
- 返回内容可能含隐私数据，需注意脱敏/最小化。

**稳定性**
- X/Twitter、小红书的接口与 GraphQL 名称经常变动 → 适配器按站点维护、明确降级链与错误分类（未登录 / 页面结构变化 / 请求未捕获 / 超时）。
- DOM 兜底易受改版影响，仅作最后一档。
- `intercept` 必须早注入，否则漏首屏请求。
- 后台 tab 不一定触发懒加载 → 无限流页面用 `visible` 模式。

**账号风控**
- 高频搜索/timeline 滚动可能触发平台风控 → 每站单独限流、随机延迟、最大页数/item 数上限、失败后冷却。
- 默认只读；写类操作（下一期）必须具备：操作预览、用户显式确认、站点级权限开关、操作审计日志、明确失败/取消语义。

## 11. 评审沟通问题的回应（对照 review §「建议沟通的重点问题」）

1. 废弃任意 `eval` 改 allowlist —— **接受**（§4.1），并补充 params 不可拼接。
2. 先做协议升级再做适配 —— **部分接受**：采纳「需要通用承载」，但**不改现有协议，改为新增字段共存**（§6），且协议与 Reddit 试点共同演进。
3. `intercept` 早注入 —— **接受**（§4.3），`document_start` 注入。
4. `twitter_timeline` 纳入范围 —— **接受并扩展**：经产品确认，timeline 与 likes 同步均为高频刚需，已提升为一等 MVP 场景，拆为 `twitter_timeline` 与 `twitter_likes` 两个独立工具（§7）。
5. 小红书放最后 + 限流冷却 —— **接受**（§9 第 5 步、§10）。
6. 第一期只做读 —— **接受**（§3）。
7. 敏感站点显式开关 —— **接受**（§10 安全）。
