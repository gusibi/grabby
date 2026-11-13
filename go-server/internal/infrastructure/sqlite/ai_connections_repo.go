package sqlite

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIConnectionRecord stores one provider endpoint (the old "profile").
type AIConnectionRecord struct {
	ID                string    `gorm:"primaryKey;column:id"`
	Name              string    `gorm:"column:name"`
	Driver            string    `gorm:"column:driver"`
	BaseURL           string    `gorm:"column:base_url"`
	APIKey            string    `gorm:"column:api_key"`
	Disabled          bool      `gorm:"column:disabled"`
	Priority          int       `gorm:"column:priority"`
	RequestsPerMinute int       `gorm:"column:requests_per_minute"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (AIConnectionRecord) TableName() string { return "ai_connections" }

// AIModelRecord stores one model under a connection. ModelID is the raw
// provider id; ID is a stable composite key for upserts.
type AIModelRecord struct {
	ID            string     `gorm:"primaryKey;column:id"`
	ConnectionID  string     `gorm:"index;column:connection_id"`
	ModelID       string     `gorm:"column:model_id"`
	DisplayName   string     `gorm:"column:display_name"`
	Enabled       bool       `gorm:"column:enabled"`
	Priority      int        `gorm:"column:priority"`
	ContextWindow int64      `gorm:"column:context_window"`
	InputPrice    float64    `gorm:"column:input_price"`
	OutputPrice   float64    `gorm:"column:output_price"`
	Capabilities  string     `gorm:"column:capabilities"`
	LastCheckedAt *time.Time `gorm:"column:last_checked_at"`
	LatencyMS     int64      `gorm:"column:latency_ms"`
	Status        string     `gorm:"column:status"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (AIModelRecord) TableName() string { return "ai_models" }

// AIRouteRecord stores the routing policy. The current release keeps one row
// ("default") for the existing single/round-robin/failover strategy; the task
// column leaves room for per-task routing later.
type AIRouteRecord struct {
	Task      string    `gorm:"primaryKey;column:task"`
	Strategy  string    `gorm:"column:strategy"`
	Entries   string    `gorm:"column:entries"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (AIRouteRecord) TableName() string { return "ai_routes" }

// AIUsageRecord is one model call, measured for the usage dashboard.
type AIUsageRecord struct {
	ID               uint      `gorm:"primaryKey;autoIncrement;column:id"`
	ConnectionID     string    `gorm:"index;column:connection_id"`
	ModelID          string    `gorm:"column:model_id"`
	Task             string    `gorm:"column:task"`
	PromptTokens     int       `gorm:"column:prompt_tokens"`
	CompletionTokens int       `gorm:"column:completion_tokens"`
	TotalTokens      int       `gorm:"column:total_tokens"`
	Cost             float64   `gorm:"column:cost"`
	LatencyMS        int64     `gorm:"column:latency_ms"`
	Status           string    `gorm:"column:status"`
	Error            string    `gorm:"column:error"`
	CreatedAt        time.Time `gorm:"index;column:created_at"`
}

func (AIUsageRecord) TableName() string { return "ai_usage" }

func modelRecordID(connectionID, modelID string) string {
	return connectionID + "::" + modelID
}

// HasAIConnections reports whether the normalized connection table has data.
func (d *Database) HasAIConnections() (bool, error) {
	var count int64
	if err := d.gorm.Model(&AIConnectionRecord{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ReplaceAIConnections upserts every connection/model and removes rows that are
// no longer part of the settings document.
func (d *Database) ReplaceAIConnections(profiles []AIProviderProfile) error {
	return d.gorm.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		ids := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			ids = append(ids, profile.ID)
			conn := AIConnectionRecord{
				ID:                profile.ID,
				Name:              profile.Name,
				Driver:            profile.Provider,
				BaseURL:           profile.BaseURL,
				APIKey:            profile.APIKey,
				Disabled:          profile.Disabled,
				Priority:          profile.Priority,
				RequestsPerMinute: profile.RequestsPerMinute,
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"name", "driver", "base_url", "api_key", "disabled",
					"priority", "requests_per_minute", "updated_at",
				}),
			}).Create(&conn).Error; err != nil {
				return err
			}

			models := profile.EffectiveModels()
			modelIDs := make([]string, 0, len(models))
			for i, modelID := range models {
				id := modelRecordID(profile.ID, modelID)
				modelIDs = append(modelIDs, id)
				model := AIModelRecord{
					ID:           id,
					ConnectionID: profile.ID,
					ModelID:      modelID,
					DisplayName:  modelID,
					Enabled:      true,
					Priority:     i + 1,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "id"}},
					DoUpdates: clause.AssignmentColumns([]string{
						"display_name", "enabled", "priority", "updated_at",
					}),
				}).Create(&model).Error; err != nil {
					return err
				}
			}
			if len(modelIDs) == 0 {
				if err := tx.Where("connection_id = ?", profile.ID).Delete(&AIModelRecord{}).Error; err != nil {
					return err
				}
			} else if err := tx.Where("connection_id = ? AND id NOT IN ?", profile.ID, modelIDs).Delete(&AIModelRecord{}).Error; err != nil {
				return err
			}
		}

		if len(ids) == 0 {
			if err := tx.Where("1 = 1").Delete(&AIConnectionRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("1 = 1").Delete(&AIModelRecord{}).Error; err != nil {
				return err
			}
			return nil
		}
		if err := tx.Where("id NOT IN ?", ids).Delete(&AIConnectionRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("connection_id NOT IN ?", ids).Delete(&AIModelRecord{}).Error
	})
}

// LoadAIConnections rebuilds the API-facing profile shape from the normalized
// tables. The result is ordered by connection priority.
func (d *Database) LoadAIConnections() ([]AIProviderProfile, error) {
	var connections []AIConnectionRecord
	if err := d.gorm.Order("priority ASC, id ASC").Find(&connections).Error; err != nil {
		return nil, err
	}
	if len(connections) == 0 {
		return nil, nil
	}
	var models []AIModelRecord
	if err := d.gorm.Order("connection_id ASC, priority ASC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	byConnection := make(map[string][]AIModelRecord, len(connections))
	for _, model := range models {
		if !model.Enabled {
			continue
		}
		byConnection[model.ConnectionID] = append(byConnection[model.ConnectionID], model)
	}

	profiles := make([]AIProviderProfile, 0, len(connections))
	for _, connection := range connections {
		profile := AIProviderProfile{
			ID:                connection.ID,
			Name:              connection.Name,
			Provider:          connection.Driver,
			APIKey:            connection.APIKey,
			BaseURL:           connection.BaseURL,
			Disabled:          connection.Disabled,
			Priority:          connection.Priority,
			RequestsPerMinute: connection.RequestsPerMinute,
		}
		for _, model := range byConnection[connection.ID] {
			profile.Models = append(profile.Models, model.ModelID)
		}
		if len(profile.Models) > 0 {
			profile.Model = profile.Models[0]
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

type routeEntry struct {
	ProfileID string `json:"profile_id"`
	Priority  int    `json:"priority"`
}

// SaveAIRoute stores the current strategy and profile order.
func (d *Database) SaveAIRoute(strategy string, profiles []AIProviderProfile) error {
	entries := make([]routeEntry, 0, len(profiles))
	for i, profile := range profiles {
		priority := profile.Priority
		if priority <= 0 {
			priority = i + 1
		}
		entries = append(entries, routeEntry{ProfileID: profile.ID, Priority: priority})
	}
	encoded, _ := json.Marshal(entries)
	rec := AIRouteRecord{
		Task:      "default",
		Strategy:  strategy,
		Entries:   string(encoded),
		UpdatedAt: time.Now().UTC(),
	}
	return d.gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task"}},
		DoUpdates: clause.AssignmentColumns([]string{"strategy", "entries", "updated_at"}),
	}).Create(&rec).Error
}

// LoadAIRoute returns the saved strategy, or an empty string when none exists.
func (d *Database) LoadAIRoute() (string, error) {
	var rec AIRouteRecord
	err := d.gorm.Where("task = ?", "default").First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return rec.Strategy, nil
}

// RecordAIUsage appends one model call to the usage log.
func (d *Database) RecordAIUsage(record AIUsageRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	return d.gorm.Create(&record).Error
}

type AIUsageByModel struct {
	ConnectionID     string  `json:"connection_id"`
	ModelID          string  `json:"model_id"`
	Task             string  `json:"task"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	AvgLatencyMS     float64 `json:"avg_latency_ms"`
}

type AIUsageByDay struct {
	Day              string  `json:"day"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

type AIUsageSummary struct {
	Days             int              `json:"days"`
	TotalRequests    int64            `json:"total_requests"`
	FailedRequests   int64            `json:"failed_requests"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	TotalTokens      int64            `json:"total_tokens"`
	Cost             float64          `json:"cost"`
	AvgLatencyMS     float64          `json:"avg_latency_ms"`
	ByModel          []AIUsageByModel `json:"by_model"`
	ByDay            []AIUsageByDay   `json:"by_day"`
}

// GetAIUsageSummary aggregates the last N days of model calls.
func (d *Database) GetAIUsageSummary(days int) (AIUsageSummary, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	summary := AIUsageSummary{Days: days}
	where := func() *gorm.DB {
		return d.gorm.Model(&AIUsageRecord{}).Where("created_at >= ?", since)
	}

	if err := where().Count(&summary.TotalRequests).Error; err != nil {
		return summary, err
	}
	if err := where().Where("status <> ?", "success").Count(&summary.FailedRequests).Error; err != nil {
		return summary, err
	}
	var totals struct {
		PromptTokens     int64   `gorm:"column:prompt_tokens"`
		CompletionTokens int64   `gorm:"column:completion_tokens"`
		TotalTokens      int64   `gorm:"column:total_tokens"`
		Cost             float64 `gorm:"column:cost"`
		AvgLatencyMS     float64 `gorm:"column:avg_latency_ms"`
	}
	if err := where().Select(
		"COALESCE(SUM(prompt_tokens),0) as prompt_tokens, COALESCE(SUM(completion_tokens),0) as completion_tokens, COALESCE(SUM(total_tokens),0) as total_tokens, COALESCE(SUM(cost),0) as cost, COALESCE(AVG(latency_ms),0) as avg_latency_ms",
	).Scan(&totals).Error; err != nil {
		return summary, err
	}
	summary.PromptTokens = totals.PromptTokens
	summary.CompletionTokens = totals.CompletionTokens
	summary.TotalTokens = totals.TotalTokens
	summary.Cost = totals.Cost
	summary.AvgLatencyMS = totals.AvgLatencyMS
	var byModel []AIUsageByModel
	if err := where().Select(
		"connection_id, model_id, task, COUNT(*) as requests, COALESCE(SUM(prompt_tokens),0) as prompt_tokens, COALESCE(SUM(completion_tokens),0) as completion_tokens, COALESCE(SUM(total_tokens),0) as total_tokens, COALESCE(SUM(cost),0) as cost, COALESCE(AVG(latency_ms),0) as avg_latency_ms",
	).Group("connection_id, model_id, task").Order("requests DESC").Scan(&byModel).Error; err != nil {
		return summary, err
	}
	var byDay []AIUsageByDay
	if err := where().Select(
		"date(created_at) as day, COUNT(*) as requests, COALESCE(SUM(prompt_tokens),0) as prompt_tokens, COALESCE(SUM(completion_tokens),0) as completion_tokens, COALESCE(SUM(cost),0) as cost",
	).Group("date(created_at)").Order("day ASC").Scan(&byDay).Error; err != nil {
		return summary, err
	}
	summary.ByModel = byModel
	summary.ByDay = byDay
	if summary.ByModel == nil {
		summary.ByModel = []AIUsageByModel{}
	}
	if summary.ByDay == nil {
		summary.ByDay = []AIUsageByDay{}
	}
	return summary, nil
}
