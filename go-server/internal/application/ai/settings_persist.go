package ai

import (
	"encoding/json"
	"fmt"
	"strconv"

	domainai "go-server/internal/domain/ai"
)

// SettingsStore is the persistence surface needed to save one settings
// document. *sqlite.Database satisfies it.
type SettingsStore interface {
	SaveSetting(key string, value string) error
	ReplaceAIConnections(profiles []domainai.AIProviderProfile) error
	SaveAIRoute(strategy string, profiles []domainai.AIProviderProfile) error
}

// SettingsReloader is the runtime surface that applies a saved document to a
// live engine. *AIEngine satisfies it; tests can pass a fake.
type SettingsReloader interface {
	ReloadSettings(settings domainai.AISettings) error
}

// PersistSettings normalizes one settings document, writes it to the settings
// table plus the normalized connection/route tables, and reloads the engine.
//
// It is shared by the settings endpoint, the external-config importer and the
// backup import, so every path stores identical data.
func PersistSettings(store SettingsStore, reloader SettingsReloader, req domainai.AISettings) error {
	req = domainai.NormalizeAISettings(req)

	// 1. Save to SQLite database
	enabledStr := "false"
	if req.Enabled {
		enabledStr = "true"
	}
	_ = store.SaveSetting("ai_enabled", enabledStr)
	_ = store.SaveSetting("ai_provider", req.Provider)
	_ = store.SaveSetting("ai_api_key", req.APIKey)
	_ = store.SaveSetting("ai_model", req.Model)
	_ = store.SaveSetting("ai_base_url", req.BaseURL)
	_ = store.SaveSetting("ai_quality_threshold", strconv.Itoa(req.QualityThreshold))
	_ = store.SaveSetting("ai_system_prompt", req.SystemPrompt)
	_ = store.SaveSetting("ai_daily_prompt", req.DailyPrompt)
	_ = store.SaveSetting("ai_active_profile_id", req.ActiveProfileID)
	if profilesBytes, err := json.Marshal(req.Profiles); err == nil {
		_ = store.SaveSetting("ai_provider_profiles", string(profilesBytes))
	}

	morningEnabledStr := "false"
	if req.MorningReportEnabled {
		morningEnabledStr = "true"
	}
	eveningEnabledStr := "false"
	if req.EveningReportEnabled {
		eveningEnabledStr = "true"
	}
	_ = store.SaveSetting("ai_morning_report_enabled", morningEnabledStr)
	_ = store.SaveSetting("ai_evening_report_enabled", eveningEnabledStr)
	_ = store.SaveSetting("ai_morning_report_time", req.MorningReportTime)
	_ = store.SaveSetting("ai_evening_report_time", req.EveningReportTime)

	// 1b. Normalized tables (connections / models / route) are authoritative
	// for the engine and the usage dashboard.
	if err := store.ReplaceAIConnections(req.Profiles); err != nil {
		return fmt.Errorf("save AI connections: %w", err)
	}
	if err := store.SaveAIRoute(req.Strategy, req.Profiles); err != nil {
		return fmt.Errorf("save AI route: %w", err)
	}

	// 2. Reload settings in the AI Engine
	if reloader != nil {
		if err := reloader.ReloadSettings(req); err != nil {
			return fmt.Errorf("reload AI engine settings: %w", err)
		}
	}
	return nil
}
