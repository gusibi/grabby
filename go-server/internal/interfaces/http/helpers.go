package httpiface

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// wrapHandlerFunc wraps a standard http.HandlerFunc into echo.HandlerFunc.
func wrapHandlerFunc(f http.HandlerFunc) echo.HandlerFunc {
	return echo.WrapHandler(f)
}
