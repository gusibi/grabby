package httpiface

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/backup"
	"go-server/internal/infrastructure/sqlite"
)

func newConfigTestDB(t *testing.T) *sqlite.Database {
	t.Helper()
	db, err := sqlite.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestExportConfigReturnsDownloadableDocument(t *testing.T) {
	db := newConfigTestDB(t)
	if err := db.InsertSource(sqlite.Source{
		ID: "exported", Name: "Exported", Type: "rss", URL: "https://exported.example/feed",
		Schedule: "0 */2 * * *", Enabled: 1, DefaultCategory: "auto", Config: "{}", Category: "Tech",
	}); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	h := &configHandlers{deps: Dependencies{DB: db, Logger: zap.NewNop()}}
	req := httptest.NewRequest(http.MethodGet, "/api/config/export", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.exportConfig(c); err != nil {
		t.Fatalf("exportConfig: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if disposition := rec.Header().Get("Content-Disposition"); !strings.Contains(disposition, "grabby-config-") {
		t.Fatalf("missing attachment filename: %q", disposition)
	}

	var doc backup.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if doc.App != backup.AppName || doc.Version != backup.Version || doc.AI == nil {
		t.Fatalf("unexpected document header: %+v", doc)
	}
	found := false
	for _, src := range doc.Sources {
		if src.ID == "exported" {
			found = true
			if src.Category != "Tech" || src.URL != "https://exported.example/feed" {
				t.Fatalf("source was not exported faithfully: %+v", src)
			}
		}
	}
	if !found {
		t.Fatalf("exported document is missing the source: %+v", doc.Sources)
	}
}

func TestImportConfigAcceptsDirectDocument(t *testing.T) {
	db := newConfigTestDB(t)
	h := &configHandlers{deps: Dependencies{DB: db, Logger: zap.NewNop()}}

	body := `{
		"version": 1,
		"app": "grabby",
		"sources": [
			{"id": "restored", "name": "Restored", "type": "rss", "url": "https://restored.example/feed", "schedule": "0 * * * *", "enabled": 1, "default_category": "auto", "config": "{}", "category": "AI"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.importConfig(c); err != nil {
		t.Fatalf("importConfig: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Success        bool `json:"success"`
		SourcesCreated int  `json:"sources_created"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !res.Success || res.SourcesCreated != 1 {
		t.Fatalf("unexpected response: %+v", res)
	}
	src, err := db.GetSource("restored")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if src == nil || src.Name != "Restored" || src.Category != "AI" {
		t.Fatalf("source was not imported: %+v", src)
	}
}

func TestImportConfigRejectsInvalidBody(t *testing.T) {
	db := newConfigTestDB(t)
	h := &configHandlers{deps: Dependencies{DB: db, Logger: zap.NewNop()}}

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{"version":1,"sources":[]}`))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := h.importConfig(c); err != nil {
		t.Fatalf("importConfig: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
