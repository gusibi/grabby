package ai

import (
	"reflect"
	"testing"
)

func TestNormalizeModelListTrimsDedupesAndNamespaces(t *testing.T) {
	got := NormalizeModelList("custom", []string{" m1 ", "m1", "", "m2", "custom/m3"})
	want := []string{"custom/m1", "custom/m2", "custom/m3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeModelList() = %v, want %v", got, want)
	}
}

func TestNormalizeModelListKeepsProviderNamespace(t *testing.T) {
	got := NormalizeModelList("gemini", []string{"gemini-2.0-flash", "gemini-2.5-flash"})
	want := []string{"gemini-2.0-flash", "gemini-2.5-flash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeModelList() = %v, want %v", got, want)
	}
}

func TestDriverModelRefAddsAndReplacesNamespaces(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     string
	}{
		{"anthropic", "claude-sonnet-4-5", "anthropic/claude-sonnet-4-5"},
		{"anthropic", "anthropic/claude-sonnet-4-5", "anthropic/claude-sonnet-4-5"},
		{"ollama", "qwen3", "ollama/qwen3"},
		{"gemini", "gemini-2.5-flash", "googleai/gemini-2.5-flash"},
		{"openai", "gpt-4o", "openai/gpt-4o"},
		{"custom", "deepseek-chat", "custom/deepseek-chat"},
		{"custom", "anthropic/claude-3.5-sonnet", "custom/anthropic/claude-3.5-sonnet"},
		{"lmstudio", "local-model", "local-model"},
		{"anthropic", "openai/gpt-4o", "anthropic/gpt-4o"},
	}
	for _, tc := range cases {
		if got := DriverModelRef(tc.provider, tc.model); got != tc.want {
			t.Errorf("DriverModelRef(%q, %q) = %q, want %q", tc.provider, tc.model, got, tc.want)
		}
	}
}

func TestNormalizeAISettingsMigratesLegacyModel(t *testing.T) {
	settings := NormalizeAISettings(AISettings{
		Profiles: []AIProviderProfile{
			{ID: "p1", Provider: "gemini", Model: "gemini-2.0-flash"},
		},
	})

	p := settings.Profiles[0]
	if !reflect.DeepEqual(p.Models, []string{"gemini-2.0-flash"}) {
		t.Fatalf("Models = %v, want [gemini-2.0-flash]", p.Models)
	}
	if p.Model != "gemini-2.0-flash" {
		t.Fatalf("Model = %q, want first model", p.Model)
	}
	if settings.Model != "gemini-2.0-flash" {
		t.Fatalf("top-level Model = %q", settings.Model)
	}
	if !reflect.DeepEqual(settings.Models, []string{"gemini-2.0-flash"}) {
		t.Fatalf("top-level Models = %v", settings.Models)
	}
}

func TestNormalizeAISettingsKeepsAllModelsAndMirrorsFirst(t *testing.T) {
	settings := NormalizeAISettings(AISettings{
		Profiles: []AIProviderProfile{
			{ID: "p1", Provider: "openai", Models: []string{"gpt-4o-mini", "gpt-4o", "gpt-4o-mini"}},
		},
	})

	p := settings.Profiles[0]
	if !reflect.DeepEqual(p.Models, []string{"gpt-4o-mini", "gpt-4o"}) {
		t.Fatalf("Models = %v", p.Models)
	}
	if p.Model != "gpt-4o-mini" {
		t.Fatalf("Model = %q, want first model", p.Model)
	}
}

func TestEffectiveModelsFallsBackToModel(t *testing.T) {
	p := AIProviderProfile{Model: "legacy-model"}
	if got := p.EffectiveModels(); !reflect.DeepEqual(got, []string{"legacy-model"}) {
		t.Fatalf("EffectiveModels() = %v", got)
	}

	p = AIProviderProfile{Model: "ignored", Models: []string{"a", "b"}}
	if got := p.EffectiveModels(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("EffectiveModels() = %v", got)
	}
}
