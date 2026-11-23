package scraping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

// WebConfig controls a web_scrape subscription. Two shapes are supported:
//
//   - single page: no crawl_links/selectors/pattern. src.URL itself is the article.
//   - list page: crawl_links auto-discovers links in the main content, or
//     list_selector scopes the region, article_link_selector picks anchors
//     inside it, and/or article_url_pattern filters URLs. The same discovery +
//     extraction flow works for any public blog listing; site differences are
//     expressed only in config.
type WebConfig struct {
	// CrawlLinks is the friendly, selector-free list mode: links found in the
	// page's main content are stored as separate articles. Navigation, category,
	// pagination and image-only links are filtered out automatically.
	CrawlLinks          bool   `json:"crawl_links"`
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
	return c.CrawlLinks || strings.TrimSpace(c.ListSelector) != "" ||
		strings.TrimSpace(c.ArticleLinkSelector) != "" ||
		strings.TrimSpace(c.ArticleURLPattern) != ""
}

// autoListMode is crawl_links without any manual selectors or URL rules, so
// discovery has to figure out the main content and the article links itself.
func (c WebConfig) autoListMode() bool {
	return c.CrawlLinks &&
		strings.TrimSpace(c.ListSelector) == "" &&
		strings.TrimSpace(c.ArticleLinkSelector) == "" &&
		strings.TrimSpace(c.ArticleURLPattern) == ""
}

type listCandidate struct {
	URL   string
	Title string
}

var (
	nonArticlePathRegex = regexp.MustCompile(`(?i)/(tag|tags|category|categories|author|authors|topic|topics|about|about-us|contact|privacy|terms|legal|login|signin|sign-up|signup|register|search|feed|rss|subscribe|newsletter|account|help|careers|jobs|events|cookie|cookies|shop|store|pricing|company|solutions|products|resources|docs|documentation|demo|press|partners|support|community|security|trust|photo|photos|photography|gallery|images?)(/|$)`)
	assetPathRegex      = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp|svg|css|js|json|xml|ico|woff2?|ttf|zip|gz|mp4|mp3)$`)
	headingSelector     = "h1, h2, h3, h4, h5, h6"
	// Chrome that is always outside the article body. <header> and <footer> are
	// intentionally left alone: blog cards often put their title link there.
	autoChromeSelector = "script, style, noscript, iframe, svg, nav, aside, form, " +
		"[role=navigation], [role=banner], [role=contentinfo], [role=complementary], " +
		"[aria-hidden=true], [hidden]"
	genericLinkText = map[string]bool{
		"home": true, "more": true, "read more": true, "learn more": true, "next": true,
		"previous": true, "older": true, "newer": true, "back": true, "all": true,
		"see all": true, "view all": true, "pricing": true, "company": true, "careers": true,
		"contact": true, "login": true, "sign in": true, "sign up": true, "get started": true,
		"book a demo": true, "docs": true, "documentation": true, "products": true,
		"solutions": true, "customers": true, "resources": true,
		"read article": true, "read the article": true, "continue reading": true,
		"view more": true, "load more": true, "show more": true, "see more": true,
		"skip to content": true, "menu": true, "search": true, "share": true,
		"follow": true, "subscribe": true, "print": true, "download": true,
		"首页": true, "更多": true, "阅读全文": true, "阅读更多": true, "查看更多": true,
		"加载更多": true, "全部": true, "主页": true, "关于": true, "联系": true,
		"隐私": true, "条款": true, "登录": true, "注册": true, "订阅": true,
		"分享": true, "评论": true, "返回": true, "上一篇": true, "下一篇": true,
	}
	numericLinkTextRegex = regexp.MustCompile(`^[\d\s./:+\-–—|]+$`)
	// Class/id names that mark site chrome when they appear outside main/article.
	// Matching is by whole class name or hyphen/underscore segment so a class
	// such as "shareholder" is not mistaken for "share".
	chromeClassTokens = []string{
		"sidebar", "site-header", "site-footer", "masthead", "navbar", "navigation",
		"breadcrumb", "breadcrumbs", "pagination", "pager", "social", "share",
		"related", "recommended", "recommendations", "comments", "advert",
		"advertisement", "cookie", "newsletter", "subscribe", "promo", "modal",
		"popup", "toolbar", "site-search", "nav-bar", "footer-nav",
	}
	chromeClassParts = []string{
		"sidebar", "nav", "navbar", "navigation", "breadcrumb", "pagination",
		"pager", "social", "share", "related", "comment", "comments", "advert",
		"cookie", "newsletter", "subscribe", "promo", "modal", "popup", "toolbar",
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
//
// With crawl_links and no manual selectors it also finds the main content and
// keeps only article-looking links, so users never have to write CSS.
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

	auto := cfg.autoListMode()
	scope := doc.Selection
	if sel := strings.TrimSpace(cfg.ListSelector); sel != "" {
		scoped := doc.Find(sel)
		if scoped.Length() == 0 {
			return nil, nil
		}
		scope = scoped
	} else if auto {
		scope = autoContentScope(doc)
	}

	var anchors *goquery.Selection
	if sel := strings.TrimSpace(cfg.ArticleLinkSelector); sel != "" {
		anchors = scope.Find(sel)
	} else {
		anchors = scope.Find("a[href]")
	}

	listCanonical := canonicalLinkKey(listURL)
	var found []discoveredLink

	anchors.Each(func(_ int, sel *goquery.Selection) {
		if auto && isDecorativeAnchor(sel) {
			return
		}
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
		if key == "" || key == listCanonical {
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
		if auto && isPaginationURL(abs) {
			return
		}

		title := linkText(sel)
		heading := false
		if auto {
			title = autoLinkTitle(sel)
			heading = linkHasHeading(sel)
		}
		if genericLinkText[strings.ToLower(title)] {
			return
		}
		if auto && !plausibleAutoTitle(title) {
			return
		}
		if title == "" && includeRe == nil {
			// Without an explicit rule an icon/empty anchor is almost always
			// chrome rather than an article title.
			return
		}

		found = append(found, discoveredLink{URL: abs.String(), Title: title, Heading: heading})
	})

	if auto {
		return autoPickCandidates(found), nil
	}

	seen := map[string]bool{}
	out := make([]listCandidate, 0, len(found))
	for _, link := range found {
		key := canonicalLinkKey(link.URL)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, listCandidate{URL: link.URL, Title: link.Title})
		if len(out) >= 200 {
			break
		}
	}
	return out, nil
}

type discoveredLink struct {
	URL     string
	Title   string
	Heading bool
}

// autoContentScope strips obvious page chrome and returns the region that holds
// the page's main content: <main> when the site has one, otherwise <article>
// elements, otherwise the remaining body.
func autoContentScope(doc *goquery.Document) *goquery.Selection {
	doc.Find(autoChromeSelector).Remove()
	doc.Find("[class], [id]").Each(func(_ int, sel *goquery.Selection) {
		if sel.Is("html, body, main, article, [role=main]") {
			return
		}
		// Never remove a wrapper that holds the content itself, e.g. a layout
		// whose class is "has-sidebar", and leave long text blocks alone so a
		// mislabelled main column survives.
		if sel.Find("main, article, [role=main]").Length() > 0 {
			return
		}
		if utf8.RuneCountInString(strings.TrimSpace(sel.Text())) >= 600 {
			return
		}
		if isChromeElement(sel) {
			sel.Remove()
		}
	})

	for _, candidate := range []string{
		"main", "[role=main]", "#main-content", "#content", "#main",
		".post-list", ".article-list", ".news-list",
		".entry-content", ".post-content", ".article-content", ".markdown-body",
	} {
		scope := doc.Find(candidate)
		if scope.Length() > 0 && scope.Find("a[href]").Length() > 0 {
			return scope
		}
	}
	if articles := doc.Find("article"); articles.Length() > 0 && articles.Find("a[href]").Length() > 0 {
		return articles
	}
	return doc.Find("body")
}

func isChromeElement(sel *goquery.Selection) bool {
	raw := strings.ToLower(strings.TrimSpace(sel.AttrOr("class", "") + " " + sel.AttrOr("id", "")))
	if raw == "" {
		return false
	}
	for _, name := range strings.Fields(raw) {
		for _, token := range chromeClassTokens {
			if name == token {
				return true
			}
		}
		for _, part := range strings.FieldsFunc(name, func(r rune) bool {
			return r == '-' || r == '_'
		}) {
			for _, token := range chromeClassParts {
				if part == token {
					return true
				}
			}
		}
	}
	return false
}

// autoPickCandidates keeps the links that look like article entries. Links that
// sit in a heading (card titles, post titles) win; pages without headings fall
// back to all text links. A URL repeated like a filter pill is dropped.
func autoPickCandidates(found []discoveredLink) []listCandidate {
	if len(found) == 0 {
		return nil
	}

	headingCount := 0
	for _, link := range found {
		if link.Heading {
			headingCount++
		}
	}
	pool := found
	// Apply the heading rule only when headings are a real pattern on the page
	// (a list of cards), not when one inline link happens to sit in a heading.
	if headingCount >= 2 && headingCount*3 >= len(found) {
		pool = make([]discoveredLink, 0, headingCount)
		for _, link := range found {
			if link.Heading {
				pool = append(pool, link)
			}
		}
	}

	counts := map[string]int{}
	for _, link := range pool {
		counts[canonicalLinkKey(link.URL)]++
	}
	median := medianCount(counts)
	limit := 6
	if 4*median > limit {
		limit = 4 * median
	}

	seen := map[string]bool{}
	out := make([]listCandidate, 0, len(pool))
	for _, link := range pool {
		key := canonicalLinkKey(link.URL)
		if counts[key] > limit || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, listCandidate{URL: link.URL, Title: link.Title})
		if len(out) >= 200 {
			break
		}
	}
	return out
}

func medianCount(counts map[string]int) int {
	if len(counts) == 0 {
		return 0
	}
	values := make([]int, 0, len(counts))
	for _, n := range counts {
		values = append(values, n)
	}
	sort.Ints(values)
	return values[len(values)/2]
}

// isDecorativeAnchor drops image-only links (photo galleries, card thumbnails)
// and anchors hidden from assistive tech, which are duplicate card links.
func isDecorativeAnchor(sel *goquery.Selection) bool {
	if strings.EqualFold(strings.TrimSpace(sel.AttrOr("aria-hidden", "")), "true") {
		return true
	}
	return collapseSpace(sel.Text()) == ""
}

func linkHasHeading(sel *goquery.Selection) bool {
	return sel.Find(headingSelector).Length() > 0 || sel.ParentsFiltered(headingSelector).Length() > 0
}

// autoLinkTitle prefers the card heading over the whole anchor text, which on
// card-style listings also contains the excerpt and "Read more".
func autoLinkTitle(sel *goquery.Selection) string {
	if h := sel.Find(headingSelector).First(); h.Length() > 0 {
		if title := collapseSpace(h.Text()); title != "" {
			return title
		}
	}
	if h := sel.ParentsFiltered(headingSelector).First(); h.Length() > 0 {
		if title := collapseSpace(h.Text()); title != "" {
			return title
		}
	}
	return collapseSpace(sel.Text())
}

func plausibleAutoTitle(title string) bool {
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) < 2 {
		return false
	}
	if numericLinkTextRegex.MatchString(title) {
		return false
	}
	return !genericLinkText[strings.ToLower(title)]
}

func collapseSpace(s string) string {
	return strings.TrimSpace(multiSpaceRegex.ReplaceAllString(s, " "))
}

func isPaginationURL(u *url.URL) bool {
	q := u.Query()
	for _, key := range []string{"page", "paged", "offset", "start", "from"} {
		if q.Has(key) {
			return true
		}
	}
	return false
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
