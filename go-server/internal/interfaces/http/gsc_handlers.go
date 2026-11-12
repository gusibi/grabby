package httpiface

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/gsc"
	"go-server/internal/interfaces/dto"
)

// gscHandlers is the HTTP entry to the Search Console adapter.
//
// Deliberately browser-only (see the package doc): we never call Google's
// documented API and never send server-side requests. Each call opens the real
// console in the user's browser and reads what the page itself requested.
type gscHandlers struct {
	deps Dependencies
}

// RegisterGSCHandlers wires the GSC endpoints.
func RegisterGSCHandlers(api *echo.Group, deps Dependencies) {
	h := &gscHandlers{deps: deps}
	// Stage 0: capture real payloads so parsers are written against reality.
	api.POST("/gsc/record", h.record)
}

func (h *gscHandlers) record(c echo.Context) error {
	var req dto.GSCRecordAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}

	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = "performance"
	}
	if kind != "sites" && kind != "performance" {
		return detailErr(c, http.StatusBadRequest, "kind must be sites or performance")
	}
	if kind == "performance" && strings.TrimSpace(req.Property) == "" {
		return detailErr(c, http.StatusBadRequest,
			"property is required: sc-domain:example.com (Domain property) or https://www.example.com/ (URL-prefix property)")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(),
		time.Duration(h.deps.Settings.APIExtractTimeout*float64(time.Second)))
	defer cancel()

	opts := gsc.Options{Browser: req.Browser, Visible: req.Visible, Days: req.Days}

	var (
		res *gsc.CaptureResult
		err error
	)
	if kind == "sites" {
		res, err = gsc.RecordSites(ctx, h.deps.WSManager, h.deps.Logger, opts)
	} else {
		res, err = gsc.RecordPerformance(ctx, h.deps.WSManager, h.deps.Logger, req.Property, opts)
	}

	// Dump before error handling: an empty capture is exactly the case where the
	// raw file is most useful for diagnosis.
	dumpPath := ""
	dump := true
	if req.Dump != nil {
		dump = *req.Dump
	}
	if dump && res != nil {
		name := "gsc-" + kind
		if res.Property != "" {
			name += "-" + res.Property
		}
		path, derr := writeRawDump(name, map[string]any{
			"kind":        res.Kind,
			"property":    res.Property,
			"page_url":    res.PageURL,
			"captured_at": time.Now().UTC().Format(time.RFC3339),
			"captures":    res.Captures,
		})
		if derr != nil {
			h.deps.Logger.Warn("failed to write gsc raw dump", zap.Error(derr))
		} else {
			dumpPath = path
		}
	}

	if err != nil {
		msg := err.Error()
		if dumpPath != "" {
			msg += " (raw dump: " + dumpPath + ")"
		}
		status := http.StatusBadGateway
		if strings.Contains(msg, "browser not available") {
			status = http.StatusServiceUnavailable
		}
		if strings.Contains(msg, "REQUEST_NOT_SEEN") {
			status = http.StatusGatewayTimeout
		}
		return detailErr(c, status, msg)
	}

	summaries := make([]dto.GSCCaptureSummary, 0, len(res.Captures))
	for _, cap := range res.Captures {
		summaries = append(summaries, dto.GSCCaptureSummary{
			URL:           cap.URL,
			Status:        cap.Status,
			Method:        cap.Method,
			BodyBytes:     cap.BodyBytes,
			BodyTruncated: cap.BodyTruncated,
			RPCIDs:        cap.RPCIDs,
		})
	}

	return c.JSON(http.StatusOK, dto.GSCRecordAPIResponse{
		Success:  true,
		Kind:     res.Kind,
		Property: res.Property,
		PageURL:  res.PageURL,
		Matched:  len(summaries),
		Captures: summaries,
		DumpPath: dumpPath,
	})
}
