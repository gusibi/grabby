package scraping

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

// newBenchmarkDB opens a file-backed SQLite database in t.TempDir so tests can
// close and reopen it to prove fetch-log durability.
func newBenchmarkDB(t *testing.T) (*sqlite.Database, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "benchmark.db")
	db, err := sqlite.NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase(%s): %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

// newFastScraper keeps retries immediate so a deterministic fixture failure
// does not add the production 5s/15s backoff waits to the test suite.
func newFastScraper(db *sqlite.Database) *Scraper {
	s := NewScraper(db, nil, nil, zap.NewNop(), nil)
	s.maxRetries = 1
	s.retryBackoff = func(int) time.Duration { return 0 }
	return s
}

func insertTestSource(t *testing.T, db *sqlite.Database, src source.Source) {
	t.Helper()
	if src.Schedule == "" {
		src.Schedule = "0 * * * *"
	}
	if src.Enabled == 0 {
		src.Enabled = 1
	}
	if err := db.InsertSource(src); err != nil {
		t.Fatalf("InsertSource(%s): %v", src.ID, err)
	}
}

// scrapeFresh reloads the source from the database (as the HTTP handler and
// scheduler do) so ETag/Last-Modified state from the previous run is used.
func scrapeFresh(t *testing.T, db *sqlite.Database, s *Scraper, sourceID, trigger string) (int, error) {
	t.Helper()
	src, err := db.GetSource(sourceID)
	if err != nil || src == nil {
		t.Fatalf("GetSource(%s): %v", sourceID, err)
	}
	return s.ScrapeSourceWithTrigger(context.Background(), *src, trigger)
}

func latestFetchLog(t *testing.T, db *sqlite.Database, sourceID string) source.FetchLog {
	t.Helper()
	logs, err := db.GetFetchLogs(sourceID, 1)
	if err != nil || len(logs) == 0 {
		t.Fatalf("GetFetchLogs(%s): len=%d err=%v", sourceID, len(logs), err)
	}
	return logs[0]
}

func fetchLogItems(t *testing.T, db *sqlite.Database, logID int64) []source.FetchLogItem {
	t.Helper()
	items, err := db.GetFetchLogItems(logID)
	if err != nil {
		t.Fatalf("GetFetchLogItems(%d): %v", logID, err)
	}
	return items
}

func findLogItem(items []source.FetchLogItem, urlPart string) *source.FetchLogItem {
	for i := range items {
		if strings.Contains(items[i].URL, urlPart) {
			return &items[i]
		}
	}
	return nil
}

func findText(it *source.FetchLogItem, kind string) *source.FetchTextResult {
	if it == nil {
		return nil
	}
	for i := range it.Texts {
		if it.Texts[i].Kind == kind {
			return &it.Texts[i]
		}
	}
	return nil
}

func requireTextStatus(t *testing.T, it *source.FetchLogItem, kind, status string) *source.FetchTextResult {
	t.Helper()
	txt := findText(it, kind)
	if txt == nil {
		t.Fatalf("missing %s text result in %+v", kind, it)
	}
	if txt.Status != status {
		t.Fatalf("%s status = %q, want %q (reason=%q stage=%q)", kind, txt.Status, status, txt.Reason, txt.Stage)
	}
	return txt
}

func itemsBySource(t *testing.T, db *sqlite.Database, sourceID string) []sqlite.ScrapedItem {
	t.Helper()
	items, err := db.ListSourceItems(sourceID)
	if err != nil {
		t.Fatalf("ListSourceItems(%s): %v", sourceID, err)
	}
	return items
}

func mustItemByURL(t *testing.T, db *sqlite.Database, url string) *sqlite.ScrapedItem {
	t.Helper()
	it, err := db.GetScrapedItemFullByURL(url)
	if err != nil {
		t.Fatalf("GetScrapedItemFullByURL(%s): %v", url, err)
	}
	if it == nil {
		t.Fatalf("item %s not found", url)
	}
	return it
}

func rssFeed(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>Fixture Feed</title><link>https://example.com/</link>` + items + `</channel></rss>`
}

func rssItem(title, link, description, content string) string {
	item := `<item><title>` + title + `</title><link>` + link + `</link><description><![CDATA[` + description + `]]></description>`
	if content != "" {
		item += `<content:encoded><![CDATA[` + content + `]]></content:encoded>`
	}
	return item + `</item>`
}

func articleHTML(title, body string) string {
	return `<!doctype html><html><head><meta property="og:title" content="` + title + `">
<meta property="article:published_time" content="2026-11-18T09:30:00Z"></head>
<body><nav><a href="/">Home</a></nav><article><h1>` + title + `</h1><p>` + body + `</p></article>
<footer>fixture footer</footer></body></html>`
}

func longText(prefix string, n int) string {
	return fmt.Sprintf("%s %s", prefix, strings.Repeat("Detailed technical content for acceptance testing. ", n))
}
