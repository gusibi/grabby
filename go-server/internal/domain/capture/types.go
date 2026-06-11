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

// MarkdownContent returns the markdown content, preferring the Content field.
func (pc PageContent) MarkdownContent() string {
	if pc.Content != "" {
		return pc.Content
	}
	return pc.Markdown
}
