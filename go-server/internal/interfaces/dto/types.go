package dto

import (
	"encoding/json"
	"time"

	"go-server/internal/domain/browser"
)

// ExtractAPIRequest is the POST /api/extract request body.
type ExtractAPIRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// BrowserRegisterRequest is the POST /api/browsers/register request body.
type BrowserRegisterRequest struct {
	ConnectID string `json:"connect_id"`
	Name      string `json:"name"`
}

// BrowserRegisterResponse is the POST /api/browsers/register response body.
type BrowserRegisterResponse struct {
	Success bool                        `json:"success"`
	Browser browser.BrowserRegistration `json:"browser"`
}

// ExtractAPIResponse is the POST /api/extract response body.
type ExtractAPIResponse struct {
	Success  bool   `json:"success"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
	// Cached is true when the result came from the extract_cache table rather
	// than a fresh browser fetch.
	Cached bool `json:"cached"`
}

// ScreenshotAPIRequest is the POST /api/screenshot request body.
type ScreenshotAPIRequest struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ScreenshotAPIResponse is the POST /api/screenshot response body.
type ScreenshotAPIResponse struct {
	Success   bool   `json:"success"`
	URL       string `json:"url"`
	ImageData string `json:"imageData"`
}

// RunPageScriptAPIRequest is the POST /api/run_page_script request body.
// It invokes the extension's whitelisted-page-script primitive (plan §4.1).
type RunPageScriptAPIRequest struct {
	URL     string         `json:"url"`
	Script  string         `json:"script"`            // whitelisted script name
	Params  map[string]any `json:"params,omitempty"`  // structured args for the script
	Visible bool           `json:"visible,omitempty"` // briefly activate the tab
	Browser string         `json:"browser,omitempty"`
}

// RunPageScriptAPIResponse is the POST /api/run_page_script response body.
type RunPageScriptAPIResponse struct {
	Success bool           `json:"success"`
	URL     string         `json:"url"`
	JSON    map[string]any `json:"json,omitempty"` // object results
	Text    string         `json:"text,omitempty"` // non-object results (JSON-encoded)
}

// FetchInPageAPIRequest is the POST /api/fetch_in_page request body.
// It invokes the extension's in-page fetch primitive (plan §4.2): the request
// runs in the page's MAIN world, sharing its cookies/origin.
type FetchInPageAPIRequest struct {
	URL         string            `json:"url"`                  // page context to open
	RequestURL  string            `json:"requestUrl,omitempty"` // fetch target (defaults to url)
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Credentials string            `json:"credentials,omitempty"` // fetch credentials mode (default include)
	Visible     bool              `json:"visible,omitempty"`
	Browser     string            `json:"browser,omitempty"`
}

// FetchInPageAPIResponse is the POST /api/fetch_in_page response body.
type FetchInPageAPIResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url"`
	Status  int    `json:"status,omitempty"`
	Text    string `json:"text"`
}

// TwitterSearchAPIRequest is the POST /api/twitter/search request body.
type TwitterSearchAPIRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// TwitterTimelineAPIRequest is the POST /api/twitter/timeline request body.
type TwitterTimelineAPIRequest struct {
	Kind    string `json:"kind,omitempty"` // "for_you" | "following"
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// TwitterLikesAPIRequest is the POST /api/twitter/likes request body.
type TwitterLikesAPIRequest struct {
	Handle  string `json:"handle"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// RedditThreadAPIRequest is the POST /api/reddit/thread request body. It opens
// the post's permalink in the browser and fetches the same URL with a .json
// suffix (plan: docs/prd-likes-incremental-and-reddit.md).
type RedditThreadAPIRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// RedditThreadAPIResponse is the POST /api/reddit/thread response body.
type RedditThreadAPIResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url"`
	Post    any    `json:"post"`
	Comments []any `json:"comments"`
}

// RedditSubredditAPIRequest is the POST /api/reddit/subreddit request body.
type RedditSubredditAPIRequest struct {
	Subreddit string `json:"subreddit"`
	Limit     int    `json:"limit,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// RedditSubredditAPIResponse is the POST /api/reddit/subreddit response body.
type RedditSubredditAPIResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
	Posts   []any `json:"posts"`
}

// RedditSearchAPIRequest is the POST /api/reddit/search request body.
type RedditSearchAPIRequest struct {
	Query     string `json:"query"`
	Subreddit string `json:"subreddit,omitempty"` // limit search to one subreddit when set
	Sort      string `json:"sort,omitempty"`      // relevance | new | top | comments
	Limit     int    `json:"limit,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// RedditSearchAPIResponse is the POST /api/reddit/search response body.
type RedditSearchAPIResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
	Posts   []any `json:"posts"`
}

// XhsNoteAPIRequest is the POST /api/xiaohongshu/note request body.
type XhsNoteAPIRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// XhsNoteAPIResponse is the POST /api/xiaohongshu/note response body.
type XhsNoteAPIResponse struct {
	Success bool `json:"success"`
	Note    any  `json:"note"`
}

// XhsSearchAPIRequest is the POST /api/xiaohongshu/search request body.
type XhsSearchAPIRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// XhsSearchAPIResponse is the POST /api/xiaohongshu/search response body.
type XhsSearchAPIResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
	Notes   []any `json:"notes"`
}

// XhsUserNotesAPIRequest is the POST /api/xiaohongshu/user_notes request body.
type XhsUserNotesAPIRequest struct {
	URL    string `json:"url"`
	Limit  int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// XhsUserNotesAPIResponse is the POST /api/xiaohongshu/user_notes response body.
type XhsUserNotesAPIResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
	Notes   []any `json:"notes"`
}

// ExtractRecord is one row in the GET /api/captures/extract list. Markdown is
// omitted from the list (only its length) to keep the payload light.
type ExtractRecord struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Chars     int       `json:"chars"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HealthResponse is the GET /open/api/health response body.
type HealthResponse struct {
	Status           string    `json:"status"`
	BrowserConnected bool      `json:"browser_connected"`
	Timestamp        time.Time `json:"timestamp"`
}

// ScreenshotParams for the "screenshot" MCP tool.
type ScreenshotParams struct {
	URL      string `json:"url"`
	FullPage bool   `json:"fullPage"`
	Browser  string `json:"browser,omitempty"`
}

// ExtractParams for the "extract" MCP tool.
type ExtractParams struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// FetchInPageParams for the "fetch_in_page" MCP tool.
type FetchInPageParams struct {
	URL         string            `json:"url"`
	RequestURL  string            `json:"requestUrl,omitempty"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Credentials string            `json:"credentials,omitempty"`
	Browser     string            `json:"browser,omitempty"`
}

// LibrarySearchParams for the "library_search" MCP tool.
type LibrarySearchParams struct {
	Query          string `json:"query,omitempty"`
	Category       string `json:"category,omitempty"`
	SourceCategory string `json:"source_category,omitempty"`
	Origin         string `json:"origin,omitempty"`
	Starred        bool   `json:"starred,omitempty"`
	UnreadOnly     bool   `json:"unread_only,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
}

// LibraryItemParams for the "library_get_item" MCP tool.
type LibraryItemParams struct {
	ID int64 `json:"id"`
}

// LibraryDailyParams for the "library_daily_report" MCP tool.
type LibraryDailyParams struct {
	Date       string `json:"date,omitempty"`
	ReportType string `json:"report_type,omitempty"`
}

// BrowserListResponse is the GET /api/browsers response body.
type BrowserListResponse struct {
	Browsers []browser.BrowserInfo `json:"browsers"`
	Count    int                   `json:"count"`
}

// TwitterSearchParams for the "twitter_search" MCP tool.
type TwitterSearchParams struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// TwitterTimelineParams for the "twitter_timeline" MCP tool.
type TwitterTimelineParams struct {
	Kind    string `json:"kind,omitempty"` // "for_you" | "following"
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// TwitterLikesParams for the "twitter_likes" MCP tool.
type TwitterLikesParams struct {
	Handle  string `json:"handle"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// RedditThreadParams for the "reddit_thread" MCP tool.
type RedditThreadParams struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// RedditSubredditParams for the "reddit_subreddit" MCP tool.
type RedditSubredditParams struct {
	Subreddit string `json:"subreddit"`
	Limit     int    `json:"limit,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// RedditSearchParams for the "reddit_search" MCP tool.
type RedditSearchParams struct {
	Query     string `json:"query"`
	Subreddit string `json:"subreddit,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// XhsNoteParams for the "xiaohongshu_note" MCP tool.
type XhsNoteParams struct {
	URL     string `json:"url"`
	Browser string `json:"browser,omitempty"`
}

// XhsSearchParams for the "xiaohongshu_search" MCP tool.
type XhsSearchParams struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// XhsUserNotesParams for the "xiaohongshu_user_notes" MCP tool.
type XhsUserNotesParams struct {
	URL     string `json:"url"`
	Limit   int    `json:"limit,omitempty"`
	Browser string `json:"browser,omitempty"`
}

// ParseArgs converts raw MCP arguments (any/map) into a typed struct.
func ParseArgs[T any](raw any) (T, error) {
	var result T
	b, err := json.Marshal(raw)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return result, err
	}
	return result, nil
}
