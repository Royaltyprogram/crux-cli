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
	require.Contains(t, rec.Body.String(), "demo@example.com")
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
	require.Contains(t, rec.Body.String(), "Review changes, approve safely, and connect each CLI with its own token.")
	require.Contains(t, rec.Body.String(), "Recommended changes for this workspace")
	require.Contains(t, rec.Body.String(), "Avg Tokens / Query")
	require.Contains(t, rec.Body.String(), "Install and connect the CLI")
	require.Contains(t, rec.Body.String(), "agentopt use-project --project-id")
	require.Contains(t, rec.Body.String(), "agentopt pending --project-id")
	require.Contains(t, rec.Body.String(), "agentopt sync --project-id")
	require.Contains(t, rec.Body.String(), `data-action="copy-command"`)
	require.Contains(t, rec.Body.String(), "Manage issued CLI access")
	require.Contains(t, rec.Body.String(), `data-action="issue-cli-token"`)
	require.Contains(t, rec.Body.String(), "What improved after rollout")
	require.Contains(t, rec.Body.String(), "Focus View")
	require.Contains(t, rec.Body.String(), `data-action="refresh-dashboard"`)
	require.Contains(t, rec.Body.String(), `window.location.replace("/")`)
	require.NotContains(t, rec.Body.String(), "Approve with confidence. Measure what changed.")
	require.NotContains(t, rec.Body.String(), `id="loginForm"`)
}
