package mcpiface

import (
	"context"
	"encoding/json"
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

	// list_browsers — an agent needs to know which browsers it can drive before
	// passing a "browser" argument to the other tools.
	listTool := mcp.NewTool("list_browsers",
		mcp.WithDescription("List the browser extensions currently connected to Grabby. Use this to discover valid values for the \"browser\" argument of the other tools."),
	)
	mcpSvr.AddTool(listTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		browsers := wm.GetBrowserList()
		b, err := json.Marshal(map[string]any{"browsers": browsers, "count": len(browsers)})
		if err != nil {
			return mcp.NewToolResultError("failed to encode browser list"), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	// fetch_in_page — issue an HTTP request from inside a logged-in page, so the
	// site's cookies and origin apply. Useful for JSON APIs behind a login.
	fetchTool := mcp.NewTool("fetch_in_page",
		mcp.WithDescription("Perform an HTTP request from inside a page in the user's logged-in browser, so the site's cookies and origin apply. Use it to call JSON APIs that require being signed in. Returns the raw response body as text."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Page to open as the request context, e.g. \"https://example.com/\"")),
		mcp.WithString("requestUrl", mcp.Description("Request target (defaults to url)")),
		mcp.WithString("method", mcp.Description("HTTP method, default GET")),
		mcp.WithString("body", mcp.Description("Request body (for POST/PUT)")),
		mcp.WithString("credentials", mcp.Description("fetch credentials mode, default \"include\"")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(fetchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := dto.ParseArgs[dto.FetchInPageParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		if params.URL == "" {
			return mcp.NewToolResultError("url is required"), nil
		}

		connID, err := wm.ResolveBrowserConnID(params.Browser)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}

		fetchParams := map[string]any{}
		if params.RequestURL != "" {
			fetchParams["requestUrl"] = params.RequestURL
		}
		if params.Method != "" {
			fetchParams["method"] = params.Method
		}
		if len(params.Headers) > 0 {
			fetchParams["headers"] = params.Headers
		}
		if params.Body != "" {
			fetchParams["body"] = params.Body
		}
		if params.Credentials != "" {
			fetchParams["credentials"] = params.Credentials
		}

		logger.Info("Executing fetch_in_page", zap.String("url", params.URL), zap.String("browser", params.Browser))
		resp, err := wm.SendMessage(ctx, &capture.BrowserRequest{
			Source:   "mcp_client",
			Action:   "mcp_request",
			Command:  "fetchInPage",
			URL:      params.URL,
			CloseTab: true,
			Params:   fetchParams,
		}, connID)
		if err != nil {
			logger.Error("fetch_in_page failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("fetch_in_page failed: %s", err.Error())), nil
		}
		return mcp.NewToolResultText(resp.Result.Text), nil
	})
}
