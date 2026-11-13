// Package catalog fetches, normalizes and caches the open models.dev catalog.
//
// Grabby's settings UI should not maintain a hand-written provider/model table:
// models.dev tracks pricing, context windows and capabilities across providers.
// This service keeps the network call, the normalization and the on-disk cache
// in one place so the HTTP layer only has to return a ready-to-render snapshot.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"go-server/internal/infrastructure/sqlite"
)

const (
	// ModelsDevURL is the provider-keyed catalog used by OpenCode and others.
	ModelsDevURL = "https://models.dev/api.json"
	// Source is stored next to the snapshot so the UI can explain where the
	// data came from.
	Source = "models.dev"
	// refreshInterval is deliberately conservative: the catalog changes daily
	// at most, and a desktop app should not call a public endpoint often.
	refreshInterval = 24 * time.Hour
	// cacheVersion invalidates normalized snapshots when the mapping or schema
	// changes. Bump it whenever normalize()/inferDriver() changes semantics.
	cacheVersion = 1
)

// Model is the compact model shape Grabby needs for selection and routing.
type Model struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Context      int64    `json:"context,omitempty"`
	MaxOutput    int64    `json:"max_output,omitempty"`
	InputPrice   float64  `json:"input_price,omitempty"`
	OutputPrice  float64  `json:"output_price,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	ReleaseDate  string   `json:"release_date,omitempty"`
}

// Provider is one selectable endpoint plus the models it exposes.
type Provider struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	API    string   `json:"api,omitempty"`
	Doc    string   `json:"doc,omitempty"`
	NPM    string   `json:"npm,omitempty"`
	Env    []string `json:"env,omitempty"`
	Driver string   `json:"driver"` // openai | anthropic | gemini | ollama | custom
	Models []Model  `json:"models"`
}

// Snapshot is what the HTTP handler returns.
type Snapshot struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Source    string     `json:"source"`
	Providers []Provider `json:"providers"`
}

// Service owns the in-memory snapshot and its SQLite-backed cache.
type Service struct {
	db     *sqlite.Database
	logger *zap.Logger
	client *http.Client

	mu        sync.RWMutex
	snapshot  Snapshot
	etag      string
	hasData   bool
	refreshMu sync.Mutex
}

// NewService creates a catalog service. No network call is made until Start or
// Get is called.
func NewService(db *sqlite.Database, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

// Start loads the cached snapshot and refreshes it when empty, then keeps it
// fresh in the background until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	if err := s.loadCache(); err != nil {
		s.logger.Warn("Failed to load cached AI catalog", zap.Error(err))
	}
	if !s.HasData() {
		refreshCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		if err := s.Refresh(refreshCtx); err != nil {
			s.logger.Warn("Failed to fetch AI catalog at startup", zap.Error(err))
		}
		cancel()
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			if err := s.Refresh(refreshCtx); err != nil {
				s.logger.Warn("Scheduled AI catalog refresh failed", zap.Error(err))
			}
			cancel()
		}
	}
}

// HasData reports whether any snapshot is currently available.
func (s *Service) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasData
}

// Get returns the current snapshot. With refresh=true it first attempts a
// network refresh; if that fails but a cached snapshot exists, the cached copy
// is returned so the settings page still works offline.
func (s *Service) Get(ctx context.Context, refresh bool) (Snapshot, error) {
	if refresh {
		if err := s.Refresh(ctx); err != nil {
			s.logger.Warn("AI catalog refresh failed; falling back to cache", zap.Error(err))
		}
	}

	s.mu.RLock()
	snapshot := s.snapshot
	hasData := s.hasData
	s.mu.RUnlock()
	if hasData {
		return snapshot, nil
	}

	// No cache: try one synchronous fetch so a first-run request is useful.
	if err := s.Refresh(ctx); err != nil {
		return Snapshot{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot, nil
}

// Refresh fetches models.dev, normalizes it and atomically replaces both the
// in-memory snapshot and the SQLite cache.
func (s *Service) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsDevURL, nil)
	if err != nil {
		return fmt.Errorf("create catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	s.mu.RLock()
	etag := s.etag
	s.mu.RUnlock()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch models.dev: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		now := time.Now().UTC()
		s.mu.Lock()
		s.snapshot.FetchedAt = now
		s.mu.Unlock()
		if err := s.db.SaveAICatalogSnapshot(mustMarshal(s.snapshot.Providers), etag, Source, cacheVersion, now); err != nil {
			s.logger.Warn("Failed to touch cached AI catalog", zap.Error(err))
		}
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("models.dev returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw map[string]rawProvider
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode models.dev catalog: %w", err)
	}

	providers := normalize(raw)
	if len(providers) == 0 {
		return fmt.Errorf("models.dev returned no usable providers")
	}

	now := time.Now().UTC()
	payload, err := json.Marshal(providers)
	if err != nil {
		return fmt.Errorf("encode normalized catalog: %w", err)
	}
	newETag := resp.Header.Get("ETag")

	s.mu.Lock()
	s.snapshot = Snapshot{FetchedAt: now, Source: Source, Providers: providers}
	s.etag = newETag
	s.hasData = true
	s.mu.Unlock()

	if err := s.db.SaveAICatalogSnapshot(string(payload), newETag, Source, cacheVersion, now); err != nil {
		// The in-memory copy is still usable; report the persistence issue but
		// do not fail the refresh.
		s.logger.Warn("Failed to persist AI catalog", zap.Error(err))
	}

	s.logger.Info("AI catalog refreshed",
		zap.Int("providers", len(providers)),
		zap.Int("models", countModels(providers)),
		zap.String("source", Source))
	return nil
}

func (s *Service) loadCache() error {
	rec, err := s.db.GetAICatalogSnapshot()
	if err != nil || rec == nil {
		return err
	}
	if rec.Version != cacheVersion {
		// The normalized schema changed; keep the row but ignore it so the
		// next refresh rebuilds the snapshot.
		return nil
	}
	var providers []Provider
	if err := json.Unmarshal([]byte(rec.Payload), &providers); err != nil {
		return fmt.Errorf("decode cached catalog: %w", err)
	}
	if len(providers) == 0 {
		return nil
	}
	s.mu.Lock()
	s.snapshot = Snapshot{FetchedAt: rec.FetchedAt, Source: rec.Source, Providers: providers}
	s.etag = rec.ETag
	s.hasData = true
	s.mu.Unlock()
	s.logger.Info("Loaded AI catalog from cache",
		zap.Int("providers", len(providers)),
		zap.Time("fetched_at", rec.FetchedAt))
	return nil
}

func countModels(providers []Provider) int {
	total := 0
	for _, provider := range providers {
		total += len(provider.Models)
	}
	return total
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ---------------------------------------------------------------------------
// models.dev schema + normalization
// ---------------------------------------------------------------------------

type rawProvider struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	API    string              `json:"api"`
	Doc    string              `json:"doc"`
	NPM    string              `json:"npm"`
	Env    []string            `json:"env"`
	Models map[string]rawModel `json:"models"`
}

type rawModel struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Attachment       bool          `json:"attachment"`
	Reasoning        bool          `json:"reasoning"`
	ToolCall         bool          `json:"tool_call"`
	StructuredOutput bool          `json:"structured_output"`
	Modalities       rawModalities `json:"modalities"`
	Limit            rawLimit      `json:"limit"`
	Cost             rawCost       `json:"cost"`
	ReleaseDate      string        `json:"release_date"`
}

type rawModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type rawLimit struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

type rawCost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

func normalize(raw map[string]rawProvider) []Provider {
	providers := make([]Provider, 0, len(raw))
	for key, p := range raw {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			id = key
		}
		driver, ok := inferDriver(id, p.NPM)
		if !ok {
			continue
		}
		api := strings.TrimSpace(p.API)
		if api == "" {
			api = defaultAPI(id)
		}
		if api == "" && driver != "ollama" && id != "openai" && id != "anthropic" && id != "google" {
			continue
		}
		models := normalizeModels(p.Models)
		if len(models) == 0 {
			continue
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = id
		}
		providers = append(providers, Provider{
			ID:     id,
			Name:   name,
			API:    api,
			Doc:    strings.TrimSpace(p.Doc),
			NPM:    strings.TrimSpace(p.NPM),
			Env:    p.Env,
			Driver: driver,
			Models: models,
		})
	}
	sort.Slice(providers, func(i, j int) bool {
		return strings.ToLower(providers[i].Name) < strings.ToLower(providers[j].Name)
	})
	return providers
}

func normalizeModels(raw map[string]rawModel) []Model {
	models := make([]Model, 0, len(raw))
	for key, m := range raw {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			id = key
		}
		if id == "" || isNonChatModel(id, m.Name) {
			continue
		}
		// Only chat-capable entries are useful for analysis and reports.
		if len(m.Modalities.Output) > 0 && !containsFold(m.Modalities.Output, "text") {
			continue
		}
		caps := make([]string, 0, 4)
		if m.Attachment || containsFold(m.Modalities.Input, "image") {
			caps = append(caps, "vision")
		}
		if m.Reasoning {
			caps = append(caps, "reasoning")
		}
		if m.ToolCall {
			caps = append(caps, "tool")
		}
		if m.StructuredOutput {
			caps = append(caps, "structured")
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		models = append(models, Model{
			ID:           id,
			Name:         name,
			Context:      m.Limit.Context,
			MaxOutput:    m.Limit.Output,
			InputPrice:   m.Cost.Input,
			OutputPrice:  m.Cost.Output,
			Capabilities: caps,
			ReleaseDate:  strings.TrimSpace(m.ReleaseDate),
		})
	}
	sort.Slice(models, func(i, j int) bool {
		// Newest first; entries without a date fall behind dated ones.
		if models[i].ReleaseDate != models[j].ReleaseDate {
			if models[i].ReleaseDate == "" {
				return false
			}
			if models[j].ReleaseDate == "" {
				return true
			}
			return models[i].ReleaseDate > models[j].ReleaseDate
		}
		return strings.ToLower(models[i].Name) < strings.ToLower(models[j].Name)
	})
	return models
}

func isNonChatModel(id, name string) bool {
	lower := strings.ToLower(id + " " + name)
	for _, marker := range []string{"embedding", "embed-", "tts", "whisper", "moderation", "rerank", "dall-e", "image-gen"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// inferDriver maps a models.dev provider to one of Grabby's Genkit drivers.
// Unknown or unsupported protocols are dropped instead of being silently
// routed to the wrong backend.
// defaultAPI fills in endpoints that models.dev leaves empty for some
// providers (for example xAI). These are the public OpenAI-compatible roots;
// keeping them here avoids dropping otherwise usable providers.
func defaultAPI(id string) string {
	switch id {
	case "openai", "anthropic", "google":
		return "" // official SDKs know their endpoints
	case "xai":
		return "https://api.x.ai/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "cerebras":
		return "https://api.cerebras.ai/v1"
	case "togetherai":
		return "https://api.together.xyz/v1"
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1"
	case "perplexity":
		return "https://api.perplexity.ai"
	case "cohere":
		return "https://api.cohere.ai/compatibility/v1"
	case "moonshot":
		return "https://api.moonshot.cn/v1"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "siliconflow":
		return "https://api.siliconflow.cn/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return ""
	}
}

func inferDriver(id, npm string) (string, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	npm = strings.ToLower(strings.TrimSpace(npm))

	// Vertex and other cloud-specific protocols need extra credentials that
	// the current profile model cannot express yet.
	if strings.Contains(id, "vertex") || strings.Contains(id, "bedrock") ||
		strings.Contains(id, "azure") || strings.Contains(id, "cloudflare") ||
		strings.Contains(id, "watsonx") || strings.Contains(id, "snowflake") {
		return "", false
	}

	switch id {
	case "openai":
		return "openai", true
	case "anthropic":
		return "anthropic", true
	case "google":
		return "gemini", true
	case "ollama":
		return "ollama", true
	}

	switch {
	case strings.Contains(npm, "anthropic"):
		return "anthropic", true
	case strings.Contains(npm, "google"):
		return "gemini", true
	case strings.Contains(npm, "openrouter"):
		return "custom", true
	case strings.Contains(npm, "openai-compatible"):
		return "custom", true
	case strings.HasPrefix(npm, "@ai-sdk/openai"):
		// Azure and other vendor-specific OpenAI wrappers were filtered above.
		return "custom", true
	}

	switch id {
	case "deepseek", "xai", "mistral", "groq", "cerebras", "togetherai",
		"fireworks", "perplexity", "cohere", "moonshot", "zhipu", "qwen",
		"siliconflow", "openrouter":
		if id == "zhipu" || id == "qwen" || id == "siliconflow" || id == "moonshot" {
			return "custom", true
		}
		return "custom", true
	}

	return "", false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
