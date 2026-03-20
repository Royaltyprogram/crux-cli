package controller_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/routes"
	"github.com/Royaltyprogram/aiops/routes/controller"
	"github.com/Royaltyprogram/aiops/service"
)

func TestLandingRouteServesLandingPage(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	route := controller.NewDashboardRoute(controller.Options{})
	route.RegisterRoute(echo.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "Gets Smarter Automatically")
	require.Contains(t, body, "Adaptive memory for coding agents")
	require.Contains(t, body, "Login / Sign Up")
	require.Contains(t, body, `href="/login"`)
	require.Contains(t, body, "living skill set")
	require.NotContains(t, body, "demo@example.com")
}

func TestDashboardRouteServesWorkspaceDashboard(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	route := controller.NewDashboardRoute(controller.Options{})
	route.RegisterRoute(echo.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "AutoSkills Dashboard")
	require.Contains(t, body, `<link rel="stylesheet" href="/assets/dashboard.css">`)
	require.Contains(t, body, `<script src="/assets/dashboard.js"></script>`)
	require.Contains(t, body, "Set up your workspace")
	require.Contains(t, body, "Install the CLI")
	require.Contains(t, body, "Run setup")
	require.Contains(t, body, `data-action="copy-command"`)
	require.Contains(t, body, "Issued CLI tokens")
	require.Contains(t, body, "Create CLI token")
	require.Contains(t, body, "curl -fsSL")
	require.Contains(t, body, "scripts/install.sh | sh")
	require.Contains(t, body, "autoskills setup")
	require.Contains(t, body, `data-action="onboarding-issue-token"`)
	require.Contains(t, body, `data-action="issue-cli-token"`)
	require.Contains(t, body, "Current policy documents")
	require.Contains(t, body, "Version history")
	require.Contains(t, body, `data-action="refresh-dashboard"`)
	require.Contains(t, body, `class="app-layout no-sidebar"`)
	require.Contains(t, body, `class="main-area"`)
	require.Contains(t, body, `class="hero-grid"`)
	require.NotContains(t, body, `id="loginForm"`)
	require.NotContains(t, body, `id="adminLink"`)
}

func TestDashboardAssetRoutesServeSplitAssets(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	route := controller.NewDashboardRoute(controller.Options{})
	route.RegisterRoute(echo.Group(""))

	cases := []struct {
		path        string
		contentType string
		bodySnippet string
	}{
		{
			path:        "/assets/dashboard.css",
			contentType: "text/css",
			bodySnippet: ".onboarding-inline",
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `window.location.replace("/login")`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `/api/v1/skill-sets/latest?project_id=`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `/api/v1/dashboard/token-impact?project_id=`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `renderAutoSkillsHero`,
		},
		{
			path:        "/assets/logo.ico",
			contentType: "image/",
			bodySnippet: "",
		},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()

		echo.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, tc.path)
		require.Contains(t, rec.Header().Get("Content-Type"), tc.contentType, tc.path)
		if tc.bodySnippet != "" {
			require.Contains(t, rec.Body.String(), tc.bodySnippet, tc.path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/admin.js", nil)
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDashboardRouteServesFavicon(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	route := controller.NewDashboardRoute(controller.Options{})
	route.RegisterRoute(echo.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/logo.ico", nil)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "image/")
	require.NotEmpty(t, rec.Body.Bytes())
}

func TestAdminRouteIsRemoved(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	route := controller.NewDashboardRoute(controller.Options{
		AnalyticsService: analyticsSvc,
	})
	route.RegisterRoute(echo.Group(""))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
