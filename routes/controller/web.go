package controller

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v5"
)

//go:embed assets/dashboard.html assets/landing.html
var uiFS embed.FS

type DashboardRoute struct {
	Options
}

func NewDashboardRoute(opt Options) *DashboardRoute {
	return &DashboardRoute{Options: opt}
}

func (r *DashboardRoute) RegisterRoute(router *echo.Group) {
	router.GET("/", r.landing)
	router.GET("/dashboard", r.dashboard)
}

func (r *DashboardRoute) landing(c *echo.Context) error {
	page, err := uiFS.ReadFile("assets/landing.html")
	if err != nil {
		return err
	}
	return c.HTML(http.StatusOK, string(page))
}

func (r *DashboardRoute) dashboard(c *echo.Context) error {
	page, err := uiFS.ReadFile("assets/dashboard.html")
	if err != nil {
		return err
	}
	return c.HTML(http.StatusOK, string(page))
}
