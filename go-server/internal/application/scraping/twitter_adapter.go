package scraping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"go-server/internal/application/render"
	"go-server/internal/application/twitter"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

const (
	defaultTwitterMaxItems          = 40
	defaultTwitterMaxArticleFetches = 8
	defaultTwitterFetchTimeout      = 120 * time.Second
	minArticleBodyRunes             = 200
)

// TwitterConfig controls a twitter subscription source (stored in source.Config JSON).
type TwitterConfig struct {
	Kind              string `json:"kind,omitempty"`                // "for_you" (default) or "following"
	Browser           string `json:"browser,omitempty"`             // optional target browser name
	MaxItems          int    `json:"max_items,omitempty"`           // default 40
	ScrollRounds      int    `json:"scroll_rounds,omitempty"`       // default 6
	MaxArticleFetches int    `json:"max_article_fetches,omitempty"` // default 8; caps per-run full-article fetches
	MinLikes          int    `json:"min_likes,omitempty"`           // minimum favorite_count required to store
	MinViews          int    `json:"min_views,omitempty"`           // minimum view_count required to store
}

// TwitterAdapter fetches X/Twitter timeline, search, or likes via the connected
// browser extension and converts each tweet into a ScrapedItem for the reader
// and AI analysis pipeline.
type TwitterAdapter struct {
	scraper *Scraper
	// fetchFn allows tests to stub the browser-backed twitter fetch.
	fetchFn func(ctx context.Context, src source.Source, cfg TwitterConfig) ([]twitter.Tweet, error)
	// fetchArticleFn allows tests to stub full-article extraction (for X Articles and external links).
	fetchArticleFn func(ctx context.Context, targetURL string, preferBrowser bool) (title string, body string, err error)
}

func NewTwitterAdapter(s *Scraper) *TwitterAdapter {
	a := &TwitterAdapter{scraper: s}
	a.fetchFn = a.fetchFromBrowser
	a.fetchArticleFn = a.fetchArticleContent
	return a
}

func (a *TwitterAdapter) Type() string { return "twitter" }

func (a *TwitterAdapter) Fetch(ctx context.Context, src source.Source) ([]item.ScrapedItem, error) {
	items, _, err := a.FetchWithReports(ctx, src)
	return items, err
}

func (a *TwitterAdapter) FetchWithReports(ctx context.Context, src source.Source) ([]item.ScrapedItem, []ItemReport, error) {
	cfg := parseTwitterConfig(src.Config)

	fetchCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		fetchCtx, cancel = context.WithTimeout(ctx, defaultTwitterFetchTimeout)
		defer cancel()
	}

	tweets, err := a.fetchFn(fetchCtx, src, cfg)
	if err != nil {
		return nil, nil, err
	}

	category := strings.TrimSpace(src.DefaultCategory)
	if category == "" || category == "auto" {
		category = "tweet"
	}

	items := make([]item.ScrapedItem, 0, len(tweets))
	reports := make([]ItemReport, 0, len(tweets))
	keptForArchive := make([]twitter.Tweet, 0, len(tweets))
	seen := make(map[string]struct{}, len(tweets))
	articleFetches := 0

	for _, t := range tweets {
		tweetURL := normalizeTweetURL(t)
		if tweetURL == "" {
			continue
		}
		if _, dup := seen[tweetURL]; dup {
			continue
		}
		seen[tweetURL] = struct{}{}

		title := formatTweetTitle(t)

		if reason := tweetFilterReason(t, cfg); reason != "" {
			reports = append(reports, ItemReport{
				Title:    title,
				URL:      tweetURL,
				Category: category,
				Status:   "skipped",
				Reason:   reason,
			})
			continue
		}

		keptForArchive = append(keptForArchive, t)

		// Skip tweets already stored in the library so periodic timeline polling
		// does not re-trigger AI analysis on the same tweets.
		if a.scraper != nil && a.scraper.db != nil {
			if existing, err := a.scraper.db.GetScrapedItemFullByURL(tweetURL); err == nil && existing != nil {
				reports = append(reports, ItemReport{
					ItemID:   existing.ID,
					Title:    firstNonEmpty(existing.Title, title),
					URL:      tweetURL,
					Category: firstNonEmpty(existing.Category, category),
					Status:   "kept_existing",
					Reason:   "推文已存在，保留已有内容",
					Texts: []source.FetchTextResult{{
						Kind:   "content",
						Status: "kept_existing",
						Source: "twitter_timeline",
						URL:    tweetURL,
						Chars:  utf8.RuneCountInString(existing.Content),
					}},
				})
				continue
			}
		}

		var fetchedArticleTitle, fetchedArticleBody, fetchedArticleURL string
		var articleTextResult *source.FetchTextResult

		// 1) X Article without full body in GraphQL -> open via browser to grab full article.
		if t.IsArticle {
			if utf8.RuneCountInString(strings.TrimSpace(t.ArticleText)) >= minArticleBodyRunes {
				fetchedArticleTitle = t.ArticleTitle
				fetchedArticleBody = strings.TrimSpace(t.ArticleText)
				fetchedArticleURL = firstNonEmpty(t.ArticleURL, tweetURL)
			} else if articleFetches < cfg.MaxArticleFetches {
				articleFetches++
				targetURL := firstNonEmpty(t.ArticleURL, tweetURL)
				artTitle, artBody, artErr := a.fetchArticleFn(fetchCtx, targetURL, true)
				if artErr == nil && utf8.RuneCountInString(artBody) >= minArticleBodyRunes {
					fetchedArticleTitle = firstNonEmpty(t.ArticleTitle, artTitle)
					fetchedArticleBody = artBody
					fetchedArticleURL = targetURL
					articleTextResult = &source.FetchTextResult{
						Kind: "article", Status: "stored", Source: "x_article_browser",
						URL: targetURL, Chars: utf8.RuneCountInString(artBody),
					}
				} else if artErr != nil {
					articleTextResult = &source.FetchTextResult{
						Kind: "article", Status: "failed", Source: "x_article_browser",
						URL: targetURL, Reason: artErr.Error(), Stage: failureStage(artErr),
					}
				}
			}
		} else if len(t.Links) > 0 && articleFetches < cfg.MaxArticleFetches {
			// 2) Tweet linking to an external article -> fetch full article (HTTP first, browser fallback).
			extURL := t.Links[0]
			articleFetches++
			artTitle, artBody, artErr := a.fetchArticleFn(fetchCtx, extURL, false)
			if artErr == nil && utf8.RuneCountInString(artBody) >= minArticleBodyRunes {
				fetchedArticleTitle = artTitle
				fetchedArticleBody = artBody
				fetchedArticleURL = extURL
				articleTextResult = &source.FetchTextResult{
					Kind: "article", Status: "stored", Source: "linked_article",
					URL: extURL, Chars: utf8.RuneCountInString(artBody),
				}
			} else if artErr != nil {
				articleTextResult = &source.FetchTextResult{
					Kind: "article", Status: "skipped", Source: "linked_article",
					URL: extURL, Reason: artErr.Error(),
				}
			}
		}

		if t.IsArticle && fetchedArticleTitle != "" && t.ArticleTitle == "" {
			t.ArticleTitle = fetchedArticleTitle
			title = formatTweetTitle(t)
		}

		content := formatTweetMarkdown(t, fetchedArticleTitle, fetchedArticleBody, fetchedArticleURL)
		summarySrc := strings.TrimSpace(t.Text)
		if t.IsArticle && fetchedArticleBody != "" {
			summarySrc = firstNonEmpty(fetchedArticleTitle, t.ArticleTitle) + " — " + fetchedArticleBody
		}
		summary := truncateSummary(summarySrc, 240)
		pub := parseTweetTime(t.CreatedAt)
		origin := "X (Twitter)"
		if t.Author != "" {
			origin = "X · @" + t.Author
		}

		meta := map[string]any{
			"content_source": "twitter_timeline",
			"tweet_id":       t.ID,
			"author":         t.Author,
			"author_name":    t.AuthorName,
			"favorite_count": t.FavoriteCount,
			"retweet_count":  t.RetweetCount,
			"reply_count":    t.ReplyCount,
			"quote_count":    t.QuoteCount,
			"view_count":     t.ViewCount,
		}
		if len(t.Media) > 0 {
			meta["media"] = t.Media
		}
		if t.IsArticle {
			meta["is_article"] = true
		}
		if fetchedArticleURL != "" && fetchedArticleBody != "" {
			meta["article_fetched"] = true
			meta["article_url"] = fetchedArticleURL
		}

		itemCategory := category
		if t.IsArticle && (strings.TrimSpace(src.DefaultCategory) == "" || src.DefaultCategory == "auto") {
			itemCategory = "article"
		}

		items = append(items, item.ScrapedItem{
			SourceID:     src.ID,
			OriginSource: origin,
			Title:        title,
			URL:          tweetURL,
			Summary:      summary,
			Content:      content,
			Category:     itemCategory,
			Tags:         t.Author,
			PublishedAt:  pub,
			Metadata:     marshalMetadata(meta),
		})
		texts := []source.FetchTextResult{{
			Kind:   "content",
			Status: "stored",
			Source: "twitter_timeline",
			URL:    tweetURL,
			Chars:  utf8.RuneCountInString(content),
		}}
		if articleTextResult != nil {
			texts = append(texts, *articleTextResult)
		}
		reports = append(reports, ItemReport{
			Title:    title,
			URL:      tweetURL,
			Category: itemCategory,
			Status:   "stored",
			Texts:    texts,
		})
	}

	a.archiveTweets(keptForArchive, twitterArchiveSource(src.URL))

	return items, reports, nil
}

func tweetFilterReason(t twitter.Tweet, cfg TwitterConfig) string {
	var reasons []string
	if cfg.MinLikes > 0 && t.FavoriteCount < cfg.MinLikes {
		reasons = append(reasons, fmt.Sprintf("点赞 %d < %d", t.FavoriteCount, cfg.MinLikes))
	}
	if cfg.MinViews > 0 && t.ViewCount < cfg.MinViews {
		reasons = append(reasons, fmt.Sprintf("阅读 %d < %d", t.ViewCount, cfg.MinViews))
	}
	if len(reasons) == 0 {
		return ""
	}
	return "未达到筛选条件（" + strings.Join(reasons, "，") + "）"
}

func (a *TwitterAdapter) fetchArticleContent(ctx context.Context, targetURL string, preferBrowser bool) (string, string, error) {
	if preferBrowser {
		if a.scraper == nil || a.scraper.wsManager == nil {
			return "", "", fmt.Errorf("未连接浏览器扩展")
		}
		md, title, err := a.scraper.extractPageViaBrowser(ctx, targetURL)
		if err != nil {
			return "", "", err
		}
		_, body := render.ExtractFrontMatter(md)
		return strings.TrimSpace(title), strings.TrimSpace(body), nil
	}

	// External link: try fast HTTP extraction first, then fall back to browser.
	artCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := ExtractWebArticleDetails(artCtx, targetURL)
	if err == nil && !res.IsPDF {
		_, body := render.ExtractFrontMatter(res.Markdown)
		if utf8.RuneCountInString(strings.TrimSpace(body)) >= minArticleBodyRunes {
			return strings.TrimSpace(res.Title), strings.TrimSpace(body), nil
		}
	}
	if a.scraper != nil && a.scraper.wsManager != nil {
		md, title, berr := a.scraper.extractPageViaBrowser(artCtx, targetURL)
		if berr == nil {
			_, body := render.ExtractFrontMatter(md)
			if utf8.RuneCountInString(strings.TrimSpace(body)) >= minArticleBodyRunes {
				return strings.TrimSpace(title), strings.TrimSpace(body), nil
			}
		}
		if err == nil {
			err = berr
		}
	}
	if err != nil {
		return "", "", err
	}
	return "", "", fmt.Errorf("外链正文不足 %d 字", minArticleBodyRunes)
}

func (a *TwitterAdapter) fetchFromBrowser(ctx context.Context, src source.Source, cfg TwitterConfig) ([]twitter.Tweet, error) {
	if a.scraper == nil || a.scraper.wsManager == nil {
		return nil, fmt.Errorf("未连接浏览器扩展，无法抓取推特时间线")
	}

	opts := twitter.Options{
		Browser:      cfg.Browser,
		Limit:        cfg.MaxItems,
		ScrollRounds: cfg.ScrollRounds,
	}

	mode, query, kind := resolveTwitterSourceTarget(src.URL, cfg.Kind)
	switch mode {
	case "search":
		return twitter.Search(ctx, a.scraper.wsManager, a.scraper.logger, query, opts)
	case "likes":
		return twitter.Likes(ctx, a.scraper.wsManager, a.scraper.logger, opts)
	default:
		return twitter.Timeline(ctx, a.scraper.wsManager, a.scraper.logger, kind, opts)
	}
}

func (a *TwitterAdapter) archiveTweets(tweets []twitter.Tweet, archiveSrc string) {
	if a.scraper == nil || a.scraper.db == nil || len(tweets) == 0 {
		return
	}
	now := time.Now()
	records := make([]sqlite.TweetRecord, 0, len(tweets))
	for _, t := range tweets {
		if t.ID == "" {
			continue
		}
		records = append(records, sqlite.TweetRecord{
			ID:             t.ID,
			Text:           t.Text,
			Author:         t.Author,
			AuthorName:     t.AuthorName,
			TweetCreatedAt: t.CreatedAt,
			FavoriteCount:  t.FavoriteCount,
			RetweetCount:   t.RetweetCount,
			ReplyCount:     t.ReplyCount,
			QuoteCount:     t.QuoteCount,
			URL:            normalizeTweetURL(t),
			Media:          t.Media,
			Source:         archiveSrc,
			FetchedAt:      now,
		})
	}
	if len(records) == 0 {
		return
	}
	if err := a.scraper.db.SaveTweets(records); err != nil && a.scraper.logger != nil {
		a.scraper.logger.Warn("failed to archive tweets from subscription", zap.Error(err))
	}
}

func parseTwitterConfig(raw string) TwitterConfig {
	cfg := TwitterConfig{
		MaxItems:          defaultTwitterMaxItems,
		MaxArticleFetches: defaultTwitterMaxArticleFetches,
	}
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = defaultTwitterMaxItems
	}
	if cfg.MaxItems > 200 {
		cfg.MaxItems = 200
	}
	if cfg.MaxArticleFetches <= 0 {
		cfg.MaxArticleFetches = defaultTwitterMaxArticleFetches
	}
	if cfg.MaxArticleFetches > 30 {
		cfg.MaxArticleFetches = 30
	}
	return cfg
}

// resolveTwitterSourceTarget maps a user-entered X/Twitter URL into the
// browser adapter mode ("timeline", "search", or "likes").
func resolveTwitterSourceTarget(rawURL, defaultKind string) (mode, query, kind string) {
	kind = strings.TrimSpace(defaultKind)
	if kind == "" {
		kind = "for_you"
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "timeline", "", kind
	}
	path := strings.ToLower(strings.TrimRight(u.Path, "/"))
	switch {
	case path == "/search":
		if q := strings.TrimSpace(u.Query().Get("q")); q != "" {
			return "search", q, kind
		}
	case path == "/i/history/likes" || strings.HasSuffix(path, "/likes"):
		return "likes", "", kind
	}
	if qKind := strings.ToLower(strings.TrimSpace(u.Query().Get("kind"))); qKind == "following" || qKind == "latest" || qKind == "for_you" {
		kind = qKind
	}
	return "timeline", "", kind
}

func twitterArchiveSource(rawURL string) string {
	mode, _, _ := resolveTwitterSourceTarget(rawURL, "")
	return mode
}

func normalizeTweetURL(t twitter.Tweet) string {
	if u := strings.TrimSpace(t.URL); u != "" {
		return u
	}
	if t.Author != "" && t.ID != "" {
		return "https://x.com/" + t.Author + "/status/" + t.ID
	}
	if t.ID != "" {
		return "https://x.com/i/status/" + t.ID
	}
	return ""
}

func formatTweetTitle(t twitter.Tweet) string {
	snippet := strings.TrimSpace(t.ArticleTitle)
	if snippet == "" {
		snippet = strings.Join(strings.Fields(t.Text), " ")
		snippet = truncateSummary(snippet, 72)
	}

	authorLabel := ""
	switch {
	case t.AuthorName != "" && t.Author != "":
		authorLabel = fmt.Sprintf("%s (@%s)", t.AuthorName, t.Author)
	case t.Author != "":
		authorLabel = "@" + t.Author
	case t.AuthorName != "":
		authorLabel = t.AuthorName
	}

	switch {
	case authorLabel != "" && snippet != "":
		return authorLabel + "：" + snippet
	case snippet != "":
		return snippet
	case authorLabel != "":
		return authorLabel + " 的推文"
	default:
		return "X 推文"
	}
}

func formatTweetMarkdown(t twitter.Tweet, articleTitle, articleBody, articleURL string) string {
	var b strings.Builder

	if t.Author != "" {
		profileURL := "https://x.com/" + t.Author
		if t.AuthorName != "" {
			fmt.Fprintf(&b, "**%s** ([@%s](%s))\n\n", t.AuthorName, t.Author, profileURL)
		} else {
			fmt.Fprintf(&b, "[@%s](%s)\n\n", t.Author, profileURL)
		}
	} else if t.AuthorName != "" {
		fmt.Fprintf(&b, "**%s**\n\n", t.AuthorName)
	}

	if t.IsArticle && articleBody != "" {
		heading := firstNonEmpty(articleTitle, t.ArticleTitle)
		if heading != "" {
			fmt.Fprintf(&b, "## %s\n\n", heading)
		}
		b.WriteString(strings.TrimSpace(articleBody))
		b.WriteString("\n")
	} else {
		if text := strings.TrimSpace(t.Text); text != "" {
			b.WriteString(text)
			b.WriteString("\n")
		}
	}

	if len(t.Media) > 0 {
		b.WriteString("\n")
		for _, m := range t.Media {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			fmt.Fprintf(&b, "![](%s)\n\n", m)
		}
	}

	if !t.IsArticle && articleBody != "" {
		b.WriteString("\n---\n\n")
		if articleTitle != "" {
			fmt.Fprintf(&b, "### 附带文章：%s\n\n", articleTitle)
		} else {
			b.WriteString("### 附带文章全文\n\n")
		}
		if articleURL != "" {
			fmt.Fprintf(&b, "原文链接：%s\n\n", articleURL)
		}
		b.WriteString(strings.TrimSpace(articleBody))
		b.WriteString("\n")
	}

	var stats []string
	if t.ViewCount > 0 {
		stats = append(stats, fmt.Sprintf("阅读 %d", t.ViewCount))
	}
	if t.FavoriteCount > 0 {
		stats = append(stats, fmt.Sprintf("点赞 %d", t.FavoriteCount))
	}
	if t.RetweetCount > 0 {
		stats = append(stats, fmt.Sprintf("转发 %d", t.RetweetCount))
	}
	if t.ReplyCount > 0 {
		stats = append(stats, fmt.Sprintf("回复 %d", t.ReplyCount))
	}
	if t.QuoteCount > 0 {
		stats = append(stats, fmt.Sprintf("引用 %d", t.QuoteCount))
	}
	if len(stats) > 0 {
		b.WriteString("\n---\n\n")
		b.WriteString(strings.Join(stats, " · "))
	}

	return strings.TrimSpace(b.String())
}

func parseTweetTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RubyDate, time.RFC3339, time.RFC1123Z} {
		if ts, err := time.Parse(layout, raw); err == nil {
			utc := ts.UTC()
			return &utc
		}
	}
	return nil
}
