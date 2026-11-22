package scraping

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
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

// reportingAdapter is implemented by adapters that can explain what happened
// to each text field (description, caption, transcript) for the fetch log.
// Fetch remains the stable public interface.
type reportingAdapter interface {
	FetchWithReports(ctx context.Context, src source.Source) ([]item.ScrapedItem, []ItemReport, error)
}

type sourceRuntimeConfig struct {
	FullContent        bool   `json:"full_content"`
	FetchFullViaScrape bool   `json:"fetch_full_via_scrape"`
	TranscribeAudio    *bool  `json:"transcribe_audio"`
	MaxItems           int    `json:"max_items"`
	MaxASRItems        int    `json:"max_asr_items"`
	MaxBackfill        int    `json:"max_backfill"`
	MediaChecked       bool   `json:"media_checked"`
	FeedURL            string `json:"feed_url"`
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

	// Retry policy. Tests set maxRetries=1 / zero backoff to stay fast; the
	// production defaults keep the historical 3 attempts with 5s/15s waits.
	maxRetries   int
	retryBackoff func(attempt int) time.Duration
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
		maxRetries:  3,
		retryBackoff: func(attempt int) time.Duration {
			return time.Duration(5*math.Pow(3, float64(attempt-1))) * time.Second
		},
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

// ScrapeSource triggers a manual scrape (the default entry point retained for
// callers and tests).
func (s *Scraper) ScrapeSource(ctx context.Context, src source.Source) (int, error) {
	return s.ScrapeSourceWithTrigger(ctx, src, "manual")
}

// ScrapeSourceWithTrigger runs one fetch and persists a fetch_logs row plus a
// per-entry detail row for everything discovered, stored, backfilled, skipped
// or failed. trigger is "manual" or "scheduled".
func (s *Scraper) ScrapeSourceWithTrigger(ctx context.Context, src source.Source, trigger string) (int, error) {
	count, _, err := s.RunSource(ctx, src, trigger)
	return count, err
}

// RunSource returns the ID of the run it started, including when work is queued.
func (s *Scraper) RunSource(ctx context.Context, src source.Source, trigger string) (int, int64, error) {
	if trigger == "" {
		trigger = "manual"
	}
	s.logger.Info("Starting scrape for source", zap.String("id", src.ID), zap.String("type", src.Type), zap.String("trigger", trigger))

	startedAt := time.Now()
	logID, err := s.db.InsertFetchLog(source.FetchLog{
		SourceID:  src.ID,
		StartedAt: startedAt,
		Status:    "running",
		Trigger:   trigger,
	})
	if err != nil {
		s.logger.Error("Failed to insert fetch log", zap.Error(err))
	}
	run := newScrapeRun(s.db, logID, src.ID)

	var count int
	var scrapeErr error

	scrapeFunc := func() (int, error) {
		run.reset()
		switch src.Type {
		case "rss":
			return s.scrapeRSSWithContext(ctx, &src, run)
		case "api":
			return s.ScrapeAPIWithRun(ctx, src, run)
		case "web_scrape":
			return s.ScrapeWebWithRun(ctx, &src, run)
		default:
			if adapter, ok := s.adapters[src.Type]; ok {
				return s.runAdapter(ctx, adapter, src, run)
			}
			return 0, fmt.Errorf("unsupported source type: %s", src.Type)
		}
	}

	// Retry wrapper with exponential backoff.
	maxRetries := s.maxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	for attempt := 1; attempt <= maxRetries; attempt++ {
		count, scrapeErr = scrapeFunc()
		if scrapeErr == nil {
			break
		}
		if attempt < maxRetries {
			backoff := time.Duration(0)
			if s.retryBackoff != nil {
				backoff = s.retryBackoff(attempt)
			}
			s.logger.Warn("Scrape failed, retrying...",
				zap.String("source", src.ID),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(scrapeErr),
			)
			select {
			case <-ctx.Done():
				scrapeErr = ctx.Err()
				attempt = maxRetries
			case <-time.After(backoff):
			}
		}
	}

	run.flush()

	errMsg := ""
	if scrapeErr != nil {
		errMsg = sanitizeText(scrapeErr.Error())
		s.logger.Error("Scrape finished with error", zap.String("source", src.ID), zap.Error(scrapeErr))
	} else {
		s.logger.Info("Scrape finished successfully", zap.String("source", src.ID), zap.Int("added", count))
	}

	override := ""
	if run.notModified {
		override = "not_modified"
	}
	if run.overrideStatus != "" {
		override = run.overrideStatus
	}
	if scrapeErr != nil {
		stored, updated, _, failed, _ := run.counts()
		if failed == 0 && run.textFailures() == 0 {
			if stored+updated > 0 {
				override = "partial"
			} else {
				override = "error"
			}
		}
	}
	if logID > 0 {
		if pendingCount, err := s.db.CountPendingFetchLogItems(logID); err == nil && pendingCount > 0 {
			override = "running"
		}
	}

	summary := run.summaryText(override, errMsg)
	final, finalizeErr := s.finalizeRun(logID, override, summary, errMsg)
	if finalizeErr != nil {
		s.logger.Warn("Failed to finalize fetch log", zap.Int64("log_id", logID), zap.Error(finalizeErr))
	}
	status := "success"
	if finalizeErr == nil && final != nil {
		status = final.Status
	} else if scrapeErr != nil {
		status = "error"
	}
	_ = s.db.UpdateSourceFetchStatus(src.ID, status, src.LastETag, src.LastModified)
	run.dispatchBrowser(s.taskQueue)

	return count, logID, scrapeErr
}

func (s *Scraper) finalizeRun(logID int64, override, summary, errMsg string) (*source.FetchLog, error) {
	if logID <= 0 {
		return nil, nil
	}
	log, err := s.db.FinalizeFetchLog(logID, override, summary, errMsg)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *scrapeRun) summaryText(override, errMsg string) string {
	stored, updated, skipped, failed, pending := r.counts()
	textFailed := r.textFailures()
	note := ""
	switch override {
	case "not_modified":
		note = "Feed 返回 304，未变更"
	case "no_new_items":
		note = "没有新条目"
	case "error":
		note = "本轮失败"
	case "running":
		note = "异步任务处理中"
	}
	parts := []string{
		fmt.Sprintf("发现 %d 条", len(r.snapshot())),
		fmt.Sprintf("新增 %d", stored),
		fmt.Sprintf("补全 %d", updated),
		fmt.Sprintf("跳过 %d", skipped),
		fmt.Sprintf("失败 %d", failed),
	}
	if textFailed > 0 {
		parts = append(parts, fmt.Sprintf("文本失败 %d", textFailed))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("处理中 %d", pending))
	}
	if note != "" {
		parts = append(parts, note)
	}
	if errMsg != "" && (override == "error" || override == "partial") {
		parts = append(parts, "原因："+truncateSummary(errMsg, 120))
	}
	return strings.Join(parts, "，")
}

func (s *Scraper) runAdapter(ctx context.Context, adapter SourceAdapter, src source.Source, run *scrapeRun) (int, error) {
	var items []item.ScrapedItem
	var reports []ItemReport
	var err error
	if reporting, ok := adapter.(reportingAdapter); ok {
		items, reports, err = reporting.FetchWithReports(ctx, src)
	} else {
		items, err = adapter.Fetch(ctx, src)
	}
	if err != nil {
		if len(reports) == 0 {
			run.add(ItemReport{Title: src.Name, URL: src.URL, Status: "failed",
				Reason: "抓取失败: " + err.Error(), ErrorStage: failureStage(err),
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: src.Type,
					URL: src.URL, Reason: err.Error(), Stage: failureStage(err)}}})
		} else {
			for _, rep := range reports {
				if rep.Status == "" || rep.Status == "stored" {
					rep.Status = "failed"
				}
				run.add(rep)
			}
		}
		run.overrideStatus = "error"
		return 0, err
	}

	// Reports may include skipped entries that are not returned as items, so
	// align them by URL instead of by slice index.
	reportByURL := map[string]ItemReport{}
	for _, rep := range reports {
		if key := canonicalLinkKey(rep.URL); key != "" {
			reportByURL[key] = rep
		}
		if rep.URL != "" {
			reportByURL[rep.URL] = rep
		}
	}
	consumed := map[string]bool{}

	itemsAdded := 0
	for _, it := range items {
		if it.SourceID == "" {
			it.SourceID = src.ID
		}
		if it.PublishedAt == nil {
			now := time.Now()
			it.PublishedAt = &now
		}
		key := canonicalLinkKey(it.URL)
		rep, hasRep := reportByURL[key]
		if !hasRep {
			rep, hasRep = reportByURL[it.URL]
		}
		if hasRep {
			consumed[key] = true
			consumed[it.URL] = true
		}

		id, created, upsertErr := s.db.UpsertScrapedItem(it)
		if upsertErr != nil {
			s.logger.Error("Failed to upsert adapter item", zap.String("url", it.URL), zap.Error(upsertErr))
			if !hasRep {
				rep = ItemReport{Title: it.Title, URL: it.URL, Category: it.Category}
			}
			rep.Status = "failed"
			rep.Reason = "保存到内容库失败: " + upsertErr.Error()
			rep.ErrorStage = "persist"
			run.add(rep)
			continue
		}
		adapterPersisted := hasRep && rep.ItemID != 0
		if created {
			itemsAdded++
		} else if adapterPersisted && rep.Status == "stored" {
			// Adapters persist new rows themselves (to merge captions and
			// transcripts atomically), so a follow-up upsert sees an existing
			// row. The report is still authoritative for "new in this run".
			itemsAdded++
		} else if !created && !adapterPersisted && hasRep && rep.Status == "stored" {
			// A concurrent run inserted this URL between the adapter's
			// existence check and this upsert. Keep the existing row.
			rep.Status = "kept_existing"
			rep.Reason = "条目已存在（并发写入），保留已有内容"
		}
		if id > 0 && s.aiEngine != nil && (created || utf8.RuneCountInString(it.Content) > 0) {
			s.aiEngine.Enqueue(id)
		}
		if !hasRep {
			status := "updated"
			if created {
				status = "stored"
			}
			rep = ItemReport{Title: it.Title, URL: it.URL, Category: it.Category, Status: status}
		}
		rep.ItemID = id
		if created && (rep.Status == "" || rep.Status == "updated") {
			rep.Status = "stored"
		} else if !created && rep.Status == "" {
			rep.Status = "updated"
		}
		run.add(rep)
	}

	// Entries the adapter deliberately skipped (already complete, over limit)
	// still belong in this run's log.
	for _, rep := range reports {
		key := canonicalLinkKey(rep.URL)
		if consumed[key] || consumed[rep.URL] {
			continue
		}
		if rep.Status == "" {
			rep.Status = "skipped"
		}
		run.add(rep)
	}
	return itemsAdded, nil
}

// ScrapeRSS fetches and parses RSS/Atom feeds.
func (s *Scraper) ScrapeRSS(src source.Source) (int, error) {
	run := newScrapeRun(nil, 0, src.ID)
	count, err := s.scrapeRSSWithContext(context.Background(), &src, run)
	run.dispatchBrowser(s.taskQueue)
	return count, err
}

func (s *Scraper) scrapeRSSWithContext(ctx context.Context, src *source.Source, run *scrapeRun) (int, error) {
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
		run.add(ItemReport{Title: "Feed", URL: src.URL, Status: "failed",
			Reason: "访问 Feed 失败: " + err.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "feed",
				URL: src.URL, Reason: err.Error(), Stage: "access"}}})
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		s.logger.Info("RSS feed returned 304 Not Modified", zap.String("id", src.ID))
		run.notModified = true
		// A 304 says "the feed did not change", not "nothing to do": entries
		// whose original page failed earlier must still be retried.
		if cfg.fullContentEnabled() {
			s.backfillKnownRSSItems(ctx, src, run, cfg)
		}
		s.persistMediaCheckedFlag(src, needsMediaScan)
		return 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP error code: %d", resp.StatusCode)
		run.add(ItemReport{Title: "Feed", URL: src.URL, Status: "failed",
			Reason: "访问 Feed 失败: " + err.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "feed",
				URL: src.URL, Reason: err.Error(), Stage: "access"}}})
		return 0, err
	}

	fp := gofeed.NewParser()
	feed, err := fp.Parse(resp.Body)
	if err != nil {
		parseErr := fmt.Errorf("failed to parse feed: %w", err)
		run.add(ItemReport{Title: "Feed", URL: src.URL, Status: "failed",
			Reason: "解析 Feed 失败: " + parseErr.Error(), ErrorStage: "extract",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "feed",
				URL: src.URL, Reason: parseErr.Error(), Stage: "extract"}}})
		return 0, parseErr
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
	if newETag != nil || newLastMod != nil {
		src.LastETag = newETag
		src.LastModified = newLastMod
	}

	fullEnabled := cfg.fullContentEnabled()
	maxDeepFetches := cfg.MaxBackfill
	if maxDeepFetches <= 0 {
		maxDeepFetches = 15
	}
	deepAttempts := 0
	itemsAdded := 0
	for _, feedItem := range feed.Items {
		rawLink := strings.TrimSpace(feedItem.Link)
		if rawLink == "" {
			continue
		}
		// RSS links are occasionally relative; resolve them against the feed URL.
		itemURL := resolveArticleURL(src.URL, rawLink)
		if itemURL == "" {
			continue
		}

		category := classifier.Classify(itemURL, src.DefaultCategory)
		origin := classifier.ExtractOrigin(itemURL, feed.Title)
		content := feedItem.Content
		if content == "" {
			content = feedItem.Description
		}
		summaryRaw := firstNonEmpty(feedItem.Description, content)
		summaryText := truncateSummary(StripHTMLToText(summaryRaw), 280)

		meta := map[string]any{}
		for _, enc := range feedItem.Enclosures {
			if enc == nil || strings.TrimSpace(enc.URL) == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(enc.Type), "audio") || isDirectAudioURL(enc.URL) {
				meta["media_url"] = resolveArticleURL(src.URL, enc.URL)
				if feedItem.ITunesExt != nil && feedItem.ITunesExt.Duration != "" {
					meta["duration"] = feedItem.ITunesExt.Duration
				}
				if feed.Title != "" {
					meta["podcast"] = feed.Title
				}
				if category == "article" {
					category = "podcast"
				}
				break
			}
		}
		if needsMediaScan && len(meta) > 0 {
			if err := s.db.UpdateSourceItemMedia(src.ID, itemURL, category, marshalMetadata(meta)); err != nil {
				s.logger.Warn("Failed to backfill podcast media metadata", zap.String("url", itemURL), zap.Error(err))
			}
		}

		publishedAt := feedItem.PublishedParsed
		if publishedAt == nil {
			now := time.Now()
			publishedAt = &now
		}

		if existing, err := s.db.GetScrapedItemFullByURL(itemURL); err != nil {
			run.add(ItemReport{Title: feedItem.Title, URL: itemURL, Category: category, Status: "failed",
				Reason: "读取内容库失败: " + err.Error(), ErrorStage: "persist"})
			continue
		} else if existing != nil {
			if fullEnabled && rssNeedsBackfill(existing) {
				if deepAttempts >= maxDeepFetches {
					run.add(ItemReport{
						Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
						Status: "skipped", Reason: "本轮补全达到上限，稍后继续",
						Texts: []source.FetchTextResult{{Kind: "content", Status: "not_attempted", Source: "library",
							Reason: "本轮全文补全达到上限", Chars: utf8.RuneCountInString(existing.Content)}},
					})
					continue
				}
				deepAttempts++
			}
			s.rssHandleExisting(ctx, src, run, existing, cfg)
			continue
		}

		scrapedItem := item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: origin,
			Title:        feedItem.Title,
			URL:          itemURL,
			Summary:      summaryText,
			Content:      content,
			Category:     category,
			PublishedAt:  publishedAt,
		}
		rep := ItemReport{Title: feedItem.Title, URL: itemURL, Category: category, Status: "stored"}
		if summaryText != "" {
			rep.Texts = append(rep.Texts, source.FetchTextResult{
				Kind: "summary", Status: "stored", Source: "feed", Chars: utf8.RuneCountInString(summaryText),
			})
		}

		needDeep := false
		browserPending := false
		targetURL := itemURL
		if fullEnabled {
			needDeep, targetURL = NeedsDeepFetch(content, itemURL)
		}

		switch {
		case fullEnabled && needDeep && targetURL != "" && deepAttempts >= maxDeepFetches:
			meta["content_source"] = "feed_summary"
			meta["full_content_status"] = "deferred"
			rep.Texts = append(rep.Texts, source.FetchTextResult{
				Kind: "content", Status: "not_attempted", Source: "feed_summary",
				Reason: "本轮全文抓取达到上限，保留 Feed 简介供后续补抓",
				Chars:  utf8.RuneCountInString(StripHTMLToText(content)),
			})
		case fullEnabled && needDeep && targetURL != "":
			deepAttempts++
			fullText, pageTitle, pending, extErr := s.fetchOriginalArticle(ctx, targetURL)
			browserPending = pending
			if browserPending {
				meta["content_source"] = "feed_summary"
				meta["full_content_status"] = "deferred"
				meta["full_content_url"] = targetURL
				rep.Texts = append(rep.Texts, pendingArticleText(targetURL))
			} else if extErr == nil && utf8.RuneCountInString(fullText) >= 200 {
				scrapedItem.Content = fullText
				if scrapedItem.Title == "" && pageTitle != "" {
					scrapedItem.Title = pageTitle
				}
				meta["content_source"] = "web_page"
				meta["full_content_status"] = "fetched"
				meta["full_content_url"] = targetURL
				rep.Texts = append(rep.Texts, source.FetchTextResult{
					Kind: "content", Status: "stored", Source: "web_page", URL: targetURL,
					Chars: utf8.RuneCountInString(fullText),
				})
			} else {
				reason := "empty or short article text"
				stage := "extract"
				if extErr != nil {
					reason = extErr.Error()
					stage = failureStage(extErr)
				}
				meta["content_source"] = "feed_summary"
				meta["full_content_status"] = "failed"
				meta["full_content_error"] = sanitizeText(reason)
				meta["full_content_attempted_at"] = time.Now().UnixNano()
				meta["full_content_url"] = targetURL
				rep.Texts = append(rep.Texts, source.FetchTextResult{
					Kind: "content", Status: "failed", Source: "feed_summary", URL: targetURL,
					Reason: "原文获取失败，保留 Feed 简介: " + reason, Stage: stage,
					Chars: utf8.RuneCountInString(StripHTMLToText(content)),
				})
			}
		case fullEnabled:
			meta["content_source"] = "feed"
			meta["full_content_status"] = "not_needed"
			rep.Texts = append(rep.Texts, source.FetchTextResult{
				Kind: "content", Status: "stored", Source: "feed", Chars: utf8.RuneCountInString(StripHTMLToText(content)),
			})
		default:
			plainLen := utf8.RuneCountInString(StripHTMLToText(content))
			sourceName := "feed_summary"
			reason := "full_content_disabled：仅保存 Feed 内容"
			if plainLen >= defaultDeepFetchMinRunes {
				sourceName = "feed"
				reason = "full_content_disabled：保存 Feed 自带正文"
			}
			meta["content_source"] = sourceName
			meta["full_content_status"] = "disabled"
			rep.Texts = append(rep.Texts, source.FetchTextResult{
				Kind: "content", Status: "stored", Source: sourceName,
				Reason: reason,
				Chars:  plainLen,
			})
		}
		scrapedItem.Metadata = marshalMetadata(meta)

		id, insertErr := s.db.InsertScrapedItem(scrapedItem)
		if insertErr != nil {
			rep.Status = "failed"
			rep.Reason = "写入内容库失败: " + insertErr.Error()
			rep.ErrorStage = "persist"
			run.add(rep)
			continue
		}
		if id == 0 {
			// A concurrent run inserted the same URL first.
			if existing, _ := s.db.GetScrapedItemFullByURL(itemURL); existing != nil {
				rep.ItemID = existing.ID
				rep.Status = "kept_existing"
				rep.Reason = "条目已存在（并发写入）"
				run.add(rep)
			} else {
				rep.Status = "failed"
				rep.Reason = "条目写入后无法读回"
				rep.ErrorStage = "persist"
				run.add(rep)
			}
			continue
		}
		itemsAdded++
		rep.ItemID = id
		if browserPending {
			rep.Status = "pending"
			run.queueBrowser(ScrapeTask{SourceID: src.ID, URL: targetURL, ItemID: id, NewItem: true}, rep)
			continue
		}
		run.add(rep)
		if s.aiEngine != nil {
			s.aiEngine.Enqueue(id)
		}
	}

	s.persistMediaCheckedFlag(src, needsMediaScan)
	return itemsAdded, nil
}

// fetchOriginalArticle extracts a public article via HTTP, then falls back to
// the browser executor for hosts that require JS/session cookies.
func (s *Scraper) fetchOriginalArticle(ctx context.Context, pageURL string) (string, string, bool, error) {
	queueAvailable := s.taskQueue != nil && s.taskQueue.IsRunning()
	if !RequiresBrowserExtract(pageURL) {
		res, err := ExtractWebArticleDetails(ctx, pageURL)
		if err == nil && utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) >= 200 {
			return res.Markdown, res.Title, false, nil
		}
		if !queueAvailable && s.wsManager == nil {
			if err != nil {
				return "", "", false, err
			}
			return res.Markdown, res.Title, false, fmt.Errorf("extracted article was shorter than 200 characters")
		}
	}
	if queueAvailable {
		return "", "", true, nil
	}
	markdown, title, err := s.extractPageViaBrowser(ctx, pageURL)
	return markdown, title, false, err
}

func pendingArticleText(target string) source.FetchTextResult {
	return source.FetchTextResult{Kind: "content", Status: "pending", Source: "browser_page", URL: target,
		Reason: "等待浏览器提取原文"}
}

func (s *Scraper) extractPageViaBrowser(ctx context.Context, pageURL string) (string, string, error) {
	if s.wsManager == nil {
		return "", "", fmt.Errorf("browser executor unavailable for %s", pageURL)
	}
	connID, err := s.wsManager.ResolveBrowserConnID("")
	if err != nil {
		return "", "", fmt.Errorf("no browser connected: %w", err)
	}
	extCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	resp, err := s.wsManager.SendMessage(extCtx, &capture.BrowserRequest{
		Source:  "subscription_extract",
		Action:  "mcp_request",
		Command: "extract",
		URL:     pageURL,
		Visible: false,
	}, connID)
	if err != nil {
		return "", "", fmt.Errorf("browser extract failed: %w", err)
	}
	if !resp.Success {
		return "", "", fmt.Errorf("browser extract failed: %s", resp.Error)
	}
	md := strings.TrimSpace(resp.Result.Content.MarkdownContent())
	if md == "" {
		return "", "", fmt.Errorf("browser extracted empty markdown")
	}
	title := firstNonEmpty(resp.Result.Title, resp.Result.Content.Title)
	return md, title, nil
}

func failureStage(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "http ") || strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "context deadline") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "connection") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "no browser") ||
		strings.Contains(msg, "browser extract failed") || strings.Contains(msg, "eof") ||
		strings.Contains(msg, "redirect") {
		return "access"
	}
	return "extract"
}

// rssHandleExisting backfills a stored entry when full content is enabled.
// It never changes the item ID, read status, starred flag or original summary.
func (s *Scraper) rssHandleExisting(ctx context.Context, src *source.Source, run *scrapeRun, existing *item.ScrapedItem, cfg sourceRuntimeConfig) {
	if !cfg.fullContentEnabled() {
		run.add(ItemReport{
			Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
			Status: "skipped", Reason: "条目已存在；全文抓取未启用",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "kept_existing", Source: "library", Chars: utf8.RuneCountInString(existing.Content)}},
		})
		return
	}
	if !rssNeedsBackfill(existing) {
		run.add(ItemReport{
			Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
			Status: "skipped", Reason: "已有正文，无需补全",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "kept_existing", Source: "library", Chars: utf8.RuneCountInString(existing.Content)}},
		})
		return
	}
	s.backfillRSSItem(ctx, existing, run)
}

func rssNeedsBackfill(existing *item.ScrapedItem) bool {
	meta := parseMetadata(existing.Metadata)
	status := metaString(meta, "full_content_status")
	if status == "fetched" {
		return false
	}
	if status == "failed" || status == "deferred" {
		return true
	}
	if utf8.RuneCountInString(strings.TrimSpace(existing.Content)) < defaultDeepFetchMinRunes {
		return true
	}
	need, _ := NeedsDeepFetch(existing.Content, existing.URL)
	return need
}

func (s *Scraper) backfillRSSItem(ctx context.Context, existing *item.ScrapedItem, run *scrapeRun) {
	_, target := NeedsDeepFetch(existing.Content, existing.URL)
	if target == "" {
		target = existing.URL
	}
	meta := parseMetadata(existing.Metadata)
	fullText, pageTitle, browserPending, err := s.fetchOriginalArticle(ctx, target)
	if browserPending {
		run.queueBrowser(ScrapeTask{SourceID: run.sourceID, URL: target, ItemID: existing.ID}, ItemReport{
			Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
			Status: "pending", Reason: "等待浏览器补全正文", Texts: []source.FetchTextResult{
				{Kind: "summary", Status: "kept_existing", Source: "feed", Chars: utf8.RuneCountInString(existing.Summary)},
				pendingArticleText(target),
			},
		})
		return
	}
	if err == nil && utf8.RuneCountInString(fullText) >= 200 {
		meta["content_source"] = "web_page"
		meta["full_content_status"] = "fetched"
		meta["full_content_url"] = target
		delete(meta, "full_content_error")
		title := existing.Title
		if title == "" {
			title = pageTitle
		}
		if uerr := s.db.UpdateScrapedItemFromScrape(existing.ID, title, existing.Summary, fullText, existing.Category,
			marshalMetadata(meta), existing.PublishedAt); uerr != nil {
			run.add(ItemReport{Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
				Status: "failed", Reason: "补全写入内容库失败: " + uerr.Error(), ErrorStage: "persist",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "web_page", URL: target,
					Reason: uerr.Error(), Stage: "persist"}}})
			return
		}
		run.add(ItemReport{Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
			Status: "updated", Reason: "已补全原文正文",
			Texts: []source.FetchTextResult{
				{Kind: "summary", Status: "kept_existing", Source: "feed", Chars: utf8.RuneCountInString(existing.Summary)},
				{Kind: "content", Status: "updated", Source: "web_page", URL: target, Chars: utf8.RuneCountInString(fullText)},
			}})
		if s.aiEngine != nil {
			s.aiEngine.Enqueue(existing.ID)
		}
		return
	}

	reason := "empty or short article text"
	stage := "extract"
	if err != nil {
		reason = err.Error()
		stage = failureStage(err)
	}
	meta["content_source"] = prefString(metaString(meta, "content_source"), "feed_summary")
	meta["full_content_status"] = "failed"
	meta["full_content_error"] = sanitizeText(reason)
	meta["full_content_attempted_at"] = time.Now().UnixNano()
	meta["full_content_url"] = target
	_ = s.db.UpdateScrapedItemFromScrape(existing.ID, "", existing.Summary, "", existing.Category,
		marshalMetadata(meta), nil)
	run.add(ItemReport{Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
		Status: "kept_existing", Reason: "补全失败，保留已有内容: " + reason,
		Texts: []source.FetchTextResult{
			{Kind: "content", Status: "failed", Source: "web_page", URL: target,
				Reason: reason, Stage: stage, Chars: utf8.RuneCountInString(existing.Content)},
		}})
}

func prefString(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// backfillKnownRSSItems retries known summary-only entries without a new feed
// body (notably after a 304). Work is bounded per run.
func (s *Scraper) backfillKnownRSSItems(ctx context.Context, src *source.Source, run *scrapeRun, cfg sourceRuntimeConfig) {
	items, err := s.db.ListSourceItems(src.ID)
	if err != nil {
		run.add(ItemReport{Title: "读取已有条目", URL: src.URL, Status: "failed",
			Reason: "读取内容库失败: " + err.Error(), ErrorStage: "persist"})
		return
	}
	limit := cfg.MaxBackfill
	if limit <= 0 {
		limit = 15
	}
	// Rotate failed entries behind work that has not been tried recently.
	attemptTimes := make(map[int64]float64, len(items))
	for _, entry := range items {
		attemptTimes[entry.ID], _ = parseMetadata(entry.Metadata)["full_content_attempted_at"].(float64)
	}
	slices.SortStableFunc(items, func(a, b item.ScrapedItem) int {
		return cmp.Compare(attemptTimes[a.ID], attemptTimes[b.ID])
	})
	attempted := 0
	for i := range items {
		if attempted >= limit {
			break
		}
		if !rssNeedsBackfill(&items[i]) {
			continue
		}
		attempted++
		s.backfillRSSItem(ctx, &items[i], run)
	}
}

func (s *Scraper) persistMediaCheckedFlag(src *source.Source, needsMediaScan bool) {
	if !needsMediaScan {
		return
	}
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
	return s.ScrapeAPIWithRun(context.Background(), src, newScrapeRun(nil, 0, src.ID))
}

func (s *Scraper) ScrapeAPIWithRun(ctx context.Context, src source.Source, run *scrapeRun) (int, error) {
	var config APIConfig
	if src.Config != "" {
		_ = json.Unmarshal([]byte(src.Config), &config)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
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
			run.add(ItemReport{Title: title, URL: itemURL, Category: category, Status: "failed",
				Reason: "写入内容库失败: " + err.Error(), ErrorStage: "persist",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "api", Reason: err.Error(), Stage: "persist"}}})
			continue
		}
		if id > 0 {
			itemsAdded++
			if s.aiEngine != nil {
				s.aiEngine.Enqueue(id)
			}
			run.add(ItemReport{Title: title, URL: itemURL, Category: category, ItemID: id, Status: "stored",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "stored", Source: "api", Chars: utf8.RuneCountInString(summary)}}})
		} else {
			run.add(ItemReport{Title: title, URL: itemURL, Category: category, Status: "skipped", Reason: "条目已存在",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "kept_existing", Source: "library"}}})
		}
	}

	return itemsAdded, nil
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

// ScrapeWeb is retained for callers that do not need the detailed run record.
func (s *Scraper) ScrapeWeb(src source.Source, logID int64) (int, error) {
	run := newScrapeRun(s.db, logID, src.ID)
	count, err := s.ScrapeWebWithRun(context.Background(), &src, run)
	run.flush()
	run.dispatchBrowser(s.taskQueue)
	return count, err
}

// ScrapeWebWithRun handles both single-page subscriptions and public blog
// listing pages. A listing page is discovered with the same flow regardless of
// the site: scope by config, resolve relative links, filter chrome, then open
// each article and store title/body/published date.
func (s *Scraper) ScrapeWebWithRun(ctx context.Context, src *source.Source, run *scrapeRun) (int, error) {
	var cfg WebConfig
	if strings.TrimSpace(src.Config) != "" {
		if err := json.Unmarshal([]byte(src.Config), &cfg); err != nil {
			s.logger.Warn("Invalid web scrape config, using defaults", zap.String("source", src.ID), zap.Error(err))
		}
	}
	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = 20
	}
	client := &http.Client{Timeout: 30 * time.Second}

	if !cfg.listMode() {
		return s.scrapeSingleWebPage(ctx, src, cfg, run)
	}

	// --- Stage 1: discover article links -------------------------------------
	var candidates []listCandidate
	var listErr error
	useBrowser := cfg.UseBrowser != nil && *cfg.UseBrowser
	if !useBrowser {
		doc, base, err := fetchListDocument(ctx, client, src.URL)
		if err != nil {
			listErr = err
		} else {
			candidates, err = discoverArticleLinks(doc, base, src.URL, cfg)
			if err != nil {
				listErr = err
			}
		}
	}
	if useBrowser || (listErr != nil && s.wsManager != nil) || (len(candidates) == 0 && s.wsManager != nil) {
		if md, _, err := s.extractPageViaBrowser(ctx, src.URL); err == nil {
			base, _ := url.Parse(src.URL)
			if browserCandidates := candidatesFromMarkdown(md, base, src.URL, cfg); len(browserCandidates) > 0 {
				candidates = browserCandidates
				listErr = nil
			}
		} else if listErr == nil {
			listErr = err
		}
	}
	if listErr != nil {
		run.add(ItemReport{Title: "列表页", URL: src.URL, Status: "failed",
			Reason: "访问列表页失败: " + listErr.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "blog_list", URL: src.URL,
				Reason: listErr.Error(), Stage: "access"}}})
		return 0, listErr
	}
	if len(candidates) == 0 {
		run.overrideStatus = "no_new_items"
		run.add(ItemReport{Title: "列表页", URL: src.URL, Status: "skipped",
			Reason: "列表页未发现符合条件的文章链接"})
		return 0, nil
	}

	// --- Stage 2: fetch and store new articles -------------------------------
	itemsAdded := 0
	attempts := 0
	for _, cand := range candidates {
		articleURL := canonicalLinkKey(cand.URL)
		if articleURL == "" {
			continue
		}
		existing, err := s.db.GetScrapedItemFullByURL(articleURL)
		if err != nil {
			run.add(ItemReport{Title: cand.Title, URL: articleURL, Status: "failed",
				Reason: "读取内容库失败: " + err.Error(), ErrorStage: "persist"})
			continue
		}
		if existing != nil {
			run.add(ItemReport{Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
				Status: "skipped", Reason: "文章已入库",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "kept_existing", Source: "library",
					Chars: utf8.RuneCountInString(existing.Content)}}})
			continue
		}

		category := NewClassifier().Classify(articleURL, src.DefaultCategory)
		if category == "article" && (src.DefaultCategory == "" || src.DefaultCategory == "auto") {
			category = "blog"
		}
		if attempts >= maxItems {
			run.add(ItemReport{Title: firstNonEmpty(cand.Title, articleURL), URL: articleURL, Category: category,
				Status: "skipped", Reason: "达到本轮文章处理上限，稍后继续"})
			continue
		}
		attempts++

		// Browser-only hosts go through the async queue when one is running so
		// the log can distinguish "still processing" from a final failure.
		if RequiresBrowserExtract(articleURL) && s.taskQueue != nil && s.taskQueue.IsRunning() {
			rep := ItemReport{Title: firstNonEmpty(cand.Title, articleURL), URL: articleURL, Category: category,
				Status: "pending", Reason: "等待浏览器提取",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "pending", Source: "browser_page",
					URL: articleURL, Reason: "async browser task queued"}}}
			run.queueBrowser(ScrapeTask{SourceID: src.ID, URL: articleURL, Category: category,
				TitleHint: cand.Title}, rep)
			continue
		}

		res, err := ExtractWebArticleDetails(ctx, articleURL)
		if (err != nil || utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) < 200) && s.wsManager != nil {
			if md, title, berr := s.extractPageViaBrowser(ctx, articleURL); berr == nil {
				res.Markdown, res.Title = md, firstNonEmpty(title, res.Title)
				res.FinalURL = articleURL
				err = nil
			}
		}
		if err != nil || utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) < 200 {
			reason := "empty or short article text"
			stage := "extract"
			if err != nil {
				reason = err.Error()
				stage = failureStage(err)
			}
			run.add(ItemReport{Title: firstNonEmpty(cand.Title, articleURL), URL: articleURL, Category: category,
				Status: "failed", Reason: "文章页提取失败: " + reason, ErrorStage: stage,
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "blog_article",
					URL: articleURL, Reason: reason, Stage: stage}}})
			continue
		}

		title := firstNonEmpty(res.Title, cand.Title, articleURL)
		summary := truncateSummary(res.Markdown, 240)
		it := item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: NewClassifier().ExtractOrigin(articleURL, src.Name),
			Title:        title,
			URL:          articleURL,
			Summary:      summary,
			Content:      res.Markdown,
			Category:     category,
			PublishedAt:  res.PublishedAt,
			Metadata: marshalMetadata(map[string]any{
				"content_source": "blog_article",
				"article_url":    articleURL,
			}),
		}
		id, insertErr := s.db.InsertScrapedItem(it)
		if insertErr != nil {
			run.add(ItemReport{Title: title, URL: articleURL, Category: category, Status: "failed",
				Reason: "写入内容库失败: " + insertErr.Error(), ErrorStage: "persist",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "blog_article",
					URL: articleURL, Reason: insertErr.Error(), Stage: "persist"}}})
			continue
		}
		if id == 0 {
			run.add(ItemReport{Title: title, URL: articleURL, Category: category, Status: "skipped",
				Reason: "文章已存在（并发写入）"})
			continue
		}
		itemsAdded++
		run.add(ItemReport{Title: title, URL: articleURL, Category: category, ItemID: id, Status: "stored",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "stored", Source: "blog_article",
				URL: articleURL, Chars: utf8.RuneCountInString(res.Markdown)}}})
		if s.aiEngine != nil {
			s.aiEngine.Enqueue(id)
		}
	}
	return itemsAdded, nil
}

func (s *Scraper) scrapeSingleWebPage(ctx context.Context, src *source.Source, cfg WebConfig, run *scrapeRun) (int, error) {
	existing, err := s.db.GetScrapedItemFullByURL(src.URL)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		run.add(ItemReport{Title: existing.Title, URL: existing.URL, Category: existing.Category, ItemID: existing.ID,
			Status: "skipped", Reason: "网页已入库"})
		run.overrideStatus = "no_new_items"
		return 0, nil
	}

	res, err := ExtractWebArticleDetails(ctx, src.URL)
	if (err != nil || utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) < 200) && s.wsManager != nil {
		if md, title, berr := s.extractPageViaBrowser(ctx, src.URL); berr == nil {
			res.Markdown, res.Title = md, firstNonEmpty(title, res.Title)
			res.FinalURL = src.URL
			err = nil
		}
	}
	if err != nil {
		run.add(ItemReport{Title: src.Name, URL: src.URL, Status: "failed",
			Reason: "网页提取失败: " + err.Error(), ErrorStage: failureStage(err),
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "web_page",
				URL: src.URL, Reason: err.Error(), Stage: failureStage(err)}}})
		return 0, err
	}
	if utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) == 0 {
		err := fmt.Errorf("extracted empty content from %s", src.URL)
		run.add(ItemReport{Title: src.Name, URL: src.URL, Status: "failed",
			Reason: err.Error(), ErrorStage: "extract"})
		return 0, err
	}

	title := firstNonEmpty(res.Title, src.Name, src.URL)
	category := NewClassifier().Classify(src.URL, src.DefaultCategory)
	it := item.ScrapedItem{
		SourceID:     src.ID,
		OriginSource: NewClassifier().ExtractOrigin(src.URL, src.Name),
		Title:        title,
		URL:          src.URL,
		Summary:      truncateSummary(res.Markdown, 240),
		Content:      res.Markdown,
		Category:     category,
		PublishedAt:  res.PublishedAt,
		Metadata:     marshalMetadata(map[string]any{"content_source": "web_page"}),
	}
	id, err := s.db.InsertScrapedItem(it)
	if err != nil {
		run.add(ItemReport{Title: title, URL: src.URL, Category: category, Status: "failed",
			Reason: "写入内容库失败: " + err.Error(), ErrorStage: "persist"})
		return 0, err
	}
	if id == 0 {
		run.overrideStatus = "no_new_items"
		run.add(ItemReport{Title: title, URL: src.URL, Category: category, Status: "skipped", Reason: "网页已存在"})
		return 0, nil
	}
	run.add(ItemReport{Title: title, URL: src.URL, Category: category, ItemID: id, Status: "stored",
		Texts: []source.FetchTextResult{{Kind: "content", Status: "stored", Source: "web_page",
			URL: src.URL, Chars: utf8.RuneCountInString(res.Markdown)}}})
	if s.aiEngine != nil {
		s.aiEngine.Enqueue(id)
	}
	return 1, nil
}

// candidatesFromMarkdown is the browser-executor fallback for listing pages
// whose HTML is not readable over plain HTTP. It applies the same filters as
// the DOM path; selectors cannot be evaluated on markdown, so URL rules and
// chrome filtering still apply.
func candidatesFromMarkdown(markdown string, base *url.URL, listURL string, cfg WebConfig) []listCandidate {
	links := extractLinks(markdown)
	seen := map[string]bool{}
	listKey := canonicalLinkKey(listURL)
	var out []listCandidate
	for _, link := range links {
		abs := resolveArticleURL(listURL, link)
		u, err := url.Parse(abs)
		if err != nil {
			continue
		}
		key := canonicalLinkKey(abs)
		if key == "" || key == listKey || seen[key] || !sameSite(base, u) {
			continue
		}
		if nonArticlePathRegex.MatchString(u.Path) || assetPathRegex.MatchString(strings.ToLower(u.Path)) {
			continue
		}
		if p := strings.TrimSpace(cfg.ArticleURLPattern); p != "" {
			if re, err := regexp.Compile(p); err == nil && !re.MatchString(abs) {
				continue
			}
		}
		if p := strings.TrimSpace(cfg.ExcludeURLPattern); p != "" {
			if re, err := regexp.Compile(p); err == nil && re.MatchString(abs) {
				continue
			}
		}
		seen[key] = true
		out = append(out, listCandidate{URL: abs})
	}
	return out
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
