package backup

import (
	"encoding/json"
	"strings"
	"testing"

	"go-server/internal/domain/ai"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

type fakeReloader struct {
	got ai.AISettings
}

func (f *fakeReloader) ReloadSettings(settings ai.AISettings) error {
	f.got = settings
	return nil
}

func newTestDB(t *testing.T) *sqlite.Database {
	t.Helper()
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestBuildEncodeParseRoundTrip(t *testing.T) {
	settings := ai.AISettings{
		Enabled:  true,
		Provider: "custom",
		APIKey:   "sk-secret",
		Model:    "custom/deepseek-chat",
		Models:   []string{"custom/deepseek-chat"},
		BaseURL:  "https://api.example.com/v1",
		Profiles: []ai.AIProviderProfile{{
			ID:       "p1",
			Name:     "Example",
			Provider: "custom",
			APIKey:   "sk-secret",
			Models:   []string{"custom/deepseek-chat"},
			BaseURL:  "https://api.example.com/v1",
		}},
		ActiveProfileID:      "p1",
		QualityThreshold:     7,
		MorningReportEnabled: true,
		MorningReportTime:    "08:30",
		EveningReportTime:    "22:30",
	}
	sources := []source.Source{
		{ID: "b-source", Name: "B", Type: "rss", URL: "https://b.example/feed", Schedule: "0 */2 * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}", Category: "AI"},
		{ID: "a-source", Name: "A", Type: "rss", URL: "https://a.example/feed", Schedule: "0 * * * *", Enabled: 0, DefaultCategory: "auto", Config: `{"x":1}`, Category: "Tech"},
	}

	raw, err := Encode(Build(settings, sources))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.App != AppName || doc.Version != Version {
		t.Fatalf("unexpected header: %+v", doc)
	}
	if doc.AI == nil || doc.AI.APIKey != "sk-secret" {
		t.Fatalf("AI settings were not round-tripped: %+v", doc.AI)
	}
	if len(doc.Sources) != 2 || doc.Sources[0].ID != "a-source" {
		t.Fatalf("sources were not sorted/exposed: %+v", doc.Sources)
	}
	if doc.Sources[0].Enabled != 0 || doc.Sources[1].Enabled != 1 {
		t.Fatalf("source enabled flags changed: %+v", doc.Sources)
	}
}

func TestParseFillsDefaultsAndRejectsBadInput(t *testing.T) {
	doc, err := Parse([]byte(`{"version":1,"sources":[{"id":" plain ","url":"https://example.com/feed"}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := doc.Sources[0]
	if got.ID != "plain" || got.Name != "plain" || got.Type != "rss" ||
		got.Schedule == "" || got.DefaultCategory != "auto" || got.Category != "General" || got.Config != "{}" {
		t.Fatalf("defaults not applied: %+v", got)
	}

	if _, err := Parse([]byte(`{"version":99,"ai":{"enabled":true}}`)); err == nil {
		t.Fatal("expected a newer-version document to be rejected")
	}
	if _, err := Parse([]byte(`{"version":1,"sources":[{"id":"x","url":"https://example.com","config":"{broken"}]}`)); err == nil {
		t.Fatal("expected invalid source JSON to be rejected")
	}
	if _, err := Parse([]byte(`{"version":1,"sources":[]}`)); err == nil {
		t.Fatal("expected an empty document to be rejected")
	}
	if _, err := Parse([]byte(`{"version":1,"app":"other","sources":[{"id":"x","url":"https://example.com"}]}`)); err == nil {
		t.Fatal("expected a foreign app document to be rejected")
	}
}

func TestApplyRestoresAIAndUpsertsSources(t *testing.T) {
	db := newTestDB(t)
	reloader := &fakeReloader{}

	if err := db.InsertSource(sqlite.Source{
		ID: "keep", Name: "Keep", Type: "rss", URL: "https://keep.example/feed",
		Schedule: "0 */2 * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}", Category: "General",
	}); err != nil {
		t.Fatalf("InsertSource keep: %v", err)
	}
	if err := db.InsertSource(sqlite.Source{
		ID: "edit", Name: "Old name", Type: "rss", URL: "https://old.example/feed",
		Schedule: "0 */2 * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}", Category: "General",
	}); err != nil {
		t.Fatalf("InsertSource edit: %v", err)
	}

	baseline, err := db.GetSources()
	if err != nil {
		t.Fatalf("GetSources: %v", err)
	}

	enabled := 1
	doc := Document{
		Version: Version,
		App:     AppName,
		AI: &ai.AISettings{
			Enabled:          true,
			Provider:         "openai",
			APIKey:           "sk-restored",
			Model:            "openai/gpt-4o-mini",
			Models:           []string{"openai/gpt-4o-mini"},
			QualityThreshold: 8,
			Profiles: []ai.AIProviderProfile{{
				ID: "restored", Name: "Restored", Provider: "openai", APIKey: "sk-restored", Models: []string{"openai/gpt-4o-mini"},
			}},
			ActiveProfileID: "restored",
			Strategy:        "failover",
		},
		Sources: []SourceConfig{
			{ID: "edit", Name: "New name", Type: "rss", URL: "https://new.example/feed", Schedule: "*/30 * * * *", Enabled: enabled, DefaultCategory: "article", Config: `{"page":2}`, Category: "Tech"},
			{ID: "added", Name: "Added", Type: "rss", URL: "https://added.example/feed", Schedule: "0 * * * *", Enabled: 0, DefaultCategory: "auto", Config: "{}", Category: "News"},
		},
	}

	result, err := Apply(db, reloader, doc, Options{ImportAI: true, ImportSources: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.AIImported || result.SourcesCreated != 1 || result.SourcesUpdated != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if reloader.got.Provider != "openai" || reloader.got.Strategy != "failover" {
		t.Fatalf("reloader did not receive the restored settings: %+v", reloader.got)
	}

	sources, err := db.GetSources()
	if err != nil {
		t.Fatalf("GetSources: %v", err)
	}
	if len(sources) != len(baseline)+1 {
		t.Fatalf("expected %d sources, got %d", len(baseline)+1, len(sources))
	}
	byID := make(map[string]sqlite.Source, len(sources))
	for _, src := range sources {
		byID[src.ID] = src
	}
	if got := byID["keep"]; got.Name != "Keep" {
		t.Fatalf("unrelated source was modified: %+v", got)
	}
	if got := byID["edit"]; got.Name != "New name" || got.URL != "https://new.example/feed" ||
		got.Schedule != "*/30 * * * *" || got.Category != "Tech" || got.Config != `{"page":2}` {
		t.Fatalf("existing source was not upserted: %+v", got)
	}
	if got := byID["added"]; got.Enabled != 0 || got.Name != "Added" {
		t.Fatalf("new source was not inserted as configured: %+v", got)
	}

	provider, err := db.GetSetting("ai_provider", "")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if provider != "openai" {
		t.Fatalf("AI provider not persisted, got %q", provider)
	}
	threshold, err := db.GetSetting("ai_quality_threshold", "")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if threshold != "8" {
		t.Fatalf("quality threshold not persisted, got %q", threshold)
	}
	var profiles []ai.AIProviderProfile
	profilesRaw, err := db.GetSetting("ai_provider_profiles", "")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if err := json.Unmarshal([]byte(profilesRaw), &profiles); err != nil {
		t.Fatalf("decode profiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != "restored" || profiles[0].APIKey != "sk-restored" {
		t.Fatalf("profiles not persisted: %+v", profiles)
	}
}

func TestApplyWithoutSectionsFails(t *testing.T) {
	db := newTestDB(t)
	if _, err := Apply(db, nil, Document{}, Options{}); err == nil || !strings.Contains(err.Error(), "没有选择") {
		t.Fatalf("expected a section-selection error, got %v", err)
	}
}
