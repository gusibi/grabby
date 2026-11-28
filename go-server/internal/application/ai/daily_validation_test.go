package ai

import (
	"encoding/json"
	"errors"
	"fmt"
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

// reportFixture builds a valid report payload with the given body counts and
// scores. The dashboard always carries two stat rows, which must not count
// against the body budget.
func reportFixture(headline, tech, finance, dashboard int) string {
	section := func(name string, count int, score string) string {
		items := make([]string, 0, count)
		for i := 0; i < count; i++ {
			items = append(items, fmt.Sprintf(
				`{"title":"%s-%d","summary":"摘要","score":"%s"}`, name, i, score))
		}
		return fmt.Sprintf(`"%s":{"title":"%s","items":[%s]}`, name, name, strings.Join(items, ","))
	}
	return fmt.Sprintf(`{"title":"日报","date":"2026-06-12","editor":"主编","sections":{%s,%s,%s,%s}}`,
		section("headline", headline, "9/10"),
		section("tech_ai", tech, "8/10"),
		section("finance", finance, "7/10"),
		section("dashboard", dashboard, "0"))
}

func TestNormalizeDailyReportContentRejectsOverBudget(t *testing.T) {
	raw := reportFixture(3, 10, 10, 2) // 23 body items, dashboard excluded

	_, err := normalizeDailyReportContent(raw, "2026-06-12")
	if !errors.Is(err, errDailyReportOverBudget) {
		t.Fatalf("expected over-budget error, got %v", err)
	}
}

func TestNormalizeDailyReportContentOverBudgetTrimsLowestScores(t *testing.T) {
	raw := reportFixture(5, 12, 10, 2) // 27 body items

	normalized, err := normalizeDailyReportContentOverBudget(raw, "2026-06-12")
	if err != nil {
		t.Fatalf("normalizeDailyReportContentOverBudget returned error: %v", err)
	}

	var payload dailyReportPayload
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		t.Fatalf("trimmed output is not valid JSON: %v\n%s", err, normalized)
	}
	if got := countDailyReportItems(payload.Sections); got != maxDailyReportItems {
		t.Fatalf("body items = %d, want %d", got, maxDailyReportItems)
	}
	if got := len(payload.Sections["headline"].Items); got != 5 {
		t.Fatalf("headline items = %d, want 5 (headline is protected)", got)
	}
	if got := len(payload.Sections["dashboard"].Items); got != 2 {
		t.Fatalf("dashboard items = %d, want 2 (never trimmed)", got)
	}
	// Only the lowest-scored section is cut: 12 tech + 5 headline + 3 finance.
	if got := len(payload.Sections["tech_ai"].Items); got != 12 {
		t.Fatalf("tech_ai items = %d, want 12", got)
	}
	if got := len(payload.Sections["finance"].Items); got != 3 {
		t.Fatalf("finance items = %d, want 3", got)
	}
}

func TestParseReportScore(t *testing.T) {
	cases := map[string]int{
		"9/10":  9,
		"8":     8,
		"评分 7":  7,
		"":      0,
		"high":  0,
		"10/10": 10,
	}
	for raw, want := range cases {
		if got := parseReportScore(raw); got != want {
			t.Errorf("parseReportScore(%q) = %d, want %d", raw, got, want)
		}
	}
}
