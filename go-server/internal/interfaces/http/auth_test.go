package httpiface

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"go-server/internal/config"
)

func TestAdminAuthMiddlewareRejectsMissingSession(t *testing.T) {
	auth := newAdminAuth(&config.Settings{AdminKey: "secret"})
	e := echo.New()
	api := e.Group("/api")
	api.Use(auth.middleware)
	api.POST("/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/protected", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuthLoginSetsUsableSessionCookie(t *testing.T) {
	settings := &config.Settings{AdminKey: "secret"}
	e := echo.New()
	api := e.Group("/api")
	auth := newAdminAuth(settings)
	api.Use(auth.middleware)
	RegisterAuthHandlers(api, Dependencies{Settings: settings})
	api.POST("/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"key":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	e.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one auth cookie, got %d", len(cookies))
	}

	protectedReq := httptest.NewRequest(http.MethodPost, "/api/protected", nil)
	protectedReq.AddCookie(cookies[0])
	protectedRec := httptest.NewRecorder()

	e.ServeHTTP(protectedRec, protectedReq)

	if protectedRec.Code != http.StatusNoContent {
		t.Fatalf("expected protected request 204, got %d: %s", protectedRec.Code, protectedRec.Body.String())
	}
}

func TestAdminAuthMiddlewareAcceptsBearerToken(t *testing.T) {
	auth := newAdminAuth(&config.Settings{AdminKey: "secret", APIToken: "api-secret"})
	e := echo.New()
	api := e.Group("/api")
	api.Use(auth.middleware)
	api.POST("/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer api-secret")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected protected request 204, got %d: %s", rec.Code, rec.Body.String())
	}
}
