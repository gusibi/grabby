package sqlite

import (
	"testing"
	"time"
)

func TestAIConnectionsRoundTrip(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	profiles := []AIProviderProfile{
		{
			ID: "p1", Name: "DeepSeek", Provider: "custom",
			APIKey: "sk-1", BaseURL: "https://api.deepseek.com/v1",
			Model: "deepseek-chat", Models: []string{"deepseek-chat", "deepseek-reasoner"},
			Priority: 1, RequestsPerMinute: 20,
		},
		{
			ID: "p2", Name: "Claude", Provider: "anthropic",
			APIKey: "sk-2", Model: "claude-sonnet-5-5",
			Models: []string{"claude-sonnet-5-5"}, Priority: 2, RequestsPerMinute: 10,
			Disabled: true,
		},
	}
	if err := db.ReplaceAIConnections(profiles); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.LoadAIConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded[0].ID != "p1" || len(loaded[0].Models) != 2 || loaded[0].Models[1] != "deepseek-reasoner" {
		t.Errorf("p1 = %+v", loaded[0])
	}
	if !loaded[1].Disabled || loaded[1].Provider != "anthropic" {
		t.Errorf("p2 = %+v", loaded[1])
	}

	// Removing a model and a profile must delete the stale rows.
	if err := db.ReplaceAIConnections([]AIProviderProfile{{
		ID: "p1", Name: "DeepSeek", Provider: "custom",
		APIKey: "sk-1", BaseURL: "https://api.deepseek.com/v1",
		Models: []string{"deepseek-chat"}, Priority: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadAIConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Models) != 1 {
		t.Fatalf("loaded after prune = %+v", loaded)
	}
}

func TestAIRouteAndUsage(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SaveAIRoute("failover", []AIProviderProfile{{ID: "p1", Priority: 1}}); err != nil {
		t.Fatal(err)
	}
	strategy, err := db.LoadAIRoute()
	if err != nil {
		t.Fatal(err)
	}
	if strategy != "failover" {
		t.Errorf("strategy = %q", strategy)
	}

	now := time.Now().UTC()
	if err := db.RecordAIUsage(AIUsageRecord{
		ConnectionID: "p1", ModelID: "deepseek-chat", Task: "analysis",
		PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140,
		LatencyMS: 1200, Status: "success", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordAIUsage(AIUsageRecord{
		ConnectionID: "p1", ModelID: "deepseek-chat", Task: "analysis",
		LatencyMS: 300, Status: "failed", Error: "429", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := db.GetAIUsageSummary(7)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 2 || summary.FailedRequests != 1 {
		t.Errorf("summary = %+v", summary)
	}
	if summary.PromptTokens != 100 || summary.CompletionTokens != 40 || summary.TotalTokens != 140 {
		t.Errorf("token totals = %+v", summary)
	}
	if len(summary.ByModel) != 1 || summary.ByModel[0].Requests != 2 {
		t.Errorf("by model = %+v", summary.ByModel)
	}
}
