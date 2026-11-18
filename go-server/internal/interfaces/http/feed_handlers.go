package httpiface

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/render"
	"go-server/internal/domain/item"
)

type feedHandlers struct {
	deps Dependencies
}

// RegisterFeedHandlers registers the unified Agent feed, daily picks, realtime
// ingest, and ASR environment endpoints on both /open/api and /api.
func RegisterFeedHandlers(openAPI, api *echo.Group, deps Dependencies) {
	h := &feedHandlers{deps: deps}

	for _, grp := range []*echo.Group{openAPI, api} {
		grp.GET("/feed", h.handleFeed)
		grp.GET("/feed/picks", h.handleFeedPicks)
		grp.POST("/feed/ingest", h.handleFeedIngest)
		grp.GET("/asr/env", h.handleASREnv)
		grp.POST("/asr/install", h.handleASRInstall)
	}
}

// UnifiedFeedItem is the single normalized response shape returned to Agents
// across news, blogs, YouTube videos, and podcasts.
type UnifiedFeedItem struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	URL            string         `json:"url"`
	OriginSource   string         `json:"origin_source"`
	SourceID       string         `json:"source_id"`
	SourceCategory string         `json:"source_category,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Content        string         `json:"content,omitempty"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	FetchedAt      time.Time      `json:"fetched_at"`
	QualityScore   int            `json:"quality_score,omitempty"`
	AICategory     string         `json:"ai_category,omitempty"`
	AISubcategory  string         `json:"ai_subcategory,omitempty"`
	AISummary      string         `json:"ai_summary,omitempty"`
	AIComment      string         `json:"ai_comment,omitempty"`
	AITags         string         `json:"ai_tags,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

func toUnifiedFeedItem(it item.ScrapedItemWithAI, includeContent bool) UnifiedFeedItem {
	itemType := strings.ToLower(strings.TrimSpace(it.Category))
	if itemType == "" || itemType == "article" {
		itemType = "news"
	}

	var meta map[string]any
	if strings.TrimSpace(it.Metadata) != "" && it.Metadata != "{}" {
		_ = json.Unmarshal([]byte(it.Metadata), &meta)
	}

	out := UnifiedFeedItem{
		ID:             it.ID,
		Type:           itemType,
		Title:          it.Title,
		URL:            it.URL,
		OriginSource:   it.OriginSource,
		SourceID:       it.SourceID,
		SourceCategory: it.SourceCategory,
		Summary:        it.Summary,
		PublishedAt:    it.PublishedAt,
		FetchedAt:      it.FetchedAt,
		QualityScore:   it.QualityScore,
		AICategory:     it.AICategory,
		AISubcategory:  it.AISubcategory,
		AISummary:      it.AISummary,
		AIComment:      it.AIComment,
		AITags:         it.AITags,
		Metadata:       meta,
	}
	if includeContent {
		out.Content = it.Content
	}
	return out
}

func (h *feedHandlers) handleFeed(c echo.Context) error {
	contentType := strings.ToLower(strings.TrimSpace(c.QueryParam("type")))
	if contentType == "" {
		contentType = "all"
	}

	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}
	scoreMin := 0
	if s, err := strconv.Atoi(c.QueryParam("score_min")); err == nil && s > 0 {
		scoreMin = s
	}
	days := 0
	if d, err := strconv.Atoi(c.QueryParam("days")); err == nil && d > 0 {
		days = d
	}

	includeContent := true
	if ic := strings.TrimSpace(c.QueryParam("include_content")); ic != "" {
		includeContent = !(ic == "0" || strings.EqualFold(ic, "false"))
	}

	rawItems, nextCursor, err := h.deps.DB.QueryUnifiedFeed(item.ItemsFilter{
		ContentType: contentType,
		Category:    c.QueryParam("category"),
		SourceID:    c.QueryParam("source_id"),
		Q:           c.QueryParam("q"),
		ScoreMin:    scoreMin,
		Days:        days,
		Limit:       limit,
		Cursor:      c.QueryParam("cursor"),
	})
	if err != nil {
		h.deps.Logger.Error("Failed to query unified feed", zap.Error(err))
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	items := make([]UnifiedFeedItem, 0, len(rawItems))
	for _, it := range rawItems {
		items = append(items, toUnifiedFeedItem(it, includeContent))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":     true,
		"type":        contentType,
		"count":       len(items),
		"items":       items,
		"next_cursor": nextCursor,
	})
}

func (h *feedHandlers) handleFeedPicks(c echo.Context) error {
	dateStr := strings.TrimSpace(c.QueryParam("date"))
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}

	explicitDays := false
	days := 1
	if dStr := strings.TrimSpace(c.QueryParam("days")); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d > 0 {
			days = d
			explicitDays = true
		}
	}

	explicitScore := false
	scoreMin := 7
	if h.deps.AIEngine != nil {
		if t := h.deps.AIEngine.Settings().QualityThreshold; t > 0 {
			scoreMin = t
		}
	}
	if sStr := strings.TrimSpace(c.QueryParam("score_min")); sStr != "" {
		if s, err := strconv.Atoi(sStr); err == nil && s >= 0 {
			scoreMin = s
			explicitScore = true
		}
	}

	includeContent := true
	if ic := strings.TrimSpace(c.QueryParam("include_content")); ic != "" {
		includeContent = !(ic == "0" || strings.EqualFold(ic, "false"))
	}

	fetchPicks := func(d, s int) ([]item.ScrapedItemWithAI, error) {
		rows, _, err := h.deps.DB.QueryUnifiedFeed(item.ItemsFilter{
			ScoreMin: s,
			Days:     d,
			Limit:    100,
		})
		return rows, err
	}

	rawItems, err := fetchPicks(days, scoreMin)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	// Graceful relaxation when caller didn't force strict bounds and today's
	// quality queue hasn't produced enough items yet.
	if len(rawItems) == 0 && !explicitDays {
		days = 3
		rawItems, err = fetchPicks(days, scoreMin)
		if err != nil {
			return detailErr(c, http.StatusInternalServerError, err.Error())
		}
	}
	if len(rawItems) == 0 && !explicitScore && scoreMin > 0 {
		scoreMin = 0
		rawItems, err = fetchPicks(days, scoreMin)
		if err != nil {
			return detailErr(c, http.StatusInternalServerError, err.Error())
		}
	}

	sort.SliceStable(rawItems, func(i, j int) bool {
		if rawItems[i].QualityScore != rawItems[j].QualityScore {
			return rawItems[i].QualityScore > rawItems[j].QualityScore
		}
		ti := rawItems[i].FetchedAt
		if rawItems[i].PublishedAt != nil {
			ti = *rawItems[i].PublishedAt
		}
		tj := rawItems[j].FetchedAt
		if rawItems[j].PublishedAt != nil {
			tj = *rawItems[j].PublishedAt
		}
		return ti.After(tj)
	})

	if len(rawItems) > limit {
		rawItems = rawItems[:limit]
	}

	items := make([]UnifiedFeedItem, 0, len(rawItems))
	byType := make(map[string][]UnifiedFeedItem)
	for _, raw := range rawItems {
		u := toUnifiedFeedItem(raw, includeContent)
		items = append(items, u)
		byType[u.Type] = append(byType[u.Type], u)
	}

	var dailyReport map[string]any
	for _, rType := range []string{"daily", "morning", "evening"} {
		rpt, rErr := h.deps.DB.GetAIDailyReport(dateStr, rType)
		if rErr == nil && rpt != nil {
			var parsed map[string]any
			if json.Unmarshal([]byte(rpt.Content), &parsed) != nil {
				parsed = map[string]any{
					"format":       "markdown",
					"content":      rpt.Content,
					"html_content": render.Render(rpt.Content),
				}
			} else {
				parsed["format"] = "json"
			}
			parsed["report_date"] = rpt.ReportDate
			parsed["report_type"] = rpt.ReportType
			parsed["title"] = rpt.Title
			parsed["total_items"] = rpt.TotalItems
			parsed["quality_items"] = rpt.QualityItems
			parsed["generated_at"] = rpt.GeneratedAt
			dailyReport = parsed
			break
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":      true,
		"date":         dateStr,
		"days":         days,
		"score_min":    scoreMin,
		"count":        len(items),
		"items":        items,
		"by_type":      byType,
		"daily_report": dailyReport,
	})
}

func (h *feedHandlers) handleFeedIngest(c echo.Context) error {
	var req struct {
		URL     string `json:"url"`
		Type    string `json:"type"`
		Analyze *bool  `json:"analyze"`
	}
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		return detailErr(c, http.StatusBadRequest, "url is required")
	}
	if h.deps.Scraper == nil {
		return detailErr(c, http.StatusServiceUnavailable, "scraper service unavailable")
	}

	analyze := true
	if req.Analyze != nil {
		analyze = *req.Analyze
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
	defer cancel()

	it, err := h.deps.Scraper.IngestURL(ctx, req.URL, req.Type, analyze)
	if err != nil {
		h.deps.Logger.Warn("Realtime feed ingest failed", zap.String("url", req.URL), zap.Error(err))
		return detailErr(c, http.StatusBadGateway, err.Error())
	}
	if it == nil {
		return detailErr(c, http.StatusInternalServerError, "ingested item could not be loaded")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"item":    toUnifiedFeedItem(*it, true),
	})
}

func (h *feedHandlers) handleASREnv(c echo.Context) error {
	if h.deps.Scraper == nil || h.deps.Scraper.Transcriber() == nil {
		return detailErr(c, http.StatusServiceUnavailable, "transcriber unavailable")
	}
	env := h.deps.Scraper.Transcriber().CheckEnvironment()
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"env":     env,
	})
}

func (h *feedHandlers) handleASRInstall(c echo.Context) error {
	if h.deps.Scraper == nil || h.deps.Scraper.Transcriber() == nil {
		return detailErr(c, http.StatusServiceUnavailable, "transcriber unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Minute)
	defer cancel()

	output, err := h.deps.Scraper.Transcriber().InstallLocalTools(ctx)
	env := h.deps.Scraper.Transcriber().CheckEnvironment()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   err.Error(),
			"output":  output,
			"env":     env,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"output":  output,
		"env":     env,
	})
}
