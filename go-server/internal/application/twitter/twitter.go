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
	URL           string   `json:"url"`
	Media         []string `json:"media,omitempty"`
}

// capturedRecord mirrors one item produced by the browser intercept hook.
type capturedRecord struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	Method string `json:"method"`
	Body   string `json:"body"`
	TS     int64  `json:"ts"`
}

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

// Likes reads a user's likes. handle empty => current logged-in user's profile.
//
// NOTE: X's likes-page visibility and GraphQL shape change frequently; callers
// must treat an empty result as "verify page/endpoint still accessible", not as
// "no likes". See docs/browser-executor-plan.md §7.2.
func Likes(ctx context.Context, wm Sender, logger *zap.Logger, handle string, opts Options) ([]Tweet, error) {
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		// Without a handle we cannot build the /likes URL; X has no stable
		// "my likes" alias that avoids resolving the current handle first.
		return nil, fmt.Errorf("handle is required (e.g. \"jack\") to open <handle>/likes")
	}
	pageURL := "https://x.com/" + url.PathEscape(handle) + "/likes"
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

	for _, raw := range resp.Result.Items {
		rec, ok := toRecord(raw)
		if !ok || !matchesOp(rec.URL, opNames) {
			continue
		}
		matched++
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
			zap.Int("tweets", len(tweets)),
		)
	}

	if len(tweets) == 0 {
		// Distinguish "captured nothing relevant" (likely not logged in / page
		// structure changed) from a genuine empty result.
		return tweets, fmt.Errorf("no tweets captured (matched %d/%d responses) — check login state or that X page structure still matches", matched, len(resp.Result.Items))
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

	t := Tweet{
		Text:          text,
		CreatedAt:     asString(legacy["created_at"]),
		FavoriteCount: asInt(legacy["favorite_count"]),
		RetweetCount:  asInt(legacy["retweet_count"]),
		ReplyCount:    asInt(legacy["reply_count"]),
		QuoteCount:    asInt(legacy["quote_count"]),
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
	}
	return 0
}
