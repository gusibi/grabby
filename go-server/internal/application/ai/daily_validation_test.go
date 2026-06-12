package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeDailyReportContentAcceptsTrailingGarbage(t *testing.T) {
	raw := `{
	  "title": "日报",
	  "date": "2026-06-11",
	  "sections": {
	    "headline": {
	      "title": "今日头条",
	      "items": [
	        {
	          "title": "AI时代的\"光进铜退\"",
	          "summary": "光互连成为算力集群的重要基础设施。",
	          "source": "钛媒体",
	          "link": "https://example.com",
	          "rating": "8/10",
	          "comment": "有行业洞察。"
	        }
	      ]
	    }
	  }
	}}`

	normalized, err := normalizeDailyReportContent(raw, "2026-06-11")
	if err != nil {
		t.Fatalf("normalizeDailyReportContent returned error: %v", err)
	}
	if strings.Contains(normalized, "}}}") {
		t.Fatalf("normalized JSON kept trailing garbage: %s", normalized)
	}

	var payload dailyReportPayload
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		t.Fatalf("normalized output is not valid JSON: %v\n%s", err, normalized)
	}
	item := payload.Sections["headline"].Items[0]
	if item.Score != "8/10" {
		t.Fatalf("expected rating to normalize to score, got %+v", item)
	}
}

func TestNormalizeDailyReportContentAcceptsSectionArray(t *testing.T) {
	raw := `{
	  "title": "日报",
	  "date": "2026-06-09",
	  "sections": [
	    {
	      "heading": "今日头条",
	      "items": [
	        {
	          "title": "重大新闻",
	          "summary": "这是一条摘要。",
	          "score": 9
	        }
	      ]
	    }
	  ]
	}`

	normalized, err := normalizeDailyReportContent(raw, "2026-06-09")
	if err != nil {
		t.Fatalf("normalizeDailyReportContent returned error: %v", err)
	}

	var payload dailyReportPayload
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		t.Fatalf("normalized output is not valid JSON: %v\n%s", err, normalized)
	}
	section := payload.Sections["section_0"]
	if section.Title != "今日头条" || len(section.Items) != 1 || section.Items[0].Score != "9" {
		t.Fatalf("unexpected normalized section: %+v", section)
	}
}

func TestNormalizeDailyReportContentRejectsMarkdownWrapper(t *testing.T) {
	raw := `{"title":"日报","content":"# Markdown 日报"}`

	_, err := normalizeDailyReportContent(raw, "2026-06-12")
	if err == nil {
		t.Fatal("expected error for report without sections")
	}
}
