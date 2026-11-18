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
	"unicode/utf8"

	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/application/transcribe"
	domainai "go-server/internal/domain/ai"
	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
)

// SourceAdapter is the single adapter interface implemented by all subscription
// source types (RSS, YouTube, Podcast, API). Every adapter returns normalized
// ScrapedItem values ready for unified storage and AI analysis.
type SourceAdapter interface {
	Fetch(ctx context.Context, src source.Source) ([]item.ScrapedItem, error)
}

type sourceRuntimeConfig struct {
	FullContent        bool  `json:"full_content"`
	FetchFullViaScrape bool  `json:"fetch_full_via_scrape"`
	TranscribeAudio    *bool `json:"transcribe_audio"`
	MaxItems           int   `json:"max_items"`
	MaxASRItems        int   `json:"max_asr_items"`
	MediaChecked       bool  `json:"media_checked"`
}

func parseSourceRuntimeConfig(raw string) sourceRuntimeConfig {
	var cfg sourceRuntimeConfig
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	return cfg
}

func (c sourceRuntimeConfig) fullContentEnabled() bool {
	return c.FullContent || c.FetchFullViaScrape
}

// transcribeAudioEnabled reports whether scraping may download media and run
// ASR. This is opt-in only: absent configuration means no audio is fetched, so
// podcast episodes are never downloaded in the background. Playback is the only
// path that pulls audio, and the player does it on demand.
func (c sourceRuntimeConfig) transcribeAudioEnabled() bool {
	return c.TranscribeAudio != nil && *c.TranscribeAudio
}

// Scraper coordinates scraping from different source types.
type Scraper struct {
	db          *sqlite.Database
	wsManager   *browserws.WebSocketManager
	taskQueue   *TaskQueue
	logger      *zap.Logger
	aiEngine    *appai.AIEngine
	transcriber *transcribe.Transcriber
	youtube     *YouTubeAdapter
	podcast     *PodcastAdapter
	adapters    map[string]SourceAdapter
}

func NewScraper(db *sqlite.Database, wsManager *browserws.WebSocketManager, taskQueue *TaskQueue, logger *zap.Logger, aiEngine *appai.AIEngine) *Scraper {
	if logger == nil {
		logger = zap.NewNop()
	}
	settingsProvider := func() domainai.AISettings {
		if aiEngine != nil {
			return aiEngine.Settings()
		}
		return domainai.NormalizeAISettings(domainai.AISettings{})
	}
	tr := transcribe.NewTranscriber(settingsProvider, logger)

	s := &Scraper{
		db:          db,
		wsManager:   wsManager,
		taskQueue:   taskQueue,
		logger:      logger,
		aiEngine:    aiEngine,
		transcriber: tr,
	}
	s.youtube = NewYouTubeAdapter(s)
	s.podcast = NewPodcastAdapter(s)
	s.adapters = map[string]SourceAdapter{
		"youtube": s.youtube,
		"podcast": s.podcast,
	}
	return s
}

// Transcriber returns the shared audio/video transcriber instance.
func (s *Scraper) Transcriber() *transcribe.Transcriber {
	return s.transcriber
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
			return s.scrapeRSSWithContext(ctx, &src, logID)
		case "api":
			return s.ScrapeAPI(src)
		case "web_scrape":
			return s.ScrapeWeb(src, logID)
		default:
			if adapter, ok := s.adapters[src.Type]; ok {
				return s.runAdapter(ctx, adapter, src)
			}
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

func (s *Scraper) runAdapter(ctx context.Context, adapter SourceAdapter, src source.Source) (int, error) {
	items, err := adapter.Fetch(ctx, src)
	if err != nil {
		return 0, err
	}
	itemsAdded := 0
	for _, it := range items {
		if it.SourceID == "" {
			it.SourceID = src.ID
		}
		if it.PublishedAt == nil {
			now := time.Now()
			it.PublishedAt = &now
		}
		id, created, err := s.db.UpsertScrapedItem(it)
		if err != nil {
			s.logger.Error("Failed to upsert adapter item", zap.String("url", it.URL), zap.Error(err))
			continue
		}
		if created {
			itemsAdded++
		}
		if id > 0 && s.aiEngine != nil && (created || utf8.RuneCountInString(it.Content) > 0) {
			s.aiEngine.Enqueue(id)
		}
	}
	return itemsAdded, nil
}

// ScrapeRSS fetches and parses RSS/Atom feeds.
func (s *Scraper) ScrapeRSS(src source.Source) (int, error) {
	return s.scrapeRSSWithContext(context.Background(), &src, 0)
}

func (s *Scraper) scrapeRSSWithContext(ctx context.Context, src *source.Source, logID int64) (int, error) {
	cfg := parseSourceRuntimeConfig(src.Config)
	// Podcast feeds added as plain RSS predate audio-enclosure capture. Run one
	// full fetch (ignoring conditional headers) so existing episodes can be
	// reclassified as podcasts and made playable.
	needsMediaScan := !cfg.MediaChecked

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Grabby/2.0)")

	// Conditional GET headers
	if !needsMediaScan && src.LastETag != nil && *src.LastETag != "" {
		req.Header.Set("If-None-Match", *src.LastETag)
	}
	if !needsMediaScan && src.LastModified != nil && *src.LastModified != "" {
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

	deepFetchEnabled := cfg.fullContentEnabled()
	const maxSyncDeepFetches = 15
	syncDeepFetchCount := 0
	var backfillTasks []ScrapeTask

	itemsAdded := 0
	for _, feedItem := range feed.Items {
		urlStr := strings.TrimSpace(feedItem.Link)
		if urlStr == "" {
			continue
		}

		category := classifier.Classify(urlStr, src.DefaultCategory)
		origin := classifier.ExtractOrigin(urlStr, feed.Title)

		content := feedItem.Content
		if content == "" {
			content = feedItem.Description
		}

		scrapedItem := item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: origin,
			Title:        feedItem.Title,
			URL:          urlStr,
			Summary:      truncateSummary(StripHTMLToText(firstNonEmpty(feedItem.Description, content)), 280),
			Content:      content,
			Category:     category,
		}

		// Podcast feeds fetched through the generic RSS path still need their
		// audio enclosure so the reader can offer a player.
		for _, enc := range feedItem.Enclosures {
			if enc == nil || strings.TrimSpace(enc.URL) == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(enc.Type), "audio") || isDirectAudioURL(enc.URL) {
				meta := map[string]any{"media_url": enc.URL}
				if feedItem.ITunesExt != nil && feedItem.ITunesExt.Duration != "" {
					meta["duration"] = feedItem.ITunesExt.Duration
				}
				if feed.Title != "" {
					meta["podcast"] = feed.Title
				}
				if metaJSON, err := json.Marshal(meta); err == nil {
					scrapedItem.Metadata = string(metaJSON)
				}
				if scrapedItem.Category == "article" {
					scrapedItem.Category = "podcast"
				}
				// One-time backfill: existing rows were stored before audio
				// enclosures were captured, so upgrade them in place.
				if needsMediaScan && scrapedItem.Metadata != "" {
					if err := s.db.UpdateSourceItemMedia(src.ID, urlStr, scrapedItem.Category, scrapedItem.Metadata); err != nil {
						s.logger.Warn("Failed to backfill podcast media metadata", zap.String("url", urlStr), zap.Error(err))
					}
				}
				break
			}
		}

		if feedItem.PublishedParsed != nil {
			scrapedItem.PublishedAt = feedItem.PublishedParsed
		} else {
			now := time.Now()
			scrapedItem.PublishedAt = &now
		}

		needsBrowserBackfill := false
		backfillURL := urlStr
		if deepFetchEnabled {
			exists, _ := s.db.ItemExistsByURL(urlStr)
			if !exists {
				needDeep, targetURL := NeedsDeepFetch(content, urlStr)
				if needDeep && targetURL != "" {
					backfillURL = targetURL
					if !RequiresBrowserExtract(targetURL) && syncDeepFetchCount < maxSyncDeepFetches {
						syncDeepFetchCount++
						pageTitle, md, extErr := ExtractWebArticle(ctx, targetURL)
						if extErr == nil && utf8.RuneCountInString(md) >= 200 {
							scrapedItem.Content = md
							if scrapedItem.Title == "" && pageTitle != "" {
								scrapedItem.Title = pageTitle
							}
							if scrapedItem.Summary == "" {
								scrapedItem.Summary = truncateSummary(md, 240)
							}
						} else {
							needsBrowserBackfill = true
						}
					} else {
						needsBrowserBackfill = true
					}
				}
			}
		}

		id, err := s.db.InsertScrapedItem(scrapedItem)
		if err != nil {
			s.logger.Error("Failed to insert RSS item", zap.Error(err))
			continue
		}
		if id > 0 {
			itemsAdded++
			if needsBrowserBackfill && s.taskQueue != nil {
				backfillTasks = append(backfillTasks, ScrapeTask{
					SourceID: src.ID,
					URL:      backfillURL,
					LogID:    logID,
					ItemID:   id,
				})
			} else if s.aiEngine != nil {
				s.aiEngine.Enqueue(id)
			}
		}
	}

	if needsMediaScan {
		raw := map[string]any{}
		if strings.TrimSpace(src.Config) != "" {
			_ = json.Unmarshal([]byte(src.Config), &raw)
		}
		raw["media_checked"] = true
		if b, err := json.Marshal(raw); err == nil {
			src.Config = string(b)
			if err := s.db.UpdateSource(*src); err != nil {
				s.logger.Warn("Failed to persist media scan flag", zap.String("source", src.ID), zap.Error(err))
			}
		}
	}

	if len(backfillTasks) > 0 && s.taskQueue != nil {
		s.logger.Info("Enqueuing RSS deep-fetch tasks for browser extension",
			zap.String("source", src.ID),
			zap.Int("count", len(backfillTasks)))
		s.taskQueue.Enqueue(backfillTasks)
	}

	return itemsAdded, nil
}

// IngestURL scrapes a single article, YouTube video, or podcast episode on demand,
// extracts full text or transcript, stores it in scraped_items, and optionally runs AI analysis.
func (s *Scraper) IngestURL(ctx context.Context, rawURL string, preferredType string, analyze bool) (*item.ScrapedItemWithAI, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return nil, errors.New("url is required")
	}

	if err := s.ensureRealtimeSource(); err != nil {
		return nil, fmt.Errorf("ensure realtime source: %w", err)
	}

	kind := strings.ToLower(strings.TrimSpace(preferredType))
	if kind == "" || kind == "auto" {
		switch {
		case IsYouTubeURL(trimmedURL):
			kind = "youtube"
		case IsPodcastURL(trimmedURL):
			kind = "podcast"
		default:
			kind = "article"
		}
	}

	var scraped item.ScrapedItem
	var err error

	switch kind {
	case "youtube":
		scraped, err = s.youtube.FetchYouTubeVideoItem(ctx, trimmedURL, "__realtime__", true, "", nil)
	case "podcast":
		// Never pull audio for a one-off ingest; the reader fetches it only
		// when the user presses play.
		scraped, err = s.podcast.FetchPodcastEpisodeItem(ctx, trimmedURL, "__realtime__")
	default:
		scraped, err = s.ingestWebPage(ctx, trimmedURL, kind)
	}
	if err != nil {
		return nil, err
	}

	if scraped.SourceID == "" {
		scraped.SourceID = "__realtime__"
	}
	if scraped.PublishedAt == nil {
		now := time.Now()
		scraped.PublishedAt = &now
	}

	id, _, err := s.db.UpsertScrapedItem(scraped)
	if err != nil {
		return nil, fmt.Errorf("save ingested item: %w", err)
	}

	if analyze && s.aiEngine != nil && s.aiEngine.IsEnabled() {
		if aiErr := s.aiEngine.AnalyzeItem(id); aiErr != nil {
			s.logger.Warn("Sync AI analysis failed for ingested URL", zap.Int64("item_id", id), zap.Error(aiErr))
		}
	}

	return s.db.GetScrapedItemWithAI(id)
}

func (s *Scraper) ingestWebPage(ctx context.Context, pageURL string, kind string) (item.ScrapedItem, error) {
	classifier := NewClassifier()
	category := kind
	if category == "" || category == "auto" {
		category = classifier.Classify(pageURL, "")
	}
	origin := classifier.ExtractOrigin(pageURL, "")

	var title, md string
	var httpErr error

	if !RequiresBrowserExtract(pageURL) {
		title, md, httpErr = ExtractWebArticle(ctx, pageURL)
	}

	if (httpErr != nil || utf8.RuneCountInString(strings.TrimSpace(md)) < 150) && s.wsManager != nil {
		if connID, err := s.wsManager.ResolveBrowserConnID(""); err == nil {
			extCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
			defer cancel()
			resp, err := s.wsManager.SendMessage(extCtx, &capture.BrowserRequest{
				Source:  "realtime_ingest",
				Action:  "mcp_request",
				Command: "extract",
				URL:     pageURL,
				Visible: false,
			}, connID)
			if err == nil && resp.Success {
				if extMD := strings.TrimSpace(resp.Result.Content.MarkdownContent()); extMD != "" {
					md = extMD
					title = firstNonEmpty(resp.Result.Title, resp.Result.Content.Title, title)
					if resp.Result.Content.Site != "" {
						origin = resp.Result.Content.Site
					}
					httpErr = nil
				}
			}
		}
	}

	if strings.TrimSpace(md) == "" {
		if httpErr != nil {
			return item.ScrapedItem{}, fmt.Errorf("failed to extract webpage: %w", httpErr)
		}
		return item.ScrapedItem{}, fmt.Errorf("extracted empty content from %s", pageURL)
	}

	if title == "" {
		title = pageURL
	}

	return item.ScrapedItem{
		SourceID:     "__realtime__",
		OriginSource: origin,
		Title:        title,
		URL:          pageURL,
		Summary:      truncateSummary(md, 240),
		Content:      md,
		Category:     category,
		Metadata:     `{"transcript_source":"web_extract"}`,
	}, nil
}

func (s *Scraper) ensureRealtimeSource() error {
	src, err := s.db.GetSource("__realtime__")
	if err != nil {
		return err
	}
	if src != nil {
		return nil
	}
	return s.db.InsertSource(source.Source{
		ID:              "__realtime__",
		Name:            "实时抓取",
		Type:            "api",
		URL:             "https://grabby.local/realtime",
		Schedule:        "0 0 * * *",
		Enabled:         0,
		DefaultCategory: "auto",
		Category:        "实时采集",
		Config:          "{}",
	})
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
			if s.aiEngine != nil {
				s.aiEngine.Enqueue(id)
			}
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
			Visible: false,
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
