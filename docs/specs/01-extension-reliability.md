# Spec 01：扩展可靠性——把扩展变成无人值守节点

> 领域术语见根目录 `CONTEXT.md`。本方案涉及 `chrome-extension/` 与 go-server 两端。
> 实现前先通读：`chrome-extension/background.js`、`chrome-extension/lib/websocket.js`、`chrome-extension/manifest.json`。

## 目标

Grabby 的目标形态是"服务部署一次，多台浏览器长期在线接受调度"。当前扩展按"用户手动使用"设计，掉线后不会自愈、结果可能丢失、失败原因不可区分。本方案修复这些基础设施问题。

## 非目标

- 不改动任何抓取原语（intercept / fetchInPage / runPageScript）的业务逻辑
- 不做登录态自动检测（见 Spec 02）
- 不在扩展端做内容缓存（缓存全部在服务端，见 Spec 03）

---

## 改动 1：连接自愈

**现状问题**（`lib/websocket.js`）：
- `maxReconnectAttempts = 5`（第 9 行），指数退避后彻底放弃，需要用户手动点 popup 才能恢复
- `handleClose` 中被 `isHandshakeError()` 判定为握手错误的关闭（含 connecting 状态下的 code 1006——服务器重启、网络瞬断都会产生）直接设 `reconnectAttempts = maxReconnectAttempts` 并停止重连（约第 379–394 行）
- manifest 无 `alarms` 权限，MV3 service worker 被 Chrome 杀掉后（WS 已断的情况下）没有任何机制唤醒它

**要求**：

1. **重连策略改为"除认证失败外无限重连"**：
   - 删除 `maxReconnectAttempts` 的放弃逻辑。退避保留指数增长，封顶 60 秒。
   - 唯一停止重连的情形：服务端明确返回认证失败（`auth_response.success === false`，或握手被 401/403 拒绝且 reason 含 `unauthorized`/`forbidden`）。此时状态置为 `error`，展示"token 错误"提示，直到用户修改配置（`updateConfig` 触发）才恢复重连。
   - 服务器重启、超时、1006 等一律视为临时故障，继续退避重连。
2. **alarms 兜底**：
   - `manifest.json` 的 `permissions` 加 `"alarms"`。
   - `background.js` 的 `init()` 中创建 `chrome.alarms.create('grabby-keepalive', { periodInMinutes: 1 })`，并注册 `chrome.alarms.onAlarm` 监听：alarm 触发时若 `autoConnect` 开启、配置有效、且状态不是 `connected`/`connecting`，调用 `connectToServer()`。
   - 注册 `chrome.runtime.onStartup` 与 `chrome.runtime.onInstalled` 监听，两者都调用 `init()` 里的自动连接逻辑（注意 init 已在文件末尾顶层执行一次，避免重复注册监听器——把监听器注册放在顶层，init 只做连接）。

## 改动 2：WebSocket 握手认证（两端）

**现状问题**：token（`apiToken`）只在 HTTP register（`POST /api/browsers/register`，header `X-Grabby-Token`）时校验；WS 连接 URL 只带 `conn_id` 和 `name`（`lib/websocket.js` connect() 中 `urlWithConnId`）。服务暴露在局域网时，任何知道 conn_id 的进程都可冒充浏览器节点接收命令（命令含 fetchInPage，等于可借用登录态发任意请求）。

**要求**：

1. **扩展端**：`connect()` 构造 WS URL 时追加 `token` 查询参数（值为 `this.apiToken`，为空则不加）。
2. **服务端（go-server）**：在浏览器 WS 端点（`/ws_browser`，具体 handler 用 `grep -r "ws_browser" go-server/` 定位）的 upgrade 之前校验：若配置了 `GRABBY_API_TOKEN`，请求必须带匹配的 `token` 查询参数或 `X-Grabby-Token` header，否则返回 HTTP 401（不进行 upgrade）。未配置 token 时保持现状（放行）。
3. 401 拒绝属于"认证失败"，扩展端按改动 1 的规则停止重连并提示。

## 改动 3：统一错误码，消灭假成功

**现状问题**（`background.js`）：
- 所有失败只有 `error.message` 字符串，服务端无法区分故障类型
- `navigateToUrl` 的 30 秒超时兜底是 `resolve(tab)` 而非报错（约第 912–915 行）；`waitForPageStable` 超时也静默继续——页面没加载好却返回 `success: true` + 空内容

**要求**：

1. 定义错误码常量（扩展端新建 `lib/errors.js`，服务端同步一份 Go 常量）：

   | error_code | 含义 | HTTP 映射（服务端） |
   |------|------|------|
   | `NAV_TIMEOUT` | 页面 30s 未触发 load complete | 504 |
   | `PAGE_UNSTABLE` | 页面加载完成但内容未稳定（警告，不算失败） | — |
   | `EMPTY_RESULT` | 抓取流程成功但内容为空（extract markdown 为空 / intercept 0 条） | 502 |
   | `NOT_LOGGED_IN` | 目标平台未登录（本 spec 只定义码，检测在 Spec 02） | 502 |
   | `SCRIPT_NOT_ALLOWED` | runPageScript 脚本名不在白名单 | 400 |
   | `FETCH_FAILED` | fetchInPage 的 fetch 抛错 | 502 |
   | `INTERNAL` | 其他未分类错误 | 502 |

2. 所有 response 消息格式统一为：
   ```json
   { "type": "response", "message_id": "...", "command": "...",
     "success": false, "error_code": "NAV_TIMEOUT", "error": "人类可读描述" }
   ```
   成功响应可携带非致命警告：`"warnings": ["PAGE_UNSTABLE"]`。
3. 行为修正：
   - `navigateToUrl` 30s 超时改为 reject（`NAV_TIMEOUT`），并关闭已创建的 tab
   - `waitForPageStable` 超时不再静默：结果加 `warnings: ["PAGE_UNSTABLE"]`
   - extract 结果 markdown 为空、intercept 捕获 0 条时返回 `EMPTY_RESULT` 失败（服务端不写缓存、不落库）
4. **服务端**：浏览器 response 里的 `error_code` 按上表映射 HTTP 状态码，错误响应体统一为 `{"detail": "...", "error_code": "..."}`（保持现有 `detail` 字段兼容）。

## 改动 4：结果补发缓冲

**现状问题**：任务执行中 WS 断开时 `sendMessage` 只打日志、结果丢弃（`lib/websocket.js` sendMessage），服务端只能等超时。

**要求**：

1. `WebSocketManager` 加内存缓冲 `pendingResponses`（数组，上限 20 条，FIFO 淘汰）。`sendMessage` 失败（未连接或 send 抛错）且消息 `type === 'response'` 时入缓冲。
2. `handleOpen`（连接建立）后遍历缓冲逐条重发并清空。
3. 服务端：`message_id` 对应的等待 channel 已关闭/不存在时，直接忽略该 response（不报错）。确认现有实现已如此（`sync.Map` 查不到即丢弃），若会 panic 或报错则修复。
4. 已知局限（接受，不需处理）：缓冲在内存里，SW 被杀则丢失。

## 改动 5：任务队列与取消

**要求**：

1. `background.js` 加一个简单的串行任务队列：所有会开 tab 的命令（capture/extract/intercept/runPageScript/fetchInPage）入队，并发数常量 `MAX_CONCURRENT_TASKS = 1`。队列只在内存。
2. 新增 `cancel` 命令：`{ "command": "cancel", "target_message_id": "..." }`。
   - 还在排队：直接移除，回 `{ success: true, cancelled: true }`
   - 正在执行：置入 `cancelledIds` 集合；intercept 的滚动循环每轮开始检查该集合，命中则提前结束并关 tab，response 带 `"cancelled": true`；其他命令不强制中断（执行完丢弃结果即可）
3. 服务端：请求超时放弃等待时，向该浏览器发送 cancel 命令（best-effort，不等回执）。

## 改动 6：register 上报版本

`registerBrowser()`（`lib/websocket.js`）的 body 增加 `"version": chrome.runtime.getManifest().version`。服务端 register handler 接受并存入浏览器注册信息，`/api/browsers` 返回该字段。

---

## 验收标准

1. 起服务、连上扩展、杀掉服务、等 3 分钟、重启服务 → 扩展在 1 分钟内自动重连，无人工操作
2. 重启浏览器 → 扩展自动重连
3. 服务端配置 `GRABBY_API_TOKEN`，扩展 token 错误 → WS 被 401 拒绝，扩展显示认证错误且不再重连；改对 token 后自动恢复
4. extract 一个不存在的域名 → 返回 `error_code: NAV_TIMEOUT`，HTTP 504，无残留 tab
5. 抓取执行中途 kill 服务进程再重启 → 结果通过补发缓冲送达（或服务端已超时则被静默丢弃，无报错日志风暴）
6. 同时发 3 个 extract 请求 → 扩展串行执行，浏览器同一时刻最多 1 个任务 tab
7. `/api/browsers` 能看到扩展版本号
