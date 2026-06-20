package httpiface

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/reddit"
	"go-server/internal/infrastructure/sqlite"
	"go-server/internal/interfaces/dto"
)

// redditHandlers is the HTTP entry to the Reddit site adapter
// (docs/prd-likes-incremental-and-reddit.md). It mirrors the MCP reddit_thread
// tool: same adapter, same "fetchInPage" primitive underneath, just exposed
// over HTTP for the http-api -> browser workflow.
type redditHandlers struct {
	deps Dependencies
}

// RegisterRedditHandlers wires the Reddit HTTP endpoints.
func RegisterRedditHandlers(api *echo.Group, deps Dependencies) {
	h := &redditHandlers{deps: deps}
	api.POST("/reddit/thread", h.thread)
	api.POST("/reddit/subreddit", h.subreddit)
	api.POST("/reddit/search", h.search)
}

func (h *redditHandlers) thread(c echo.Context) error {
	var req dto.RedditThreadAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.URL == "" {
		return detailErr(c, http.StatusBadRequest, "url is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	thread, err := reddit.FetchThread(ctx, h.deps.WSManager, h.deps.Logger, req.URL, reddit.Options{Browser: req.Browser})
	if err != nil {
		return redditErr(c, err)
	}
	return c.JSON(http.StatusOK, dto.RedditThreadAPIResponse{
		Success:  true,
		URL:      thread.Post.URL,
		Post:     thread.Post,
		Comments: toAnySlice(thread.Comments),
	})
}

func (h *redditHandlers) subreddit(c echo.Context) error {
	var req dto.RedditSubredditAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.Subreddit == "" {
		return detailErr(c, http.StatusBadRequest, "subreddit is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	posts, err := reddit.FetchSubredditPages(ctx, h.deps.WSManager, h.deps.Logger, req.Subreddit, reddit.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return redditErr(c, err)
	}
	h.archivePosts(posts)
	return c.JSON(http.StatusOK, dto.RedditSubredditAPIResponse{
		Success: true,
		Count:   len(posts),
		Posts:   postsToAny(posts),
	})
}

func (h *redditHandlers) search(c echo.Context) error {
	var req dto.RedditSearchAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.Query == "" {
		return detailErr(c, http.StatusBadRequest, "query is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	posts, err := reddit.FetchSearchPages(ctx, h.deps.WSManager, h.deps.Logger, req.Query, req.Subreddit, req.Sort, reddit.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return redditErr(c, err)
	}
	h.archivePosts(posts)
	return c.JSON(http.StatusOK, dto.RedditSearchAPIResponse{
		Success: true,
		Count:   len(posts),
		Posts:   postsToAny(posts),
	})
}

// archivePosts upserts fetched posts into the reddit_posts table for later
// reuse. Storage is best-effort: a failure is logged but never fails the
// request.
func (h *redditHandlers) archivePosts(posts []reddit.Post) {
	if len(posts) == 0 {
		return
	}
	now := time.Now()
	records := make([]sqlite.RedditPost, 0, len(posts))
	for _, p := range posts {
		records = append(records, sqlite.RedditPost{
			ID:          p.ID,
			Title:       p.Title,
			Author:      p.Author,
			Subreddit:   p.Subreddit,
			URL:         p.URL,
			ContentURL:  p.ContentURL,
			Body:        p.Body,
			Score:       p.Score,
			NumComments: p.NumComments,
			CreatedUTC:  p.CreatedUTC,
			FetchedAt:   now,
		})
	}
	if err := h.deps.DB.SaveRedditPosts(records); err != nil {
		h.deps.Logger.Warn("failed to archive reddit posts",
			zap.String("subreddit", req0Subreddit(records)),
			zap.Int("count", len(records)), zap.Error(err))
	}
}

func req0Subreddit(records []sqlite.RedditPost) string {
	if len(records) > 0 {
		return records[0].Subreddit
	}
	return ""
}

// withTimeout derives a request context bounded by the configured extract timeout.
func (h *redditHandlers) withTimeout(c echo.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request().Context(), time.Duration(h.deps.Settings.APIExtractTimeout*float64(time.Second)))
}

// toAnySlice converts []reddit.Comment to []any for the response DTO.
func toAnySlice(comments []reddit.Comment) []any {
	out := make([]any, len(comments))
	for i, c := range comments {
		out[i] = c
	}
	return out
}

// postsToAny converts []reddit.Post to []any for the response DTO.
func postsToAny(posts []reddit.Post) []any {
	out := make([]any, len(posts))
	for i, p := range posts {
		out[i] = p
	}
	return out
}

// redditErr maps a reddit adapter error to an HTTP status. A missing browser is
// 503 (no executor connected); anything else is 502 (the browser/page produced
// no usable result — e.g. not logged in or Reddit returned an error).
func redditErr(c echo.Context, err error) error {
	status := http.StatusBadGateway
	if strings.Contains(err.Error(), "browser not available") {
		status = http.StatusServiceUnavailable
	}
	return detailErr(c, status, err.Error())
}
