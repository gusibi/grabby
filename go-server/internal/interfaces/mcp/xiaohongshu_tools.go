package mcpiface

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/application/xiaohongshu"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

// registerXiaohongshuTools registers the intent-level Xiaohongshu tools. They
// run in the user's logged-in browser via the runPageScript (note detail, SSR
// __INITIAL_STATE__) and intercept (search / user notes) primitives.
func registerXiaohongshuTools(mcpSvr *server.MCPServer, wm *browserws.WebSocketManager, logger *zap.Logger) {
	// xiaohongshu_note
	noteTool := mcp.NewTool("xiaohongshu_note",
		mcp.WithDescription("Fetch a Xiaohongshu (小红书) note's detail from its explore URL, using the user's logged-in browser. Returns the note (title, desc, images, author, interaction counts) and up to the first 100 top-level comments."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Xiaohongshu note explore URL")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(noteTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.XhsNoteParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		note, err := xiaohongshu.FetchNote(ctx, wm, logger, p.URL, xiaohongshu.Options{Browser: p.Browser})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return xhsNoteToJSON(note)
	})

	// xiaohongshu_search
	searchTool := mcp.NewTool("xiaohongshu_search",
		mcp.WithDescription("Search Xiaohongshu (小红书) for notes matching a query, using the user's logged-in browser. Returns structured notes (title, author, interaction counts, cover)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("limit", mcp.Description("Max notes to return (default 50)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.XhsSearchParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		notes, err := xiaohongshu.Search(ctx, wm, logger, p.Query, xiaohongshu.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return xhsNotesToJSON(notes)
	})

	// xiaohongshu_user_notes
	userNotesTool := mcp.NewTool("xiaohongshu_user_notes",
		mcp.WithDescription("Fetch a Xiaohongshu (小红书) user's posted notes from their profile URL, using the user's logged-in browser. Returns structured notes."),
		mcp.WithString("url", mcp.Required(), mcp.Description("User profile URL, e.g. https://www.xiaohongshu.com/user/profile/{userId}")),
		mcp.WithNumber("limit", mcp.Description("Max notes to return (default 50)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(userNotesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.XhsUserNotesParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		notes, err := xiaohongshu.UserNotes(ctx, wm, logger, p.URL, xiaohongshu.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return xhsNotesToJSON(notes)
	})
}

func xhsNoteToJSON(note *xiaohongshu.Note) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode note"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func xhsNotesToJSON(notes []xiaohongshu.Note) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode notes"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
