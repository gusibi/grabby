package httpiface

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
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

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   "admin login required",
	})
}

func RegisterAuthHandlers(group *echo.Group, deps Dependencies) {
	auth := newAdminAuth(deps.Settings)

	group.GET("/auth/session", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"auth_required": auth.enabled(),
			"authenticated": auth.isAuthenticated(r),
		})
	}))

	group.POST("/auth/login", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !auth.enabled() {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":       true,
				"auth_required": false,
				"authenticated": true,
			})
			return
		}

		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"success":false,"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(req.Key), []byte(auth.adminKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "invalid admin key",
			})
			return
		}

		auth.setSessionCookie(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"auth_required": true,
			"authenticated": true,
		})
	}))

	group.POST("/auth/logout", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		clearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"authenticated": false,
		})
	}))
}
