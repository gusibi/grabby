package main

import (
	"net/url"
	"strings"
)

// Classifier handles category classification and origin source extraction.
type Classifier struct{}

func NewClassifier() *Classifier {
	return &Classifier{}
}

// Classify determines the category of an item based on its URL and the source's default category.
func (c *Classifier) Classify(itemURL string, defaultCategory string) string {
	if defaultCategory != "" && defaultCategory != "auto" {
		return defaultCategory
	}

	lowerURL := strings.ToLower(itemURL)

	if strings.Contains(lowerURL, "x.com") || strings.Contains(lowerURL, "twitter.com") {
		return "tweet"
	}
	if strings.Contains(lowerURL, "github.com") || strings.Contains(lowerURL, "gitlab.com") {
		return "project"
	}
	if strings.Contains(lowerURL, "arxiv.org") || strings.Contains(lowerURL, "biorxiv.org") || strings.HasSuffix(lowerURL, ".pdf") {
		return "paper"
	}
	if strings.Contains(lowerURL, "mp.weixin.qq.com") {
		return "article"
	}

	return "article"
}

// ExtractOrigin extracts the original publisher name from the URL or an aggregator source field.
func (c *Classifier) ExtractOrigin(itemURL string, aggregatorSource string) string {
	aggregatorSource = strings.TrimSpace(aggregatorSource)
	if aggregatorSource != "" {
		// Clean and map aggregator source field
		// Examples:
		// "X：宝玉 (@dotey)" -> "X (Twitter)"
		// "公众号：XXX" -> "微信公众号"
		lowerSrc := strings.ToLower(aggregatorSource)
		if strings.HasPrefix(lowerSrc, "x：") || strings.HasPrefix(lowerSrc, "x:") || strings.Contains(lowerSrc, "twitter") || strings.Contains(lowerSrc, "推特") {
			return "X (Twitter)"
		}
		if strings.HasPrefix(lowerSrc, "公众号：") || strings.HasPrefix(lowerSrc, "公众号:") || strings.Contains(lowerSrc, "微信") {
			return "微信公众号"
		}
		return aggregatorSource
	}

	// Direct parsing from domain
	u, err := url.Parse(itemURL)
	if err != nil || u.Host == "" {
		return "Unknown"
	}

	host := strings.TrimPrefix(u.Host, "www.")
	return host
}
