package httpiface

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/twitter"
	"go-server/internal/infrastructure/sqlite"
	"go-server/internal/interfaces/dto"
)

// twitterHandlers is the HTTP entry to the X/Twitter site adapter (plan §7). It
// mirrors the MCP twitter_* tools: same adapter, same "intercept" primitive
// underneath, just exposed over HTTP for the http-api -> browser workflow.
type twitterHandlers struct {
	deps Dependencies
}

// RegisterTwitterHandlers wires the X/Twitter HTTP endpoints.
func RegisterTwitterHandlers(api *echo.Group, deps Dependencies) {
	h := &twitterHandlers{deps: deps}
	api.POST("/twitter/search", h.search)
	api.POST("/twitter/timeline", h.timeline)
	api.POST("/twitter/likes", h.likes)

	// Capture records (read-only archive of fetched tweets).
	api.GET("/captures/twitter", h.listCaptures)
}

// listCaptures returns archived tweets, optionally filtered by ?source=.
func (h *twitterHandlers) listCaptures(c echo.Context) error {
	limit, offset := pageParams(c, 50)
	rows, total, err := h.deps.DB.ListTweets(c.QueryParam("source"), limit, offset)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"items": rows, "total": total})
}

func (h *twitterHandlers) search(c echo.Context) error {
	var req dto.TwitterSearchAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	tweets, err := twitter.Search(ctx, h.deps.WSManager, h.deps.Logger, req.Query, twitter.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return twitterErr(c, err)
	}
	h.archive(tweets, "search")
	return writeTweets(c, tweets)
}

func (h *twitterHandlers) timeline(c echo.Context) error {
	var req dto.TwitterTimelineAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	tweets, err := twitter.Timeline(ctx, h.deps.WSManager, h.deps.Logger, req.Kind, twitter.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return twitterErr(c, err)
	}
	h.archive(tweets, "timeline")
	return writeTweets(c, tweets)
}

func (h *twitterHandlers) likes(c echo.Context) error {
	var req dto.TwitterLikesAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	tweets, err := twitter.Likes(ctx, h.deps.WSManager, h.deps.Logger, req.Handle, twitter.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return twitterErr(c, err)
	}
	h.archive(tweets, "likes")
	return writeTweets(c, tweets)
}

// withTimeout derives a request context bounded by the configured extract timeout.
func (h *twitterHandlers) withTimeout(c echo.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request().Context(), time.Duration(h.deps.Settings.APIExtractTimeout*float64(time.Second)))
}

// archive upserts fetched tweets into the tweets table for later reuse. Storage
// is best-effort: a failure is logged but never fails the request.
func (h *twitterHandlers) archive(tweets []twitter.Tweet, source string) {
	if len(tweets) == 0 {
		return
	}
	now := time.Now()
	records := make([]sqlite.TweetRecord, 0, len(tweets))
	for _, t := range tweets {
		records = append(records, sqlite.TweetRecord{
			ID:             t.ID,
			Text:           t.Text,
			Author:         t.Author,
			AuthorName:     t.AuthorName,
			TweetCreatedAt: t.CreatedAt,
			FavoriteCount:  t.FavoriteCount,
			RetweetCount:   t.RetweetCount,
			ReplyCount:     t.ReplyCount,
			QuoteCount:     t.QuoteCount,
			URL:            t.URL,
			Media:          t.Media,
			Source:         source,
			FetchedAt:      now,
		})
	}
	if err := h.deps.DB.SaveTweets(records); err != nil {
		h.deps.Logger.Warn("failed to archive tweets", zap.String("source", source), zap.Int("count", len(records)), zap.Error(err))
	}
}

// writeTweets encodes a successful tweet list as the shared Twitter response.
func writeTweets(c echo.Context, tweets []twitter.Tweet) error {
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"count":   len(tweets),
		"tweets":  tweets,
	})
}

// twitterErr maps a twitter adapter error to an HTTP status. A missing browser
// is 503 (no executor connected); anything else is 502 (the browser/page
// produced no usable result — e.g. not logged in or X changed).
func twitterErr(c echo.Context, err error) error {
	status := http.StatusBadGateway
	if strings.Contains(err.Error(), "browser not available") {
		status = http.StatusServiceUnavailable
	}
	return detailErr(c, status, err.Error())
}
