package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
	"go-server/internal/infrastructure/sqlite"
)

const (
	// dailyCandidateLimit bounds how many qualified items reach the editor
	// model. A day routinely has 200+ items above the score threshold; feeding
	// all of them makes the model treat the digest as a list of high scores
	// instead of a selection of must-reads.
	dailyCandidateLimit = 40
	// dailyCandidatePerCategory keeps one busy category (usually tech/AI) from
	// crowding every other category out of the candidate pool.
	dailyCandidatePerCategory = 12
	// maxDailyReportItems is the hard ceiling for a report body, dashboard
	// excluded. Enforced after generation, not just asked for in the prompt.
	maxDailyReportItems = 20
	// dailyDashboardSection is a stats panel, not editorial content, and does
	// not count against the body budget.
	dailyDashboardSection = "dashboard"
)

// errDailyReportOverBudget is returned by normalization when the model lists
// more items than the report body allows.
var errDailyReportOverBudget = errors.New("daily report exceeds item budget")

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

	candidates := selectDailyCandidates(items)
	categoryCounts := dailyCategoryCounts(candidates)
	feedText := buildDailyFeedText(candidates)
	adm.logger.Info("Daily report candidates selected",
		zap.String("date", dateStr),
		zap.Int("qualified", len(items)),
		zap.Int("candidates", len(candidates)))

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
	prompt = strings.ReplaceAll(prompt, "{{.Count}}", fmt.Sprintf("%d", len(candidates)))
	prompt = strings.ReplaceAll(prompt, "{{.FeedText}}", feedText)
	prompt = strings.ReplaceAll(prompt, "{{.TotalItems}}", fmt.Sprintf("%d", totalItems))
	prompt = strings.ReplaceAll(prompt, "{{.QualityItems}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.Date}}", dateStr)

	// Generate structured JSON via selector (with failover across profile+model slots)
	var content string
	usedModel := ""
	maxAttempts := selector.EnabledCount()
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	// A model list can be long; bound the total retry time so one bad provider
	// cannot stall report generation indefinitely.
	retryDeadline := time.Now().Add(20 * time.Minute)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if time.Now().After(retryDeadline) {
			break
		}
		sel := selector.Next()
		if sel == nil {
			return nil, fmt.Errorf("no AI profile available for daily report")
		}
		pc, ok := clients[sel.Profile.ID]
		if !ok {
			selector.MarkUnhealthy(sel.Key())
			continue
		}
		reportCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		content, err = adm.generateValidatedDailyContent(reportCtx, pc, sel.Model, prompt, dateStr)
		cancel()
		if err != nil {
			adm.logger.Warn("Model failed for daily report, trying next",
				zap.String("profile", sel.Profile.Name),
				zap.String("model", sel.Model),
				zap.Error(err))
			selector.MarkUnhealthy(sel.Key())
			continue
		}
		selector.MarkHealthy(sel.Key())
		usedModel = sel.Model
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
		ModelUsed:         usedModel,
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

	candidates := selectDailyCandidates(items)
	categoryCounts := dailyCategoryCounts(candidates)
	feedText := buildDailyFeedText(candidates)
	adm.logger.Info("Daily report candidates selected",
		zap.String("date", dateStr),
		zap.Int("qualified", len(items)),
		zap.Int("candidates", len(candidates)))

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
	prompt = strings.ReplaceAll(prompt, "{{.Count}}", fmt.Sprintf("%d", len(candidates)))
	prompt = strings.ReplaceAll(prompt, "{{.FeedText}}", feedText)
	prompt = strings.ReplaceAll(prompt, "{{.TotalItems}}", fmt.Sprintf("%d", totalItems))
	prompt = strings.ReplaceAll(prompt, "{{.QualityItems}}", fmt.Sprintf("%d", len(items)))
	prompt = strings.ReplaceAll(prompt, "{{.Date}}", dateStr)

	var content string
	usedModel := ""
	maxAttempts := selector.EnabledCount()
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	retryDeadline := time.Now().Add(20 * time.Minute)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if time.Now().After(retryDeadline) {
			break
		}
		sel := selector.Next()
		if sel == nil {
			return nil, fmt.Errorf("no AI profile available for ranged report")
		}
		pc, ok := clients[sel.Profile.ID]
		if !ok {
			selector.MarkUnhealthy(sel.Key())
			continue
		}
		reportCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		content, err = adm.generateValidatedDailyContent(reportCtx, pc, sel.Model, prompt, dateStr)
		cancel()
		if err != nil {
			adm.logger.Warn("Model failed for ranged report, trying next",
				zap.String("profile", sel.Profile.Name),
				zap.String("model", sel.Model),
				zap.Error(err))
			selector.MarkUnhealthy(sel.Key())
			continue
		}
		selector.MarkHealthy(sel.Key())
		usedModel = sel.Model
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
		ModelUsed:         usedModel,
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

func (adm *AIDailyManager) generateValidatedDailyContent(ctx context.Context, pc *profileClient, model, prompt string, dateStr string) (string, error) {
	const maxValidationAttempts = 3

	currentPrompt := prompt
	var lastErr error
	for attempt := 1; attempt <= maxValidationAttempts; attempt++ {
		rawContent, err := adm.aiEngine.callDailyProfile(ctx, pc, model, currentPrompt)
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
		if attempt == maxValidationAttempts && errors.Is(err, errDailyReportOverBudget) {
			// A model that keeps ignoring the budget still must not flood the
			// day's report: trim the lowest-scored items deterministically.
			if trimmed, trimErr := normalizeDailyReportContentOverBudget(rawContent, dateStr); trimErr == nil {
				adm.logger.Warn("Daily report still over budget after repair, trimmed deterministically",
					zap.String("date", dateStr),
					zap.Int("max_items", maxDailyReportItems))
				return trimmed, nil
			} else {
				lastErr = trimErr
			}
		}
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
	fixInstruction := "请只修复格式，不要新增事实，不要改写含义。"
	if errors.Is(validationErr, errDailyReportOverBudget) {
		fixInstruction = fmt.Sprintf(
			"正文条目超过了 %d 条上限。请删除重要性最低、重复或信息增量不足的条目，直到正文条目不超过 %d 条；保留条目的措辞不要改动。",
			maxDailyReportItems, maxDailyReportItems)
	}
	return fmt.Sprintf(`你刚才生成的 Grabby AI 日报 JSON 没有通过校验。

校验错误：%s

%s必须返回一个合法 JSON 对象，不能使用 Markdown 代码块，不能输出任何解释文字。

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
%s`, validationErr, fixInstruction, dateStr, rawContent)
}

func enforceDailyReportJSONContract(prompt string) string {
	if strings.Contains(prompt, "只能返回合法 JSON") && strings.Contains(prompt, `"sections"`) {
		return prompt
	}
	return prompt + `

【强制格式约束】：
无论前文如何描述，最终只能返回一个合法 JSON 对象，不能使用 Markdown 代码块，不能输出解释文字。JSON 顶层必须包含 title、date、editor、sections；sections 必须是对象；每个 section 必须包含 title 和 items；每个 item 至少包含 title 和 summary。`
}

// selectDailyCandidates turns the day's qualified items into the bounded,
// de-duplicated shortlist the editor model chooses from. Items arrive ordered
// by score, so the shortlist keeps the strongest material while the per-
// category cap reserves room for other topics.
func selectDailyCandidates(items []sqlite.ScrapedItemWithAI) []sqlite.ScrapedItemWithAI {
	seenURL := make(map[string]struct{}, len(items))
	seenTitle := make(map[string]struct{}, len(items))
	perCategory := make(map[string]int)
	candidates := make([]sqlite.ScrapedItemWithAI, 0, dailyCandidateLimit)
	for _, item := range items {
		if len(candidates) >= dailyCandidateLimit {
			break
		}
		if url := strings.TrimSpace(item.URL); url != "" {
			if _, seen := seenURL[url]; seen {
				continue
			}
			seenURL[url] = struct{}{}
		}
		if titleKey := dailyTitleKey(item.Title); titleKey != "" {
			if _, seen := seenTitle[titleKey]; seen {
				continue
			}
			seenTitle[titleKey] = struct{}{}
		}
		category := dailyItemCategory(item)
		if perCategory[category] >= dailyCandidatePerCategory {
			continue
		}
		perCategory[category]++
		candidates = append(candidates, item)
	}
	return candidates
}

// dailyTitleKey folds a title down to letters and digits so syndicated copies
// of the same story collapse whether or not punctuation or spacing differs.
func dailyTitleKey(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dailyItemCategory(item sqlite.ScrapedItemWithAI) string {
	if category := strings.TrimSpace(item.AICategory); category != "" {
		return category
	}
	return "其他"
}

func dailyCategoryCounts(items []sqlite.ScrapedItemWithAI) map[string]int {
	counts := make(map[string]int, 8)
	for _, item := range items {
		counts[dailyItemCategory(item)]++
	}
	return counts
}

func buildDailyFeedText(items []sqlite.ScrapedItemWithAI) string {
	var b strings.Builder
	for i, item := range items {
		fmt.Fprintf(&b, "【资讯 #%d】\n", i+1)
		fmt.Fprintf(&b, "标题: %s\n", item.Title)
		fmt.Fprintf(&b, "分类: %s (%s)\n", item.AICategory, item.AISubcategory)
		fmt.Fprintf(&b, "来源: %s\n", item.OriginSource)
		if item.PublishedAt != nil {
			fmt.Fprintf(&b, "发布时间: %s\n", item.PublishedAt.Format("2006-01-02 15:04"))
		}
		fmt.Fprintf(&b, "评分: %d/10\n", item.QualityScore)
		fmt.Fprintf(&b, "AI摘要: %s\n", item.AISummary)
		fmt.Fprintf(&b, "推荐理由: %s\n", item.AIComment)
		fmt.Fprintf(&b, "链接: %s\n\n", item.URL)
	}
	return b.String()
}
