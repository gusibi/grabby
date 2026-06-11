package scraping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
)

// Scraper coordinates scraping from different source types.
type Scraper struct {
	db        *sqlite.Database
	wsManager *browserws.WebSocketManager
	taskQueue *TaskQueue
	logger    *zap.Logger
	aiEngine  *appai.AIEngine
}

func NewScraper(db *sqlite.Database, wsManager *browserws.WebSocketManager, taskQueue *TaskQueue, logger *zap.Logger, aiEngine *appai.AIEngine) *Scraper {
	return &Scraper{
		db:        db,
		wsManager: wsManager,
		taskQueue: taskQueue,
		logger:    logger,
		aiEngine:  aiEngine,
	}
}

// ScrapeSource triggers the scrape job for a source, including retry logic.
func (s *Scraper) ScrapeSource(ctx context.Context, src source.Source) (int, error) {
	s.logger.Info("Starting scrape for source", zap.String("id", src.ID), zap.String("type", src.Type))

	startedAt := time.Now()
	// Insert fetch log in DB as running
	logID, err := s.db.InsertFetchLog(source.FetchLog{
		SourceID:  src.ID,
		StartedAt: startedAt,
		Status:    "running",
	})
	if err != nil {
		s.logger.Error("Failed to insert fetch log", zap.Error(err))
	}

	var count int
	var scrapeErr error

	scrapeFunc := func() (int, error) {
		switch src.Type {
		case "rss":
			return s.ScrapeRSS(src)
		case "api":
			return s.ScrapeAPI(src)
		case "web_scrape":
			return s.ScrapeWeb(src, logID)
		default:
			return 0, fmt.Errorf("unsupported source type: %s", src.Type)
		}
	}

	// Retry wrapper with exponential backoff (max 3 retries)
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		count, scrapeErr = scrapeFunc()
		if scrapeErr == nil {
			break
		}
		if attempt < maxRetries {
			backoff := time.Duration(5*math.Pow(3, float64(attempt-1))) * time.Second
			s.logger.Warn("Scrape failed, retrying...",
				zap.String("source", src.ID),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(scrapeErr),
			)
			select {
			case <-ctx.Done():
				scrapeErr = ctx.Err()
				break
			case <-time.After(backoff):
			}
		}
	}

	finishedAt := time.Now()
	status := "success"
	errMsg := ""
	if scrapeErr != nil {
		status = "error"
		errMsg = scrapeErr.Error()
		s.logger.Error("Scrape finished with error", zap.String("source", src.ID), zap.Error(scrapeErr))
	} else {
		s.logger.Info("Scrape finished successfully", zap.String("source", src.ID), zap.Int("added", count))
	}

	// For web_scrape, the tasks run asynchronously in the task queue.
	// So we don't mark the log as completed success/error immediately;
	// the task queue workers will update it as tasks finish.
	// However, if we fail to enqueue or start Stage 1, we mark it error now.
	if src.Type != "web_scrape" || scrapeErr != nil {
		if logID > 0 {
			s.db.UpdateFetchLog(source.FetchLog{
				ID:           logID,
				SourceID:     src.ID,
				StartedAt:    startedAt,
				FinishedAt:   &finishedAt,
				Status:       status,
				ItemsFound:   count,
				ItemsAdded:   count,
				ErrorMessage: errMsg,
			})
		}
		// Update source last status
		_ = s.db.UpdateSourceFetchStatus(src.ID, status, src.LastETag, src.LastModified)
	}

	return count, scrapeErr
}

// ScrapeRSS fetches and parses RSS/Atom feeds.
func (s *Scraper) ScrapeRSS(src source.Source) (int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", src.URL, nil)
	if err != nil {
		return 0, err
	}

	// Conditional GET headers
	if src.LastETag != nil && *src.LastETag != "" {
		req.Header.Set("If-None-Match", *src.LastETag)
	}
	if src.LastModified != nil && *src.LastModified != "" {
		req.Header.Set("If-Modified-Since", *src.LastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		s.logger.Info("RSS feed returned 304 Not Modified", zap.String("id", src.ID))
		return 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP error code: %d", resp.StatusCode)
	}

	fp := gofeed.NewParser()
	feed, err := fp.Parse(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to parse feed: %w", err)
	}

	classifier := NewClassifier()
	var newETag *string
	var newLastMod *string

	if etag := resp.Header.Get("ETag"); etag != "" {
		newETag = &etag
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		newLastMod = &lm
	}

	// Update LastETag and LastModified on the source
	if newETag != nil || newLastMod != nil {
		src.LastETag = newETag
		src.LastModified = newLastMod
	}

	itemsAdded := 0
	for _, feedItem := range feed.Items {
		urlStr := feedItem.Link
		if urlStr == "" {
			continue
		}

		category := classifier.Classify(urlStr, src.DefaultCategory)
		origin := classifier.ExtractOrigin(urlStr, feed.Title)

		scrapedItem := item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: origin,
			Title:        feedItem.Title,
			URL:          urlStr,
			Summary:      feedItem.Description,
			Content:      feedItem.Content,
			Category:     category,
		}

		if scrapedItem.Content == "" {
			scrapedItem.Content = feedItem.Description // fallback
		}

		if feedItem.PublishedParsed != nil {
			scrapedItem.PublishedAt = feedItem.PublishedParsed
		} else {
			now := time.Now()
			scrapedItem.PublishedAt = &now
		}

		id, err := s.db.InsertScrapedItem(scrapedItem)
		if err != nil {
			s.logger.Error("Failed to insert RSS item", zap.Error(err))
			continue
		}
		if id > 0 {
			itemsAdded++
			s.aiEngine.Enqueue(id)
		}
	}

	return itemsAdded, nil
}

type APIConfig struct {
	ResponsePath   string            `json:"response_path"`
	TitleField     string            `json:"title_field"`
	URLField       string            `json:"url_field"`
	SummaryField   string            `json:"summary_field"`
	SourceField    string            `json:"source_field"`
	PublishedField string            `json:"published_field"`
	Headers        map[string]string `json:"headers"`
}

// ScrapeAPI scrapes structured data from JSON APIs.
func (s *Scraper) ScrapeAPI(src source.Source) (int, error) {
	var config APIConfig
	if src.Config != "" {
		_ = json.Unmarshal([]byte(src.Config), &config)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", src.URL, nil)
	if err != nil {
		return 0, err
	}

	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP error code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var jsonResp any
	if err := json.Unmarshal(bodyBytes, &jsonResp); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var dataItems []any
	if config.ResponsePath != "" {
		fieldVal, ok := getJSONField(jsonResp, config.ResponsePath)
		if !ok {
			return 0, fmt.Errorf("response path '%s' not found in JSON response", config.ResponsePath)
		}
		sliceItems, ok := fieldVal.([]any)
		if !ok {
			return 0, fmt.Errorf("response path '%s' is not a list", config.ResponsePath)
		}
		dataItems = sliceItems
	} else {
		sliceItems, ok := jsonResp.([]any)
		if !ok {
			return 0, errors.New("response root is not an array")
		}
		dataItems = sliceItems
	}

	classifier := NewClassifier()
	itemsAdded := 0

	for _, rawItem := range dataItems {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		title, _ := itemMap[firstNonEmpty(config.TitleField, "title")].(string)
		itemURL, _ := itemMap[firstNonEmpty(config.URLField, "url")].(string)
		summary, _ := itemMap[firstNonEmpty(config.SummaryField, "summary")].(string)
		sourceVal, _ := itemMap[firstNonEmpty(config.SourceField, "source")].(string)

		if itemURL == "" || title == "" {
			continue
		}

		category := classifier.Classify(itemURL, src.DefaultCategory)
		origin := classifier.ExtractOrigin(itemURL, sourceVal)

		scrapedItem := item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: origin,
			Title:        title,
			URL:          itemURL,
			Summary:      summary,
			Content:      summary,
			Category:     category,
		}

		pubField := firstNonEmpty(config.PublishedField, "published_at", "created_at")
		if pubVal, ok := itemMap[pubField]; ok {
			var parsedTime time.Time
			var timeErr error
			switch val := pubVal.(type) {
			case string:
				parsedTime, timeErr = time.Parse(time.RFC3339, val)
				if timeErr != nil {
					parsedTime, timeErr = time.Parse("2006-01-02 15:04:05", val)
				}
			case float64:
				parsedTime = time.Unix(int64(val), 0)
			}
			if timeErr == nil && !parsedTime.IsZero() {
				scrapedItem.PublishedAt = &parsedTime
			}
		}

		if scrapedItem.PublishedAt == nil {
			now := time.Now()
			scrapedItem.PublishedAt = &now
		}

		id, err := s.db.InsertScrapedItem(scrapedItem)
		if err != nil {
			s.logger.Error("Failed to insert API item", zap.Error(err))
			continue
		}
		if id > 0 {
			itemsAdded++
			s.aiEngine.Enqueue(id)
		}
	}

	return itemsAdded, nil
}

type WebConfig struct {
	ListSelector string `json:"list_selector"`
	MaxItems     int    `json:"max_items"`
	Concurrency  int    `json:"concurrency"`
}

var mdLinkRegex = regexp.MustCompile(`\[[^\]]+\]\((https?://[^)]+)\)`)
var htmlLinkRegex = regexp.MustCompile(`href=["'](https?://[^"']+)["']`)

func extractLinks(content string) []string {
	var links []string
	seen := make(map[string]bool)

	// Extract Markdown style links [text](url)
	mdMatches := mdLinkRegex.FindAllStringSubmatch(content, -1)
	for _, m := range mdMatches {
		if len(m) > 1 {
			u := m[1]
			if !seen[u] {
				seen[u] = true
				links = append(links, u)
			}
		}
	}

	// Extract HTML style links href="url"
	htmlMatches := htmlLinkRegex.FindAllStringSubmatch(content, -1)
	for _, m := range htmlMatches {
		if len(m) > 1 {
			u := m[1]
			if !seen[u] {
				seen[u] = true
				links = append(links, u)
			}
		}
	}

	return links
}

// ScrapeWeb enqueues page urls to the TaskQueue for async extraction.
func (s *Scraper) ScrapeWeb(src source.Source, logID int64) (int, error) {
	var config WebConfig
	if src.Config != "" {
		_ = json.Unmarshal([]byte(src.Config), &config)
	}
	if config.MaxItems <= 0 {
		config.MaxItems = 20
	}

	// Check if any browser is connected
	browserConnID, err := s.wsManager.ResolveBrowserConnID("")
	if err != nil {
		return 0, fmt.Errorf("no browser connected: %w", err)
	}

	var urlsToScrape []string

	if config.ListSelector != "" {
		s.logger.Info("ScrapeWeb Stage 1: Extracting list page links", zap.String("url", src.URL))
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		resp, err := s.wsManager.SendMessage(ctx, &capture.BrowserRequest{
			Source:  "scheduler_scrapers",
			Action:  "mcp_request",
			Command: "extract",
			URL:     src.URL,
		}, browserConnID)
		cancel()

		if err != nil {
			return 0, fmt.Errorf("failed to extract list page: %w", err)
		}
		if !resp.Success {
			return 0, fmt.Errorf("browser extract failed: %s", resp.Error)
		}

		allLinks := extractLinks(resp.Result.Content.MarkdownContent())
		s.logger.Info("Extracted links from page markdown", zap.Int("count", len(allLinks)))

		// Filter for new links that are not in DB yet
		for _, link := range allLinks {
			// Ignore home/self links
			if link == src.URL || strings.HasSuffix(link, "/") && len(link) < len(src.URL)+2 {
				continue
			}

			exists, err := s.db.ItemExistsByURL(link)
			if err != nil {
				continue
			}
			if !exists {
				urlsToScrape = append(urlsToScrape, link)
			}

			if len(urlsToScrape) >= config.MaxItems {
				break
			}
		}
	} else {
		// Scrape the url directly
		exists, err := s.db.ItemExistsByURL(src.URL)
		if err == nil && !exists {
			urlsToScrape = append(urlsToScrape, src.URL)
		}
	}

	if len(urlsToScrape) == 0 {
		s.logger.Info("No new URLs found to scrape for source", zap.String("id", src.ID))
		// Log as skipped
		s.db.MarkFetchLogSkippedSimple(logID)
		return 0, nil
	}

	s.logger.Info("Enqueuing web scrape tasks to queue", zap.Int("count", len(urlsToScrape)))

	var tasks []ScrapeTask
	for _, u := range urlsToScrape {
		tasks = append(tasks, ScrapeTask{
			SourceID: src.ID,
			URL:      u,
			LogID:    logID,
		})
	}
	s.taskQueue.Enqueue(tasks)

	return len(urlsToScrape), nil
}

// Helpers

// FirstNonEmpty returns the first non-empty string from the arguments.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// firstNonEmpty is an unexported alias for FirstNonEmpty for internal use.
func firstNonEmpty(vals ...string) string {
	return FirstNonEmpty(vals...)
}

func getJSONField(obj any, path string) (any, bool) {
	if path == "" {
		return obj, true
	}
	parts := strings.Split(path, ".")
	curr := obj
	for _, part := range parts {
		m, ok := curr.(map[string]any)
		if !ok {
			return nil, false
		}
		curr, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return curr, true
}
