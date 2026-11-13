package httpiface

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/application/render"
	"go-server/internal/domain/ai"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/catalog"
	"go-server/internal/infrastructure/configimport"
	"go-server/internal/infrastructure/modeldiscovery"
	"go-server/internal/infrastructure/sqlite"
)

// AIHandlers encapsulates all AI HTTP handlers.
type AIHandlers struct {
	db           *sqlite.Database
	aiEngine     *appai.AIEngine
	dailyManager *appai.AIDailyManager
	catalog      *catalog.Service
	logger       *zap.Logger
}

// NewAIHandlers creates a new AIHandlers instance.
func NewAIHandlers(db *sqlite.Database, aiEngine *appai.AIEngine, dailyManager *appai.AIDailyManager, catalogSvc *catalog.Service, logger *zap.Logger) *AIHandlers {
	return &AIHandlers{
		db:           db,
		aiEngine:     aiEngine,
		dailyManager: dailyManager,
		catalog:      catalogSvc,
		logger:       logger,
	}
}

// aiErr writes the AI envelope error body `{"success":false,"error":...}`.
func aiErr(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]any{"success": false, "error": msg})
}

// HandleQuality retrieves quality items filtering by AI scores.
func (h *AIHandlers) HandleQuality(c echo.Context) error {
	scoreMin := h.aiEngine.Settings().QualityThreshold
	if s, err := strconv.Atoi(c.QueryParam("score_min")); err == nil {
		scoreMin = s
	}

	days := 7
	if d, err := strconv.Atoi(c.QueryParam("days")); err == nil {
		days = d
	}

	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
		limit = l
	}

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(item.AIItemsFilter{
		AICategory:     c.QueryParam("category"),
		SourceCategory: c.QueryParam("source_category"),
		ScoreMin:       scoreMin,
		Days:           days,
		Limit:          limit,
		Cursor:         c.QueryParam("cursor"),
	})
	if err != nil {
		h.logger.Error("Failed to query quality items", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":     true,
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// HandleCategories gets all AI semantic categories stats.
func (h *AIHandlers) HandleCategories(c echo.Context) error {
	stats, err := h.db.GetAICategories()
	if err != nil {
		h.logger.Error("Failed to query AI categories", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":    true,
		"categories": stats,
	})
}

// HandleItems gets items filtered by AI category.
func (h *AIHandlers) HandleItems(c echo.Context) error {
	scoreMin := 0
	if s, err := strconv.Atoi(c.QueryParam("score_min")); err == nil {
		scoreMin = s
	}

	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
		limit = l
	}

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(item.AIItemsFilter{
		AICategory:     c.QueryParam("category"),
		SourceCategory: c.QueryParam("source_category"),
		ScoreMin:       scoreMin,
		Limit:          limit,
		Cursor:         c.QueryParam("cursor"),
	})
	if err != nil {
		h.logger.Error("Failed to query AI items", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":     true,
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// HandleAnalysis retrieves details of an AI analysis for a specific item.
func (h *AIHandlers) HandleAnalysis(c echo.Context) error {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid ID format")
	}

	analysis, err := h.db.GetAIAnalysis(itemID)
	if err != nil {
		h.logger.Error("Failed to query AI analysis", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}
	if analysis == nil {
		return aiErr(c, http.StatusNotFound, "Analysis not found")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":  true,
		"analysis": analysis,
	})
}

// HandleDaily retrieves the AI Daily Report for a date and type.
func (h *AIHandlers) HandleDaily(c echo.Context) error {
	dateStr := c.QueryParam("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	reportType := c.QueryParam("type")
	if reportType == "" {
		reportType = "daily"
	}

	report, err := h.db.GetAIDailyReport(dateStr, reportType)
	if err != nil {
		h.logger.Error("Failed to query daily report", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	if report == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"success":     true,
			"report_date": dateStr,
			"report_type": reportType,
		})
	}

	// Older reports are stored as Markdown prose, newer ones as a JSON document
	// with per-section items. Support both instead of rejecting the Markdown
	// ones with a 500.
	var content map[string]any
	if err := json.Unmarshal([]byte(report.Content), &content); err != nil {
		content = map[string]any{
			"format":  "markdown",
			"content": report.Content,
		}
	} else {
		content["format"] = "json"
	}

	// Markdown reports get a server-rendered, sanitized HTML body so the
	// desktop reader (and a future native client) does not need its own
	// Markdown/HTML renderer.
	if format, _ := content["format"].(string); format == "markdown" {
		if text, ok := content["content"].(string); ok {
			content["html_content"] = render.Render(text)
		}
	}

	content["success"] = true
	content["id"] = report.ID
	content["report_date"] = report.ReportDate
	content["report_type"] = report.ReportType
	content["total_items"] = report.TotalItems
	content["quality_items"] = report.QualityItems
	content["categories_summary"] = report.CategoriesSummary
	content["model_used"] = report.ModelUsed
	content["generated_at"] = report.GeneratedAt

	return c.JSON(http.StatusOK, content)
}

// HandleDailyList retrieves a list of recent daily reports.
func (h *AIHandlers) HandleDailyList(c echo.Context) error {
	limit := 10
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
		limit = l
	}

	reports, err := h.db.GetAIDailyReports(limit, c.QueryParam("type"))
	if err != nil {
		h.logger.Error("Failed to query daily reports list", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"reports": reports,
	})
}

// HandleDailyGenerate manually triggers a daily report generation.
func (h *AIHandlers) HandleDailyGenerate(c echo.Context) error {
	var req struct {
		Date       string `json:"date"`
		ReportType string `json:"report_type"`
	}
	_ = c.Bind(&req)

	if req.Date == "" {
		req.Date = c.QueryParam("date")
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	if req.ReportType == "" {
		req.ReportType = "daily"
	}

	go func() {
		ctx := context.Background()
		var genErr error

		if req.ReportType == "morning" || req.ReportType == "evening" {
			settings := h.aiEngine.Settings()
			targetDate, err := time.ParseInLocation("2006-01-02", req.Date, time.Local)
			if err != nil {
				targetDate = time.Now()
			}
			var start, end time.Time

			if req.ReportType == "morning" {
				mt := settings.MorningReportTime
				if mt == "" {
					mt = "08:30"
				}
				var hour, min int
				fmt.Sscanf(mt[:2], "%d", &hour)
				if len(mt) >= 5 {
					fmt.Sscanf(mt[3:5], "%d", &min)
				}
				end = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), hour, min, 0, 0, targetDate.Location())
				start = end.Add(-24 * time.Hour)
			} else {
				et := settings.EveningReportTime
				if et == "" {
					et = "22:30"
				}
				var eHour, eMin int
				fmt.Sscanf(et[:2], "%d", &eHour)
				if len(et) >= 5 {
					fmt.Sscanf(et[3:5], "%d", &eMin)
				}
				end = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), eHour, eMin, 0, 0, targetDate.Location())
				start = end.Add(-14 * time.Hour)
			}

			_, genErr = h.dailyManager.GenerateRangedReport(ctx, req.Date, req.ReportType, start, end)
		} else {
			_, genErr = h.dailyManager.GenerateDailyReport(ctx, req.Date)
		}

		if genErr != nil {
			h.logger.Error("Async background generation failed", zap.String("date", req.Date), zap.String("type", req.ReportType), zap.Error(genErr))
		} else {
			h.logger.Info("Async background generation completed successfully", zap.String("date", req.Date), zap.String("type", req.ReportType))
		}
	}()

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "Report generation started in the background",
	})
}

// HandleDailyRSS generates an RSS 2.0 feed of recent daily reports.
func (h *AIHandlers) HandleDailyRSS(c echo.Context) error {
	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}

	reports, err := h.db.GetAIDailyReports(limit, "")
	if err != nil {
		h.logger.Error("Failed to query reports for RSS", zap.Error(err))
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	r := c.Request()
	baseURL := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		baseURL += "s"
	}
	baseURL += "://" + r.Host

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">`)
	buf.WriteString("\n<channel>\n")
	buf.WriteString("<title>Grabby AI 智能日报</title>\n")
	buf.WriteString("<link>" + xmlEscape(baseURL) + "</link>\n")
	buf.WriteString("<description>Grabby AI 自动生成的智能早晚报和日报 RSS 订阅</description>\n")
	buf.WriteString("<language>zh-cn</language>\n")
	if len(reports) > 0 {
		buf.WriteString("<lastBuildDate>" + reports[0].GeneratedAt.Format(time.RFC1123Z) + "</lastBuildDate>\n")
	}
	buf.WriteString(fmt.Sprintf(`<atom:link href="%s/open/api/ai/daily/rss" rel="self" type="application/rss+xml"/>`, baseURL))
	buf.WriteString("\n")

	for _, rpt := range reports {
		htmlContent := render.Render(rpt.Content)

		typeLabel := "日报"
		switch rpt.ReportType {
		case "morning":
			typeLabel = "早报"
		case "evening":
			typeLabel = "晚报"
		}

		link := fmt.Sprintf("%s/#daily?date=%s&type=%s", baseURL, rpt.ReportDate, rpt.ReportType)
		pubDate := rpt.GeneratedAt.Format(time.RFC1123Z)

		buf.WriteString("<item>\n")
		buf.WriteString("<title>" + xmlEscape(rpt.Title) + "</title>\n")
		buf.WriteString("<link>" + xmlEscape(link) + "</link>\n")
		buf.WriteString("<guid isPermaLink=\"false\">grabby-daily-" + xmlEscape(rpt.ReportDate) + "-" + xmlEscape(rpt.ReportType) + "</guid>\n")
		buf.WriteString("<pubDate>" + pubDate + "</pubDate>\n")
		buf.WriteString("<category>" + xmlEscape(typeLabel) + "</category>\n")
		buf.WriteString("<description>" + html.EscapeString(fmt.Sprintf("优质内容 %d 条 / 共处理 %d 条", rpt.QualityItems, rpt.TotalItems)) + "</description>\n")
		buf.WriteString("<content:encoded xmlns:content=\"http://purl.org/rss/1.0/modules/content/\">" + xmlCdata(htmlContent) + "</content:encoded>\n")
		buf.WriteString("</item>\n")
	}

	buf.WriteString("</channel>\n</rss>")

	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.Blob(http.StatusOK, "application/rss+xml; charset=utf-8", buf.Bytes())
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func xmlCdata(s string) string {
	s = strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
	return "<![CDATA[" + s + "]]>"
}

// HandleReanalyze reanalyzes an item synchronously.
func (h *AIHandlers) HandleReanalyze(c echo.Context) error {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid ID format")
	}

	if err := h.aiEngine.AnalyzeItem(itemID); err != nil {
		h.logger.Error("Manual AI analysis failed", zap.Int64("item_id", itemID), zap.Error(err))
		return aiErr(c, http.StatusInternalServerError, err.Error())
	}

	analysis, err := h.db.GetAIAnalysis(itemID)
	if err != nil {
		h.logger.Error("Failed to get updated analysis", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":  true,
		"analysis": analysis,
	})
}

// HandleStats returns AI processing pipeline statistics.
func (h *AIHandlers) HandleStats(c echo.Context) error {
	totalProcessed, err := h.db.CountAIAnalyses()
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	totalPending, err := h.db.CountUnprocessedAIItems()
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}

	avgScore, _ := h.db.AverageAIQualityScore()

	return c.JSON(http.StatusOK, map[string]any{
		"success":         true,
		"total_processed": totalProcessed,
		"total_pending":   totalPending,
		"average_score":   avgScore,
		"queue_length":    h.aiEngine.QueueLength(),
	})
}

// HandleGetSettings retrieves the current AI settings.
func (h *AIHandlers) HandleGetSettings(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"success":  true,
		"settings": h.aiEngine.Settings(),
	})
}

// HandleSaveSettings updates the AI settings.
func (h *AIHandlers) HandleSaveSettings(c echo.Context) error {
	var req ai.AISettings
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to decode settings request body", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
	}

	if err := h.persistSettings(req); err != nil {
		h.logger.Error("Failed to save AI settings", zap.Error(err))
		return aiErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

// persistSettings normalizes, stores and reloads one AI settings document. It is
// shared by the settings endpoint and the config importer.
func (h *AIHandlers) persistSettings(req ai.AISettings) error {
	return appai.PersistSettings(h.db, h.aiEngine, req)
}

// HandleTestConnection tests the AI connection. When the request carries a
// draft `profile`, the test runs against that unsaved configuration without
// touching the database — this lets the settings UI verify a connection before
// the user commits it. Without a profile it falls back to the legacy behaviour
// of analysing a real scraped item with the saved engine settings.
func (h *AIHandlers) HandleTestConnection(c echo.Context) error {
	var testReq struct {
		Profile      *ai.AIProviderProfile `json:"profile"`
		Model        string                `json:"model"`
		SystemPrompt string                `json:"system_prompt"`
	}
	_ = c.Bind(&testReq)

	if testReq.Profile != nil {
		profile := *testReq.Profile
		// A profile may hold several models; the UI tests one concrete model.
		// Explicit `model` wins, otherwise the primary (first) model is used.
		if strings.TrimSpace(testReq.Model) != "" {
			profile.Model = testReq.Model
		}
		if strings.TrimSpace(profile.Model) == "" {
			if models := profile.EffectiveModels(); len(models) > 0 {
				profile.Model = models[0]
			}
		}
		if strings.TrimSpace(profile.Model) == "" {
			return aiErr(c, http.StatusBadRequest, "请先选择或填写模型 ID。")
		}
		analysis, latency, err := h.aiEngine.TestProfile(c.Request().Context(), profile, testReq.SystemPrompt)
		if err != nil {
			h.logger.Warn("Draft AI profile connection test failed", zap.Error(err))
			return aiErr(c, http.StatusBadGateway, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]any{
			"success":    true,
			"item_title": "Grabby 连通性样本",
			"analysis":   analysis,
			"latency_ms": latency,
			"model_used": profile.Model,
		})
	}

	if !h.aiEngine.IsEnabled() {
		return aiErr(c, http.StatusBadRequest, "AI 引擎未启用，请先开启 '启用 AI 语义分析与评分' 开关并保存配置。")
	}

	items, err := h.db.GetUnanalyzedItems(1)
	var testItemID int64
	var testItemTitle string

	if err == nil && len(items) > 0 {
		testItemID = items[0].ID
		testItemTitle = items[0].Title
	} else {
		testItemID, testItemTitle, err = h.db.GetLatestScrapedItem()
		if err != nil {
			testItemID = 0
		}
	}

	if testItemID == 0 {
		h.logger.Info("No scraped items found in DB. Inserting mock item for AI test.")
		mockItem := item.ScrapedItem{
			SourceID:     "ai-test-source",
			OriginSource: "System Test",
			Title:        "测试文章 - 验证 AI 模型连通性",
			URL:          "https://example.com/ai-test-connection-mock-url",
			Summary:      "这是一篇用于验证 AI 接口与模型连通性的测试文章。",
			Content:      "今天我们在这里进行 Grabby AI 语义评估连通性的测试。系统将会尝试连接大语言模型并根据自定义的 System Prompt 对本正文进行分类打分。如果连接成功，您将在前端设置页面看到包含评分、分类和简短摘要的结果反馈。",
			Category:     "article",
		}

		_ = h.db.InsertSource(source.Source{
			ID:              "ai-test-source",
			Name:            "系统测试源",
			Type:            "api",
			URL:             "https://example.com/ai-test-connection-mock-url",
			Schedule:        "0 0 * * *",
			Enabled:         0,
			DefaultCategory: "auto",
			Config:          "{}",
		})

		if _, err = h.db.InsertScrapedItem(mockItem); err != nil {
			h.logger.Error("Failed to insert mock item for test", zap.Error(err))
			return aiErr(c, http.StatusInternalServerError, fmt.Sprintf("写入测试数据失败: %v", err))
		}

		testItemID, testItemTitle, err = h.db.GetScrapedItemByURL(mockItem.URL)
		if err != nil {
			testItemID = 0
		}
	}

	if testItemID == 0 {
		return aiErr(c, http.StatusInternalServerError, "无法创建或获取测试数据项，请手动添加数据源抓取部分内容后重试。")
	}

	h.logger.Info("Running synchronous AI test on item", zap.Int64("item_id", testItemID), zap.String("title", testItemTitle))

	if err := h.aiEngine.AnalyzeItem(testItemID); err != nil {
		h.logger.Error("AI test analysis failed", zap.Error(err))
		return aiErr(c, http.StatusInternalServerError, fmt.Sprintf("AI 接口连接失败: %v", err))
	}

	analysis, err := h.db.GetAIAnalysis(testItemID)
	if err != nil || analysis == nil {
		return aiErr(c, http.StatusInternalServerError, "AI 分析已执行，但读取结果记录失败。")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":    true,
		"item_title": testItemTitle,
		"analysis":   analysis,
	})
}

// HandleStartEvaluation enqueues all unanalyzed items for processing.
func (h *AIHandlers) HandleStartEvaluation(c echo.Context) error {
	if !h.aiEngine.IsEnabled() {
		return aiErr(c, http.StatusBadRequest, "AI 引擎未启用，请先开启 AI 评估功能并保存配置。")
	}

	go h.aiEngine.RunBackfill()

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"message": "AI 评测队列已启动，正在后台增量评估未分析的文章。",
	})
}

// HandleCatalog returns the normalized models.dev provider/model catalog. The
// settings UI uses it to replace the hand-maintained preset table; `refresh=1`
// forces a network refresh before responding.
func (h *AIHandlers) HandleCatalog(c echo.Context) error {
	if h.catalog == nil {
		return aiErr(c, http.StatusServiceUnavailable, "模型目录服务未初始化")
	}
	refresh := c.QueryParam("refresh") == "1" || strings.EqualFold(c.QueryParam("refresh"), "true")
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	snapshot, err := h.catalog.Get(ctx, refresh)
	if err != nil {
		h.logger.Warn("AI catalog request failed", zap.Error(err))
		return aiErr(c, http.StatusBadGateway, "模型目录暂时不可用："+err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success":    true,
		"source":     snapshot.Source,
		"fetched_at": snapshot.FetchedAt,
		"count":      len(snapshot.Providers),
		"providers":  snapshot.Providers,
	})
}

// HandleDiscoverModels lists the models exposed by a provider endpoint. The
// request may carry a draft provider/base_url/api_key (for an unsaved form) or
// a profile_id, in which case the saved key is reused so the user never has to
// paste it again.
func (h *AIHandlers) HandleDiscoverModels(c echo.Context) error {
	var req struct {
		Provider  string `json:"provider"`
		BaseURL   string `json:"base_url"`
		APIKey    string `json:"api_key"`
		ProfileID string `json:"profile_id"`
	}
	if err := c.Bind(&req); err != nil {
		return aiErr(c, http.StatusBadRequest, "无效的请求体")
	}

	provider := strings.TrimSpace(req.Provider)
	baseURL := strings.TrimSpace(req.BaseURL)
	apiKey := strings.TrimSpace(req.APIKey)

	if provider == "" || baseURL == "" || apiKey == "" {
		settings := h.aiEngine.Settings()
		profileID := strings.TrimSpace(req.ProfileID)
		if profileID == "" {
			profileID = settings.ActiveProfileID
		}
		for _, profile := range settings.Profiles {
			if profile.ID != profileID {
				continue
			}
			if provider == "" {
				provider = profile.Provider
			}
			if baseURL == "" {
				baseURL = profile.BaseURL
			}
			if apiKey == "" {
				apiKey = profile.APIKey
			}
			break
		}
	}

	if provider == "" {
		return aiErr(c, http.StatusBadRequest, "请先选择服务商")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 40*time.Second)
	defer cancel()
	models, err := modeldiscovery.Discover(ctx, provider, baseURL, apiKey)
	if err != nil {
		h.logger.Warn("Model discovery failed",
			zap.String("provider", provider),
			zap.String("base_url", baseURL),
			zap.Error(err))
		return aiErr(c, http.StatusBadGateway, "获取模型列表失败："+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":  true,
		"provider": provider,
		"base_url": baseURL,
		"count":    len(models),
		"models":   models,
	})
}

// HandleUsage returns the token/latency aggregation for recent model calls.
func (h *AIHandlers) HandleUsage(c echo.Context) error {
	days := 7
	if value, err := strconv.Atoi(c.QueryParam("days")); err == nil && value > 0 {
		days = value
	}
	summary, err := h.db.GetAIUsageSummary(days)
	if err != nil {
		h.logger.Error("Failed to query AI usage", zap.Error(err))
		return aiErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"summary": summary,
	})
}

// HandleImportConfig previews or applies provider configurations discovered in
// Claude Code, Codex, OpenCode and cc-switch. Source files are only read.
func (h *AIHandlers) HandleImportConfig(c echo.Context) error {
	var req struct {
		Source string `json:"source"`
		Apply  bool   `json:"apply"`
	}
	_ = c.Bind(&req)

	result := configimport.Import(req.Source)
	imported := 0

	if req.Apply && len(result.Connections) > 0 {
		settings := h.aiEngine.Settings()
		existing := make(map[string]struct{}, len(settings.Profiles))
		for _, profile := range settings.Profiles {
			existing[profileKey(profile)] = struct{}{}
		}

		firstImportedID := ""
		for i, connection := range result.Connections {
			profile := profileFromConnection(connection, len(settings.Profiles)+i+1)
			key := profileKey(profile)
			if _, ok := existing[key]; ok {
				continue
			}
			existing[key] = struct{}{}
			settings.Profiles = append(settings.Profiles, profile)
			if firstImportedID == "" {
				firstImportedID = profile.ID
			}
			imported++
		}

		if imported > 0 {
			// If the app is still on the generated placeholder profile, make
			// the first imported connection immediately usable.
			if settings.ActiveProfileID == "" || settings.ActiveProfileID == "default" || !profileExists(settings.Profiles, settings.ActiveProfileID) {
				settings.ActiveProfileID = firstImportedID
			}
			if err := h.persistSettings(settings); err != nil {
				h.logger.Error("Failed to apply imported AI config", zap.Error(err))
				return aiErr(c, http.StatusInternalServerError, err.Error())
			}
		}
	}

	// Never echo keys back to the client; the imported profiles already hold
	// them when apply=true.
	sanitized := make([]configimport.Connection, len(result.Connections))
	copy(sanitized, result.Connections)
	for i := range sanitized {
		sanitized[i].APIKey = ""
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":     true,
		"applied":     req.Apply,
		"imported":    imported,
		"sources":     result.Sources,
		"connections": sanitized,
	})
}

func profileKey(profile ai.AIProviderProfile) string {
	models := profile.EffectiveModels()
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(profile.Provider),
		strings.TrimRight(strings.TrimSpace(profile.BaseURL), "/"),
		strings.Join(models, ","),
	}, "|"))
}

func profileExists(profiles []ai.AIProviderProfile, id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func profileFromConnection(connection configimport.Connection, index int) ai.AIProviderProfile {
	name := strings.TrimSpace(connection.Name)
	if name == "" {
		name = connection.SourceName
	}
	provider := strings.TrimSpace(connection.Provider)
	if provider == "" {
		provider = "custom"
	}
	models := append([]string(nil), connection.Models...)
	if len(models) == 0 && strings.TrimSpace(connection.Model) != "" {
		models = []string{strings.TrimSpace(connection.Model)}
	}
	primary := ""
	if len(models) > 0 {
		primary = models[0]
	}
	return ai.AIProviderProfile{
		ID:                fmt.Sprintf("import-%s-%d", sanitizeID(connection.Source), index),
		Name:              name,
		Provider:          provider,
		APIKey:            connection.APIKey,
		Model:             primary,
		Models:            models,
		BaseURL:           strings.TrimSpace(connection.BaseURL),
		Priority:          index,
		RequestsPerMinute: 10,
	}
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "config"
	}
	return b.String()
}

// RegisterAIHandlers wires the AI read/generate/settings endpoints.
func RegisterAIHandlers(openAPI, api *echo.Group, deps Dependencies) {
	h := NewAIHandlers(deps.DB, deps.AIEngine, deps.DailyManager, deps.Catalog, deps.Logger)
	openAPI.GET("/ai/quality", h.HandleQuality)
	openAPI.GET("/ai/categories", h.HandleCategories)
	openAPI.GET("/ai/items", h.HandleItems)
	openAPI.GET("/ai/analysis/:id", h.HandleAnalysis)
	openAPI.GET("/ai/daily", h.HandleDaily)
	openAPI.GET("/ai/daily/list", h.HandleDailyList)
	openAPI.GET("/ai/daily/rss", h.HandleDailyRSS)
	openAPI.GET("/ai/stats", h.HandleStats)

	api.GET("/ai/catalog", h.HandleCatalog)
	api.POST("/ai/models/discover", h.HandleDiscoverModels)
	api.POST("/ai/import", h.HandleImportConfig)
	api.GET("/ai/usage", h.HandleUsage)
	api.POST("/ai/daily/generate", h.HandleDailyGenerate)
	api.POST("/ai/reanalyze/:id", h.HandleReanalyze)
	api.GET("/ai/settings", h.HandleGetSettings)
	api.POST("/ai/settings", h.HandleSaveSettings)
	api.POST("/ai/test", h.HandleTestConnection)
	api.POST("/ai/start_eval", h.HandleStartEvaluation)
}
