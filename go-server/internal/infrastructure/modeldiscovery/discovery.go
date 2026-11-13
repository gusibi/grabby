// Package modeldiscovery lists the models a provider exposes through its
// public model-listing API. It deliberately uses each provider's documented
// endpoint instead of scraping a catalog, so custom/relay endpoints work too.
package modeldiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Model is the normalized result returned to the settings UI.
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
	Created int64  `json:"created,omitempty"`
}

// Discover lists models for a provider driver.
//
// Supported drivers:
//   - openai / custom / lmstudio: OpenAI-compatible GET /v1/models
//   - anthropic / claude: Anthropic GET /v1/models
//   - gemini / google: Google Generative Language GET /v1beta/models
//   - ollama: native GET /api/tags, with an OpenAI-compatible fallback
func Discover(ctx context.Context, driver, baseURL, apiKey string) ([]Model, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)

	var (
		models []Model
		err    error
	)
	switch driver {
	case "openai", "custom", "lmstudio":
		models, err = discoverOpenAI(ctx, baseURL, apiKey)
	case "anthropic", "claude":
		models, err = discoverAnthropic(ctx, baseURL, apiKey)
	case "gemini", "google", "googleai":
		models, err = discoverGemini(ctx, baseURL, apiKey)
	case "ollama":
		models, err = discoverOllama(ctx, baseURL, apiKey)
	case "vertex", "vertexai":
		return nil, fmt.Errorf("Vertex AI 的模型列表需要 GCP 凭证，暂不支持自动发现")
	default:
		return nil, fmt.Errorf("暂不支持服务商驱动 %q 的模型发现", driver)
	}
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("接口返回了空的模型列表")
	}
	sort.Slice(models, func(i, j int) bool {
		return strings.ToLower(models[i].ID) < strings.ToLower(models[j].ID)
	})
	return models, nil
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func discoverOpenAI(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	candidates := openAIURLs(baseURL)
	var lastErr error
	for _, endpoint := range candidates {
		models, err := getOpenAICompatible(ctx, endpoint, apiKey)
		if err == nil {
			return models, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func openAIURLs(baseURL string) []string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return []string{"https://api.openai.com/v1/models"}
	}
	if strings.HasSuffix(baseURL, "/v1") || strings.Contains(baseURL, "/v1/") {
		return []string{baseURL + "/models", baseURL + "/v1/models"}
	}
	return []string{baseURL + "/v1/models", baseURL + "/models"}
}

func getOpenAICompatible(ctx context.Context, endpoint, apiKey string) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	body, err := doJSON(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析模型列表失败: %w", err)
	}
	models := make([]Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = item.ID
		}
		models = append(models, Model{ID: item.ID, Name: name, OwnedBy: item.OwnedBy, Created: item.Created})
	}
	return models, nil
}

func discoverAnthropic(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic 模型发现需要 API Key")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	endpoint := baseURL + "/v1/models?limit=1000"
	if strings.HasSuffix(baseURL, "/v1") {
		endpoint = baseURL + "/models?limit=1000"
	}

	models := make([]Model, 0, 32)
	next := ""
	for page := 0; page < 10; page++ {
		pageURL := endpoint
		if next != "" {
			pageURL += "&after_id=" + url.QueryEscape(next)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		body, err := doJSON(req)
		if err != nil {
			return nil, err
		}
		var payload struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析 Anthropic 模型列表失败: %w", err)
		}
		for _, item := range payload.Data {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			name := strings.TrimSpace(item.DisplayName)
			if name == "" {
				name = item.ID
			}
			models = append(models, Model{ID: item.ID, Name: name, OwnedBy: "anthropic"})
		}
		if !payload.HasMore || payload.LastID == "" {
			break
		}
		next = payload.LastID
	}
	return models, nil
}

func discoverGemini(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini 模型发现需要 API Key")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	endpoint := baseURL + "/models"
	if !strings.HasSuffix(baseURL, "/v1beta") && !strings.HasSuffix(baseURL, "/v1") {
		endpoint = baseURL + "/v1beta/models"
	}

	models := make([]Model, 0, 32)
	pageToken := ""
	for page := 0; page < 10; page++ {
		pageURL := endpoint + "?pageSize=1000"
		if pageToken != "" {
			pageURL += "&pageToken=" + url.QueryEscape(pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-goog-api-key", apiKey)
		body, err := doJSON(req)
		if err != nil {
			return nil, err
		}
		var payload struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析 Gemini 模型列表失败: %w", err)
		}
		for _, item := range payload.Models {
			if len(item.SupportedGenerationMethods) > 0 && !containsFold(item.SupportedGenerationMethods, "generateContent") {
				continue
			}
			id := strings.TrimPrefix(item.Name, "models/")
			if id == "" {
				continue
			}
			name := strings.TrimSpace(item.DisplayName)
			if name == "" {
				name = id
			}
			models = append(models, Model{ID: id, Name: name, OwnedBy: "google"})
		}
		if payload.NextPageToken == "" {
			break
		}
		pageToken = payload.NextPageToken
	}
	return models, nil
}

func discoverOllama(ctx context.Context, baseURL, apiKey string) ([]Model, error) {
	root := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	root = strings.TrimSuffix(root, "/v1")
	if root == "" {
		root = "http://localhost:11434"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	body, err := doJSON(req)
	if err != nil {
		// Some Ollama-compatible gateways only expose /v1/models.
		return discoverOpenAI(ctx, baseURL, apiKey)
	}
	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析 Ollama 模型列表失败: %w", err)
	}
	models := make([]Model, 0, len(payload.Models))
	for _, item := range payload.Models {
		id := strings.TrimSpace(item.Model)
		if id == "" {
			id = strings.TrimSpace(item.Name)
		}
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		models = append(models, Model{ID: id, Name: name, OwnedBy: "ollama"})
	}
	return models, nil
}

func doJSON(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "Grabby/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求模型列表失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取模型列表失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型列表接口返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
