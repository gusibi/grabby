package httpiface

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/infrastructure/sqlite"
)

func TestGetItemRendersEscapedMarkdownAndHTML(t *testing.T) {
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	if err := db.InsertSource(sqlite.Source{
		ID:              "test-source",
		Name:            "Test",
		Type:            "rss",
		URL:             "https://example.com/feed",
		Schedule:        "0 0 * * *",
		Enabled:         1,
		DefaultCategory: "auto",
		Config:          "{}",
	}); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	itemID, err := db.InsertScrapedItem(sqlite.ScrapedItem{
		SourceID:     "test-source",
		OriginSource: "Test",
		Title:        "Mixed content",
		URL:          "https://example.com/mixed",
		Summary:      "A **summary**",
		Content:      `&lt;p&gt;Hello &lt;strong&gt;world&lt;/strong&gt;&lt;/p&gt;&lt;p&gt;&lt;a href="https://example.com"&gt;link&lt;/a&gt;&lt;/p&gt;`,
		Category:     "article",
	})
	if err != nil {
		t.Fatalf("InsertScrapedItem: %v", err)
	}

	h := &contentHandlers{deps: Dependencies{DB: db, Logger: zap.NewNop()}}
	req := httptest.NewRequest(http.MethodGet, "/open/api/items/1", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	if err := h.getItem(c); err != nil {
		t.Fatalf("getItem: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		HTMLContent string `json:"html_content"`
		SummaryHTML string `json:"summary_html"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.Contains(body.HTMLContent, "&lt;p&gt;") {
		t.Errorf("html_content still escaped: %s", body.HTMLContent)
	}
	for _, want := range []string{"<p>Hello", "<strong>world</strong>", `<a href="https://example.com"`} {
		if !strings.Contains(body.HTMLContent, want) {
			t.Errorf("html_content = %q, missing %q", body.HTMLContent, want)
		}
	}
	if !strings.Contains(body.SummaryHTML, "<strong>summary</strong>") {
		t.Errorf("summary_html = %q, markdown was not rendered", body.SummaryHTML)
	}

	// Keep the inserted id referenced so a future schema change that returns a
	// different id is caught instead of silently testing nothing.
	if itemID <= 0 {
		t.Fatalf("InsertScrapedItem returned id %d", itemID)
	}
}

func TestListItemsFiltersBySourceAndStatsExposeCounts(t *testing.T) {
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	for _, s := range []sqlite.Source{
		{ID: "src-a", Name: "Source A", Type: "rss", URL: "https://example.com/a", Schedule: "0 * * * *", Enabled: 1, Config: "{}"},
		{ID: "src-b", Name: "Source B", Type: "rss", URL: "https://example.com/b", Schedule: "0 * * * *", Enabled: 1, Config: "{}"},
	} {
		if err := db.InsertSource(s); err != nil {
			t.Fatalf("InsertSource(%s): %v", s.ID, err)
		}
	}
	for _, it := range []sqlite.ScrapedItem{
		{SourceID: "src-a", Title: "A one", URL: "https://example.com/a1", Category: "article", Metadata: "{}"},
		{SourceID: "src-b", Title: "B one", URL: "https://example.com/b1", Category: "article", Metadata: "{}"},
	} {
		if _, err := db.InsertScrapedItem(it); err != nil {
			t.Fatalf("InsertScrapedItem(%s): %v", it.URL, err)
		}
	}

	h := &contentHandlers{deps: Dependencies{DB: db, Logger: zap.NewNop()}}
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/open/api/items?source_id=src-a", nil)
	rec := httptest.NewRecorder()
	if err := h.listItems(e.NewContext(req, rec)); err != nil {
		t.Fatalf("listItems: %v", err)
	}
	var page struct {
		Items []sqlite.ScrapedItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].SourceID != "src-a" {
		t.Fatalf("items = %+v, want only src-a", page.Items)
	}

	req = httptest.NewRequest(http.MethodGet, "/open/api/stats", nil)
	rec = httptest.NewRecorder()
	if err := h.stats(e.NewContext(req, rec)); err != nil {
		t.Fatalf("stats: %v", err)
	}
	var stats struct {
		SourceCounts []sqlite.SourceCount `json:"source_counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	byID := map[string]sqlite.SourceCount{}
	for _, c := range stats.SourceCounts {
		byID[c.ID] = c
	}
	if got := byID["src-a"]; got.Total != 1 || got.Name != "Source A" {
		t.Fatalf("source_counts[src-a] = %+v", got)
	}
	if got := byID["src-b"]; got.Total != 1 || got.Name != "Source B" {
		t.Fatalf("source_counts[src-b] = %+v", got)
	}
}
