package main

import (
	"encoding/json"
	"time"
)

// ---------- WebSocket Messages ----------

// BrowserRequest is sent from server to browser extension via WebSocket.
type BrowserRequest struct {
	Type      string `json:"type,omitempty"`
	Source    string `json:"source,omitempty"`
	Action    string `json:"action,omitempty"`
	Command   string `json:"command"`
	URL       string `json:"url"`
	FullPage  bool   `json:"fullPage,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Browser   string `json:"browser,omitempty"`
}

// BrowserResponse is returned by the browser extension.
type BrowserResponse struct {
	Type      string     `json:"type,omitempty"`
	MessageID string     `json:"message_id,omitempty"`
	Command   string     `json:"command,omitempty"`
	Success   bool       `json:"success,omitempty"`
	Error     string     `json:"error,omitempty"`
	Result    PageResult `json:"result,omitempty"`
}

// PageResult is the nested result data from the browser.
type PageResult struct {
	URL       string      `json:"url"`
	Title     string      `json:"title"`
	Timestamp string      `json:"timestamp"`
	Content   PageContent `json:"content"`
	ImageData string      `json:"imageData"`
	Format    string      `json:"format"`
	Quality   int         `json:"quality"`
}

// PageContent is the extracted page content (Markdown from defuddle).
type PageContent struct {
	Title      string `json:"title"`
	Content    string `json:"content"`  // Markdown
	Markdown   string `json:"markdown"` // Redundant field for clarity
	Author     string `json:"author"`
	Published  string `json:"published"`
	Site       string `json:"site"`
	Language   string `json:"language"`
	WordCount  int    `json:"wordCount"`
	Image      string `json:"image"`
	Favicon    string `json:"favicon"`
	Domain     string `json:"domain"`
	HTML       string `json:"html"`
	TextLength int    `json:"textLength"`
}

// Markdown returns the markdown content, preferring the Content field.
func (pc PageContent) MarkdownContent() string {
	if pc.Content != "" {
		return pc.Content
	}
	return pc.Markdown
}

// ---------- HTTP API Types ----------

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
	Success bool                `json:"success"`
	Browser BrowserRegistration `json:"browser"`
}

// ExtractAPIResponse is the POST /api/extract response body.
type ExtractAPIResponse struct {
	Success  bool   `json:"success"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
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

// HealthResponse is the GET /api/health response body.
type HealthResponse struct {
	Status           string    `json:"status"`
	BrowserConnected bool      `json:"browser_connected"`
	Timestamp        time.Time `json:"timestamp"`
}

// ---------- MCP Tool Parameter Types ----------

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
	Browsers []BrowserInfo `json:"browsers"`
	Count    int           `json:"count"`
}

// AddParams for the "add" MCP tool.
type AddParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// parseArgs converts raw MCP arguments (any/map) into a typed struct.
func parseArgs[T any](raw any) (T, error) {
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
