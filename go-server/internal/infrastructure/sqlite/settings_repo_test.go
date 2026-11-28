package sqlite

import "testing"

func TestLoadAISettingsUpgradesLegacyDailyPrompt(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// The prompt earlier installs actually stored: Markdown output, no budget,
	// every qualified candidate listed.
	legacy := "你是 Grabby AI 日报主编。请基于以下整理的今日高分优质资讯，撰写一份排版精美、结构清晰的 Markdown 格式日报。\n" +
		"- 📰 今日头条 / 要闻：挑选 2-3 篇最重磅（评分最高）的内容进行详细介绍和专业点评。\n" +
		"- 🔬 前沿探索 (科技与 AI)：汇总所有科技、AI 相关的资讯。"
	if err := db.SaveSetting("ai_daily_prompt", legacy); err != nil {
		t.Fatalf("save legacy prompt: %v", err)
	}

	settings, err := db.LoadAISettings(AISettings{})
	if err != nil {
		t.Fatalf("LoadAISettings: %v", err)
	}
	if settings.DailyPrompt != DefaultDailyPrompt {
		t.Fatalf("legacy prompt was not upgraded:\n%s", settings.DailyPrompt)
	}

	version, err := db.GetSetting(dailyPromptVersionKey, "")
	if err != nil {
		t.Fatalf("read prompt version: %v", err)
	}
	if version != dailyPromptVersion {
		t.Fatalf("prompt version = %q, want %q", version, dailyPromptVersion)
	}
}

func TestLoadAISettingsPreservesCustomDailyPrompt(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	custom := "你是我的日报编辑，只关注半导体和货币政策，其他一律不要。"
	if err := db.SaveSetting("ai_daily_prompt", custom); err != nil {
		t.Fatalf("save custom prompt: %v", err)
	}

	settings, err := db.LoadAISettings(AISettings{})
	if err != nil {
		t.Fatalf("LoadAISettings: %v", err)
	}
	if settings.DailyPrompt != custom {
		t.Fatalf("custom prompt was overwritten:\n%s", settings.DailyPrompt)
	}
}
