package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"go-server/internal/application/scraping"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

func TestAcceptanceSchedulerTriggersSameScrapePath(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Cron Fixture</title>
<item><title>Cron Item</title><link>%s/article</link><description><![CDATA[<p>Cron summary</p>]]></description></item>
</channel></rss>`, srv.URL)))
	}))
	defer srv.Close()

	db, err := sqlite.NewDatabase(filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	src := source.Source{
		ID: "cron-rss", Name: "Cron RSS", Type: "rss", URL: srv.URL + "/feed.xml",
		Schedule: "@every 1s", Enabled: 1, DefaultCategory: "article", Config: `{"full_content":false}`,
	}
	if err := db.InsertSource(src); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	scraper := scraping.NewScraper(db, nil, nil, zap.NewNop(), nil)
	s := NewScheduler(db, scraper, nil, zap.NewNop())
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("scheduler Start: %v", err)
	}
	defer func() {
		ctx := s.Stop()
		<-ctx.Done()
	}()

	deadline := time.Now().Add(8 * time.Second)
	var logs []source.FetchLog
	for time.Now().Before(deadline) {
		logs, _ = db.GetFetchLogs(src.ID, 10)
		if len(logs) > 0 && logs[0].Status != "running" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(logs) == 0 {
		t.Fatalf("scheduler did not trigger a fetch within the deadline")
	}
	found := false
	for _, l := range logs {
		if l.Trigger == "scheduled" {
			found = true
			if l.Status != "success" && l.Status != "partial" {
				t.Fatalf("scheduled log status = %q", l.Status)
			}
			items, err := db.GetFetchLogItems(l.ID)
			if err != nil || len(items) == 0 {
				t.Fatalf("scheduled log has no item details: len=%d err=%v", len(items), err)
			}
			if items[0].ItemID == 0 {
				t.Fatalf("scheduled log item has no library ID: %+v", items[0])
			}
		}
	}
	if !found {
		t.Fatalf("no scheduled trigger found in logs: %+v", logs)
	}
	items, _, err := db.GetScrapedItems(sqlite.ItemsFilter{SourceID: src.ID, Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("scheduled scrape did not store the item: len=%d err=%v", len(items), err)
	}
	if items[0].Title != "Cron Item" {
		t.Fatalf("stored item = %+v", items[0])
	}
}
