package sqlite

import "testing"

func TestLoadAISettingsUsesNormalizedTables(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	profiles := []AIProviderProfile{{
		ID:                "p1",
		Name:              "DeepSeek",
		Provider:          "custom",
		APIKey:            "sk-test",
		BaseURL:           "https://api.deepseek.com/v1",
		Model:             "deepseek-chat",
		Models:            []string{"deepseek-chat", "deepseek-reasoner"},
		Priority:          1,
		RequestsPerMinute: 30,
	}}
	if err := db.ReplaceAIConnections(profiles); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAIRoute("failover", profiles); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSetting("ai_active_profile_id", "p1"); err != nil {
		t.Fatal(err)
	}

	loaded, err := db.LoadAISettings(AISettings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("profiles = %+v", loaded.Profiles)
	}
	profile := loaded.Profiles[0]
	if profile.ID != "p1" || profile.Provider != "custom" || len(profile.Models) != 2 || profile.Models[1] != "custom/deepseek-reasoner" {
		t.Errorf("profile = %+v", profile)
	}
	if loaded.Strategy != "failover" {
		t.Errorf("strategy = %q, want failover", loaded.Strategy)
	}
	if loaded.ActiveProfileID != "p1" {
		t.Errorf("active profile = %q", loaded.ActiveProfileID)
	}
	if loaded.Provider != "custom" || loaded.Model != "custom/deepseek-chat" || loaded.APIKey != "sk-test" {
		t.Errorf("top-level mirror = %+v", loaded)
	}
}
