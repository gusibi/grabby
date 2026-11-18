package mcpiface

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scraping"
	"go-server/internal/domain/item"
	"go-server/internal/infrastructure/sqlite"
	"go-server/internal/interfaces/dto"
)

// itemSummary is the token-cheap shape returned by library_search: enough for an
// agent to pick what to read, without dumping every article body into context.
type itemSummary struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	Summary        string     `json:"summary,omitempty"`
	Category       string     `json:"category,omitempty"`
	SourceCategory string     `json:"source_category,omitempty"`
	SourceID       string     `json:"source_id,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	FetchedAt      time.Time  `json:"fetched_at"`
	ReadStatus     int        `json:"read_status"`
	Starred        int        `json:"starred"`
	Chars          int        `json:"chars"`
}

// registerLibraryTools exposes the server's own stored content (scraped items,
// sources, AI daily digests) plus the unified feed adapter tools.
func registerLibraryTools(mcpSvr *server.MCPServer, db *sqlite.Database, scraper *scraping.Scraper, aiEngine *aiapp.AIEngine, logger *zap.Logger) {
	if db == nil {
		return
	}

	// library_search
	searchTool := mcp.NewTool("library_search",
		mcp.WithDescription("Search Grabby's own library of collected items (RSS/API/scraped articles and saved pages). Returns metadata only — call library_get_item for the full Markdown. Use this before browsing the web: the answer may already be collected."),
		mcp.WithString("query", mcp.Description("Full-text query over title and content (optional)")),
		mcp.WithString("category", mcp.Description("Item category filter (optional)")),
		mcp.WithString("source_category", mcp.Description("Source topic category filter, e.g. \"AI\" (optional)")),
		mcp.WithString("origin", mcp.Description("Origin source filter (optional)")),
		mcp.WithBoolean("starred", mcp.Description("Only starred items")),
		mcp.WithBoolean("unread_only", mcp.Description("Only unread items")),
		mcp.WithNumber("limit", mcp.Description("Max items to return (default 20)")),
		mcp.WithString("cursor", mcp.Description("Pagination cursor from a previous call (optional)")),
	)
	mcpSvr.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.LibrarySearchParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		if params.Limit <= 0 {
			params.Limit = 20
		}

		filter := item.ItemsFilter{
			Category:       params.Category,
			SourceCategory: params.SourceCategory,
			Origin:         params.Origin,
			Q:              params.Query,
			Cursor:         params.Cursor,
			Limit:          params.Limit,
		}
		if params.Starred {
			one := 1
			filter.Starred = &one
		}
		if params.UnreadOnly {
			zero := 0
			filter.ReadStatus = &zero
		}

		items, cursor, err := db.GetScrapedItems(filter)
		if err != nil {
			logger.Error("library_search failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("library_search failed: %s", err.Error())), nil
		}

		summaries := make([]itemSummary, 0, len(items))
		for _, it := range items {
			summaries = append(summaries, itemSummary{
				ID:             it.ID,
				Title:          it.Title,
				URL:            it.URL,
				Summary:        it.Summary,
				Category:       it.Category,
				SourceCategory: it.SourceCategory,
				SourceID:       it.SourceID,
				PublishedAt:    it.PublishedAt,
				FetchedAt:      it.FetchedAt,
				ReadStatus:     it.ReadStatus,
				Starred:        it.Starred,
				Chars:          len([]rune(it.Content)),
			})
		}
		return jsonResult(map[string]any{"items": summaries, "cursor": cursor, "count": len(summaries)})
	})

	// library_get_item
	getTool := mcp.NewTool("library_get_item",
		mcp.WithDescription("Fetch one collected item from Grabby's library by id, including its full Markdown content. Get ids from library_search."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Item id")),
	)
	mcpSvr.AddTool(getTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.LibraryItemParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		if params.ID <= 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		it, err := db.GetScrapedItem(params.ID)
		if err != nil {
			logger.Error("library_get_item failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("library_get_item failed: %s", err.Error())), nil
		}
		if it == nil {
			return mcp.NewToolResultError("item not found"), nil
		}
		return jsonResult(it)
	})

	// library_list_sources
	sourcesTool := mcp.NewTool("library_list_sources",
		mcp.WithDescription("List the configured content sources (RSS/API/scrape) that feed Grabby's library, with their category, schedule and last fetch status."),
	)
	mcpSvr.AddTool(sourcesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := db.GetSources()
		if err != nil {
			logger.Error("library_list_sources failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("library_list_sources failed: %s", err.Error())), nil
		}
		return jsonResult(map[string]any{"sources": sources, "count": len(sources)})
	})

	// library_daily_report
	dailyTool := mcp.NewTool("library_daily_report",
		mcp.WithDescription("Get an AI-generated daily digest of the collected items as Markdown. Defaults to the most recent report."),
		mcp.WithString("date", mcp.Description("Report date, YYYY-MM-DD (optional, defaults to the latest report)")),
		mcp.WithString("report_type", mcp.Description("\"morning\" | \"evening\" | \"daily\" (optional)")),
	)
	mcpSvr.AddTool(dailyTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.LibraryDailyParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		if params.Date != "" {
			report, err := db.GetAIDailyReport(params.Date, params.ReportType)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("library_daily_report failed: %s", err.Error())), nil
			}
			if report == nil {
				return mcp.NewToolResultError("no report for that date"), nil
			}
			return mcp.NewToolResultText(report.Content), nil
		}

		reports, err := db.GetAIDailyReports(1, params.ReportType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("library_daily_report failed: %s", err.Error())), nil
		}
		if len(reports) == 0 {
			return mcp.NewToolResultError("no daily report available yet"), nil
		}
		return mcp.NewToolResultText(reports[0].Content), nil
	})

	// library_stats
	statsTool := mcp.NewTool("library_stats",
		mcp.WithDescription("Summary counts for Grabby's library: total / unread / starred items and per-category breakdowns."),
	)
	mcpSvr.AddTool(statsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		total, err := db.CountItems()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("library_stats failed: %s", err.Error())), nil
		}
		unread, _ := db.CountUnreadItems()
		starred, _ := db.CountStarredItems()
		byCategory, _ := db.CountItemsByCategory()
		bySourceCategory, _ := db.CountItemsBySourceCategory()
		return jsonResult(map[string]any{
			"total":              total,
			"unread":             unread,
			"starred":            starred,
			"by_category":        byCategory,
			"by_source_category": bySourceCategory,
		})
	})

	// feed_query
	feedQueryTool := mcp.NewTool("feed_query",
		mcp.WithDescription("Unified Agent feed adapter across all content types (news, blog, youtube, podcast, all). Returns full Markdown or audio/video transcripts along with AI scores and summaries."),
		mcp.WithString("type", mcp.Description("Content type: all | news | blog | youtube | podcast (default all)")),
		mcp.WithString("category", mcp.Description("Topic or AI category filter, e.g. AI, 科技新闻 (optional)")),
		mcp.WithString("query", mcp.Description("Keyword search over title, summary, or transcript/content (optional)")),
		mcp.WithNumber("score_min", mcp.Description("Minimum AI quality score 1-10 (optional)")),
		mcp.WithNumber("days", mcp.Description("Only items from the last N days (optional)")),
		mcp.WithNumber("limit", mcp.Description("Max items to return (default 20)")),
		mcp.WithBoolean("include_content", mcp.Description("Include full Markdown/transcript body (default true)")),
	)
	mcpSvr.AddTool(feedQueryTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.FeedQueryParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		if params.Type == "" {
			params.Type = "all"
		}
		if params.Limit <= 0 {
			params.Limit = 20
		}
		includeContent := true
		if params.IncludeContent != nil {
			includeContent = *params.IncludeContent
		}

		items, cursor, err := db.QueryUnifiedFeed(item.ItemsFilter{
			ContentType: params.Type,
			Category:    params.Category,
			Q:           params.Query,
			ScoreMin:    params.ScoreMin,
			Days:        params.Days,
			Limit:       params.Limit,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("feed_query failed: %s", err.Error())), nil
		}
		if !includeContent {
			for i := range items {
				items[i].Content = ""
			}
		}
		return jsonResult(map[string]any{"success": true, "type": params.Type, "count": len(items), "items": items, "next_cursor": cursor})
	})

	// feed_daily_picks
	picksTool := mcp.NewTool("feed_daily_picks",
		mcp.WithDescription("Get Grabby's cross-type Daily Picks (news + blogs + YouTube + podcasts) ranked by AI quality score, together with today's AI daily report if generated."),
		mcp.WithString("date", mcp.Description("YYYY-MM-DD (optional, defaults to today)")),
		mcp.WithNumber("days", mcp.Description("Lookback window in days (default 1, relaxes to 3 if empty)")),
		mcp.WithNumber("score_min", mcp.Description("Minimum AI score (optional)")),
		mcp.WithNumber("limit", mcp.Description("Max picks to return (default 20)")),
		mcp.WithBoolean("include_content", mcp.Description("Include full Markdown/transcript body (default true)")),
	)
	mcpSvr.AddTool(picksTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.FeedPicksParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		dateStr := strings.TrimSpace(params.Date)
		if dateStr == "" {
			dateStr = time.Now().Format("2006-01-02")
		}
		limit := params.Limit
		if limit <= 0 {
			limit = 20
		}
		days := params.Days
		explicitDays := days > 0
		if days <= 0 {
			days = 1
		}
		scoreMin := 7
		if aiEngine != nil && aiEngine.Settings().QualityThreshold > 0 {
			scoreMin = aiEngine.Settings().QualityThreshold
		}
		explicitScore := false
		if params.ScoreMin != nil {
			scoreMin = *params.ScoreMin
			explicitScore = true
		}
		includeContent := true
		if params.IncludeContent != nil {
			includeContent = *params.IncludeContent
		}

		items, _, err := db.QueryUnifiedFeed(item.ItemsFilter{ScoreMin: scoreMin, Days: days, Limit: 100})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("feed_daily_picks failed: %s", err.Error())), nil
		}
		if len(items) == 0 && !explicitDays {
			days = 3
			items, _, _ = db.QueryUnifiedFeed(item.ItemsFilter{ScoreMin: scoreMin, Days: days, Limit: 100})
		}
		if len(items) == 0 && !explicitScore && scoreMin > 0 {
			scoreMin = 0
			items, _, _ = db.QueryUnifiedFeed(item.ItemsFilter{ScoreMin: 0, Days: days, Limit: 100})
		}

		sort.SliceStable(items, func(i, j int) bool {
			return items[i].QualityScore > items[j].QualityScore
		})
		if len(items) > limit {
			items = items[:limit]
		}
		if !includeContent {
			for i := range items {
				items[i].Content = ""
			}
		}

		var dailyReport any
		for _, rType := range []string{"daily", "morning", "evening"} {
			if rpt, err := db.GetAIDailyReport(dateStr, rType); err == nil && rpt != nil {
				dailyReport = rpt
				break
			}
		}

		return jsonResult(map[string]any{
			"success":      true,
			"date":         dateStr,
			"days":         days,
			"score_min":    scoreMin,
			"count":        len(items),
			"items":        items,
			"daily_report": dailyReport,
		})
	})

	// feed_ingest
	if scraper != nil {
		ingestTool := mcp.NewTool("feed_ingest",
			mcp.WithDescription("Fetch any single YouTube video URL, podcast episode URL (Xiaoyuzhou/Apple/audio), or web article URL right now, extract full Markdown or transcript text, save to Grabby, and return it."),
			mcp.WithString("url", mcp.Required(), mcp.Description("YouTube video URL, podcast episode URL, or webpage URL")),
			mcp.WithString("type", mcp.Description("Optional hint: auto | youtube | podcast | blog | news (default auto)")),
			mcp.WithBoolean("analyze", mcp.Description("Run AI summary & scoring if AI is enabled (default true)")),
		)
		mcpSvr.AddTool(ingestTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params, err := dto.ParseArgs[dto.FeedIngestParams](req.Params.Arguments)
			if err != nil || strings.TrimSpace(params.URL) == "" {
				return mcp.NewToolResultError("url is required"), nil
			}
			analyze := true
			if params.Analyze != nil {
				analyze = *params.Analyze
			}
			ingested, err := scraper.IngestURL(ctx, params.URL, params.Type, analyze)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("feed_ingest failed: %s", err.Error())), nil
			}
			return jsonResult(map[string]any{"success": true, "item": ingested})
		})
	}
}

// jsonResult marshals v as the tool's text payload.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("failed to encode result"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
