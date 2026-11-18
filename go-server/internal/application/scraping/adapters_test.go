package scraping

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"go.uber.org/zap"

	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

func TestExtractWebArticleAndNeedsDeepFetch(t *testing.T) {
	articleSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head>
  <title>Site Title</title>
  <meta property="og:title" content="Deep Engineering Post">
</head>
<body>
  <nav><a href="/home">Home</a></nav>
  <article>
    <h1>Deep Engineering Post</h1>
    <p>This is a full-length technical article explaining how local-first agents ingest feeds, extract clean markdown, and unify RSS, YouTube, and podcasts behind one adapter interface.</p>
    <p>Second paragraph with concrete implementation details and code examples like <code>SourceAdapter</code> and <code>QueryUnifiedFeed</code> so it exceeds three hundred characters easily without noise.</p>
    <ul><li>Point one about clean extraction</li><li>Point two about transcripts</li></ul>
  </article>
  <footer>Copyright 2026</footer>
</body>
</html>`))
	}))
	defer articleSrv.Close()

	need, target := NeedsDeepFetch(`<a href="`+articleSrv.URL+`">Comments</a>`, "https://news.ycombinator.com/item?id=12345")
	if !need {
		t.Fatalf("expected NeedsDeepFetch=true for link-only HN item")
	}
	if target != articleSrv.URL {
		t.Fatalf("target = %q, want %q", target, articleSrv.URL)
	}

	title, md, err := ExtractWebArticle(context.Background(), articleSrv.URL)
	if err != nil {
		t.Fatalf("ExtractWebArticle failed: %v", err)
	}
	if title != "Deep Engineering Post" {
		t.Errorf("title = %q, want Deep Engineering Post", title)
	}
	if !strings.Contains(md, "SourceAdapter") || !strings.Contains(md, "- Point one about clean extraction") {
		t.Errorf("unexpected extracted markdown: %s", md)
	}
	if strings.Contains(md, "Copyright 2026") {
		t.Errorf("footer noise should be stripped: %s", md)
	}

	if !RequiresBrowserExtract("https://mp.weixin.qq.com/s/abc123") {
		t.Errorf("expected mp.weixin.qq.com to require browser extract")
	}
	if RequiresBrowserExtract("https://example.com/blog/post") {
		t.Errorf("did not expect example.com to require browser extract")
	}
}

func TestScrapeRSS_DeepFetchToggle(t *testing.T) {
	articleHTML := `<html><head><meta property="og:title" content="Full Linked Article"></head><body><article><p>` +
		strings.Repeat("Detailed technical content about Grabby adapters and transcripts. ", 12) +
		`</p></article></body></html>`

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed.xml" {
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Short Feed</title>
<item>
  <title>Short Link Item</title>
  <link>` + srv.URL + `/article-1</link>
  <description>Comments</description>
</item>
</channel></rss>`))
			return
		}
		if r.URL.Path == "/article-1" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(articleHTML))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	src := source.Source{
		ID:              "short-rss",
		Name:            "Short RSS",
		Type:            "rss",
		URL:             srv.URL + "/feed.xml",
		Schedule:        "0 * * * *",
		Enabled:         1,
		DefaultCategory: "article",
		Config:          `{"full_content":true}`,
	}
	if err := db.InsertSource(src); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	scraper := NewScraper(db, nil, nil, zap.NewNop(), nil)
	added, err := scraper.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("ScrapeSource: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}

	items, _, err := db.GetScrapedItems(sqlite.ItemsFilter{SourceID: src.ID, Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("GetScrapedItems: len=%d err=%v", len(items), err)
	}
	if utf8.RuneCountInString(items[0].Content) < 300 {
		t.Fatalf("expected deep fetched content >= 300 chars, got %d: %q", len(items[0].Content), items[0].Content)
	}
}

func TestYouTubeAdapterParsingAndChannelResolution(t *testing.T) {
	if vid := ExtractYouTubeVideoID("https://www.youtube.com/watch?v=dQw4w9WgXcQ"); vid != "dQw4w9WgXcQ" {
		t.Errorf("ExtractYouTubeVideoID watch = %q", vid)
	}
	if vid := ExtractYouTubeVideoID("https://youtu.be/dQw4w9WgXcQ?t=12"); vid != "dQw4w9WgXcQ" {
		t.Errorf("ExtractYouTubeVideoID youtu.be = %q", vid)
	}
	if vid := ExtractYouTubeVideoID("https://www.youtube.com/shorts/abcDEF12345"); vid != "abcDEF12345" {
		t.Errorf("ExtractYouTubeVideoID shorts = %q", vid)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@testchannel":
			_, _ = w.Write([]byte(`<html><meta itemprop="channelId" content="UC1234567890123456789012"></html>`))
		case "/captions":
			_, _ = w.Write([]byte(`{"events":[
				{"tStartMs":1000,"segs":[{"utf8":"Hello world"}]},
				{"tStartMs":35000,"segs":[{"utf8":"Second segment"}]}
			]}`))
		case "/watch":
			player := map[string]any{
				"videoDetails": map[string]any{
					"videoId":          "vid12345678",
					"title":            "Test Video Title",
					"author":           "Test Creator",
					"channelId":        "UC1234567890123456789012",
					"shortDescription": "Video overview here",
					"lengthSeconds":    "125",
				},
				"captions": map[string]any{
					"playerCaptionsTracklistRenderer": map[string]any{
						"captionTracks": []map[string]any{
							{"baseUrl": srv.URL + "/captions?lang=en", "languageCode": "en"},
						},
					},
				},
			}
			b, _ := json.Marshal(player)
			_, _ = w.Write([]byte(`<html><script>var ytInitialPlayerResponse = ` + string(b) + `;</script></html>`))
		}
	}))
	defer srv.Close()

	yt := NewYouTubeAdapter(nil)
	yt.httpClient = srv.Client()

	feedURL, _, isSingle, err := yt.ResolveYouTubeFeedURL(context.Background(), srv.URL+"/@testchannel")
	if err != nil {
		t.Fatalf("ResolveYouTubeFeedURL handle: %v", err)
	}
	if isSingle || feedURL != "https://www.youtube.com/feeds/videos.xml?channel_id=UC1234567890123456789012" {
		t.Fatalf("unexpected resolved feed: isSingle=%v feedURL=%q", isSingle, feedURL)
	}

	htmlStr, err := yt.fetchPageHTML(context.Background(), srv.URL+"/watch")
	if err != nil {
		t.Fatalf("fetchPageHTML: %v", err)
	}
	payload, err := parseYouTubePlayerResponseFromHTML(context.Background(), yt.httpClient, htmlStr)
	if err != nil {
		t.Fatalf("parseYouTubePlayerResponseFromHTML: %v", err)
	}
	if payload.Title != "Test Video Title" || !strings.Contains(payload.Transcript, "[00:01] Hello world") || !strings.Contains(payload.Transcript, "[00:35] Second segment") {
		t.Fatalf("unexpected parsed YouTube payload: %+v", payload)
	}
}

func TestPodcastAdapterRSSAndXiaoyuzhou(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/transcript.srt":
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:05,000\nWelcome to the podcast.\n\n2\n00:00:06,000 --> 00:00:10,000\nToday we discuss AI agents.\n"))
		case "/podcast.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:podcast="https://podcastindex.org/namespace/1.0">
<channel>
  <title>Tech Talk Pod</title>
  <item>
    <title>EP 10: Local Agents</title>
    <link>` + srv.URL + `/ep10</link>
    <description><![CDATA[<p>Show notes paragraph.</p>]]></description>
    <enclosure url="` + srv.URL + `/ep10.mp3" type="audio/mpeg" length="1024"/>
    <itunes:duration>42:10</itunes:duration>
    <podcast:transcript url="` + srv.URL + `/transcript.srt" type="application/srt"/>
  </item>
</channel>
</rss>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	src := source.Source{
		ID:              "tech-pod",
		Name:            "Tech Pod",
		Type:            "podcast",
		URL:             srv.URL + "/podcast.xml",
		Schedule:        "0 */4 * * *",
		Enabled:         1,
		DefaultCategory: "podcast",
		Config:          `{"transcribe_audio":false}`,
	}
	if err := db.InsertSource(src); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	scraper := NewScraper(db, nil, nil, zap.NewNop(), nil)
	added, err := scraper.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("ScrapeSource podcast: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}

	items, _, err := db.GetScrapedItems(sqlite.ItemsFilter{ContentType: "podcast", Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("GetScrapedItems podcast: len=%d err=%v", len(items), err)
	}
	if !strings.Contains(items[0].Content, "Welcome to the podcast.") || !strings.Contains(items[0].Content, "Show notes paragraph.") {
		t.Errorf("unexpected podcast content: %s", items[0].Content)
	}
	if !strings.Contains(items[0].Metadata, `"transcript_source":"podcast_transcript"`) {
		t.Errorf("unexpected podcast metadata: %s", items[0].Metadata)
	}

	// Test Xiaoyuzhou __NEXT_DATA__ parsing
	xyzHTML := `<html><body><script id="__NEXT_DATA__" type="application/json">{
		"props":{"pageProps":{"episode":{
			"eid":"ep999",
			"title":"小宇宙单集测试",
			"shownotes":"<p>这是一期关于 Agent 的播客。</p>",
			"duration":1800,
			"enclosure":{"url":"https://media.xyzcdn.net/ep999.m4a"},
			"podcast":{"title":"AI 漫谈"}
		}}}
	}</script></body></html>`
	eps, err := parseXiaoyuzhouHTML(xyzHTML, "https://www.xiaoyuzhoufm.com/episode/ep999")
	if err != nil || len(eps) != 1 {
		t.Fatalf("parseXiaoyuzhouHTML: len=%d err=%v", len(eps), err)
	}
	if eps[0].Title != "小宇宙单集测试" || eps[0].AudioURL != "https://media.xyzcdn.net/ep999.m4a" || eps[0].Duration != "30:00" {
		t.Fatalf("unexpected xiaoyuzhou episode: %+v", eps[0])
	}
}

func TestScraper_IngestURL_WebAndFeedQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Instant Blog</title></head><body><main><p>` +
			strings.Repeat("Instant ingestion content for AI agent consumption. ", 10) +
			`</p></main></body></html>`))
	}))
	defer srv.Close()

	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	scraper := NewScraper(db, nil, nil, zap.NewNop(), nil)
	res, err := scraper.IngestURL(context.Background(), srv.URL+"/post", "blog", false)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	if res == nil || res.Category != "blog" || !strings.Contains(res.Content, "Instant ingestion content") {
		t.Fatalf("unexpected IngestURL result: %+v", res)
	}

	feedItems, _, err := db.QueryUnifiedFeed(sqlite.ItemsFilter{ContentType: "blog", Limit: 10})
	if err != nil || len(feedItems) != 1 {
		t.Fatalf("QueryUnifiedFeed blog: len=%d err=%v", len(feedItems), err)
	}
}
