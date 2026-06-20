// Package reddit is a server-side site adapter for Reddit. It turns
// intent-level requests (thread / subreddit) into the generic "fetchInPage"
// browser primitive, then parses the JSON responses into structured data.
//
// Design: docs/prd-likes-incremental-and-reddit.md. Reddit's .json endpoints are
// public; the browser fetch runs in the page context (cookies/origin), bypassing
// server-IP 403s. Following the atomic-capability model (see root CONTEXT.md),
// this adapter only fetches data correctly — login state is ensured by the user
// ahead of time, rate control is the caller's concern.
package reddit

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

// Post is one Reddit submission parsed from a listing .json.
type Post struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	URL         string  `json:"url"`          // permalink on reddit
	ContentURL  string  `json:"content_url"`  // the link the post points to (image/article)
	Body        string  `json:"body"`         // selftext (text posts)
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
}

// Comment is one node in a thread's comment tree.
type Comment struct {
	ID       string    `json:"id"`
	Author   string    `json:"author"`
	Body     string    `json:"body"`
	Score    int       `json:"score"`
	Created  float64   `json:"created_utc"`
	Replies  []Comment `json:"replies,omitempty"`
}

// Thread is the parsed result of a post comments page .json.
type Thread struct {
	Post     Post      `json:"post"`
	Comments []Comment `json:"comments"`
}

// Sender is the subset of the WebSocket manager this adapter needs. Same shape
// as the twitter adapter's Sender.
type Sender interface {
	ResolveBrowserConnID(name string) (string, error)
	SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
}

var _ Sender = (*browserws.WebSocketManager)(nil)

// Options controls a fetch.
type Options struct {
	Browser string
	Limit   int
	// Pages caps how many pages to fetch for subreddit listings (each page is
	// ~25 posts on Reddit). 0 falls back to a sensible default.
	Pages int
}

func (o Options) limit(def int) int {
	if o.Limit > 0 {
		return o.Limit
	}
	return def
}

func (o Options) pages(def int) int {
	if o.Pages > 0 {
		return o.Pages
	}
	return def
}

// FetchThread fetches one Reddit post + its comment tree.
//
// pageURL is the post's permalink (e.g. https://www.reddit.com/r/golang/comments/xxxx/title/).
// The adapter opens that page in the browser (for cookie/origin context) and
// fetches the same URL with a .json suffix.
func FetchThread(ctx context.Context, wm Sender, logger *zap.Logger, pageURL string, opts Options) (*Thread, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	jsonURL := jsonURLFor(pageURL)

	body, err := fetchInPage(ctx, wm, logger, opts.Browser, pageURL, jsonURL)
	if err != nil {
		return nil, err
	}

	thread, err := parseThread(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reddit thread: %w", err)
	}
	if thread.Post.ID == "" && thread.Post.Title == "" {
		return nil, fmt.Errorf("reddit thread .json returned no post — check that the page is accessible / logged in")
	}

	if logger != nil {
		logger.Info("reddit thread parsed",
			zap.String("url", pageURL),
			zap.String("post_id", thread.Post.ID),
			zap.Int("comments", len(thread.Comments)),
		)
	}
	return thread, nil
}

// jsonURLFor turns a Reddit permalink into its .json endpoint. Handles trailing
// slashes and existing query strings.
func jsonURLFor(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		// Fall back to a naive append; the browser will surface a fetch error.
		return strings.TrimSuffix(pageURL, "/") + ".json"
	}
	p := u.Path
	p = strings.TrimSuffix(p, "/")
	p += ".json"
	u.Path = p
	return u.String()
}

// ListingResult holds one listing fetch plus the `after` cursor for pagination.
type ListingResult struct {
	Posts []Post
	After string // cursor for the next page; empty when exhausted
}

// fetchListing fetches one page of any Reddit listing (.json). pageURL is opened
// in the browser for cookie/origin context; reqURL is the .json endpoint (with
// the after cursor appended by the caller). Used by subreddit + search.
func fetchListing(ctx context.Context, wm Sender, logger *zap.Logger, browser, pageURL, reqURL, label string) (*ListingResult, error) {
	body, err := fetchInPage(ctx, wm, logger, browser, pageURL, reqURL)
	if err != nil {
		return nil, err
	}
	posts, nextAfter, err := parseListing(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reddit listing: %w", err)
	}
	if logger != nil {
		logger.Info("reddit listing parsed",
			zap.String("label", label),
			zap.String("page_url", pageURL),
			zap.Int("posts", len(posts)),
			zap.String("after", nextAfter),
		)
	}
	return &ListingResult{Posts: posts, After: nextAfter}, nil
}

// FetchSubreddit fetches one page of a subreddit's newest posts.
//
// subreddit is the bare name (e.g. "golang"). after is the pagination cursor
// from a previous page (empty for the first page).
func FetchSubreddit(ctx context.Context, wm Sender, logger *zap.Logger, subreddit string, after string, opts Options) (*ListingResult, error) {
	subreddit = strings.TrimSpace(subreddit)
	subreddit = strings.TrimPrefix(strings.TrimPrefix(subreddit, "/r/"), "r/")
	if subreddit == "" {
		return nil, fmt.Errorf("subreddit is required")
	}

	pageURL := "https://www.reddit.com/r/" + url.PathEscape(subreddit) + "/new/"
	reqURL := "https://www.reddit.com/r/" + url.PathEscape(subreddit) + "/new.json?raw_json=1"
	if after != "" {
		reqURL += "&after=" + url.QueryEscape(after)
	}
	return fetchListing(ctx, wm, logger, opts.Browser, pageURL, reqURL, "subreddit:"+subreddit)
}

// FetchSearch fetches one page of Reddit search results.
//
// query is the search string. subreddit limits the search to a single
// subreddit when non-empty (uses /r/{name}/search?restrict_sr=1); otherwise it
// searches all of Reddit (/search). sort is "relevance"/"new"/"top"/"comments"
// (empty => Reddit default).
func FetchSearch(ctx context.Context, wm Sender, logger *zap.Logger, query, subreddit, sort, after string, opts Options) (*ListingResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("raw_json", "1")
	if subreddit != "" {
		subreddit = strings.TrimSpace(subreddit)
		subreddit = strings.TrimPrefix(strings.TrimPrefix(subreddit, "/r/"), "r/")
	}
	if sort != "" {
		q.Set("sort", sort)
	}

	var pageURL, reqURL string
	if subreddit != "" {
		pageURL = "https://www.reddit.com/r/" + url.PathEscape(subreddit) + "/search/?restrict_sr=1&" + q.Encode()
		reqURL = "https://www.reddit.com/r/" + url.PathEscape(subreddit) + "/search.json?restrict_sr=1&" + q.Encode()
	} else {
		pageURL = "https://www.reddit.com/search/?" + q.Encode()
		reqURL = "https://www.reddit.com/search.json?" + q.Encode()
	}
	if after != "" {
		reqURL += "&after=" + url.QueryEscape(after)
	}
	label := "search:" + query
	if subreddit != "" {
		label = "search:r/" + subreddit + ":" + query
	}
	return fetchListing(ctx, wm, logger, opts.Browser, pageURL, reqURL, label)
}

// fetchListingPages fetches up to `pages` pages of a listing via a per-page
// fetcher, concatenating results and deduping by post id. Stops early when a
// page returns no `after` cursor (listing exhausted). emptyMsg describes the
// "no posts captured" error when nothing came back.
func fetchListingPages(ctx context.Context, wm Sender, logger *zap.Logger, opts Options, emptyMsg string, page func(after string) (*ListingResult, error)) ([]Post, error) {
	pages := opts.pages(4)
	limit := opts.limit(100)
	seen := make(map[string]struct{})
	out := make([]Post, 0, limit)
	after := ""
	for i := 0; i < pages; i++ {
		res, err := page(after)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			// partial: return what we have so far if a later page fails
			break
		}
		for _, p := range res.Posts {
			if p.ID == "" {
				continue
			}
			if _, dup := seen[p.ID]; dup {
				continue
			}
			seen[p.ID] = struct{}{}
			out = append(out, p)
			if len(out) >= limit {
				return out, nil
			}
		}
		if res.After == "" {
			break
		}
		after = res.After
	}
	if len(out) == 0 {
		return out, fmt.Errorf("%s", emptyMsg)
	}
	return out, nil
}

// FetchSubredditPages fetches up to `pages` pages of a subreddit's newest posts,
// concatenating results and deduping by post id.
func FetchSubredditPages(ctx context.Context, wm Sender, logger *zap.Logger, subreddit string, opts Options) ([]Post, error) {
	return fetchListingPages(ctx, wm, logger, opts,
		fmt.Sprintf("no posts captured for r/%s — check that the subreddit is accessible / logged in", subreddit),
		func(after string) (*ListingResult, error) {
			return FetchSubreddit(ctx, wm, logger, subreddit, after, opts)
		})
}

// FetchSearchPages fetches up to `pages` pages of Reddit search results,
// concatenating results and deduping by post id.
func FetchSearchPages(ctx context.Context, wm Sender, logger *zap.Logger, query, subreddit, sort string, opts Options) ([]Post, error) {
	return fetchListingPages(ctx, wm, logger, opts,
		fmt.Sprintf("no posts captured for search %q — check login state or query", query),
		func(after string) (*ListingResult, error) {
			return FetchSearch(ctx, wm, logger, query, subreddit, sort, after, opts)
		})
}

// parseListing parses a subreddit listing .json body into posts + the `after`
// pagination cursor.
func parseListing(body string) ([]Post, string, error) {
	var root json.RawMessage
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, "", fmt.Errorf("expected json object: %w", err)
	}
	// A subreddit listing is {kind, data:{children:[...], after:"..."}}.
	var listing struct {
		Data struct {
			Children []json.RawMessage `json:"children"`
			After    string            `json:"after"`
		} `json:"data"`
	}
	if err := json.Unmarshal(root, &listing); err != nil {
		return nil, "", fmt.Errorf("expected listing shape: %w", err)
	}
	posts := make([]Post, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		d, ok := thingData(child)
		if !ok {
			continue
		}
		if asString(d["__kind"]) == "more" {
			continue
		}
		posts = append(posts, postFromData(d))
	}
	return posts, listing.Data.After, nil
}

// fetchInPage opens pageURL in the browser for request context, then fetches
// requestURL in that page's MAIN world (sharing cookies/origin). Returns the
// raw response body text.
func fetchInPage(ctx context.Context, wm Sender, logger *zap.Logger, browser, pageURL, requestURL string) (string, error) {
	connID, err := wm.ResolveBrowserConnID(browser)
	if err != nil {
		return "", fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "fetchInPage",
		URL:      pageURL,
		CloseTab: true,
		Params: map[string]any{
			"requestUrl":  requestURL,
			"method":      "GET",
			"headers":     map[string]string{"Accept": "application/json"},
			"credentials": "include",
		},
	}

	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return "", fmt.Errorf("fetchInPage failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return "", fmt.Errorf("fetchInPage error: %s", resp.Error)
	}

	status := 0
	if resp.Result.JSON != nil {
		if s, ok := resp.Result.JSON["status"].(float64); ok {
			status = int(s)
		}
	}
	if status != 0 && status >= 400 {
		return "", fmt.Errorf("reddit returned HTTP %d for %s", status, requestURL)
	}
	if resp.Result.Text == "" {
		return "", fmt.Errorf("fetchInPage returned empty body for %s — check login state or that the endpoint is accessible", requestURL)
	}
	return resp.Result.Text, nil
}

// parseThread parses a Reddit comments-page .json body.
//
// Reddit returns a 2-element array: [0] is the post listing (one child), [1]
// is the comments listing. Each listing is {kind, data:{children:[...]}}. We
// tolerate shape variation by walking defensively.
func parseThread(body string) (*Thread, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		return nil, fmt.Errorf("expected json array: %w", err)
	}
	thread := &Thread{}
	if len(arr) >= 1 {
		if post, ok := firstChildData(arr[0]); ok {
			thread.Post = postFromData(post)
		}
	}
	if len(arr) >= 2 {
		thread.Comments = commentsFromListing(arr[1], 0)
	}
	return thread, nil
}

// listingData extracts the .data.children array from a Reddit listing object.
func listingData(raw json.RawMessage) []json.RawMessage {
	var listing struct {
		Data struct {
			Children []json.RawMessage `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &listing); err != nil {
		return nil
	}
	return listing.Data.Children
}

// firstChildData returns the .data of the first child of a listing, if any.
func firstChildData(raw json.RawMessage) (map[string]any, bool) {
	children := listingData(raw)
	if len(children) == 0 {
		return nil, false
	}
	return thingData(children[0])
}

// thingData extracts .data from a Reddit "thing" ({kind, data:{...}}).
func thingData(raw json.RawMessage) (map[string]any, bool) {
	var thing struct {
		Kind string         `json:"kind"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &thing); err != nil {
		return nil, false
	}
	if thing.Data == nil {
		thing.Data = map[string]any{}
	}
	thing.Data["__kind"] = thing.Kind
	return thing.Data, true
}

func postFromData(d map[string]any) Post {
	p := Post{
		ID:          asString(d["id"]),
		Title:       asString(d["title"]),
		Author:      asString(d["author"]),
		Subreddit:   asString(d["subreddit"]),
		Body:        asString(d["selftext"]),
		Score:       asInt(d["score"]),
		NumComments: asInt(d["num_comments"]),
		CreatedUTC:  asFloat(d["created_utc"]),
	}
	if permalink := asString(d["permalink"]); permalink != "" {
		p.URL = "https://www.reddit.com" + permalink
	}
	p.ContentURL = asString(d["url"])
	return p
}

// commentsFromListing parses the comments listing into a tree, capped at depth
// to avoid runaway recursion on deeply nested threads.
func commentsFromListing(raw json.RawMessage, depth int) []Comment {
	if depth > 16 {
		return nil
	}
	out := make([]Comment, 0)
	for _, child := range listingData(raw) {
		d, ok := thingData(child)
		if !ok {
			continue
		}
		// "more" placeholders (kind:"more") carry no body — skip them.
		if asString(d["__kind"]) == "more" {
			continue
		}
		c := Comment{
			ID:      asString(d["id"]),
			Author:  asString(d["author"]),
			Body:    asString(d["body"]),
			Score:   asInt(d["score"]),
			Created: asFloat(d["created_utc"]),
		}
		c.Replies = commentReplies(d["replies"], depth)
		out = append(out, c)
	}
	return out
}

// commentReplies parses a comment's "replies" field, which is either "" (none),
// a listing object, or absent.
func commentReplies(v any, depth int) []Comment {
	if v == nil {
		return nil
	}
	s, ok := v.(string)
	if ok && s == "" {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return commentsFromListing(raw, depth+1)
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
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}
