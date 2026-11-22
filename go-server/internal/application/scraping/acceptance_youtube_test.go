package scraping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

func ytAtomFeed(srvURL string) string {
	entry := func(id, title, link, description string) string {
		return fmt.Sprintf(`<entry>
  <id>yt:video:%s</id><yt:videoId>%s</yt:videoId>
  <title>%s</title>
  <link rel="alternate" href="%s"/>
  <published>2026-11-18T08:00:00+00:00</published>
  <media:group><media:description>%s</media:description></media:group>
</entry>`, id, id, title, link, description)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
<title>Fixture Channel</title>
` + entry("VIDA000001", "Video A", "https://youtu.be/VIDA000001", "Feed summary A") +
		entry("VIDB000002", "Video B", "https://www.youtube.com/watch?v=VIDB000002&t=12", "Feed summary B") +
		entry("VIDC000003", "Video C", "https://www.youtube.com/watch?v=VIDC000003", "Feed summary C") +
		`</feed>`
}

func ytPlayerHTML(details map[string]any, tracks []map[string]any) string {
	player := map[string]any{"videoDetails": details}
	if tracks != nil {
		player["captions"] = map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{"captionTracks": tracks},
		}
	}
	b, _ := json.Marshal(player)
	return `<html><head><title>fixture</title></head><body><script>var ytInitialPlayerResponse = ` + string(b) + `;</script></body></html>`
}

func TestAcceptanceYouTubeDescriptionsCaptionsAndBackfill(t *testing.T) {
	var captionBrokenHits atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/feed.xml":
			w.Header().Set("Content-Type", "application/atom+xml")
			_, _ = w.Write([]byte(ytAtomFeed(srv.URL)))
		case r.URL.Path == "/channel":
			_, _ = w.Write([]byte(`<html><meta itemprop="channelId" content="UCfixture000000000000000"></html>`))
		case r.URL.Path == "/watch/VIDA000001":
			_, _ = w.Write([]byte(ytPlayerHTML(map[string]any{
				"videoId": "VIDA000001", "title": "Video A", "author": "Fixture Creator",
				"channelId": "UCfixture000000000000000", "shortDescription": "Original description A",
				"lengthSeconds": "125",
			}, []map[string]any{
				{"baseUrl": srv.URL + "/captions/auto-a?lang=en", "languageCode": "en", "kind": "asr"},
				{"baseUrl": srv.URL + "/captions/manual-a?lang=en", "languageCode": "en"},
			})))
		case r.URL.Path == "/watch/VIDB000002":
			_, _ = w.Write([]byte(ytPlayerHTML(map[string]any{
				"videoId": "VIDB000002", "title": "Video B", "author": "Fixture Creator",
				"shortDescription": "Description B with no captions", "lengthSeconds": "60",
			}, nil)))
		case r.URL.Path == "/watch/VIDC000003":
			_, _ = w.Write([]byte(ytPlayerHTML(map[string]any{
				"videoId": "VIDC000003", "title": "Video C", "author": "Fixture Creator",
				"shortDescription": "Description C", "lengthSeconds": "90",
			}, []map[string]any{
				{"baseUrl": srv.URL + "/captions/broken-c?lang=en", "languageCode": "en"},
			})))
		case r.URL.Path == "/captions/manual-a":
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":1000,"segs":[{"utf8":"Manual caption text A"}]},{"tStartMs":33000,"segs":[{"utf8":"second manual paragraph"}]}]}`))
		case r.URL.Path == "/captions/auto-a":
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":1000,"segs":[{"utf8":"Auto caption text A"}]}]}`))
		case r.URL.Path == "/captions/broken-c":
			captionBrokenHits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("caption backend down"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	yt := scraper.youtube
	yt.feedURLOverride = srv.URL + "/feed.xml"
	channelURL := "https://www.youtube.com/@fixture/videos"
	yt.videoPageURLResolver = func(raw string) string {
		switch {
		case raw == channelURL:
			return srv.URL + "/channel"
		case strings.HasPrefix(raw, "https://www.youtube.com/watch?v="):
			return srv.URL + "/watch/" + strings.TrimPrefix(raw, "https://www.youtube.com/watch?v=")
		}
		return raw
	}

	src := source.Source{ID: "yt-fixture", Name: "YT Fixture", Type: "youtube", URL: channelURL,
		Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "youtube",
		Config: `{"max_items":10,"feed_url":"` + srv.URL + `/feed.xml"}`}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("youtube scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3: %+v", len(items), items)
	}

	findByVideo := func(id string) *sqliteItemRef {
		for i := range items {
			if strings.Contains(items[i].URL, id) {
				return &sqliteItemRef{items: items, idx: i}
			}
		}
		return nil
	}
	a := findByVideo("VIDA000001")
	b := findByVideo("VIDB000002")
	c := findByVideo("VIDC000003")
	if a == nil || b == nil || c == nil {
		t.Fatalf("missing video items: %+v", items)
	}
	if a.get().URL != "https://www.youtube.com/watch?v=VIDA000001" {
		t.Fatalf("video identity not canonicalised from youtu.be: %q", a.get().URL)
	}
	if !strings.Contains(a.get().Content, "Original description A") || !strings.Contains(a.get().Content, "Manual caption text A") {
		t.Fatalf("video A content lacks description/manual captions: %q", a.get().Content)
	}
	if strings.Contains(a.get().Content, "Auto caption text A") {
		t.Fatalf("manual captions should be preferred over auto captions")
	}
	if !strings.Contains(a.get().Metadata, `"caption_status":"available"`) ||
		!strings.Contains(a.get().Metadata, `"transcript_source":"youtube_captions"`) ||
		!strings.Contains(a.get().Metadata, `"caption_language":"en"`) ||
		!strings.Contains(a.get().Metadata, `"description":"Original description A"`) {
		t.Fatalf("video A metadata = %s", a.get().Metadata)
	}
	if !strings.Contains(b.get().Content, "Description B") || strings.Contains(b.get().Content, "视频文字稿") {
		t.Fatalf("video B should keep description and no transcript: %q", b.get().Content)
	}
	if !strings.Contains(b.get().Metadata, `"caption_status":"unavailable"`) {
		t.Fatalf("video B should be confirmed no-captions: %s", b.get().Metadata)
	}
	if (!strings.Contains(c.get().Content, "Description C") && !strings.Contains(c.get().Content, "Feed summary C")) ||
		!strings.Contains(c.get().Metadata, `"caption_status":"failed"`) {
		t.Fatalf("video C should keep description and mark caption failure: %s / %q", c.get().Metadata, c.get().Content)
	}

	log := latestFetchLog(t, db, src.ID)
	if log.Status != "partial" {
		t.Fatalf("log status = %q, want partial (caption C failed)", log.Status)
	}
	entries := fetchLogItems(t, db, log.ID)
	aEntry := findLogItem(entries, "VIDA000001")
	bEntry := findLogItem(entries, "VIDB000002")
	cEntry := findLogItem(entries, "VIDC000003")
	requireTextStatus(t, aEntry, "description", "stored")
	aCap := requireTextStatus(t, aEntry, "transcript", "stored")
	if aCap.Source != "youtube_captions" || aCap.Language != "en" || aCap.Chars == 0 {
		t.Fatalf("video A caption log = %+v", aCap)
	}
	requireTextStatus(t, bEntry, "description", "stored")
	requireTextStatus(t, bEntry, "transcript", "no_public_text")
	requireTextStatus(t, cEntry, "description", "stored")
	cCap := requireTextStatus(t, cEntry, "transcript", "failed")
	if cCap.Stage != "access" || !strings.Contains(cCap.Reason, "500") {
		t.Fatalf("video C caption failure = %+v", cCap)
	}

	// Pin user state, then poll again: complete videos are skipped, the failed
	// caption is retried, and no duplicates appear.
	if err := db.MarkItemRead(a.get().ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.ToggleItemStarred(a.get().ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("second youtube scrape: %v", err)
	}
	items2 := itemsBySource(t, db, src.ID)
	if len(items2) != 3 {
		t.Fatalf("duplicate videos after repeat: %d", len(items2))
	}
	for _, it := range items2 {
		if strings.Contains(it.URL, "VIDA000001") {
			if it.ID != a.get().ID || it.ReadStatus != 1 || it.Starred != 1 {
				t.Fatalf("video A changed on repeat: %+v", it)
			}
			if !strings.Contains(it.Content, "Manual caption text A") {
				t.Fatalf("video A transcript lost on repeat: %q", it.Content)
			}
		}
	}

	// A stored old entry (description only, no caption status) is backfilled.
	oldIDs := map[string]int64{}
	for _, it := range items2 {
		oldIDs[it.URL] = it.ID
	}
	oldURL := "https://www.youtube.com/watch?v=VIDD000004"
	if _, _, err := scraper.youtube.persistYouTubeItem(item.ScrapedItem{
		SourceID: src.ID, OriginSource: "Fixture Creator", Title: "Video D (old)",
		URL: oldURL, Summary: "old summary", Content: "old body", Category: "youtube",
		Metadata: `{"video_id":"VIDD000004","description":"Old description D"}`,
	}, ItemReport{Title: "Video D (old)", URL: oldURL, Category: "youtube", Status: "stored"}, nil); err != nil {
		t.Fatalf("seed old video: %v", err)
	}
	oldItem := mustItemByURL(t, db, oldURL)
	if err := db.MarkItemRead(oldItem.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.ToggleItemStarred(oldItem.ID, 1); err != nil {
		t.Fatal(err)
	}
	// Extend the feed and player fixture with D.
	feedWithD := strings.Replace(ytAtomFeed(srv.URL), `</feed>`, `<entry>
  <id>yt:video:VIDD000004</id><yt:videoId>VIDD000004</yt:videoId>
  <title>Video D</title><link rel="alternate" href="https://www.youtube.com/watch?v=VIDD000004"/>
  <published>2026-11-18T09:00:00+00:00</published><summary>Feed summary D</summary>
</entry></feed>`, 1)
	// Re-wrap the server handler to serve D via a tiny proxy route.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			_, _ = w.Write([]byte(feedWithD))
		case "/watch/VIDD000004":
			_, _ = w.Write([]byte(ytPlayerHTML(map[string]any{
				"videoId": "VIDD000004", "title": "Video D", "author": "Fixture Creator",
				"shortDescription": "Recovered description D that is longer", "lengthSeconds": "100",
			}, []map[string]any{{"baseUrl": srv.URL + "/captions/manual-d?lang=en", "languageCode": "en"}})))
		case "/captions/manual-d":
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":1000,"segs":[{"utf8":"Recovered transcript D"}]}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("backfill youtube scrape: %v", err)
	}
	itemsWithD := itemsBySource(t, db, src.ID)
	if len(itemsWithD) != 4 {
		t.Fatalf("items after backfill = %d, want 4: %+v", len(itemsWithD), itemsWithD)
	}
	d := mustItemByURL(t, db, oldURL)
	if d.ID != oldItem.ID || d.ReadStatus != 1 || d.Starred != 1 {
		t.Fatalf("video D identity/user state lost: %+v", d)
	}
	if !strings.Contains(d.Content, "Recovered transcript D") || !strings.Contains(d.Metadata, `"caption_status":"available"`) {
		t.Fatalf("video D was not backfilled: %q / %s", d.Content, d.Metadata)
	}
	if !strings.Contains(d.Content, "Recovered description D") {
		t.Fatalf("video D description not upgraded: %q", d.Content)
	}

	// An incomplete result must not erase existing description/transcript.
	worse := item.ScrapedItem{
		SourceID: src.ID, Title: "Video D", URL: oldURL, Summary: "worse",
		Content: "# Video D\n\n## 视频简介\n\n", Category: "youtube",
		Metadata: `{"video_id":"VIDD000004","caption_status":"failed","caption_error":"temporary","description":""}`,
	}
	out, _, err := scraper.youtube.persistYouTubeItem(worse, ItemReport{
		Title: "Video D", URL: oldURL, Category: "youtube", Status: "stored",
		Texts: []source.FetchTextResult{{Kind: "transcript", Status: "failed", Stage: "access", Reason: "temporary"}},
	}, d)
	if err != nil {
		t.Fatalf("persist worse: %v", err)
	}
	if !strings.Contains(out.Content, "Recovered transcript D") || !strings.Contains(out.Content, "Recovered description D") {
		t.Fatalf("worse result overwrote stored text: %q", out.Content)
	}
	after := mustItemByURL(t, db, oldURL)
	if !strings.Contains(after.Content, "Recovered transcript D") || after.ReadStatus != 1 || after.Starred != 1 {
		t.Fatalf("worse result damaged DB row: %+v", after)
	}
	if captionBrokenHits.Load() < 2 {
		t.Fatalf("failed caption was not retried on the second run")
	}
}

// sqliteItemRef is a lightweight pointer into a slice while the test mutates
// the slice length is fixed by the fixture.
type sqliteItemRef struct {
	items []sqlite.ScrapedItem
	idx   int
}

func (r *sqliteItemRef) get() *sqlite.ScrapedItem { return &r.items[r.idx] }

func TestAcceptanceYouTubeAutoCaptionsAndChannelID(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch/AUTO000001":
			_, _ = w.Write([]byte(ytPlayerHTML(map[string]any{
				"videoId": "AUTO000001", "title": "Auto Caption Video", "author": "Fixture Creator",
				"shortDescription": "Description with auto captions", "lengthSeconds": "42",
			}, []map[string]any{
				{"baseUrl": srv.URL + "/captions/auto-only?lang=en", "languageCode": "en", "kind": "asr"},
			})))
		case "/captions/auto-only":
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":500,"segs":[{"utf8":"Auto generated caption text"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	yt := scraper.youtube
	yt.videoPageURLResolver = func(raw string) string {
		return srv.URL + "/watch/" + strings.TrimPrefix(raw, "https://www.youtube.com/watch?v=")
	}

	channelFeed, _, isSingle, err := yt.ResolveYouTubeFeedURL(context.Background(),
		"https://www.youtube.com/channel/UC1234567890123456789012")
	if err != nil || isSingle || channelFeed != "https://www.youtube.com/feeds/videos.xml?channel_id=UC1234567890123456789012" {
		t.Fatalf("channel ID resolution = %q single=%v err=%v", channelFeed, isSingle, err)
	}

	it, report, err := yt.fetchYouTubeVideo(context.Background(),
		"https://www.youtube.com/watch?v=AUTO000001", "auto-src", false, "", nil, "")
	if err != nil {
		t.Fatalf("fetch auto video: %v", err)
	}
	if !strings.Contains(it.Content, "Auto generated caption text") {
		t.Fatalf("auto caption text missing: %q", it.Content)
	}
	if !strings.Contains(it.Metadata, `"transcript_source":"youtube_auto_captions"`) ||
		!strings.Contains(it.Metadata, `"caption_status":"available"`) ||
		!strings.Contains(it.Metadata, `"caption_language":"en"`) {
		t.Fatalf("auto caption metadata = %s", it.Metadata)
	}
	cap := findText(&source.FetchLogItem{Texts: report.Texts}, "transcript")
	if cap == nil || cap.Source != "youtube_auto_captions" || cap.Status != "stored" {
		t.Fatalf("auto caption report = %+v", report.Texts)
	}
}
