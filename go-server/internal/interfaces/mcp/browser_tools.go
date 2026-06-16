package mcpiface

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

// registerBrowserTools registers the generic browser tools (screenshot, extract)
// that forward a single command to the user's browser extension.
func registerBrowserTools(mcpSvr *server.MCPServer, wm *browserws.WebSocketManager, logger *zap.Logger) {
	// screenshot
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

	// extract
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
}
