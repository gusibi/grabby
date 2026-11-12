package httpiface

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"go-server/internal/config"
)

const adminSessionCookie = "grabby_admin_session"

type adminAuth struct {
	settings *config.Settings
	// Fallback for the headless server (and unit tests), where no shared
	// Credentials object is attached to Settings.
	adminKey string
	apiToken string
}

func newAdminAuth(settings *config.Settings) adminAuth {
	a := adminAuth{settings: settings}
	if settings != nil {
		a.adminKey, a.apiToken = settings.AdminKey, settings.APIToken
	}
	return a
}

// credentials reads the shared, mutable credential pair when present. This is
// what lets the desktop settings page turn a token on/off without a restart.
func (a adminAuth) credentials() (string, string) {
	if a.settings != nil && a.settings.Credentials != nil {
		return a.settings.Credentials.Get()
	}
	return a.adminKey, a.apiToken
}

func (a adminAuth) updateCredentials(adminKey, apiToken string) {
	if a.settings != nil && a.settings.Credentials != nil {
		a.settings.Credentials.Set(adminKey, apiToken)
	}
}

func (a adminAuth) enabled() bool {
	adminKey, apiToken := a.credentials()
	return adminKey != "" || apiToken != ""
}

// desktopLocal reports whether this request originates from the machine the
// desktop app runs on. The app's own WebView, local CLI, MCP clients and the
// browser extension all connect over loopback, so they must never be asked for
// a token. Remote callers (only reachable when the app is bound to 0.0.0.0)
// still go through the normal token check.
func (a adminAuth) desktopLocal(r *http.Request) bool {
	if a.settings == nil || !a.settings.DesktopMode {
		return false
	}
	return isLoopbackAddr(r.RemoteAddr)
}

func isLoopbackAddr(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackBind(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a adminAuth) isAuthenticated(r *http.Request) bool {
	if !a.enabled() {
		return true
	}
	if a.desktopLocal(r) {
		return true
	}
	if a.hasValidToken(r) {
		return true
	}
	adminKey, _ := a.credentials()
	if adminKey == "" {
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
		if !a.enabled() || a.desktopLocal(c.Request()) || a.hasValidToken(c.Request()) {
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
	adminKey, apiToken := a.credentials()
	if apiToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(apiToken)) == 1 {
		return true
	}
	if adminKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(adminKey)) == 1 {
		return true
	}
	return false
}

func (a adminAuth) sessionToken() string {
	adminKey, _ := a.credentials()
	mac := hmac.New(sha256.New, []byte(adminKey))
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
		// On desktop, loopback callers are already trusted; never make the
		// bundled UI type an admin key.
		if !auth.enabled() || auth.desktopLocal(c.Request()) {
			return c.JSON(http.StatusOK, map[string]any{
				"success":       true,
				"auth_required": auth.enabled(),
				"authenticated": true,
			})
		}

		var req struct {
			Key string `json:"key"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		}
		adminKey, _ := auth.credentials()
		if subtle.ConstantTimeCompare([]byte(req.Key), []byte(adminKey)) != 1 {
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

	if deps.Settings != nil && deps.Settings.DesktopMode {
		registerDesktopSettings(group, deps, auth)
	}
}

// registerDesktopSettings exposes the preferences owned by the desktop shell.
// These routes sit behind the normal /api middleware, so a remote caller can
// only reach them with a valid token; the local WebView is always allowed.
func registerDesktopSettings(group *echo.Group, deps Dependencies, auth adminAuth) {
	snapshot := func() map[string]any {
		cfg := config.LoadDesktopSettings()
		_, token := auth.credentials()
		host := deps.Settings.Host
		envHost := os.Getenv("GRABBY_DESKTOP_HOST")
		envForcedExternal := envHost != "" && !isLoopbackBind(envHost)
		activeExternal := !isLoopbackBind(host)
		return map[string]any{
			"success":             true,
			"desktop_mode":        true,
			"auto_start":          true,
			"data_dir":            config.DesktopDataDir(),
			"host":                host,
			"port":                deps.Settings.Port,
			"active_external":     activeExternal,
			"env_forced_external": envForcedExternal,
			"external_enabled":    cfg.ExternalEnabled || activeExternal,
			"token_enabled":       token != "",
			"api_token":           token,
			"auth_required":       auth.enabled(),
			"port_shifted":        os.Getenv("GRABBY_DESKTOP_PORT_SHIFTED") == "1",
			"restart_required":    cfg.ExternalEnabled != activeExternal,
		}
	}

	group.GET("/desktop/settings", func(c echo.Context) error {
		return c.JSON(http.StatusOK, snapshot())
	})

	group.POST("/desktop/settings", func(c echo.Context) error {
		var req struct {
			ExternalEnabled *bool   `json:"external_enabled"`
			APIToken        *string `json:"api_token"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		}

		cfg := config.LoadDesktopSettings()
		if req.ExternalEnabled != nil {
			cfg.ExternalEnabled = *req.ExternalEnabled
		}
		if req.APIToken != nil {
			cfg.APIToken = strings.TrimSpace(*req.APIToken)
		}
		if cfg.ExternalEnabled && cfg.APIToken == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"success": false,
				"error":   "开放局域网访问时必须设置 API Token",
			})
		}
		if err := config.SaveDesktopSettings(cfg); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{
				"success": false,
				"error":   "无法保存设置: " + err.Error(),
			})
		}

		// Apply the token immediately; the app's own UI keeps working because
		// loopback requests bypass the token check.
		auth.updateCredentials("", cfg.APIToken)
		return c.JSON(http.StatusOK, snapshot())
	})
}
