package scraping

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

func TestAcceptancePodcastShowNotesTranscriptAndNoMediaDownload(t *testing.T) {
	var audioHits atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".mp3") {
			audioHits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch {
		case r.URL.Path == "/podcast.xml":
			longNotes := strings.Repeat("Show notes paragraph for the fixture episode. ", 12)
			feed := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:podcast="https://podcastindex.org/namespace/1.0">
<channel><title>Fixture Podcast</title>
<item><title>Episode One</title><link>%[1]s/ep1</link>
  <description><![CDATA[<p>Show notes for episode one.</p>]]></description>
  <enclosure url="%[1]s/audio/ep1.mp3" type="audio/mpeg" length="1024"/>
  <itunes:duration>12:30</itunes:duration>
  <podcast:transcript url="%[1]s/transcripts/ep1.srt" type="application/srt"/>
</item>
<item><title>Episode Two</title><link>%[1]s/ep2</link>
  <description><![CDATA[<p>%[2]s</p>]]></description>
  <enclosure url="%[1]s/audio/ep2.mp3" type="audio/mpeg" length="1024"/>
  <itunes:duration>20:00</itunes:duration>
</item>
<item><title>Episode Three</title><link>%[1]s/ep3</link>
  <description><![CDATA[<p>Show notes for episode three.</p>]]></description>
  <enclosure url="%[1]s/audio/ep3.mp3" type="audio/mpeg" length="1024"/>
  <podcast:transcript url="%[1]s/transcripts/missing.srt" type="application/srt"/>
</item>
<item><title>Episode Four</title><link>%[1]s/ep4</link>
  <description><![CDATA[<p>SHOW NOTES</p><p>Find a full transcript for this episode ✔️ <a href="%[1]s/transcript-page">here</a></p>]]></description>
  <enclosure url="%[1]s/audio/ep4.mp3" type="audio/mpeg" length="1024"/>
</item>
</channel></rss>`, srv.URL, longNotes)
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(feed))
		case r.URL.Path == "/transcripts/ep1.srt":
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:05,000\nTranscript one line.\n\n2\n00:00:06,000 --> 00:00:10,000\nSecond transcript line.\n"))
		case r.URL.Path == "/transcript-page":
			_, _ = w.Write([]byte(articleHTML("Episode Four Transcript", longText("TRANSCRIPT PAGE BODY", 24))))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, _ := newBenchmarkDB(t)
	scraper := newFastScraper(db)
	src := source.Source{ID: "pod-fixture", Name: "Fixture Podcast", Type: "podcast",
		URL: srv.URL + "/podcast.xml", Schedule: "0 * * * *", Enabled: 1,
		DefaultCategory: "podcast", Config: `{"max_items":10}`}
	insertTestSource(t, db, src)

	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("podcast scrape: %v", err)
	}
	items := itemsBySource(t, db, src.ID)
	if len(items) != 4 {
		t.Fatalf("episodes = %d, want 4: %+v", len(items), items)
	}
	find := func(suffix string) *sqlite.ScrapedItem {
		for i := range items {
			if strings.HasSuffix(items[i].URL, suffix) {
				return &items[i]
			}
		}
		return nil
	}
	ep1, ep2, ep3, ep4 := find("/ep1"), find("/ep2"), find("/ep3"), find("/ep4")
	if ep1 == nil || ep2 == nil || ep3 == nil || ep4 == nil {
		t.Fatalf("missing episodes: %+v", items)
	}
	if !strings.Contains(ep1.Content, "Show notes for episode one") || !strings.Contains(ep1.Content, "Transcript one line") {
		t.Fatalf("episode one content lacks notes/transcript: %q", ep1.Content)
	}
	if !strings.Contains(ep1.Metadata, `"transcript_status":"available"`) ||
		!strings.Contains(ep1.Metadata, `"transcript_source":"podcast_transcript"`) ||
		!strings.Contains(ep1.Metadata, srv.URL+"/transcripts/ep1.srt") {
		t.Fatalf("episode one metadata = %s", ep1.Metadata)
	}
	if !strings.Contains(ep1.Metadata, `"media_url":"`+srv.URL+`/audio/ep1.mp3"`) {
		t.Fatalf("episode audio link not stored: %s", ep1.Metadata)
	}
	if !strings.Contains(ep2.Content, "Show notes paragraph") || strings.Contains(ep2.Content, "## 播客文字稿") {
		t.Fatalf("episode two should keep notes and omit transcript: %q", ep2.Content)
	}
	if !strings.Contains(ep2.Metadata, `"transcript_status":"no_public_text"`) {
		t.Fatalf("episode two transcript status = %s", ep2.Metadata)
	}
	if !strings.Contains(ep3.Content, "Show notes for episode three") || strings.Contains(ep3.Content, "## 播客文字稿") {
		t.Fatalf("episode three should keep notes and omit transcript: %q", ep3.Content)
	}
	if !strings.Contains(ep3.Metadata, `"transcript_status":"failed"`) || !strings.Contains(ep3.Metadata, "404") {
		t.Fatalf("episode three transcript failure missing: %s", ep3.Metadata)
	}
	if !strings.Contains(ep4.Content, "TRANSCRIPT PAGE BODY") ||
		!strings.Contains(ep4.Metadata, `"transcript_source":"shownotes_link"`) ||
		!strings.Contains(ep4.Metadata, `"transcript_status":"available"`) ||
		!strings.Contains(ep4.Metadata, srv.URL+"/transcript-page") {
		t.Fatalf("episode four show-notes transcript not captured: %s / %q", ep4.Metadata, ep4.Content)
	}

	log := latestFetchLog(t, db, src.ID)
	if log.Status != "partial" {
		t.Fatalf("log status = %q, want partial (episode three transcript failed)", log.Status)
	}
	entries := fetchLogItems(t, db, log.ID)
	e1 := findLogItem(entries, "/ep1")
	e2 := findLogItem(entries, "/ep2")
	e3 := findLogItem(entries, "/ep3")
	requireTextStatus(t, e1, "show_notes", "stored")
	audio := requireTextStatus(t, e1, "audio", "stored")
	if !strings.Contains(audio.URL, "/audio/ep1.mp3") {
		t.Fatalf("audio log link = %q", audio.URL)
	}
	tr1 := requireTextStatus(t, e1, "transcript", "stored")
	if tr1.Source != "podcast_transcript" || !strings.Contains(tr1.URL, "/transcripts/ep1.srt") {
		t.Fatalf("episode one transcript log = %+v", tr1)
	}
	requireTextStatus(t, e2, "show_notes", "stored")
	requireTextStatus(t, e2, "audio", "stored")
	requireTextStatus(t, e2, "transcript", "no_public_text")
	requireTextStatus(t, e3, "show_notes", "stored")
	tr3 := requireTextStatus(t, e3, "transcript", "failed")
	if tr3.Stage != "access" || !strings.Contains(tr3.Reason, "404") {
		t.Fatalf("episode three transcript log = %+v", tr3)
	}
	if audioHits.Load() != 0 {
		t.Fatalf("scraping downloaded %d audio files; it must never fetch media", audioHits.Load())
	}

	// Repeated polling must not duplicate episodes or lose the audio link.
	if _, err := scrapeFresh(t, db, scraper, src.ID, "manual"); err != nil {
		t.Fatalf("second podcast scrape: %v", err)
	}
	if n := len(itemsBySource(t, db, src.ID)); n != 4 {
		t.Fatalf("episodes after repeat = %d, want 4", n)
	}
	again := mustItemByURL(t, db, srv.URL+"/ep1")
	if !strings.Contains(again.Metadata, `"media_url":"`+srv.URL+`/audio/ep1.mp3"`) ||
		!strings.Contains(again.Content, "Transcript one line") {
		t.Fatalf("repeat run damaged episode one: %+v", again)
	}
	if audioHits.Load() != 0 {
		t.Fatalf("repeat run downloaded media")
	}
}
