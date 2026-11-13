# Grabby 能力差距分析与 AI 模型配置优化建议

> 分析对象：`go-server`（Go 后端 + React Web UI）、`go-server/desktop`（MyGO 桌面 APP）、
> `chrome-extension`、`go-cli` / `python-cli`、MCP 接入层。
>
> 结论先行：Grabby 的「抓取 → 归档 → AI 分析 → 日报 → MCP 输出」主链路已经成立，
> 真正拖后腿的不是平台数量，而是 **AI 基础设施的产品化程度**。
> 其中 AI 模型配置是当前投入产出比最高的一块：现状是「手写表单 + 4 个驱动 + 手输模型 ID」，
> 而成熟工具已经是「模型目录 + 连接管理 + 自动发现 + 一键切换 + 导入导出 + 密钥托管」。
> 这块不建议继续手搓，下面给出可以直接采用的开源组件与改造方案。
>
> **实施进度（本轮）**：M0 的 provider 驱动修复、Anthropic / Ollama 接入、桌面草稿连通性测试已完成；
> M1 的 models.dev 目录服务、SQLite 缓存、`/api/ai/catalog`、Web/桌面设置页接入已完成；
> M2 的 `/v1/models` 自动发现、Claude Code / Codex / OpenCode / CC Switch 导入、
> `ai_connections` / `ai_models` / `ai_routes` / `ai_usage` 数据模型与用量统计已完成（按产品决定不使用系统钥匙串）。
> 内容详情的 Markdown + HTML 混合渲染已改为 Go 侧 goldmark + bluemonday。
> 任务级路由（按 analysis / daily 分别选模型）仍待落地。

---

## 1. 现状盘点：已经具备的能力

| 模块 | 已有能力 | 评价 |
|---|---|---|
| 浏览器抓取 | 扩展执行 `extract` / `screenshot` / `fetch_in_page` / `intercept` / `ui_action`，服务端只做调度 | 设计正确，是核心壁垒 |
| 平台适配 | X（搜索 / 时间线 / 点赞）、Reddit（帖子 / 子版 / 搜索）、小红书（笔记 / 搜索 / 用户） | 覆盖了最难的部分 |
| 内容库 | RSS 订阅、cron 调度、SQLite 存储、搜索、收藏、已读、分类 | 可用，但停在「能看」 |
| AI | 单条分析（分类 / 评分 / 摘要 / 评论 / 标签）、优质筛选、日报 / 早晚报、RSS 输出、多 profile 故障转移 | 引擎能力不弱，配置层很弱 |
| 接入 | HTTP API、WebSocket、MCP（Streamable HTTP / SSE）、CLI | 对 Agent 友好 |
| 桌面 | MyGO 单进程内嵌 HTTP/WS/MCP、托盘常驻、单实例、端口顺延、loopback 免密 | 形态正确 |
| 管理界面 | React Web UI（已用 Radix/shadcn/Tailwind）；桌面另有手写 `app.js` | 两套 UI，桌面这套是主要短板 |

---

## 2. 能力差距总表

按「不做会直接影响日常使用 / 做了能立刻提升体验」排序。

### P0：现在就应该补

| # | 能力 | 现状 | 差距 |
|---|---|---|---|
| 1 | **AI 供应商与模型管理** | 后端只认 `openai` / `gemini` / `custom` / `lmstudio`；桌面只有 6 个预设；模型靠手输 | 缺 Claude / Ollama / Vertex / xAI / OpenRouter / DeepSeek / Kimi / GLM / DashScope 等；缺模型目录、自动发现、按模型测试、能力/价格/上下文元数据 |
| 2 | **密钥安全** | API Key 明文存 SQLite，`GET /api/ai/settings` 原样返回 | 应存系统钥匙串（macOS Keychain / Windows Credential Manager / Linux Secret Service），API 只返回 `has_key`，永不回显 |
| 3 | **配置导入 / 导出 / 备份** | 无 | 换机、重装、多环境迁移全靠重填；应支持 JSON 导入导出 + 自动备份，并能从 Claude Code / Codex / OpenCode / cc-switch 读取已有配置 |
| 4 | **连接健康与可观测** | 只有「全量分析」式测试；桌面测试按钮用的是已保存配置，不是当前草稿 | 缺单模型连通性测试、延迟、结构化输出校验、失败原因分类、健康状态、熔断 |
| 5 | **扩展可靠性 / 错误可诊断** | 重连有上限、错误只有 message、失败可能假成功 | `docs/specs/01-extension-reliability.md` 已写清楚，按方案落地即可 |
| 6 | **用量与成本** | 无 | 不知道每天花了多少钱、哪个模型在烧钱、429 多不多 |

### P1：产品化需要的下一层

| # | 能力 | 说明 |
|---|---|---|
| 7 | 多浏览器路由 | 按平台标签路由、同平台设备故障切换（`docs/specs/02`） |
| 8 | 缓存与在途合并 | 避免 Agent 重试打穿浏览器（`docs/specs/03`） |
| 9 | 统一数据出口 / 导出 | `/api/records`、JSONL / Markdown 批量导出（`docs/specs/04`） |
| 10 | 语义搜索 / 问内容库 | 现在只有 SQL `LIKE`；可用 `sqlite-vec` 或 Genkit `localvec`，不要手搓向量检索 |
| 11 | 日报 / 故障通知 | 邮件、Telegram、飞书、Webhook，至少一种 |
| 12 | 任务级模型路由 | 分析用便宜模型、日报用强模型、失败再升级，而不是所有任务共用一个模型 |
| 13 | MCP 权限与审计 | 工具级白名单、调用日志、危险操作二次确认 |
| 14 | 规则 / 模板 | 自定义提取规则、站点适配器插件化（README 的 TODO 里也提到） |

### P2：可以后置

- YouTube / B 站 / Reddit saved / 小红书收藏等新适配器（`docs/specs/05`）
- 自动更新、代码签名 / 公证
- 移动端 / PWA
- 完整 i18n（目前中英混排）
- 计费 / 多用户

---

## 3. 重点：为什么 AI 模型配置「难用」

### 3.1 表层：桌面 APP 的配置界面太原始

证据：`go-server/desktop/dist/app.js`

- 只有 6 个供应商预设（OpenAI / DeepSeek / Gemini / OpenRouter / LM Studio / 自定义），见 `PROVIDER_PRESETS`（约 496 行）。
- 模型是一个普通 `<input list="model-options">`（约 861 行），用户必须自己知道模型 ID；`datalist` 只是静态提示。
- 没有「从供应商拉取模型列表」的能力，只能手输。
- 没有按模型测试；「测试模型连通性」直接 `POST /api/ai/test {}`（约 1419 行），用的是**已保存**配置，而不是当前表单草稿。
- 没有导入 / 导出 / 备份；没有延迟、价格、上下文、能力标签。
- 多模型轮询只能在 profile 里堆，桌面 UI 甚至没有模型列表编辑器。
- 整个设置页是 80KB 手写 `app.js` 拼字符串，交互和可维护性都受限。

### 3.2 架构：后端驱动被 4 个 provider 绑死

证据：`go-server/internal/application/ai/engine.go` 的 `initGenkit`（约 143–216 行）

```go
switch providerName {
case "gemini":  ...
case "openai":  ...
case "custom":  ...
default:
    // 未知 provider 直接回退到 Gemini 默认
}
```

问题：

- 没有 `anthropic`、`ollama`、`vertexai` 原生分支；前端目录里的 Ollama 目前是借 `custom` + OpenAI 兼容地址绕过去的，并不是真正的 Ollama 驱动。
- 真正未知的 provider 会走到 `default`，静默回退到 Gemini 默认配置——这是一个**实际 bug**，会让配置错误表现为「模型答非所问」而不是明确报错。
- 没有统一的「模型列表」接口，`/v1/models`、Anthropic `/v1/models`、Gemini `models.list` 都没接。
- `AIProviderProfile` 把「连接」和「模型」混在一起：一个 profile 既是供应商连接，又是模型列表，还是路由单元。
- 没有模型能力元数据（是否支持结构化输出 / tool / vision / 上下文长度 / 价格），无法做能力门控与成本控制。
- 没有 per-model 健康状态、延迟、失败率、用量记录。
- 设置是扁平的 `ai_*` key + `ai_provider_profiles` JSON，扩展性差，迁移成本高。

### 3.3 安全：Key 的处理方式不及格

证据：`go-server/internal/infrastructure/sqlite/settings_repo.go`

- `ai_api_key` 与 profile 的 `api_key` 都以明文写入 SQLite（约 245 行、252 行）。
- `GET /api/ai/settings` 返回完整 `AISettings`，包含 `api_key` 和所有 profile 的 key（`ai_handlers.go` 约 428 行）。
- 桌面端还提供「显示」按钮，会把已保存的 key 填回输入框（`app.js` 约 1288–1302 行）。
- 没有使用系统钥匙串，也没有字段级加密。

对于本地工具，这不是「立刻被攻破」的问题，但一旦开了局域网访问、共享数据库、或做云同步，就是硬伤；也不符合成熟工具的基本预期。

---

## 4. 与 cc-switch / Claude Code / OpenCode 的差距

| 维度 | cc-switch / Claude Code / OpenCode | Grabby 现状 |
|---|---|---|
| 供应商预设 | 几十个，覆盖官方、聚合、中转、本地 | 桌面 6 个；Web 16 个（静态） |
| 模型目录 | 在线目录（models.dev），含价格 / 上下文 / 能力 | 手写静态快照，容易过期 |
| 模型发现 | 调 `/v1/models` 自动列出 | 手输 ID |
| 切换方式 | 主界面 / 托盘一键切换，立即生效 | 编辑 profile → 保存 |
| 连接测试 | 单模型、延迟、错误诊断 | 全量分析；桌面测试已保存配置 |
| 导入 / 导出 | 配置备份、导入、跨工具读取 | 无 |
| 认证方式 | API Key / OAuth / 环境变量 | 只有 API Key |
| 协议 | OpenAI / Anthropic / Gemini / 本地 | OpenAI / Gemini / OpenAI 兼容 / LM Studio |
| 路由 / 代理 | 本地代理、模型映射、额度 | 只有 profile 级 failover |
| 用量 / 成本 | 请求日志、额度、模型映射 | 无 |
| 密钥安全 | 系统钥匙串 / 本地加密 | SQLite 明文 |

**关键洞察**：cc-switch 好用的本质不是 UI 花哨，而是三件事：

1. **预设足够全**（用户只需要填 Key）；
2. **配置是「连接」而不是「一堆字段」**（可切换、可备份、可导入导出）；
3. **切换成本极低**（托盘 / 一键，立即生效）。

Grabby 应该抄的是这套产品模型，而不是继续在设置页里堆输入框。

---

## 5. 推荐采用的开源组件（不要手搓）

### 5.1 模型 / 供应商目录：models.dev

- 仓库：<https://github.com/anomalyco/models.dev>（MIT）
- 数据：`https://models.dev/api.json`（provider 维度，含模型、价格、上下文、能力、npm 包、API 地址）；另有 `models.json`、`catalog.json`。
- 这是 OpenCode 等工具背后的同一份目录，覆盖 75+ provider、数千模型。
- 建议做法：
  1. 后端定时（如每天）拉取 `api.json`，落 SQLite 缓存表 `ai_catalog_providers` / `ai_catalog_models`；
  2. 提供 `GET /api/ai/catalog`（支持 `?refresh=1`），前端只读后端，不直连外网；
  3. 离线时用上次缓存；首次启动可内置一份精简快照；
  4. 用 `provider.id` + `model.id` 作为稳定键，展示名 / 价格 / 上下文 / 能力全部来自目录。
- 备选/补充：LiteLLM 的 `model_prices_and_context_window.json`（价格与上下文兜底）。

这样可以直接替换：
- `go-server/frontend/src/lib/model-catalog.ts` 的 447 行静态表；
- `go-server/desktop/dist/app.js` 的 `PROVIDER_PRESETS`。

### 5.2 Provider 驱动：Genkit 官方插件

Genkit Go v1.8 已经内置了需要的插件，不需要为每个厂商写 HTTP client：

| 厂商 | 插件 | 备注 |
|---|---|---|
| Anthropic / Claude | `plugins/anthropic` | 支持 `BaseURL`，可接中转 |
| OpenAI | `plugins/compat_oai/openai` | 已有 |
| Google Gemini | `plugins/googlegenai` | 已有 |
| Vertex AI | `plugins/googlegenai`（VertexAI） | 新增 |
| Ollama | `plugins/ollama` | 本地模型 |
| xAI / DeepSeek / OpenRouter / Kimi / DashScope / Z.ai | `plugins/compat_oai` 及子包 | OpenAI 兼容，统一接入 |

建议：

- 把 `initGenkit` 的 `switch` 改成 `driver` 映射；
- `custom` 继续走 `compat_oai`；
- `lmstudio` 的直连 client 可以保留作为兜底，但通用 OpenAI 兼容端点不要再手写；
- **修掉 unknown provider 静默回退 Gemini 的 bug**，改成返回明确错误。

### 5.3 配置管理模式：借鉴 cc-switch（MIT）

cc-switch 仓库：<https://github.com/farion1231/cc-switch>（MIT）

不建议直接拷贝它的代码（它是 Tauri 桌面应用，和 Go/React 架构不同），但可以复用它的产品模式与配置格式：

1. **Provider Preset + Connection 分离**：预设只负责填充 `base_url` / `protocol` / 默认模型，用户填 Key 后生成一个 connection。
2. **一键切换**：托盘 / 主界面切换 active connection，立即对新任务生效。
3. **导入 / 导出 / 备份**：connection 列表导出为 JSON；切换前自动备份。
4. **从现有工具导入**：
   - Claude Code：`~/.claude/settings.json` 的 `ANTHROPIC_BASE_URL` / `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN`；
   - Codex：`~/.codex/config.toml`；
   - OpenCode：`~/.config/opencode/opencode.json`；
   - cc-switch：读取其配置里的 provider / key / model（注意只读、不要改写对方文件）。
5. **模型映射**：同一个 connection 下可以有多个模型别名，任务路由按别名选模型。

### 5.4 UI 组件：不要再手写桌面表单

Web 端已经引入了 `radix-ui` + `shadcn` + `tailwind`，但桌面端仍是手写 `app.js`。

建议二选一：

- **首选**：桌面端直接复用 `go-server/frontend` 的 React 构建产物，用 `?desktop=1` 或独立入口切换主题/布局，删除 `desktop/dist/app.js` 的重复实现；
- 次选：如果短期必须保留轻量 UI，至少引入 Preact + Radix（或 Web Components），用现成的 Combobox / Dialog / Toast / Tabs / Switch，不要继续拼字符串。

具体可直接用的组件：
- 供应商搜索 / 选择：`Command` + `Combobox`（Radix/shadcn）
- 模型多选：带搜索、分组、badge 的 `Command` 列表
- 测试结果：`Toast` + `Progress` + `Tooltip`
- 密钥输入：`PasswordInput` + 「已保存」状态，不回显
- 连接切换：`DropdownMenu` + 托盘菜单

### 5.5 密钥托管：系统钥匙串

- Go 库：`github.com/zalando/go-keyring` 或 `github.com/99designs/keyring`
- macOS → Keychain，Windows → Credential Manager，Linux → Secret Service
- SQLite 只存 `key_ref`（如 `grabby:ai:<connection-id>`），不存明文
- API 返回 `has_key: true/false`，永不返回 key；保存时「留空表示不修改」

### 5.6 语义搜索 / 问内容库（按需）

- `sqlite-vec`（SQLite 扩展）或 Genkit `localvec` / `pgvector`
- 不要自己实现 embedding 存储和相似度检索
- 模型侧可直接用 Genkit 的 embedding 接口，和现有 provider 配置复用

---

## 6. 目标数据模型与 API

### 6.1 数据模型

把现在的 `AIProviderProfile` 拆成四层：

```
AIConnection（连接）
  id, name, driver(openai|anthropic|gemini|vertex|ollama|custom),
  base_url, auth_ref, headers, proxy, enabled, created_at

AIModel（模型）
  id, connection_id, model_id, display_name,
  capabilities, context_window, input_price, output_price, source,
  enabled, last_checked_at, latency_ms, status

AIRoute（任务路由）
  task(analyze|daily|embedding), strategy(single|round-robin|failover),
  entries[{connection_id, model_id, priority}], budget

AIUsage（用量）
  id, connection_id, model_id, task, prompt_tokens, completion_tokens,
  cost, latency_ms, status, created_at
```

兼容策略：

- 启动时把旧 `ai_provider_profiles` 自动迁移成 `AIConnection` + `AIModel`；
- 旧字段 `provider/model/base_url/api_key` 继续保留一段时间，只读回填；
- 迁移失败要能回滚，且日志里明确提示。

### 6.2 新增 API

| 方法 | 路径 | 作用 |
|---|---|---|
| GET | `/api/ai/catalog` | 供应商 / 模型目录（可 `refresh=1`） |
| GET | `/api/ai/connections` | 连接列表（不含 key） |
| POST | `/api/ai/connections` | 新建连接 |
| PUT | `/api/ai/connections/:id` | 更新连接 |
| DELETE | `/api/ai/connections/:id` | 删除连接 |
| POST | `/api/ai/connections/:id/models` | 调供应商 `/v1/models` 拉取模型 |
| POST | `/api/ai/connections/test` | 测试草稿连接（单模型、延迟、JSON 校验） |
| POST | `/api/ai/connections/import` | 从 cc-switch / Claude Code / Codex / OpenCode 导入 |
| GET | `/api/ai/connections/export` | 导出连接（key 可选脱敏） |
| POST | `/api/ai/connections/:id/activate` | 切换当前连接 |
| GET | `/api/ai/usage` | 用量 / 成本统计 |

### 6.3 交互流程（3 步完成配置）

1. **选供应商**：搜索 / 分类（云端 / 聚合 / 本地）/ 推荐；数据来自 models.dev，显示「需要 Key / 本地免 Key」。
2. **填 Key + 拉模型**：粘贴 Key 后点「获取模型」，后端调 `/v1/models`（Anthropic / Gemini / OpenAI 兼容分别适配）；列表支持搜索、多选、显示价格 / 上下文 / 能力。
3. **一键测试 + 保存**：并发测试选中模型（延迟、是否返回合法 JSON、错误原因），绿灯后保存为连接；可设为默认或备用。

附加能力：

- 顶部「当前连接」下拉 + 托盘快速切换；
- 「导入已有配置」按钮（自动扫描 Claude Code / Codex / OpenCode / cc-switch）；
- 「导出 / 备份」按钮；
- 模型卡片上显示最近一次测试延迟、成功率、今日调用次数 / 成本。

---

## 7. 实施路线（建议 2–3 周）

### M0：止血（0.5 天）

- 修 unknown provider 静默回退 Gemini 的 bug；
- 把 `anthropic`、`ollama`、`vertexai` 加进 driver 映射；
- 桌面「测试连通性」改成测试当前表单草稿（`profile` 参数已经支持）。

### M1：目录服务（3 天）

- 新增 `internal/application/catalog`：拉取 models.dev、缓存 SQLite、定时刷新；
- 提供 `/api/ai/catalog`；
- 前端 model-catalog 改为读接口，保留内置快照作为离线兜底。

### M2：连接 / 模型 / 密钥（3–4 天）

- 新增 `AIConnection` / `AIModel` / `AIUsage` 表与仓储；
- 接 `go-keyring`；
- 实现 `/v1/models` 发现（OpenAI 兼容 / Anthropic / Gemini）；
- 实现草稿测试 + 延迟 + JSON 校验；
- 旧 profile 自动迁移。

### M3：Web UI 重构（3 天）

- 供应商搜索 / 选择；
- 多模型选择列表（价格、上下文、能力）；
- 连接测试面板；
- 导入 / 导出 / 切换；
- 密钥只显示「已保存」。

### M4：桌面端与托盘（2 天）

- 复用 React 构建产物（首选），或至少把设置页换成成熟组件；
- 托盘菜单加「当前连接」子菜单；
- 删除桌面端重复的 provider/model 逻辑。

### M5：路由与用量（2 天）

- 任务级路由（analyze / daily / embedding）；
- 用量与成本记录；
- 预算 / 429 告警。

### 验收标准

- 新用户 3 步内完成一个可用模型配置，不需要知道 `custom/` 前缀；
- 支持至少 Claude / OpenAI / Gemini / DeepSeek / OpenRouter / Ollama；
- 能自动拉取模型列表，且失败时有明确错误；
- Key 不再出现在 API 响应和 SQLite 明文中；
- 配置可导出、导入、备份；
- 桌面与 Web 使用同一套配置模型。

---

## 8. 不建议继续做的事

1. **不要继续扩展 `desktop/dist/app.js` 的手写表单**——它是当前体验最差、维护成本最高的一块。
2. **不要自己维护 provider / model 大表**——用 models.dev + LiteLLM 兜底。
3. **不要为每个厂商写 HTTP client**——用 Genkit 官方插件。
4. **不要把 API Key 放 SQLite / JSON 明文**——用系统钥匙串。
5. **不要用「全量分析」当连通性测试**——单模型、轻量、可并发、可诊断。
6. **不要把「供应商连接」和「模型」继续混在一个 profile 里**——这会限制路由、用量、能力门控。

---

## 9. 证据索引

| 主题 | 文件 |
|---|---|
| 桌面 AI 设置 / 预设 / 保存逻辑 | `go-server/desktop/dist/app.js`（约 496、595–935、1135–1204、1419 行） |
| Web AI 设置 | `go-server/frontend/src/features/ai-settings/AISettingsView.tsx` |
| Web 静态模型目录 | `go-server/frontend/src/lib/model-catalog.ts` |
| Provider 驱动 / 回退逻辑 | `go-server/internal/application/ai/engine.go`（约 143–216 行） |
| Profile 数据结构 | `go-server/internal/domain/ai/types.go` |
| 设置持久化 | `go-server/internal/infrastructure/sqlite/settings_repo.go` |
| AI 设置 API | `go-server/internal/interfaces/http/ai_handlers.go` |
| 内容渲染（Markdown + HTML） | `go-server/internal/application/render` |
| 扩展可靠性方案 | `docs/specs/01-extension-reliability.md` |
| 多浏览器 / 缓存 / 统一出口 | `docs/specs/02`、`03`、`04` |
| 产品路线 | `docs/roadmap.md` |
