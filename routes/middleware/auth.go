package middleware

import (
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/liushuangls/go-server-template/pkg/ecode"
)

const APIAuthHeader = "X-AgentOpt-Token"

func RequireAPIToken(configuredToken string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if configuredToken == "" {
				return next(c)
			}

			path := c.Request().URL.Path
			if !strings.HasPrefix(path, "/api/v1/") {
				return next(c)
			}

			if c.Request().Header.Get(APIAuthHeader) != configuredToken {
				return ecode.Unauthorized(1001, "invalid api token")
			}

			return next(c)
		}
	}
}
