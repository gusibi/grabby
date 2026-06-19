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
