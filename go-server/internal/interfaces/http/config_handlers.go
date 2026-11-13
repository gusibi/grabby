package httpiface

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	appai "go-server/internal/application/ai"
	"go-server/internal/application/backup"
	"go-server/internal/domain/ai"
)

// configHandlers exposes the configuration backup/restore endpoints.
type configHandlers struct {
	deps Dependencies
}

// RegisterConfigHandlers wires export/import of the user-level configuration.
func RegisterConfigHandlers(api *echo.Group, deps Dependencies) {
	h := &configHandlers{deps: deps}
	api.GET("/config/export", h.exportConfig)
	api.POST("/config/import", h.importConfig)
}

// exportConfig downloads one JSON document with the AI settings and sources.
func (h *configHandlers) exportConfig(c echo.Context) error {
	sources, err := h.deps.DB.GetSources()
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	var settings ai.AISettings
	if h.deps.AIEngine != nil {
		settings = h.deps.AIEngine.Settings()
	}
	doc := backup.Build(settings, sources)
	raw, err := backup.Encode(doc)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	filename := fmt.Sprintf("grabby-config-%s.json", time.Now().Format("20060102-150405"))
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "application/json; charset=utf-8", raw)
}

// importConfig restores a previously exported document. The body may be the
// document itself (curl -d @grabby-config.json) or an envelope with `data` plus
// `include_ai` / `include_sources` flags.
func (h *configHandlers) importConfig(c echo.Context) error {
	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, 16<<20))
	if err != nil {
		return detailErr(c, http.StatusBadRequest, "读取上传文件失败："+err.Error())
	}
	if len(body) == 0 {
		return detailErr(c, http.StatusBadRequest, "请求体为空")
	}

	// The envelope's flags share no keys with Document, so a direct document
	// simply fails the typed decode and is used as-is below.
	var envelope struct {
		Data           json.RawMessage `json:"data"`
		IncludeAI      *bool           `json:"include_ai"`
		IncludeSources *bool           `json:"include_sources"`
	}
	raw := body
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
		raw = envelope.Data
	}

	doc, err := backup.Parse(raw)
	if err != nil {
		return detailErr(c, http.StatusBadRequest, err.Error())
	}

	opts := backup.Options{ImportAI: true, ImportSources: true}
	if envelope.IncludeAI != nil {
		opts.ImportAI = *envelope.IncludeAI
	}
	if envelope.IncludeSources != nil {
		opts.ImportSources = *envelope.IncludeSources
	}

	var reloader appai.SettingsReloader
	if h.deps.AIEngine != nil {
		reloader = h.deps.AIEngine
	}
	result, err := backup.Apply(h.deps.DB, reloader, doc, opts)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	// Re-register imported sources with the scheduler so the new schedules are
	// live without restarting the server.
	if h.deps.Scheduler != nil && result.SourcesCreated+result.SourcesUpdated > 0 {
		for _, entry := range doc.Sources {
			if src, getErr := h.deps.DB.GetSource(entry.ID); getErr == nil && src != nil {
				h.deps.Scheduler.AddOrUpdateSource(*src)
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success":         true,
		"ai_imported":     result.AIImported,
		"sources_created": result.SourcesCreated,
		"sources_updated": result.SourcesUpdated,
		"warnings":        result.Warnings,
	})
}
