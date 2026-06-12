package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
	"go-server/internal/infrastructure/sqlite"
)

// AIDailyManager handles generating and retrieving AI daily reports.
type AIDailyManager struct {
	db       *sqlite.Database
	aiEngine *AIEngine
	logger   *zap.Logger
}

// NewAIDailyManager creates a new Daily Report manager.
func NewAIDailyManager(db *sqlite.Database, aiEngine *AIEngine, logger *zap.Logger) *AIDailyManager {
	return &AIDailyManager{
		db:       db,
		aiEngine: aiEngine,
		logger:   logger,
	}
}

// GenerateDailyReport compiles high quality items for a specific date (YYYY-MM-DD) and generates a markdown report.
func (adm *AIDailyManager) GenerateDailyReport(ctx context.Context, dateStr string) (*domainai.AIDailyReport, error) {
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
	for i, it := range items {
		categoryCounts[it.AICategory]++
		feedText += fmt.Sprintf("【资讯 #%d】\n", i+1)
		feedText += fmt.Sprintf("标题: %s\n", it.Title)
		feedText += fmt.Sprintf("分类: %s (%s)\n", it.AICategory, it.AISubcategory)
		feedText += fmt.Sprintf("来源: %s\n", it.OriginSource)
		feedText += fmt.Sprintf("评分: %d/10\n", it.QualityScore)
		feedText += fmt.Sprintf("AI摘要: %s\n", it.AISummary)
		feedText += fmt.Sprintf("推荐理由: %s\n", it.AIComment)
		feedText += fmt.Sprintf("链接: %s\n\n", it.URL)
	}

	catJSON, err := json.Marshal(categoryCounts)
	if err != nil {
		catJSON = []byte("{}")
	}

	title := fmt.Sprintf("Grabby AI 智能日报 · %s", dateStr)

	// If no quality items found, write a short default notice
	if len(items) == 0 {
		notice := fmt.Sprintf("# %s\n\n今日共抓取 %d 条资讯。未筛选出评分高于 %d 分的优质内容，故今日无推荐要闻。", title, totalItems, threshold)
		report := domainai.AIDailyReport{
			ReportDate:        dateStr,
			ReportType:        "daily",
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
	dailyPrompt = enforceDailyReportJSONContract(dailyPrompt)

	prompt := dailyPrompt
	prompt = strings.ReplaceAll(prompt, "{{.Count}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.FeedText}}", feedText)
	prompt = strings.ReplaceAll(prompt, "{{.TotalItems}}", fmt.Sprintf("%d", totalItems))
	prompt = strings.ReplaceAll(prompt, "{{.QualityItems}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.Date}}", dateStr)

	// Generate structured JSON via selector (with failover)
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
		reportCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		content, err = adm.generateValidatedDailyContent(reportCtx, pc, prompt, dateStr)
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

	report := domainai.AIDailyReport{
		ReportDate:        dateStr,
		ReportType:        "daily",
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

// GenerateRangedReport generates a report for a specific time range (used by scheduled morning/evening reports).
func (adm *AIDailyManager) GenerateRangedReport(ctx context.Context, dateStr string, reportType string, start, end time.Time) (*domainai.AIDailyReport, error) {
	adm.aiEngine.mu.RLock()
	enabled := adm.aiEngine.settings.Enabled
	threshold := adm.aiEngine.settings.QualityThreshold
	adm.aiEngine.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("AI engine is disabled")
	}

	adm.logger.Info("Starting ranged report generation",
		zap.String("date", dateStr),
		zap.String("type", reportType),
		zap.Time("start", start),
		zap.Time("end", end))

	items, err := adm.db.GetQualityItemsForTimeRange(start, end, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality items for ranged report: %w", err)
	}

	totalItems, err := adm.db.GetTotalItemsCountForTimeRange(start, end)
	if err != nil {
		adm.logger.Warn("Failed to get total items count for time range", zap.Time("start", start), zap.Time("end", end), zap.Error(err))
	}

	categoryCounts := make(map[string]int)
	var feedText string
	for i, it := range items {
		categoryCounts[it.AICategory]++
		feedText += fmt.Sprintf("【资讯 #%d】\n", i+1)
		feedText += fmt.Sprintf("标题: %s\n", it.Title)
		feedText += fmt.Sprintf("分类: %s (%s)\n", it.AICategory, it.AISubcategory)
		feedText += fmt.Sprintf("来源: %s\n", it.OriginSource)
		feedText += fmt.Sprintf("评分: %d/10\n", it.QualityScore)
		feedText += fmt.Sprintf("AI摘要: %s\n", it.AISummary)
		feedText += fmt.Sprintf("推荐理由: %s\n", it.AIComment)
		feedText += fmt.Sprintf("链接: %s\n\n", it.URL)
	}

	catJSON, err := json.Marshal(categoryCounts)
	if err != nil {
		catJSON = []byte("{}")
	}

	typeLabel := "日报"
	switch reportType {
	case "morning":
		typeLabel = "早报"
	case "evening":
		typeLabel = "晚报"
	}
	title := fmt.Sprintf("Grabby AI %s · %s", typeLabel, dateStr)

	if len(items) == 0 {
		notice := fmt.Sprintf("# %s\n\n本次时间范围内共抓取 %d 条资讯。未筛选出评分高于 %d 分的优质内容，故无推荐要闻。", title, totalItems, threshold)
		report := domainai.AIDailyReport{
			ReportDate:        dateStr,
			ReportType:        reportType,
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
			return nil, fmt.Errorf("failed to save empty ranged report: %w", err)
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
	dailyPrompt = enforceDailyReportJSONContract(dailyPrompt)

	prompt := dailyPrompt
	prompt = strings.ReplaceAll(prompt, "{{.Count}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.FeedText}}", feedText)
	prompt = strings.ReplaceAll(prompt, "{{.TotalItems}}", fmt.Sprintf("%d", totalItems))
	prompt = strings.ReplaceAll(prompt, "{{.QualityItems}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.Date}}", dateStr)

	var content string
	maxAttempts := selector.EnabledCount()
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		profile := selector.Next()
		if profile == nil {
			return nil, fmt.Errorf("no AI profile available for ranged report")
		}
		pc, ok := clients[profile.ID]
		if !ok {
			selector.MarkUnhealthy(profile.ID)
			continue
		}
		reportCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		content, err = adm.generateValidatedDailyContent(reportCtx, pc, prompt, dateStr)
		cancel()
		if err != nil {
			adm.logger.Warn("Profile failed for ranged report, trying next",
				zap.String("profile", profile.Name), zap.Error(err))
			selector.MarkUnhealthy(profile.ID)
			continue
		}
		selector.MarkHealthy(profile.ID)
		break
	}
	if content == "" {
		return nil, fmt.Errorf("all AI profiles failed to generate ranged report")
	}

	report := domainai.AIDailyReport{
		ReportDate:        dateStr,
		ReportType:        reportType,
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
		return nil, fmt.Errorf("failed to save generated ranged report: %w", err)
	}

	adm.logger.Info("Successfully generated ranged report",
		zap.String("date", dateStr),
		zap.String("type", reportType),
		zap.Int("items_included", len(items)))
	return &report, nil
}

func (adm *AIDailyManager) generateValidatedDailyContent(ctx context.Context, pc *profileClient, prompt string, dateStr string) (string, error) {
	const maxValidationAttempts = 3

	currentPrompt := prompt
	var lastErr error
	for attempt := 1; attempt <= maxValidationAttempts; attempt++ {
		rawContent, err := adm.aiEngine.callDailyProfile(ctx, pc, currentPrompt)
		if err != nil {
			return "", err
		}

		content, err := normalizeDailyReportContent(rawContent, dateStr)
		if err == nil {
			if attempt > 1 {
				adm.logger.Info("Daily report passed validation after repair",
					zap.Int("attempt", attempt),
					zap.String("date", dateStr))
			}
			return content, nil
		}

		lastErr = err
		adm.logger.Warn("Daily report failed validation",
			zap.Int("attempt", attempt),
			zap.String("date", dateStr),
			zap.Error(err))
		currentPrompt = buildDailyReportRepairPrompt(rawContent, err, dateStr)
	}

	return "", fmt.Errorf("daily report response failed validation after %d attempts: %w", maxValidationAttempts, lastErr)
}

func buildDailyReportRepairPrompt(rawContent string, validationErr error, dateStr string) string {
	if len(rawContent) > 12000 {
		rawContent = rawContent[:12000] + "\n...(truncated)"
	}
	return fmt.Sprintf(`你刚才生成的 Grabby AI 日报 JSON 没有通过格式校验。

校验错误：%s

请只修复格式，不要新增事实，不要改写含义。必须返回一个合法 JSON 对象，不能使用 Markdown 代码块，不能输出任何解释文字。

必需结构：
{
  "title": "string",
  "date": "%s",
  "editor": "string",
  "sections": {
    "headline": {
      "title": "string",
      "items": [
        {
          "title": "string",
          "summary": "string",
          "source": "string",
          "link": "string",
          "score": "string",
          "comment": "string"
        }
      ]
    }
  }
}

待修复内容：
%s`, validationErr, dateStr, rawContent)
}

func enforceDailyReportJSONContract(prompt string) string {
	if strings.Contains(prompt, "只能返回合法 JSON") && strings.Contains(prompt, `"sections"`) {
		return prompt
	}
	return prompt + `

【强制格式约束】：
无论前文如何描述，最终只能返回一个合法 JSON 对象，不能使用 Markdown 代码块，不能输出解释文字。JSON 顶层必须包含 title、date、editor、sections；sections 必须是对象；每个 section 必须包含 title 和 items；每个 item 至少包含 title 和 summary。`
}
