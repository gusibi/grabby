package modeldiscovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverOpenAICompatible(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "gpt-4o", "owned_by": "openai"},
				{"id": "gpt-4o-mini", "owned_by": "openai"},
			},
		})
	}))
	defer srv.Close()

	models, err := Discover(context.Background(), "custom", srv.URL, "sk-test")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(models) != 2 || models[0].ID != "gpt-4o" {
		t.Fatalf("models = %+v", models)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestDiscoverAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "claude-sonnet-5-5", "display_name": "Claude Sonnet 5.5"},
			},
			"has_more": false,
		})
	}))
	defer srv.Close()

	models, err := Discover(context.Background(), "anthropic", srv.URL, "sk-ant")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(models) != 1 || models[0].ID != "claude-sonnet-5-5" || models[0].Name != "Claude Sonnet 5.5" {
		t.Fatalf("models = %+v", models)
	}
}

func TestDiscoverGeminiFiltersNonChatModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "AIza-test" {
			t.Errorf("x-goog-api-key = %q", r.Header.Get("x-goog-api-key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "models/gemini-2.5-flash", "displayName": "Gemini 2.5 Flash", "supportedGenerationMethods": []string{"generateContent"}},
				{"name": "models/text-embedding-004", "supportedGenerationMethods": []string{"embedContent"}},
			},
		})
	}))
	defer srv.Close()

	models, err := Discover(context.Background(), "gemini", srv.URL+"/v1beta", "AIza-test")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("models = %+v", models)
	}
}

func TestDiscoverOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/tags") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "qwen3:latest", "model": "qwen3:latest"},
			},
		})
	}))
	defer srv.Close()

	models, err := Discover(context.Background(), "ollama", srv.URL+"/v1", "")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(models) != 1 || models[0].ID != "qwen3:latest" {
		t.Fatalf("models = %+v", models)
	}
}

func TestDiscoverRejectsUnknownDriver(t *testing.T) {
	if _, err := Discover(context.Background(), "bedrock", "", "key"); err == nil {
		t.Fatal("expected an error for an unsupported driver")
	}
}
