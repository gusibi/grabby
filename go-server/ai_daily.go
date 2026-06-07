package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// AIDailyManager handles generating and retrieving AI daily reports.
type AIDailyManager struct {
	db       *Database
	aiEngine *AIEngine
	logger   *zap.Logger
}

// NewAIDailyManager creates a new Daily Report manager.
func NewAIDailyManager(db *Database, aiEngine *AIEngine, logger *zap.Logger) *AIDailyManager {
	return &AIDailyManager{
		db:       db,
		aiEngine: aiEngine,
		logger:   logger,
	}
}

// GenerateDailyReport compiles high quality items for a specific date (YYYY-MM-DD) and generates a markdown report.
func (adm *AIDailyManager) GenerateDailyReport(ctx context.Context, dateStr string) (*AIDailyReport, error) {
	adm.aiEngine.mu.RLock()
	enabled := adm.aiEngine.settings.Enabled
	threshold := adm.aiEngine.settings.QualityThreshold
	adm.aiEngine.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("AI engine is disabled")
	}

	adm.logger.Info("Starting daily report generation", zap.String("date", dateStr))
	items, err := adm.db.GetQualityItemsForDate(dateStr, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality items for report: %w", err)
	}

	totalItems, err := adm.db.GetTotalItemsCountForDate(dateStr)
	if err != nil {
		adm.logger.Warn("Failed to get total items count for date", zap.String("date", dateStr), zap.Error(err))
	}

	categoryCounts := make(map[string]int)
	var feedText string
	for i, item := range items {
		categoryCounts[item.AICategory]++
		feedText += fmt.Sprintf("【资讯 #%d】\n", i+1)
		feedText += fmt.Sprintf("标题: %s\n", item.Title)
		feedText += fmt.Sprintf("分类: %s (%s)\n", item.AICategory, item.AISubcategory)
		feedText += fmt.Sprintf("来源: %s\n", item.OriginSource)
		feedText += fmt.Sprintf("评分: %d/10\n", item.QualityScore)
		feedText += fmt.Sprintf("AI摘要: %s\n", item.AISummary)
		feedText += fmt.Sprintf("推荐理由: %s\n", item.AIComment)
		feedText += fmt.Sprintf("链接: %s\n\n", item.URL)
	}

	catJSON, err := json.Marshal(categoryCounts)
	if err != nil {
		catJSON = []byte("{}")
	}

	title := fmt.Sprintf("Grabby AI 智能日报 · %s", dateStr)

	// If no quality items found, write a short default notice
	if len(items) == 0 {
		notice := fmt.Sprintf("# %s\n\n今日共抓取 %d 条资讯。未筛选出评分高于 %d 分的优质内容，故今日无推荐要闻。", title, totalItems, threshold)
		report := AIDailyReport{
			ReportDate:        dateStr,
			Title:             title,
			Content:           notice,
			TotalItems:        totalItems,
			QualityItems:      0,
			CategoriesSummary: string(catJSON),
			ModelUsed:         "system",
			GeneratedAt:       time.Now(),
		}
		err = adm.db.InsertAIDailyReport(report)
		if err != nil {
			return nil, fmt.Errorf("failed to save empty daily report: %w", err)
		}
		return &report, nil
	}

	adm.aiEngine.mu.RLock()
	engineSettings := adm.aiEngine.settings
	selector := adm.aiEngine.selector
	clients := adm.aiEngine.clients
	adm.aiEngine.mu.RUnlock()

	dailyPrompt := engineSettings.DailyPrompt
	if dailyPrompt == "" {
		dailyPrompt = DefaultDailyPrompt
	}

	prompt := dailyPrompt
	prompt = strings.ReplaceAll(prompt, "{{.Count}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.FeedText}}", feedText)
	prompt = strings.ReplaceAll(prompt, "{{.TotalItems}}", fmt.Sprintf("%d", totalItems))
	prompt = strings.ReplaceAll(prompt, "{{.QualityItems}}", fmt.Sprintf("%d", len(items)))

	// Generate Markdown via selector (with failover)
	var content string
	maxAttempts := selector.EnabledCount()
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		profile := selector.Next()
		if profile == nil {
			return nil, fmt.Errorf("no AI profile available for daily report")
		}
		pc, ok := clients[profile.ID]
		if !ok {
			selector.MarkUnhealthy(profile.ID)
			continue
		}
		reportCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		content, err = adm.aiEngine.callProfile(reportCtx, pc, prompt)
		cancel()
		if err != nil {
			adm.logger.Warn("Profile failed for daily report, trying next",
				zap.String("profile", profile.Name), zap.Error(err))
			selector.MarkUnhealthy(profile.ID)
			continue
		}
		selector.MarkHealthy(profile.ID)
		break
	}
	if content == "" {
		return nil, fmt.Errorf("all AI profiles failed to generate daily report")
	}

	report := AIDailyReport{
		ReportDate:        dateStr,
		Title:             title,
		Content:           content,
		TotalItems:        totalItems,
		QualityItems:      len(items),
		CategoriesSummary: string(catJSON),
		ModelUsed:         engineSettings.Model,
		GeneratedAt:       time.Now(),
	}

	err = adm.db.InsertAIDailyReport(report)
	if err != nil {
		return nil, fmt.Errorf("failed to save generated daily report: %w", err)
	}

	adm.logger.Info("Successfully generated daily report", zap.String("date", dateStr), zap.Int("items_included", len(items)))
	return &report, nil
}
