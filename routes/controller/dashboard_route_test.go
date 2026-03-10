package controller_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/liushuangls/go-server-template/configs"
	"github.com/liushuangls/go-server-template/routes"
	"github.com/liushuangls/go-server-template/routes/controller"
	"github.com/liushuangls/go-server-template/service"
)

func TestLandingRouteServesLandingPage(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "agentopt-store.json")

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
	require.Contains(t, rec.Body.String(), "Approve with confidence. Measure what changed.")
	require.Contains(t, rec.Body.String(), "Sign in to dashboard")
	require.Contains(t, rec.Body.String(), `id="loginForm"`)
	require.Contains(t, rec.Body.String(), "Run `agentopt login` and paste the token")
	require.Contains(t, rec.Body.String(), "closed beta")
	require.NotContains(t, rec.Body.String(), "demo@example.com")
}

func TestDashboardRouteServesWorkspaceDashboard(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "agentopt-store.json")

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
	require.Contains(t, rec.Body.String(), "Review AI usage and approve recommended changes for your workspace.")
	require.Contains(t, rec.Body.String(), `<link rel="stylesheet" href="/assets/dashboard.css">`)
	require.Contains(t, rec.Body.String(), `<script src="/assets/dashboard.js"></script>`)
	require.Contains(t, rec.Body.String(), "Suggested changes")
	require.Contains(t, rec.Body.String(), "Recent sessions")
	require.Contains(t, rec.Body.String(), "AI usage history")
	require.Contains(t, rec.Body.String(), `data-action="copy-command"`)
	require.Contains(t, rec.Body.String(), "Issued CLI tokens")
	require.Contains(t, rec.Body.String(), "Create CLI token")
	require.Contains(t, rec.Body.String(), `data-action="issue-cli-token"`)
	require.Contains(t, rec.Body.String(), "Overview")
	require.Contains(t, rec.Body.String(), "CLI Access")
	require.Contains(t, rec.Body.String(), `data-action="refresh-dashboard"`)
	require.NotContains(t, rec.Body.String(), "Approve with confidence. Measure what changed.")
	require.NotContains(t, rec.Body.String(), `id="loginForm"`)
}

func TestDashboardAssetRoutesServeSplitAssets(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "agentopt-store.json")

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
			bodySnippet: ".wizard {",
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `window.location.replace("/")`,
		},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()

		echo.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, tc.path)
		require.Contains(t, rec.Header().Get("Content-Type"), tc.contentType, tc.path)
		require.Contains(t, rec.Body.String(), tc.bodySnippet, tc.path)
	}
}
