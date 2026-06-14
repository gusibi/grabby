package capture

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

	// --- Additive fields, used only by new commands (runPageScript / fetchInPage /
	// intercept). Old commands and old extensions ignore them. See
	// docs/browser-executor-plan.md §6. ---

	// Params carries arbitrary command-specific arguments.
	Params map[string]any `json:"params,omitempty"`
	// TimeoutMs is a per-command timeout hint for the extension.
	TimeoutMs int `json:"timeoutMs,omitempty"`
	// Visible asks the extension to briefly activate the tab (for lazy-load /
	// infinite-scroll pages) and restore the previous tab afterwards.
	Visible bool `json:"visible,omitempty"`
	// CloseTab tells the extension to close the temporary tab when done.
	CloseTab bool `json:"closeTab,omitempty"`
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

	// --- Additive fields, filled only by new commands. Old commands leave them
	// empty; old extensions never set them. See docs/browser-executor-plan.md §6. ---

	// Text carries raw text results (e.g. fetchInPage body).
	Text string `json:"text,omitempty"`
	// JSON carries a single structured object result.
	JSON map[string]any `json:"json,omitempty"`
	// Items carries a list of structured results (e.g. intercepted responses).
	Items []any `json:"items,omitempty"`
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

// MarkdownContent returns the markdown content, preferring the Content field.
func (pc PageContent) MarkdownContent() string {
	if pc.Content != "" {
		return pc.Content
	}
	return pc.Markdown
}
