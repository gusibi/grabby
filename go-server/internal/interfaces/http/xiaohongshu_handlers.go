package httpiface

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/xiaohongshu"
	"go-server/internal/infrastructure/sqlite"
	"go-server/internal/interfaces/dto"
)

// xhsHandlers is the HTTP entry to the Xiaohongshu site adapter. It mirrors
// the MCP xiaohongshu_* tools: same adapter, same primitives (runPageScript for
// note detail, intercept for search/user notes), exposed over HTTP.
type xhsHandlers struct {
	deps Dependencies
}

// RegisterXiaohongshuHandlers wires the Xiaohongshu HTTP endpoints.
func RegisterXiaohongshuHandlers(api *echo.Group, deps Dependencies) {
	h := &xhsHandlers{deps: deps}
	api.POST("/xiaohongshu/note", h.note)
	api.POST("/xiaohongshu/search", h.search)
	api.POST("/xiaohongshu/user_notes", h.userNotes)
}

func (h *xhsHandlers) note(c echo.Context) error {
	var req dto.XhsNoteAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.URL == "" {
		return detailErr(c, http.StatusBadRequest, "url is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	note, err := xiaohongshu.FetchNote(ctx, h.deps.WSManager, h.deps.Logger, req.URL, xiaohongshu.Options{Browser: req.Browser})
	if err != nil {
		return xhsErr(c, err)
	}
	h.archiveNotes([]xiaohongshu.Note{*note})
	return c.JSON(http.StatusOK, dto.XhsNoteAPIResponse{Success: true, Note: note})
}

func (h *xhsHandlers) search(c echo.Context) error {
	var req dto.XhsSearchAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.Query == "" {
		return detailErr(c, http.StatusBadRequest, "query is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	notes, err := xiaohongshu.Search(ctx, h.deps.WSManager, h.deps.Logger, req.Query, xiaohongshu.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return xhsErr(c, err)
	}
	h.archiveNotes(notes)
	return c.JSON(http.StatusOK, dto.XhsSearchAPIResponse{Success: true, Count: len(notes), Notes: xhsNotesToAny(notes)})
}

func (h *xhsHandlers) userNotes(c echo.Context) error {
	var req dto.XhsUserNotesAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.URL == "" {
		return detailErr(c, http.StatusBadRequest, "url is required")
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()
	notes, err := xiaohongshu.UserNotes(ctx, h.deps.WSManager, h.deps.Logger, req.URL, xiaohongshu.Options{Browser: req.Browser, Limit: req.Limit})
	if err != nil {
		return xhsErr(c, err)
	}
	h.archiveNotes(notes)
	return c.JSON(http.StatusOK, dto.XhsUserNotesAPIResponse{Success: true, Count: len(notes), Notes: xhsNotesToAny(notes)})
}

func (h *xhsHandlers) withTimeout(c echo.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request().Context(), time.Duration(h.deps.Settings.APIExtractTimeout*float64(time.Second)))
}

// archiveNotes upserts fetched notes into the xhs_notes table for later reuse.
// Storage is best-effort: a failure is logged but never fails the request.
func (h *xhsHandlers) archiveNotes(notes []xiaohongshu.Note) {
	if len(notes) == 0 {
		return
	}
	now := time.Now()
	records := make([]sqlite.XhsNote, 0, len(notes))
	for _, n := range notes {
		records = append(records, sqlite.XhsNote{
			ID:             n.ID,
			Title:          n.Title,
			Desc:           n.Desc,
			Type:           n.Type,
			Author:         n.Author,
			AuthorID:       n.AuthorID,
			LikedCount:     n.LikedCount,
			CollectedCount: n.CollectedCount,
			CommentCount:   n.CommentCount,
			ShareCount:     n.ShareCount,
			URL:            n.URL,
			Images:         n.Images,
			FetchedAt:      now,
		})
	}
	if err := h.deps.DB.SaveXhsNotes(records); err != nil {
		h.deps.Logger.Warn("failed to archive xiaohongshu notes", zap.Int("count", len(records)), zap.Error(err))
	}
}

func xhsNotesToAny(notes []xiaohongshu.Note) []any {
	out := make([]any, len(notes))
	for i, n := range notes {
		out[i] = n
	}
	return out
}

// xhsErr maps a xiaohongshu adapter error to an HTTP status. A missing browser
// is 503; anything else is 502.
func xhsErr(c echo.Context, err error) error {
	status := http.StatusBadGateway
	if strings.Contains(err.Error(), "browser not available") {
		status = http.StatusServiceUnavailable
	}
	return detailErr(c, status, err.Error())
}
