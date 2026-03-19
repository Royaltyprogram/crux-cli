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
	require.Contains(t, body, "Welcome to AutoSkills")
	require.Contains(t, body, "Install the CLI")
	require.Contains(t, body, "Run setup")
	require.Contains(t, body, "Analysis reports")
	require.Contains(t, body, "Collected session traces")
	require.Contains(t, body, "Usage Analytics")
	require.Contains(t, body, `data-action="copy-command"`)
	require.Contains(t, body, "Issued CLI tokens")
	require.Contains(t, body, "Create CLI token")
	require.Contains(t, body, "curl -fsSL")
	require.Contains(t, body, "scripts/install.sh | sh")
	require.Contains(t, body, "autoskills setup")
	require.Contains(t, body, `data-action="wizard-issue-token"`)
	require.Contains(t, body, `data-action="issue-cli-token"`)
	require.Contains(t, body, "Latest trace analysis")
	require.Contains(t, body, "Auto Skills")
	require.Contains(t, body, "Current policy documents")
	require.Contains(t, body, "Backfill jobs and failure drill-down")
	require.Contains(t, body, "Recent failed imports")
	require.Contains(t, body, "Config snapshots & recent events")
	require.Contains(t, body, "Settings")
	require.Contains(t, body, `data-action="refresh-dashboard"`)
	require.NotContains(t, body, "Approve with confidence. Measure what changed.")
	require.NotContains(t, body, `id="loginForm"`)
	require.Contains(t, body, `id="adminLink"`)
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
			bodySnippet: ".wizard {",
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `window.location.replace("/login")`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `data-action="cancel-import-job"`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `/api/v1/skill-sets/latest?project_id=`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `Shadow score`,
		},
		{
			path:        "/assets/dashboard.js",
			contentType: "javascript",
			bodySnippet: `item.applied_version`,
		},
		{
			path:        "/assets/admin.js",
			contentType: "javascript",
			bodySnippet: `/api/v1/admin/users`,
		},
		{
			path:        "/assets/admin.js",
			contentType: "javascript",
			bodySnippet: `/api/v1/admin/import-job-metrics?limit=6`,
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

func TestAdminRouteRedirectsWithoutWebSession(t *testing.T) {
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

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/", rec.Header().Get("Location"))
}

func TestAdminRouteServesPageForAdminSession(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	closeGoogle := configureGoogleAuthControllerTest(t, conf, googleAuthTestUser{
		Code:     "demo-login",
		Subject:  "google-demo-subject",
		Email:    "demo@example.com",
		Name:     "Demo Operator",
		Verified: true,
	})
	defer closeGoogle()

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	controller.NewAnalyticsRoute(controller.Options{
		AnalyticsService: analyticsSvc,
	}).RegisterRoute(echo.Group(""))
	controller.NewDashboardRoute(controller.Options{
		AnalyticsService: analyticsSvc,
	}).RegisterRoute(echo.Group(""))

	sessionCookie := loginWithGoogleControllerTest(t, echo, "demo-login")

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "User Management")
	require.Contains(t, rec.Body.String(), "Async import telemetry")
	require.Contains(t, rec.Body.String(), `<script src="/assets/admin.js"></script>`)
}

func TestAdminRouteRedirectsNonAdminToDashboard(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.Auth.BootstrapUsers = []configs.BootstrapUser{{
		ID:      "member-1",
		OrgID:   "member-org",
		OrgName: "Member Org",
		Email:   "member@example.com",
		Name:    "Member User",
		Role:    "member",
	}}
	closeGoogle := configureGoogleAuthControllerTest(t, conf, googleAuthTestUser{
		Code:     "member-login",
		Subject:  "google-member-subject",
		Email:    "member@example.com",
		Name:     "Member User",
		Verified: true,
	})
	defer closeGoogle()

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})

	echo, err := routes.NewEcho(conf, nil, store)
	require.NoError(t, err)

	controller.NewAnalyticsRoute(controller.Options{
		AnalyticsService: analyticsSvc,
	}).RegisterRoute(echo.Group(""))
	controller.NewDashboardRoute(controller.Options{
		AnalyticsService: analyticsSvc,
	}).RegisterRoute(echo.Group(""))

	sessionCookie := loginWithGoogleControllerTest(t, echo, "member-login")

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()

	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/dashboard", rec.Header().Get("Location"))
}
