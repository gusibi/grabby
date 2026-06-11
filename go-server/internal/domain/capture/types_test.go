package capture

import "testing"

func TestPageContentMarkdownContentPrefersContent(t *testing.T) {
	page := PageContent{Content: "primary", Markdown: "fallback"}

	if got := page.MarkdownContent(); got != "primary" {
		t.Fatalf("MarkdownContent() = %q, want %q", got, "primary")
	}
}

func TestPageContentMarkdownContentFallsBackToMarkdown(t *testing.T) {
	page := PageContent{Markdown: "fallback"}

	if got := page.MarkdownContent(); got != "fallback" {
		t.Fatalf("MarkdownContent() = %q, want %q", got, "fallback")
	}
}
