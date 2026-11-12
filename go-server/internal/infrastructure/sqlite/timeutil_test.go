package sqlite

import (
	"strings"
	"testing"
	"time"
)

func newUTCTestSource(t *testing.T, db *Database, id string) Source {
	t.Helper()
	src := Source{
		ID:              id,
		Name:            "UTC Source",
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
	return src
}

func TestGoWrittenTimestampsAreStoredAsUTC(t *testing.T) {
	db := newTestDB(t)
	src := newUTCTestSource(t, db, "utc-src")

	cst := time.FixedZone("UTC+8", 8*3600)
	started := time.Date(2026, 11, 11, 22, 0, 0, 0, cst) // 14:00 UTC
	finished := started.Add(time.Minute)

	logID, err := db.InsertFetchLog(FetchLog{
		SourceID:   src.ID,
		StartedAt:  started,
		FinishedAt: &finished,
		Status:     "success",
	})
	if err != nil {
		t.Fatalf("InsertFetchLog: %v", err)
	}

	var storedStart, storedFinish string
	if err := db.db.QueryRow(
		"SELECT CAST(started_at AS TEXT), CAST(finished_at AS TEXT) FROM fetch_logs WHERE id = ?", logID,
	).Scan(&storedStart, &storedFinish); err != nil {
		t.Fatalf("read fetch log: %v", err)
	}
	if storedStart != "2026-11-11 14:00:00" {
		t.Errorf("started_at stored as %q, want UTC 2026-11-11 14:00:00", storedStart)
	}
	if storedFinish != "2026-11-11 14:01:00" {
		t.Errorf("finished_at stored as %q, want UTC 2026-11-11 14:01:00", storedFinish)
	}

	logs, err := db.GetFetchLogs(src.ID, 1)
	if err != nil || len(logs) != 1 {
		t.Fatalf("GetFetchLogs: %v (len=%d)", err, len(logs))
	}
	if !logs[0].StartedAt.Equal(started) {
		t.Errorf("StartedAt read back as %v, want %v", logs[0].StartedAt, started.UTC())
	}

	published := time.Date(2026, 11, 11, 23, 30, 0, 0, cst) // 15:30 UTC
	if _, err := db.InsertScrapedItem(ScrapedItem{
		SourceID:    src.ID,
		Title:       "utc item",
		URL:         "https://example.com/utc-item",
		Category:    "article",
		PublishedAt: &published,
	}); err != nil {
		t.Fatalf("InsertScrapedItem: %v", err)
	}
	var storedPublished string
	if err := db.db.QueryRow(
		"SELECT CAST(published_at AS TEXT) FROM scraped_items WHERE url = ?", "https://example.com/utc-item",
	).Scan(&storedPublished); err != nil {
		t.Fatalf("read published_at: %v", err)
	}
	if storedPublished != "2026-11-11 15:30:00" {
		t.Errorf("published_at stored as %q, want UTC 2026-11-11 15:30:00", storedPublished)
	}

	if err := db.SaveTweets([]TweetRecord{{
		ID:        "tweet-1",
		Text:      "hello",
		Source:    "search",
		FetchedAt: started,
	}}); err != nil {
		t.Fatalf("SaveTweets: %v", err)
	}
	var storedFetchedAt string
	if err := db.db.QueryRow("SELECT CAST(fetched_at AS TEXT) FROM tweets WHERE id = ?", "tweet-1").Scan(&storedFetchedAt); err != nil {
		t.Fatalf("read tweet fetched_at: %v", err)
	}
	if !strings.HasPrefix(storedFetchedAt, "2026-11-11 14:00:00") || strings.Contains(storedFetchedAt, "+08") {
		t.Errorf("tweet fetched_at stored as %q, want UTC", storedFetchedAt)
	}
}

func TestNormalizeTimestampsToUTC(t *testing.T) {
	db := newTestDB(t)
	src := newUTCTestSource(t, db, "legacy-src")

	if _, err := db.db.Exec(`
		INSERT INTO fetch_logs (source_id, started_at, finished_at, status)
		VALUES (?, '2026-11-11 22:00:00.00975+08:00', '2026-11-11 22:01:00.5+08:00', 'success')
	`, src.ID); err != nil {
		t.Fatalf("insert legacy fetch log: %v", err)
	}
	if _, err := db.db.Exec(`
		INSERT INTO scraped_items (source_id, title, url, category, published_at)
		VALUES (?, 'legacy item', 'https://example.com/legacy', 'article', '2026-11-03 20:00:03.041282+08:00')
	`, src.ID); err != nil {
		t.Fatalf("insert legacy item: %v", err)
	}

	tx, err := db.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := normalizeTimestampsToUTC(tx); err != nil {
		tx.Rollback()
		t.Fatalf("normalizeTimestampsToUTC: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var startedAt, publishedAt string
	if err := db.db.QueryRow("SELECT CAST(started_at AS TEXT) FROM fetch_logs WHERE source_id = ?", src.ID).Scan(&startedAt); err != nil {
		t.Fatalf("read started_at: %v", err)
	}
	if startedAt != "2026-11-11 14:00:00" {
		t.Errorf("started_at normalised to %q, want 2026-11-11 14:00:00", startedAt)
	}
	if err := db.db.QueryRow("SELECT CAST(published_at AS TEXT) FROM scraped_items WHERE source_id = ?", src.ID).Scan(&publishedAt); err != nil {
		t.Fatalf("read published_at: %v", err)
	}
	if publishedAt != "2026-11-03 12:00:03" {
		t.Errorf("published_at normalised to %q, want 2026-11-03 12:00:03", publishedAt)
	}
}
