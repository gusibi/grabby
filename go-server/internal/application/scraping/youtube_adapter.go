package scraping

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
)

var (
	ytChannelIDMetaRegex  = regexp.MustCompile(`itemprop="channelId"\s+content="(UC[a-zA-Z0-9_-]{22})"`)
	ytChannelIDJSONRegex  = regexp.MustCompile(`"(?:externalId|channelId)"\s*:\s*"(UC[a-zA-Z0-9_-]{22})"`)
	ytChannelIDParamRegex = regexp.MustCompile(`channel_id=(UC[a-zA-Z0-9_-]{22})`)
	ytChannelIDPathRegex  = regexp.MustCompile(`/channel/(UC[a-zA-Z0-9_-]{22})`)
	ytValidVideoIDRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]{6,20}$`)
)

// YouTubeAdapter implements SourceAdapter for YouTube channels, playlists, and videos.
type YouTubeAdapter struct {
	scraper    *Scraper
	httpClient *http.Client
	// Test seams: production leaves these nil. Tests point the Atom feed and
	// watch pages at a local httptest server so the real parsing/persistence
	// path runs without the public network.
	feedURLOverride      string
	videoPageURLResolver func(string) string
}

// NewYouTubeAdapter creates a YouTubeAdapter bound to the parent Scraper.
func NewYouTubeAdapter(scraper *Scraper) *YouTubeAdapter {
	return &YouTubeAdapter{
		scraper:    scraper,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// IsYouTubeURL reports whether rawURL points to a YouTube domain.
func IsYouTubeURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "youtu.be" || host == "youtube.com" || strings.HasSuffix(host, ".youtube.com")
}

// ExtractYouTubeVideoID extracts the video ID from watch, shorts, live, embed, or youtu.be URLs.
func ExtractYouTubeVideoID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")

	if host == "youtu.be" {
		parts := strings.Split(path, "/")
		if len(parts) > 0 && ytValidVideoIDRegex.MatchString(parts[0]) {
			return parts[0]
		}
		return ""
	}

	if host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") {
		if path == "watch" {
			v := strings.TrimSpace(u.Query().Get("v"))
			if ytValidVideoIDRegex.MatchString(v) {
				return v
			}
		}
		for _, prefix := range []string{"shorts/", "live/", "embed/"} {
			if strings.HasPrefix(path, prefix) {
				id := strings.SplitN(strings.TrimPrefix(path, prefix), "/", 2)[0]
				if ytValidVideoIDRegex.MatchString(id) {
					return id
				}
			}
		}
	}
	return ""
}

// Fetch retrieves items from a YouTube channel/playlist feed or single video URL.
func (a *YouTubeAdapter) Fetch(ctx context.Context, src source.Source) ([]item.ScrapedItem, error) {
	items, _, err := a.FetchWithReports(ctx, src)
	return items, err
}

// FetchWithReports polls a channel (channel ID, @handle, playlist) or a single
// video and reports each video's description and caption outcome separately.
func (a *YouTubeAdapter) FetchWithReports(ctx context.Context, src source.Source) ([]item.ScrapedItem, []ItemReport, error) {
	cfg := parseSourceRuntimeConfig(src.Config)
	allowASR := cfg.transcribeAudioEnabled()

	feedURL, videoID, isSingleVideo, err := a.ResolveYouTubeFeedURL(ctx, src.URL)
	if err != nil && cfg.FeedURL == "" && a.feedURLOverride == "" {
		return nil, []ItemReport{{Title: src.URL, URL: src.URL, Status: "failed",
			Reason: "频道解析失败: " + err.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "youtube_channel",
				URL: src.URL, Reason: err.Error(), Stage: "access"}}}}, err
	}

	if cfg.FeedURL != "" {
		feedURL, isSingleVideo, videoID = cfg.FeedURL, false, ""
	}
	if a.feedURLOverride != "" {
		feedURL, isSingleVideo, videoID = a.feedURLOverride, false, ""
	}

	if isSingleVideo {
		canonical := "https://www.youtube.com/watch?v=" + videoID
		existing, _ := a.scraper.db.FindScrapedItemByVideoID(videoID)
		if existing == nil {
			existing, _ = a.scraper.db.GetScrapedItemFullByURL(canonical)
		}
		incoming, rep, ferr := a.fetchYouTubeVideo(ctx, canonical, src.ID, allowASR, "", nil, "")
		if ferr != nil {
			return nil, []ItemReport{rep}, ferr
		}
		it, rep2, perr := a.persistYouTubeItem(incoming, rep, existing)
		if perr != nil {
			return nil, []ItemReport{rep2}, perr
		}
		return []item.ScrapedItem{it}, []ItemReport{rep2}, nil
	}

	feed, err := a.fetchAtomFeed(ctx, feedURL)
	if err != nil {
		return nil, []ItemReport{{Title: src.URL, URL: src.URL, Status: "failed",
			Reason: "YouTube 频道 feed 获取失败: " + err.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "youtube_feed",
				URL: feedURL, Reason: err.Error(), Stage: "access"}}}}, fmt.Errorf("fetch YouTube feed %s: %w", feedURL, err)
	}

	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	var items []item.ScrapedItem
	var reports []ItemReport
	attempts := 0
	for _, entry := range feed.Items {
		videoURL := resolveArticleURL(feedURL, strings.TrimSpace(entry.Link))
		if videoURL == "" {
			continue
		}
		videoID := ExtractYouTubeVideoID(videoURL)
		canonical := videoURL
		if videoID != "" {
			canonical = "https://www.youtube.com/watch?v=" + videoID
		}
		feedDescription := youtubeFeedDescription(entry)

		existing := a.findExistingYouTube(videoID, canonical, videoURL)
		need := existing == nil || youtubeNeedsBackfill(existing)

		if !need {
			reports = append(reports, reportForExistingYouTube(existing, "已有简介与字幕，跳过"))
			continue
		}
		if attempts >= maxItems {
			if existing != nil {
				reports = append(reports, reportForExistingYouTube(existing, "达到本轮处理上限，稍后补抓"))
			} else {
				reports = append(reports, ItemReport{Title: firstNonEmpty(entry.Title, canonical), URL: canonical,
					Category: "youtube", Status: "skipped", Reason: "达到本轮视频处理上限，稍后补抓"})
			}
			continue
		}
		attempts++

		incoming, rep, ferr := a.fetchYouTubeVideo(ctx, canonical, src.ID, allowASR, feed.Title, entry.PublishedParsed, feedDescription)
		if ferr != nil {
			a.logger().Warn("Failed to extract YouTube video", zap.String("url", canonical), zap.Error(ferr))
			// Keep the Atom entry: it carries the original description even
			// when the watch page is temporarily blocked.
			fallback := youtubeFallbackFromEntry(src.ID, canonical, feed.Title, entry, feedDescription, ferr)
			incoming, rep = fallback, youtubeFallbackReport(canonical, entry.Title, feedTitleOr(feed.Title, "YouTube"), ferr, feedDescription)
		}
		// The channel Atom entry is authoritative for title/description. A
		// bot-blocked watch page can return "- YouTube" and a generic site
		// blurb through OpenGraph; never let that replace the feed.
		if strings.TrimSpace(entry.Title) != "" {
			incoming.Title = entry.Title
			rep.Title = entry.Title
		}
		it, finalRep, perr := a.persistYouTubeItem(incoming, rep, existing)
		if perr != nil {
			finalRep.Status = "failed"
			finalRep.Reason = "写入内容库失败: " + perr.Error()
			finalRep.ErrorStage = "persist"
			reports = append(reports, finalRep)
			continue
		}
		if it.Title == "" {
			it.Title = entry.Title
		}
		items = append(items, it)
		reports = append(reports, finalRep)
	}
	return items, reports, nil
}

func (a *YouTubeAdapter) logger() *zap.Logger {
	if a != nil && a.scraper != nil && a.scraper.logger != nil {
		return a.scraper.logger
	}
	return zap.NewNop()
}

func (a *YouTubeAdapter) findExistingYouTube(videoID, canonical, rawURL string) *item.ScrapedItem {
	if a.scraper == nil || a.scraper.db == nil {
		return nil
	}
	if videoID != "" {
		if ex, _ := a.scraper.db.FindScrapedItemByVideoID(videoID); ex != nil {
			return ex
		}
	}
	if ex, _ := a.scraper.db.GetScrapedItemFullByURL(canonical); ex != nil {
		return ex
	}
	if rawURL != canonical {
		if ex, _ := a.scraper.db.GetScrapedItemFullByURL(rawURL); ex != nil {
			return ex
		}
	}
	return nil
}

func youtubeNeedsBackfill(ex *item.ScrapedItem) bool {
	if ex == nil {
		return true
	}
	meta := parseMetadata(ex.Metadata)
	if metaString(meta, "description") == "" {
		return true
	}
	if metaString(meta, "transcript") != "" {
		return false
	}
	switch metaString(meta, "caption_status") {
	case "unavailable":
		return false
	case "available":
		// Inconsistent legacy row: status says available but no text landed.
		return true
	case "failed", "unverified", "":
		return true
	}
	return true
}

func reportForExistingYouTube(ex *item.ScrapedItem, reason string) ItemReport {
	meta := parseMetadata(ex.Metadata)
	desc := metaString(meta, "description")
	status := metaString(meta, "caption_status")
	texts := []source.FetchTextResult{}
	if desc != "" {
		texts = append(texts, source.FetchTextResult{Kind: "description", Status: "kept_existing", Source: "library",
			Chars: utf8.RuneCountInString(desc)})
	} else {
		texts = append(texts, source.FetchTextResult{Kind: "description", Status: "kept_existing", Source: "library"})
	}
	capStatus := "kept_existing"
	if status == "unavailable" {
		capStatus = "no_public_text"
	}
	texts = append(texts, source.FetchTextResult{Kind: "transcript", Status: capStatus, Source: "library",
		Language: metaString(meta, "caption_language"), Chars: utf8.RuneCountInString(metaString(meta, "transcript"))})
	return ItemReport{
		Title: ex.Title, URL: ex.URL, Category: ex.Category, ItemID: ex.ID,
		Status: "skipped", Reason: reason, Texts: texts,
	}
}

func youtubeFallbackFromEntry(sourceID, canonical, channel string, entry *gofeed.Item, description string, fetchErr error) item.ScrapedItem {
	title := firstNonEmpty(entry.Title, "YouTube Video "+ExtractYouTubeVideoID(canonical), canonical)
	desc := strings.TrimSpace(description)
	meta := map[string]any{
		"video_id":          ExtractYouTubeVideoID(canonical),
		"channel":           firstNonEmpty(channel, "YouTube"),
		"description":       desc,
		"caption_status":    "unverified",
		"caption_error":     sanitizeText(fetchErr.Error()),
		"transcript_source": "none",
	}
	if desc == "" {
		meta["description_status"] = "no_public_text"
	} else {
		meta["description_status"] = "feed"
	}
	content := buildYouTubeMarkdown(title, firstNonEmpty(channel, "YouTube"), canonical, "", desc, "", "", "", 0)
	return item.ScrapedItem{
		SourceID:     sourceID,
		OriginSource: firstNonEmpty(channel, "YouTube"),
		Title:        title,
		URL:          canonical,
		Summary:      truncateSummary(firstNonEmpty(desc, title), 240),
		Content:      content,
		Category:     "youtube",
		PublishedAt:  entry.PublishedParsed,
		Metadata:     marshalMetadata(meta),
	}
}

func youtubeFallbackReport(canonical, title, channel string, fetchErr error, description string) ItemReport {
	texts := []source.FetchTextResult{}
	if strings.TrimSpace(description) != "" {
		texts = append(texts, source.FetchTextResult{Kind: "description", Status: "stored", Source: "youtube_feed",
			Chars: utf8.RuneCountInString(description)})
	} else {
		texts = append(texts, source.FetchTextResult{Kind: "description", Status: "no_public_text",
			Source: "youtube_feed", Reason: "频道 feed 未提供简介"})
	}
	texts = append(texts, source.FetchTextResult{Kind: "transcript", Status: "unverified", Source: "youtube_caption",
		Reason: "视频页临时获取失败，字幕状态未确认: " + fetchErr.Error(), Stage: "access"})
	return ItemReport{Title: firstNonEmpty(title, canonical), URL: canonical, Category: "youtube",
		Status: "stored", Reason: "视频页获取失败，已保留频道 feed 简介", ErrorStage: "access", Texts: texts}
}

func feedTitleOr(title, fallback string) string {
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return fallback
}

func isGenericYouTubeText(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(lower, "在 youtube 上畅享") ||
		strings.HasPrefix(lower, "enjoy the videos and music you love") ||
		lower == "- youtube" || lower == "youtube"
}

// youtubeFeedDescription pulls the original video description out of a YouTube
// Atom entry. gofeed stores <media:group><media:description> under generic
// extensions rather than Item.Description.
func youtubeFeedDescription(entry *gofeed.Item) string {
	if entry == nil {
		return ""
	}
	if d := strings.TrimSpace(entry.Description); d != "" {
		return d
	}
	if d := strings.TrimSpace(entry.Content); d != "" {
		return d
	}
	media, ok := entry.Extensions["media"]
	if !ok {
		return ""
	}
	for _, group := range media["group"] {
		for _, desc := range group.Children["description"] {
			if v := strings.TrimSpace(desc.Value); v != "" {
				return v
			}
		}
	}
	return ""
}

// ResolveYouTubeFeedURL converts a YouTube channel/handle/playlist URL into its Atom feed URL,
// or detects single video URLs.
func (a *YouTubeAdapter) ResolveYouTubeFeedURL(ctx context.Context, rawURL string) (feedURL string, videoID string, isSingleVideo bool, err error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", false, errors.New("empty YouTube URL")
	}

	if vid := ExtractYouTubeVideoID(trimmed); vid != "" {
		return "", vid, true, nil
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", "", false, err
	}

	if strings.Contains(u.Path, "/feeds/videos.xml") {
		return trimmed, "", false, nil
	}

	if listID := strings.TrimSpace(u.Query().Get("list")); listID != "" {
		return "https://www.youtube.com/feeds/videos.xml?playlist_id=" + url.QueryEscape(listID), "", false, nil
	}

	if m := ytChannelIDPathRegex.FindStringSubmatch(u.Path); len(m) > 1 {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + m[1], "", false, nil
	}

	// Handle /@handle, /c/..., /user/... via HTTP lookup first, then extension fallback.
	if chID, httpErr := a.resolveChannelIDHTTP(ctx, trimmed); httpErr == nil && chID != "" {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + chID, "", false, nil
	}

	if chID, extErr := a.resolveChannelIDExtension(ctx, trimmed); extErr == nil && chID != "" {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + chID, "", false, nil
	}

	return "", "", false, fmt.Errorf("could not resolve YouTube channel ID from %s", rawURL)
}

func (a *YouTubeAdapter) resolveChannelIDHTTP(ctx context.Context, channelURL string) (string, error) {
	body, err := a.fetchPageHTML(ctx, channelURL)
	if err != nil {
		return "", err
	}
	for _, re := range []*regexp.Regexp{ytChannelIDMetaRegex, ytChannelIDJSONRegex, ytChannelIDParamRegex} {
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			return m[1], nil
		}
	}
	return "", errors.New("channel ID not found in HTML")
}

func (a *YouTubeAdapter) resolveChannelIDExtension(ctx context.Context, channelURL string) (string, error) {
	if a.scraper == nil || a.scraper.wsManager == nil {
		return "", errors.New("no browser manager available")
	}
	connID, err := a.scraper.wsManager.ResolveBrowserConnID("")
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	resp, err := a.scraper.wsManager.SendMessage(reqCtx, &capture.BrowserRequest{
		Source:   "youtube_adapter",
		Action:   "mcp_request",
		Command:  "runPageScript",
		URL:      channelURL,
		Visible:  false,
		CloseTab: true,
		Params: map[string]any{
			"script": "youtube.resolveChannel",
		},
	}, connID)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", errors.New(resp.Error)
	}
	if chID, _ := resp.Result.JSON["channelId"].(string); chID != "" {
		return chID, nil
	}
	return "", errors.New("extension did not find channelId")
}

func (a *YouTubeAdapter) fetchAtomFeed(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Grabby/2.0)")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	fp := gofeed.NewParser()
	return fp.Parse(resp.Body)
}

type ytVideoPayload struct {
	VideoID          string
	Title            string
	Author           string
	ChannelID        string
	ShortDescription string
	LengthSeconds    int
	Transcript       string
	LanguageCode     string
	TranscriptSource string

	CaptionStatus    string // available, unavailable, failed, unverified
	CaptionError     string
	CaptionURL       string
	CaptionKind      string // "", "asr" (auto captions), or manual
	CaptionLanguages []string
}

// FetchYouTubeVideoItem extracts video details and converts captions (or ASR
// audio) into Markdown. Kept for the single-URL ingest path.
func (a *YouTubeAdapter) FetchYouTubeVideoItem(ctx context.Context, videoURL, sourceID string, allowASR bool, channelHint string, publishedAt *time.Time) (item.ScrapedItem, error) {
	it, _, err := a.fetchYouTubeVideo(ctx, videoURL, sourceID, allowASR, channelHint, publishedAt, "")
	return it, err
}

func (a *YouTubeAdapter) fetchYouTubeVideo(ctx context.Context, videoURL, sourceID string, allowASR bool, channelHint string, publishedAt *time.Time, feedDescription string) (item.ScrapedItem, ItemReport, error) {
	vid := ExtractYouTubeVideoID(videoURL)
	canonicalURL := strings.TrimSpace(videoURL)
	if vid != "" {
		canonicalURL = "https://www.youtube.com/watch?v=" + vid
	}

	var payload ytVideoPayload
	payload.VideoID = vid

	// 1. Try browser extension first (uses user's cookies/session, avoids 429).
	extErr := error(nil)
	if extPayload, err := a.extractVideoViaExtension(ctx, canonicalURL); err == nil {
		payload = extPayload
	} else {
		extErr = err
	}

	// 2. Fallback to direct HTTP extraction if extension unavailable or missed
	// description/transcript.
	if payload.Transcript == "" || payload.Title == "" || payload.ShortDescription == "" {
		httpPayload, httpErr := a.extractVideoViaHTTP(ctx, canonicalURL)
		if httpErr == nil {
			payload = mergeYouTubePayload(payload, httpPayload)
		} else if payload.Title == "" {
			return item.ScrapedItem{}, ItemReport{Title: canonicalURL, URL: canonicalURL, Category: "youtube",
				Status: "failed", Reason: "视频页获取失败: " + httpErr.Error(), ErrorStage: "access",
				Texts: []source.FetchTextResult{{Kind: "transcript", Status: "unverified", Source: "youtube_caption",
					URL: canonicalURL, Reason: httpErr.Error(), Stage: "access"}}}, httpErr
		}
	}

	if strings.TrimSpace(feedDescription) != "" {
		payload.ShortDescription = preferLonger(feedDescription, payload.ShortDescription)
	} else if isGenericYouTubeText(payload.ShortDescription) {
		payload.ShortDescription = ""
	}

	// 3. Fallback to ASR audio transcription when the video has no captions.
	if strings.TrimSpace(payload.Transcript) == "" && allowASR && a.scraper != nil && a.scraper.transcriber != nil {
		if asrText, err := a.scraper.transcriber.TranscribeMedia(ctx, canonicalURL, true); err == nil && strings.TrimSpace(asrText) != "" {
			payload.Transcript = strings.TrimSpace(asrText)
			payload.TranscriptSource = "asr"
			payload.CaptionStatus = "available"
		} else if err != nil && a.scraper.logger != nil {
			a.scraper.logger.Warn("YouTube ASR fallback failed", zap.String("url", canonicalURL), zap.Error(err))
		}
	}

	if payload.TranscriptSource == "" {
		switch {
		case payload.Transcript != "":
			payload.TranscriptSource = "youtube_captions"
		case payload.CaptionStatus == "":
			payload.CaptionStatus = "unverified"
		}
	}
	if payload.CaptionStatus == "" {
		if payload.Transcript != "" {
			payload.CaptionStatus = "available"
		} else if payload.CaptionKind == "" && len(payload.CaptionLanguages) == 0 {
			payload.CaptionStatus = "unverified"
		}
	}
	if payload.CaptionStatus == "available" && payload.Transcript == "" {
		payload.CaptionStatus = "failed"
		if payload.CaptionError == "" {
			payload.CaptionError = "caption track returned no text"
		}
	}
	if payload.CaptionError == "" && extErr != nil {
		payload.CaptionError = sanitizeText(extErr.Error())
	}

	author := firstNonEmpty(payload.Author, channelHint, "YouTube")
	title := firstNonEmpty(payload.Title, "YouTube Video "+payload.VideoID, canonicalURL)
	if strings.EqualFold(strings.TrimSpace(title), "- YouTube") || strings.EqualFold(strings.TrimSpace(title), "YouTube") {
		title = "YouTube Video " + payload.VideoID
	}
	durationStr := formatDurationSeconds(payload.LengthSeconds)
	desc := strings.TrimSpace(payload.ShortDescription)

	content := buildYouTubeMarkdown(title, author, canonicalURL, durationStr, desc, payload.Transcript,
		payload.TranscriptSource, payload.LanguageCode, payload.LengthSeconds)

	meta := map[string]any{
		"video_id":          payload.VideoID,
		"channel":           author,
		"channel_id":        payload.ChannelID,
		"duration":          durationStr,
		"length_seconds":    payload.LengthSeconds,
		"language_code":     payload.LanguageCode,
		"transcript_source": payload.TranscriptSource,
		"caption_status":    payload.CaptionStatus,
		"caption_language":  payload.LanguageCode,
		"caption_source":    payload.TranscriptSource,
		"caption_url":       payload.CaptionURL,
		"caption_error":     sanitizeText(payload.CaptionError),
		"caption_languages": payload.CaptionLanguages,
		"description":       desc,
		"transcript":        payload.Transcript,
	}
	if desc == "" {
		meta["description_status"] = "no_public_text"
	} else {
		meta["description_status"] = "stored"
	}

	summarySource := firstNonEmpty(desc, payload.Transcript, title)
	scraped := item.ScrapedItem{
		SourceID:     sourceID,
		OriginSource: author,
		Title:        title,
		URL:          canonicalURL,
		Summary:      truncateSummary(summarySource, 240),
		Content:      content,
		Category:     "youtube",
		PublishedAt:  publishedAt,
		Metadata:     marshalMetadata(meta),
	}

	rep := ItemReport{Title: title, URL: canonicalURL, Category: "youtube", Status: "stored"}
	rep.Texts = append(rep.Texts, youtubeDescriptionText(desc, feedDescription))
	rep.Texts = append(rep.Texts, youtubeTranscriptText(payload))
	return scraped, rep, nil
}

func mergeYouTubePayload(base, incoming ytVideoPayload) ytVideoPayload {
	if base.VideoID == "" {
		base.VideoID = incoming.VideoID
	}
	if base.Title == "" {
		base.Title = incoming.Title
	}
	if base.Author == "" {
		base.Author = incoming.Author
	}
	if base.ChannelID == "" {
		base.ChannelID = incoming.ChannelID
	}
	if base.ShortDescription == "" {
		base.ShortDescription = incoming.ShortDescription
	}
	if base.LengthSeconds == 0 {
		base.LengthSeconds = incoming.LengthSeconds
	}
	if base.Transcript == "" && incoming.Transcript != "" {
		base.Transcript = incoming.Transcript
		base.LanguageCode = incoming.LanguageCode
		base.TranscriptSource = incoming.TranscriptSource
		base.CaptionURL = incoming.CaptionURL
		base.CaptionKind = incoming.CaptionKind
	}
	if captionRank(incoming.CaptionStatus) > captionRank(base.CaptionStatus) {
		base.CaptionStatus = incoming.CaptionStatus
		base.CaptionError = incoming.CaptionError
	}
	if len(incoming.CaptionLanguages) > 0 {
		base.CaptionLanguages = mergeStrings(base.CaptionLanguages, incoming.CaptionLanguages)
	}
	return base
}

func captionRank(status string) int {
	switch status {
	case "available":
		return 4
	case "unavailable":
		return 3
	case "failed":
		return 1
	default: // "", unverified, unknown
		return 0
	}
}

func mergeStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{a, b} {
		for _, v := range list {
			v = strings.TrimSpace(v)
			if v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

func youtubeDescriptionText(desc, feedDesc string) source.FetchTextResult {
	if strings.TrimSpace(desc) == "" {
		return source.FetchTextResult{Kind: "description", Status: "no_public_text", Source: "youtube_page",
			Reason: "视频页没有公开简介"}
	}
	srcName := "youtube_page"
	if feedDesc != "" && strings.TrimSpace(desc) == strings.TrimSpace(feedDesc) {
		srcName = "youtube_feed"
	}
	return source.FetchTextResult{Kind: "description", Status: "stored", Source: srcName,
		Chars: utf8.RuneCountInString(desc)}
}

func youtubeTranscriptText(payload ytVideoPayload) source.FetchTextResult {
	res := source.FetchTextResult{Kind: "transcript", Source: payload.TranscriptSource,
		Language: payload.LanguageCode, URL: payload.CaptionURL}
	if res.Source == "" || res.Source == "none" {
		res.Source = "youtube_caption"
	}
	switch payload.CaptionStatus {
	case "available":
		res.Status = "stored"
		res.Chars = utf8.RuneCountInString(payload.Transcript)
	case "unavailable":
		res.Status = "no_public_text"
		res.Reason = "视频确认没有公开字幕"
	case "failed":
		res.Status = "failed"
		res.Stage = "access"
		res.Reason = prefString(payload.CaptionError, "字幕轨道获取失败")
	case "pending":
		res.Status = "pending"
		res.Reason = payload.CaptionError
	default:
		res.Status = "unverified"
		res.Stage = "access"
		res.Reason = prefString(payload.CaptionError, "字幕状态未能确认")
	}
	return res
}

func buildYouTubeMarkdown(title, author, url, duration, description, transcript, transcriptSource, language string, lengthSeconds int) string {
	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", title))
	md.WriteString(fmt.Sprintf("- **频道**: %s\n", author))
	if duration != "" {
		md.WriteString(fmt.Sprintf("- **时长**: %s\n", duration))
	}
	md.WriteString(fmt.Sprintf("- **视频链接**: %s\n", url))
	if language != "" {
		md.WriteString(fmt.Sprintf("- **文字来源**: %s (%s)\n", transcriptSource, language))
	} else {
		md.WriteString(fmt.Sprintf("- **文字来源**: %s\n", transcriptSource))
	}
	if strings.TrimSpace(description) != "" {
		md.WriteString("\n## 视频简介\n\n")
		md.WriteString(strings.TrimSpace(description) + "\n")
	}
	if strings.TrimSpace(transcript) != "" {
		md.WriteString("\n## 视频文字稿 (Transcript)\n\n")
		md.WriteString(strings.TrimSpace(transcript) + "\n")
	}
	return strings.TrimSpace(md.String())
}

// persistYouTubeItem merges a fetch result with an existing row. Description
// and transcript are never erased by a worse/incomplete result, and user state
// (read/starred) is untouched.
func (a *YouTubeAdapter) persistYouTubeItem(incoming item.ScrapedItem, rep ItemReport, existing *item.ScrapedItem) (item.ScrapedItem, ItemReport, error) {
	if a.scraper == nil || a.scraper.db == nil {
		return incoming, rep, nil
	}
	if existing == nil {
		id, err := a.scraper.db.InsertScrapedItem(incoming)
		if err != nil {
			return incoming, rep, err
		}
		if id == 0 {
			vid := ExtractYouTubeVideoID(incoming.URL)
			existing, _ = a.scraper.db.FindScrapedItemByVideoID(vid)
			if existing == nil {
				existing, _ = a.scraper.db.GetScrapedItemFullByURL(incoming.URL)
			}
			if existing == nil {
				return incoming, rep, fmt.Errorf("video row existed but could not be read back")
			}
		} else {
			rep.ItemID = id
			rep.Status = "stored"
			if a.scraper.aiEngine != nil {
				a.scraper.aiEngine.Enqueue(id)
			}
			return incoming, rep, nil
		}
	}

	exMeta := parseMetadata(existing.Metadata)
	inMeta := parseMetadata(incoming.Metadata)

	existingDesc := metaString(exMeta, "description")
	incomingDesc := metaString(inMeta, "description")
	mergedDesc := preferLonger(existingDesc, incomingDesc)

	existingTranscript := metaString(exMeta, "transcript")
	incomingTranscript := metaString(inMeta, "transcript")
	mergedTranscript := preferLonger(existingTranscript, incomingTranscript)

	incomingStatus := metaString(inMeta, "caption_status")
	existingStatus := metaString(exMeta, "caption_status")
	winnerMeta := inMeta
	if captionRank(existingStatus) > captionRank(incomingStatus) ||
		(captionRank(existingStatus) == captionRank(incomingStatus) && len(existingTranscript) > len(incomingTranscript)) {
		winnerMeta = exMeta
	}
	mergedStatus := metaString(winnerMeta, "caption_status")
	if mergedStatus == "" {
		mergedStatus = prefString(existingStatus, incomingStatus)
		if mergedStatus == "" {
			mergedStatus = "unverified"
		}
	}

	merged := map[string]any{}
	for k, v := range exMeta {
		merged[k] = v
	}
	for k, v := range inMeta {
		merged[k] = v
	}
	merged["description"] = mergedDesc
	merged["transcript"] = mergedTranscript
	merged["caption_status"] = mergedStatus
	merged["transcript_source"] = metaString(winnerMeta, "transcript_source")
	merged["caption_source"] = metaString(winnerMeta, "caption_source")
	merged["caption_language"] = metaString(winnerMeta, "caption_language")
	merged["language_code"] = prefString(metaString(winnerMeta, "language_code"), metaString(merged, "language_code"))
	merged["caption_url"] = metaString(winnerMeta, "caption_url")
	merged["caption_error"] = metaString(winnerMeta, "caption_error")
	merged["caption_languages"] = mergeAnyStringList(exMeta["caption_languages"], inMeta["caption_languages"])
	merged["video_id"] = prefString(metaString(inMeta, "video_id"), metaString(exMeta, "video_id"))
	merged["channel"] = prefString(metaString(inMeta, "channel"), metaString(exMeta, "channel"))
	merged["channel_id"] = prefString(metaString(inMeta, "channel_id"), metaString(exMeta, "channel_id"))
	merged["duration"] = prefString(metaString(inMeta, "duration"), metaString(exMeta, "duration"))
	merged["length_seconds"] = maxInt(anyInt(inMeta["length_seconds"]), anyInt(exMeta["length_seconds"]))
	if mergedDesc == "" {
		merged["description_status"] = "no_public_text"
	} else {
		merged["description_status"] = prefString(metaString(inMeta, "description_status"), metaString(exMeta, "description_status"))
	}

	title := existing.Title
	if strings.TrimSpace(incoming.Title) != "" {
		title = incoming.Title
	}
	if title == "" {
		title = existing.Title
	}
	content := buildYouTubeMarkdown(title, prefString(metaString(merged, "channel"), "YouTube"), existing.URL,
		metaString(merged, "duration"), mergedDesc, mergedTranscript, metaString(merged, "transcript_source"),
		metaString(merged, "caption_language"), anyInt(merged["length_seconds"]))
	summarySource := firstNonEmpty(mergedDesc, mergedTranscript, title)
	summary := truncateSummary(summarySource, 240)
	published := existing.PublishedAt
	if published == nil {
		published = incoming.PublishedAt
	}

	changed := content != existing.Content || title != existing.Title || mergedDesc != existingDesc ||
		mergedTranscript != existingTranscript || mergedStatus != existingStatus
	if !changed {
		rep.ItemID = existing.ID
		rep.Status = "kept_existing"
		rep.Reason = "本次结果未优于已有简介/字幕，保留原内容"
		rep.Texts = mergeKeptTexts(rep.Texts)
		return *existing, rep, nil
	}

	if err := a.scraper.db.UpdateScrapedItemFromScrape(existing.ID, title, summary, content, existing.Category,
		marshalMetadata(merged), published); err != nil {
		return *existing, rep, err
	}
	if mergedDesc == existingDesc && incomingDesc == "" && incomingTranscript == "" {
		rep.Status = "kept_existing"
	} else {
		rep.Status = "updated"
	}
	rep.ItemID = existing.ID
	rep.Title = title
	rep.Texts = reconcileYouTubeTexts(rep.Texts, exMeta, merged, incomingDesc, incomingTranscript)
	if a.scraper.aiEngine != nil {
		a.scraper.aiEngine.Enqueue(existing.ID)
	}
	updated := *existing
	updated.Title = title
	updated.Summary = summary
	updated.Content = content
	updated.Metadata = marshalMetadata(merged)
	updated.PublishedAt = published
	return updated, rep, nil
}

func mergeKeptTexts(texts []source.FetchTextResult) []source.FetchTextResult {
	for i := range texts {
		if texts[i].Status == "stored" || texts[i].Status == "updated" {
			texts[i].Status = "kept_existing"
		}
	}
	return texts
}

func reconcileYouTubeTexts(texts []source.FetchTextResult, exMeta map[string]any, merged map[string]any, incomingDesc, incomingTranscript string) []source.FetchTextResult {
	for i := range texts {
		switch texts[i].Kind {
		case "description":
			if (incomingDesc == "" || metaString(merged, "description") != incomingDesc) &&
				metaString(exMeta, "description") != "" {
				texts[i].Status = "kept_existing"
				texts[i].Source = "library"
			}
		case "transcript":
			if (incomingTranscript == "" || metaString(merged, "transcript") != incomingTranscript) &&
				metaString(exMeta, "transcript") != "" {
				texts[i].Status = "kept_existing"
				texts[i].Source = "library"
			}
			texts[i].Language = metaString(merged, "caption_language")
		}
	}
	return texts
}

func mergeAnyStringList(a, b any) []string {
	toStrings := func(v any) []string {
		switch list := v.(type) {
		case []string:
			return list
		case []any:
			out := make([]string, 0, len(list))
			for _, x := range list {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
		return nil
	}
	return mergeStrings(toStrings(a), toStrings(b))
}

func anyInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *YouTubeAdapter) extractVideoViaExtension(ctx context.Context, videoURL string) (ytVideoPayload, error) {
	if a.scraper == nil || a.scraper.wsManager == nil {
		return ytVideoPayload{}, errors.New("no browser manager")
	}
	connID, err := a.scraper.wsManager.ResolveBrowserConnID("")
	if err != nil {
		return ytVideoPayload{}, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()

	resp, err := a.scraper.wsManager.SendMessage(reqCtx, &capture.BrowserRequest{
		Source:   "youtube_adapter",
		Action:   "mcp_request",
		Command:  "runPageScript",
		URL:      videoURL,
		Visible:  false,
		CloseTab: true,
		Params: map[string]any{
			"script": "youtube.extractTranscript",
		},
	}, connID)
	if err != nil {
		return ytVideoPayload{}, err
	}
	if !resp.Success {
		return ytVideoPayload{}, errors.New(resp.Error)
	}
	if resp.Result.JSON == nil {
		return ytVideoPayload{}, errors.New("empty extension JSON result")
	}

	m := resp.Result.JSON
	p := ytVideoPayload{
		VideoID:          asString(m["videoId"]),
		Title:            asString(m["title"]),
		Author:           asString(m["author"]),
		ChannelID:        asString(m["channelId"]),
		ShortDescription: asString(m["shortDescription"]),
		Transcript:       asString(m["transcript"]),
		LanguageCode:     asString(m["languageCode"]),
		CaptionKind:      asString(m["captionKind"]),
	}
	if ls := asString(m["lengthSeconds"]); ls != "" {
		if sec, err := strconv.Atoi(ls); err == nil {
			p.LengthSeconds = sec
		}
	} else if f, ok := m["lengthSeconds"].(float64); ok {
		p.LengthSeconds = int(f)
	}
	if langs, ok := m["captionLanguages"].([]any); ok {
		for _, l := range langs {
			if s, ok := l.(string); ok && strings.TrimSpace(s) != "" {
				p.CaptionLanguages = append(p.CaptionLanguages, s)
			}
		}
	}
	switch {
	case p.Transcript != "":
		p.TranscriptSource = "youtube_captions"
		if p.CaptionKind == "asr" {
			p.TranscriptSource = "youtube_auto_captions"
		}
		p.CaptionStatus = "available"
	case strings.EqualFold(asString(m["captionStatus"]), "unavailable"):
		p.CaptionStatus = "unavailable"
		p.TranscriptSource = "none"
	default:
		p.CaptionStatus = "unverified"
		p.TranscriptSource = "none"
	}
	return p, nil
}

func (a *YouTubeAdapter) extractVideoViaHTTP(ctx context.Context, videoURL string) (ytVideoPayload, error) {
	htmlStr, err := a.fetchPageHTML(ctx, videoURL)
	if err != nil {
		return ytVideoPayload{}, err
	}
	payload, parseErr := parseYouTubePlayerResponseFromHTML(ctx, a.httpClient, htmlStr)
	if parseErr != nil {
		// Bot checks and consent screens hide ytInitialPlayerResponse behind a
		// login. OpenGraph metadata still exposes title/description, so keep
		// those and mark captions as unverified rather than "no captions".
		if fallback, ok := parseYouTubeMetaFallback(htmlStr); ok {
			fallback.CaptionStatus = "unverified"
			fallback.CaptionError = sanitizeText(parseErr.Error())
			fallback.TranscriptSource = "none"
			return fallback, nil
		}
		return payload, parseErr
	}
	return payload, nil
}

func parseYouTubePlayerResponseFromHTML(ctx context.Context, client *http.Client, htmlStr string) (ytVideoPayload, error) {
	jsonText := extractJSONObjectAfterMarker(htmlStr, "ytInitialPlayerResponse")
	if jsonText == "" {
		return ytVideoPayload{}, errors.New("ytInitialPlayerResponse not found in YouTube HTML")
	}

	var pr struct {
		PlayabilityStatus struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"playabilityStatus"`
		VideoDetails struct {
			VideoID          string `json:"videoId"`
			Title            string `json:"title"`
			Author           string `json:"author"`
			ChannelID        string `json:"channelId"`
			ShortDescription string `json:"shortDescription"`
			LengthSeconds    string `json:"lengthSeconds"`
		} `json:"videoDetails"`
		Captions struct {
			PlayerCaptionsTracklistRenderer struct {
				CaptionTracks []ytCaptionTrack `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err := json.Unmarshal([]byte(jsonText), &pr); err != nil {
		return ytVideoPayload{}, fmt.Errorf("parse ytInitialPlayerResponse: %w", err)
	}
	if pr.VideoDetails.VideoID == "" && pr.VideoDetails.Title == "" {
		reason := pr.PlayabilityStatus.Reason
		if reason == "" {
			reason = pr.PlayabilityStatus.Status
		}
		return ytVideoPayload{}, fmt.Errorf("YouTube player response unavailable: %s", reason)
	}

	p := ytVideoPayload{
		VideoID:          pr.VideoDetails.VideoID,
		Title:            pr.VideoDetails.Title,
		Author:           pr.VideoDetails.Author,
		ChannelID:        pr.VideoDetails.ChannelID,
		ShortDescription: pr.VideoDetails.ShortDescription,
	}
	if sec, err := strconv.Atoi(pr.VideoDetails.LengthSeconds); err == nil {
		p.LengthSeconds = sec
	}

	tracks := pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks
	if len(tracks) == 0 {
		p.CaptionStatus = "unavailable"
		p.TranscriptSource = "none"
		return p, nil
	}
	for _, tr := range tracks {
		label := tr.LanguageCode
		if tr.Kind == "asr" {
			label += " (auto)"
		}
		p.CaptionLanguages = append(p.CaptionLanguages, label)
	}

	picked, found := pickCaptionTrack(tracks)
	if !found {
		p.CaptionStatus = "unavailable"
		p.TranscriptSource = "none"
		return p, nil
	}

	p.LanguageCode = picked.LanguageCode
	p.CaptionKind = picked.Kind
	p.CaptionURL = picked.BaseURL
	p.CaptionStatus = "failed"
	if picked.BaseURL != "" && client != nil {
		transcript, err := fetchCaptionTrack(ctx, client, picked.BaseURL)
		if err == nil && strings.TrimSpace(transcript) != "" {
			p.Transcript = transcript
			p.CaptionStatus = "available"
			if picked.Kind == "asr" {
				p.TranscriptSource = "youtube_auto_captions"
			} else {
				p.TranscriptSource = "youtube_captions"
			}
			return p, nil
		}
		if err != nil {
			p.CaptionError = sanitizeText(err.Error())
		} else {
			p.CaptionError = "caption track returned no text"
		}
	}
	return p, nil
}

type ytCaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

// pickCaptionTrack prefers published/manual captions over automatic ones, and
// within each class prefers Chinese then English, then the first track.
func pickCaptionTrack(tracks []ytCaptionTrack) (ytCaptionTrack, bool) {
	if len(tracks) == 0 {
		return ytCaptionTrack{}, false
	}
	pickFrom := func(candidates []ytCaptionTrack) (ytCaptionTrack, bool) {
		if len(candidates) == 0 {
			return ytCaptionTrack{}, false
		}
		for _, tr := range candidates {
			if strings.HasPrefix(strings.ToLower(tr.LanguageCode), "zh") {
				return tr, true
			}
		}
		for _, tr := range candidates {
			if strings.HasPrefix(strings.ToLower(tr.LanguageCode), "en") {
				return tr, true
			}
		}
		return candidates[0], true
	}
	var manual, auto []ytCaptionTrack
	for _, tr := range tracks {
		if strings.EqualFold(tr.Kind, "asr") {
			auto = append(auto, tr)
		} else {
			manual = append(manual, tr)
		}
	}
	if picked, ok := pickFrom(manual); ok {
		return picked, true
	}
	return pickFrom(auto)
}

func fetchCaptionTrack(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	jsonURL := baseURL
	if strings.Contains(jsonURL, "fmt=") {
		jsonURL = regexp.MustCompile(`fmt=[^&]+`).ReplaceAllString(jsonURL, "fmt=json3")
	} else {
		jsonURL += "&fmt=json3"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			b, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
			if readErr == nil {
				if txt := parseYouTubeJSON3Captions(b); txt != "" {
					return txt, nil
				}
			}
		}
	}

	// Fallback to default XML caption format.
	xmlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return "", err
	}
	xmlReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	xmlResp, err := client.Do(xmlReq)
	if err != nil {
		return "", err
	}
	defer xmlResp.Body.Close()
	if xmlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("caption track HTTP %d", xmlResp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(xmlResp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return parseYouTubeXMLCaptions(b), nil
}

type captionCue struct {
	startMs int64
	text    string
}

func parseYouTubeJSON3Captions(raw []byte) string {
	var data struct {
		Events []struct {
			TStartMs int64 `json:"tStartMs"`
			Segs     []struct {
				UTF8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	var cues []captionCue
	for _, ev := range data.Events {
		var sb strings.Builder
		for _, seg := range ev.Segs {
			sb.WriteString(seg.UTF8)
		}
		txt := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(strings.ReplaceAll(sb.String(), "\n", " "), " "))
		if txt != "" {
			cues = append(cues, captionCue{startMs: ev.TStartMs, text: txt})
		}
	}
	return groupCaptionCues(cues)
}

func parseYouTubeXMLCaptions(raw []byte) string {
	var transcriptXML struct {
		Texts []struct {
			Start string `xml:"start,attr"`
			Text  string `xml:",chardata"`
		} `xml:"text"`
	}
	if err := xml.Unmarshal(raw, &transcriptXML); err != nil {
		return ""
	}
	var cues []captionCue
	for _, t := range transcriptXML.Texts {
		sec, _ := strconv.ParseFloat(t.Start, 64)
		txt := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(stdhtml.UnescapeString(t.Text), " "))
		if txt != "" {
			cues = append(cues, captionCue{startMs: int64(sec * 1000), text: txt})
		}
	}
	return groupCaptionCues(cues)
}

func groupCaptionCues(cues []captionCue) string {
	if len(cues) == 0 {
		return ""
	}
	var paragraphs []string
	bucketStart := cues[0].startMs
	var words []string
	for _, cue := range cues {
		if cue.startMs-bucketStart >= 30000 && len(words) > 0 {
			paragraphs = append(paragraphs, fmt.Sprintf("%s %s", formatTimestampMs(bucketStart), strings.Join(words, " ")))
			bucketStart = cue.startMs
			words = []string{cue.text}
		} else {
			words = append(words, cue.text)
		}
	}
	if len(words) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%s %s", formatTimestampMs(bucketStart), strings.Join(words, " ")))
	}
	return strings.Join(paragraphs, "\n\n")
}

func formatTimestampMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	totalSec := ms / 1000
	return fmt.Sprintf("[%02d:%02d]", totalSec/60, totalSec%60)
}

func formatDurationSeconds(sec int) string {
	if sec <= 0 {
		return ""
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (a *YouTubeAdapter) fetchPageHTML(ctx context.Context, pageURL string) (string, error) {
	if a.videoPageURLResolver != nil {
		if resolved := a.videoPageURLResolver(pageURL); resolved != "" {
			pageURL = resolved
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cookie", "CONSENT=YES+cb")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLFetchBytes))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseYouTubeMetaFallback reads title/description from OpenGraph tags when the
// player response is hidden by a login/consent interstitial.
func parseYouTubeMetaFallback(htmlStr string) (ytVideoPayload, bool) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ytVideoPayload{}, false
	}
	meta := func(selectors ...string) string {
		for _, sel := range selectors {
			if v := strings.TrimSpace(doc.Find(sel).AttrOr("content", "")); v != "" {
				return v
			}
		}
		return ""
	}
	title := meta(`meta[property="og:title"]`, `meta[name="title"]`)
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	desc := meta(`meta[property="og:description"]`, `meta[name="description"]`)
	if title == "" && desc == "" {
		return ytVideoPayload{}, false
	}
	p := ytVideoPayload{Title: title, ShortDescription: desc}
	if m := ytJSONVideoIDRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		p.VideoID = m[1]
	}
	if m := ytChannelIDJSONRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
		p.ChannelID = m[1]
	}
	return p, true
}

var ytJSONVideoIDRegex = regexp.MustCompile(`"videoId"\s*:\s*"([a-zA-Z0-9_-]{6,20})"`)

func extractJSONObjectAfterMarker(text, marker string) string {
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	start := strings.IndexByte(text[idx+len(marker):], '{')
	if start < 0 {
		return ""
	}
	start += idx + len(marker)
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inStr {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func truncateSummary(s string, maxRunes int) string {
	clean := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(strings.ReplaceAll(s, "\n", " "), " "))
	runes := []rune(clean)
	if len(runes) <= maxRunes {
		return clean
	}
	return string(runes[:maxRunes]) + "..."
}
