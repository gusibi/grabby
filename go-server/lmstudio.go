package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// LMStudioClient directly calls LM Studio's OpenAI-compatible API,
// bypassing Genkit to avoid response_format issues.
type LMStudioClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewLMStudioClient creates a new LM Studio API client.
func NewLMStudioClient(baseURL, model string, logger *zap.Logger) *LMStudioClient {
	if baseURL == "" {
		baseURL = "http://localhost:1234"
	}
	return &LMStudioClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

type chatRequest struct {
	Model        string          `json:"model"`
	Messages     []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string       `json:"type"`
	JSONSchema *jsonSchema  `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error json.RawMessage `json:"error,omitempty"`
}

// errorText extracts the error message from chatResponse.Error,
// which may be a plain string or an object with a "message" field.
func (r *chatResponse) errorText() string {
	if len(r.Error) == 0 {
		return ""
	}
	// Try as string first
	var s string
	if err := json.Unmarshal(r.Error, &s); err == nil {
		return s
	}
	// Try as object with message field
	var obj struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(r.Error, &obj); err == nil {
		return obj.Message
	}
	return string(r.Error)
}

// Generate sends a prompt to LM Studio and returns the response text.
func (c *LMStudioClient) Generate(ctx context.Context, prompt string) (string, error) {
	return c.GenerateWithSchema(ctx, prompt, nil)
}

// analysisResponseSchema is the JSON Schema for structured output from LM Studio.
var analysisResponseSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"category": {"type": "string"},
		"subcategory": {"type": "string"},
		"quality_score": {"type": "integer"},
		"summary": {"type": "string"},
		"comment": {"type": "string"},
		"tags": {"type": "array", "items": {"type": "string"}}
	},
	"required": ["category", "subcategory", "quality_score", "summary", "comment", "tags"]
}`)

// GenerateWithSchema sends a prompt to LM Studio with optional structured output.
// If schema is non-nil, uses response_format json_schema to force valid JSON.
func (c *LMStudioClient) GenerateWithSchema(ctx context.Context, prompt string, schema *json.RawMessage) (string, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	if schema != nil {
		reqBody.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchema{Name: "analysis_result", Schema: *schema},
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Calling LM Studio API", zap.String("url", url), zap.String("model", c.model))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LM Studio API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("LM Studio API response",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(respBody)))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LM Studio API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if errMsg := chatResp.errorText(); errMsg != "" {
		return "", fmt.Errorf("LM Studio error: %s", errMsg)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LM Studio returned no choices")
	}

	return StripMarkdownFences(chatResp.Choices[0].Message.Content), nil
}

// StripMarkdownFences removes ```json ... ``` wrappers that some LLMs add.
func StripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Find end of opening fence line (e.g. "```json\n" or "```\n")
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove trailing closing fence
		if strings.HasSuffix(s, "```") {
			s = strings.TrimSuffix(s, "```")
		}
		s = strings.TrimSpace(s)
	}
	return s
}
