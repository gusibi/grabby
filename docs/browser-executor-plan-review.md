# Grabby 浏览器执行器升级方案评审

## 总体结论

方案整体方向可行，且和当前 Grabby 的代码基础匹配：现有浏览器扩展已经具备 WebSocket 双向通信、请求响应配对、标签页导航、截图、正文提取等能力，把它升级为“浏览器执行器”是合理演进。

但这不是一个只在 `background.js` 里加几个 case 的小改动。真正需要先完成的是：扩展服务端和浏览器扩展之间的请求/响应协议，定义安全边界，再逐步实现站点适配器。尤其是 `eval`、`fetchInPage`、`intercept` 三个底层原语，不能按方案中的示例直接实现，否则会带来安全、稳定性和账号风控风险。

建议结论：

- 架构方向可以继续推进。
- MVP 应先做受控的读能力，不做发帖、点赞、关注等写操作。
- 不应把底层浏览器原语直接暴露给 MCP client。
- 需要先补充协议设计、安全约束、早注入机制和站点级限流，再进入 Twitter/X、小红书等高风险站点。

## 当前代码基础判断

当前 Grabby 的实际链路是：

```text
MCP extract/screenshot
  -> Go WebSocketManager.SendMessage
  -> BrowserRequest{command, url, fullPage}
  -> chrome-extension/background.js command switch
  -> capture/extract/navigate/open
  -> 浏览器标签页执行
  -> BrowserResponse
```

现状具备的基础：

- 浏览器扩展已经有 `capture`、`extract`、`navigate`、`open` 四类命令分发。
- 扩展已有 `tabs`、`scripting`、`offscreen`、`host_permissions: <all_urls>` 等关键权限。
- Go 端已经有 WebSocket 请求响应配对和浏览器连接选择机制。
- 当前 `extract` 已经能打开页面、等待页面稳定、抽取 Markdown 并关闭临时 tab。

主要缺口：

- `BrowserRequest` 结构过窄，目前基本只能表达 `command/url/fullPage/browser`。
- `BrowserResponse.Result` 主要服务截图和 Markdown 提取，缺少通用 JSON/raw text/结构化结果承载方式。
- 现有 `navigateToUrl` 默认 `active:true`，自动化抓取时会抢用户焦点。
- MCP 层仍有模板残留工具 `add` 和 `get_server_time`，应清理。

## 认可的设计点

### 1. 插件作为浏览器执行器是正确方向

服务端爬虫很难同时解决登录态、浏览器指纹、住宅 IP、站点前端 token、反爬策略等问题。浏览器扩展运行在用户真实浏览器里，天然更适合做登录态读取、页面内 API 调用和页面行为观察。

推荐继续采用：

```text
MCP Client
  -> 服务端：调度 + 站点适配 + 解析
  -> WebSocket
  -> 浏览器扩展：导航 + 页面内读取 + 截图 + 受控执行
```

### 2. 意图级 MCP 工具优于底层原语

方案中“不把 `eval`、`fetchInPage`、`intercept` 直接暴露给 MCP client”是正确的。MCP 工具应该是：

- `youtube_transcript(url)`
- `reddit_thread(url)`
- `twitter_search(query)`
- `twitter_timeline(kind, handle?, limit?)`
- `xiaohongshu_note(url)`
- `bilibili_video(url)`
- `web_search(query)`

底层浏览器能力应只作为服务端适配器内部实现细节。

### 3. 本期只做读能力是必要边界

发帖、点赞、关注、评论、私信等写类操作不可逆，且容易触发平台风控。第一期应只做读取，不做对外状态变更。

如果后续要做写操作，必须有：

- 操作预览。
- 用户显式确认。
- 站点级权限开关。
- 操作审计日志。
- 明确的失败和取消语义。

## 必须修正的问题

### 1. 不应实现任意字符串 `eval`

方案里的示例：

```js
func: (expr) => eval(expr),
args: [data.script]
```

不建议采用。

原因：

- 这是在用户登录态页面的 MAIN world 执行任意 JS。
- 即使 MCP 不直接暴露该能力，只要服务端适配器输入被污染，也可能变成任意页面脚本执行。
- 对 Twitter/X、小红书、B站这类站点，这相当于给自动化系统极高权限。

建议替代方案：

```json
{
  "command": "runPageScript",
  "url": "https://www.youtube.com/watch?v=...",
  "script": "youtube.initialPlayerResponse",
  "params": {
    "videoId": "..."
  }
}
```

扩展侧只允许执行内置 allowlist 脚本，例如：

- `youtube.initialPlayerResponse`
- `bilibili.initialState`
- `page.extractJsonLd`
- `page.readMeta`

不要接受任意 JS 字符串。

### 2. `fetchInPage` 不能假设自动解决所有站内 API

页面 MAIN world 里的 `fetch` 确实有机会带上用户登录态，但不能假设它一定成功。

仍然可能遇到：

- CORS 限制。
- SameSite Cookie 限制。
- `credentials` 默认值问题。
- CSRF token。
- 站点自定义 header。
- GraphQL bearer token。
- Referer、Origin、client transaction id 等动态参数。

对 Reddit、YouTube timedtext 这类简单场景，`fetchInPage` 可能足够。对 Twitter/X 和小红书，更多时候需要拦截页面自己发出的请求和响应，复用其真实 header/token，而不是手写请求。

### 3. `intercept` 必须早注入

如果页面加载完成后再注入 fetch/XHR hook，首批关键请求可能已经发完：

- Twitter/X 搜索首屏 GraphQL。
- Twitter/X home timeline 初始请求。
- 小红书搜索/笔记列表首屏接口。

建议把 `intercept` 设计为：

1. 创建 tab。
2. 在页面尽可能早的阶段注入 hook。
3. 再导航或 reload 到目标 URL。
4. 等待匹配响应、滚动触发后续请求。
5. 返回匹配结果。

如 MV3 API 支持受限，可考虑：

- 动态 content script + `run_at: document_start`。
- `chrome.scripting.executeScript` 后立即 `chrome.tabs.reload`。
- 仅在高级模式下考虑 `chrome.debugger` / CDP，但这会带来浏览器调试提示和更高权限感知。

### 4. 后台 tab 对无限流页面不可靠

将 `navigateToUrl` 改成 `active:false` 是必要优化，但不能假设所有站点后台都能正常加载。

对以下场景，后台 tab 可能不可靠：

- Twitter/X timeline。
- 小红书搜索结果。
- 需要滚动触发的无限流页面。
- 有可见性检测的站点。

建议适配器支持声明：

```json
{
  "visible": true,
  "restorePreviousTab": true,
  "scrollPlan": "timeline"
}
```

即默认后台执行，但站点适配器可以要求短暂激活，并在结束后恢复用户原 tab。

### 5. Twitter/X timeline 需要单独纳入设计

原方案提到了 Twitter 搜索，但用户目标还包括“查看 Twitter timeline”。这和搜索不是同一个场景。

建议新增意图级工具：

```text
twitter_timeline(kind: "home" | "user", handle?, limit?, browser?)
```

差异：

- `home timeline` 必须依赖用户登录态。
- `user timeline` 可从用户主页进入，但仍可能需要登录才能完整访问。
- timeline 通常需要滚动触发分页。
- 返回结果需要去重、排序、截断和结构化。

## 建议补充的协议设计

当前 `BrowserRequest` 建议扩展为更通用的执行协议：

```go
type BrowserRequest struct {
    Type      string         `json:"type,omitempty"`
    Source    string         `json:"source,omitempty"`
    Action    string         `json:"action,omitempty"`
    Command   string         `json:"command"`
    URL       string         `json:"url,omitempty"`
    Browser   string         `json:"browser,omitempty"`
    MessageID string         `json:"message_id,omitempty"`

    Params    map[string]any `json:"params,omitempty"`
    TimeoutMs int            `json:"timeoutMs,omitempty"`
    Visible   bool           `json:"visible,omitempty"`
    CloseTab  bool           `json:"closeTab,omitempty"`
}
```

响应建议支持结构化结果：

```go
type BrowserResponse struct {
    Type      string `json:"type,omitempty"`
    MessageID string `json:"message_id,omitempty"`
    Command   string `json:"command,omitempty"`
    Success   bool   `json:"success,omitempty"`
    Error     string `json:"error,omitempty"`

    Result    BrowserResult `json:"result,omitempty"`
}

type BrowserResult struct {
    URL       string         `json:"url,omitempty"`
    Title     string         `json:"title,omitempty"`
    Timestamp string         `json:"timestamp,omitempty"`
    Content   PageContent    `json:"content,omitempty"`
    ImageData string         `json:"imageData,omitempty"`
    Text      string         `json:"text,omitempty"`
    JSON      map[string]any `json:"json,omitempty"`
    Items     []any          `json:"items,omitempty"`
}
```

需要同时考虑：

- 单次响应大小限制。
- 超时控制。
- tab 生命周期。
- 是否关闭临时 tab。
- 是否允许跨域 fetch。
- 域名 allowlist。
- 站点级限流。

## 建议落地顺序

### 第一步：浏览器执行协议最小升级

目标：

- 支持 `Params`、`TimeoutMs`、`Visible`、`CloseTab`。
- 响应支持 raw text / JSON / items。
- 保持现有 `extract` 和 `screenshot` 不破坏。

验证：

- 老工具 `extract`、`screenshot` 仍能工作。
- 新协议能通过一个简单内部命令返回结构化 JSON。

### 第二步：实现 `fetchInPage` 最小闭环

推荐先做 Reddit 或 YouTube timedtext。

原因：

- 风控低。
- 数据结构简单。
- 能验证“服务端适配器 -> 浏览器原语 -> 结构化解析 -> MCP 工具”的完整链路。

验证：

- `reddit_thread(url)` 能返回标题、正文、评论。
- 或 `youtube_transcript(url)` 能返回字幕文本和元信息。

### 第三步：实现安全版页面脚本读取

不要做任意 `eval`，改做 allowlist 脚本。

验证：

- `youtube.initialPlayerResponse` 能读取字幕轨信息。
- `bilibili.initialState` 能读取视频基础信息。

### 第四步：实现早注入 `intercept`

先做 Twitter/X search，不先做小红书。

验证：

- 打开搜索页后能捕获 `SearchTimeline` 相关响应。
- 能解析出 tweet 列表。
- 支持 limit、去重、超时。

### 第五步：实现 Twitter/X timeline

单独做 `twitter_timeline`，不要混在 `twitter_search` 里。

验证：

- 登录态存在时能读取 home timeline 或指定用户 timeline。
- 能滚动分页。
- 能按时间或页面顺序返回结构化 items。
- 失败时能明确提示“未登录 / 页面结构变化 / 请求未捕获 / 超时”。

### 第六步：最后做小红书

小红书应最后做，因为风控更敏感。

必须具备：

- 站点级限流。
- 随机延迟。
- 失败冷却。
- 单次结果数量限制。
- 明确账号风险提示。
- 不做写类操作。

## 风险清单

### 安全风险

- 任意 JS 执行风险。
- MCP client 或 LLM 工具调用参数污染。
- 页面内登录态被过度利用。
- 返回内容可能包含隐私数据。

建议：

- 底层原语仅内部使用。
- 页面脚本使用 allowlist。
- 限制可执行域名。
- 对敏感站点增加显式配置开关。

### 稳定性风险

- X/Twitter、小红书接口和 GraphQL 名称经常变化。
- DOM 兜底容易被改版影响。
- 后台 tab 不一定触发懒加载。
- 首屏请求可能在 hook 注入前完成。

建议：

- 适配器按站点维护。
- 明确降级链和错误分类。
- intercept 必须早注入。
- 对无限流页面支持 visible 模式。

### 账号风控风险

- 高频搜索和 timeline 滚动可能触发平台风控。
- 小红书、B站、X/Twitter 都可能对异常访问频率敏感。

建议：

- 每站单独限流。
- 随机延迟。
- 最大页数和最大 item 数限制。
- 失败后冷却。
- 默认只读。

## 建议和方案作者沟通的重点问题

1. 是否同意废弃任意字符串 `eval`，改成 allowlist 页面脚本？
2. `BrowserRequest/BrowserResponse` 是否先做协议升级，再做站点适配？
3. `intercept` 如何保证早注入，而不是页面加载完成后再 hook？
4. Twitter/X timeline 是否作为独立工具加入范围？
5. 小红书是否放到最后，并加入站点级限流和失败冷却？
6. 是否接受第一期只做读，不做任何写操作？
7. 是否需要为敏感站点提供用户显式开启开关？

## 最终建议

这份方案可以作为升级方向继续推进，但建议先修订后再进入实现。

优先修订点：

1. 把“加三个底层原语”改成“先扩展浏览器执行协议”。
2. 把任意 `eval` 改成 allowlist 脚本执行。
3. 明确 `intercept` 的早注入实现。
4. 增加 `twitter_timeline` 场景。
5. 增加站点级限流、结果大小限制、超时和 tab 生命周期设计。

修订后，推荐以 Reddit 或 YouTube 字幕作为 MVP，不建议直接从 Twitter/X 或小红书开始。
