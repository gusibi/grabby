package mcpiface

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/application/reddit"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

// registerRedditTools registers the intent-level Reddit tools. They run in the
// user's browser via the "fetchInPage" primitive; see
// docs/prd-likes-incremental-and-reddit.md.
func registerRedditTools(mcpSvr *server.MCPServer, wm *browserws.WebSocketManager, logger *zap.Logger) {
	// reddit_thread
	threadTool := mcp.NewTool("reddit_thread",
		mcp.WithDescription("Fetch a Reddit post and its comment tree from its permalink, using the user's logged-in browser. Returns the post (title, body, author, score) and a nested comment tree."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Reddit post permalink URL")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(threadTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.RedditThreadParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		thread, err := reddit.FetchThread(ctx, wm, logger, p.URL, reddit.Options{Browser: p.Browser})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return redditThreadToJSON(thread)
	})

	// reddit_subreddit
	subredditTool := mcp.NewTool("reddit_subreddit",
		mcp.WithDescription("Fetch the newest posts from a Reddit subreddit (e.g. \"golang\"), using the user's logged-in browser. Returns structured posts (title, author, score, url, body) from the /new listing."),
		mcp.WithString("subreddit", mcp.Required(), mcp.Description("Subreddit name without r/, e.g. \"golang\"")),
		mcp.WithNumber("limit", mcp.Description("Max posts to return (default 100)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(subredditTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.RedditSubredditParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		posts, err := reddit.FetchSubredditPages(ctx, wm, logger, p.Subreddit, reddit.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return redditPostsToJSON(posts)
	})

	// reddit_search
	searchTool := mcp.NewTool("reddit_search",
		mcp.WithDescription("Search Reddit for posts matching a query, using the user's logged-in browser. Optionally limit to a single subreddit. Returns structured posts (title, author, score, url, body)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("subreddit", mcp.Description("Limit search to one subreddit (without r/), e.g. \"golang\" (optional)")),
		mcp.WithString("sort", mcp.Description("\"relevance\" | \"new\" | \"top\" | \"comments\" (optional)")),
		mcp.WithNumber("limit", mcp.Description("Max posts to return (default 100)")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional)")),
	)
	mcpSvr.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := dto.ParseArgs[dto.RedditSearchParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}
		posts, err := reddit.FetchSearchPages(ctx, wm, logger, p.Query, p.Subreddit, p.Sort, reddit.Options{Browser: p.Browser, Limit: p.Limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return redditPostsToJSON(posts)
	})
}

// redditPostsToJSON serializes parsed Reddit posts as a pretty JSON string for
// the MCP client.
func redditPostsToJSON(posts []reddit.Post) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode posts"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// redditThreadToJSON serializes a parsed Reddit thread as a pretty JSON string
// for the MCP client.
func redditThreadToJSON(thread *reddit.Thread) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode thread"), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
