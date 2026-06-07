package main

import (
	"testing"
)

func TestClassifier_Classify(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		url             string
		defaultCategory string
		expected        string
	}{
		{"https://x.com/dotey/status/123", "auto", "tweet"},
		{"https://twitter.com/dotey/status/123", "auto", "tweet"},
		{"https://github.com/google/gemini", "auto", "project"},
		{"https://gitlab.com/some/repo", "auto", "project"},
		{"https://arxiv.org/abs/2401.12345", "auto", "paper"},
		{"https://example.com/paper.pdf", "auto", "paper"},
		{"https://mp.weixin.qq.com/s/xyz", "auto", "article"},
		{"https://example.com/blog/post", "auto", "article"},
		{"https://x.com/dotey/status/123", "article", "article"},
		{"https://github.com/google/gemini", "paper", "paper"},
	}

	for _, tt := range tests {
		result := c.Classify(tt.url, tt.defaultCategory)
		if result != tt.expected {
			t.Errorf("Classify(%q, %q) = %q; want %q", tt.url, tt.defaultCategory, result, tt.expected)
		}
	}
}

func TestClassifier_ExtractOrigin(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		url              string
		aggregatorSource string
		expected         string
	}{
		{"https://x.com/status/123", "X：宝玉 (@dotey)", "X (Twitter)"},
		{"https://twitter.com/status/123", "Twitter: info", "X (Twitter)"},
		{"https://mp.weixin.qq.com/s/xyz", "公众号：新智元", "微信公众号"},
		{"https://openai.com/blog/gemini", "", "openai.com"},
		{"https://www.nature.com/articles/123", "", "nature.com"},
		{"invalid-url", "", "Unknown"},
	}

	for _, tt := range tests {
		result := c.ExtractOrigin(tt.url, tt.aggregatorSource)
		if result != tt.expected {
			t.Errorf("ExtractOrigin(%q, %q) = %q; want %q", tt.url, tt.aggregatorSource, result, tt.expected)
		}
	}
}
