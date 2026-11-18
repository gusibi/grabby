package sqlite

import (
	"testing"
	"time"
)

func TestCleanupOldDataRetainsProtectedData(t *testing.T) {
	db := newTestDB(t)

	src := Source{
		ID:              "cleanup-src",
		Name:            "Cleanup Source",
		Type:            "rss",
		URL:             "https://example.com/feed",
		Schedule:        "0 * * * *",
		Enabled:         1,
		DefaultCategory: "article",
		Config:          "{}",
	}
	if err := db.InsertSource(src); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	insertItem := func(title, url string) int64 {
		t.Helper()
		id, err := db.InsertScrapedItem(ScrapedItem{
			SourceID: src.ID,
			Title:    title,
			URL:      url,
			Category: "article",
		})
		if err != nil {
			t.Fatalf("InsertScrapedItem(%s): %v", title, err)
		}
		if id == 0 {
			t.Fatalf("InsertScrapedItem(%s) returned id 0", title)
		}
		return id
	}
	setFetchedAt := func(id int64, at time.Time) {
		t.Helper()
		if _, err := db.db.Exec(
			"UPDATE scraped_items SET fetched_at = ? WHERE id = ?",
			at.UTC().Format("2006-01-02 15:04:05"), id,
		); err != nil {
			t.Fatalf("set fetched_at for %d: %v", id, err)
		}
	}
	insertAnalysis := func(itemID int64) {
		t.Helper()
		if err := db.InsertAIAnalysis(AIAnalysis{
			ItemID:       itemID,
			AICategory:   "tech",
			QualityScore: 8,
		}); err != nil {
			t.Fatalf("InsertAIAnalysis(%d): %v", itemID, err)
		}
	}

	oldAt := time.Now().AddDate(0, 0, -31)

	staleID := insertItem("stale item", "https://example.com/stale")
	setFetchedAt(staleID, oldAt)
	insertAnalysis(staleID)

	oldStarredID := insertItem("old starred item", "https://example.com/starred")
	setFetchedAt(oldStarredID, oldAt)
	if err := db.ToggleItemStarred(oldStarredID, 1); err != nil {
		t.Fatalf("ToggleItemStarred: %v", err)
	}
	insertAnalysis(oldStarredID)

	recentID := insertItem("recent item", "https://example.com/recent")
	insertAnalysis(recentID)

	// Data that must survive the cleanup.
	if err := db.InsertAIDailyReport(AIDailyReport{
		ReportDate: "2026-01-01",
		ReportType: "daily",
		Title:      "Report",
		Content:    "content",
	}); err != nil {
		t.Fatalf("InsertAIDailyReport: %v", err)
	}
	cacheURL := "https://example.com/page"
	if err := db.SaveExtractCache(cacheURL, "Page", "# Page"); err != nil {
		t.Fatalf("SaveExtractCache: %v", err)
	}

	oldLogID, err := db.InsertFetchLog(FetchLog{
		SourceID:  src.ID,
		StartedAt: oldAt,
		Status:    "success",
	})
	if err != nil {
		t.Fatalf("InsertFetchLog old: %v", err)
	}
	_ = oldLogID
	recentLogID, err := db.InsertFetchLog(FetchLog{
		SourceID:  src.ID,
		StartedAt: time.Now(),
		Status:    "success",
	})
	if err != nil {
		t.Fatalf("InsertFetchLog recent: %v", err)
	}

	if err := db.CleanupOldData(); err != nil {
		t.Fatalf("CleanupOldData: %v", err)
	}

	// Stale, unstarred raw data and its AI score are gone.
	if item, err := db.GetScrapedItem(staleID); err != nil || item != nil {
		t.Fatalf("stale item should be deleted, got item=%v err=%v", item, err)
	}
	if analysis, err := db.GetAIAnalysis(staleID); err != nil || analysis != nil {
		t.Fatalf("stale item analysis should be deleted, got analysis=%v err=%v", analysis, err)
	}

	// Starred old data and its score are kept.
	if item, err := db.GetScrapedItem(oldStarredID); err != nil || item == nil {
		t.Fatalf("starred item should be kept, got item=%v err=%v", item, err)
	}
	if analysis, err := db.GetAIAnalysis(oldStarredID); err != nil || analysis == nil {
		t.Fatalf("starred item analysis should be kept, got analysis=%v err=%v", analysis, err)
	}

	// Recent raw data is untouched.
	if item, err := db.GetScrapedItem(recentID); err != nil || item == nil {
		t.Fatalf("recent item should be kept, got item=%v err=%v", item, err)
	}
	if analysis, err := db.GetAIAnalysis(recentID); err != nil || analysis == nil {
		t.Fatalf("recent item analysis should be kept, got analysis=%v err=%v", analysis, err)
	}

	// AI daily reports and webpage extract captures are not raw data.
	if report, err := db.GetAIDailyReport("2026-01-01", "daily"); err != nil || report == nil {
		t.Fatalf("AI daily report should be kept, got report=%v err=%v", report, err)
	}
	if cached, err := db.GetExtractCache(cacheURL); err != nil || cached == nil {
		t.Fatalf("extract cache should be kept, got cached=%v err=%v", cached, err)
	}

	logs, err := db.GetFetchLogs(src.ID, 10)
	if err != nil {
		t.Fatalf("GetFetchLogs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != recentLogID {
		t.Fatalf("expected only the recent fetch log to remain, got %+v", logs)
	}
}

func TestContentTypeFilterAndMediaBackfill(t *testing.T) {
	db := newTestDB(t)

	insertSource := func(id, typ string) {
		t.Helper()
		if err := db.InsertSource(Source{
			ID: id, Name: id, Type: typ, URL: "https://example.com/" + id,
			Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}",
		}); err != nil {
			t.Fatalf("InsertSource(%s): %v", id, err)
		}
	}
	insertItem := func(sourceID, url, category, metadata string) int64 {
		t.Helper()
		id, err := db.InsertScrapedItem(ScrapedItem{
			SourceID: sourceID, Title: url, URL: url, Category: category, Metadata: metadata,
		})
		if err != nil {
			t.Fatalf("InsertScrapedItem(%s): %v", url, err)
		}
		return id
	}

	insertSource("rss-src", "rss")
	insertSource("pod-src", "podcast")

	insertItem("rss-src", "https://example.com/article", "article", "{}")
	insertItem("rss-src", "https://example.com/episode", "podcast", `{"media_url":"https://cdn.example.com/a.mp3"}`)
	insertItem("rss-src", "https://example.com/video", "youtube", "{}")
	// Legacy row from a source that is already typed as a podcast.
	insertItem("pod-src", "https://example.com/legacy", "article", "{}")

	count := func(typ string) int {
		t.Helper()
		items, _, err := db.GetScrapedItems(ItemsFilter{ContentType: typ, Limit: 50})
		if err != nil {
			t.Fatalf("GetScrapedItems(%s): %v", typ, err)
		}
		return len(items)
	}

	if got := count("rss"); got != 1 {
		t.Fatalf("rss filter = %d, want 1", got)
	}
	if got := count("podcast"); got != 2 {
		t.Fatalf("podcast filter = %d, want 2", got)
	}
	if got := count("youtube"); got != 1 {
		t.Fatalf("youtube filter = %d, want 1", got)
	}

	// The one-time media backfill upgrades an existing article row in place.
	if err := db.UpdateSourceItemMedia("rss-src", "https://example.com/article", "podcast", `{"media_url":"https://cdn.example.com/backfill.mp3"}`); err != nil {
		t.Fatalf("UpdateSourceItemMedia: %v", err)
	}
	if got := count("podcast"); got != 3 {
		t.Fatalf("podcast filter after backfill = %d, want 3", got)
	}
	if got := count("rss"); got != 0 {
		t.Fatalf("rss filter after backfill = %d, want 0", got)
	}
}

func TestSetSourceItemsCategory(t *testing.T) {
	db := newTestDB(t)
	if err := db.InsertSource(Source{
		ID: "src", Name: "src", Type: "rss", URL: "https://example.com/feed",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}",
	}); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}
	for _, u := range []string{"https://example.com/1", "https://example.com/2"} {
		if _, err := db.InsertScrapedItem(ScrapedItem{
			SourceID: "src", Title: u, URL: u, Category: "article", Metadata: "{}",
		}); err != nil {
			t.Fatalf("InsertScrapedItem(%s): %v", u, err)
		}
	}

	// Switching the source to podcast must reclassify existing items so the
	// feed filter reflects the change without waiting for a re-scrape.
	if err := db.SetSourceItemsCategory("src", "podcast"); err != nil {
		t.Fatalf("SetSourceItemsCategory: %v", err)
	}
	items, _, err := db.GetScrapedItems(ItemsFilter{SourceID: "src", Limit: 50})
	if err != nil {
		t.Fatalf("GetScrapedItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, it := range items {
		if it.Category != "podcast" {
			t.Fatalf("category = %q, want podcast", it.Category)
		}
	}

	// "auto" is not a concrete category and must never rewrite stored items.
	if err := db.SetSourceItemsCategory("src", "auto"); err != nil {
		t.Fatalf("SetSourceItemsCategory(auto): %v", err)
	}
	items, _, _ = db.GetScrapedItems(ItemsFilter{SourceID: "src", Limit: 50})
	if len(items) != 2 || items[0].Category != "podcast" {
		t.Fatalf("auto category should be a no-op")
	}
}
