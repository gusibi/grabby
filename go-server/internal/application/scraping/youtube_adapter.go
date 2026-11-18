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
	cfg := parseSourceRuntimeConfig(src.Config)
	allowASR := cfg.transcribeAudioEnabled()

	feedURL, videoID, isSingleVideo, err := a.ResolveYouTubeFeedURL(ctx, src.URL)
	if err != nil {
		return nil, err
	}

	if isSingleVideo {
		canonical := "https://www.youtube.com/watch?v=" + videoID
		it, err := a.FetchYouTubeVideoItem(ctx, canonical, src.ID, allowASR, "", nil)
		if err != nil {
			return nil, err
		}
		return []item.ScrapedItem{it}, nil
	}

	feed, err := a.fetchAtomFeed(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("fetch YouTube feed %s: %w", feedURL, err)
	}

	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	var items []item.ScrapedItem
	for _, entry := range feed.Items {
		if len(items) >= maxItems {
			break
		}
		videoURL := strings.TrimSpace(entry.Link)
		if videoURL == "" {
			continue
		}
		// Skip already collected videos during scheduled polling so transcripts
		// are not re-fetched or re-transcribed unnecessarily.
		if a.scraper != nil && a.scraper.db != nil {
			if exists, err := a.scraper.db.ItemExistsByURL(videoURL); err == nil && exists {
				continue
			}
		}

		it, err := a.FetchYouTubeVideoItem(ctx, videoURL, src.ID, allowASR, feed.Title, entry.PublishedParsed)
		if err != nil {
			if a.scraper != nil && a.scraper.logger != nil {
				a.scraper.logger.Warn("Failed to extract YouTube video", zap.String("url", videoURL), zap.Error(err))
			}
			// Fallback to Atom entry metadata so the video isn't silently lost.
			fallbackContent := strings.TrimSpace(entry.Description)
			if fallbackContent == "" {
				fallbackContent = entry.Title
			}
			it = item.ScrapedItem{
				SourceID:     src.ID,
				OriginSource: firstNonEmpty(feed.Title, "YouTube"),
				Title:        firstNonEmpty(entry.Title, videoURL),
				URL:          videoURL,
				Summary:      truncateSummary(fallbackContent, 240),
				Content:      fallbackContent,
				Category:     "youtube",
				PublishedAt:  entry.PublishedParsed,
				Metadata:     mustJSON(map[string]any{"video_id": ExtractYouTubeVideoID(videoURL), "transcript_source": "feed_fallback"}),
			}
		}
		if it.Title == "" {
			it.Title = entry.Title
		}
		if it.PublishedAt == nil && entry.PublishedParsed != nil {
			it.PublishedAt = entry.PublishedParsed
		}
		items = append(items, it)
	}
	return items, nil
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
}

// FetchYouTubeVideoItem extracts video details and converts captions (or ASR audio) into Markdown.
func (a *YouTubeAdapter) FetchYouTubeVideoItem(ctx context.Context, videoURL, sourceID string, allowASR bool, channelHint string, publishedAt *time.Time) (item.ScrapedItem, error) {
	vid := ExtractYouTubeVideoID(videoURL)
	canonicalURL := strings.TrimSpace(videoURL)
	if vid != "" {
		canonicalURL = "https://www.youtube.com/watch?v=" + vid
	}

	var payload ytVideoPayload
	payload.VideoID = vid

	// 1. Try browser extension first (uses user's cookies/session, avoids 429).
	if extPayload, err := a.extractVideoViaExtension(ctx, canonicalURL); err == nil {
		payload = extPayload
	}

	// 2. Fallback to direct HTTP extraction if extension unavailable or missed transcript.
	if payload.Transcript == "" || payload.Title == "" {
		if httpPayload, err := a.extractVideoViaHTTP(ctx, canonicalURL); err == nil {
			if payload.VideoID == "" {
				payload.VideoID = httpPayload.VideoID
			}
			if payload.Title == "" {
				payload.Title = httpPayload.Title
			}
			if payload.Author == "" {
				payload.Author = httpPayload.Author
			}
			if payload.ChannelID == "" {
				payload.ChannelID = httpPayload.ChannelID
			}
			if payload.ShortDescription == "" {
				payload.ShortDescription = httpPayload.ShortDescription
			}
			if payload.LengthSeconds == 0 {
				payload.LengthSeconds = httpPayload.LengthSeconds
			}
			if payload.Transcript == "" && httpPayload.Transcript != "" {
				payload.Transcript = httpPayload.Transcript
				payload.LanguageCode = httpPayload.LanguageCode
				payload.TranscriptSource = httpPayload.TranscriptSource
			}
		} else if payload.Title == "" {
			return item.ScrapedItem{}, err
		}
	}

	// 3. Fallback to ASR audio transcription when the video has no captions.
	if strings.TrimSpace(payload.Transcript) == "" && allowASR && a.scraper != nil && a.scraper.transcriber != nil {
		if asrText, err := a.scraper.transcriber.TranscribeMedia(ctx, canonicalURL, true); err == nil && strings.TrimSpace(asrText) != "" {
			payload.Transcript = strings.TrimSpace(asrText)
			payload.TranscriptSource = "asr"
		} else if err != nil && a.scraper.logger != nil {
			a.scraper.logger.Warn("YouTube ASR fallback failed", zap.String("url", canonicalURL), zap.Error(err))
		}
	}

	if payload.TranscriptSource == "" {
		if payload.Transcript != "" {
			payload.TranscriptSource = "youtube_captions"
		} else {
			payload.TranscriptSource = "none"
		}
	}

	author := firstNonEmpty(payload.Author, channelHint, "YouTube")
	title := firstNonEmpty(payload.Title, "YouTube Video "+payload.VideoID)
	durationStr := formatDurationSeconds(payload.LengthSeconds)

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", title))
	md.WriteString(fmt.Sprintf("- **频道**: %s\n", author))
	if durationStr != "" {
		md.WriteString(fmt.Sprintf("- **时长**: %s\n", durationStr))
	}
	md.WriteString(fmt.Sprintf("- **视频链接**: %s\n", canonicalURL))
	if payload.LanguageCode != "" {
		md.WriteString(fmt.Sprintf("- **文字来源**: %s (%s)\n", payload.TranscriptSource, payload.LanguageCode))
	} else {
		md.WriteString(fmt.Sprintf("- **文字来源**: %s\n", payload.TranscriptSource))
	}
	if strings.TrimSpace(payload.ShortDescription) != "" {
		md.WriteString("\n## 视频简介\n\n")
		md.WriteString(strings.TrimSpace(payload.ShortDescription) + "\n")
	}
	if strings.TrimSpace(payload.Transcript) != "" {
		md.WriteString("\n## 视频文字稿 (Transcript)\n\n")
		md.WriteString(strings.TrimSpace(payload.Transcript) + "\n")
	}

	summarySource := firstNonEmpty(payload.Transcript, payload.ShortDescription, title)
	metaJSON := mustJSON(map[string]any{
		"video_id":          payload.VideoID,
		"channel":           author,
		"channel_id":        payload.ChannelID,
		"duration":          durationStr,
		"length_seconds":    payload.LengthSeconds,
		"language_code":     payload.LanguageCode,
		"transcript_source": payload.TranscriptSource,
	})

	return item.ScrapedItem{
		SourceID:     sourceID,
		OriginSource: author,
		Title:        title,
		URL:          canonicalURL,
		Summary:      truncateSummary(summarySource, 240),
		Content:      strings.TrimSpace(md.String()),
		Category:     "youtube",
		PublishedAt:  publishedAt,
		Metadata:     metaJSON,
	}, nil
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
	}
	if ls := asString(m["lengthSeconds"]); ls != "" {
		if sec, err := strconv.Atoi(ls); err == nil {
			p.LengthSeconds = sec
		}
	} else if f, ok := m["lengthSeconds"].(float64); ok {
		p.LengthSeconds = int(f)
	}
	if p.Transcript != "" {
		p.TranscriptSource = "extension_captions"
	}
	return p, nil
}

func (a *YouTubeAdapter) extractVideoViaHTTP(ctx context.Context, videoURL string) (ytVideoPayload, error) {
	htmlStr, err := a.fetchPageHTML(ctx, videoURL)
	if err != nil {
		return ytVideoPayload{}, err
	}
	return parseYouTubePlayerResponseFromHTML(ctx, a.httpClient, htmlStr)
}

func parseYouTubePlayerResponseFromHTML(ctx context.Context, client *http.Client, htmlStr string) (ytVideoPayload, error) {
	jsonText := extractJSONObjectAfterMarker(htmlStr, "ytInitialPlayerResponse")
	if jsonText == "" {
		return ytVideoPayload{}, errors.New("ytInitialPlayerResponse not found in YouTube HTML")
	}

	var pr struct {
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
				CaptionTracks []struct {
					BaseURL      string `json:"baseUrl"`
					LanguageCode string `json:"languageCode"`
				} `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err := json.Unmarshal([]byte(jsonText), &pr); err != nil {
		return ytVideoPayload{}, fmt.Errorf("parse ytInitialPlayerResponse: %w", err)
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
		return p, nil
	}

	picked := tracks[0]
	foundPref := false
	for _, tr := range tracks {
		if strings.HasPrefix(strings.ToLower(tr.LanguageCode), "zh") {
			picked = tr
			foundPref = true
			break
		}
	}
	if !foundPref {
		for _, tr := range tracks {
			if strings.HasPrefix(strings.ToLower(tr.LanguageCode), "en") {
				picked = tr
				break
			}
		}
	}

	p.LanguageCode = picked.LanguageCode
	if picked.BaseURL != "" && client != nil {
		transcript, err := fetchCaptionTrack(ctx, client, picked.BaseURL)
		if err == nil && transcript != "" {
			p.Transcript = transcript
			p.TranscriptSource = "youtube_captions"
		}
	}
	return p, nil
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
