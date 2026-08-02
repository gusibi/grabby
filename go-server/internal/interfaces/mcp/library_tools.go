package mcpiface

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

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
// sources, AI daily digests). These need no browser connection — they read the
// local database — so an agent can use them even when no browser is attached.
func registerLibraryTools(mcpSvr *server.MCPServer, db *sqlite.Database, logger *zap.Logger) {
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
}

// jsonResult marshals v as the tool's text payload.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("failed to encode result"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
