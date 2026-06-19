package httpiface

import (
	"net/http"
	"strconv"

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

// pageParams reads limit/offset query params with a default limit and sane caps.
func pageParams(c echo.Context, defLimit int) (limit, offset int) {
	limit = defLimit
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > 200 {
		limit = 200
	}
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil && o > 0 {
		offset = o
	}
	return limit, offset
}
