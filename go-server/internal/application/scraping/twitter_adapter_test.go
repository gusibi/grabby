package scraping

import (
	"context"
	"strings"
	"testing"

	"go-server/internal/application/twitter"
	"go-server/internal/domain/source"
)

func TestResolveTwitterSourceTarget(t *testing.T) {
	cases := []struct {
		url      string
		defKind  string
		wantMode string
		wantQ    string
		wantKind string
	}{
		{"https://x.com/home", "", "timeline", "", "for_you"},
		{"https://x.com/home?kind=following", "", "timeline", "", "following"},
		{"https://twitter.com/home", "following", "timeline", "", "following"},
		{"https://x.com/search?q=ai+agents&f=top", "", "search", "ai agents", "for_you"},
		{"https://x.com/i/history/likes", "", "likes", "", "for_you"},
	}
	for _, tc := range cases {
		mode, q, kind := resolveTwitterSourceTarget(tc.url, tc.defKind)
		if mode != tc.wantMode || q != tc.wantQ || kind != tc.wantKind {
			t.Errorf("resolveTwitterSourceTarget(%q, %q) = (%q, %q, %q), want (%q, %q, %q)",
				tc.url, tc.defKind, mode, q, kind, tc.wantMode, tc.wantQ, tc.wantKind)
		}
	}
}

func TestTwitterAdapterFetchAndDedupe(t *testing.T) {
	db, _ := newBenchmarkDB(t)
	s := newFastScraper(db)

	tweets := []twitter.Tweet{
		{
			ID:            "1001",
			Text:          "Just shipped a local-first RSS + X reader!",
			Author:        "alice",
			AuthorName:    "Alice",
			CreatedAt:     "Wed Nov 25 10:00:00 +0000 2026",
			FavoriteCount: 42,
			RetweetCount:  7,
			URL:           "https://x.com/alice/status/1001",
			Media:         []string{"https://pbs.twimg.com/media/demo.jpg"},
		},
		{
			ID:         "1002",
			Text:       "Second timeline post",
			Author:     "bob",
			AuthorName: "Bob",
			URL:        "https://x.com/bob/status/1002",
		},
	}

	s.twitter.fetchFn = func(ctx context.Context, src source.Source, cfg TwitterConfig) ([]twitter.Tweet, error) {
		if cfg.MaxItems != 25 {
			t.Errorf("cfg.MaxItems = %d, want 25", cfg.MaxItems)
		}
		return tweets, nil
	}

	src := source.Source{
		ID:       "x-home",
		Name:     "X 推荐时间线",
		Type:     "twitter",
		URL:      "https://x.com/home",
		Schedule: "0 * * * *",
		Enabled:  1,
		Config:   `{"max_items":25}`,
	}
	insertTestSource(t, db, src)

	added, err := s.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("first ScrapeSource failed: %v", err)
	}
	if added != 2 {
		t.Fatalf("first ScrapeSource added = %d, want 2", added)
	}

	stored, err := db.GetScrapedItemFullByURL("https://x.com/alice/status/1001")
	if err != nil || stored == nil {
		t.Fatalf("stored tweet not found: %v", err)
	}
	if stored.Category != "tweet" {
		t.Errorf("category = %q, want tweet", stored.Category)
	}
	if stored.OriginSource != "X · @alice" {
		t.Errorf("origin_source = %q, want X · @alice", stored.OriginSource)
	}
	if !strings.Contains(stored.Content, "![](https://pbs.twimg.com/media/demo.jpg)") {
		t.Errorf("content missing media markdown: %q", stored.Content)
	}
	if !strings.Contains(stored.Content, "点赞 42") {
		t.Errorf("content missing engagement stats: %q", stored.Content)
	}

	// Second run with the same tweets should keep existing and add 0 new items.
	added2, err := s.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("second ScrapeSource failed: %v", err)
	}
	if added2 != 0 {
		t.Fatalf("second ScrapeSource added = %d, want 0", added2)
	}
}

func TestTwitterAdapterAutoFetchesXArticleAndExternalLink(t *testing.T) {
	db, _ := newBenchmarkDB(t)
	s := newFastScraper(db)

	longBody := strings.Repeat("This is a full-length technical article explaining local-first systems. ", 5)

	s.twitter.fetchFn = func(ctx context.Context, src source.Source, cfg TwitterConfig) ([]twitter.Tweet, error) {
		return []twitter.Tweet{
			{
				ID:           "4001",
				Text:         "https://x.com/i/article/777",
				Author:       "writer",
				AuthorName:   "Writer",
				URL:          "https://x.com/writer/status/4001",
				IsArticle:    true,
				ArticleTitle: "Deep Dive on Local AI",
				ArticleURL:   "https://x.com/i/article/777",
			},
			{
				ID:         "4002",
				Text:       "Great post worth reading: https://blog.example.com/post",
				Author:     "curator",
				AuthorName: "Curator",
				URL:        "https://x.com/curator/status/4002",
				Links:      []string{"https://blog.example.com/post"},
			},
		}, nil
	}

	var fetched []string
	s.twitter.fetchArticleFn = func(ctx context.Context, targetURL string, preferBrowser bool) (string, string, error) {
		fetched = append(fetched, targetURL)
		if targetURL == "https://x.com/i/article/777" {
			if !preferBrowser {
				t.Errorf("X Article should prefer browser fetch")
			}
			return "Deep Dive on Local AI", longBody, nil
		}
		if preferBrowser {
			t.Errorf("external link should try HTTP before browser")
		}
		return "External Post Title", longBody, nil
	}

	src := source.Source{
		ID:       "x-articles",
		Name:     "X 推荐",
		Type:     "twitter",
		URL:      "https://x.com/home",
		Schedule: "0 * * * *",
		Enabled:  1,
	}
	insertTestSource(t, db, src)

	added, err := s.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("ScrapeSource failed: %v", err)
	}
	if added != 2 {
		t.Fatalf("added = %d, want 2", added)
	}
	if len(fetched) != 2 {
		t.Fatalf("expected 2 article fetches, got %v", fetched)
	}

	xArt, err := db.GetScrapedItemFullByURL("https://x.com/writer/status/4001")
	if err != nil || xArt == nil {
		t.Fatalf("X Article item missing: %v", err)
	}
	if xArt.Category != "article" {
		t.Errorf("X Article category = %q, want article", xArt.Category)
	}
	if !strings.Contains(xArt.Content, "## Deep Dive on Local AI") || !strings.Contains(xArt.Content, "local-first systems") {
		t.Errorf("X Article content missing full body: %q", xArt.Content)
	}

	extItem, err := db.GetScrapedItemFullByURL("https://x.com/curator/status/4002")
	if err != nil || extItem == nil {
		t.Fatalf("external link tweet missing: %v", err)
	}
	if !strings.Contains(extItem.Content, "### 附带文章：External Post Title") || !strings.Contains(extItem.Content, "local-first systems") {
		t.Errorf("external link tweet missing appended article body: %q", extItem.Content)
	}
}

func TestTwitterAdapterFiltersByMinLikesAndMinViews(t *testing.T) {
	db, _ := newBenchmarkDB(t)
	s := newFastScraper(db)

	s.twitter.fetchFn = func(ctx context.Context, src source.Source, cfg TwitterConfig) ([]twitter.Tweet, error) {
		return []twitter.Tweet{
			{ID: "5001", Text: "Low likes junk", Author: "u1", URL: "https://x.com/u1/status/5001", FavoriteCount: 5, ViewCount: 500, Links: []string{"https://example.com/should-not-fetch"}},
			{ID: "5002", Text: "Low views junk", Author: "u2", URL: "https://x.com/u2/status/5002", FavoriteCount: 50, ViewCount: 80},
			{ID: "5003", Text: "High quality tweet", Author: "u3", URL: "https://x.com/u3/status/5003", FavoriteCount: 35, ViewCount: 1200},
		}, nil
	}

	articleFetchCalled := false
	s.twitter.fetchArticleFn = func(ctx context.Context, targetURL string, preferBrowser bool) (string, string, error) {
		articleFetchCalled = true
		return "", "", nil
	}

	src := source.Source{
		ID:       "x-filtered",
		Name:     "X 精选",
		Type:     "twitter",
		URL:      "https://x.com/home",
		Schedule: "0 * * * *",
		Enabled:  1,
		Config:   `{"min_likes":20,"min_views":100}`,
	}
	insertTestSource(t, db, src)

	added, err := s.ScrapeSource(context.Background(), src)
	if err != nil {
		t.Fatalf("ScrapeSource failed: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if articleFetchCalled {
		t.Fatalf("filtered-out tweet should not trigger external article fetch")
	}

	kept, err := db.GetScrapedItemFullByURL("https://x.com/u3/status/5003")
	if err != nil || kept == nil {
		t.Fatalf("qualifying tweet not stored: %v", err)
	}
	if !strings.Contains(kept.Content, "阅读 1200") {
		t.Errorf("kept tweet missing view count in content: %q", kept.Content)
	}
	if dropped, _ := db.GetScrapedItemFullByURL("https://x.com/u1/status/5001"); dropped != nil {
		t.Errorf("low-likes tweet should have been skipped")
	}
	if dropped, _ := db.GetScrapedItemFullByURL("https://x.com/u2/status/5002"); dropped != nil {
		t.Errorf("low-views tweet should have been skipped")
	}
}
