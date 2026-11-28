package nativeui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// pickerFixture returns an app whose active profile points at the anthropic
// catalog provider, so the model tab can offer checkboxes without a network
// round trip.
func pickerFixture() *App {
	a := fixtureApp()
	a.settingsTab = "models"
	a.aiCatalog = []CatalogProvider{{
		ID: "anthropic", Name: "Anthropic", Driver: "anthropic",
		Models: []CatalogModel{
			{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Context: 200000},
			{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Context: 200000},
			{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Context: 200000},
		},
	}}
	return a
}

func TestModelPickerOffersCatalogCheckboxes(t *testing.T) {
	a := pickerFixture()
	a.view = "settings"
	tester := ui.NewTester(a.View, 1280, 1000)
	tester.Frame()
	for _, want := range []string{"首选模型", "claude-sonnet-5-5", "claude-haiku-5-5", "已选 1 个"} {
		if !tester.HasText(want) {
			t.Fatalf("model tab misses %q", want)
		}
	}
}

func TestToggleProfileModelKeepsPreferredModel(t *testing.T) {
	p := ProviderProfile{Model: "claude-haiku-5-5", Models: []string{"claude-sonnet-5-5", "claude-haiku-5-5"}}
	a := New(NewClient("http://127.0.0.1:1"))

	// Adding another model leaves the preferred one alone.
	a.toggleProfileModel(&p, "claude-opus-5-5", true)
	if len(p.Models) != 3 || p.Model != "claude-haiku-5-5" {
		t.Fatalf("models = %v, preferred = %q", p.Models, p.Model)
	}

	// Removing the preferred model falls back to the first remaining one.
	a.toggleProfileModel(&p, "claude-haiku-5-5", false)
	if len(p.Models) != 2 || p.Model != "claude-sonnet-5-5" {
		t.Fatalf("models = %v, preferred = %q", p.Models, p.Model)
	}

	// Namespaced and bare ids are the same choice.
	a.toggleProfileModel(&p, "anthropic/claude-sonnet-5-5", false)
	if len(p.Models) != 1 || p.Models[0] != "claude-opus-5-5" || p.Model != "claude-opus-5-5" {
		t.Fatalf("models = %v, preferred = %q", p.Models, p.Model)
	}

	a.toggleProfileModel(&p, "claude-opus-5-5", false)
	if len(p.Models) != 0 || p.Model != "" {
		t.Fatalf("models = %v, preferred = %q", p.Models, p.Model)
	}
}

func TestManualModelEntryAddsModel(t *testing.T) {
	a := pickerFixture()
	a.view = "settings"
	a.aiManualOpen = true
	a.aiManualModel = "my-relay-model"
	tester := ui.NewTester(a.View, 1280, 1000)
	tester.Frame()
	if !tester.HasText("手动填写模型 ID") {
		t.Fatal("manual entry is not rendered")
	}

	p := a.currentProfile()
	a.toggleProfileModel(p, a.aiManualModel, true)
	if !hasProfileModel(p.Models, "my-relay-model") {
		t.Fatalf("models = %v, want the manual id", p.Models)
	}
}

func TestCatalogProviderMatchesCustomBaseURL(t *testing.T) {
	a := pickerFixture()
	a.aiCatalog = append(a.aiCatalog, CatalogProvider{
		ID: "deepseek", Name: "DeepSeek", API: "https://api.deepseek.com/v1", Driver: "custom",
		Models: []CatalogModel{{ID: "deepseek-chat"}},
	})
	p := &ProviderProfile{Provider: "custom", BaseURL: "https://api.deepseek.com/v1/"}
	cp := a.catalogProviderFor(p)
	if cp == nil || cp.ID != "deepseek" {
		t.Fatalf("catalog provider = %+v, want deepseek", cp)
	}
}
