// Package render turns scraped content into safe HTML for the reader views.
//
// Scraped feeds are inconsistent: some sources return Markdown, some return
// HTML, and some return HTML that has been escaped once or twice by an RSS
// layer. Rendering therefore has to accept mixed Markdown + HTML. Goldmark
// (already used by the project) handles both when raw HTML is enabled, and
// bluemonday sanitizes the result so scripts, event handlers and javascript:
// URLs never reach the UI.
//
// Keeping this in Go means the same renderer can be reused when the desktop UI
// is eventually rebuilt with native views instead of a WebView.
package render

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var (
	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			extension.Typographer,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// Scraped content often contains legitimate raw HTML (images, tables,
		// <details>). Goldmark escapes raw HTML by default; enable it and rely
		// on bluemonday below for safety.
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)

	sanitizer = newSanitizer()

	// escapedBlockTag detects HTML that was entity-escaped by an upstream RSS
	// layer (`&lt;p&gt;...`) or escaped twice (`&amp;lt;p&amp;gt;`). Only
	// block-level tags are used as the signal so a code snippet containing
	// `&lt;span&gt;` is not decoded by accident.
	escapedBlockTag = regexp.MustCompile(`(?i)&(?:amp;)?lt;/?\s*(p|div|h[1-6]|ul|ol|li|blockquote|pre|table|thead|tbody|tr|td|th|article|section|figure|figcaption|img|a|br|hr)\b`)
)

func newSanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// UGCPolicy is built for Markdown output plus raw HTML. Keep the useful
	// link hardening defaults and allow the lazy-loading attributes modern
	// images use.
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("loading", "decoding").OnElements("img")
	return p
}

// Render converts mixed Markdown/HTML into a sanitized HTML fragment.
func Render(content string) string {
	content = normalize(content)
	if content == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		// A renderer failure should not blank the reader. Fall back to a
		// sanitized escaped paragraph.
		return sanitizer.Sanitize("<p>" + html.EscapeString(content) + "</p>")
	}
	return sanitizer.Sanitize(buf.String())
}

// normalize decodes HTML entities when the payload looks like escaped block
// HTML. Two passes cover the common "RSS description contains escaped HTML"
// and "escaped HTML inside JSON" cases without repeatedly decoding arbitrary
// ampersands.
func normalize(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	for i := 0; i < 2 && escapedBlockTag.MatchString(content); i++ {
		decoded := html.UnescapeString(content)
		if decoded == content {
			break
		}
		content = decoded
	}
	return content
}
