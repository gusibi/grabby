package scraping

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

func TestAcceptanceRSSFullContentRetentionAndBackfill(t *testing.T) {
	longFeed := longText("FEED BODY", 14)
	shortBody := longText("EXTRACTED ARTICLE BODY", 12)

	var articleRequests sync.Map
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feeds/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(rssFeed(
				rssItem("Full Feed Item", srv.URL+"/articles/full", "Feed summary one", `<p>`+longFeed+`</p>`) +
					rssItem("Summary Only Item", "../articles/short", "<p>Only a short feed summary.</p>", ""),
			)))
		case "/articles/short":
			articleRequests.Store("/articles/short", true)
			http.Redirect(w, r, "/articles/short-final", http.StatusFound)
		case "/articles/short-final":
			articleRequests.Store("/articles/short-final", true)
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(articleHTML("Linked Article", shortBody)))
		case "/articles/full":
			articleRequests.Store("/articles/full", true)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{
		ID: "rss-full", Name: "RSS Full", Type: "rss",
		URL: srv.URL + "/feeds/feed.xml", Schedule: "0 * * * *", Enabled: 1,
		DefaultCategory: "article", Config: `{"full_content":true}`,
	}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}

	items := itemsBySource(t, db, src.ID)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	var full, short *sqlite.ScrapedItem
	for i := range items {
		switch {
		case strings.Contains(items[i].URL, "/articles/full"):
			full = &items[i]
		case strings.Contains(items[i].URL, "/articles/short"):
			short = &items[i]
		}
	}
	if full == nil || short == nil {
		t.Fatalf("missing expected items: %+v", items)
	}
	if !strings.Contains(full.Content, "FEED BODY") {
		t.Fatalf("feed-provided body was not preserved: %q", full.Content)
	}
	if _, ok := articleRequests.Load("/articles/full"); ok {
		t.Fatalf("feed-provided full body must not trigger a second article request")
	}
	if !strings.Contains(short.Content, "EXTRACTED ARTICLE BODY") {
		t.Fatalf("summary-only item was not backfilled from the linked page: %q", short.Content)
	}
	if !strings.Contains(short.Summary, "Only a short feed summary") {
		t.Fatalf("original feed summary was not preserved: %q", short.Summary)
	}
	if !strings.Contains(short.URL, "/articles/short") || strings.Contains(short.URL, "short-final") {
		t.Fatalf("source link should stay the feed link (redirect followed for extraction), got %q", short.URL)
	}
	if _, ok := articleRequests.Load("/articles/short-final"); !ok {
		t.Fatalf("redirect target was not fetched")
	}
	if !strings.Contains(short.Metadata, `"content_source":"web_page"`) {
		t.Fatalf("metadata lacks content_source=web_page: %s", short.Metadata)
	}

	log := latestFetchLog(t, db, src.ID)
	if log.Trigger != "manual" || log.Status != "success" {
		t.Fatalf("log = %+v, want manual/success", log)
	}
	entries := fetchLogItems(t, db, log.ID)
	fullEntry := findLogItem(entries, "/articles/full")
	shortEntry := findLogItem(entries, "/articles/short")
	requireTextStatus(t, fullEntry, "content", "stored")
	if fullEntry.Texts[0].Source == "web_page" {
		t.Fatalf("feed body should be labelled as feed, got %+v", fullEntry.Texts)
	}
	shortContent := requireTextStatus(t, shortEntry, "content", "stored")
	if shortContent.Source != "web_page" || shortContent.Chars < 200 {
		t.Fatalf("short item content result = %+v", shortContent)
	}
	if shortEntry.ItemID == 0 || shortEntry.ItemID != short.ID {
		t.Fatalf("log item id = %d, want %d", shortEntry.ItemID, short.ID)
	}
}

func TestAcceptanceRSSFullContentDisabledDoesNotRequestArticle(t *testing.T) {
	var articleHits int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			_, _ = w.Write([]byte(rssFeed(rssItem("Disabled", srv.URL+"/article", "<p>Summary only.</p>", ""))))
		case "/article":
			atomic.AddInt32(&articleHits, 1)
			_, _ = w.Write([]byte(articleHTML("Article", longText("BODY", 10))))
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "rss-off", Name: "RSS Off", Type: "rss", URL: srv.URL + "/feed.xml",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Config: `{"full_content":false}`}
	insertTestSource(t, db, src)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if got := atomic.LoadInt32(&articleHits); got != 0 {
		t.Fatalf("article fetched %d times with full_content disabled", got)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 1 || !strings.Contains(items[0].Content, "Summary only") {
		t.Fatalf("expected feed summary content, got %+v", items)
	}
	log := latestFetchLog(t, db, src.ID)
	entry := findLogItem(fetchLogItems(t, db, log.ID), "/article")
	txt := requireTextStatus(t, entry, "content", "stored")
	if txt.Source != "feed_summary" || !strings.Contains(txt.Reason, "full_content_disabled") {
		t.Fatalf("disabled full content should be explicit in the log: %+v", txt)
	}
}

func TestAcceptanceRSSPDFLinks(t *testing.T) {
	// Long enough to be accepted as article text, unlike the tiny fixture
	// used by the extraction unit test.
	longPDF := buildTestPDF("BT /F1 12 Tf 72 720 Td (" + longText("PDF ARTICLE BODY", 16) + ") Tj ET")
	scanPDF := buildTestPDF("")
	var pdfHits atomic.Int32

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			_, _ = w.Write([]byte(rssFeed(
				rssItem("Annual Report", srv.URL+"/paper.pdf", "<p>Read the report.</p>", "") +
					rssItem("Scanned Memo", srv.URL+"/scan.pdf", "<p>A scanned memo.</p>", ""),
			)))
		case "/paper.pdf":
			pdfHits.Add(1)
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(longPDF)
		case "/scan.pdf":
			pdfHits.Add(1)
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(scanPDF)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "rss-pdf", Name: "RSS PDF", Type: "rss", URL: srv.URL + "/feed.xml",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Config: `{"full_content":true}`}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(items), items)
	}
	var report, scan *sqlite.ScrapedItem
	for i := range items {
		switch {
		case strings.Contains(items[i].URL, "/paper.pdf"):
			report = &items[i]
		case strings.Contains(items[i].URL, "/scan.pdf"):
			scan = &items[i]
		}
	}
	if report == nil || scan == nil {
		t.Fatalf("missing PDF items: %+v", items)
	}
	if !strings.Contains(report.Content, "PDF ARTICLE BODY") {
		t.Fatalf("PDF text was not stored as content: %q", report.Content)
	}
	for _, want := range []string{`"content_source":"pdf_text"`, `"pdf_url":"` + srv.URL + `/paper.pdf"`, `"pdf_pages":1`} {
		if !strings.Contains(report.Metadata, want) {
			t.Fatalf("report metadata misses %s: %s", want, report.Metadata)
		}
	}
	if !strings.Contains(scan.Content, "A scanned memo") {
		t.Fatalf("scan should keep the feed summary: %q", scan.Content)
	}
	if !strings.Contains(scan.Metadata, `"content_source":"pdf"`) || !strings.Contains(scan.Metadata, `"pdf_url"`) {
		t.Fatalf("scan metadata should mark a text-less PDF: %s", scan.Metadata)
	}
	if got := pdfHits.Load(); got != 2 {
		t.Fatalf("PDF downloads = %d, want 2", got)
	}

	log := latestFetchLog(t, db, src.ID)
	entries := fetchLogItems(t, db, log.ID)
	reportText := requireTextStatus(t, findLogItem(entries, "/paper.pdf"), "content", "stored")
	if reportText.Source != "pdf_text" {
		t.Fatalf("report content source = %q, want pdf_text", reportText.Source)
	}
	scanText := requireTextStatus(t, findLogItem(entries, "/scan.pdf"), "content", "stored")
	if scanText.Source != "pdf" || !strings.Contains(scanText.Reason, "文字层") {
		t.Fatalf("scan content result = %+v", scanText)
	}

	// A text-less PDF will not improve on retry; the next run must not
	// download it again.
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("second scrape: %v", err)
	}
	if got := pdfHits.Load(); got != 2 {
		t.Fatalf("PDFs re-downloaded on the next run: hits = %d", got)
	}
}

func TestAcceptanceRSSFailureThen304BackfillPreservesUserState(t *testing.T) {
	longBody := longText("RECOVERED ARTICLE BODY", 14)
	var articleOK atomic.Bool
	var feed304 atomic.Int32
	var ifNoneMatch atomic.Value
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			if r.Header.Get("If-None-Match") == `"fixture-v1"` {
				feed304.Add(1)
				ifNoneMatch.Store(r.Header.Get("If-None-Match"))
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"fixture-v1"`)
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(rssFeed(rssItem("Broken First", srv.URL+"/article-broken",
				"<p>Feed summary survives the failure.</p>", ""))))
		case "/article-broken":
			if !articleOK.Load() {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("not found yet"))
				return
			}
			_, _ = w.Write([]byte(articleHTML("Recovered Article", longBody)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "rss-recover", Name: "RSS Recover", Type: "rss", URL: srv.URL + "/feed.xml",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Config: `{"full_content":true}`}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("first scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 1 {
		t.Fatalf("items after first run = %d, want 1", len(items))
	}
	firstID := items[0].ID
	if !strings.Contains(items[0].Content, "Feed summary survives") {
		t.Fatalf("first run should keep the feed summary: %q", items[0].Content)
	}
	if !strings.Contains(items[0].Metadata, `"full_content_status":"failed"`) {
		t.Fatalf("first run metadata should record the failure: %s", items[0].Metadata)
	}
	// Pin user state: backfill must not reset these.
	if err := db.MarkItemRead(firstID, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.ToggleItemStarred(firstID, 1); err != nil {
		t.Fatal(err)
	}
	log1 := latestFetchLog(t, db, src.ID)
	entry1 := findLogItem(fetchLogItems(t, db, log1.ID), "/article-broken")
	txt1 := requireTextStatus(t, entry1, "content", "failed")
	if txt1.Stage != "access" || !strings.Contains(txt1.Reason, "404") {
		t.Fatalf("failure should identify access/HTTP 404, got %+v", txt1)
	}

	articleOK.Store(true)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("second scrape: %v", err)
	}
	if feed304.Load() == 0 {
		t.Fatalf("second run did not send If-None-Match; header history=%v", ifNoneMatch.Load())
	}
	items = itemsBySource(t, db, src.ID)
	if len(items) != 1 || items[0].ID != firstID {
		t.Fatalf("backfill must update the same row, got %+v", items)
	}
	if !strings.Contains(items[0].Content, "RECOVERED ARTICLE BODY") {
		t.Fatalf("304 backfill did not store recovered content: %q", items[0].Content)
	}
	if !strings.Contains(items[0].Summary, "Feed summary survives") {
		t.Fatalf("original summary was replaced: %q", items[0].Summary)
	}
	if items[0].ReadStatus != 1 || items[0].Starred != 1 {
		t.Fatalf("user state lost: read=%d starred=%d", items[0].ReadStatus, items[0].Starred)
	}
	if !strings.Contains(items[0].Metadata, `"full_content_status":"fetched"`) {
		t.Fatalf("metadata not updated after recovery: %s", items[0].Metadata)
	}

	log2 := latestFetchLog(t, db, src.ID)
	if log2.ID == log1.ID || log2.Status != "not_modified" {
		t.Fatalf("second log = %+v, want a distinct not_modified run", log2)
	}
	entry2 := findLogItem(fetchLogItems(t, db, log2.ID), "/article-broken")
	requireTextStatus(t, entry2, "content", "updated")
	if entry2.ItemID != firstID {
		t.Fatalf("backfill log item id = %d, want %d", entry2.ItemID, firstID)
	}

	// The earlier failure remains queryable after the successful backfill.
	logs, err := db.GetFetchLogs(src.ID, 10)
	if err != nil || len(logs) != 2 {
		t.Fatalf("logs = %d err=%v, want 2", len(logs), err)
	}
	foundFailure := false
	for _, l := range logs {
		if l.ID == log1.ID {
			foundFailure = true
			if l.Status != "partial" && l.Status != "success" {
				t.Fatalf("first log status = %q", l.Status)
			}
		}
	}
	if !foundFailure {
		t.Fatalf("previous failure log disappeared: %+v", logs)
	}
}

func TestAcceptanceRSSPersistFailureIsLoggedNotCountedAsStored(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssFeed(rssItem("Poison", srv.URL+"/poison", "<p>poison item</p>", ""))))
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	if _, err := db.ExecRaw(`
		CREATE TRIGGER fail_poison BEFORE INSERT ON scraped_items
		WHEN NEW.url LIKE '%poison%'
		BEGIN SELECT RAISE(ABORT, 'simulated persist failure'); END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	scraper := newFastScraper(db)
	src := source.Source{ID: "rss-poison", Name: "Poison", Type: "rss", URL: srv.URL + "/feed.xml",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Config: `{"full_content":false}`}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if items := itemsBySource(t, db, src.ID); len(items) != 0 {
		t.Fatalf("poison item should not be stored, got %+v", items)
	}
	log := latestFetchLog(t, db, src.ID)
	if log.Status != "error" || log.ItemsAdded != 0 {
		t.Fatalf("log = %+v, want error with 0 added", log)
	}
	entry := findLogItem(fetchLogItems(t, db, log.ID), "/poison")
	if entry == nil || entry.Status != "failed" || entry.ErrorStage != "persist" {
		t.Fatalf("persist failure entry = %+v", entry)
	}
	if !strings.Contains(entry.Reason, "simulated persist failure") {
		t.Fatalf("entry reason = %q", entry.Reason)
	}
}

func TestAcceptanceRSS304RotatesPastFailedArticles(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/feed":
			if req.Header.Get("If-None-Match") != "" {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"unchanged"`)
			_, _ = w.Write([]byte(rssFeed(
				rssItem("Failing", server.URL+"/failing", "Short summary", "") +
					rssItem("Older", server.URL+"/older", "Another summary", ""),
			)))
		case "/older":
			_, _ = w.Write([]byte(articleHTML("Older", longText("LATER BACKFILL", 12))))
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "rss-rotate", Name: "RSS rotate", Type: "rss", URL: server.URL + "/feed",
		Config: `{"full_content":true,"max_backfill":1}`}
	insertTestSource(t, db, src)
	for range 2 {
		if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
			t.Fatal(err)
		}
	}
	entry := mustItemByURL(t, db, server.URL+"/older")
	if !strings.Contains(entry.Content, "LATER BACKFILL") {
		t.Fatalf("the older entry did not get its turn: %+v", entry)
	}
	log := latestFetchLog(t, db, src.ID)
	if log.Status != "not_modified" || log.ItemsUpdated != 1 {
		t.Fatalf("unexpected backfill log: %+v", log)
	}
}
