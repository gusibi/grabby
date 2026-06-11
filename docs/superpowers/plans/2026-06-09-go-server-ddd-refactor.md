# go-server DDD Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `go-server` from a flat `package main` layout into a gradual DDD-style structure while preserving existing API, WebSocket, MCP, database, and frontend behavior.

**Architecture:** Move code into `cmd/grabby-server` plus `internal/{domain,application,infrastructure,interfaces,bootstrap,config,logging}`. This first pass prioritizes package boundaries and compile-safe migration over rewriting business behavior. Domain packages hold data types, application packages hold orchestration, infrastructure packages hold SQLite/WebSocket/LLM/file-backed implementations, interfaces packages hold HTTP/MCP/WebSocket adapters, and bootstrap wires everything together.

**Tech Stack:** Go 1.25, `net/http`, `modernc.org/sqlite`, `github.com/gorilla/websocket`, `github.com/mark3labs/mcp-go`, `github.com/firebase/genkit/go`, `go.uber.org/zap`, Vite frontend embedded via Go `embed`.

---

## Current Baseline

Run before implementation:

```bash
cd go-server && go test ./...
```

Current expected result before any refactor: FAIL at compile time because existing `db_ai_test.go` calls old signatures:

```text
./db_ai_test.go:222:46: not enough arguments in call to db.GetAIDailyReport
./db_ai_test.go:231:43: not enough arguments in call to db.GetAIDailyReports
```

This means Task 1 first fixes the already-broken tests so the refactor has a trustworthy baseline.

---

## File Structure Map

### Create

- `go-server/cmd/grabby-server/main.go` — final CLI entrypoint.
- `go-server/internal/bootstrap/app.go` — app construction, dependency wiring, HTTP server lifecycle, graceful shutdown.
- `go-server/internal/config/settings.go` — moved config loading and AI settings normalization access.
- `go-server/internal/logging/logger.go` — moved zap logger singleton.
- `go-server/internal/domain/browser/types.go` — browser registration and active browser info types.
- `go-server/internal/domain/capture/types.go` — browser request/response/page content types.
- `go-server/internal/domain/source/types.go` — source form, source entity, fetch log.
- `go-server/internal/domain/item/types.go` — scraped item, item filters, cursor helpers, item-with-AI view.
- `go-server/internal/domain/ai/types.go` — AI settings, provider profiles, analysis, category stat, daily report.
- `go-server/internal/interfaces/dto/types.go` — HTTP and MCP request/response DTOs plus `ParseArgs`.
- `go-server/internal/infrastructure/sqlite/database.go` — database open/close and migration entry.
- `go-server/internal/infrastructure/sqlite/migrations.go` — schema migrations.
- `go-server/internal/infrastructure/sqlite/seed.go` — default source seeding.
- `go-server/internal/infrastructure/sqlite/source_repo.go` — source CRUD and fetch log methods.
- `go-server/internal/infrastructure/sqlite/item_repo.go` — item CRUD, cursor, stats item queries.
- `go-server/internal/infrastructure/sqlite/ai_repo.go` — AI analysis/report queries.
- `go-server/internal/infrastructure/sqlite/settings_repo.go` — settings persistence and AI settings loading.
- `go-server/internal/infrastructure/browserregistry/registry.go` — file-backed browser registry.
- `go-server/internal/infrastructure/browserws/manager.go` — WebSocket connection manager.
- `go-server/internal/infrastructure/llm/lmstudio.go` — LM Studio client and rate limiter.
- `go-server/internal/application/ai/engine.go` — AI engine.
- `go-server/internal/application/ai/daily.go` — daily report manager.
- `go-server/internal/application/ai/selector.go` — AI profile selector.
- `go-server/internal/application/scraping/classifier.go` — classifier.
- `go-server/internal/application/scraping/scraper.go` — scraper.
- `go-server/internal/application/scraping/task_queue.go` — async web scrape queue.
- `go-server/internal/application/scheduler/scheduler.go` — cron scheduler.
- `go-server/internal/interfaces/http/router.go` — all REST route registration and static frontend serving.
- `go-server/internal/interfaces/http/handlers.go` — non-AI REST handlers.
- `go-server/internal/interfaces/http/ai_handlers.go` — AI REST and RSS handlers.
- `go-server/internal/interfaces/websocket/handlers.go` — `/ws_browser` and `/ws_command` handlers.
- `go-server/internal/interfaces/mcp/server.go` — MCP server and tools.

### Modify

- `go-server/go.mod` — module name may remain `go-server`; imports use `go-server/internal/...`.
- Existing tests moved into matching package directories and updated imports.
- `go-server/main.go` — either removed after creating `cmd/grabby-server/main.go` or replaced with a thin compatibility entrypoint if needed for existing run commands.

### Remove After Migration

- `go-server/config.go`
- `go-server/logger.go`
- `go-server/types.go`
- `go-server/db.go`
- `go-server/browser_registry.go`
- `go-server/websocket_manager.go`
- `go-server/lmstudio.go`
- `go-server/ai_engine.go`
- `go-server/ai_daily.go`
- `go-server/ai_selector.go`
- `go-server/ai_handlers.go`
- `go-server/classifier.go`
- `go-server/scrapers.go`
- `go-server/task_queue.go`
- `go-server/scheduler.go`
- root-level tests after they are moved.

---

### Task 1: Repair Existing Baseline Tests

**Files:**
- Modify: `go-server/db_ai_test.go`

- [ ] **Step 1: Update daily report test calls to current signatures**

In `go-server/db_ai_test.go`, replace the old calls:

```go
retrievedReport, err := db.GetAIDailyReport(dateStr)
reportsList, err := db.GetAIDailyReports(10)
```

with:

```go
retrievedReport, err := db.GetAIDailyReport(dateStr, "daily")
reportsList, err := db.GetAIDailyReports(10, "")
```

- [ ] **Step 2: Run database AI test**

Run:

```bash
cd go-server && go test ./... 
```

Expected: either PASS for root package or reveal the next existing compile/test failure. If another failure appears, fix only the existing broken test expectation, not refactor code yet.

- [ ] **Step 3: Commit baseline test repair**

```bash
git add go-server/db_ai_test.go
git commit -m "test: fix go-server daily report test signatures"
```

---

### Task 2: Create DDD Directory Skeleton

**Files:**
- Create all directories listed in the file structure map.

- [ ] **Step 1: Create directories**

Run:

```bash
mkdir -p \
  go-server/cmd/grabby-server \
  go-server/internal/bootstrap \
  go-server/internal/config \
  go-server/internal/logging \
  go-server/internal/domain/browser \
  go-server/internal/domain/capture \
  go-server/internal/domain/source \
  go-server/internal/domain/item \
  go-server/internal/domain/ai \
  go-server/internal/interfaces/dto \
  go-server/internal/interfaces/http \
  go-server/internal/interfaces/websocket \
  go-server/internal/interfaces/mcp \
  go-server/internal/infrastructure/sqlite \
  go-server/internal/infrastructure/browserregistry \
  go-server/internal/infrastructure/browserws \
  go-server/internal/infrastructure/llm \
  go-server/internal/application/ai \
  go-server/internal/application/scraping \
  go-server/internal/application/scheduler
```

- [ ] **Step 2: Add package marker files with minimal comments**

Create `go-server/internal/domain/ai/doc.go`:

```go
package ai
```

Create `go-server/internal/domain/browser/doc.go`:

```go
package browser
```

Create `go-server/internal/domain/capture/doc.go`:

```go
package capture
```

Create `go-server/internal/domain/source/doc.go`:

```go
package source
```

Create `go-server/internal/domain/item/doc.go`:

```go
package item
```

- [ ] **Step 3: Verify empty package skeleton does not affect tests**

Run:

```bash
cd go-server && go test ./...
```

Expected: same result as Task 1 Step 2.

- [ ] **Step 4: Commit skeleton**

```bash
git add go-server/internal go-server/cmd
git commit -m "refactor: add go-server DDD package skeleton"
```

---

### Task 3: Split Domain and DTO Types

**Files:**
- Create: `go-server/internal/domain/capture/types.go`
- Create: `go-server/internal/domain/source/types.go`
- Create: `go-server/internal/domain/item/types.go`
- Create: `go-server/internal/domain/ai/types.go`
- Create: `go-server/internal/interfaces/dto/types.go`
- Modify later consumers after this task.

- [ ] **Step 1: Create capture domain types**

Create `go-server/internal/domain/capture/types.go`:

```go
package capture

// BrowserRequest is sent from server to browser extension via WebSocket.
type BrowserRequest struct {
	Type      string `json:"type,omitempty"`
	Source    string `json:"source,omitempty"`
	Action    string `json:"action,omitempty"`
	Command   string `json:"command"`
	URL       string `json:"url"`
	FullPage  bool   `json:"fullPage,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// BrowserResponse is returned by the browser extension.
type BrowserResponse struct {
	Type      string     `json:"type,omitempty"`
	MessageID string     `json:"message_id,omitempty"`
	Command   string     `json:"command,omitempty"`
	Success   bool       `json:"success,omitempty"`
	Error     string     `json:"error,omitempty"`
	Result    PageResult `json:"result,omitempty"`
}

// PageResult is the nested result data from the browser.
type PageResult struct {
	URL       string      `json:"url"`
	Title     string      `json:"title"`
	Timestamp string      `json:"timestamp"`
	Content   PageContent `json:"content"`
	ImageData string      `json:"imageData"`
	Format    string      `json:"format"`
	Quality   int         `json:"quality"`
}

// PageContent is the extracted page content.
type PageContent struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	Markdown   string `json:"markdown"`
	Author     string `json:"author"`
	Published  string `json:"published"`
	Site       string `json:"site"`
	Language   string `json:"language"`
	WordCount  int    `json:"wordCount"`
	Image      string `json:"image"`
	Favicon    string `json:"favicon"`
	Domain     string `json:"domain"`
	HTML       string `json:"html"`
	TextLength int    `json:"textLength"`
}

// MarkdownContent returns the markdown content, preferring Content.
func (pc PageContent) MarkdownContent() string {
	if pc.Content != "" {
		return pc.Content
	}
	return pc.Markdown
}
```

- [ ] **Step 2: Create source domain types**

Create `go-server/internal/domain/source/types.go`:

```go
package source

import "time"

// Source represents the configuration of a data source.
type Source struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	URL             string     `json:"url"`
	Schedule        string     `json:"schedule"`
	Enabled         int        `json:"enabled"`
	DefaultCategory string     `json:"default_category"`
	Config          string     `json:"config"`
	LastETag        *string    `json:"last_etag"`
	LastModified    *string    `json:"last_modified"`
	LastFetchAt     *time.Time `json:"last_fetch_at"`
	LastFetchStatus *string    `json:"last_fetch_status"`
	Category        string     `json:"category"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// FetchLog represents the history of a scrape execution.
type FetchLog struct {
	ID           int64      `json:"id"`
	SourceID     string     `json:"source_id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"`
	ItemsFound   int        `json:"items_found"`
	ItemsAdded   int        `json:"items_added"`
	ErrorMessage string     `json:"error_message"`
}

// SourceForm represents the form data submitted to create/update a Source.
type SourceForm struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Schedule        string `json:"schedule"`
	DefaultCategory string `json:"default_category"`
	Config          string `json:"config"`
	Category        string `json:"category"`
}
```

- [ ] **Step 3: Create item domain types**

Create `go-server/internal/domain/item/types.go`:

```go
package item

import "time"

// ScrapedItem represents a single scraped content item.
type ScrapedItem struct {
	ID             int64      `json:"id"`
	SourceID       string     `json:"source_id"`
	OriginSource   string     `json:"origin_source"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	Summary        string     `json:"summary"`
	Content        string     `json:"content"`
	Category       string     `json:"category"`
	SourceCategory string     `json:"source_category"`
	PublishedAt    *time.Time `json:"published_at"`
	FetchedAt      time.Time  `json:"fetched_at"`
	ReadStatus     int        `json:"read_status"`
	Starred        int        `json:"starred"`
	Tags           string     `json:"tags"`
}

// ItemsFilter controls item listing queries.
type ItemsFilter struct {
	Category     string
	OriginSource string
	ReadStatus   *int
	Starred      *int
	Limit        int
	Cursor       string
}

// AIItemsFilter controls AI-enriched item listing queries.
type AIItemsFilter struct {
	AICategory string
	ScoreMin   int
	Days       int
	Limit      int
	Cursor     string
}

// ScrapedItemWithAI represents a scraped item along with its AI analysis details.
type ScrapedItemWithAI struct {
	ScrapedItem
	AICategory    string     `json:"ai_category"`
	AISubcategory string     `json:"ai_subcategory"`
	QualityScore  int        `json:"quality_score"`
	AISummary     string     `json:"ai_summary"`
	AIComment     string     `json:"ai_comment"`
	AITags        string     `json:"ai_tags"`
	AIModelUsed   string     `json:"ai_model_used"`
	AIProcessedAt *time.Time `json:"ai_processed_at"`
}
```

- [ ] **Step 4: Create AI domain types**

Create `go-server/internal/domain/ai/types.go`:

```go
package ai

import "time"

// AIAnalysis represents the AI analysis details of a scraped item.
type AIAnalysis struct {
	ID            int64     `json:"id"`
	ItemID        int64     `json:"item_id"`
	AICategory    string    `json:"ai_category"`
	AISubcategory string    `json:"ai_subcategory"`
	QualityScore  int       `json:"quality_score"`
	AISummary     string    `json:"ai_summary"`
	AIComment     string    `json:"ai_comment"`
	AITags        string    `json:"ai_tags"`
	ModelUsed     string    `json:"model_used"`
	ProcessedAt   time.Time `json:"processed_at"`
}

// AIDailyReport represents the generated daily report.
type AIDailyReport struct {
	ID                int64     `json:"id"`
	ReportDate        string    `json:"report_date"`
	ReportType        string    `json:"report_type"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	TotalItems        int       `json:"total_items"`
	QualityItems      int       `json:"quality_items"`
	CategoriesSummary string    `json:"categories_summary"`
	ModelUsed         string    `json:"model_used"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// AICategoryStat contains category aggregation stats.
type AICategoryStat struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
	AvgScore int    `json:"avg_score"`
}

// AISettings configures AI processing.
type AISettings struct {
	Enabled          bool                `json:"enabled"`
	Provider         string              `json:"provider"`
	APIKey           string              `json:"api_key"`
	Model            string              `json:"model"`
	BaseURL          string              `json:"base_url"`
	QualityThreshold int                 `json:"quality_threshold"`
	SystemPrompt     string              `json:"system_prompt"`
	DailyPrompt      string              `json:"daily_prompt"`
	WorkerCount      int                 `json:"worker_count"`
	BackfillEnabled  bool                `json:"backfill_enabled"`
	ActiveProfileID  string              `json:"active_profile_id"`
	Profiles         []AIProviderProfile `json:"profiles"`
}

// AIProviderProfile configures one AI provider profile.
type AIProviderProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url"`
	Enabled      bool   `json:"enabled"`
	Priority     int    `json:"priority"`
	Strategy     string `json:"strategy"`
	RPM          int    `json:"rpm"`
	Timeout      int    `json:"timeout"`
	SystemPrompt string `json:"system_prompt"`
	DailyPrompt  string `json:"daily_prompt"`
}
```

- [ ] **Step 5: Create DTO types and MCP parser**

Create `go-server/internal/interfaces/dto/types.go`:

```go
package dto

import (
	"encoding/json"
	"time"

	"go-server/internal/domain/browser"
)

// ExtractAPIRequest is the POST /api/extract request body.
type ExtractAPIRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// ExtractAPIResponse is the POST /api/extract response body.
type ExtractAPIResponse struct {
	Success  bool   `json:"success"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

// ScreenshotAPIRequest is the POST /api/screenshot request body.
type ScreenshotAPIRequest struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ScreenshotAPIResponse is the POST /api/screenshot response body.
type ScreenshotAPIResponse struct {
	Success   bool   `json:"success"`
	URL       string `json:"url"`
	ImageData string `json:"imageData"`
}

// HealthResponse is the GET /api/health response body.
type HealthResponse struct {
	Status           string    `json:"status"`
	BrowserConnected bool      `json:"browser_connected"`
	Timestamp        time.Time `json:"timestamp"`
}

// BrowserRegisterRequest is the POST /api/browsers/register request body.
type BrowserRegisterRequest struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

// BrowserRegisterResponse is the POST /api/browsers/register response body.
type BrowserRegisterResponse struct {
	Success bool                        `json:"success"`
	Browser browser.BrowserRegistration `json:"browser"`
}

// BrowserListResponse is the GET /api/browsers response body.
type BrowserListResponse struct {
	Browsers []browser.BrowserInfo `json:"browsers"`
	Count    int                   `json:"count"`
}

// ScreenshotParams for the "screenshot" MCP tool.
type ScreenshotParams struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ExtractParams for the "extract" MCP tool.
type ExtractParams struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// AddParams for the "add" MCP tool.
type AddParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// ParseArgs converts raw MCP arguments into a typed struct.
func ParseArgs[T any](raw any) (T, error) {
	var result T
	b, err := json.Marshal(raw)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return result, err
	}
	return result, nil
}
```

- [ ] **Step 6: Create browser domain types**

Create `go-server/internal/domain/browser/types.go`:

```go
package browser

// BrowserRegistration is one persisted browser registration.
type BrowserRegistration struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

// BrowserInfo describes an active named browser connection.
type BrowserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

- [ ] **Step 7: Run gofmt on new files**

```bash
gofmt -w \
  go-server/internal/domain/browser/types.go \
  go-server/internal/domain/capture/types.go \
  go-server/internal/domain/source/types.go \
  go-server/internal/domain/item/types.go \
  go-server/internal/domain/ai/types.go \
  go-server/internal/interfaces/dto/types.go
```

- [ ] **Step 8: Commit domain and DTO types**

```bash
git add go-server/internal/domain go-server/internal/interfaces/dto
git commit -m "refactor: split go-server domain and DTO types"
```

---

### Task 4: Move Config and Logging Packages

**Files:**
- Create: `go-server/internal/config/settings.go`
- Create: `go-server/internal/logging/logger.go`
- Modify: original imports after later tasks.

- [ ] **Step 1: Move config file**

Run:

```bash
git mv go-server/config.go go-server/internal/config/settings.go
```

Then edit the first line of `go-server/internal/config/settings.go` from:

```go
package main
```

to:

```go
package config
```

- [ ] **Step 2: Import AI domain in settings**

In `go-server/internal/config/settings.go`, add this import if it does not already exist:

```go
import "go-server/internal/domain/ai"
```

Replace references to bare AI settings types:

```go
AISettings
AIProviderProfile
NormalizeAISettings
```

with:

```go
ai.AISettings
ai.AIProviderProfile
ai.NormalizeAISettings
```

If `NormalizeAISettings` still lives in the old root `types.go` at this point, leave only the type references changed and move `NormalizeAISettings` in Task 5.

- [ ] **Step 3: Move logger file**

Run:

```bash
git mv go-server/logger.go go-server/internal/logging/logger.go
```

Then edit the first line of `go-server/internal/logging/logger.go` from:

```go
package main
```

to:

```go
package logging
```

- [ ] **Step 4: Run gofmt**

```bash
gofmt -w go-server/internal/config/settings.go go-server/internal/logging/logger.go
```

- [ ] **Step 5: Commit config and logging move**

```bash
git add go-server/internal/config go-server/internal/logging
git commit -m "refactor: move config and logging packages"
```

---

### Task 5: Move AI Settings Normalization Into Domain

**Files:**
- Modify: `go-server/internal/domain/ai/types.go`
- Modify: `go-server/types.go` before deletion.

- [ ] **Step 1: Copy NormalizeAISettings from old types file into AI domain**

Append to `go-server/internal/domain/ai/types.go` the existing normalization logic from `go-server/types.go`. The function signature must be:

```go
// NormalizeAISettings fills defaults and normalizes AI provider profiles.
func NormalizeAISettings(settings AISettings) AISettings {
	if settings.Provider == "" {
		settings.Provider = "lmstudio"
	}
	if settings.Model == "" {
		settings.Model = "local-model"
	}
	if settings.BaseURL == "" {
		settings.BaseURL = "http://localhost:1234"
	}
	if settings.QualityThreshold == 0 {
		settings.QualityThreshold = 7
	}
	if settings.WorkerCount <= 0 {
		settings.WorkerCount = 1
	}
	if settings.ActiveProfileID == "" && len(settings.Profiles) > 0 {
		settings.ActiveProfileID = settings.Profiles[0].ID
	}
	for i := range settings.Profiles {
		if settings.Profiles[i].ID == "" {
			settings.Profiles[i].ID = settings.Profiles[i].Name
		}
		if settings.Profiles[i].Provider == "" {
			settings.Profiles[i].Provider = settings.Provider
		}
		if settings.Profiles[i].Model == "" {
			settings.Profiles[i].Model = settings.Model
		}
		if settings.Profiles[i].BaseURL == "" {
			settings.Profiles[i].BaseURL = settings.BaseURL
		}
		if settings.Profiles[i].Timeout == 0 {
			settings.Profiles[i].Timeout = 60
		}
		if settings.Profiles[i].RPM == 0 {
			settings.Profiles[i].RPM = 60
		}
	}
	return settings
}
```

If the old implementation has additional fields or defaults, preserve those exact rules instead of the sample body above.

- [ ] **Step 2: Update config to call domain normalizer**

In `go-server/internal/config/settings.go`, ensure calls use:

```go
settings.AISettings = ai.NormalizeAISettings(settings.AISettings)
```

- [ ] **Step 3: Run targeted compile check**

```bash
cd go-server && go test ./internal/domain/ai ./internal/config
```

Expected: PASS or `? [no test files]` for both packages.

- [ ] **Step 4: Commit AI settings domain move**

```bash
git add go-server/internal/domain/ai/types.go go-server/internal/config/settings.go
git commit -m "refactor: move AI settings normalization to domain"
```

---

### Task 6: Move Browser Registry Infrastructure

**Files:**
- Move: `go-server/browser_registry.go` to `go-server/internal/infrastructure/browserregistry/registry.go`
- Move test: `go-server/browser_registry_test.go` to `go-server/internal/infrastructure/browserregistry/registry_test.go`
- Modify: package and imports.

- [ ] **Step 1: Move registry files**

```bash
git mv go-server/browser_registry.go go-server/internal/infrastructure/browserregistry/registry.go
git mv go-server/browser_registry_test.go go-server/internal/infrastructure/browserregistry/registry_test.go
```

- [ ] **Step 2: Change package names**

In both moved files, replace:

```go
package main
```

with:

```go
package browserregistry
```

- [ ] **Step 3: Import browser domain and update type names**

In `registry.go`, add:

```go
import "go-server/internal/domain/browser"
```

Replace persisted registration type usage:

```go
BrowserRegistration
```

with:

```go
browser.BrowserRegistration
```

The exported constructor and error names remain:

```go
func NewBrowserRegistry(path string) (*BrowserRegistry, error)
var ErrBrowserRegistryConflict error
```

- [ ] **Step 4: Run registry tests**

```bash
cd go-server && go test ./internal/infrastructure/browserregistry
```

Expected: PASS.

- [ ] **Step 5: Commit browser registry move**

```bash
git add go-server/internal/infrastructure/browserregistry
git commit -m "refactor: move browser registry infrastructure"
```

---

### Task 7: Move Browser WebSocket Infrastructure

**Files:**
- Move: `go-server/websocket_manager.go` to `go-server/internal/infrastructure/browserws/manager.go`
- Move test: `go-server/websocket_manager_test.go` to `go-server/internal/infrastructure/browserws/manager_test.go`
- Modify package and imports.

- [ ] **Step 1: Move files**

```bash
git mv go-server/websocket_manager.go go-server/internal/infrastructure/browserws/manager.go
git mv go-server/websocket_manager_test.go go-server/internal/infrastructure/browserws/manager_test.go
```

- [ ] **Step 2: Change package names**

In both files, replace:

```go
package main
```

with:

```go
package browserws
```

- [ ] **Step 3: Import domain packages**

In `manager.go`, add imports:

```go
"go-server/internal/domain/browser"
"go-server/internal/domain/capture"
```

Replace:

```go
BrowserInfo
BrowserRequest
BrowserResponse
```

with:

```go
browser.BrowserInfo
capture.BrowserRequest
capture.BrowserResponse
```

Keep public method names unchanged:

```go
func NewWebSocketManager(logger *zap.Logger) *WebSocketManager
func NewWSConn(conn *websocket.Conn, logger *zap.Logger) *WSConn
func (wm *WebSocketManager) SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
```

- [ ] **Step 4: Update tests for browser domain type**

In `manager_test.go`, if expected browser list literals use `BrowserInfo`, change them to:

```go
browser.BrowserInfo{ID: "conn-1", Name: "chrome"}
```

and add:

```go
import "go-server/internal/domain/browser"
```

- [ ] **Step 5: Run browserws tests**

```bash
cd go-server && go test ./internal/infrastructure/browserws
```

Expected: PASS.

- [ ] **Step 6: Commit browserws move**

```bash
git add go-server/internal/infrastructure/browserws
git commit -m "refactor: move browser websocket infrastructure"
```

---

### Task 8: Move LLM Infrastructure

**Files:**
- Move: `go-server/lmstudio.go` to `go-server/internal/infrastructure/llm/lmstudio.go`

- [ ] **Step 1: Move LM Studio file**

```bash
git mv go-server/lmstudio.go go-server/internal/infrastructure/llm/lmstudio.go
```

- [ ] **Step 2: Change package name**

In `go-server/internal/infrastructure/llm/lmstudio.go`, replace:

```go
package main
```

with:

```go
package llm
```

- [ ] **Step 3: Keep exported API stable**

Ensure these names remain exported for AI engine migration:

```go
type RateLimiter struct
func NewRateLimiter(maxPerMinute int) *RateLimiter
type LMStudioClient struct
func NewLMStudioClient(baseURL, model string, logger *zap.Logger) *LMStudioClient
func StripMarkdownFences(s string) string
```

- [ ] **Step 4: Run package compile check**

```bash
cd go-server && go test ./internal/infrastructure/llm
```

Expected: PASS or `? [no test files]`.

- [ ] **Step 5: Commit LLM move**

```bash
git add go-server/internal/infrastructure/llm
git commit -m "refactor: move LLM infrastructure"
```

---

### Task 9: Split SQLite Infrastructure

**Files:**
- Move/split: `go-server/db.go` into `go-server/internal/infrastructure/sqlite/*.go`
- Move tests: `go-server/db_test.go`, `go-server/db_ai_test.go`

- [ ] **Step 1: Move db.go as initial database.go**

```bash
git mv go-server/db.go go-server/internal/infrastructure/sqlite/database.go
git mv go-server/db_test.go go-server/internal/infrastructure/sqlite/database_test.go
git mv go-server/db_ai_test.go go-server/internal/infrastructure/sqlite/ai_repo_test.go
```

- [ ] **Step 2: Change package names**

In all three moved files, replace:

```go
package main
```

with:

```go
package sqlite
```

- [ ] **Step 3: Add domain aliases during migration**

At the top of `database.go`, import domain packages:

```go
import (
	// existing imports...
	"go-server/internal/domain/ai"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
)
```

Then add type aliases after imports to keep the first split mechanical:

```go
type Source = source.Source
type FetchLog = source.FetchLog
type ScrapedItem = item.ScrapedItem
type ItemsFilter = item.ItemsFilter
type AIItemsFilter = item.AIItemsFilter
type ScrapedItemWithAI = item.ScrapedItemWithAI
type AIAnalysis = ai.AIAnalysis
type AIDailyReport = ai.AIDailyReport
type AICategoryStat = ai.AICategoryStat
type AISettings = ai.AISettings
type AIProviderProfile = ai.AIProviderProfile
```

- [ ] **Step 4: Update AI settings normalizer call**

In `LoadAISettings`, replace old normalizer call:

```go
return NormalizeAISettings(settings), nil
```

with:

```go
return ai.NormalizeAISettings(settings), nil
```

- [ ] **Step 5: Avoid external access to raw sql.DB for stats**

Add these methods to `database.go` or later `stats_repo.go`:

```go
func (d *Database) CountItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items").Scan(&count)
	return count, err
}

func (d *Database) CountUnreadItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE read_status = 0").Scan(&count)
	return count, err
}

func (d *Database) CountStarredItems() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE starred = 1").Scan(&count)
	return count, err
}

func (d *Database) CountAIAnalyses() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM ai_analyses").Scan(&count)
	return count, err
}

func (d *Database) CountUnprocessedAIItems() (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*)
		FROM scraped_items si
		LEFT JOIN ai_analyses aa ON si.id = aa.item_id
		WHERE aa.id IS NULL
	`).Scan(&count)
	return count, err
}

func (d *Database) AverageAIQualityScore() (float64, error) {
	var avg float64
	err := d.db.QueryRow("SELECT COALESCE(AVG(quality_score), 0) FROM ai_analyses").Scan(&avg)
	return avg, err
}
```

- [ ] **Step 6: Run sqlite tests**

```bash
cd go-server && go test ./internal/infrastructure/sqlite
```

Expected: PASS after package/import fixes.

- [ ] **Step 7: Split database.go into focused files**

Move functions by responsibility without changing signatures:

```text
migrate() -> migrations.go
seedDefaultSources() -> seed.go
GetSources/GetSource/InsertSource/UpdateSource/DeleteSource/ToggleSource/UpdateSourceFetchStatus/FetchLog methods -> source_repo.go
GetScrapedItems/GetScrapedItem/InsertScrapedItem/MarkItemRead/ToggleItemStarred/CleanupOldData/count methods/cursor helpers -> item_repo.go
InsertAIAnalysis/GetAIAnalysis/GetUnanalyzedItems/GetScrapedItemsWithAI/GetAICategories/AIDailyReport methods/date range methods -> ai_repo.go
GetSetting/SaveSetting/LoadAISettings -> settings_repo.go
NewDatabase/Close/Database struct -> database.go
```

When moving code, keep all functions in package `sqlite`, so no exported signatures need to change.

- [ ] **Step 8: Run sqlite tests again**

```bash
cd go-server && go test ./internal/infrastructure/sqlite
```

Expected: PASS.

- [ ] **Step 9: Commit sqlite migration**

```bash
git add go-server/internal/infrastructure/sqlite
git commit -m "refactor: split sqlite infrastructure package"
```

---

### Task 10: Move AI Application Package

**Files:**
- Move: `go-server/ai_engine.go` to `go-server/internal/application/ai/engine.go`
- Move: `go-server/ai_daily.go` to `go-server/internal/application/ai/daily.go`
- Move: `go-server/ai_selector.go` to `go-server/internal/application/ai/selector.go`
- Move tests: `go-server/ai_test.go` to `go-server/internal/application/ai/engine_test.go`
- Modify imports.

- [ ] **Step 1: Move AI files**

```bash
git mv go-server/ai_engine.go go-server/internal/application/ai/engine.go
git mv go-server/ai_daily.go go-server/internal/application/ai/daily.go
git mv go-server/ai_selector.go go-server/internal/application/ai/selector.go
git mv go-server/ai_test.go go-server/internal/application/ai/engine_test.go
```

- [ ] **Step 2: Change package names**

In moved files, replace:

```go
package main
```

with:

```go
package aiapp
```

- [ ] **Step 3: Add imports and aliases**

In `engine.go`, `daily.go`, and `selector.go`, import:

```go
"go-server/internal/domain/ai"
"go-server/internal/domain/item"
"go-server/internal/infrastructure/llm"
"go-server/internal/infrastructure/sqlite"
```

Use these signatures:

```go
func NewAIEngine(settings ai.AISettings, db *sqlite.Database, logger *zap.Logger) (*AIEngine, error)
func NewAIDailyManager(db *sqlite.Database, aiEngine *AIEngine, logger *zap.Logger) *AIDailyManager
func NewProfileSelector(settings ai.AISettings) *ProfileSelector
```

Replace old type names:

```go
AISettings -> ai.AISettings
AIProviderProfile -> ai.AIProviderProfile
AIAnalysis -> ai.AIAnalysis
AIDailyReport -> ai.AIDailyReport
ScrapedItem -> item.ScrapedItem
LMStudioClient -> llm.LMStudioClient
NewLMStudioClient -> llm.NewLMStudioClient
NewRateLimiter -> llm.NewRateLimiter
StripMarkdownFences -> llm.StripMarkdownFences
Database -> sqlite.Database
```

- [ ] **Step 4: Update tests**

In `engine_test.go`, import:

```go
"go-server/internal/domain/ai"
"go-server/internal/infrastructure/sqlite"
"go-server/internal/logging"
```

Replace:

```go
NewDatabase(":memory:")
AISettings{...}
NewAIEngine(...)
NewAIDailyManager(...)
GetLogger()
```

with:

```go
sqlite.NewDatabase(":memory:")
ai.AISettings{...}
NewAIEngine(...)
NewAIDailyManager(...)
logging.GetLogger()
```

- [ ] **Step 5: Run AI application tests**

```bash
cd go-server && go test ./internal/application/ai
```

Expected: PASS.

- [ ] **Step 6: Commit AI application move**

```bash
git add go-server/internal/application/ai
git commit -m "refactor: move AI application services"
```

---

### Task 11: Move Scraping Application Package

**Files:**
- Move: `go-server/classifier.go` to `go-server/internal/application/scraping/classifier.go`
- Move: `go-server/scrapers.go` to `go-server/internal/application/scraping/scraper.go`
- Move: `go-server/task_queue.go` to `go-server/internal/application/scraping/task_queue.go`
- Move test: `go-server/classifier_test.go` to `go-server/internal/application/scraping/classifier_test.go`

- [ ] **Step 1: Move files**

```bash
git mv go-server/classifier.go go-server/internal/application/scraping/classifier.go
git mv go-server/scrapers.go go-server/internal/application/scraping/scraper.go
git mv go-server/task_queue.go go-server/internal/application/scraping/task_queue.go
git mv go-server/classifier_test.go go-server/internal/application/scraping/classifier_test.go
```

- [ ] **Step 2: Change package names**

In moved files, replace:

```go
package main
```

with:

```go
package scraping
```

- [ ] **Step 3: Add imports and update types**

In `scraper.go` and `task_queue.go`, import:

```go
"go-server/internal/application/ai"
"go-server/internal/domain/capture"
"go-server/internal/domain/item"
"go-server/internal/domain/source"
"go-server/internal/infrastructure/browserws"
"go-server/internal/infrastructure/sqlite"
```

To avoid alias conflict with domain AI, import the app package as:

```go
aiapp "go-server/internal/application/ai"
```

Replace old types:

```go
Database -> sqlite.Database
WebSocketManager -> browserws.WebSocketManager
AIEngine -> aiapp.AIEngine
BrowserRequest -> capture.BrowserRequest
Source -> source.Source
ScrapedItem -> item.ScrapedItem
FetchLog -> source.FetchLog
```

Keep constructors:

```go
func NewScraper(db *sqlite.Database, wsManager *browserws.WebSocketManager, taskQueue *TaskQueue, logger *zap.Logger, aiEngine *aiapp.AIEngine) *Scraper
func NewTaskQueue(wsManager *browserws.WebSocketManager, db *sqlite.Database, logger *zap.Logger, concurrency int, aiEngine *aiapp.AIEngine) *TaskQueue
```

- [ ] **Step 4: Replace raw `db.db` writes in TaskQueue and Scraper**

Add methods in sqlite package if not added in Task 9:

```go
func (d *Database) MarkFetchLogSkipped(logID int64, message string) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET status = 'skipped', error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?", message, logID)
	return err
}

func (d *Database) MarkFetchLogProgress(logID int64, added int) error {
	_, err := d.db.Exec("UPDATE fetch_logs SET items_found = items_found + 1, items_added = items_added + ?, status = 'success', finished_at = CURRENT_TIMESTAMP WHERE id = ?", added, logID)
	return err
}

func (d *Database) ItemExistsByURL(url string) (bool, error) {
	var exists bool
	err := d.db.QueryRow("SELECT EXISTS(SELECT 1 FROM scraped_items WHERE url = ?)", url).Scan(&exists)
	return exists, err
}
```

Then replace direct `tq.db.db.Exec` and `s.db.db.QueryRow` calls with these methods.

- [ ] **Step 5: Run scraping tests**

```bash
cd go-server && go test ./internal/application/scraping ./internal/infrastructure/sqlite
```

Expected: PASS.

- [ ] **Step 6: Commit scraping move**

```bash
git add go-server/internal/application/scraping go-server/internal/infrastructure/sqlite
git commit -m "refactor: move scraping application services"
```

---

### Task 12: Move Scheduler Application Package

**Files:**
- Move: `go-server/scheduler.go` to `go-server/internal/application/scheduler/scheduler.go`

- [ ] **Step 1: Move scheduler**

```bash
git mv go-server/scheduler.go go-server/internal/application/scheduler/scheduler.go
```

- [ ] **Step 2: Change package name and imports**

Change package:

```go
package scheduler
```

Import:

```go
aiapp "go-server/internal/application/ai"
"go-server/internal/application/scraping"
"go-server/internal/domain/source"
"go-server/internal/infrastructure/sqlite"
```

Use constructor:

```go
func NewScheduler(db *sqlite.Database, scraper *scraping.Scraper, dailyManager *aiapp.AIDailyManager, logger *zap.Logger) *Scheduler
```

Replace source type references with:

```go
source.Source
```

- [ ] **Step 3: Run scheduler compile check**

```bash
cd go-server && go test ./internal/application/scheduler
```

Expected: PASS or `? [no test files]`.

- [ ] **Step 4: Commit scheduler move**

```bash
git add go-server/internal/application/scheduler
git commit -m "refactor: move scheduler application service"
```

---

### Task 13: Extract WebSocket Interface Handlers

**Files:**
- Create: `go-server/internal/interfaces/websocket/handlers.go`
- Later remove copied functions from `go-server/main.go`.

- [ ] **Step 1: Create websocket handlers file**

Create `go-server/internal/interfaces/websocket/handlers.go` with the two functions copied from old `main.go`, package-renamed and imports updated:

```go
package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleWebSocketBrowser(wm *browserws.WebSocketManager, registry *browserregistry.BrowserRegistry, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connID := r.URL.Query().Get("conn_id")
		name := r.URL.Query().Get("name")
		if connID == "" || name == "" {
			http.Error(w, "Missing conn_id or name", http.StatusForbidden)
			return
		}
		if !registry.Validate(connID, name) {
			http.Error(w, "Browser is not registered", http.StatusForbidden)
			return
		}
		if wm.HasConnection(connID) {
			http.Error(w, "Browser id already connected", http.StatusConflict)
			return
		}
		if wm.IsBrowserNameActive(name) {
			http.Error(w, "Browser name already connected", http.StatusConflict)
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", zap.Error(err))
			return
		}
		conn := browserws.NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		_ = wm.RegisterBrowserName(connID, name)
		defer func() {
			wm.UnregisterBrowserName(connID)
			wm.Disconnect(connID)
		}()
		wm.ReadLoop(connID, conn)
	}
}

func HandleWebSocketCommand(wm *browserws.WebSocketManager, settings *config.Settings, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Command WebSocket upgrade failed", zap.Error(err))
			return
		}
		connID := "command"
		conn := browserws.NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		defer wm.Disconnect(connID)
		wm.ReadLoop(connID, conn)
	}
}
```

If old `handleWebSocketCommand` contains additional command-specific behavior, copy that exact body instead of the simplified body above and only update package/type references.

- [ ] **Step 2: Compile websocket interface package**

```bash
cd go-server && go test ./internal/interfaces/websocket
```

Expected: PASS or `? [no test files]`.

- [ ] **Step 3: Commit websocket handlers**

```bash
git add go-server/internal/interfaces/websocket
git commit -m "refactor: extract websocket interface handlers"
```

---

### Task 14: Extract MCP Interface Server

**Files:**
- Create: `go-server/internal/interfaces/mcp/server.go`

- [ ] **Step 1: Create MCP server extraction**

Create `go-server/internal/interfaces/mcp/server.go`:

```go
package mcpiface

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"go-server/internal/config"
	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

func NewServer(wm *browserws.WebSocketManager, settings *config.Settings) *server.SSEServer {
	mcpSvr := server.NewMCPServer("Grabby", "1.0.0")

	screenshotTool := mcp.NewTool("screenshot",
		mcp.WithDescription("Capture a screenshot of a webpage"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL to capture")),
		mcp.WithBoolean("fullPage", mcp.Description("Capture full page")),
		mcp.WithString("browser", mcp.Description("Browser name")),
	)
	mcpSvr.AddTool(screenshotTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.ScreenshotParams](req.GetArguments())
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		browserConnID, err := wm.ResolveBrowserConnID(params.Browser)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(settings.APIExtractTimeout*float64(time.Second)))
		defer cancel()
		resp, err := wm.SendMessage(ctx, &capture.BrowserRequest{Source: "mcp", Action: "mcp_request", Command: "capture", URL: params.URL, FullPage: params.FullPage}, browserConnID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Screenshot failed: %s", err.Error())), nil
		}
		if resp.Success {
			return mcp.NewToolResultText(resp.Result.ImageData), nil
		}
		return mcp.NewToolResultText(""), nil
	})

	extractTool := mcp.NewTool("extract",
		mcp.WithDescription("Extract readable markdown from a webpage"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL to extract")),
		mcp.WithString("browser", mcp.Description("Browser name")),
	)
	mcpSvr.AddTool(extractTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.ExtractParams](req.GetArguments())
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		browserConnID, err := wm.ResolveBrowserConnID(params.Browser)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(settings.APIExtractTimeout*float64(time.Second)))
		defer cancel()
		resp, err := wm.SendMessage(ctx, &capture.BrowserRequest{Source: "mcp", Action: "mcp_request", Command: "extract", URL: params.URL}, browserConnID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Extract failed: %s", err.Error())), nil
		}
		if resp.Success {
			return mcp.NewToolResultText(resp.Result.Content.MarkdownContent()), nil
		}
		return mcp.NewToolResultText(""), nil
	})

	addTool := mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a", mcp.Required()),
		mcp.WithNumber("b", mcp.Required()),
	)
	mcpSvr.AddTool(addTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.AddParams](req.GetArguments())
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%v", params.A+params.B)), nil
	})

	timeTool := mcp.NewTool("get_server_time", mcp.WithDescription("Get server time"))
	mcpSvr.AddTool(timeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(time.Now().Format(time.RFC3339)), nil
	})

	return server.NewSSEServer(mcpSvr)
}
```

- [ ] **Step 2: Compare against old MCP block**

Open old `main.go` MCP registration block and ensure tool names, descriptions, parameter names, timeout behavior, command names, and return values match. If there is a mismatch, edit `server.go` to match old behavior exactly.

- [ ] **Step 3: Compile MCP package**

```bash
cd go-server && go test ./internal/interfaces/mcp
```

Expected: PASS or `? [no test files]`.

- [ ] **Step 4: Commit MCP extraction**

```bash
git add go-server/internal/interfaces/mcp
git commit -m "refactor: extract MCP interface server"
```

---

### Task 15: Extract HTTP Handlers and Router

**Files:**
- Create: `go-server/internal/interfaces/http/router.go`
- Create: `go-server/internal/interfaces/http/handlers.go`
- Move: `go-server/ai_handlers.go` to `go-server/internal/interfaces/http/ai_handlers.go`
- Modify package imports.

- [ ] **Step 1: Move AI handlers**

```bash
git mv go-server/ai_handlers.go go-server/internal/interfaces/http/ai_handlers.go
```

Change package:

```go
package httpiface
```

Import aliases:

```go
aiapp "go-server/internal/application/ai"
"go-server/internal/domain/ai"
"go-server/internal/domain/item"
"go-server/internal/domain/source"
"go-server/internal/infrastructure/sqlite"
```

Constructor signature becomes:

```go
func NewAIHandlers(db *sqlite.Database, aiEngine *aiapp.AIEngine, dailyManager *aiapp.AIDailyManager, logger *zap.Logger) *AIHandlers
```

Replace domain type references accordingly.

- [ ] **Step 2: Replace direct AI stats SQL in AI handlers**

In `HandleStats`, replace direct `h.db.db.QueryRow` calls with sqlite methods from Task 9:

```go
totalProcessed, err := h.db.CountAIAnalyses()
remaining, err := h.db.CountUnprocessedAIItems()
avgScore, err := h.db.AverageAIQualityScore()
```

Keep response JSON fields unchanged.

- [ ] **Step 3: Create app dependency struct for HTTP router**

Create `go-server/internal/interfaces/http/router.go`:

```go
package httpiface

import (
	"embed"
	"net/http"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
	mcpiface "go-server/internal/interfaces/mcp"
	wsiface "go-server/internal/interfaces/websocket"

	"go.uber.org/zap"
)

type Dependencies struct {
	Settings        *config.Settings
	Logger          *zap.Logger
	DB              *sqlite.Database
	WSManager       *browserws.WebSocketManager
	BrowserRegistry *browserregistry.BrowserRegistry
	AIEngine        *aiapp.AIEngine
	DailyManager    *aiapp.AIDailyManager
	TaskQueue       *scraping.TaskQueue
	Scraper         *scraping.Scraper
	Scheduler       *scheduler.Scheduler
	FrontendFS      embed.FS
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	RegisterHandlers(mux, deps)
	RegisterAIHandlers(mux, deps)
	mux.HandleFunc("/ws_browser", wsiface.HandleWebSocketBrowser(deps.WSManager, deps.BrowserRegistry, deps.Logger))
	mux.HandleFunc("/ws_command", wsiface.HandleWebSocketCommand(deps.WSManager, deps.Settings, deps.Logger))
	sseSvr := mcpiface.NewServer(deps.WSManager, deps.Settings)
	mux.Handle("/mcp/sse", sseSvr)
	RegisterStaticHandlers(mux, deps.FrontendFS, deps.Logger)
	return mux
}
```

- [ ] **Step 4: Create non-AI handlers**

Create `go-server/internal/interfaces/http/handlers.go` by copying non-AI `/api/*` handler bodies from old `main.go` and updating package references. Required exported functions:

```go
func RegisterHandlers(mux *http.ServeMux, deps Dependencies)
func RegisterStaticHandlers(mux *http.ServeMux, frontendFS embed.FS, logger *zap.Logger)
func firstNonEmpty(vals ...string) string
```

Within copied code, replace old types:

```go
ExtractAPIRequest -> dto.ExtractAPIRequest
ExtractAPIResponse -> dto.ExtractAPIResponse
ScreenshotAPIRequest -> dto.ScreenshotAPIRequest
ScreenshotAPIResponse -> dto.ScreenshotAPIResponse
BrowserRegisterRequest -> dto.BrowserRegisterRequest
BrowserRegisterResponse -> dto.BrowserRegisterResponse
BrowserListResponse -> dto.BrowserListResponse
BrowserRequest -> capture.BrowserRequest
ItemsFilter -> item.ItemsFilter
Source -> source.Source
SourceForm -> source.SourceForm
```

Replace browser registry conflict check:

```go
errors.Is(err, browserregistry.ErrBrowserRegistryConflict)
```

Replace direct stats SQL with sqlite methods:

```go
totalCount, _ := deps.DB.CountItems()
unreadCount, _ := deps.DB.CountUnreadItems()
starredCount, _ := deps.DB.CountStarredItems()
```

Add sqlite methods for category stats if needed:

```go
func (d *Database) CountItemsByCategory() (map[string]int, error) {
	rows, err := d.db.Query("SELECT category, COUNT(*) FROM scraped_items GROUP BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		result[category] = count
	}
	return result, rows.Err()
}
```

- [ ] **Step 5: Register AI handlers function**

In `ai_handlers.go`, add:

```go
func RegisterAIHandlers(mux *http.ServeMux, deps Dependencies) {
	aiHandlers := NewAIHandlers(deps.DB, deps.AIEngine, deps.DailyManager, deps.Logger)
	mux.HandleFunc("/api/ai/quality", aiHandlers.HandleQuality)
	mux.HandleFunc("/api/ai/categories", aiHandlers.HandleCategories)
	mux.HandleFunc("/api/ai/items", aiHandlers.HandleItems)
	mux.HandleFunc("/api/ai/analysis/", aiHandlers.HandleAnalysis)
	mux.HandleFunc("/api/ai/daily", aiHandlers.HandleDaily)
	mux.HandleFunc("/api/ai/daily/list", aiHandlers.HandleDailyList)
	mux.HandleFunc("/api/ai/daily/generate", aiHandlers.HandleDailyGenerate)
	mux.HandleFunc("/api/ai/daily/rss", aiHandlers.HandleDailyRSS)
	mux.HandleFunc("/api/ai/reanalyze/", aiHandlers.HandleReanalyze)
	mux.HandleFunc("/api/ai/stats", aiHandlers.HandleStats)
	mux.HandleFunc("/api/ai/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			aiHandlers.HandleGetSettings(w, r)
		} else if r.Method == http.MethodPost {
			aiHandlers.HandleSaveSettings(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/ai/test", aiHandlers.HandleTestConnection)
	mux.HandleFunc("/api/ai/start_eval", aiHandlers.HandleStartEvaluation)
}
```

- [ ] **Step 6: Compile HTTP package**

```bash
cd go-server && go test ./internal/interfaces/http
```

Expected: PASS or `? [no test files]` after import fixes.

- [ ] **Step 7: Commit HTTP extraction**

```bash
git add go-server/internal/interfaces/http go-server/internal/infrastructure/sqlite
git commit -m "refactor: extract HTTP interface handlers"
```

---

### Task 16: Add Bootstrap and New Entrypoint

**Files:**
- Create: `go-server/internal/bootstrap/app.go`
- Create: `go-server/cmd/grabby-server/main.go`
- Modify/remove: `go-server/main.go`

- [ ] **Step 1: Create bootstrap app**

Create `go-server/internal/bootstrap/app.go`:

```go
package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
	httpiface "go-server/internal/interfaces/http"

	"go.uber.org/zap"
)

type App struct {
	settings        *config.Settings
	logger          *zap.Logger
	db              *sqlite.Database
	wsManager       *browserws.WebSocketManager
	browserRegistry *browserregistry.BrowserRegistry
	aiEngine        *aiapp.AIEngine
	dailyManager    *aiapp.AIDailyManager
	taskQueue       *scraping.TaskQueue
	scraper         *scraping.Scraper
	scheduler       *scheduler.Scheduler
	frontendFS      embed.FS
}

func NewApp(settings *config.Settings, logger *zap.Logger, frontendFS embed.FS) (*App, error) {
	db, err := sqlite.NewDatabase(config.GetEnv("DB_PATH", "grabby.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	dbAISettings, err := db.LoadAISettings(settings.AISettings)
	if err != nil {
		logger.Error("Failed to load AI settings from database, using env/defaults", zap.Error(err))
	} else {
		settings.AISettings = dbAISettings
	}
	wsManager := browserws.NewWebSocketManager(logger)
	browserRegistry, err := browserregistry.NewBrowserRegistry("")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load browser registry: %w", err)
	}
	aiEngine, err := aiapp.NewAIEngine(settings.AISettings, db, logger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize AI engine: %w", err)
	}
	dailyManager := aiapp.NewAIDailyManager(db, aiEngine, logger)
	taskQueue := scraping.NewTaskQueue(wsManager, db, logger, 1, aiEngine)
	scraper := scraping.NewScraper(db, wsManager, taskQueue, logger, aiEngine)
	scheduler := scheduler.NewScheduler(db, scraper, dailyManager, logger)
	return &App{settings: settings, logger: logger, db: db, wsManager: wsManager, browserRegistry: browserRegistry, aiEngine: aiEngine, dailyManager: dailyManager, taskQueue: taskQueue, scraper: scraper, scheduler: scheduler, frontendFS: frontendFS}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.aiEngine.Start()
	defer a.aiEngine.Stop()
	a.taskQueue.Start(ctx)
	defer a.taskQueue.Shutdown()
	if err := a.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	defer a.scheduler.Stop()

	router := httpiface.NewRouter(httpiface.Dependencies{Settings: a.settings, Logger: a.logger, DB: a.db, WSManager: a.wsManager, BrowserRegistry: a.browserRegistry, AIEngine: a.aiEngine, DailyManager: a.dailyManager, TaskQueue: a.taskQueue, Scraper: a.scraper, Scheduler: a.scheduler, FrontendFS: a.frontendFS})
	server := &http.Server{Addr: fmt.Sprintf(":%d", a.settings.Port), Handler: router}
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("Server starting", zap.Int("port", a.settings.Port))
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		a.wsManager.CloseAll()
		return a.db.Close()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
```

If `config.GetEnv` is not exported, export the old helper by renaming `getEnv` to `GetEnv` in `internal/config/settings.go`.

- [ ] **Step 2: Create new command entrypoint**

Create `go-server/cmd/grabby-server/main.go`:

```go
package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	"go-server/internal/bootstrap"
	"go-server/internal/config"
	"go-server/internal/logging"

	"go.uber.org/zap"
)

//go:embed ../../frontend/dist
var frontendFS embed.FS

func main() {
	settings := config.GetSettings()
	logger := logging.GetLogger()
	defer logging.SyncLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(settings, logger, frontendFS)
	if err != nil {
		logger.Fatal("Failed to initialize app", zap.Error(err))
	}
	if err := app.Run(ctx); err != nil {
		logger.Fatal("Server stopped with error", zap.Error(err))
	}
}
```

If Go rejects `//go:embed ../../frontend/dist`, keep embed in root package compatibility entrypoint or move frontend static serving to read from `frontend/dist` at runtime. Go embed patterns cannot contain `..`; the safe fallback is Task 16 Step 3.

- [ ] **Step 3: Keep root entrypoint as embed compatibility wrapper**

Replace `go-server/main.go` with a thin root entrypoint so existing `go run .` and embed path keep working:

```go
package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	"go-server/internal/bootstrap"
	"go-server/internal/config"
	"go-server/internal/logging"

	"go.uber.org/zap"
)

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
	settings := config.GetSettings()
	logger := logging.GetLogger()
	defer logging.SyncLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(settings, logger, frontendFS)
	if err != nil {
		logger.Fatal("Failed to initialize app", zap.Error(err))
	}
	if err := app.Run(ctx); err != nil {
		logger.Fatal("Server stopped with error", zap.Error(err))
	}
}
```

Because `cmd/grabby-server` cannot embed `../../frontend/dist`, either remove `cmd/grabby-server/main.go` or leave it out until static serving is changed. For this first refactor, preserving `go run .` is more important.

- [ ] **Step 4: Run full compile**

```bash
cd go-server && go test ./...
```

Expected: PASS after all imports and deleted root references are fixed.

- [ ] **Step 5: Commit bootstrap and entrypoint**

```bash
git add go-server/main.go go-server/cmd/grabby-server go-server/internal/bootstrap go-server/internal/config
git commit -m "refactor: add bootstrap and thin server entrypoint"
```

---

### Task 17: Remove Old Root Types File and Fix Imports

**Files:**
- Remove: `go-server/types.go`
- Modify: all packages still referencing root `package main` types.

- [ ] **Step 1: Search for old root type references**

Run:

```bash
rg -n "\b(BrowserRequest|BrowserResponse|PageResult|PageContent|ExtractAPIRequest|ScreenshotAPIRequest|Source|ScrapedItem|FetchLog|AISettings|AIAnalysis|AIDailyReport|ItemsFilter|AIItemsFilter)\b" go-server --glob '*.go'
```

Expected: all matches are in new packages with qualified imports or aliases; no reliance on root `types.go` remains.

- [ ] **Step 2: Remove old types file**

```bash
git rm go-server/types.go
```

- [ ] **Step 3: Run gofmt and full tests**

```bash
gofmt -w $(find go-server -name '*.go' -not -path '*/frontend/*')
cd go-server && go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit cleanup**

```bash
git add go-server
git commit -m "refactor: remove root shared types"
```

---

### Task 18: Final Verification

**Files:**
- Potentially modify only files needed to fix compile/test failures discovered here.

- [ ] **Step 1: Run full Go test suite**

```bash
cd go-server && go test ./...
```

Expected: PASS for all Go packages. The frontend node_modules Go package may show `? [no test files]`; that is acceptable.

- [ ] **Step 2: Build server binary**

```bash
cd go-server && go build ./...
```

Expected: PASS with no compile errors.

- [ ] **Step 3: Run root server help-free smoke compile**

```bash
cd go-server && go test .
```

Expected: PASS or `? go-server [no test files]`.

- [ ] **Step 4: Verify no old flat Go implementation files remain**

Run:

```bash
find go-server -maxdepth 1 -type f -name '*.go' -print | sort
```

Expected output contains only the thin root entrypoint, or no files if the static serving strategy later removes root entrypoint:

```text
go-server/main.go
```

- [ ] **Step 5: Check git status**

```bash
git status --short
```

Expected: clean if all previous task commits were made, or only intentional uncommitted changes if commit prompts were skipped by the execution environment.

- [ ] **Step 6: Commit final fixes if needed**

```bash
git add go-server
git commit -m "refactor: verify go-server DDD migration"
```

---

## Self-Review Notes

- Spec coverage: package structure, config/logging, domain types, SQLite split, AI/scraping/scheduler application moves, HTTP/WebSocket/MCP adapters, bootstrap, tests, and build verification are all covered.
- Scope: this plan intentionally preserves existing APIs and schema; it does not add repository interfaces everywhere or rewrite business logic.
- Known implementation caveat: Go `embed` cannot reference parent directories from `cmd/grabby-server`, so the root `main.go` compatibility entrypoint is retained for this first migration.
- Commit steps are included for workers following the plan, but the current agent must only commit when explicitly authorized by the user or project policy.
