package httpiface

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"go.uber.org/zap"

	appai "go-server/internal/application/ai"
	"go-server/internal/domain/ai"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
	"go-server/internal/infrastructure/sqlite"
)

// AIHandlers encapsulates all AI HTTP handlers.
type AIHandlers struct {
	db           *sqlite.Database
	aiEngine     *appai.AIEngine
	dailyManager *appai.AIDailyManager
	logger       *zap.Logger
}

// NewAIHandlers creates a new AIHandlers instance.
func NewAIHandlers(db *sqlite.Database, aiEngine *appai.AIEngine, dailyManager *appai.AIDailyManager, logger *zap.Logger) *AIHandlers {
	return &AIHandlers{
		db:           db,
		aiEngine:     aiEngine,
		dailyManager: dailyManager,
		logger:       logger,
	}
}

// HandleQuality retrieves quality items filtering by AI scores.
func (h *AIHandlers) HandleQuality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scoreMin := h.aiEngine.Settings().QualityThreshold
	if sMinStr := r.URL.Query().Get("score_min"); sMinStr != "" {
		if s, err := strconv.Atoi(sMinStr); err == nil {
			scoreMin = s
		}
	}

	category := r.URL.Query().Get("category")

	days := 7
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil {
			days = d
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	cursor := r.URL.Query().Get("cursor")

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(item.AIItemsFilter{
		AICategory: category,
		ScoreMin:   scoreMin,
		Days:       days,
		Limit:      limit,
		Cursor:     cursor,
	})
	if err != nil {
		h.logger.Error("Failed to query quality items", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// HandleCategories gets all AI semantic categories stats.
func (h *AIHandlers) HandleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.db.GetAICategories()
	if err != nil {
		h.logger.Error("Failed to query AI categories", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"categories": stats,
	})
}

// HandleItems gets items filtered by AI category.
func (h *AIHandlers) HandleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.URL.Query().Get("category")

	scoreMin := 0
	if sMinStr := r.URL.Query().Get("score_min"); sMinStr != "" {
		if s, err := strconv.Atoi(sMinStr); err == nil {
			scoreMin = s
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	cursor := r.URL.Query().Get("cursor")

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(item.AIItemsFilter{
		AICategory: category,
		ScoreMin:   scoreMin,
		Limit:      limit,
		Cursor:     cursor,
	})
	if err != nil {
		h.logger.Error("Failed to query AI items", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// HandleAnalysis retrieves details of an AI analysis for a specific item.
func (h *AIHandlers) HandleAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	idStr := parts[4]
	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	analysis, err := h.db.GetAIAnalysis(itemID)
	if err != nil {
		h.logger.Error("Failed to query AI analysis", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if analysis == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Analysis not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"analysis": analysis,
	})
}

// HandleDaily retrieves the AI Daily Report for a date and type.
func (h *AIHandlers) HandleDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	reportType := r.URL.Query().Get("type")
	if reportType == "" {
		reportType = "daily"
	}

	report, err := h.db.GetAIDailyReport(dateStr, reportType)
	if err != nil {
		h.logger.Error("Failed to query daily report", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	htmlContent := ""
	if report != nil && report.Content != "" {
		if err := goldmark.Convert([]byte(report.Content), &buf); err == nil {
			htmlContent = buf.String()
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"report":       report,
		"html_content": htmlContent,
	})
}

// HandleDailyList retrieves a list of recent daily reports.
func (h *AIHandlers) HandleDailyList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	reportType := r.URL.Query().Get("type")

	reports, err := h.db.GetAIDailyReports(limit, reportType)
	if err != nil {
		h.logger.Error("Failed to query daily reports list", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"reports": reports,
	})
}

// HandleDailyGenerate manually triggers a daily report generation.
func (h *AIHandlers) HandleDailyGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Date       string `json:"date"`
		ReportType string `json:"report_type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Date == "" {
		req.Date = r.URL.Query().Get("date")
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	if req.ReportType == "" {
		req.ReportType = "daily"
	}

	var report *ai.AIDailyReport
	var genErr error

	if req.ReportType == "morning" || req.ReportType == "evening" {
		settings := h.aiEngine.Settings()
		now := time.Now()
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
			end = time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
			start = end.Add(-24 * time.Hour)
		} else {
			mt := settings.MorningReportTime
			if mt == "" {
				mt = "08:30"
			}
			var mHour, mMin int
			fmt.Sscanf(mt[:2], "%d", &mHour)
			if len(mt) >= 5 {
				fmt.Sscanf(mt[3:5], "%d", &mMin)
			}
			et := settings.EveningReportTime
			if et == "" {
				et = "22:30"
			}
			var eHour, eMin int
			fmt.Sscanf(et[:2], "%d", &eHour)
			if len(et) >= 5 {
				fmt.Sscanf(et[3:5], "%d", &eMin)
			}
			start = time.Date(now.Year(), now.Month(), now.Day(), mHour, mMin, 0, 0, now.Location())
			end = time.Date(now.Year(), now.Month(), now.Day(), eHour, eMin, 0, 0, now.Location())
		}

		report, genErr = h.dailyManager.GenerateRangedReport(r.Context(), req.Date, req.ReportType, start, end)
	} else {
		report, genErr = h.dailyManager.GenerateDailyReport(r.Context(), req.Date)
	}
	if genErr != nil {
		h.logger.Error("Failed to generate daily report manually", zap.Error(genErr))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   genErr.Error(),
		})
		return
	}

	var buf bytes.Buffer
	htmlContent := ""
	if report != nil && report.Content != "" {
		if err := goldmark.Convert([]byte(report.Content), &buf); err == nil {
			htmlContent = buf.String()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"report":       report,
		"html_content": htmlContent,
	})
}

// HandleDailyRSS generates an RSS 2.0 feed of recent daily reports.
func (h *AIHandlers) HandleDailyRSS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	reports, err := h.db.GetAIDailyReports(limit, "")
	if err != nil {
		h.logger.Error("Failed to query reports for RSS", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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
	buf.WriteString(fmt.Sprintf(`<atom:link href="%s/api/ai/daily/rss" rel="self" type="application/rss+xml"/>`, baseURL))
	buf.WriteString("\n")

	for _, rpt := range reports {
		var htmlBuf bytes.Buffer
		htmlContent := rpt.Content
		if err := goldmark.Convert([]byte(rpt.Content), &htmlBuf); err == nil {
			htmlContent = htmlBuf.String()
		}

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

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(buf.Bytes())
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
func (h *AIHandlers) HandleReanalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	idStr := parts[4]
	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	err = h.aiEngine.AnalyzeItem(itemID)
	if err != nil {
		h.logger.Error("Manual AI analysis failed", zap.Int64("item_id", itemID), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	analysis, err := h.db.GetAIAnalysis(itemID)
	if err != nil {
		h.logger.Error("Failed to get updated analysis", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"analysis": analysis,
	})
}

// HandleStats returns AI processing pipeline statistics.
func (h *AIHandlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	totalProcessed, err := h.db.CountAIAnalyses()
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	totalPending, err := h.db.CountUnprocessedAIItems()
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	avgScore, _ := h.db.AverageAIQualityScore()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"total_processed": totalProcessed,
		"total_pending":   totalPending,
		"average_score":   avgScore,
		"queue_length":    h.aiEngine.QueueLength(),
	})
}

// HandleGetSettings retrieves the current AI settings.
func (h *AIHandlers) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := h.aiEngine.Settings()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"settings": settings,
	})
}

// HandleSaveSettings updates the AI settings.
func (h *AIHandlers) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ai.AISettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode settings request body", zap.Error(err))
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Normalize settings
	req = ai.NormalizeAISettings(req)

	// 1. Save to SQLite database
	enabledStr := "false"
	if req.Enabled {
		enabledStr = "true"
	}
	_ = h.db.SaveSetting("ai_enabled", enabledStr)
	_ = h.db.SaveSetting("ai_provider", req.Provider)
	_ = h.db.SaveSetting("ai_api_key", req.APIKey)
	_ = h.db.SaveSetting("ai_model", req.Model)
	_ = h.db.SaveSetting("ai_base_url", req.BaseURL)
	_ = h.db.SaveSetting("ai_quality_threshold", strconv.Itoa(req.QualityThreshold))
	_ = h.db.SaveSetting("ai_system_prompt", req.SystemPrompt)
	_ = h.db.SaveSetting("ai_daily_prompt", req.DailyPrompt)
	_ = h.db.SaveSetting("ai_active_profile_id", req.ActiveProfileID)
	if profilesBytes, err := json.Marshal(req.Profiles); err == nil {
		_ = h.db.SaveSetting("ai_provider_profiles", string(profilesBytes))
	}

	// 2. Reload settings in the AI Engine
	err := h.aiEngine.ReloadSettings(req)
	if err != nil {
		h.logger.Error("Failed to reload AI engine settings", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

// HandleTestConnection tests the AI connection using the latest available scraped item.
func (h *AIHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := h.aiEngine.IsEnabled()

	if !enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "AI 引擎未启用，请先开启 '启用 AI 语义分析与评分' 开关并保存配置。",
		})
		return
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

		_, err = h.db.InsertScrapedItem(mockItem)
		if err != nil {
			h.logger.Error("Failed to insert mock item for test", zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   fmt.Sprintf("写入测试数据失败: %v", err),
			})
			return
		}

		testItemID, testItemTitle, err = h.db.GetScrapedItemByURL(mockItem.URL)
		if err != nil {
			testItemID = 0
		}
	}

	if testItemID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "无法创建或获取测试数据项，请手动添加数据源抓取部分内容后重试。",
		})
		return
	}

	h.logger.Info("Running synchronous AI test on item", zap.Int64("item_id", testItemID), zap.String("title", testItemTitle))

	err = h.aiEngine.AnalyzeItem(testItemID)
	if err != nil {
		h.logger.Error("AI test analysis failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("AI 接口连接失败: %v", err),
		})
		return
	}

	analysis, err := h.db.GetAIAnalysis(testItemID)
	if err != nil || analysis == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "AI 分析已执行，但读取结果记录失败。",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"item_title": testItemTitle,
		"analysis":   analysis,
	})
}

// HandleStartEvaluation enqueues all unanalyzed items for processing.
func (h *AIHandlers) HandleStartEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := h.aiEngine.IsEnabled()

	if !enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "AI 引擎未启用，请先开启 AI 评估功能并保存配置。",
		})
		return
	}

	go h.aiEngine.RunBackfill()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "AI 评测队列已启动，正在后台增量评估未分析的文章。",
	})
}
