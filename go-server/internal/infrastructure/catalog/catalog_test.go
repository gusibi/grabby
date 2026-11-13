package catalog

import "testing"

func TestNormalizeFiltersUnsupportedProvidersAndNonChatModels(t *testing.T) {
	raw := map[string]rawProvider{
		"openai": {
			ID:   "openai",
			Name: "OpenAI",
			API:  "https://api.openai.com/v1",
			NPM:  "@ai-sdk/openai",
			Env:  []string{"OPENAI_API_KEY"},
			Models: map[string]rawModel{
				"gpt-4o": {
					ID: "gpt-4o", Name: "GPT-4o",
					ToolCall: true, StructuredOutput: true, Attachment: true,
					Limit: rawLimit{Context: 128000, Output: 16384},
					Cost:  rawCost{Input: 2.5, Output: 10},
					Modalities: rawModalities{
						Input:  []string{"text", "image"},
						Output: []string{"text"},
					},
				},
				"text-embedding-3-large": {
					ID: "text-embedding-3-large", Name: "Embedding",
					Modalities: rawModalities{Input: []string{"text"}, Output: []string{"text"}},
				},
			},
		},
		"anthropic": {
			ID: "anthropic", Name: "Anthropic", API: "https://api.anthropic.com/v1", NPM: "@ai-sdk/anthropic",
			Models: map[string]rawModel{
				"claude-sonnet-4-5": {ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", ToolCall: true, StructuredOutput: true},
			},
		},
		"xai": {
			ID: "xai", Name: "xAI", NPM: "@ai-sdk/xai",
			Models: map[string]rawModel{
				"grok-4.6": {ID: "grok-4.6", Name: "Grok 4.6", ToolCall: true, StructuredOutput: true},
			},
		},
		"azure": {
			ID: "azure", Name: "Azure", API: "https://example.openai.azure.com", NPM: "@ai-sdk/azure",
			Models: map[string]rawModel{"gpt-4o": {ID: "gpt-4o", Name: "GPT-4o"}},
		},
		"google-vertex": {
			ID: "google-vertex", Name: "Vertex", API: "https://example.vertex", NPM: "@ai-sdk/google-vertex",
			Models: map[string]rawModel{"gemini-2.5-pro": {ID: "gemini-2.5-pro", Name: "Gemini"}},
		},
	}

	providers := normalize(raw)
	if len(providers) != 3 {
		t.Fatalf("normalize() returned %d providers, want 3 (openai + anthropic + xai)", len(providers))
	}
	byID := map[string]Provider{}
	for _, p := range providers {
		byID[p.ID] = p
	}

	openai := byID["openai"]
	if openai.Driver != "openai" {
		t.Errorf("openai driver = %q, want openai", openai.Driver)
	}
	if len(openai.Models) != 1 || openai.Models[0].ID != "gpt-4o" {
		t.Fatalf("openai models = %+v, want only gpt-4o", openai.Models)
	}
	if openai.Models[0].Context != 128000 || openai.Models[0].InputPrice != 2.5 {
		t.Errorf("openai model metadata not copied: %+v", openai.Models[0])
	}
	if !containsFold(openai.Models[0].Capabilities, "vision") || !containsFold(openai.Models[0].Capabilities, "structured") {
		t.Errorf("openai capabilities = %v, want vision + structured", openai.Models[0].Capabilities)
	}

	anthropic := byID["anthropic"]
	if anthropic.Driver != "anthropic" {
		t.Errorf("anthropic driver = %q, want anthropic", anthropic.Driver)
	}

	xai := byID["xai"]
	if xai.API != "https://api.x.ai/v1" {
		t.Errorf("xai api = %q, want default API fallback", xai.API)
	}
	if xai.Driver != "custom" {
		t.Errorf("xai driver = %q, want custom", xai.Driver)
	}
}

func TestInferDriverRejectsUnsupportedProtocols(t *testing.T) {
	cases := []struct {
		id, npm string
		want    string
		ok      bool
	}{
		{"openai", "@ai-sdk/openai", "openai", true},
		{"anthropic", "@ai-sdk/anthropic", "anthropic", true},
		{"google", "@ai-sdk/google", "gemini", true},
		{"ollama", "@ai-sdk/openai-compatible", "ollama", true},
		{"deepseek", "@ai-sdk/openai-compatible", "custom", true},
		{"openrouter", "@openrouter/ai-sdk-provider", "custom", true},
		{"azure", "@ai-sdk/azure", "", false},
		{"google-vertex", "@ai-sdk/google-vertex", "", false},
		{"amazon-bedrock", "@ai-sdk/amazon-bedrock", "", false},
	}
	for _, tc := range cases {
		got, ok := inferDriver(tc.id, tc.npm)
		if ok != tc.ok || got != tc.want {
			t.Errorf("inferDriver(%q, %q) = (%q, %v), want (%q, %v)", tc.id, tc.npm, got, ok, tc.want, tc.ok)
		}
	}
}
