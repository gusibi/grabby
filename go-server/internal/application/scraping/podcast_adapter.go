package scraping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"

	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
)

var (
	applePodcastIDRegex = regexp.MustCompile(`podcasts\.apple\.com/.*/id(\d+)`)
	srtTimeLineRegex    = regexp.MustCompile(`^\d{1,2}:?\d{2}:\d{2}[.,]\d{3}\s*-->\s*.*$`)
	srtIndexLineRegex   = regexp.MustCompile(`^\d+$`)
)

// PodcastAdapter implements SourceAdapter for Podcast RSS, Apple Podcasts,
// Xiaoyuzhou links, and direct audio URLs.
type PodcastAdapter struct {
	scraper    *Scraper
	httpClient *http.Client
}

// NewPodcastAdapter creates a PodcastAdapter bound to the parent Scraper.
func NewPodcastAdapter(scraper *Scraper) *PodcastAdapter {
	return &PodcastAdapter{
		scraper:    scraper,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// IsPodcastURL reports whether rawURL looks like an Apple Podcasts, Xiaoyuzhou,
// or direct audio file URL.
func IsPodcastURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, "podcasts.apple.com") || strings.Contains(host, "xiaoyuzhoufm.com") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".mp3", ".m4a", ".wav", ".ogg", ".opus", ".aac":
		return true
	}
	return false
}

type podcastEpisodeInput struct {
	Title           string
	URL             string
	PodcastName     string
	ShowNotesHTML   string
	AudioURL        string
	Duration        string
	PublishedAt     *time.Time
	TranscriptLinks []string
}

// Fetch retrieves episodes for a podcast subscription or single episode link.
//
// Podcast audio is deliberately never downloaded here: scraping only reads the
// feed, transcript links, and (short) episode pages. The audio file is fetched
// by the player when the user presses play, which is also the only time it gets
// cached. ASR on the audio is therefore never part of the scrape path.
func (a *PodcastAdapter) Fetch(ctx context.Context, src source.Source) ([]item.ScrapedItem, error) {
	items, _, err := a.FetchWithReports(ctx, src)
	return items, err
}

// FetchWithReports reports show notes, audio and public transcript outcomes
// separately for each episode.
func (a *PodcastAdapter) FetchWithReports(ctx context.Context, src source.Source) ([]item.ScrapedItem, []ItemReport, error) {
	cfg := parseSourceRuntimeConfig(src.Config)
	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	rawURL := strings.TrimSpace(src.URL)

	// 1. Direct audio file URL (.mp3/.m4a)
	if isDirectAudioURL(rawURL) {
		ep := podcastEpisodeInput{
			Title:       firstNonEmpty(src.Name, filepath.Base(rawURL)),
			URL:         rawURL,
			PodcastName: firstNonEmpty(src.Name, "Podcast"),
			AudioURL:    rawURL,
		}
		return a.processEpisodesWithReports(ctx, src.ID, []podcastEpisodeInput{ep}, 1)
	}

	// 2. Xiaoyuzhou podcast or single episode page
	if strings.Contains(strings.ToLower(rawURL), "xiaoyuzhoufm.com") {
		episodes, err := a.fetchXiaoyuzhouEpisodes(ctx, rawURL)
		if err != nil {
			return nil, []ItemReport{{Title: src.URL, URL: src.URL, Status: "failed",
				Reason: "小宇宙页面获取失败: " + err.Error(), ErrorStage: "access",
				Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "xiaoyuzhou",
					URL: src.URL, Reason: err.Error(), Stage: "access"}}}}, err
		}
		return a.processEpisodesWithReports(ctx, src.ID, episodes, maxItems)
	}

	// 3. Apple Podcasts URL -> iTunes Lookup API -> standard RSS feed
	feedURL := rawURL
	var targetEpisodeID string
	if m := applePodcastIDRegex.FindStringSubmatch(rawURL); len(m) > 1 {
		resolved, err := a.resolveApplePodcastFeed(ctx, m[1])
		if err != nil {
			return nil, []ItemReport{{Title: src.URL, URL: src.URL, Status: "failed",
				Reason: "Apple Podcasts feed 解析失败: " + err.Error(), ErrorStage: "access"}}, fmt.Errorf("resolve Apple Podcasts feed: %w", err)
		}
		feedURL = resolved
		if u, err := url.Parse(rawURL); err == nil {
			targetEpisodeID = strings.TrimSpace(u.Query().Get("i"))
		}
	}

	// 4. Standard Podcast RSS Feed
	episodes, err := a.fetchPodcastRSSEpisodes(ctx, feedURL, targetEpisodeID)
	if err != nil {
		return nil, []ItemReport{{Title: src.URL, URL: src.URL, Status: "failed",
			Reason: "播客 RSS 获取失败: " + err.Error(), ErrorStage: "access",
			Texts: []source.FetchTextResult{{Kind: "content", Status: "failed", Source: "podcast_rss",
				URL: feedURL, Reason: err.Error(), Stage: "access"}}}}, err
	}
	return a.processEpisodesWithReports(ctx, src.ID, episodes, maxItems)
}

func (a *PodcastAdapter) processEpisodesWithReports(ctx context.Context, sourceID string, episodes []podcastEpisodeInput, maxItems int) ([]item.ScrapedItem, []ItemReport, error) {
	var out []item.ScrapedItem
	var reports []ItemReport
	attempts := 0
	loggedSkips := 0
	const maxLoggedSkips = 30
	appendSkip := func(rep ItemReport) {
		if loggedSkips >= maxLoggedSkips {
			return
		}
		loggedSkips++
		reports = append(reports, rep)
	}
	for _, ep := range episodes {
		if ep.URL == "" {
			continue
		}
		existing := a.findExistingEpisode(ep)
		if existing != nil {
			if !podcastNeedsBackfill(existing, ep) {
				appendSkip(reportForExistingPodcast(existing, "已有节目简介/文字稿，跳过"))
				continue
			}
			if attempts >= maxItems {
				appendSkip(reportForExistingPodcast(existing, "达到本轮处理上限，稍后补抓"))
				continue
			}
			attempts++
			incoming, rep := a.buildPodcastItem(ctx, sourceID, ep)
			it, rep2, err := a.mergePodcastItem(existing, incoming, rep)
			if err != nil {
				rep2.Status = "failed"
				rep2.Reason = "写入内容库失败: " + err.Error()
				rep2.ErrorStage = "persist"
				reports = append(reports, rep2)
				continue
			}
			out = append(out, it)
			reports = append(reports, rep2)
			continue
		}

		if attempts >= maxItems {
			appendSkip(ItemReport{Title: firstNonEmpty(ep.Title, ep.URL), URL: ep.URL, Category: "podcast",
				Status: "skipped", Reason: "达到本轮单集处理上限，稍后补抓"})
			continue
		}
		attempts++
		it, rep := a.buildPodcastItem(ctx, sourceID, ep)
		out = append(out, it)
		reports = append(reports, rep)
	}
	return out, reports, nil
}

func (a *PodcastAdapter) findExistingEpisode(ep podcastEpisodeInput) *item.ScrapedItem {
	if a.scraper == nil || a.scraper.db == nil {
		return nil
	}
	if ep.URL != "" {
		if ex, _ := a.scraper.db.GetScrapedItemFullByURL(ep.URL); ex != nil {
			return ex
		}
	}
	if ep.AudioURL != "" && ep.AudioURL != ep.URL {
		if ex, _ := a.scraper.db.GetScrapedItemFullByURL(ep.AudioURL); ex != nil {
			return ex
		}
	}
	return nil
}

func podcastNeedsBackfill(ex *item.ScrapedItem, ep podcastEpisodeInput) bool {
	meta := parseMetadata(ex.Metadata)
	switch metaString(meta, "transcript_status") {
	case "available":
		// Legacy rows may have status available but no stored transcript text.
		return metaString(meta, "transcript") == "" && !strings.Contains(ex.Content, "## 播客文字稿")
	case "failed", "unverified":
		return true
	case "no_public_text":
		return false
	}
	// Legacy rows predate transcript_status. A stored transcript is final;
	// otherwise retry only when the feed actually offers a transcript link.
	if strings.Contains(ex.Content, "## 播客文字稿") {
		return false
	}
	switch metaString(meta, "transcript_source") {
	case "podcast_transcript", "web_article", "shownotes_link":
		return false
	case "shownotes_only":
		// Feeds without a Podcast 2.0 transcript can still link a public
		// transcript from the show notes. Retry legacy rows so the newer
		// capability can upgrade them in place.
		return len(transcriptLinksFromShowNotes(ep.ShowNotesHTML, ep.URL)) > 0
	}
	return len(ep.TranscriptLinks) > 0
}

func reportForExistingPodcast(ex *item.ScrapedItem, reason string) ItemReport {
	meta := parseMetadata(ex.Metadata)
	status := metaString(meta, "transcript_status")
	if status == "" {
		switch metaString(meta, "transcript_source") {
		case "podcast_transcript", "web_article":
			status = "available"
		}
		if status == "" && metaString(meta, "transcript") != "" {
			status = "available"
		}
		if status == "" {
			status = "no_public_text"
		}
	}
	texts := []source.FetchTextResult{
		{Kind: "audio", Status: "kept_existing", Source: "library", URL: metaString(meta, "media_url")},
		{Kind: "show_notes", Status: "kept_existing", Source: "library", Chars: utf8.RuneCountInString(metaString(meta, "show_notes"))},
	}
	switch status {
	case "available":
		texts = append(texts, source.FetchTextResult{Kind: "transcript", Status: "kept_existing", Source: "library",
			URL: metaString(meta, "transcript_url"), Chars: utf8.RuneCountInString(metaString(meta, "transcript"))})
	case "failed":
		texts = append(texts, source.FetchTextResult{Kind: "transcript", Status: "failed", Source: "library",
			URL: metaString(meta, "transcript_url"), Reason: metaString(meta, "transcript_error"), Stage: "access"})
	default:
		texts = append(texts, source.FetchTextResult{Kind: "transcript", Status: "no_public_text", Source: "library"})
	}
	return ItemReport{Title: ex.Title, URL: ex.URL, Category: ex.Category, ItemID: ex.ID,
		Status: "skipped", Reason: reason, Texts: texts}
}

func (a *PodcastAdapter) resolveApplePodcastFeed(ctx context.Context, collectionID string) (string, error) {
	lookupURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s", url.QueryEscape(collectionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("iTunes lookup HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		Results []struct {
			FeedURL        string `json:"feedUrl"`
			CollectionName string `json:"collectionName"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&parsed); err != nil {
		return "", err
	}
	for _, r := range parsed.Results {
		if r.FeedURL != "" {
			return r.FeedURL, nil
		}
	}
	return "", fmt.Errorf("no feedUrl found for Apple Podcasts ID %s", collectionID)
}

func (a *PodcastAdapter) fetchPodcastRSSEpisodes(ctx context.Context, feedURL string, targetEpisodeID string) ([]podcastEpisodeInput, error) {
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
		return nil, fmt.Errorf("podcast RSS HTTP %d", resp.StatusCode)
	}

	fp := gofeed.NewParser()
	feed, err := fp.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse podcast RSS: %w", err)
	}

	var episodes []podcastEpisodeInput
	for _, fi := range feed.Items {
		var audioURL string
		for _, enc := range fi.Enclosures {
			if enc.URL != "" && (strings.HasPrefix(strings.ToLower(enc.Type), "audio") || isDirectAudioURL(enc.URL) || audioURL == "") {
				audioURL = resolveArticleURL(feedURL, enc.URL)
			}
		}

		epURL := resolveArticleURL(feedURL, strings.TrimSpace(fi.Link))
		if epURL == "" {
			epURL = audioURL
		}
		if epURL == "" {
			continue
		}

		if targetEpisodeID != "" && !strings.Contains(epURL, targetEpisodeID) && !strings.Contains(fi.GUID, targetEpisodeID) {
			continue
		}

		var transcriptLinks []string
		if podExts, ok := fi.Extensions["podcast"]; ok {
			if trExts, ok := podExts["transcript"]; ok {
				for _, tr := range trExts {
					if u := strings.TrimSpace(tr.Attrs["url"]); u != "" {
						transcriptLinks = append(transcriptLinks, resolveArticleURL(feedURL, u))
					}
				}
			}
		}

		var duration string
		if fi.ITunesExt != nil {
			duration = fi.ITunesExt.Duration
		}

		showNotes := fi.Content
		if showNotes == "" {
			showNotes = fi.Description
		}

		episodes = append(episodes, podcastEpisodeInput{
			Title:           fi.Title,
			URL:             epURL,
			PodcastName:     feed.Title,
			ShowNotesHTML:   showNotes,
			AudioURL:        audioURL,
			Duration:        duration,
			PublishedAt:     fi.PublishedParsed,
			TranscriptLinks: transcriptLinks,
		})
	}
	return episodes, nil
}

func (a *PodcastAdapter) fetchXiaoyuzhouEpisodes(ctx context.Context, pageURL string) ([]podcastEpisodeInput, error) {
	htmlStr, err := a.fetchHTMLWithExtensionFallback(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	return parseXiaoyuzhouHTML(htmlStr, pageURL)
}

func (a *PodcastAdapter) fetchHTMLWithExtensionFallback(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		resp, httpErr := a.httpClient.Do(req)
		if httpErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				b, readErr := io.ReadAll(io.LimitReader(resp.Body, maxHTMLFetchBytes))
				if readErr == nil && strings.Contains(string(b), "__NEXT_DATA__") {
					return string(b), nil
				}
			}
		}
	}

	if a.scraper != nil && a.scraper.wsManager != nil {
		if connID, err := a.scraper.wsManager.ResolveBrowserConnID(""); err == nil {
			reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			resp, err := a.scraper.wsManager.SendMessage(reqCtx, &capture.BrowserRequest{
				Source:   "podcast_adapter",
				Action:   "mcp_request",
				Command:  "fetchInPage",
				URL:      pageURL,
				Visible:  false,
				CloseTab: true,
				Params: map[string]any{
					"requestUrl": pageURL,
				},
			}, connID)
			if err == nil && resp.Success && strings.Contains(resp.Result.Text, "__NEXT_DATA__") {
				return resp.Result.Text, nil
			}
		}
	}

	return "", fmt.Errorf("failed to fetch Xiaoyuzhou page %s", pageURL)
}

func parseXiaoyuzhouHTML(htmlStr, pageURL string) ([]podcastEpisodeInput, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}
	nextData := strings.TrimSpace(doc.Find(`script#__NEXT_DATA__`).First().Text())
	if nextData == "" {
		return nil, errors.New("__NEXT_DATA__ not found in Xiaoyuzhou HTML")
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(nextData), &root); err != nil {
		return nil, fmt.Errorf("parse Xiaoyuzhou __NEXT_DATA__: %w", err)
	}

	props, _ := root["props"].(map[string]any)
	pageProps, _ := props["pageProps"].(map[string]any)
	if pageProps == nil {
		return nil, errors.New("missing pageProps in Xiaoyuzhou data")
	}

	parseEpisodeMap := func(epMap map[string]any, podcastTitleFallback string) (podcastEpisodeInput, bool) {
		if epMap == nil {
			return podcastEpisodeInput{}, false
		}
		eid := asString(epMap["eid"])
		title := asString(epMap["title"])
		if title == "" {
			return podcastEpisodeInput{}, false
		}
		epURL := pageURL
		if eid != "" {
			epURL = "https://www.xiaoyuzhoufm.com/episode/" + eid
		}

		podcastName := podcastTitleFallback
		if podMap, ok := epMap["podcast"].(map[string]any); ok {
			if t := asString(podMap["title"]); t != "" {
				podcastName = t
			}
		}

		showNotes := firstNonEmpty(asString(epMap["shownotes"]), asString(epMap["description"]))

		var audioURL string
		if enc, ok := epMap["enclosure"].(map[string]any); ok {
			audioURL = asString(enc["url"])
		}
		if audioURL == "" {
			if media, ok := epMap["media"].(map[string]any); ok {
				if src, ok := media["source"].(map[string]any); ok {
					audioURL = asString(src["url"])
				}
			}
		}

		var duration string
		if dFloat, ok := epMap["duration"].(float64); ok && dFloat > 0 {
			duration = formatDurationSeconds(int(dFloat))
		}

		var pubAt *time.Time
		if pubStr := asString(epMap["pubDate"]); pubStr != "" {
			if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
				pubAt = &t
			}
		}

		return podcastEpisodeInput{
			Title:         title,
			URL:           epURL,
			PodcastName:   firstNonEmpty(podcastName, "小宇宙"),
			ShowNotesHTML: showNotes,
			AudioURL:      audioURL,
			Duration:      duration,
			PublishedAt:   pubAt,
		}, true
	}

	if epMap, ok := pageProps["episode"].(map[string]any); ok {
		if ep, valid := parseEpisodeMap(epMap, "小宇宙"); valid {
			return []podcastEpisodeInput{ep}, nil
		}
	}

	var podcastTitle string
	var rawList []any
	if podMap, ok := pageProps["podcast"].(map[string]any); ok {
		podcastTitle = asString(podMap["title"])
		if eps, ok := podMap["episodes"].([]any); ok {
			rawList = eps
		}
	}
	if len(rawList) == 0 {
		if eps, ok := pageProps["episodes"].([]any); ok {
			rawList = eps
		}
	}

	var out []podcastEpisodeInput
	for _, raw := range rawList {
		epMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if ep, valid := parseEpisodeMap(epMap, podcastTitle); valid {
			out = append(out, ep)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no episodes found in Xiaoyuzhou page")
	}
	return out, nil
}

func (a *PodcastAdapter) buildPodcastItem(ctx context.Context, sourceID string, ep podcastEpisodeInput) (item.ScrapedItem, ItemReport) {
	showNotesMD := HTMLToMarkdown(ep.ShowNotesHTML)
	transcript := ""
	transcriptStatus := ""
	transcriptSource := ""
	transcriptURL := ""
	transcriptErr := ""

	// Tier 1: Podcast 2.0 <podcast:transcript> link. Only this public link is
	// downloaded; the audio enclosure is never fetched.
	if len(ep.TranscriptLinks) > 0 {
		for _, trURL := range ep.TranscriptLinks {
			txt, err := a.fetchPodcastTranscript(ctx, trURL)
			if err == nil && strings.TrimSpace(txt) != "" {
				transcript = txt
				transcriptStatus = "available"
				transcriptSource = "podcast_transcript"
				transcriptURL = trURL
				transcriptErr = ""
				break
			}
			if err != nil {
				transcriptErr = err.Error()
				transcriptURL = trURL
			}
		}
		if transcriptStatus == "" {
			transcriptStatus = "failed"
			if transcriptErr == "" {
				transcriptErr = "transcript link returned empty text"
			}
		}
	}

	// Tier 1b: a public transcript linked from the show notes (for example
	// "Find a full transcript for this episode: <link>"). Reuses the article
	// extractor; only longer, article-like pages are accepted as transcripts.
	if transcriptStatus != "available" {
		for _, link := range transcriptLinksFromShowNotes(ep.ShowNotesHTML, ep.URL) {
			res, err := ExtractWebArticleDetails(ctx, link)
			if err == nil && utf8.RuneCountInString(strings.TrimSpace(res.Markdown)) >= 800 {
				transcript = res.Markdown
				transcriptStatus = "available"
				transcriptSource = "shownotes_link"
				transcriptURL = link
				transcriptErr = ""
				break
			}
			if transcriptStatus == "" {
				transcriptStatus = "unverified"
				transcriptURL = link
				if err != nil {
					transcriptErr = err.Error()
				} else {
					transcriptErr = "linked transcript page had too little text"
				}
			}
		}
	}

	// Tier 2: Episode web page transcript (when Show Notes is short and the
	// page is an article rather than the audio file).
	if transcriptStatus == "" && utf8.RuneCountInString(StripHTMLToText(ep.ShowNotesHTML)) < 400 && ep.URL != "" && ep.URL != ep.AudioURL {
		if _, webMD, err := ExtractWebArticle(ctx, ep.URL); err == nil && utf8.RuneCountInString(webMD) >= 800 {
			transcript = webMD
			transcriptStatus = "available"
			transcriptSource = "web_article"
			transcriptURL = ep.URL
		} else if err != nil {
			transcriptStatus = "unverified"
			transcriptErr = err.Error()
			transcriptURL = ep.URL
		}
	}
	if transcriptStatus == "" {
		transcriptStatus = "no_public_text"
		transcriptSource = "shownotes_only"
	}
	if transcriptStatus != "available" && transcriptSource == "" {
		transcriptSource = "none"
	}

	podcastName := firstNonEmpty(ep.PodcastName, "Podcast")
	title := firstNonEmpty(ep.Title, "Podcast Episode")
	content := buildPodcastMarkdown(title, podcastName, ep.Duration, ep.URL, ep.AudioURL, showNotesMD, transcript, transcriptSource)

	summarySrc := firstNonEmpty(transcript, showNotesMD, title)
	meta := map[string]any{
		"podcast":           podcastName,
		"media_url":         ep.AudioURL,
		"duration":          ep.Duration,
		"transcript_source": transcriptSource,
		"transcript_status": transcriptStatus,
		"transcript_url":    transcriptURL,
		"transcript_error":  sanitizeText(transcriptErr),
		"show_notes":        showNotesMD,
		"transcript":        transcript,
	}

	rep := ItemReport{Title: title, URL: ep.URL, Category: "podcast", Status: "stored"}
	if showNotesMD != "" {
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "show_notes", Status: "stored", Source: "podcast_feed",
			Chars: utf8.RuneCountInString(showNotesMD)})
	} else {
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "show_notes", Status: "no_public_text", Source: "podcast_feed",
			Reason: "Feed 未提供节目简介"})
	}
	if ep.AudioURL != "" {
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "audio", Status: "stored", Source: "audio_enclosure",
			URL: ep.AudioURL})
	} else {
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "audio", Status: "no_public_text", Source: "podcast_feed",
			Reason: "Feed 未提供音频链接"})
	}
	switch transcriptStatus {
	case "available":
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "transcript", Status: "stored",
			Source: transcriptSource, URL: transcriptURL, Chars: utf8.RuneCountInString(transcript)})
	case "failed":
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "transcript", Status: "failed",
			Source: "transcript_link", URL: transcriptURL, Reason: transcriptErr, Stage: "access"})
	case "unverified":
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "transcript", Status: "unverified",
			Source: "web_page", URL: transcriptURL, Reason: transcriptErr, Stage: "access"})
	default:
		rep.Texts = append(rep.Texts, source.FetchTextResult{Kind: "transcript", Status: "no_public_text",
			Source: "podcast_feed", Reason: "Feed 未提供公开文字稿"})
	}

	return item.ScrapedItem{
		SourceID:     sourceID,
		OriginSource: podcastName,
		Title:        title,
		URL:          ep.URL,
		Summary:      truncateSummary(summarySrc, 240),
		Content:      content,
		Category:     "podcast",
		PublishedAt:  ep.PublishedAt,
		Metadata:     marshalMetadata(meta),
	}, rep
}

func transcriptLinksFromShowNotes(rawHTML, episodeURL string) []string {
	trimmed := strings.TrimSpace(rawHTML)
	if trimmed == "" || !strings.Contains(trimmed, "<") {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + trimmed + "</body>"))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var links []string
	doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
		href := strings.TrimSpace(sel.AttrOr("href", ""))
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || isDirectAudioURL(href) {
			return
		}
		contextText := strings.ToLower(sel.Closest("p, li, div").Text())
		label := strings.ToLower(strings.TrimSpace(sel.Text()))
		if strings.Contains(contextText, "transcript") || strings.Contains(label, "transcript") ||
			strings.Contains(strings.ToLower(href), "transcript") {
			abs := resolveArticleURL(episodeURL, href)
			if abs != "" && !seen[abs] {
				seen[abs] = true
				links = append(links, abs)
			}
		}
	})
	if len(links) > 3 {
		links = links[:3]
	}
	return links
}

func buildPodcastMarkdown(title, podcastName, duration, episodeURL, audioURL, showNotesMD, transcript, transcriptSource string) string {
	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", title))
	md.WriteString(fmt.Sprintf("- **播客**: %s\n", podcastName))
	if duration != "" {
		md.WriteString(fmt.Sprintf("- **时长**: %s\n", duration))
	}
	if episodeURL != "" {
		md.WriteString(fmt.Sprintf("- **单集链接**: %s\n", episodeURL))
	}
	if audioURL != "" {
		md.WriteString(fmt.Sprintf("- **音频链接**: %s\n", audioURL))
	}
	if transcriptSource != "" {
		md.WriteString(fmt.Sprintf("- **文字来源**: %s\n", transcriptSource))
	}
	if strings.TrimSpace(showNotesMD) != "" {
		md.WriteString("\n## 节目简介 (Show Notes)\n\n")
		md.WriteString(strings.TrimSpace(showNotesMD) + "\n")
	}
	if strings.TrimSpace(transcript) != "" {
		md.WriteString("\n## 播客文字稿 (Transcript)\n\n")
		md.WriteString(strings.TrimSpace(transcript) + "\n")
	}
	return strings.TrimSpace(md.String())
}

func podcastTranscriptRank(status string) int {
	switch status {
	case "available":
		return 3
	case "no_public_text":
		return 2
	case "failed":
		return 1
	default:
		return 0
	}
}

// mergePodcastItem keeps show notes, audio and any imported transcript when a
// later run only confirms "still unavailable" or fails temporarily.
func (a *PodcastAdapter) mergePodcastItem(existing *item.ScrapedItem, incoming item.ScrapedItem, rep ItemReport) (item.ScrapedItem, ItemReport, error) {
	exMeta := parseMetadata(existing.Metadata)
	inMeta := parseMetadata(incoming.Metadata)

	mergedShow := preferLonger(metaString(exMeta, "show_notes"), metaString(inMeta, "show_notes"))
	mergedTranscript := preferLonger(metaString(exMeta, "transcript"), metaString(inMeta, "transcript"))
	status := metaString(inMeta, "transcript_status")
	sourceName := metaString(inMeta, "transcript_source")
	transcriptURL := metaString(inMeta, "transcript_url")
	transcriptErr := metaString(inMeta, "transcript_error")
	if podcastTranscriptRank(metaString(exMeta, "transcript_status")) > podcastTranscriptRank(status) ||
		(podcastTranscriptRank(metaString(exMeta, "transcript_status")) == podcastTranscriptRank(status) &&
			len(metaString(exMeta, "transcript")) > len(metaString(inMeta, "transcript"))) {
		status = metaString(exMeta, "transcript_status")
		sourceName = metaString(exMeta, "transcript_source")
		transcriptURL = metaString(exMeta, "transcript_url")
		transcriptErr = metaString(exMeta, "transcript_error")
	}
	audio := prefString(metaString(exMeta, "media_url"), metaString(inMeta, "media_url"))
	duration := prefString(metaString(exMeta, "duration"), metaString(inMeta, "duration"))
	podcastName := prefString(metaString(exMeta, "podcast"), metaString(inMeta, "podcast"))
	title := prefString(incoming.Title, existing.Title)
	sourceID := existing.SourceID
	if sourceID == "" {
		sourceID = incoming.SourceID
	}

	merged := map[string]any{}
	for k, v := range exMeta {
		merged[k] = v
	}
	for k, v := range inMeta {
		merged[k] = v
	}
	merged["show_notes"] = mergedShow
	merged["transcript"] = mergedTranscript
	merged["transcript_status"] = status
	merged["transcript_source"] = sourceName
	merged["transcript_url"] = transcriptURL
	merged["transcript_error"] = transcriptErr
	merged["media_url"] = audio
	merged["duration"] = duration
	merged["podcast"] = podcastName

	content := buildPodcastMarkdown(title, podcastName, duration, existing.URL, audio, mergedShow, mergedTranscript, sourceName)
	summary := truncateSummary(firstNonEmpty(mergedTranscript, mergedShow, title), 240)
	published := existing.PublishedAt
	if published == nil {
		published = incoming.PublishedAt
	}
	changed := content != existing.Content || title != existing.Title ||
		mergedShow != metaString(exMeta, "show_notes") ||
		mergedTranscript != metaString(exMeta, "transcript") ||
		status != metaString(exMeta, "transcript_status")
	if !changed {
		rep.ItemID = existing.ID
		rep.Status = "kept_existing"
		rep.Reason = "本轮结果未优于已有简介/文字稿，保留原内容"
		rep.Texts = mergeKeptTexts(rep.Texts)
		return *existing, rep, nil
	}

	if err := a.scraper.db.UpdateScrapedItemFromScrape(existing.ID, title, summary, content, existing.Category,
		marshalMetadata(merged), published); err != nil {
		return *existing, rep, err
	}
	rep.ItemID = existing.ID
	rep.Title = title
	rep.Status = "updated"
	for i := range rep.Texts {
		if rep.Texts[i].Kind == "transcript" {
			if status != "available" && metaString(exMeta, "transcript") != "" {
				rep.Texts[i].Status = "kept_existing"
				rep.Texts[i].Source = "library"
			}
		}
	}
	updated := *existing
	updated.Title = title
	updated.Summary = summary
	updated.Content = content
	updated.Metadata = marshalMetadata(merged)
	updated.PublishedAt = published
	return updated, rep, nil
}

func (a *PodcastAdapter) fetchPodcastTranscript(ctx context.Context, trURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Grabby/2.0)")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcript HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(b))
	if strings.HasPrefix(raw, "{") {
		var jt struct {
			Segments []struct {
				Speaker string `json:"speaker"`
				Body    string `json:"body"`
			} `json:"segments"`
		}
		if json.Unmarshal(b, &jt) == nil && len(jt.Segments) > 0 {
			var lines []string
			for _, seg := range jt.Segments {
				txt := strings.TrimSpace(seg.Body)
				if txt == "" {
					continue
				}
				if seg.Speaker != "" {
					lines = append(lines, fmt.Sprintf("**%s**: %s", seg.Speaker, txt))
				} else {
					lines = append(lines, txt)
				}
			}
			return strings.Join(lines, "\n\n"), nil
		}
	}
	return cleanSRTOrVTT(raw), nil
}

func cleanSRTOrVTT(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []string
	var cur strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "WEBVTT") || strings.HasPrefix(trimmed, "NOTE ") {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		if srtIndexLineRegex.MatchString(trimmed) || srtTimeLineRegex.MatchString(trimmed) {
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(trimmed)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return strings.Join(out, "\n\n")
}

func isDirectAudioURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	switch ext {
	case ".mp3", ".m4a", ".wav", ".ogg", ".opus", ".aac":
		return true
	}
	return false
}

// FetchPodcastEpisodeItem fetches a single podcast episode URL (Xiaoyuzhou/Apple/audio)
// and turns it into a ScrapedItem. It never downloads audio; playback does that.
func (a *PodcastAdapter) FetchPodcastEpisodeItem(ctx context.Context, rawURL, sourceID string) (item.ScrapedItem, error) {
	src := source.Source{
		ID:     sourceID,
		Type:   "podcast",
		URL:    rawURL,
		Config: `{"max_items":1}`,
	}
	items, err := a.Fetch(ctx, src)
	if err != nil {
		return item.ScrapedItem{}, err
	}
	if len(items) == 0 {
		return item.ScrapedItem{}, fmt.Errorf("no podcast episode resolved from %s", rawURL)
	}
	return items[0], nil
}

func strconvItoaSafe(v int) string {
	return strconv.Itoa(v)
}
