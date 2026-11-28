// Package twitter is a server-side site adapter for X/Twitter. It turns
// intent-level requests (search / home timeline / likes) into the generic
// "intercept" browser primitive, then filters and parses the captured GraphQL
// responses into structured tweets.
//
// Design: docs/browser-executor-plan.md §5 and §7. The browser hook captures
// every data response generically; all operation-specific filtering and
// parsing lives here so a site redesign only touches this file.
package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
)

// Tweet is a structured tweet extracted from a GraphQL response.
type Tweet struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Author        string   `json:"author"`      // @screen_name
	AuthorName    string   `json:"author_name"` // display name
	CreatedAt     string   `json:"created_at"`
	FavoriteCount int      `json:"favorite_count"`
	RetweetCount  int      `json:"retweet_count"`
	ReplyCount    int      `json:"reply_count"`
	QuoteCount    int      `json:"quote_count"`
	ViewCount     int      `json:"view_count,omitempty"`
	URL           string   `json:"url"`
	Media         []string `json:"media,omitempty"`
	Links         []string `json:"links,omitempty"`
	IsArticle     bool     `json:"is_article,omitempty"`
	ArticleTitle  string   `json:"article_title,omitempty"`
	ArticleText   string   `json:"article_text,omitempty"`
	ArticleURL    string   `json:"article_url,omitempty"`
}

// capturedRecord mirrors one item produced by the browser intercept hook.
type capturedRecord struct {
	URL           string `json:"url"`
	Status        int    `json:"status"`
	Method        string `json:"method"`
	Body          string `json:"body"`
	BodyBytes     int    `json:"bodyBytes"`
	BodyTruncated bool   `json:"bodyTruncated"`
	TS            int64  `json:"ts"`
}

// interceptMaxBodyBytes is the per-response body budget for intercepted X
// responses. Home timeline GraphQL payloads routinely run 250–400 KiB, and the
// extension's 256 KiB default cuts them mid-JSON — a truncated body cannot be
// parsed, which surfaces as a misleading "no tweets captured". Keep this below
// the hook's 5 MiB hard cap (larger bodies are dropped entirely).
const interceptMaxBodyBytes = 4 << 20

// Sender is the subset of the WebSocket manager this adapter needs.
type Sender interface {
	ResolveBrowserConnID(name string) (string, error)
	SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
}

var _ Sender = (*browserws.WebSocketManager)(nil)

// Options controls a fetch.
type Options struct {
	Browser      string
	Limit        int
	ScrollRounds int
}

func (o Options) scrollRounds(def int) int {
	if o.ScrollRounds > 0 {
		return o.ScrollRounds
	}
	return def
}

func (o Options) limit(def int) int {
	if o.Limit > 0 {
		return o.Limit
	}
	return def
}

// Search runs an X search and returns matching tweets.
func Search(ctx context.Context, wm Sender, logger *zap.Logger, query string, opts Options) ([]Tweet, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	pageURL := "https://x.com/search?q=" + url.QueryEscape(query) + "&f=top"
	return runIntercept(ctx, wm, logger, opts.Browser, pageURL, []string{"SearchTimeline"}, opts.scrollRounds(5), opts.limit(40))
}

// Timeline reads the home timeline. kind is "for_you" or "following".
func Timeline(ctx context.Context, wm Sender, logger *zap.Logger, kind string, opts Options) ([]Tweet, error) {
	pageURL := "https://x.com/home"
	ops := []string{"HomeTimeline", "HomeLatestTimeline"}
	if kind == "following" || kind == "latest" {
		// X switches to HomeLatestTimeline when the "Following" tab is active;
		// landing on /home defaults to "For you". We still match both ops so the
		// adapter works regardless of which tab the user last left selected.
		pageURL = "https://x.com/home"
	}
	return runIntercept(ctx, wm, logger, opts.Browser, pageURL, ops, opts.scrollRounds(6), opts.limit(40))
}

// Likes reads the current logged-in user's likes history.
//
// X exposes the likes history at https://x.com/i/history/likes; the old
// per-handle /<handle>/likes page is no longer used. The browser must be
// logged in, and the result always belongs to that account.
//
// NOTE: X's GraphQL shape changes frequently; callers must treat an empty
// result as "verify page/endpoint still accessible", not as "no likes".
// See docs/browser-executor-plan.md §7.2.
func Likes(ctx context.Context, wm Sender, logger *zap.Logger, opts Options) ([]Tweet, error) {
	const pageURL = "https://x.com/i/history/likes"
	return runIntercept(ctx, wm, logger, opts.Browser, pageURL, []string{"Likes"}, opts.scrollRounds(8), opts.limit(60))
}

// runIntercept sends one intercept command, filters captures by operation name,
// parses tweets, dedupes, and truncates to limit.
func runIntercept(ctx context.Context, wm Sender, logger *zap.Logger, browser, pageURL string, opNames []string, scrollRounds, limit int) ([]Tweet, error) {
	connID, err := wm.ResolveBrowserConnID(browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "intercept",
		URL:      pageURL,
		Visible:  true, // timeline/likes/search are infinite-scroll; need a visible tab
		CloseTab: true,
		Params: map[string]any{
			"scrollRounds": scrollRounds,
			"maxCaptures":  200,
			// Only ship the responses we actually parse back over the
			// WebSocket; the home page also fires large unrelated GraphQL
			// calls that would otherwise eat capture slots and bandwidth.
			"patterns": opNames,
			// A truncated body is invalid JSON, so the extension's 256 KiB
			// default silently yields zero tweets on large timelines.
			"maxBodyBytes": interceptMaxBodyBytes,
		},
	}

	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("intercept failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("intercept error: %s", resp.Error)
	}

	tweets := make([]Tweet, 0, limit)
	seen := make(map[string]struct{})
	matched := 0
	truncated := 0

	for _, raw := range resp.Result.Items {
		rec, ok := toRecord(raw)
		if !ok || !matchesOp(rec.URL, opNames) {
			continue
		}
		matched++
		if rec.BodyTruncated {
			truncated++
		}
		for _, t := range extractTweets(rec.Body) {
			if t.ID == "" {
				continue
			}
			if _, dup := seen[t.ID]; dup {
				continue
			}
			seen[t.ID] = struct{}{}
			tweets = append(tweets, t)
			if len(tweets) >= limit {
				break
			}
		}
		if len(tweets) >= limit {
			break
		}
	}

	if logger != nil {
		logger.Info("twitter intercept parsed",
			zap.String("url", pageURL),
			zap.Int("captures", len(resp.Result.Items)),
			zap.Int("matched_ops", matched),
			zap.Int("truncated_bodies", truncated),
			zap.Int("tweets", len(tweets)),
		)
	}

	if len(tweets) == 0 {
		if truncated > 0 {
			return tweets, fmt.Errorf("no tweets captured: %d of %d matched responses were truncated at %d bytes — the capture body budget is too small", truncated, matched, interceptMaxBodyBytes)
		}
		// Distinguish "captured nothing relevant" (likely not logged in / page
		// structure changed) from a genuine empty result.
		return tweets, fmt.Errorf("no tweets captured (matched %d/%d responses) — check login state or that X page structure still matches", matched, len(resp.Result.Items))
	}
	if truncated > 0 && logger != nil {
		logger.Warn("twitter intercept dropped truncated responses",
			zap.String("url", pageURL),
			zap.Int("truncated_bodies", truncated),
			zap.Int("max_body_bytes", interceptMaxBodyBytes),
		)
	}
	return tweets, nil
}

func toRecord(raw any) (capturedRecord, bool) {
	b, err := json.Marshal(raw)
	if err != nil {
		return capturedRecord{}, false
	}
	var rec capturedRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return capturedRecord{}, false
	}
	return rec, true
}

func matchesOp(u string, opNames []string) bool {
	for _, op := range opNames {
		if strings.Contains(u, "/"+op) || strings.Contains(u, op) {
			return true
		}
	}
	return false
}

// extractTweets parses a GraphQL response body and recursively finds tweet
// objects. A tweet "result" node is a map that has a "legacy" map containing
// "full_text". This avoids hardcoding the (frequently changing) nesting path.
func extractTweets(body string) []Tweet {
	if body == "" {
		return nil
	}
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil
	}
	var out []Tweet
	walk(root, &out)
	return out
}

func walk(node any, out *[]Tweet) {
	switch n := node.(type) {
	case map[string]any:
		if t, ok := tweetFromResult(n); ok {
			*out = append(*out, t)
			// keep walking — quoted/retweeted tweets nest deeper
		}
		for _, v := range n {
			walk(v, out)
		}
	case []any:
		for _, v := range n {
			walk(v, out)
		}
	}
}

// tweetFromResult tries to interpret a map as a tweet "result" node.
func tweetFromResult(n map[string]any) (Tweet, bool) {
	legacy, ok := asMap(n["legacy"])
	if !ok {
		return Tweet{}, false
	}
	text := asString(legacy["full_text"])
	if text == "" {
		return Tweet{}, false
	}

	// Long tweets (>280 chars) carry the full, untruncated body in
	// note_tweet.note_tweet_results.result.text.
	var noteEntitySet map[string]any
	if nt, ok := asMap(n["note_tweet"]); ok {
		if ntr, ok := asMap(nt["note_tweet_results"]); ok {
			if res, ok := asMap(ntr["result"]); ok {
				if full := asString(res["text"]); full != "" {
					text = full
				}
				if es, ok := asMap(res["entity_set"]); ok {
					noteEntitySet = es
				}
			}
		}
	}

	t := Tweet{
		Text:          text,
		CreatedAt:     asString(legacy["created_at"]),
		FavoriteCount: asInt(legacy["favorite_count"]),
		RetweetCount:  asInt(legacy["retweet_count"]),
		ReplyCount:    asInt(legacy["reply_count"]),
		QuoteCount:    asInt(legacy["quote_count"]),
	}
	if views, ok := asMap(n["views"]); ok {
		t.ViewCount = asInt(views["count"])
	}

	t.ID = asString(n["rest_id"])
	if t.ID == "" {
		t.ID = asString(legacy["id_str"])
	}

	// Author: n.core.user_results.result.legacy.{screen_name,name}
	if core, ok := asMap(n["core"]); ok {
		if ur, ok := asMap(core["user_results"]); ok {
			if res, ok := asMap(ur["result"]); ok {
				if ulegacy, ok := asMap(res["legacy"]); ok {
					t.Author = asString(ulegacy["screen_name"])
					t.AuthorName = asString(ulegacy["name"])
				}
				// newer shape: result.core.screen_name
				if rc, ok := asMap(res["core"]); ok {
					if t.Author == "" {
						t.Author = asString(rc["screen_name"])
					}
					if t.AuthorName == "" {
						t.AuthorName = asString(rc["name"])
					}
				}
			}
		}
	}

	if t.Author != "" && t.ID != "" {
		t.URL = "https://x.com/" + t.Author + "/status/" + t.ID
	}

	// Expand t.co short links and collect external URLs / X Article links.
	urlEntities := collectURLEntities(legacy["entities"], noteEntitySet)
	seenLinks := make(map[string]struct{})
	for _, ue := range urlEntities {
		if ue.short != "" && ue.expanded != "" {
			t.Text = strings.ReplaceAll(t.Text, ue.short, ue.expanded)
		}
		if ue.expanded == "" {
			continue
		}
		if isXArticleURL(ue.expanded) {
			t.IsArticle = true
			if t.ArticleURL == "" {
				t.ArticleURL = ue.expanded
			}
			continue
		}
		if isExternalArticleCandidate(ue.expanded) {
			if _, dup := seenLinks[ue.expanded]; !dup {
				seenLinks[ue.expanded] = struct{}{}
				t.Links = append(t.Links, ue.expanded)
			}
		}
	}

	// X Articles carry structured metadata (and sometimes full Draft.js blocks)
	// under article.article_results.result.
	if artRes := extractArticleResult(n); artRes != nil {
		t.IsArticle = true
		if title := strings.TrimSpace(asString(artRes["title"])); title != "" {
			t.ArticleTitle = title
		}
		if blocksMD := renderArticleContentState(artRes["content_state"]); blocksMD != "" {
			t.ArticleText = blocksMD
		} else if preview := strings.TrimSpace(asString(artRes["preview_text"])); preview != "" {
			t.ArticleText = preview
		}
		if cover := extractArticleCoverURL(artRes); cover != "" {
			t.Media = append(t.Media, cover)
		}
	}

	// Media URLs from extended_entities.media[].media_url_https
	if ee, ok := asMap(legacy["extended_entities"]); ok {
		if media, ok := ee["media"].([]any); ok {
			for _, m := range media {
				if mm, ok := asMap(m); ok {
					if u := asString(mm["media_url_https"]); u != "" {
						t.Media = append(t.Media, u)
					}
				}
			}
		}
	}

	return t, true
}

type urlEntity struct {
	short    string
	expanded string
}

func collectURLEntities(entityMaps ...any) []urlEntity {
	var out []urlEntity
	for _, em := range entityMaps {
		m, ok := asMap(em)
		if !ok {
			continue
		}
		urls, ok := m["urls"].([]any)
		if !ok {
			continue
		}
		for _, raw := range urls {
			u, ok := asMap(raw)
			if !ok {
				continue
			}
			short := strings.TrimSpace(asString(u["url"]))
			expanded := strings.TrimSpace(asString(u["expanded_url"]))
			if expanded == "" {
				expanded = strings.TrimSpace(asString(u["unwound_url"]))
			}
			if short != "" || expanded != "" {
				out = append(out, urlEntity{short: short, expanded: expanded})
			}
		}
	}
	return out
}

func isXArticleURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "x.com" && host != "twitter.com" {
		return false
	}
	p := strings.ToLower(u.Path)
	return strings.HasPrefix(p, "/i/article/") || strings.Contains(p, "/article/")
}

func isExternalArticleCandidate(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	switch host {
	case "", "x.com", "twitter.com", "t.co", "pic.twitter.com", "pic.x.com", "pbs.twimg.com", "video.twimg.com":
		return false
	}
	return true
}

func extractArticleResult(n map[string]any) map[string]any {
	if art, ok := asMap(n["article"]); ok {
		if ar, ok := asMap(art["article_results"]); ok {
			if res, ok := asMap(ar["result"]); ok {
				return res
			}
		}
	}
	if ar, ok := asMap(n["article_results"]); ok {
		if res, ok := asMap(ar["result"]); ok {
			return res
		}
	}
	return nil
}

func extractArticleCoverURL(artRes map[string]any) string {
	cm, ok := asMap(artRes["cover_media"])
	if !ok {
		return ""
	}
	if mi, ok := asMap(cm["media_info"]); ok {
		if u := strings.TrimSpace(asString(mi["original_img_url"])); u != "" {
			return u
		}
	}
	return ""
}

// renderArticleContentState converts X Article Draft.js content_state.blocks
// into clean Markdown paragraphs/headings when present in the GraphQL payload.
func renderArticleContentState(raw any) string {
	cs, ok := asMap(raw)
	if !ok {
		return ""
	}
	blocks, ok := cs["blocks"].([]any)
	if !ok || len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		bm, ok := asMap(b)
		if !ok {
			continue
		}
		text := strings.TrimSpace(asString(bm["text"]))
		if text == "" {
			continue
		}
		switch asString(bm["type"]) {
		case "header-one":
			parts = append(parts, "# "+text)
		case "header-two":
			parts = append(parts, "## "+text)
		case "header-three":
			parts = append(parts, "### "+text)
		case "blockquote":
			parts = append(parts, "> "+text)
		case "unordered-list-item":
			parts = append(parts, "- "+text)
		case "ordered-list-item":
			parts = append(parts, "1. "+text)
		case "code-block":
			parts = append(parts, "```\n"+text+"\n```")
		default:
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(x))
		return i
	}
	return 0
}
