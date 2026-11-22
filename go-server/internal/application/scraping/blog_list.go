package scraping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// WebConfig controls a web_scrape subscription. Two shapes are supported:
//
//   - single page: no selectors/pattern. src.URL itself is the article.
//   - list page: list_selector scopes the region, article_link_selector picks
//     anchors inside it, and/or article_url_pattern filters URLs. The same
//     discovery + extraction flow works for any public blog listing; site
//     differences are expressed only in config.
type WebConfig struct {
	ListSelector        string `json:"list_selector"`
	ArticleLinkSelector string `json:"article_link_selector"`
	ArticleURLPattern   string `json:"article_url_pattern"`
	ExcludeURLPattern   string `json:"exclude_url_pattern"`
	MaxItems            int    `json:"max_items"`
	Concurrency         int    `json:"concurrency"`
	// FullContent defaults to true for web sources: opening the article page is
	// the whole point of a blog list subscription.
	FullContent *bool `json:"full_content"`
	// UseBrowser forces the browser executor instead of direct HTTP discovery.
	UseBrowser *bool `json:"use_browser"`
}

func (c WebConfig) listMode() bool {
	return strings.TrimSpace(c.ListSelector) != "" ||
		strings.TrimSpace(c.ArticleLinkSelector) != "" ||
		strings.TrimSpace(c.ArticleURLPattern) != ""
}

type listCandidate struct {
	URL   string
	Title string
}

var (
	nonArticlePathRegex = regexp.MustCompile(`(?i)/(tag|tags|category|categories|author|authors|topic|topics|about|about-us|contact|privacy|terms|legal|login|signin|sign-up|signup|register|search|feed|rss|subscribe|newsletter|account|help|careers|jobs|events|cookie|cookies|shop|store|pricing|company|solutions|products|resources|docs|documentation|demo|press|partners|support|community|security|trust)(/|$)`)
	assetPathRegex      = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp|svg|css|js|json|xml|ico|woff2?|ttf|zip|gz|mp4|mp3|pdf)$`)
	genericLinkText     = map[string]bool{
		"home": true, "more": true, "read more": true, "learn more": true, "next": true,
		"previous": true, "older": true, "newer": true, "back": true, "all": true,
		"see all": true, "view all": true, "pricing": true, "company": true, "careers": true,
		"contact": true, "login": true, "sign in": true, "sign up": true, "get started": true,
		"book a demo": true, "docs": true, "documentation": true, "products": true,
		"solutions": true, "customers": true, "resources": true,
		"首页": true, "更多": true, "阅读全文": true,
	}
)

// fetchListDocument downloads a public listing page and returns the parsed DOM
// plus the redirect-resolved base URL used to resolve relative links.
func fetchListDocument(ctx context.Context, client *http.Client, listURL string) (*goquery.Document, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %d fetching list page %s", resp.StatusCode, listURL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLFetchBytes))
	if err != nil {
		return nil, nil, err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, err
	}
	base := resp.Request.URL
	if base == nil {
		base, _ = url.Parse(listURL)
	}
	// <base href> overrides the request URL for relative links.
	if href, ok := doc.Find("base[href]").First().Attr("href"); ok {
		if u, err := base.Parse(strings.TrimSpace(href)); err == nil {
			base = u
		}
	}
	return doc, base, nil
}

// discoverArticleLinks extracts, resolves and filters candidate article links
// from a listing page. It is intentionally deterministic: document order is
// preserved and duplicates are dropped by canonical URL.
func discoverArticleLinks(doc *goquery.Document, base *url.URL, listURL string, cfg WebConfig) ([]listCandidate, error) {
	var includeRe, excludeRe *regexp.Regexp
	var err error
	if p := strings.TrimSpace(cfg.ArticleURLPattern); p != "" {
		if includeRe, err = regexp.Compile(p); err != nil {
			return nil, fmt.Errorf("invalid article_url_pattern: %w", err)
		}
	}
	if p := strings.TrimSpace(cfg.ExcludeURLPattern); p != "" {
		if excludeRe, err = regexp.Compile(p); err != nil {
			return nil, fmt.Errorf("invalid exclude_url_pattern: %w", err)
		}
	}

	scope := doc.Selection
	if sel := strings.TrimSpace(cfg.ListSelector); sel != "" {
		scoped := doc.Find(sel)
		if scoped.Length() == 0 {
			return nil, nil
		}
		scope = scoped
	}

	var anchors *goquery.Selection
	if sel := strings.TrimSpace(cfg.ArticleLinkSelector); sel != "" {
		anchors = scope.Find(sel)
	} else {
		anchors = scope.Find("a[href]")
	}

	listCanonical := canonicalLinkKey(listURL)
	seen := map[string]bool{}
	var out []listCandidate

	anchors.Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			href = sel.Find("a[href]").First().AttrOr("href", "")
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") ||
			strings.HasPrefix(strings.ToLower(href), "mailto:") ||
			strings.HasPrefix(strings.ToLower(href), "tel:") {
			return
		}
		ref, err := url.Parse(href)
		if err != nil {
			return
		}
		abs := base.ResolveReference(ref)
		if abs.Scheme != "http" && abs.Scheme != "https" {
			return
		}
		abs.Fragment = ""

		key := canonicalLinkKey(abs.String())
		if key == "" || key == listCanonical || seen[key] {
			return
		}

		if !sameSite(base, abs) && includeRe == nil {
			return
		}
		if strings.TrimSpace(abs.Path) == "" || abs.Path == "/" {
			return
		}
		if nonArticlePathRegex.MatchString(abs.Path) {
			return
		}
		if assetPathRegex.MatchString(strings.ToLower(abs.Path)) {
			return
		}
		if includeRe != nil && !includeRe.MatchString(abs.String()) {
			return
		}
		if excludeRe != nil && excludeRe.MatchString(abs.String()) {
			return
		}

		title := linkText(sel)
		if genericLinkText[strings.ToLower(title)] {
			return
		}
		if title == "" && includeRe == nil {
			// Without an explicit rule an icon/empty anchor is almost always
			// chrome rather than an article title.
			return
		}

		seen[key] = true
		out = append(out, listCandidate{URL: abs.String(), Title: title})
		if len(out) >= 200 {
			return
		}
	})
	return out, nil
}

func linkText(sel *goquery.Selection) string {
	text := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(sel.Text(), " "))
	if text != "" {
		return text
	}
	if t := strings.TrimSpace(sel.AttrOr("title", "")); t != "" {
		return t
	}
	if alt := strings.TrimSpace(sel.Find("img[alt]").First().AttrOr("alt", "")); alt != "" {
		return alt
	}
	return ""
}

func sameSite(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	ha := strings.TrimPrefix(strings.ToLower(a.Hostname()), "www.")
	hb := strings.TrimPrefix(strings.ToLower(b.Hostname()), "www.")
	return ha == hb
}

// canonicalLinkKey normalises a URL for de-duplication: no fragment, lowercase
// scheme/host, no default port, and no trailing slash on non-root paths.
func canonicalLinkKey(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if u.Scheme == "" {
		return ""
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) ||
		(u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = u.Hostname()
	}
	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	// Drop campaign noise so the same article linked from two placements is
	// stored once. Identity-bearing query parameters are kept.
	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			lower := strings.ToLower(key)
			switch {
			case strings.HasPrefix(lower, "utm_"), lower == "fbclid", lower == "gclid",
				lower == "mc_cid", lower == "mc_eid", lower == "ref_src":
				q.Del(key)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// resolveArticleURL resolves a possibly-relative RSS link against the feed URL.
func resolveArticleURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.IsAbs() {
		return ref
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	return b.ResolveReference(u).String()
}
