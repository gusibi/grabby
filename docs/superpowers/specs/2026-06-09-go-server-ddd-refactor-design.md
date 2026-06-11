# go-server 渐进式 DDD 架构改造设计

日期：2026-06-09

## 背景

当前 [go-server](../../../go-server/) 的 Go 代码集中在单层目录下，主要问题是职责边界不清：

- [go-server/main.go](../../../go-server/main.go) 同时负责启动、依赖组装、HTTP API、WebSocket、MCP tools 和静态资源服务。
- [go-server/db.go](../../../go-server/db.go) 同时包含 SQLite 初始化、迁移、seed、Source/Item/AI/Report/Settings 的所有数据访问。
- [go-server/types.go](../../../go-server/types.go) 混合了领域实体、HTTP DTO、MCP 参数、WebSocket 消息和 AI 配置。
- AI、采集、调度、浏览器连接等业务对象已经存在，但都在 `package main` 平铺，并且部分代码直接穿透到 `Database.db` 执行 SQL。

本次改造采用渐进式 DDD 分层：先建立清晰目录和包边界，保持现有行为不变，再为后续更深的领域建模和接口抽象留出空间。

## 目标

1. 将 `go-server` 从平铺目录改造成清晰的 DDD 分层结构。
2. 拆分 `main.go`、`db.go`、`types.go` 的职责，降低单文件复杂度。
3. 保持现有 HTTP API、MCP tools、WebSocket 路径、数据库 schema、前端行为不变。
4. 迁移现有测试，并补充必要的包级测试，确保重构不改变业务语义。
5. 为后续按领域继续演进到 repository interface、use case service 和更纯粹 domain model 打基础。

## 非目标

- 不修改 SQLite schema 或迁移版本。
- 不改变前端调用的 URL、JSON 字段、RSS 输出格式。
- 不引入大型依赖注入框架。
- 不重写业务逻辑或 AI prompt 逻辑。
- 不改造 frontend、chrome-extension、go-cli、python-server 或 site。
- 不一次性把所有依赖抽象成接口；第一阶段以安全迁移和边界清晰为主。

## 目标目录结构

```text
go-server/
  cmd/grabby-server/
    main.go

  internal/
    bootstrap/
      app.go

    config/
      settings.go

    logging/
      logger.go

    domain/
      browser/
      capture/
      source/
      item/
      ai/

    application/
      browser/
      capture/
      source/
      item/
      scraping/
      ai/
      scheduler/

    infrastructure/
      sqlite/
      browserregistry/
      browserws/
      llm/

    interfaces/
      http/
      websocket/
      mcp/
```

## 分层职责

### `cmd/grabby-server`

只保留进程入口：读取配置、初始化 logger、调用 bootstrap 启动应用。这里不直接注册路由、不写业务逻辑、不创建具体 handler 细节。

### `internal/bootstrap`

负责依赖组装和生命周期管理：

- 创建 SQLite 数据库连接。
- 从数据库加载 AI settings。
- 初始化 WebSocket manager、browser registry、AI engine、daily manager、task queue、scraper、scheduler。
- 挂载 HTTP、WebSocket、MCP、静态资源路由。
- 处理 graceful shutdown。

### `internal/domain/*`

放核心数据结构和简单领域方法：

- `domain/capture`：`BrowserRequest`、`BrowserResponse`、`PageResult`、`PageContent`。
- `domain/browser`：`BrowserRegistration`、`BrowserInfo` 等。
- `domain/source`：`Source`、`SourceForm`、`FetchLog`。
- `domain/item`：`ScrapedItem`、`ScrapedItemWithAI`、`ItemsFilter`。
- `domain/ai`：`AISettings`、`AIProviderProfile`、`AIAnalysis`、`AIDailyReport`、`AICategoryStat`。

Domain 不依赖 HTTP、SQLite、WebSocket、MCP、zap logger 或环境变量。

### `internal/application/*`

放业务流程和用例协调：

- `application/ai`：`AIEngine`、`AIDailyManager`、`ProfileSelector`。
- `application/scraping`：`Scraper`、`TaskQueue`、`Classifier`。
- `application/scheduler`：`Scheduler`。
- `application/capture`：HTTP/MCP extract 和 screenshot 共享的捕获用例。
- `application/source` 与 `application/item`：Source 和 Item 的应用服务。第一阶段可以先由 handler 调用 sqlite repository，后续再收敛到 service。
- `application/browser`：浏览器注册、连接解析等协调逻辑。

第一阶段允许 application 依赖具体基础设施类型，但迁移时应避免继续扩大直接 SQL 和 handler 内业务逻辑。

### `internal/infrastructure/*`

放技术实现：

- `sqlite`：数据库初始化、迁移、seed、Source/Item/AI/Report/Settings 数据访问。
- `browserregistry`：`browser_registry.json` 的读写和注册冲突规则。
- `browserws`：`WebSocketManager`、`WSConn`、pending response 管理。
- `llm`：`LMStudioClient`、`RateLimiter`、Genkit profile client 等外部 AI 调用细节。

SQLite 包第一阶段可以暴露一个 `Database` 聚合对象和拆分后的 repository 方法，先消除 `db.go` 单文件，再逐步避免外部访问底层 `sql.DB`。

### `internal/interfaces/*`

放入站适配器：

- `interfaces/http`：注册 REST API route，解析 HTTP 参数，返回 JSON/RSS/HTML。
- `interfaces/websocket`：`/ws_browser`、`/ws_command` 的连接入口。
- `interfaces/mcp`：MCP server 和 tools 注册。

Interfaces 层不直接写 SQL。当前 [go-server/main.go](../../../go-server/main.go) 中直接统计 SQL 的地方迁移时应优先放入 `infrastructure/sqlite` 方法，由 handler 调用。

## 核心数据流

### 启动流程

1. `cmd/grabby-server/main.go` 调用 `config.GetSettings()` 和 `logging.GetLogger()`。
2. `bootstrap.NewApp(settings, logger)` 创建基础设施和应用服务。
3. `bootstrap.App.Run(ctx)` 启动 AI engine、task queue、scheduler 和 HTTP server。
4. shutdown 时依次停止 scheduler、task queue、AI engine、WebSocket 连接并关闭数据库。

### HTTP API 流程

1. `interfaces/http.Router` 注册原有 `/api/*` 路径。
2. Handler 解析 method、query、body、path id。
3. Handler 调用 application service 或 sqlite repository 方法。
4. Handler 以原有 JSON 字段和状态码返回。

### Capture / Browser 流程

1. HTTP `/api/extract`、`/api/screenshot` 和 MCP `extract`、`screenshot` 共用 `application/capture`。
2. capture service 负责 resolve browser、构造 `BrowserRequest`、调用 `browserws.WebSocketManager.SendMessage()`。
3. HTTP/MCP adapter 只负责协议差异：HTTP 返回 JSON，MCP 返回 tool result。

### Scraping / Scheduler 流程

1. scheduler 读取 enabled sources 并按 cron 调度。
2. scraper 按 source type 执行 RSS/API/Web 抓取。
3. RSS/API 直接写入 item repository；Web source 将 page URL 入队给 task queue。
4. task queue 通过 browserws 抽取页面，写入 item repository，并按配置 enqueue AI analysis。

### AI 流程

1. AI engine 从 item repository 读取未分析 item。
2. AI engine 通过 llm client 调用 provider profile。
3. 结果解析后写入 AI analysis repository。
4. Daily manager 查询高质量 items 并写入 daily report repository。

## 迁移顺序

1. 建立 `cmd/grabby-server` 和 `internal` 目录骨架。
2. 迁移配置和日志：`config.go` 到 `internal/config`，`logger.go` 到 `internal/logging`。
3. 拆分 `types.go`：将实体和 DTO 移到相应 `domain/*` 或 `interfaces/*` 包。
4. 拆分基础设施：
   - `browser_registry.go` 到 `infrastructure/browserregistry`。
   - `websocket_manager.go` 到 `infrastructure/browserws`。
   - `lmstudio.go` 到 `infrastructure/llm`。
5. 拆分 SQLite：保留 `Database` 名称但移动到 `infrastructure/sqlite`，将大文件拆成 migration、seed、source、item、ai、settings、stats 等文件。
6. 迁移 application：`ai_*`、`scrapers.go`、`task_queue.go`、`scheduler.go`、`classifier.go` 移入对应 application 包并修正 imports。
7. 迁移 interfaces：
   - 从 `main.go` 提取 HTTP routes 和 handlers。
   - 提取 WebSocket handlers。
   - 提取 MCP server 和 tools。
8. 创建 `bootstrap.App` 统一组装依赖。
9. 将原根目录 `main.go` 替换为轻量入口，或只保留 `cmd/grabby-server/main.go` 并更新构建命令。
10. 迁移测试到对应包，运行 `go test ./...`。

## 错误处理策略

- 保持现有 HTTP 状态码和主要错误消息不变，避免前端回归。
- Handler 层继续负责协议错误：method 不允许、body 无效、path id 无效。
- Application 层返回业务错误或基础设施错误，不直接写 HTTP response。
- Infrastructure 层包装必要上下文，例如 migration failed、query failed，但不泄露协议细节。
- Browser registry 继续保留冲突错误语义，迁移为包内导出的 `ErrBrowserRegistryConflict`。

## 测试策略

- 迁移现有 Go tests，保持原测试意图不变：database CRUD、AI operations、AI engine disabled mode、classifier、browser registry、websocket manager。
- 每完成一个迁移阶段运行 `go test ./...`，优先修复 import 和 package-level API 断裂。
- 对 `interfaces/http` 可在后续补充少量 handler smoke tests，但本次第一阶段不强制新增全量 HTTP golden tests。
- 最终验证至少包括：
  - `go test ./...`
  - `go test ./internal/...`（如果 module 下存在）
  - 构建 server binary

## 风险与缓解

- **Import 循环风险**：domain 不依赖其他层，interfaces 依赖 application/infrastructure，bootstrap 依赖所有层但不被其他层依赖。
- **行为回归风险**：迁移时不改 URL、JSON 字段、SQL schema；优先用测试约束行为。
- **改动过大风险**：按包逐步迁移，每一步保持可编译；避免同时改业务逻辑。
- **直接 `db.db` 穿透残留**：迁移阶段将 main/handler/task queue/scraper 中直接 SQL 提取为 sqlite 方法，减少跨层泄漏。

## 完成标准

- `go-server` 根目录不再平铺主要业务文件；入口、domain、application、infrastructure、interfaces 分层清晰。
- [go-server/main.go](../../../go-server/main.go) 中的大部分路由和业务逻辑已迁出；最终入口只负责启动或被 `cmd/grabby-server` 替代。
- [go-server/db.go](../../../go-server/db.go) 被拆到 `internal/infrastructure/sqlite` 的多个文件。
- [go-server/types.go](../../../go-server/types.go) 中的类型被拆到对应 domain/interfaces 包。
- 现有测试迁移后通过。
- Server 构建通过。
