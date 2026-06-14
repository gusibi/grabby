package mcpiface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/application/twitter"
	"go-server/internal/config"
	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

func NewServer(wm *browserws.WebSocketManager, settings *config.Settings, logger *zap.Logger) *server.SSEServer {
	mcpSvr := server.NewMCPServer("Grabby", "1.0.0")

	// Register screenshot tool
	screenshotTool := mcp.NewTool("screenshot",
		mcp.WithDescription("Capture a screenshot of a specified webpage"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL of the webpage to capture")),
		mcp.WithBoolean("fullPage", mcp.DefaultBool(false), mcp.Description("Whether to capture the full page")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional, uses default if not specified)")),
	)
	mcpSvr.AddTool(screenshotTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.ScreenshotParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		connID, err := wm.ResolveBrowserConnID(params.Browser)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}

		logger.Info("Executing screenshot", zap.String("url", params.URL), zap.String("browser", params.Browser))
		resp, err := wm.SendMessage(ctx, &capture.BrowserRequest{
			Source:   "mcp_client",
			Action:   "mcp_request",
			Command:  "capture",
			URL:      params.URL,
			FullPage: params.FullPage,
		}, connID)
		if err != nil {
			logger.Error("Screenshot failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("Screenshot failed: %s", err.Error())), nil
		}

		if resp.Result.ImageData != "" {
			logger.Info("Screenshot succeeded", zap.String("url", params.URL))
			return mcp.NewToolResultText(resp.Result.ImageData), nil
		}
		logger.Warn("Screenshot response missing image data", zap.Any("response", resp))
		return mcp.NewToolResultText(""), nil
	})

	// Register extract tool
	extractTool := mcp.NewTool("extract",
		mcp.WithDescription("Extract the content of a specified webpage and return it in Markdown format"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL of the webpage to extract content from")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional, uses default if not specified)")),
	)
	mcpSvr.AddTool(extractTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.ExtractParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		connID, err := wm.ResolveBrowserConnID(params.Browser)
		if err != nil {
			logger.Error("Browser not available", zap.String("browser", params.Browser))
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}

		logger.Info("Executing extract", zap.String("url", params.URL), zap.String("browser", params.Browser))
		resp, err := wm.SendMessage(ctx, &capture.BrowserRequest{
			Source:  "mcp_client",
			Action:  "mcp_request",
			Command: "extract",
			URL:     params.URL,
		}, connID)
		if err != nil {
			logger.Error("Extract failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("Extract failed: %s", err.Error())), nil
		}

		markdown := resp.Result.Content.MarkdownContent()
		if markdown != "" {
			logger.Info("Extract succeeded", zap.String("url", params.URL))
			return mcp.NewToolResultText(markdown), nil
		}
		logger.Warn("Extract response missing content", zap.Any("response", resp))
		return mcp.NewToolResultText(""), nil
	})

	// Register Twitter/X tools. These run in the user's logged-in browser via the
	// "intercept" primitive; see docs/browser-executor-plan.md §7.
	registerTwitterTools(mcpSvr, wm, logger)

	return server.NewSSEServer(mcpSvr)
}

// tweetsToJSON serializes parsed tweets as a pretty JSON string for the MCP
// client. Returns an error result if marshaling fails.
func tweetsToJSON(tweets []twitter.Tweet) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(tweets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode tweets"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func registerTwitterTools(mcpSvr *server.MCPServer, wm *browserws.WebSocketManager, logger *zap.Logger) {
	// twitter_search
	searchTool := mcp.NewTool("twitter_search",
		mcp.WithDescription("Search X/Twitter for tweets matching a query, using the user's logged-in browser. Returns structured tweets (text, author, time, engagement)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("limit", mcp.Description("Max tweets to return (default 40)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.TwitterSearchParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		tweets, err := twitter.Search(ctx, wm, logger, p.Query, twitter.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return tweetsToJSON(tweets)
	})

	// twitter_timeline
	timelineTool := mcp.NewTool("twitter_timeline",
		mcp.WithDescription("Read the user's X/Twitter home timeline (requires being logged in). Returns structured tweets for picking what's worth reading."),
		mcp.WithString("kind", mcp.Description("\"for_you\" (default) or \"following\"")),
		mcp.WithNumber("limit", mcp.Description("Max tweets to return (default 40)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(timelineTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.TwitterTimelineParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		tweets, err := twitter.Timeline(ctx, wm, logger, p.Kind, twitter.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return tweetsToJSON(tweets)
	})

	// twitter_likes
	likesTool := mcp.NewTool("twitter_likes",
		mcp.WithDescription("Read a user's liked tweets from x.com/<handle>/likes (requires being logged in for your own likes). Useful for syncing/archiving likes."),
		mcp.WithString("handle", mcp.Required(), mcp.Description("X handle without @, e.g. \"jack\"")),
		mcp.WithNumber("limit", mcp.Description("Max tweets to return (default 60)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(likesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.TwitterLikesParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		tweets, err := twitter.Likes(ctx, wm, logger, p.Handle, twitter.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return tweetsToJSON(tweets)
	})
}
