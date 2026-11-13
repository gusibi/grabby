package ai

import (
	"fmt"
	"strings"
	"time"
)

// AIAnalysis represents the AI analysis details of a scraped item.
type AIAnalysis struct {
	ID            int64     `json:"id"`
	ItemID        int64     `json:"item_id"`
	AICategory    string    `json:"ai_category"`
	AISubcategory string    `json:"ai_subcategory"`
	QualityScore  int       `json:"quality_score"`
	AISummary     string    `json:"ai_summary"`
	AIComment     string    `json:"ai_comment"`
	AITags        string    `json:"ai_tags"` // comma-separated tags
	ModelUsed     string    `json:"model_used"`
	ProcessedAt   time.Time `json:"processed_at"`
}

// AIDailyReport represents the generated daily report.
type AIDailyReport struct {
	ID                int64     `json:"id"`
	ReportDate        string    `json:"report_date"` // YYYY-MM-DD
	ReportType        string    `json:"report_type"` // "morning", "evening", "daily"
	Title             string    `json:"title"`
	Content           string    `json:"content"` // Markdown
	TotalItems        int       `json:"total_items"`
	QualityItems      int       `json:"quality_items"`
	CategoriesSummary string    `json:"categories_summary"` // JSON string of category counts
	ModelUsed         string    `json:"model_used"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// AICategoryStat represents statistics for an AI category.
type AICategoryStat struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	AvgScore float64 `json:"avg_score"`
}

// AISettings represents the configuration of the AI engine.
type AISettings struct {
	Enabled              bool                `json:"enabled"`
	Provider             string              `json:"provider"` // "openai", "gemini", "custom", etc.
	APIKey               string              `json:"api_key"`
	Model                string              `json:"model"`
	Models               []string            `json:"models"`
	BaseURL              string              `json:"base_url"`
	QualityThreshold     int                 `json:"quality_threshold"`
	SystemPrompt         string              `json:"system_prompt"`
	DailyPrompt          string              `json:"daily_prompt"`
	Strategy             string              `json:"strategy"` // "single" (default), "round-robin", "failover"
	ActiveProfileID      string              `json:"active_profile_id"`
	Profiles             []AIProviderProfile `json:"profiles"`
	MorningReportTime    string              `json:"morning_report_time"`
	EveningReportTime    string              `json:"evening_report_time"`
	MorningReportEnabled bool                `json:"morning_report_enabled"`
	EveningReportEnabled bool                `json:"evening_report_enabled"`
}

// AIProviderProfile stores one selectable AI provider connection profile.
//
// A profile may carry several models. The rotation unit is the (profile, model)
// pair, so a provider with several free models is used model-by-model before
// moving on. Model is kept as the primary/first entry for backward
// compatibility with older clients and stored analysis rows.
type AIProviderProfile struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	APIKey            string   `json:"api_key"`
	Model             string   `json:"model"`
	Models            []string `json:"models"`
	BaseURL           string   `json:"base_url"`
	Disabled          bool     `json:"disabled"`            // skip this profile in multi-profile strategies
	Priority          int      `json:"priority"`            // lower = higher priority (for failover ordering)
	RequestsPerMinute int      `json:"requests_per_minute"` // per-profile rate limit; 0 = default (10/min)
}

// EffectiveModels returns the profile's model list, tolerating legacy profiles
// that only set the single Model field.
func (p AIProviderProfile) EffectiveModels() []string {
	if len(p.Models) > 0 {
		return p.Models
	}
	if p.Model != "" {
		return []string{p.Model}
	}
	return nil
}

// NormalizeModelList trims, drops empties, de-dupes (case-insensitive) and
// applies the backend namespace Genkit expects for the provider.
func NormalizeModelList(provider string, models []string) []string {
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, raw := range models {
		model := normalizeModelRef(provider, raw)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, model)
	}
	return out
}

// modelPrefix maps a provider driver to the namespace Genkit expects in a
// model reference. Local providers (LM Studio) and empty providers stay
// unprefixed because they are called through the direct client.
func modelPrefix(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gemini", "googleai", "google":
		return "googleai/"
	case "openai":
		return "openai/"
	case "anthropic", "claude":
		return "anthropic/"
	case "ollama":
		return "ollama/"
	case "vertex", "vertexai":
		return "vertexai/"
	case "custom":
		return "custom/"
	default:
		return ""
	}
}

// DriverModelRef produces the exact model reference the selected Genkit
// driver expects. Storage may keep either a bare id or an already-prefixed
// reference (older rows, env config), so this is applied at call time.
func DriverModelRef(provider, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}
	prefix := modelPrefix(provider)
	if prefix == "" {
		return model
	}
	lower := strings.ToLower(model)
	if strings.HasPrefix(lower, prefix) {
		return model
	}
	if prefix == "custom/" {
		// A custom OpenAI-compatible provider may legitimately use a
		// vendor/model id (for example OpenRouter's
		// "anthropic/claude-3.5-sonnet"), so only the "custom/" namespace
		// itself is stale.
		return prefix + model
	}
	// Drop a stale namespace from a previously selected provider so the same
	// input can be switched between drivers without producing
	// "openai/openai/gpt-4o".
	if idx := strings.Index(model, "/"); idx > 0 {
		head := lower[:idx]
		switch head {
		case "googleai", "openai", "anthropic", "ollama", "vertexai", "custom":
			model = model[idx+1:]
		}
	}
	return prefix + model
}

func normalizeModelRef(provider, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || !strings.EqualFold(provider, "custom") {
		return model
	}
	if !strings.HasPrefix(strings.ToLower(model), "custom/") {
		return "custom/" + model
	}
	return model
}

// NormalizeAISettings cleans up user inputs (trimming whitespace) and handles
// specific model name normalization for custom OpenAI-compatible endpoints.
func NormalizeAISettings(settings AISettings) AISettings {
	settings.Provider = strings.TrimSpace(settings.Provider)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.SystemPrompt = strings.TrimSpace(settings.SystemPrompt)
	settings.DailyPrompt = strings.TrimSpace(settings.DailyPrompt)
	settings.ActiveProfileID = strings.TrimSpace(settings.ActiveProfileID)
	settings.Strategy = strings.TrimSpace(settings.Strategy)
	if settings.Strategy == "" {
		settings.Strategy = "single"
	}
	settings.MorningReportTime = strings.TrimSpace(settings.MorningReportTime)
	if settings.MorningReportTime == "" {
		settings.MorningReportTime = "08:30"
	}
	settings.EveningReportTime = strings.TrimSpace(settings.EveningReportTime)
	if settings.EveningReportTime == "" {
		settings.EveningReportTime = "22:30"
	}

	seenIDs := make(map[string]struct{}, len(settings.Profiles))
	for i := range settings.Profiles {
		settings.Profiles[i].ID = strings.TrimSpace(settings.Profiles[i].ID)
		settings.Profiles[i].Name = strings.TrimSpace(settings.Profiles[i].Name)
		settings.Profiles[i].Provider = strings.TrimSpace(settings.Profiles[i].Provider)
		settings.Profiles[i].Model = strings.TrimSpace(settings.Profiles[i].Model)
		settings.Profiles[i].BaseURL = strings.TrimSpace(settings.Profiles[i].BaseURL)
		settings.Profiles[i].APIKey = strings.TrimSpace(settings.Profiles[i].APIKey)
		if settings.Profiles[i].ID == "" {
			settings.Profiles[i].ID = fmt.Sprintf("profile-%d", i+1)
		}
		// Connection IDs are primary keys in the normalized tables; keep them
		// unique even if a client sends duplicates.
		baseID := settings.Profiles[i].ID
		for suffix := 2; ; suffix++ {
			if _, exists := seenIDs[settings.Profiles[i].ID]; !exists {
				break
			}
			settings.Profiles[i].ID = fmt.Sprintf("%s-%d", baseID, suffix)
		}
		seenIDs[settings.Profiles[i].ID] = struct{}{}
		if settings.Profiles[i].Name == "" {
			settings.Profiles[i].Name = settings.Profiles[i].Provider
		}
		if settings.Profiles[i].Priority <= 0 {
			settings.Profiles[i].Priority = i + 1
		}
		if settings.Profiles[i].RequestsPerMinute <= 0 {
			settings.Profiles[i].RequestsPerMinute = 10
		}

		// Migration + canonical form: profiles saved before multi-model support
		// only carry Model. Keep Models as the source of truth and mirror the
		// first entry back into Model for older clients / stored rows.
		models := settings.Profiles[i].Models
		if len(models) == 0 && settings.Profiles[i].Model != "" {
			models = []string{settings.Profiles[i].Model}
		}
		settings.Profiles[i].Models = NormalizeModelList(settings.Profiles[i].Provider, models)
		if len(settings.Profiles[i].Models) > 0 {
			settings.Profiles[i].Model = settings.Profiles[i].Models[0]
		} else {
			settings.Profiles[i].Model = ""
		}
	}

	if len(settings.Profiles) > 0 {
		if settings.ActiveProfileID == "" {
			settings.ActiveProfileID = settings.Profiles[0].ID
		}
		for _, profile := range settings.Profiles {
			if profile.ID == settings.ActiveProfileID {
				settings.Provider = profile.Provider
				settings.Model = profile.Model
				settings.Models = append([]string(nil), profile.EffectiveModels()...)
				settings.BaseURL = profile.BaseURL
				settings.APIKey = profile.APIKey
				break
			}
		}
	}

	settings.Model = normalizeModelRef(settings.Provider, settings.Model)

	// Top-level Models mirrors the active profile for callers that only pass
	// AISettings around (e.g. initGenkit). Keep it consistent with Model.
	settings.Models = NormalizeModelList(settings.Provider, settings.Models)
	if len(settings.Models) == 0 && settings.Model != "" {
		settings.Models = []string{settings.Model}
	}
	if len(settings.Models) > 0 {
		settings.Model = settings.Models[0]
	}
	return settings
}
