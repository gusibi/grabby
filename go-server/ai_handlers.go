package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"go.uber.org/zap"
)

// AIHandlers encapsulates all AI HTTP handlers.
type AIHandlers struct {
	db           *Database
	aiEngine     *AIEngine
	dailyManager *AIDailyManager
	logger       *zap.Logger
}

// NewAIHandlers creates a new AIHandlers instance.
func NewAIHandlers(db *Database, aiEngine *AIEngine, dailyManager *AIDailyManager, logger *zap.Logger) *AIHandlers {
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

	scoreMin := h.aiEngine.settings.QualityThreshold
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

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(AIItemsFilter{
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

	items, nextCursor, err := h.db.GetScrapedItemsWithAI(AIItemsFilter{
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

// HandleDaily retrieves the AI Daily Report for a date.
func (h *AIHandlers) HandleDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	report, err := h.db.GetAIDailyReport(dateStr)
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

	reports, err := h.db.GetAIDailyReports(limit)
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

// HandleDailyGenerate manual triggers a daily report generation.
func (h *AIHandlers) HandleDailyGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Date string `json:"date"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Date == "" {
		req.Date = r.URL.Query().Get("date")
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	report, err := h.dailyManager.GenerateDailyReport(r.Context(), req.Date)
	if err != nil {
		h.logger.Error("Failed to generate daily report manually", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
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

	var totalProcessed, totalPending int
	err := h.db.db.QueryRow("SELECT COUNT(*) FROM ai_analyses").Scan(&totalProcessed)
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = h.db.db.QueryRow(`
		SELECT COUNT(*) FROM scraped_items i
		LEFT JOIN ai_analyses a ON i.id = a.item_id
		WHERE a.item_id IS NULL
	`).Scan(&totalPending)
	if err != nil {
		h.logger.Error("Stats query failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var avgScore float64
	_ = h.db.db.QueryRow("SELECT COALESCE(AVG(quality_score), 0) FROM ai_analyses").Scan(&avgScore)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"total_processed": totalProcessed,
		"total_pending":   totalPending,
		"average_score":   avgScore,
		"queue_length":    len(h.aiEngine.queue),
	})
}

// HandleGetSettings retrieves the current AI settings.
func (h *AIHandlers) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.aiEngine.mu.RLock()
	settings := h.aiEngine.settings
	h.aiEngine.mu.RUnlock()

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

	var req AISettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode settings request body", zap.Error(err))
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Normalize settings
	req = NormalizeAISettings(req)

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

	// 2. Reload settings in the AI Engine (re-creates Genkit, handles start/stop workers)
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

	h.aiEngine.mu.RLock()
	enabled := h.aiEngine.settings.Enabled
	h.aiEngine.mu.RUnlock()

	if !enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "AI 引擎未启用，请先开启 '启用 AI 语义分析与评分' 开关并保存配置。",
		})
		return
	}

	// 1. Find the latest scraped item to test
	// We check for unanalyzed items first
	items, err := h.db.GetUnanalyzedItems(1)
	var testItemID int64
	var testItemTitle string

	if err == nil && len(items) > 0 {
		testItemID = items[0].ID
		testItemTitle = items[0].Title
	} else {
		// Fallback: get the latest scraped item overall
		var itemID int64
		var title string
		err = h.db.db.QueryRow("SELECT id, title FROM scraped_items ORDER BY id DESC LIMIT 1").Scan(&itemID, &title)
		if err == nil {
			testItemID = itemID
			testItemTitle = title
		}
	}

	// 2. If no items found in DB at all, insert a dummy mock item to test connection!
	if testItemID == 0 {
		h.logger.Info("No scraped items found in DB. Inserting mock item for AI test.")
		mockItem := ScrapedItem{
			SourceID:     "ai-test-source",
			OriginSource: "System Test",
			Title:        "测试文章 - 验证 AI 模型连通性",
			URL:          "https://example.com/ai-test-connection-mock-url",
			Summary:      "这是一篇用于验证 AI 接口与模型连通性的测试文章。",
			Content:      "今天我们在这里进行 Grabby AI 语义评估连通性的测试。系统将会尝试连接大语言模型并根据自定义的 System Prompt 对本正文进行分类打分。如果连接成功，您将在前端设置页面看到包含评分、分类和简短摘要的结果反馈。",
			Category:     "article",
		}
		
		// Ensure dummy source exists
		_ = h.db.InsertSource(Source{
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

		// Query it back to get ID
		_ = h.db.db.QueryRow("SELECT id, title FROM scraped_items WHERE url = ?", mockItem.URL).Scan(&testItemID, &testItemTitle)
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

	// 3. Analyze item synchronously
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

	// 4. Retrieve the analysis result
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

	h.aiEngine.mu.RLock()
	enabled := h.aiEngine.settings.Enabled
	h.aiEngine.mu.RUnlock()

	if !enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "AI 引擎未启用，请先开启 AI 评估功能并保存配置。",
		})
		return
	}

	// Trigger runBackfill to immediately enqueue unanalyzed items
	go h.aiEngine.runBackfill()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "AI 评测队列已启动，正在后台增量评估未分析的文章。",
	})
}
