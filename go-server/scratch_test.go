package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai"
)

type ScratchAIAnalysisResult struct {
	Category     string   `json:"category"`
	Subcategory  string   `json:"subcategory"`
	QualityScore int      `json:"quality_score"`
	Summary      string   `json:"summary"`
	Comment      string   `json:"comment"`
	Tags         []string `json:"tags"`
}

func TestScratch(t *testing.T) {
	ctx := context.Background()

	// 1. Start a mock server to intercept the HTTP request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Printf("Mock Server Received Body:\n%s\n", string(body))

		// Respond with dummy OpenAI-compatible chat completion
		response := map[string]any{
			"id":     "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677858242,
			"model":  "google/gemma-4-12b",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"category":"科技","subcategory":"AI","quality_score":9,"summary":"test","comment":"test","tags":["test"]}`,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	// 2. Initialize compat_oai with the mock server URL
	customPlugin := &oai.OpenAICompatible{
		Provider: "custom",
		APIKey:   "dummy_key",
		BaseURL:  ts.URL,
	}

	g := genkit.Init(ctx, genkit.WithPlugins(customPlugin))

	// Define the model
	customPlugin.DefineModel("custom", "google/gemma-4-12b", ai.ModelOptions{
		Supports: &ai.ModelSupports{
			Multiturn:   true,
			SystemRole:  true,
			Constrained: ai.ConstrainedSupportAll,
		},
	})

	// 4. Trigger GenerateData
	_, _, err := genkit.GenerateData[ScratchAIAnalysisResult](ctx, g,
		ai.WithModelName("custom/google/gemma-4-12b"),
		ai.WithPrompt("Hello"),
	)
	if err != nil {
		t.Fatalf("GenerateData failed: %v", err)
	}
}
