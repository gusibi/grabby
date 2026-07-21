# Spec 02：多浏览器调度——平台标签与登录态路由

> 依赖：Spec 01（register 上报机制、错误码）。涉及 `chrome-extension/options/`、`lib/websocket.js`、go-server 的浏览器注册与各平台适配器入口。
> 实现前先定位：`grep -r "browsers/register" go-server/` 找到注册 handler 与浏览器注册表结构；`grep -r "DEFAULT_BROWSER\|DefaultBrowser" go-server/` 找到当前选设备逻辑。

## 目标

服务端已支持多浏览器连接，但只是"能选"（显式 `browser` 参数或默认第一个），不是"会选"。本方案让服务端知道每台浏览器登了哪些平台，平台 API 自动路由到正确的设备，并在同平台多设备时故障切换。

## 非目标

- 不做登录态自动检测的强制校验（提供可选的轻量探测，见改动 4，可后置）
- 不做负载均衡/并发扩容（低频纪律，多设备是为了多身份/多地点/高可用，不是吞吐量）

---

## 改动 1：扩展 options 配置平台标签

1. `chrome-extension/options/` 页面新增"已登录平台"区块：复选框列出 `twitter` / `reddit` / `xiaohongshu` / `youtube` / `bilibili`，另留一个自由文本输入（逗号分隔）支持未来平台。
2. 存储：`chrome.storage.sync` 键 `platforms`（string 数组，值为平台 slug，小写）。
3. `lib/websocket.js` 的 `registerBrowser()` body 增加 `"platforms": [...]`；`chrome.storage.onChanged` 已监听配置变化触发重连，把 `platforms` 加入监听列表（重连即重新 register，标签随之更新）。

## 改动 2：服务端注册表与设备选择

1. 浏览器注册信息结构增加 `Platforms []string` 与 `Version string`（Spec 01）。
2. 新增统一的设备选择函数（放在现有连接管理模块旁），**所有平台适配器的 HTTP handler 和 MCP 工具都必须改为经由它选设备**：

   ```
   SelectBrowser(explicitName string, platform string) (conn, error)
   ```

   选择顺序：
   1. `explicitName` 非空 → 找该名字的已连接设备；没有 → 503 `{"detail":"browser 'X' not connected","error_code":"NO_BROWSER"}`
   2. 有设备标注了 `platform` 且已连接 → 取第一个（注册顺序稳定即可）
   3. 有设备标注了 `platform` 但都未连接 → 503 `{"detail":"no connected browser labeled for platform 'xiaohongshu'","error_code":"NO_BROWSER_FOR_PLATFORM"}`
   4. 没有任何设备标注该平台 → 向后兼容回退：按现有逻辑（DEFAULT_BROWSER 或第一个已连接设备）；全都没有 → 现有 503
3. 通用能力（extract/screenshot/fetch_in_page/run_page_script）不带 platform，走 `SelectBrowser(explicitName, "")`，即只有第 1 和第 4 步。
4. **故障切换**：第 2 步选中的设备执行失败且错误属于连接类（发送失败/等待超时且设备已断开）时，若同平台还有其他已连接设备，自动换设备重试一次（最多一次，避免放大请求量）。解析类错误（`EMPTY_RESULT` 等）不重试。

## 改动 3：能力可见

1. `/api/browsers` 每个条目返回：`name`、`connect_id`、`connected`（bool）、`platforms`、`version`、`last_seen`（RFC3339）。
2. `/health` 增加 `platforms` 字段：所有已连接设备的平台标签并集，例如 `{"browser_connected": true, "platforms": ["twitter","xiaohongshu"]}`。
3. skill 的 `scripts/browser.sh browsers` 输出随之更新（该脚本只是 curl 包装，确认字段透传即可）；`docs/api-reference.md` 同步补文档。

## 改动 4（可选，可后置）：登录态探测

服务端新增 `POST /api/browsers/probe`，body `{"browser": "...", "platform": "twitter"}`：

- 复用 `runPageScript` 原语，为每个平台在白名单里加一个极轻量探测脚本（打开平台主页，读一个"已登录才存在"的页面变量或 DOM 特征），返回 `{"logged_in": true|false}`
- 平台适配器抓取返回疑似登录墙内容时（各适配器自行判断，如解析出 0 条且页面含登录特征），错误码用 `NOT_LOGGED_IN`（Spec 01 已定义），提示用户"设备 X 标注了 twitter 但未登录"

## 验收标准

1. 两台浏览器分别标注 `twitter` 和 `xiaohongshu`，请求 `/api/xiaohongshu/search` 不带 `browser` 参数 → 自动路由到标注小红书的设备（通过设备侧日志或服务端日志确认）
2. 断开标注小红书的设备，再请求 → 503，detail 明确说"no connected browser labeled for platform 'xiaohongshu'"
3. 两台设备都标注 `twitter`，断开第一台 → 请求自动落到第二台
4. 没有任何设备标注平台时，行为与改造前一致（回退默认设备）
5. `/api/browsers` 能看到每台设备的 platforms、version、connected、last_seen
