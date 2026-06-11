package scraping

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
)

type ScrapeTask struct {
	SourceID string
	URL      string
	LogID    int64 // logs task details to fetch_logs
}

type TaskQueue struct {
	mu          sync.Mutex
	concurrency int // Number of worker goroutines
	wsManager   *browserws.WebSocketManager
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
	if concurrency <= 0 {
		concurrency = 1
	}
	return &TaskQueue{
		concurrency: concurrency,
		wsManager:   wsManager,
		db:          db,
		logger:      logger,
		taskChan:    make(chan ScrapeTask, 1000),
		aiEngine:    aiEngine,
	}
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
	tq.mu.Lock()
	defer tq.mu.Unlock()
	if !tq.running {
		tq.logger.Warn("TaskQueue is not running. Dropping tasks", zap.Int("count", len(tasks)))
		return
	}
	for _, t := range tasks {
		select {
		case tq.taskChan <- t:
		default:
			tq.logger.Error("Task queue buffer full, dropping task", zap.String("url", t.URL))
		}
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
		// Update fetch log status to indicate skipped/browser unavailable
		if task.LogID > 0 {
			tq.db.MarkFetchLogSkipped(task.LogID, "No active browser connected")
		}
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
	}, browserConnID)

	if err != nil {
		return fmt.Errorf("websocket extract failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("browser returned error: %s", resp.Error)
	}

	// Extract markdown
	markdown := resp.Result.Content.MarkdownContent()
	title := firstNonEmpty(resp.Result.Title, resp.Result.Content.Title)
	if title == "" {
		title = "Untitled scraped page"
	}

	classifier := NewClassifier()
	category := classifier.Classify(task.URL, "")
	origin := classifier.ExtractOrigin(task.URL, resp.Result.Content.Site)

	// Create scraped item structure
	scrapedItem := item.ScrapedItem{
		SourceID:     task.SourceID,
		OriginSource: origin,
		Title:        title,
		URL:          task.URL,
		Summary:      resp.Result.Content.Site,
		Content:      markdown,
		Category:     category,
	}

	if resp.Result.Content.Author != "" {
		scrapedItem.Tags = resp.Result.Content.Author
	}

	id, err := tq.db.InsertScrapedItem(scrapedItem)
	if err != nil {
		return fmt.Errorf("failed to save scraped item: %w", err)
	}

	tq.logger.Info("Successfully processed task", zap.String("url", task.URL), zap.Int64("inserted_id", id))

	var added int64 = 0
	if id > 0 {
		added = 1
		tq.aiEngine.Enqueue(id)
	}

	// Update fetch log stats
	if task.LogID > 0 {
		tq.db.MarkFetchLogProgress(task.LogID, int(added))
	}

	return nil
}
