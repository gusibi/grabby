package httpiface

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/sqlite"
)

type e2eItemDetail struct {
	Item struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		URL      string `json:"url"`
		Summary  string `json:"summary"`
		Content  string `json:"content"`
		Metadata string `json:"metadata"`
		Read     int    `json:"read_status"`
		Starred  int    `json:"starred"`
	} `json:"item"`
	HTMLContent string `json:"html_content"`
}

type e2eLogDetail struct {
	Success bool            `json:"success"`
	Log     sqlite.FetchLog `json:"log"`
	Items   []struct {
		ID         int64                    `json:"id"`
		ItemID     int64                    `json:"item_id"`
		Title      string                   `json:"title"`
		URL        string                   `json:"url"`
		Status     string                   `json:"status"`
		Reason     string                   `json:"reason"`
		ErrorStage string                   `json:"error_stage"`
		Texts      []sqlite.FetchTextResult `json:"texts"`
	} `json:"items"`
}

func newE2ERouter(t *testing.T, db *sqlite.Database) (*httptest.Server, *scraping.Scraper) {
	t.Helper()
	scraper := scraping.NewScraper(db, nil, nil, zap.NewNop(), nil)
	sched := scheduler.NewScheduler(db, scraper, nil, zap.NewNop())
	router := NewRouter(Dependencies{
		Settings:  &config.Settings{},
		Logger:    zap.NewNop(),
		DB:        db,
		Scraper:   scraper,
		Scheduler: sched,
	})
	srv := httptest.NewServer(router)
	return srv, scraper
}

func e2eJSON(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

func TestAcceptanceAppAPIFromSourceCreationToLogDetail(t *testing.T) {
	longBody := strings.Repeat("App-level full article body content for acceptance. ", 12)
	var articleOK atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			if r.Header.Get("If-None-Match") == `"app-v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"app-v1"`)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>App API Feed</title>
<item><title>App API Item</title><link>` + srv.URL + `/article</link>
<description><![CDATA[<p>App feed summary stays here.</p>]]></description></item>
</channel></rss>`))
		case "/article":
			if !articleOK.Load() {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="App API Article"></head>
<body><article><h1>App API Article</h1><p>` + longBody + `</p></article></body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "api-benchmark.db")
	db, err := sqlite.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	appSrv, _ := newE2ERouter(t, db)
	defer appSrv.Close()

	// 1. Create the source through the normal subscription entry.
	var created sqlite.Source
	code := e2eJSON(t, http.MethodPost, appSrv.URL+"/api/sources", map[string]any{
		"id": "api-rss", "name": "API RSS", "type": "rss", "url": srv.URL + "/feed.xml",
		"schedule": "0 * * * *", "default_category": "article", "category": "AI",
		"config": `{"full_content":true}`,
	}, &created)
	if code != http.StatusOK {
		t.Fatalf("create source status = %d", code)
	}
	var sources []sqlite.Source
	e2eJSON(t, http.MethodGet, appSrv.URL+"/api/sources", nil, &sources)
	createdSrc := findSource(sources, "api-rss")
	if createdSrc == nil || !strings.Contains(createdSrc.Config, `"full_content":true`) {
		t.Fatalf("source config not persisted/read back: %+v", createdSrc)
	}

	// 2. Run the fetch (article 404s), then inspect logs and the stored item.
	var runResp map[string]any
	if code := e2eJSON(t, http.MethodPost, appSrv.URL+"/api/sources/api-rss/run", nil, &runResp); code != http.StatusOK {
		t.Fatalf("run status = %d resp=%+v", code, runResp)
	}
	var logs []sqlite.FetchLog
	e2eJSON(t, http.MethodGet, appSrv.URL+"/api/logs?source_id=api-rss", nil, &logs)
	if len(logs) != 1 || logs[0].Trigger != "manual" {
		t.Fatalf("logs = %+v", logs)
	}
	firstLogID := logs[0].ID
	var detail e2eLogDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/api/logs/%d", appSrv.URL, firstLogID), nil, &detail)
	if !detail.Success || len(detail.Items) != 1 {
		t.Fatalf("log detail = %+v", detail)
	}
	firstEntry := detail.Items[0]
	if firstEntry.Status != "stored" || firstEntry.ItemID == 0 {
		t.Fatalf("entry = %+v", firstEntry)
	}
	var contentFailed bool
	for _, txt := range firstEntry.Texts {
		if txt.Kind == "content" && txt.Status == "failed" && txt.Stage == "access" {
			contentFailed = true
		}
	}
	if !contentFailed {
		t.Fatalf("expected failed content detail, got %+v", firstEntry.Texts)
	}

	var firstItem e2eItemDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/open/api/items/%d", appSrv.URL, firstEntry.ItemID), nil, &firstItem)
	if !strings.Contains(firstItem.Item.Summary, "App feed summary stays here") {
		t.Fatalf("item summary = %q", firstItem.Item.Summary)
	}
	if strings.Contains(firstItem.Item.Content, "App-level full article body") {
		t.Fatalf("first run should not have full body")
	}

	// 3. Pin user state; recover the article; the feed answers 304 and the
	// known item is still backfilled.
	articleOK.Store(true)
	if code := e2eJSON(t, http.MethodPost, fmt.Sprintf("%s/api/items/%d/read", appSrv.URL, firstEntry.ItemID),
		map[string]any{"read_status": 1}, nil); code != http.StatusOK {
		t.Fatalf("mark read status = %d", code)
	}
	if code := e2eJSON(t, http.MethodPost, fmt.Sprintf("%s/api/items/%d/star", appSrv.URL, firstEntry.ItemID),
		map[string]any{"starred": 1}, nil); code != http.StatusOK {
		t.Fatalf("star status = %d", code)
	}
	e2eJSON(t, http.MethodPost, appSrv.URL+"/api/sources/api-rss/run", nil, &runResp)
	e2eJSON(t, http.MethodGet, appSrv.URL+"/api/logs?source_id=api-rss", nil, &logs)
	if len(logs) != 2 {
		t.Fatalf("logs after recovery = %+v", logs)
	}
	var second sqlite.FetchLog
	for _, l := range logs {
		if l.ID != firstLogID {
			second = l
		}
	}
	if second.Status != "not_modified" {
		t.Fatalf("second log = %+v, want not_modified", second)
	}
	var secondDetail e2eLogDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/api/logs/%d", appSrv.URL, second.ID), nil, &secondDetail)
	if len(secondDetail.Items) != 1 || secondDetail.Items[0].Status != "updated" {
		t.Fatalf("second detail = %+v", secondDetail)
	}
	if secondDetail.Items[0].ItemID != firstEntry.ItemID {
		t.Fatalf("backfill changed item id")
	}
	var recovered e2eItemDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/open/api/items/%d", appSrv.URL, firstEntry.ItemID), nil, &recovered)
	if !strings.Contains(recovered.Item.Content, "App-level full article body") {
		t.Fatalf("item content not backfilled: %q", recovered.Item.Content)
	}
	if recovered.Item.Read != 1 || recovered.Item.Starred != 1 {
		t.Fatalf("user state lost: read=%d starred=%d", recovered.Item.Read, recovered.Item.Starred)
	}
	if !strings.Contains(recovered.Item.Summary, "App feed summary stays here") {
		t.Fatalf("summary lost: %q", recovered.Item.Summary)
	}

	// 4. Re-open the database and read the old failure + the new success.
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	reopened, err := sqlite.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer reopened.Close()
	appSrv2, _ := newE2ERouter(t, reopened)
	defer appSrv2.Close()
	var persisted []sqlite.FetchLog
	e2eJSON(t, http.MethodGet, appSrv2.URL+"/api/logs?source_id=api-rss", nil, &persisted)
	if len(persisted) != 2 {
		t.Fatalf("persisted logs = %+v", persisted)
	}
	var oldDetail, newDetail e2eLogDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/api/logs/%d", appSrv2.URL, firstLogID), nil, &oldDetail)
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/api/logs/%d", appSrv2.URL, second.ID), nil, &newDetail)
	if len(oldDetail.Items) == 0 || len(newDetail.Items) == 0 {
		t.Fatalf("log details did not survive reopen: old=%+v new=%+v", oldDetail, newDetail)
	}
	if oldDetail.Items[0].Status == newDetail.Items[0].Status {
		t.Fatalf("old failure and new success should be distinguishable: %+v / %+v", oldDetail.Items[0], newDetail.Items[0])
	}

	// 5. Editing the source round-trips the full-content setting.
	var updated sqlite.Source
	if code := e2eJSON(t, http.MethodPut, appSrv2.URL+"/api/sources/api-rss", map[string]any{
		"name": "API RSS", "type": "rss", "url": srv.URL + "/feed.xml", "schedule": "0 * * * *",
		"default_category": "article", "category": "AI", "config": `{"fetch_full_via_scrape":false,"full_content":false}`,
	}, &updated); code != http.StatusOK {
		t.Fatalf("update source status = %d", code)
	}
	var reread []sqlite.Source
	e2eJSON(t, http.MethodGet, appSrv2.URL+"/api/sources", nil, &reread)
	updatedSrc := findSource(reread, "api-rss")
	if updatedSrc == nil || !strings.Contains(updatedSrc.Config, `"full_content":false`) {
		t.Fatalf("updated config not readable: %+v", updatedSrc)
	}
}

func TestAcceptancePodcastContentDetailAndLog(t *testing.T) {
	var audioHits atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".mp3") {
			audioHits.Add(1)
			return
		}
		switch r.URL.Path {
		case "/podcast.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:podcast="https://podcastindex.org/namespace/1.0">
<channel><title>App Podcast</title>
<item><title>App Episode</title><link>` + srv.URL + `/ep</link>
<description><![CDATA[<p>App show notes paragraph.</p>]]></description>
<enclosure url="` + srv.URL + `/ep.mp3" type="audio/mpeg"/>
<itunes:duration>30:00</itunes:duration>
<podcast:transcript url="` + srv.URL + `/ep.srt" type="application/srt"/>
</item></channel></rss>`))
		case "/ep.srt":
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:04,000\nApp transcript line.\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newE2ERouterDB(t)
	appSrv, _ := newE2ERouter(t, db)
	defer appSrv.Close()

	var created sqlite.Source
	if code := e2eJSON(t, http.MethodPost, appSrv.URL+"/api/sources", map[string]any{
		"id": "app-pod", "name": "App Podcast", "type": "podcast", "url": srv.URL + "/podcast.xml",
		"schedule": "0 * * * *", "default_category": "podcast", "config": `{"max_items":5}`,
	}, &created); code != http.StatusOK {
		t.Fatalf("create podcast source: %d", code)
	}
	var runResp map[string]any
	if code := e2eJSON(t, http.MethodPost, appSrv.URL+"/api/sources/app-pod/run", nil, &runResp); code != http.StatusOK {
		t.Fatalf("run podcast: %d", code)
	}
	var logs []sqlite.FetchLog
	e2eJSON(t, http.MethodGet, appSrv.URL+"/api/logs?source_id=app-pod", nil, &logs)
	if len(logs) != 1 {
		t.Fatalf("podcast logs = %+v", logs)
	}
	var detail e2eLogDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/api/logs/%d", appSrv.URL, logs[0].ID), nil, &detail)
	if len(detail.Items) != 1 || detail.Items[0].ItemID == 0 {
		t.Fatalf("podcast log detail = %+v", detail)
	}
	var texts = map[string]string{}
	for _, txt := range detail.Items[0].Texts {
		texts[txt.Kind] = txt.Status
	}
	if texts["show_notes"] != "stored" || texts["transcript"] != "stored" || texts["audio"] != "stored" {
		t.Fatalf("podcast text statuses = %+v", texts)
	}
	var item e2eItemDetail
	e2eJSON(t, http.MethodGet, fmt.Sprintf("%s/open/api/items/%d", appSrv.URL, detail.Items[0].ItemID), nil, &item)
	if !strings.Contains(item.Item.Content, "App show notes paragraph") ||
		!strings.Contains(item.Item.Content, "App transcript line") {
		t.Fatalf("podcast detail missing notes/transcript: %q", item.Item.Content)
	}
	if !strings.Contains(item.Item.Metadata, `"media_url":"`+srv.URL+`/ep.mp3"`) {
		t.Fatalf("podcast detail missing audio link: %s", item.Item.Metadata)
	}
	if audioHits.Load() != 0 {
		t.Fatalf("podcast scrape downloaded audio")
	}
}

func newE2ERouterDB(t *testing.T) (*sqlite.Database, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api-podcast.db")
	db, err := sqlite.NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func findSource(list []sqlite.Source, id string) *sqlite.Source {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}
