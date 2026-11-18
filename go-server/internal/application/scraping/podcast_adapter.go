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
		it := a.buildPodcastItem(ctx, src.ID, ep)
		return []item.ScrapedItem{it}, nil
	}

	// 2. Xiaoyuzhou podcast or single episode page
	if strings.Contains(strings.ToLower(rawURL), "xiaoyuzhoufm.com") {
		episodes, err := a.fetchXiaoyuzhouEpisodes(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return a.processEpisodes(ctx, src.ID, episodes, maxItems), nil
	}

	// 3. Apple Podcasts URL -> iTunes Lookup API -> standard RSS feed
	feedURL := rawURL
	var targetEpisodeID string
	if m := applePodcastIDRegex.FindStringSubmatch(rawURL); len(m) > 1 {
		resolved, err := a.resolveApplePodcastFeed(ctx, m[1])
		if err != nil {
			return nil, fmt.Errorf("resolve Apple Podcasts feed: %w", err)
		}
		feedURL = resolved
		if u, err := url.Parse(rawURL); err == nil {
			targetEpisodeID = strings.TrimSpace(u.Query().Get("i"))
		}
	}

	// 4. Standard Podcast RSS Feed
	episodes, err := a.fetchPodcastRSSEpisodes(ctx, feedURL, targetEpisodeID)
	if err != nil {
		return nil, err
	}
	return a.processEpisodes(ctx, src.ID, episodes, maxItems), nil
}

func (a *PodcastAdapter) processEpisodes(ctx context.Context, sourceID string, episodes []podcastEpisodeInput, maxItems int) []item.ScrapedItem {
	var out []item.ScrapedItem
	for _, ep := range episodes {
		if len(out) >= maxItems {
			break
		}
		if ep.URL == "" {
			continue
		}
		if a.scraper != nil && a.scraper.db != nil {
			if exists, err := a.scraper.db.ItemExistsByURL(ep.URL); err == nil && exists {
				continue
			}
		}

		out = append(out, a.buildPodcastItem(ctx, sourceID, ep))
	}
	return out
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
				audioURL = enc.URL
			}
		}

		epURL := strings.TrimSpace(fi.Link)
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
						transcriptLinks = append(transcriptLinks, u)
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

func (a *PodcastAdapter) buildPodcastItem(ctx context.Context, sourceID string, ep podcastEpisodeInput) item.ScrapedItem {
	showNotesMD := HTMLToMarkdown(ep.ShowNotesHTML)
	transcript := ""
	transcriptSource := ""

	// Tier 1: Podcast 2.0 <podcast:transcript> link
	for _, trURL := range ep.TranscriptLinks {
		if txt, err := a.fetchPodcastTranscript(ctx, trURL); err == nil && strings.TrimSpace(txt) != "" {
			transcript = txt
			transcriptSource = "podcast_transcript"
			break
		}
	}

	// Tier 2: Episode web page transcript (when Show Notes is short and webpage is an article)
	if transcript == "" && utf8.RuneCountInString(StripHTMLToText(ep.ShowNotesHTML)) < 400 && ep.URL != "" && ep.URL != ep.AudioURL {
		if _, webMD, err := ExtractWebArticle(ctx, ep.URL); err == nil && utf8.RuneCountInString(webMD) >= 800 {
			transcript = webMD
			transcriptSource = "web_article"
		}
	}

	if transcriptSource == "" {
		transcriptSource = "shownotes_only"
	}

	podcastName := firstNonEmpty(ep.PodcastName, "Podcast")
	title := firstNonEmpty(ep.Title, "Podcast Episode")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", title))
	md.WriteString(fmt.Sprintf("- **播客**: %s\n", podcastName))
	if ep.Duration != "" {
		md.WriteString(fmt.Sprintf("- **时长**: %s\n", ep.Duration))
	}
	md.WriteString(fmt.Sprintf("- **单集链接**: %s\n", ep.URL))
	if ep.AudioURL != "" {
		md.WriteString(fmt.Sprintf("- **音频链接**: %s\n", ep.AudioURL))
	}
	md.WriteString(fmt.Sprintf("- **文字来源**: %s\n", transcriptSource))

	if showNotesMD != "" {
		md.WriteString("\n## 节目简介 (Show Notes)\n\n")
		md.WriteString(showNotesMD + "\n")
	}
	if transcript != "" {
		md.WriteString("\n## 播客文字稿 (Transcript)\n\n")
		md.WriteString(transcript + "\n")
	}

	summarySrc := firstNonEmpty(transcript, showNotesMD, title)
	metaJSON := mustJSON(map[string]any{
		"podcast":           podcastName,
		"media_url":         ep.AudioURL,
		"duration":          ep.Duration,
		"transcript_source": transcriptSource,
	})

	return item.ScrapedItem{
		SourceID:     sourceID,
		OriginSource: podcastName,
		Title:        title,
		URL:          ep.URL,
		Summary:      truncateSummary(summarySrc, 240),
		Content:      strings.TrimSpace(md.String()),
		Category:     "podcast",
		PublishedAt:  ep.PublishedAt,
		Metadata:     metaJSON,
	}
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
