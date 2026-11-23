package scraping

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go-server/internal/domain/source"
)

type blogFixture struct {
	mu       sync.Mutex
	articles map[string]bool
	listHTML string
}

func TestAcceptanceBlogListTwoStructuresAndUpdate(t *testing.T) {
	fx := &blogFixture{articles: map[string]bool{}}
	fx.listHTML = `<html><body>
<nav><a href="/site-a/">Home</a><a href="/site-a/about">About</a></nav>
<main class="news-list">
  <a class="card" href="/site-a/news/alpha">Alpha Article</a>
  <a class="tag" href="/site-a/tags/ai">AI</a>
  <a class="card" href="/site-a/news/alpha">Alpha Article (duplicate)</a>
  <a class="card" href="/site-a/news/beta">Beta Article</a>
</main>
<footer><a href="/site-a/privacy">Privacy</a></footer>
</body></html>`

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fx.mu.Lock()
		list := fx.listHTML
		fx.mu.Unlock()
		switch {
		case r.URL.Path == "/site-a/news":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(list))
		case strings.HasPrefix(r.URL.Path, "/site-a/news/"):
			w.Header().Set("Content-Type", "text/html")
			name := strings.TrimPrefix(r.URL.Path, "/site-a/news/")
			_, _ = w.Write([]byte(articleHTML("Article "+name, longText("BODY "+name, 10))))
		case r.URL.Path == "/site-b/blog":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<div id="posts">
  <a href="/site-b/blog/one">Post One</a>
  <a href="/site-b/blog/two">Post Two</a>
  <a href="/site-b/about">About this blog</a>
</div>
<aside><a href="/site-b/blog/two">Post Two again</a><a href="/site-b/feed.xml">RSS</a></aside>
</body></html>`))
		case strings.HasPrefix(r.URL.Path, "/site-b/blog/"):
			w.Header().Set("Content-Type", "text/html")
			name := strings.TrimPrefix(r.URL.Path, "/site-b/blog/")
			_, _ = w.Write([]byte(articleHTML("Post "+name, longText("POST BODY "+name, 10))))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)

	srcA := source.Source{ID: "blog-a", Name: "Blog A", Type: "web_scrape", URL: srv.URL + "/site-a/news",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto",
		Config: `{"full_content":true,"list_selector":".news-list","article_link_selector":"a.card","max_items":10}`}
	srcB := source.Source{ID: "blog-b", Name: "Blog B", Type: "web_scrape", URL: srv.URL + "/site-b/blog",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto",
		Config: `{"full_content":true,"list_selector":"#posts","article_url_pattern":"/site-b/blog/[a-z]+$","max_items":10}`}
	insertTestSource(t, db, srcA)
	insertTestSource(t, db, srcB)

	if _, err := scrapeFresh(t, db, scraper, srcA.ID, "manual"); err != nil {
		t.Fatalf("blog A scrape: %v", err)
	}
	if _, err := scrapeFresh(t, db, scraper, srcB.ID, "manual"); err != nil {
		t.Fatalf("blog B scrape: %v", err)
	}

	itemsA := itemsBySource(t, db, srcA.ID)
	if len(itemsA) != 2 {
		t.Fatalf("blog A items = %d, want 2: %+v", len(itemsA), itemsA)
	}
	for _, it := range itemsA {
		if !strings.Contains(it.Content, "BODY "+strings.TrimPrefix(it.URL[strings.LastIndex(it.URL, "/")+1:], "")) {
			t.Fatalf("unexpected article body for %s: %q", it.URL, it.Content)
		}
		if it.PublishedAt == nil {
			t.Fatalf("published time not parsed for %s", it.URL)
		}
		if it.Category != "blog" {
			t.Fatalf("category = %q, want blog", it.Category)
		}
	}
	for _, bad := range []string{"/tags/", "/about", "/privacy"} {
		for _, it := range itemsA {
			if strings.Contains(it.URL, bad) {
				t.Fatalf("non-article link %q was stored: %+v", bad, itemsA)
			}
		}
	}
	for _, it := range itemsA {
		if strings.TrimSuffix(it.URL, "/") == strings.TrimSuffix(srv.URL+"/site-a/news", "/") {
			t.Fatalf("list page itself was stored as an article: %q", it.URL)
		}
	}

	itemsB := itemsBySource(t, db, srcB.ID)
	if len(itemsB) != 2 {
		t.Fatalf("blog B items = %d, want 2: %+v", len(itemsB), itemsB)
	}
	for _, it := range itemsB {
		if !strings.Contains(it.URL, "/site-b/blog/") || strings.Contains(it.URL, "about") {
			t.Fatalf("unexpected blog B article %q", it.URL)
		}
	}

	// Adding one new card must only add that article; the duplicate and old
	// entries must not create copies and keep their original IDs.
	oldIDs := map[string]int64{}
	for _, it := range itemsBySource(t, db, srcA.ID) {
		oldIDs[it.URL] = it.ID
	}
	fx.mu.Lock()
	fx.listHTML = strings.Replace(fx.listHTML,
		`<a class="card" href="/site-a/news/beta">Beta Article</a>`,
		`<a class="card" href="/site-a/news/beta">Beta Article</a><a class="card" href="/site-a/news/gamma">Gamma Article</a>`, 1)
	fx.mu.Unlock()
	if _, err := scrapeFresh(t, db, scraper, srcA.ID, "manual"); err != nil {
		t.Fatalf("blog A update scrape: %v", err)
	}
	itemsA = itemsBySource(t, db, srcA.ID)
	if len(itemsA) != 3 {
		t.Fatalf("after update items = %d, want 3: %+v", len(itemsA), itemsA)
	}
	for _, it := range itemsA {
		if id, ok := oldIDs[it.URL]; ok && id != it.ID {
			t.Fatalf("existing item %s changed ID from %d to %d", it.URL, id, it.ID)
		}
	}
	// An unchanged third run must be a no-op (no new rows, no duplicates).
	before := len(itemsBySource(t, db, srcA.ID))
	if _, err := scrapeFresh(t, db, scraper, srcA.ID, "manual"); err != nil {
		t.Fatalf("blog A noop scrape: %v", err)
	}
	if after := len(itemsBySource(t, db, srcA.ID)); after != before {
		t.Fatalf("repeated poll added duplicates: %d -> %d", before, after)
	}
	lastLog := latestFetchLog(t, db, srcA.ID)
	if lastLog.Status != "no_new_items" {
		t.Fatalf("no-op run status = %q, want no_new_items", lastLog.Status)
	}
}

func TestAcceptanceBlogSingleFailureDoesNotBlockOthers(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/news":
			_, _ = w.Write([]byte(`<html><body><div class="list">
<a href="/news/good-1">Good One</a>
<a href="/news/broken">Broken One</a>
<a href="/news/good-2">Good Two</a>
</div></body></html>`))
		case "/news/good-1", "/news/good-2":
			_, _ = w.Write([]byte(articleHTML("Good", longText("GOOD BODY", 10))))
		case "/news/broken":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "blog-mixed", Name: "Mixed", Type: "web_scrape", URL: srv.URL + "/news",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "blog",
		Config: `{"list_selector":".list","max_items":10}`}
	insertTestSource(t, db, src)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 good articles: %+v", len(items), items)
	}
	log := latestFetchLog(t, db, src.ID)
	if log.Status != "partial" {
		t.Fatalf("log status = %q, want partial", log.Status)
	}
	entry := findLogItem(fetchLogItems(t, db, log.ID), "/news/broken")
	if entry == nil || entry.Status != "failed" || entry.ErrorStage != "access" {
		t.Fatalf("broken entry = %+v", entry)
	}
	if !strings.Contains(entry.Reason, "500") {
		t.Fatalf("failure reason should include HTTP 500: %q", entry.Reason)
	}
	for _, part := range []string{"/news/good-1", "/news/good-2"} {
		if e := findLogItem(fetchLogItems(t, db, log.ID), part); e == nil || e.Status != "stored" {
			t.Fatalf("missing stored log entry for %s: %+v", part, e)
		}
	}
}

func TestAcceptanceBlogSinglePageCompatibility(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(articleHTML("Legacy Single Page", longText("LEGACY BODY", 10))))
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "legacy-web", Name: "Legacy Web", Type: "web_scrape", URL: srv.URL + "/one",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Config: `{}`}
	insertTestSource(t, db, src)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 1 || !strings.Contains(items[0].Content, "LEGACY BODY") {
		t.Fatalf("legacy single-page subscription broken: %+v", items)
	}
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("second scrape: %v", err)
	}
	if n := len(itemsBySource(t, db, src.ID)); n != 1 {
		t.Fatalf("single page duplicated on second poll: %d", n)
	}
}

func TestAcceptanceCrawlLinksAutoDiscovery(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/news":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<header class="site-header"><a href="/">Home</a><a href="/news/about">About</a></header>
<main>
  <h1>News</h1>
  <section>
    <article>
      <header><a href="/news/announcements">Announcements</a></header>
      <a href="/news/alpha"><h2>Alpha Article</h2><p>Alpha summary</p></a>
      <a href="/news/alpha"><img alt="Alpha photo"></a>
    </article>
    <article>
      <header><a href="/news/product">Product</a></header>
      <a href="/news/beta"><h3>Beta Article</h3></a>
      <a href="/photos/gallery">Photo gallery</a>
    </article>
  </section>
  <nav><a href="/news/alpha">Alpha Article</a></nav>
</main>
<footer><a href="/privacy">Privacy</a><a href="/news/gamma">Gamma in footer</a></footer>
</body></html>`))
		case "/news/alpha", "/news/beta":
			name := strings.TrimPrefix(r.URL.Path, "/news/")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(articleHTML("Article "+name, longText("BODY "+name, 10))))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "auto-crawl", Name: "Auto Crawl", Type: "web_scrape", URL: srv.URL + "/news",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto",
		Config: `{"crawl_links":true,"max_items":10}`}
	insertTestSource(t, db, src)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}

	items := itemsBySource(t, db, src.ID)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 article links: %+v", len(items), items)
	}
	for _, it := range items {
		if !strings.HasSuffix(it.URL, "/news/alpha") && !strings.HasSuffix(it.URL, "/news/beta") {
			t.Fatalf("non-article link was stored: %s", it.URL)
		}
		if !strings.Contains(it.Content, "BODY "+strings.TrimPrefix(it.URL[strings.LastIndex(it.URL, "/")+1:], "")) {
			t.Fatalf("unexpected article body for %s: %q", it.URL, it.Content)
		}
	}
	if log := latestFetchLog(t, db, src.ID); log.Status != "success" {
		t.Fatalf("log status = %q, want success: %s", log.Status, log.Summary)
	}
}

func TestAcceptanceBlogListIncludesPDFLinks(t *testing.T) {
	textPDF := buildTestPDF("BT /F1 12 Tf 72 720 Td (" + longText("PDF LIST BODY", 16) + ") Tj ET")
	scanPDF := buildTestPDF("")

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/papers":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><main class="post-list">
  <a href="/files/annual-report.pdf">Annual Report</a>
  <a href="/files/scanned-notes.pdf">Scanned Notes</a>
  <a href="/news/text-article">Text Article</a>
</main></body></html>`))
		case "/files/annual-report.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(textPDF)
		case "/files/scanned-notes.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(scanPDF)
		case "/news/text-article":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(articleHTML("Text Article", longText("TEXT ARTICLE BODY", 12))))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "pdf-list", Name: "PDF List", Type: "web_scrape", URL: srv.URL + "/papers",
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "auto",
		Config: `{"crawl_links":true,"max_items":10}`}
	insertTestSource(t, db, src)
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("scrape: %v", err)
	}

	items := itemsBySource(t, db, src.ID)
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3 (two PDFs and one article): %+v", len(items), items)
	}
	var report, scan bool
	for _, it := range items {
		switch {
		case strings.HasSuffix(it.URL, "/files/annual-report.pdf"):
			report = true
			if !strings.Contains(it.Content, "PDF LIST BODY") {
				t.Fatalf("PDF text not stored: %q", it.Content)
			}
			for _, want := range []string{`"content_source":"pdf_text"`, `"pdf_url"`, `"pdf_pages":1`} {
				if !strings.Contains(it.Metadata, want) {
					t.Fatalf("PDF metadata misses %s: %s", want, it.Metadata)
				}
			}
		case strings.HasSuffix(it.URL, "/files/scanned-notes.pdf"):
			scan = true
			if !strings.Contains(it.Metadata, `"content_source":"pdf"`) {
				t.Fatalf("scanned PDF should still be stored with a link: %s", it.Metadata)
			}
		}
	}
	if !report || !scan {
		t.Fatalf("missing PDF items: report=%v scan=%v items=%+v", report, scan, items)
	}
}
