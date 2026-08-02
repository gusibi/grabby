package httpiface

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"go-server/internal/config"
)

const adminSessionCookie = "grabby_admin_session"

type adminAuth struct {
	adminKey string
	apiToken string
}

func newAdminAuth(settings *config.Settings) adminAuth {
	if settings == nil {
		return adminAuth{}
	}
	return adminAuth{adminKey: settings.AdminKey, apiToken: settings.APIToken}
}

func (a adminAuth) enabled() bool {
	return a.adminKey != "" || a.apiToken != ""
}

func (a adminAuth) isAuthenticated(r *http.Request) bool {
	if !a.enabled() {
		return true
	}
	if a.hasValidToken(r) {
		return true
	}
	if a.adminKey == "" {
		return false
	}
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(a.sessionToken())) == 1
}

func (a adminAuth) middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if a.isAuthRoute(c.Request().URL.Path) || a.isAuthenticated(c.Request()) {
			return next(c)
		}
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"success": false,
			"error":   "admin login required",
		})
	}
}

// tokenMiddleware protects machine-to-machine endpoints (MCP). Unlike
// middleware it never accepts the browser session cookie: an agent must present
// API_TOKEN (or ADMIN_KEY) via X-API-Key / X-Grabby-Token / Authorization Bearer.
// When no token is configured at all the server is open, matching middleware.
func (a adminAuth) tokenMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !a.enabled() || a.hasValidToken(c.Request()) {
			return next(c)
		}
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"success": false,
			"error":   "missing or invalid API token",
		})
	}
}

func (a adminAuth) isAuthRoute(path string) bool {
	return path == "/api/auth/session" || path == "/api/auth/login" || path == "/api/auth/logout" || path == "/api/browsers/register"
}

func (a adminAuth) hasValidToken(r *http.Request) bool {
	token := r.Header.Get("X-API-Key")
	if token == "" {
		token = r.Header.Get("X-Grabby-Token")
	}
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[len("bearer "):])
		}
	}
	if token == "" {
		return false
	}
	if a.apiToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.apiToken)) == 1 {
		return true
	}
	if a.adminKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.adminKey)) == 1 {
		return true
	}
	return false
}

func (a adminAuth) sessionToken() string {
	mac := hmac.New(sha256.New, []byte(a.adminKey))
	mac.Write([]byte("grabby-admin-session"))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a adminAuth) setSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    a.sessionToken(),
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func RegisterAuthHandlers(group *echo.Group, deps Dependencies) {
	auth := newAdminAuth(deps.Settings)

	group.GET("/auth/session", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"success":       true,
			"auth_required": auth.enabled(),
			"authenticated": auth.isAuthenticated(c.Request()),
		})
	})

	group.POST("/auth/login", func(c echo.Context) error {
		if !auth.enabled() {
			return c.JSON(http.StatusOK, map[string]any{
				"success":       true,
				"auth_required": false,
				"authenticated": true,
			})
		}

		var req struct {
			Key string `json:"key"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		}
		if subtle.ConstantTimeCompare([]byte(req.Key), []byte(auth.adminKey)) != 1 {
			return c.JSON(http.StatusUnauthorized, map[string]any{"success": false, "error": "invalid admin key"})
		}

		auth.setSessionCookie(c.Response())
		return c.JSON(http.StatusOK, map[string]any{
			"success":       true,
			"auth_required": true,
			"authenticated": true,
		})
	})

	group.POST("/auth/logout", func(c echo.Context) error {
		clearSessionCookie(c.Response())
		return c.JSON(http.StatusOK, map[string]any{
			"success":       true,
			"authenticated": false,
		})
	})
}
