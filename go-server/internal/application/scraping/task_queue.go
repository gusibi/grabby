package scraping

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
)

type ScrapeTask struct {
	SourceID    string
	URL         string
	LogID       int64 // logs task details to fetch_logs
	LogItemID   int64 // fetch_log_items row awaiting this task's final result
	ItemID      int64 // non-zero when backfilling full content for an existing item
	Category    string
	TitleHint   string
	PublishedAt *time.Time
	NewItem     bool
	pending     *persistedReport
	notModified bool
}

// BrowserExecutor is the slice of the WebSocket manager the queue needs. The
// concrete manager satisfies it; tests can provide a deterministic stand-in.
type BrowserExecutor interface {
	ResolveBrowserConnID(name string) (string, error)
	SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
}

type TaskQueue struct {
	mu          sync.Mutex
	finalizeMu  sync.Mutex
	concurrency int // Number of worker goroutines
	wsManager   BrowserExecutor
	db          *sqlite.Database
	logger      *zap.Logger
	running     bool
	taskChan    chan ScrapeTask
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	aiEngine    *appai.AIEngine
}

func NewTaskQueue(wsManager *browserws.WebSocketManager, db *sqlite.Database, logger *zap.Logger, concurrency int, aiEngine *appai.AIEngine) *TaskQueue {
	return NewTaskQueueWithExecutor(wsManager, db, logger, concurrency, aiEngine)
}

// NewTaskQueueWithExecutor is the dependency-injected form used by tests.
func NewTaskQueueWithExecutor(executor BrowserExecutor, db *sqlite.Database, logger *zap.Logger, concurrency int, aiEngine *appai.AIEngine) *TaskQueue {
	if concurrency <= 0 {
		concurrency = 1
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TaskQueue{
		concurrency: concurrency,
		wsManager:   executor,
		db:          db,
		logger:      logger,
		taskChan:    make(chan ScrapeTask, 1000),
		aiEngine:    aiEngine,
	}
}

// IsRunning reports whether workers are consuming the queue. Callers use it to
// choose async browser extraction (with a pending log row) over a synchronous
// attempt.
func (tq *TaskQueue) IsRunning() bool {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	return tq.running
}

func (tq *TaskQueue) Start(ctx context.Context) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if tq.running {
		return
	}
	tq.running = true
	tq.ctx, tq.cancel = context.WithCancel(ctx)

	for i := 0; i < tq.concurrency; i++ {
		tq.wg.Add(1)
		go tq.worker()
	}
	tq.logger.Info("Web Scrape Task Queue started", zap.Int("concurrency", tq.concurrency))
}

func (tq *TaskQueue) Enqueue(tasks []ScrapeTask) {
	var rejected []ScrapeTask
	tq.mu.Lock()
	for _, task := range tasks {
		if !tq.running {
			rejected = append(rejected, task)
			continue
		}
		select {
		case tq.taskChan <- task:
		default:
			rejected = append(rejected, task)
		}
	}
	tq.mu.Unlock()

	for _, task := range rejected {
		tq.recordTaskFailure(task, "access", "浏览器任务队列不可用或已满")
		tq.finalizeIfDone(task)
	}
}

func (tq *TaskQueue) Shutdown() {
	tq.mu.Lock()
	if !tq.running {
		tq.mu.Unlock()
		return
	}
	tq.running = false
	tq.cancel()
	close(tq.taskChan)
	tq.mu.Unlock()

	tq.wg.Wait()
	for task := range tq.taskChan {
		tq.recordTaskFailure(task, "access", "浏览器任务队列已关闭")
		tq.finalizeIfDone(task)
	}
	tq.logger.Info("Web Scrape Task Queue stopped")
}

func (tq *TaskQueue) worker() {
	defer tq.wg.Done()
	for task := range tq.taskChan {
		err := tq.processTask(tq.ctx, task)
		if err != nil {
			tq.logger.Error("Task execution failed", zap.String("url", task.URL), zap.Error(err))
		}
		// Polite delay between tasks to avoid hammering the extension
		select {
		case <-tq.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (tq *TaskQueue) processTask(ctx context.Context, task ScrapeTask) error {
	tq.logger.Info("Processing web scrape task", zap.String("source_id", task.SourceID), zap.String("url", task.URL))

	// Check if browser is available
	browserConnID, err := tq.wsManager.ResolveBrowserConnID("")
	if err != nil {
		tq.recordTaskFailure(task, "access", "没有可用的浏览器连接: "+err.Error())
		if task.LogID > 0 && task.LogItemID == 0 && task.pending == nil {
			tq.db.MarkFetchLogSkipped(task.LogID, "No active browser connected")
		}
		tq.finalizeIfDone(task)
		return fmt.Errorf("no active browser connection to perform scrape: %w", err)
	}

	// 60s timeout for extract command
	scrapeCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := tq.wsManager.SendMessage(scrapeCtx, &capture.BrowserRequest{
		Source:  "scheduler_task_queue",
		Action:  "mcp_request",
		Command: "extract",
		URL:     task.URL,
		Visible: false,
	}, browserConnID)

	if err != nil {
		tq.recordTaskFailure(task, "access", "浏览器提取失败: "+err.Error())
		tq.finalizeIfDone(task)
		return fmt.Errorf("websocket extract failed: %w", err)
	}

	if !resp.Success {
		tq.recordTaskFailure(task, "extract", "浏览器返回错误: "+resp.Error)
		tq.finalizeIfDone(task)
		return fmt.Errorf("browser returned error: %s", resp.Error)
	}

	// Extract markdown
	markdown := strings.TrimSpace(resp.Result.Content.MarkdownContent())
	if markdown == "" {
		tq.recordTaskFailure(task, "extract", "浏览器返回空正文")
		tq.finalizeIfDone(task)
		return fmt.Errorf("browser extracted empty markdown for %s", task.URL)
	}

	if task.ItemID > 0 {
		if utf8.RuneCountInString(markdown) < 200 {
			const reason = "浏览器提取正文不足 200 字"
			tq.recordTaskFailure(task, "extract", reason)
			tq.finalizeIfDone(task)
			return fmt.Errorf("%s", reason)
		}
		if err := tq.completeRSSBackfill(task, markdown, firstNonEmpty(resp.Result.Title, resp.Result.Content.Title)); err != nil {
			tq.recordTaskFailure(task, "persist", "回填正文失败: "+err.Error())
			tq.finalizeIfDone(task)
			return err
		}
		if tq.aiEngine != nil {
			tq.aiEngine.Enqueue(task.ItemID)
		}
		tq.finalizeIfDone(task)
		return nil
	}

	title := firstNonEmpty(resp.Result.Title, resp.Result.Content.Title, task.TitleHint)
	if title == "" {
		title = "Untitled scraped page"
	}

	classifier := NewClassifier()
	category := task.Category
	if category == "" {
		category = classifier.Classify(task.URL, "")
	}
	origin := classifier.ExtractOrigin(task.URL, resp.Result.Content.Site)

	// Create scraped item structure
	scrapedItem := item.ScrapedItem{
		SourceID:     task.SourceID,
		OriginSource: origin,
		Title:        title,
		URL:          task.URL,
		Summary:      truncateSummary(markdown, 240),
		Content:      markdown,
		Category:     category,
		PublishedAt:  task.PublishedAt,
		Metadata:     marshalMetadata(map[string]any{"content_source": "browser_page"}),
	}

	if resp.Result.Content.Author != "" {
		scrapedItem.Tags = resp.Result.Content.Author
	}

	id, created, err := tq.db.UpsertScrapedItem(scrapedItem)
	if err != nil {
		tq.recordTaskFailure(task, "persist", "写入内容库失败: "+err.Error())
		tq.finalizeIfDone(task)
		return fmt.Errorf("failed to save scraped item: %w", err)
	}

	tq.logger.Info("Successfully processed task", zap.String("url", task.URL), zap.Int64("item_id", id), zap.Bool("created", created))

	if task.LogItemID > 0 && id > 0 {
		status := "updated"
		if created {
			status = "stored"
		}
		// Replace the pending row with the final outcome.
		_ = tq.db.UpdateFetchLogItem(sqlite.FetchLogItem{
			ID: task.LogItemID, ItemID: id, Title: sanitizeText(title), URL: sanitizeURL(task.URL), Category: category,
			Status: status, Reason: "浏览器提取完成",
			Texts: []source.FetchTextResult{{Kind: "content", Status: status, Source: "browser_page",
				URL: sanitizeURL(task.URL), Chars: utf8.RuneCountInString(markdown)}},
		})
	}

	if id > 0 && tq.aiEngine != nil {
		tq.aiEngine.Enqueue(id)
	}

	// Legacy counters for non-detail callers.
	if task.LogID > 0 && task.LogItemID == 0 {
		var added = 0
		if created && id > 0 {
			added = 1
		}
		_ = tq.db.MarkFetchLogProgress(task.LogID, added)
	}
	tq.finalizeIfDone(task)
	return nil
}

func (tq *TaskQueue) completeRSSBackfill(task ScrapeTask, markdown, title string) error {
	entry, err := tq.db.GetScrapedItem(task.ItemID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("backfill item %d no longer exists", task.ItemID)
	}

	if rssNeedsBackfill(entry) {
		meta := parseMetadata(entry.Metadata)
		meta["content_source"] = "browser_page"
		meta["full_content_status"] = "fetched"
		meta["full_content_url"] = task.URL
		delete(meta, "full_content_error")
		delete(meta, "full_content_attempted_at")
		title = firstNonEmpty(entry.Title, title)
		if err := tq.db.UpdateScrapedItemFromScrape(entry.ID, title, "", markdown, "", marshalMetadata(meta), nil); err != nil {
			return err
		}
	} else {
		title, markdown = entry.Title, entry.Content
	}

	status := "updated"
	if task.NewItem {
		status = "stored"
	}
	tq.recordRSSResult(task, title, status, "浏览器补全正文成功", source.FetchTextResult{
		Kind: "content", Status: status, Source: "browser_page", URL: task.URL, Chars: utf8.RuneCountInString(markdown),
	})
	return nil
}

func (tq *TaskQueue) recordRSSResult(task ScrapeTask, title, status, reason string, result source.FetchTextResult) {
	if task.LogID <= 0 {
		return
	}
	rep := ItemReport{Title: task.TitleHint, URL: task.URL, Category: task.Category, ItemID: task.ItemID}
	if task.pending != nil {
		rep = task.pending.report
		rep.Texts = append([]source.FetchTextResult(nil), rep.Texts[:len(rep.Texts)-1]...)
	}
	rep.Title = firstNonEmpty(title, rep.Title)
	rep.Status, rep.Reason, rep.ErrorStage = status, reason, result.Stage
	rep.Texts = append(rep.Texts, result)
	sanitizeReport(&rep)
	row := toFetchLogItem(task.LogID, task.SourceID, rep, task.ItemID)
	row.ID = task.LogItemID
	if row.ID > 0 {
		_ = tq.db.UpdateFetchLogItem(row)
	} else {
		_, _ = tq.db.InsertFetchLogItem(row)
	}
}

func (tq *TaskQueue) recordTaskFailure(task ScrapeTask, stage, reason string) {
	if task.ItemID > 0 && task.pending != nil {
		if entry, err := tq.db.GetScrapedItem(task.ItemID); err == nil && entry != nil && rssNeedsBackfill(entry) {
			meta := parseMetadata(entry.Metadata)
			meta["full_content_status"] = "failed"
			meta["full_content_error"] = sanitizeText(reason)
			meta["full_content_url"] = task.URL
			meta["full_content_attempted_at"] = time.Now().UnixNano()
			_ = tq.db.UpdateScrapedItemFromScrape(entry.ID, "", "", "", "", marshalMetadata(meta), nil)
		}
		status := "failed"
		if task.NewItem {
			status = "stored"
			if tq.aiEngine != nil {
				tq.aiEngine.Enqueue(task.ItemID)
			}
		}
		tq.recordRSSResult(task, "", status, reason, source.FetchTextResult{
			Kind: "content", Status: "failed", Source: "feed_summary", URL: task.URL, Reason: reason, Stage: stage,
		})
		return
	}
	if task.LogID <= 0 {
		return
	}
	if task.LogItemID > 0 {
		_ = tq.db.UpdateFetchLogItem(sqlite.FetchLogItem{
			ID: task.LogItemID, ItemID: task.ItemID, Title: sanitizeText(task.TitleHint), URL: sanitizeURL(task.URL),
			Category: task.Category, Status: "failed", Reason: sanitizeText(reason), ErrorStage: stage,
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "browser_page",
				URL: sanitizeURL(task.URL), Reason: sanitizeText(reason), Stage: stage}},
		})
	} else {
		_, _ = tq.db.InsertFetchLogItem(sqlite.FetchLogItem{
			LogID: task.LogID, SourceID: task.SourceID, ItemID: task.ItemID, Title: sanitizeText(task.TitleHint),
			URL: sanitizeURL(task.URL), Category: task.Category, Status: "failed", Reason: sanitizeText(reason),
			ErrorStage: stage,
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "browser_page",
				URL: sanitizeURL(task.URL), Reason: sanitizeText(reason), Stage: stage}},
		})
	}
}

// finalizeIfDone writes the run's final status once no pending entries remain.
// Per-item failures/successes then roll up into partial/success/error.
func (tq *TaskQueue) finalizeIfDone(task ScrapeTask) {
	logID := task.LogID
	if logID <= 0 {
		return
	}
	tq.finalizeMu.Lock()
	defer tq.finalizeMu.Unlock()
	pending, err := tq.db.CountPendingFetchLogItems(logID)
	if err != nil || pending > 0 {
		return
	}
	override := ""
	if task.notModified {
		override = "not_modified"
	}
	log, err := tq.db.FinalizeFetchLog(logID, override, "", "")
	if err != nil {
		tq.logger.Warn("Failed to finalize async fetch log", zap.Int64("log_id", logID), zap.Error(err))
		return
	}
	if err := tq.db.UpdateSourceStatusFromLatestLog(log); err != nil {
		tq.logger.Warn("Failed to update source status", zap.Int64("log_id", logID), zap.Error(err))
	}
}
