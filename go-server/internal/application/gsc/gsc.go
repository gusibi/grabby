// Package gsc is the server-side site adapter for Google Search Console.
//
// Browser-only by design: every query opens the real Search Console UI in the
// user's already-logged-in browser and reads what the page itself requested
// (Google's private batchexecute RPC) or rendered (DOM). There are no API
// credentials and no server-side HTTP calls — the page's own session, headers
// and fingerprint are the entire point.
//
// Stage 0 (this file): open one real view and capture the raw responses, so the
// parsers can be written against real payloads instead of guesses. Parsing lands
// in parse.go once fixtures exist.
package gsc

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
)

const (
	// Origin is the only host we touch. search.google.com is also where
	// batchexecute lives, so cookie/origin context is already correct.
	Origin = "https://search.google.com"
	// ConsolePath lists the account's properties.
	ConsolePath = "/search-console"
	// PerformancePath is the Performance report deep link.
	PerformancePath = "/search-console/performance/search-analytics"
)

// Sender is the subset of the WebSocket manager this adapter needs.
type Sender interface {
	ResolveBrowserConnID(name string) (string, error)
	SendMessage(ctx context.Context, req *capture.BrowserRequest, targetConnID string) (*capture.BrowserResponse, error)
}

var _ Sender = (*browserws.WebSocketManager)(nil)

// Options controls one capture run.
type Options struct {
	Browser string
	// Visible briefly activates the tab. Not needed for report pages (the SPA
	// renders in a background tab); needed only when a UI action depends on
	// layout/visibility.
	Visible bool
	// Patterns overrides the URL substrings the hook keeps. Empty => adapter
	// defaults (batchexecute for reports, batchexecute + console for sites).
	Patterns []string
	// TimeoutMs bounds "wait until the target request shows up".
	TimeoutMs int
	// MaxCaptures caps how many responses we return.
	MaxCaptures int
	// MaxBodyBytes truncates each captured body (default 256 KiB).
	MaxBodyBytes int
	// Days is the report window for the deep link (default 28).
	Days int
}

func (o Options) timeout() int {
	if o.TimeoutMs > 0 {
		return o.TimeoutMs
	}
	return 25000
}

func (o Options) maxCaptures() int {
	if o.MaxCaptures > 0 {
		return o.MaxCaptures
	}
	return 20
}

func (o Options) maxBodyBytes() int {
	if o.MaxBodyBytes > 0 {
		return o.MaxBodyBytes
	}
	return 256 * 1024
}

func (o Options) days() int {
	if o.Days > 0 {
		return o.Days
	}
	return 28
}

// Capture is one intercepted response.
type Capture struct {
	URL           string   `json:"url"`
	Status        int      `json:"status,omitempty"`
	Method        string   `json:"method,omitempty"`
	Body          string   `json:"body,omitempty"`
	BodyBytes     int      `json:"bodyBytes,omitempty"`
	BodyTruncated bool     `json:"bodyTruncated,omitempty"`
	TS            float64  `json:"ts,omitempty"`
	RPCIDs        []string `json:"rpc_ids,omitempty"`
}

// CaptureResult is the raw output of one capture run.
type CaptureResult struct {
	Kind     string    `json:"kind"` // sites | performance
	Property string    `json:"property,omitempty"`
	PageURL  string    `json:"page_url"`
	Captures []Capture `json:"captures"`
}

// ConsoleURL returns the Search Console home (property switcher / overview).
func ConsoleURL() string {
	return Origin + ConsolePath
}

// PerformanceURL builds the Performance report deep link.
//
// property must be the canonical Search Console identifier:
//   - sc-domain:example.com        (Domain property)
//   - https://www.example.com/     (URL-prefix property, trailing slash matters)
//
// Do not derive it from a display name: they are different properties.
func PerformanceURL(property string, days int) string {
	if days <= 0 {
		days = 28
	}
	q := url.Values{}
	q.Set("resource_id", property)
	q.Set("num_of_days", strconv.Itoa(days))
	return Origin + PerformancePath + "?" + q.Encode()
}

// RecordSites opens Search Console and captures the property-list responses.
// A broad-but-harmless pattern set is used here because the exact list RPC id
// is versioned; the dump tells us which one it is.
func RecordSites(ctx context.Context, wm Sender, logger *zap.Logger, opts Options) (*CaptureResult, error) {
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"batchexecute", "/search-console/"}
	}
	return runCapture(ctx, wm, logger, "sites", "", ConsoleURL(), patterns, opts)
}

// RecordPerformance opens one property's Performance report and captures the
// RPC responses behind the chart and the table.
func RecordPerformance(ctx context.Context, wm Sender, logger *zap.Logger, property string, opts Options) (*CaptureResult, error) {
	property = strings.TrimSpace(property)
	if property == "" {
		return nil, fmt.Errorf("property is required (e.g. sc-domain:example.com or https://www.example.com/)")
	}
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = []string{"batchexecute"}
	}
	pageURL := PerformanceURL(property, opts.days())
	return runCapture(ctx, wm, logger, "performance", property, pageURL, patterns, opts)
}

// runCapture sends one intercept command and normalizes the captured items.
//
// waitForRequest + exactHost are the two non-obvious bits:
//   - exactHost keeps the hook on search.google.com only (not all of google.com)
//   - waitForRequest returns as soon as a matching response appears, instead of
//     sleeping for fixed scroll rounds
func runCapture(ctx context.Context, wm Sender, logger *zap.Logger, kind, property, pageURL string, patterns []string, opts Options) (*CaptureResult, error) {
	connID, err := wm.ResolveBrowserConnID(opts.Browser)
	if err != nil {
		return nil, fmt.Errorf("browser not available: %w", err)
	}

	req := &capture.BrowserRequest{
		Source:   "mcp_client",
		Action:   "mcp_request",
		Command:  "intercept",
		URL:      pageURL,
		Visible:  opts.Visible,
		CloseTab: true,
		Params: map[string]any{
			"patterns":        patterns,
			"waitForRequest":  true,
			"timeoutMs":       opts.timeout(),
			"settleMs":        1500,
			"scrollRounds":    0,
			"maxCaptures":     opts.maxCaptures(),
			"maxBodyBytes":    opts.maxBodyBytes(),
			"includeBodies":   true,
			"exactHost":       true,
			"unregisterAfter": true,
		},
	}

	resp, err := wm.SendMessage(ctx, req, connID)
	if err != nil {
		return nil, fmt.Errorf("intercept failed: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("intercept error: %s", resp.Error)
	}

	res := &CaptureResult{Kind: kind, Property: property, PageURL: pageURL}
	for _, raw := range resp.Result.Items {
		rec, ok := toCapture(raw)
		if !ok {
			continue
		}
		rec.RPCIDs = RPCIDs(rec.Body)
		res.Captures = append(res.Captures, rec)
	}

	if len(res.Captures) == 0 {
		return res, fmt.Errorf("REQUEST_NOT_SEEN: no response matched %v on %s — check that the browser is logged in to Search Console and the property is accessible", patterns, pageURL)
	}

	if logger != nil {
		logger.Info("gsc capture",
			zap.String("kind", kind),
			zap.String("property", property),
			zap.String("page_url", pageURL),
			zap.Int("captures", len(res.Captures)),
			zap.Strings("rpc_ids", allRPCIDs(res.Captures)),
		)
	}
	return res, nil
}

func toCapture(raw any) (Capture, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Capture{}, false
	}
	c := Capture{}
	if s, ok := m["url"].(string); ok {
		c.URL = s
	}
	if n, ok := m["status"].(float64); ok {
		c.Status = int(n)
	}
	if s, ok := m["method"].(string); ok {
		c.Method = s
	}
	if s, ok := m["body"].(string); ok {
		c.Body = s
	}
	if n, ok := m["bodyBytes"].(float64); ok {
		c.BodyBytes = int(n)
	}
	if b, ok := m["bodyTruncated"].(bool); ok {
		c.BodyTruncated = b
	}
	if n, ok := m["ts"].(float64); ok {
		c.TS = n
	}
	return c, true
}

var rpcIDPattern = regexp.MustCompile(`\["wrb\.fr","([A-Za-z0-9_]+)"`)

// RPCIDs extracts the batchexecute RPC ids present in a response body.
// Responses are Google's `)]}'`-prefixed sparse-array envelopes, e.g.
//
//	)]}'
//
//	123
//	[["wrb.fr","OLiH4d","[[...]]",null,null,null,"generic"]]
//
// Knowing the ids is what makes the first fixture run useful.
func RPCIDs(body string) []string {
	matches := rpcIDPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		id := m[1]
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func allRPCIDs(caps []Capture) []string {
	seen := map[string]struct{}{}
	for _, c := range caps {
		for _, id := range c.RPCIDs {
			seen[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// StripBatchPrefix removes the `)]}'` anti-JSON-hijack prefix Google puts in
// front of batchexecute responses, leaving the parseable envelope.
func StripBatchPrefix(body string) string {
	body = strings.TrimPrefix(body, ")]}'")
	return strings.TrimLeft(body, "\r\n \t")
}
