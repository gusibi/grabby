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
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// FrontMatterField is a single key-value entry parsed from YAML frontmatter.
type FrontMatterField struct {
	Key   string
	Value string
}

var frontMatterKeyRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var frontMatterLabels = map[string]string{
	"title":       "标题",
	"source":      "来源链接",
	"author":      "作者",
	"description": "简介",
	"published":   "发布时间",
	"site":        "站点",
	"captured_at": "抓取时间",
}

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
// Leading YAML frontmatter is stripped from the Markdown body (preventing
// goldmark from turning it into <hr> + Setext <h2>) and rendered as a
// default-collapsed metadata table above the article body.
func Render(content string) string {
	content = normalize(content)
	if content == "" {
		return ""
	}

	fields, body := ExtractFrontMatter(content)
	prefix := renderFrontMatterHTML(fields)
	if body == "" {
		return prefix
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		// A renderer failure should not blank the reader. Fall back to a
		// sanitized escaped paragraph.
		return prefix + sanitizer.Sanitize("<p>"+html.EscapeString(body)+"</p>")
	}
	return prefix + sanitizer.Sanitize(buf.String())
}

// ExtractFrontMatter parses and strips a leading YAML frontmatter block
// (`---\nkey: value\n---`). If content does not start with valid frontmatter,
// it returns (nil, content).
func ExtractFrontMatter(content string) ([]FrontMatterField, string) {
	s := strings.TrimPrefix(strings.ReplaceAll(content, "\r\n", "\n"), "\uFEFF")
	s = strings.TrimLeft(s, " \t\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, content
	}
	rest := s[len("---\n"):]
	lines := strings.Split(rest, "\n")

	closeIdx := -1
	var fields []FrontMatterField
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			closeIdx = i
			break
		}
		if trimmed == "" {
			if i == 0 {
				return nil, content
			}
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			return nil, content
		}
		key := strings.TrimSpace(line[:colon])
		if !frontMatterKeyRe.MatchString(key) {
			return nil, content
		}
		val := unquoteFrontMatterValue(strings.TrimSpace(line[colon+1:]))
		if val != "" {
			fields = append(fields, FrontMatterField{Key: key, Value: val})
		}
	}
	if closeIdx < 0 || len(fields) == 0 {
		return nil, content
	}
	body := strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))
	return fields, body
}

func unquoteFrontMatterValue(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		inner := v[1 : len(v)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], `''`, `'`)
	}
	return v
}

func renderFrontMatterHTML(fields []FrontMatterField) string {
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<details class="gb-frontmatter"><summary>`)
	b.WriteString(fmt.Sprintf("页面元信息 · %d 项", len(fields)))
	b.WriteString(`</summary><table><tbody>`)
	for _, f := range fields {
		label := frontMatterLabels[strings.ToLower(f.Key)]
		if label == "" {
			label = f.Key
		}
		b.WriteString(`<tr><th>` + html.EscapeString(label) + `</th><td>`)
		if isSafeHTTPURL(f.Value) {
			esc := html.EscapeString(f.Value)
			b.WriteString(`<a href="` + esc + `" target="_blank" rel="nofollow noreferrer">` + esc + `</a>`)
		} else {
			b.WriteString(html.EscapeString(f.Value))
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table></details>`)
	return b.String()
}

func isSafeHTTPURL(s string) bool {
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
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
