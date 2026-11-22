package httpiface

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/interfaces/dto"
)

func TestDeleteOfflineBrowserRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browsers.json")
	registry, err := browserregistry.NewBrowserRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	// Old data can contain one device's ID as another device's name.
	if _, err := registry.Register("stale-id", "active-id"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Ban("stale-id"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Register("active-id", "stale-id"); err != nil {
		t.Fatal(err)
	}
	logger := zap.NewNop()
	manager := browserws.NewWebSocketManager(logger)
	manager.Connect("active-id", nil)
	e := echo.New()
	RegisterBrowserHandlers(e.Group("/api"), Dependencies{BrowserRegistry: registry, WSManager: manager, Logger: logger})

	for _, tc := range []struct {
		body string
		want int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"conn_id":"active-id"}`, http.StatusConflict},
		{`{"conn_id":"stale-id"}`, http.StatusOK},
		{`{"conn_id":"stale-id"}`, http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/browsers/delete", strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s returned %d, want %d (%s)", tc.body, w.Code, tc.want, w.Body)
		}
	}

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/browsers", nil))
	var out dto.BrowserListResponse
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 || len(out.Browsers) != 1 || out.Browsers[0].ConnID != "active-id" || !out.Browsers[0].Online {
		t.Fatalf("unexpected browser list: %+v", out)
	}

	reloaded, err := browserregistry.NewBrowserRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := reloaded.GetName("stale-id"); exists {
		t.Fatal("deleted registration returned after loading the registry from disk")
	}
	if reg, err := reloaded.Register("stale-id", "active-id"); err != nil || reg.Banned {
		t.Fatalf("could not register a fresh device: %+v (%v)", reg, err)
	}
}
