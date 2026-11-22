package scraping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

const (
	defaultDeepFetchMinRunes = 300
	maxHTMLFetchBytes        = 5 * 1024 * 1024
)

var (
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
	multiSpaceRegex   = regexp.MustCompile(`[ \t\f\v]+`)
	tagStripRegex     = regexp.MustCompile(`<[^>]*>`)
)

// ArticleExtract is the result of opening a public article page. PublishedAt is
// best-effort: many blogs only expose it through OpenGraph metadata or JSON-LD.
type ArticleExtract struct {
	Title       string
	Markdown    string
	PublishedAt *time.Time
	FinalURL    string
}

// ExtractWebArticle fetches pageURL via HTTP and extracts the article title and
// clean Markdown body using goquery heuristics.
func ExtractWebArticle(ctx context.Context, pageURL string) (string, string, error) {
	res, err := ExtractWebArticleDetails(ctx, pageURL)
	if err != nil {
		return "", "", err
	}
	return res.Title, res.Markdown, nil
}

// ExtractWebArticleDetails extracts title, Markdown, publication time and the
// redirect-resolved URL. Redirects are followed and relative links inside the
// body are resolved against the final URL.
func ExtractWebArticleDetails(ctx context.Context, pageURL string) (ArticleExtract, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return extractWebArticleWithClient(ctx, client, pageURL)
}

func extractWebArticleWithClient(ctx context.Context, client *http.Client, pageURL string) (ArticleExtract, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return ArticleExtract{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return ArticleExtract{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ArticleExtract{}, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, pageURL)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLFetchBytes))
	if err != nil {
		return ArticleExtract{}, err
	}

	baseURL := resp.Request.URL
	if baseURL == nil {
		baseURL, _ = url.Parse(pageURL)
	}

	res, err := extractArticleFromHTMLBytes(bodyBytes, baseURL)
	if err != nil {
		return ArticleExtract{}, err
	}
	if baseURL != nil {
		res.FinalURL = baseURL.String()
	}
	return res, nil
}

func extractArticleFromHTMLBytes(bodyBytes []byte, baseURL *url.URL) (ArticleExtract, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return ArticleExtract{}, err
	}

	title := strings.TrimSpace(doc.Find(`meta[property="og:title"]`).AttrOr("content", ""))
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	published := extractPublishedAt(doc)

	// Remove non-content noise before measuring candidates.
	doc.Find("script, style, noscript, iframe, svg, nav, footer, aside, form, .sidebar, .advertisement, .ads, .comments, #comments, .comment-list, .social-share, .related-posts").Remove()

	selectors := []string{
		"article",
		"main",
		`[role="main"]`,
		".post-content",
		".article-content",
		".entry-content",
		".markdown-body",
		".rich_media_content",
		"#content",
		"#main-content",
		"body",
	}

	var bestSel *goquery.Selection
	bestRunes := 0
	for _, sel := range selectors {
		candidate := doc.Find(sel).First()
		if candidate.Length() == 0 {
			continue
		}
		textLen := utf8.RuneCountInString(strings.TrimSpace(candidate.Text()))
		if textLen >= defaultDeepFetchMinRunes {
			bestSel = candidate
			break
		}
		if textLen > bestRunes {
			bestSel = candidate
			bestRunes = textLen
		}
	}

	if bestSel == nil || bestSel.Length() == 0 {
		return ArticleExtract{Title: title, PublishedAt: published}, nil
	}

	var buf strings.Builder
	for _, node := range bestSel.Nodes {
		renderNodeToMarkdown(&buf, node, baseURL, 0)
	}

	md := multiNewlineRegex.ReplaceAllString(strings.TrimSpace(buf.String()), "\n\n")
	return ArticleExtract{Title: title, Markdown: md, PublishedAt: published}, nil
}

func extractPublishedAt(doc *goquery.Document) *time.Time {
	candidates := []string{
		`meta[property="article:published_time"]`,
		`meta[property="og:article:published_time"]`,
		`meta[name="article:published_time"]`,
		`meta[name="pubdate"]`,
		`meta[name="date"]`,
		`meta[itemprop="datePublished"]`,
	}
	for _, sel := range candidates {
		if raw := strings.TrimSpace(doc.Find(sel).AttrOr("content", "")); raw != "" {
			if t, ok := parsePublishedTime(raw); ok {
				return &t
			}
		}
	}
	if raw, ok := doc.Find("time[datetime]").First().Attr("datetime"); ok {
		if t, ok := parsePublishedTime(raw); ok {
			return &t
		}
	}
	// JSON-LD is common on company blogs (for example Next.js sites).
	var jsonLD *time.Time
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return true
		}
		if t, ok := publishedFromJSONLD(raw); ok {
			jsonLD = &t
			return false
		}
		return true
	})
	if jsonLD != nil {
		return jsonLD
	}
	return nil
}

func publishedFromJSONLD(raw string) (time.Time, bool) {
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return time.Time{}, false
	}
	var walk func(any) (time.Time, bool)
	walk = func(v any) (time.Time, bool) {
		switch node := v.(type) {
		case []any:
			for _, child := range node {
				if t, ok := walk(child); ok {
					return t, true
				}
			}
		case map[string]any:
			if s, ok := node["datePublished"].(string); ok && strings.TrimSpace(s) != "" {
				if t, ok := parsePublishedTime(s); ok {
					return t, true
				}
			}
			for _, child := range node {
				if t, ok := walk(child); ok {
					return t, true
				}
			}
		}
		return time.Time{}, false
	}
	return walk(root)
}

func parsePublishedTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC1123Z,
		time.RFC1123,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// HTMLToMarkdown converts an HTML fragment (such as RSS Show Notes) into clean Markdown.
func HTMLToMarkdown(rawHTML string) string {
	trimmed := strings.TrimSpace(rawHTML)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "<") {
		return stdhtml.UnescapeString(trimmed)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + trimmed + "</body>"))
	if err != nil {
		return stdhtml.UnescapeString(trimmed)
	}
	var buf strings.Builder
	if body := doc.Find("body").First(); body.Length() > 0 {
		for _, node := range body.Nodes {
			renderNodeToMarkdown(&buf, node, nil, 0)
		}
	}
	return multiNewlineRegex.ReplaceAllString(strings.TrimSpace(buf.String()), "\n\n")
}

// StripHTMLToText strips HTML tags and collapses whitespace for length checks.
func StripHTMLToText(raw string) string {
	noTags := tagStripRegex.ReplaceAllString(raw, " ")
	decoded := stdhtml.UnescapeString(noTags)
	return strings.TrimSpace(multiSpaceRegex.ReplaceAllString(decoded, " "))
}

// NeedsDeepFetch determines whether an RSS item's body is too short or link-only
// and returns (true, targetURL) when deeper page extraction is warranted.
func NeedsDeepFetch(content string, itemURL string) (bool, string) {
	targetURL := strings.TrimSpace(itemURL)
	plain := StripHTMLToText(content)

	// If the RSS link points to an aggregator discussion (e.g. Hacker News)
	// and the content holds an external link, prefer fetching the linked page.
	if targetURL != "" && isAggregatorDiscussionURL(targetURL) {
		for _, link := range extractLinks(content) {
			if !isAggregatorDiscussionURL(link) {
				targetURL = link
				break
			}
		}
	}

	if utf8.RuneCountInString(plain) < defaultDeepFetchMinRunes {
		return true, targetURL
	}

	// Detect boilerplate/link-only descriptions slightly over 300 chars.
	lower := strings.ToLower(plain)
	if utf8.RuneCountInString(plain) < 420 && (strings.Contains(lower, "comments") || strings.Contains(lower, "article url:")) && strings.Contains(lower, "http") {
		return true, targetURL
	}

	return false, targetURL
}

// RequiresBrowserExtract returns true for domains that usually block plain HTTP
// clients or depend heavily on JS rendering / user cookies.
func RequiresBrowserExtract(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	blockedHosts := []string{
		"mp.weixin.qq.com",
		"x.com",
		"twitter.com",
		"medium.com",
		"xiaohongshu.com",
	}
	for _, h := range blockedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

func isAggregatorDiscussionURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "news.ycombinator.com/item") ||
		strings.Contains(lower, "lobste.rs/s/")
}

func renderNodeToMarkdown(buf *strings.Builder, n *xhtml.Node, baseURL *url.URL, listDepth int) {
	if n == nil {
		return
	}
	switch n.Type {
	case xhtml.TextNode:
		text := multiSpaceRegex.ReplaceAllString(n.Data, " ")
		buf.WriteString(text)
		return
	case xhtml.ElementNode:
		tag := strings.ToLower(n.Data)
		switch tag {
		case "script", "style", "noscript", "iframe", "svg":
			return
		case "br":
			buf.WriteString("\n")
			return
		case "hr":
			buf.WriteString("\n\n---\n\n")
			return
		case "h1", "h2", "h3", "h4", "h5", "h6":
			level := int(tag[1] - '0')
			inner := collectChildrenText(n, baseURL)
			if inner != "" {
				buf.WriteString("\n\n" + strings.Repeat("#", level) + " " + inner + "\n\n")
			}
			return
		case "p":
			inner := collectChildrenMarkdown(n, baseURL, listDepth)
			if strings.TrimSpace(inner) != "" {
				buf.WriteString("\n\n" + strings.TrimSpace(inner) + "\n\n")
			}
			return
		case "pre":
			code := strings.TrimSpace(collectPlainText(n))
			if code != "" {
				buf.WriteString("\n\n```\n" + code + "\n```\n\n")
			}
			return
		case "code":
			if n.Parent != nil && strings.EqualFold(n.Parent.Data, "pre") {
				buf.WriteString(collectPlainText(n))
				return
			}
			code := strings.TrimSpace(collectPlainText(n))
			if code != "" {
				buf.WriteString("`" + code + "`")
			}
			return
		case "strong", "b":
			inner := strings.TrimSpace(collectChildrenMarkdown(n, baseURL, listDepth))
			if inner != "" {
				buf.WriteString("**" + inner + "**")
			}
			return
		case "em", "i":
			inner := strings.TrimSpace(collectChildrenMarkdown(n, baseURL, listDepth))
			if inner != "" {
				buf.WriteString("*" + inner + "*")
			}
			return
		case "blockquote":
			inner := strings.TrimSpace(collectChildrenMarkdown(n, baseURL, listDepth))
			if inner != "" {
				lines := strings.Split(inner, "\n")
				buf.WriteString("\n\n")
				for _, line := range lines {
					buf.WriteString("> " + strings.TrimSpace(line) + "\n")
				}
				buf.WriteString("\n")
			}
			return
		case "ul", "ol":
			buf.WriteString("\n")
			idx := 1
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == xhtml.ElementNode && strings.EqualFold(c.Data, "li") {
					inner := strings.TrimSpace(collectChildrenMarkdown(c, baseURL, listDepth+1))
					if inner != "" {
						indent := strings.Repeat("  ", listDepth)
						if tag == "ol" {
							buf.WriteString(fmt.Sprintf("%s%d. %s\n", indent, idx, inner))
							idx++
						} else {
							buf.WriteString(fmt.Sprintf("%s- %s\n", indent, inner))
						}
					}
				}
			}
			buf.WriteString("\n")
			return
		case "a":
			href := resolveAttrURL(n, "href", baseURL)
			text := strings.TrimSpace(collectChildrenMarkdown(n, baseURL, listDepth))
			if text == "" {
				return
			}
			if href != "" && !strings.HasPrefix(strings.ToLower(href), "javascript:") {
				buf.WriteString(fmt.Sprintf("[%s](%s)", text, href))
			} else {
				buf.WriteString(text)
			}
			return
		case "img":
			src := resolveAttrURL(n, "src", baseURL)
			if src != "" && strings.HasPrefix(strings.ToLower(src), "http") {
				alt := strings.TrimSpace(attrValue(n, "alt"))
				buf.WriteString(fmt.Sprintf("![%s](%s)", alt, src))
			}
			return
		case "div", "section", "article", "main", "blockquote_inner":
			buf.WriteString("\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderNodeToMarkdown(buf, c, baseURL, listDepth)
			}
			buf.WriteString("\n")
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNodeToMarkdown(buf, c, baseURL, listDepth)
	}
}

func collectChildrenMarkdown(n *xhtml.Node, baseURL *url.URL, listDepth int) string {
	var buf strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNodeToMarkdown(&buf, c, baseURL, listDepth)
	}
	return buf.String()
}

func collectChildrenText(n *xhtml.Node, baseURL *url.URL) string {
	return strings.TrimSpace(multiSpaceRegex.ReplaceAllString(collectPlainText(n), " "))
}

func collectPlainText(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == xhtml.TextNode {
		return n.Data
	}
	var buf strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(collectPlainText(c))
	}
	return buf.String()
}

func attrValue(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func resolveAttrURL(n *xhtml.Node, key string, baseURL *url.URL) string {
	raw := strings.TrimSpace(attrValue(n, key))
	if raw == "" {
		return ""
	}
	if baseURL == nil {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return baseURL.ResolveReference(u).String()
}
