package httpiface

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// wrapHandlerFunc wraps a standard http.HandlerFunc into echo.HandlerFunc.
//
// All JSON API handlers are native Echo now; this bridge is only for the
// genuinely net/http endpoints: the static SPA file server and the WebSocket
// upgrade handlers (ws_browser / ws_command). New JSON handlers should use
// echo.HandlerFunc directly with c.Param / c.QueryParam / c.Bind / c.JSON.
func wrapHandlerFunc(f http.HandlerFunc) echo.HandlerFunc {
	return echo.WrapHandler(f)
}

// detailErr writes the project's standard `{"detail": "..."}` error body with
// the given status. Echo handles JSON escaping, so callers pass raw messages.
func detailErr(c echo.Context, code int, detail string) error {
	return c.JSON(code, map[string]string{"detail": detail})
}

// isTrue reports whether a query/form flag means true ("1" or "true").
func isTrue(v string) bool {
	return v == "1" || v == "true"
}
