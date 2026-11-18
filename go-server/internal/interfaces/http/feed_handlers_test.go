package httpiface

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/scraping"
	"go-server/internal/infrastructure/sqlite"
)

func TestFeedHandlers_UnifiedFeedPicksIngestAndASREnv(t *testing.T) {
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	for _, s := range []sqlite.Source{
		{ID: "news-src", Name: "News", Type: "rss", URL: "https://example.com/news.xml", Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "article", Category: "AI", Config: "{}"},
		{ID: "yt-src", Name: "YT", Type: "youtube", URL: "https://youtube.com/@test", Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "youtube", Category: "AI", Config: "{}"},
		{ID: "pod-src", Name: "Pod", Type: "podcast", URL: "https://example.com/pod.xml", Schedule: "0 * * * *", Enabled: 1, DefaultCategory: "podcast", Category: "科技", Config: "{}"},
	} {
		if err := db.InsertSource(s); err != nil {
			t.Fatalf("InsertSource(%s): %v", s.ID, err)
		}
	}

	now := time.Now()
	newsID, _ := db.InsertScrapedItem(sqlite.ScrapedItem{
		SourceID: "news-src", OriginSource: "NewsOrigin", Title: "AI News Article", URL: "https://example.com/n1",
		Summary: "news sum", Content: "Full news markdown body", Category: "article", PublishedAt: &now,
	})
	ytID, _ := db.InsertScrapedItem(sqlite.ScrapedItem{
		SourceID: "yt-src", OriginSource: "YTCreator", Title: "YouTube Talk", URL: "https://youtube.com/watch?v=12345678901",
		Summary: "yt sum", Content: "YouTube transcript text", Category: "youtube", PublishedAt: &now,
		Metadata: `{"video_id":"12345678901","duration":"12:30"}`,
	})
	podID, _ := db.InsertScrapedItem(sqlite.ScrapedItem{
		SourceID: "pod-src", OriginSource: "PodShow", Title: "Podcast Ep 1", URL: "https://example.com/ep1",
		Summary: "pod sum", Content: "Podcast transcript text", Category: "podcast", PublishedAt: &now,
		Metadata: `{"duration":"45:00","transcript_source":"asr"}`,
	})

	_ = db.InsertAIAnalysis(sqlite.AIAnalysis{ItemID: newsID, AICategory: "AI", QualityScore: 8, AISummary: "AI news summary"})
	_ = db.InsertAIAnalysis(sqlite.AIAnalysis{ItemID: ytID, AICategory: "AI", QualityScore: 9, AISummary: "AI yt summary"})
	_ = db.InsertAIAnalysis(sqlite.AIAnalysis{ItemID: podID, AICategory: "科技", QualityScore: 7, AISummary: "Pod summary"})

	_ = db.InsertAIDailyReport(sqlite.AIDailyReport{
		ReportDate: now.Format("2006-01-02"),
		ReportType: "daily",
		Title:      "Today Daily",
		Content:    `{"title":"Today Daily","date":"` + now.Format("2006-01-02") + `","editor":"Grabby","sections":{"headline":{"title":"Top","items":[]}}}`,
	})

	scraper := scraping.NewScraper(db, nil, nil, zap.NewNop(), nil)
	h := &feedHandlers{deps: Dependencies{DB: db, Scraper: scraper, Logger: zap.NewNop()}}
	e := echo.New()

	// 1. GET /open/api/feed?type=youtube
	req := httptest.NewRequest(http.MethodGet, "/open/api/feed?type=youtube", nil)
	rec := httptest.NewRecorder()
	if err := h.handleFeed(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handleFeed youtube: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var feedResp struct {
		Success bool              `json:"success"`
		Type    string            `json:"type"`
		Count   int               `json:"count"`
		Items   []UnifiedFeedItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &feedResp); err != nil {
		t.Fatalf("decode feedResp: %v", err)
	}
	if feedResp.Count != 1 || feedResp.Items[0].Type != "youtube" || feedResp.Items[0].Metadata["video_id"] != "12345678901" {
		t.Fatalf("unexpected youtube feed response: %+v", feedResp)
	}

	// 2. GET /open/api/feed?type=news&include_content=false
	req = httptest.NewRequest(http.MethodGet, "/open/api/feed?type=news&include_content=false", nil)
	rec = httptest.NewRecorder()
	if err := h.handleFeed(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handleFeed news: %v", err)
	}
	var newsResp struct {
		Success bool              `json:"success"`
		Type    string            `json:"type"`
		Count   int               `json:"count"`
		Items   []UnifiedFeedItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &newsResp); err != nil {
		t.Fatalf("decode newsResp: %v", err)
	}
	if newsResp.Count != 1 || newsResp.Items[0].Type != "news" || newsResp.Items[0].Content != "" {
		t.Fatalf("unexpected news feed response: %+v", newsResp)
	}

	// 3. GET /open/api/feed/picks
	req = httptest.NewRequest(http.MethodGet, "/open/api/feed/picks?score_min=7&limit=10", nil)
	rec = httptest.NewRecorder()
	if err := h.handleFeedPicks(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handleFeedPicks: %v", err)
	}
	var picksResp struct {
		Success     bool                         `json:"success"`
		Count       int                          `json:"count"`
		Items       []UnifiedFeedItem            `json:"items"`
		ByType      map[string][]UnifiedFeedItem `json:"by_type"`
		DailyReport map[string]any               `json:"daily_report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &picksResp); err != nil {
		t.Fatalf("decode picksResp: %v", err)
	}
	if picksResp.Count != 3 || picksResp.Items[0].QualityScore != 9 {
		t.Fatalf("unexpected picks order/count: %+v", picksResp)
	}
	if len(picksResp.ByType["youtube"]) != 1 || len(picksResp.ByType["podcast"]) != 1 || len(picksResp.ByType["news"]) != 1 {
		t.Fatalf("unexpected by_type grouping: %+v", picksResp.ByType)
	}
	if picksResp.DailyReport == nil || picksResp.DailyReport["title"] != "Today Daily" {
		t.Fatalf("expected daily_report in picks response, got %+v", picksResp.DailyReport)
	}

	// 4. POST /open/api/feed/ingest
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Realtime Article</title></head><body><article><p>` +
			strings.Repeat("Realtime extracted paragraph content for testing ingest. ", 8) +
			`</p></article></body></html>`))
	}))
	defer pageSrv.Close()

	bodyBytes, _ := json.Marshal(map[string]any{"url": pageSrv.URL + "/post", "type": "blog", "analyze": false})
	req = httptest.NewRequest(http.MethodPost, "/open/api/feed/ingest", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := h.handleFeedIngest(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handleFeedIngest: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ingest status = %d body=%s", rec.Code, rec.Body.String())
	}
	var ingestResp struct {
		Success bool            `json:"success"`
		Item    UnifiedFeedItem `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ingestResp); err != nil {
		t.Fatalf("decode ingestResp: %v", err)
	}
	if !ingestResp.Success || ingestResp.Item.Type != "blog" || !strings.Contains(ingestResp.Item.Content, "Realtime extracted paragraph") {
		t.Fatalf("unexpected ingestResp: %+v", ingestResp)
	}

	// 5. GET /open/api/asr/env
	req = httptest.NewRequest(http.MethodGet, "/open/api/asr/env", nil)
	rec = httptest.NewRecorder()
	if err := h.handleASREnv(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handleASREnv: %v", err)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"install_command"`) {
		t.Fatalf("unexpected asr env response: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
