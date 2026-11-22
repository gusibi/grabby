package scraping

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/domain/source"
)

type fakeBrowserExecutor struct {
	title   string
	content string
	err     error
	release <-chan struct{}
}

func (f *fakeBrowserExecutor) ResolveBrowserConnID(string) (string, error) {
	return "fake-conn", nil
}

func (f *fakeBrowserExecutor) SendMessage(ctx context.Context, req *capture.BrowserRequest, connID string) (*capture.BrowserResponse, error) {
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return &capture.BrowserResponse{
		Success: true,
		Result: capture.PageResult{
			URL:   req.URL,
			Title: f.title,
			Content: capture.PageContent{
				Title:   f.title,
				Content: f.content,
			},
		},
	}, nil
}

func TestAcceptanceAsyncLogMovesFromPendingToFinal(t *testing.T) {
	db, _ := newBenchmarkDB(t)
	insertTestSource(t, db, source.Source{ID: "async-src", Name: "Async", Type: "web_scrape",
		URL: "https://mp.weixin.qq.com/s/async", Schedule: "0 * * * *", Enabled: 1, Config: `{}`})

	logID, err := db.InsertFetchLog(source.FetchLog{SourceID: "async-src", StartedAt: time.Now().UTC(),
		Status: "running", Trigger: "manual"})
	if err != nil {
		t.Fatalf("InsertFetchLog: %v", err)
	}
	rowID, err := db.InsertFetchLogItem(source.FetchLogItem{
		LogID: logID, SourceID: "async-src", URL: "https://mp.weixin.qq.com/s/async",
		Status: "pending", Reason: "async browser task queued",
		Texts: []source.FetchTextResult{{Kind: "content", Status: "pending", Source: "browser_page"}},
	})
	if err != nil {
		t.Fatalf("InsertFetchLogItem: %v", err)
	}

	// While the browser task is queued the run is visibly "running".
	log, _ := db.GetFetchLog(logID)
	if log == nil || log.Status != "running" || log.FinishedAt != nil {
		t.Fatalf("pending log = %+v", log)
	}
	rows, _ := db.GetFetchLogItems(logID)
	if len(rows) != 1 || rows[0].Status != "pending" {
		t.Fatalf("pending detail = %+v", rows)
	}

	tq := NewTaskQueueWithExecutor(&fakeBrowserExecutor{title: "Async Article",
		content: longText("ASYNC BROWSER BODY", 12)}, db, zap.NewNop(), 1, nil)
	tq.Start(context.Background())
	defer tq.Shutdown()
	tq.Enqueue([]ScrapeTask{{
		SourceID: "async-src", URL: "https://mp.weixin.qq.com/s/async",
		LogID: logID, LogItemID: rowID, Category: "article", TitleHint: "Async Article",
	}})

	deadline := time.Now().Add(5 * time.Second)
	var final *source.FetchLog
	for time.Now().Before(deadline) {
		final, _ = db.GetFetchLog(logID)
		if final != nil && final.Status != "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if final == nil || final.Status != "success" {
		t.Fatalf("final log = %+v, want success", final)
	}
	if final.FinishedAt == nil || final.ItemsAdded != 1 {
		t.Fatalf("final counters = %+v", final)
	}
	rows, _ = db.GetFetchLogItems(logID)
	if len(rows) != 1 || rows[0].Status != "stored" || rows[0].ItemID == 0 {
		t.Fatalf("final detail = %+v", rows)
	}
	it, err := db.GetScrapedItem(rows[0].ItemID)
	if err != nil || it == nil || it.Title != "Async Article" || it.Content == "" {
		t.Fatalf("async item = %+v err=%v", it, err)
	}
}

func TestAcceptanceAsyncFailureIsFinalAndNotStored(t *testing.T) {
	db, _ := newBenchmarkDB(t)
	insertTestSource(t, db, source.Source{ID: "async-fail", Name: "Async Fail", Type: "web_scrape",
		URL: "https://mp.weixin.qq.com/s/fail", Schedule: "0 * * * *", Enabled: 1, Config: `{}`})
	logID, _ := db.InsertFetchLog(source.FetchLog{SourceID: "async-fail", StartedAt: time.Now().UTC(),
		Status: "running", Trigger: "manual"})
	rowID, _ := db.InsertFetchLogItem(source.FetchLogItem{
		LogID: logID, SourceID: "async-fail", URL: "https://mp.weixin.qq.com/s/fail", Status: "pending",
	})

	tq := NewTaskQueueWithExecutor(&fakeBrowserExecutor{err: errors.New("extension disconnected")},
		db, zap.NewNop(), 1, nil)
	tq.Start(context.Background())
	defer tq.Shutdown()
	tq.Enqueue([]ScrapeTask{{SourceID: "async-fail", URL: "https://mp.weixin.qq.com/s/fail",
		LogID: logID, LogItemID: rowID, Category: "article"}})

	deadline := time.Now().Add(5 * time.Second)
	var final *source.FetchLog
	for time.Now().Before(deadline) {
		final, _ = db.GetFetchLog(logID)
		if final != nil && final.Status != "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if final == nil || final.Status != "error" {
		t.Fatalf("final failure log = %+v", final)
	}
	rows, _ := db.GetFetchLogItems(logID)
	if len(rows) != 1 || rows[0].Status != "failed" || rows[0].ErrorStage != "access" {
		t.Fatalf("failed detail = %+v", rows)
	}
	if rows[0].ItemID != 0 {
		t.Fatalf("failed task must not point at a stored item: %+v", rows[0])
	}
	items := itemsBySource(t, db, "async-fail")
	if len(items) != 0 {
		t.Fatalf("failed task stored an item: %+v", items)
	}
}

func TestAcceptanceRSSBrowserBackfillFinishesPendingRun(t *testing.T) {
	const articleURL = "https://mp.weixin.qq.com/s/rss-async"
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(rssFeed(rssItem("An article", articleURL, "Original summary", ""))))
	}))
	t.Cleanup(feed.Close)

	db, _ := newBenchmarkDB(t)
	src := source.Source{ID: "rss-async", Name: "RSS async", Type: "rss", URL: feed.URL, Config: `{"full_content":true}`}
	insertTestSource(t, db, src)
	release := make(chan struct{})
	scraper := newFastScraper(db)
	scraper.taskQueue = NewTaskQueueWithExecutor(&fakeBrowserExecutor{
		content: longText("RSS BROWSER BODY", 12), release: release,
	}, db, zap.NewNop(), 1, nil)
	scraper.taskQueue.Start(context.Background())
	t.Cleanup(scraper.taskQueue.Shutdown)

	count, err := scrapeFresh(t, db, scraper, src.ID, "manual")
	if err != nil || count != 1 {
		t.Fatalf("scrape returned %d, %v", count, err)
	}
	log := latestFetchLog(t, db, src.ID)
	rows := fetchLogItems(t, db, log.ID)
	if log.Status != "running" || len(rows) != 1 || rows[0].Status != "pending" {
		t.Fatalf("expected one pending row, log=%+v rows=%+v", log, rows)
	}
	itemID := rows[0].ItemID
	if err := db.MarkItemRead(itemID, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.ToggleItemStarred(itemID, 1); err != nil {
		t.Fatal(err)
	}
	close(release)
	for deadline := time.Now().Add(5 * time.Second); log.Status == "running" && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		log = latestFetchLog(t, db, src.ID)
	}
	if log.Status != "success" || log.ItemsAdded != 1 || log.ItemsUpdated != 0 {
		t.Fatalf("unexpected final log: %+v", log)
	}
	rows = fetchLogItems(t, db, log.ID)
	if len(rows) != 1 || rows[0].ItemID != itemID {
		t.Fatalf("queued result should update the original detail row: %+v", rows)
	}
	requireTextStatus(t, &rows[0], "content", "stored")
	entry := mustItemByURL(t, db, articleURL)
	if entry.ID != itemID || entry.ReadStatus != 1 || entry.Starred != 1 || entry.Summary != "Original summary" ||
		!strings.Contains(entry.Content, "RSS BROWSER BODY") || rssNeedsBackfill(entry) {
		t.Fatalf("unexpected backfilled item: %+v", entry)
	}
}
