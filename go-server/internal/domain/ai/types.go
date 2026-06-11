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
type AIProviderProfile struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Provider          string `json:"provider"`
	APIKey            string `json:"api_key"`
	Model             string `json:"model"`
	BaseURL           string `json:"base_url"`
	Disabled          bool   `json:"disabled"`            // skip this profile in multi-profile strategies
	Priority          int    `json:"priority"`            // lower = higher priority (for failover ordering)
	RequestsPerMinute int    `json:"requests_per_minute"` // per-profile rate limit; 0 = default (10/min)
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
		if settings.Profiles[i].Name == "" {
			settings.Profiles[i].Name = settings.Profiles[i].Provider
		}
		if settings.Profiles[i].Priority <= 0 {
			settings.Profiles[i].Priority = i + 1
		}
		if settings.Profiles[i].RequestsPerMinute <= 0 {
			settings.Profiles[i].RequestsPerMinute = 10
		}
		if strings.ToLower(settings.Profiles[i].Provider) == "custom" && settings.Profiles[i].Model != "" {
			if !strings.HasPrefix(strings.ToLower(settings.Profiles[i].Model), "custom/") {
				settings.Profiles[i].Model = "custom/" + settings.Profiles[i].Model
			}
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
				settings.BaseURL = profile.BaseURL
				settings.APIKey = profile.APIKey
				break
			}
		}
	}

	if strings.ToLower(settings.Provider) == "custom" && settings.Model != "" {
		if !strings.HasPrefix(strings.ToLower(settings.Model), "custom/") {
			settings.Model = "custom/" + settings.Model
		}
	}
	return settings
}
