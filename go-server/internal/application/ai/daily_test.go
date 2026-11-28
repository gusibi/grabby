package ai

import (
	"fmt"
	"strings"
	"testing"

	"go-server/internal/infrastructure/sqlite"
)

func candidateItem(title, url, category string, score int) sqlite.ScrapedItemWithAI {
	item := sqlite.ScrapedItemWithAI{AICategory: category, QualityScore: score}
	item.Title = title
	item.URL = url
	return item
}

func TestSelectDailyCandidatesDedupesURLsAndTitles(t *testing.T) {
	items := []sqlite.ScrapedItemWithAI{
		candidateItem("OpenAI 发布新模型", "https://example.com/a", "AI", 9),
		candidateItem("OpenAI 发布新模型！", "https://example.com/b", "AI", 9),
		candidateItem("另一条新闻", "https://example.com/a", "科技", 8),
		candidateItem("真正不同的新闻", "https://example.com/c", "财经", 7),
	}

	candidates := selectDailyCandidates(items)
	if len(candidates) != 2 {
		t.Fatalf("candidates = %d, want 2: %+v", len(candidates), candidates)
	}
	if candidates[0].Title != "OpenAI 发布新模型" || candidates[1].Title != "真正不同的新闻" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
}

func TestSelectDailyCandidatesCapsTotal(t *testing.T) {
	items := make([]sqlite.ScrapedItemWithAI, 0, 80)
	for i := 0; i < 80; i++ {
		items = append(items, candidateItem(
			fmt.Sprintf("新闻 %d", i), fmt.Sprintf("https://example.com/%d", i), fmt.Sprintf("分类%d", i%8), 9))
	}

	candidates := selectDailyCandidates(items)
	if len(candidates) != dailyCandidateLimit {
		t.Fatalf("candidates = %d, want %d", len(candidates), dailyCandidateLimit)
	}
}

func TestSelectDailyCandidatesCapsPerCategory(t *testing.T) {
	items := make([]sqlite.ScrapedItemWithAI, 0, 60)
	for i := 0; i < 40; i++ {
		items = append(items, candidateItem(
			fmt.Sprintf("科技 %d", i), fmt.Sprintf("https://example.com/tech/%d", i), "科技", 9))
	}
	for i := 0; i < 8; i++ {
		items = append(items, candidateItem(
			fmt.Sprintf("财经 %d", i), fmt.Sprintf("https://example.com/fin/%d", i), "财经", 8))
	}

	candidates := selectDailyCandidates(items)
	if len(candidates) != 20 {
		t.Fatalf("candidates = %d, want 20; counts=%v", len(candidates), dailyCategoryCounts(candidates))
	}
	counts := dailyCategoryCounts(candidates)
	if counts["科技"] != dailyCandidatePerCategory {
		t.Fatalf("科技 candidates = %d, want %d", counts["科技"], dailyCandidatePerCategory)
	}
	if counts["财经"] != 8 {
		t.Fatalf("财经 candidates = %d, want 8", counts["财经"])
	}
}

func TestDefaultDailyPromptMatchesStoredDefault(t *testing.T) {
	if DefaultDailyPrompt != sqlite.DefaultDailyPrompt {
		t.Fatal("application and storage copies of the default daily prompt drifted apart")
	}
	if strings.Contains(DefaultDailyPrompt, "12 条到 20 条") {
		t.Fatal("default daily prompt still carries the old padding rule")
	}
	if !strings.Contains(DefaultDailyPrompt, "不能错过") || !strings.Contains(DefaultDailyPrompt, "上限 20 条") {
		t.Fatal("default daily prompt is missing the curation and hard-cap instructions")
	}
}
