package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	dtoresponse "github.com/Royaltyprogram/aiops/dto/response"
	"github.com/Royaltyprogram/aiops/pkg/buildinfo"
	"github.com/Royaltyprogram/aiops/routes/controller"
	"github.com/Royaltyprogram/aiops/service"
)

func TestHealthzAndReadyzEndpoints(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	originalDate := buildinfo.Date
	buildinfo.Version = "1.2.3-beta.4"
	buildinfo.Commit = "abc1234"
	buildinfo.Date = "2026-03-09T15:00:00Z"
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
		buildinfo.Date = originalDate
	})

	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
	})
	engine.RegisterRoute()

	for _, tc := range []struct {
		path   string
		status string
	}{
		{path: "/healthz", status: "ok"},
		{path: "/readyz", status: "ready"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		echo.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, tc.path)

		var payload struct {
			Code int                   `json:"code"`
			Data dtoresponse.ProbeResp `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), tc.path)
		require.Equal(t, 0, payload.Code, tc.path)
		require.Equal(t, tc.status, payload.Data.Status, tc.path)
		require.Equal(t, "1.2.3-beta.4", payload.Data.Version, tc.path)
		require.Equal(t, "abc1234", payload.Data.Commit, tc.path)
		require.Equal(t, "2026-03-09T15:00:00Z", payload.Data.BuildDate, tc.path)
	}
}

func TestNewEchoAppliesConfiguredCORSHeaders(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.AllowedOrigins = []string{"https://beta.example.com"}

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Analytics: controller.NewAnalyticsRoute(controller.Options{
			AnalyticsService: analyticsSvc,
		}),
	})
	engine.RegisterRoute()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/google/start", nil)
	req.Header.Set("Origin", "https://beta.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "https://beta.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestNewEchoRateLimitsAPIWhenConfigured(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.RateLimitPerMinute = 1

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Analytics: controller.NewAnalyticsRoute(controller.Options{
			AnalyticsService: analyticsSvc,
		}),
	})
	engine.RegisterRoute()

	first := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil)
	firstRec := httptest.NewRecorder()
	echo.ServeHTTP(firstRec, first)
	require.Equal(t, http.StatusSeeOther, firstRec.Code)

	second := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil)
	secondRec := httptest.NewRecorder()
	echo.ServeHTTP(secondRec, second)
	require.Equal(t, http.StatusTooManyRequests, secondRec.Code)

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
	}
	require.NoError(t, json.Unmarshal(secondRec.Body.Bytes(), &payload))
	require.Equal(t, 429, payload.Code)
	require.Equal(t, "Too Many Request", payload.Message)
}

func TestNewEchoSeparatesBulkImportRateLimiters(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.RateLimitPerMinute = 1
	conf.HTTP.BulkImportRateLimitPerMinute = 2

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 1,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Analytics: controller.NewAnalyticsRoute(controller.Options{
			AnalyticsService: analyticsSvc,
		}),
	})
	engine.RegisterRoute()

	firstAPIReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil)
	firstAPIRec := httptest.NewRecorder()
	echo.ServeHTTP(firstAPIRec, firstAPIReq)
	require.Equal(t, http.StatusSeeOther, firstAPIRec.Code)

	secondAPIReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil)
	secondAPIRec := httptest.NewRecorder()
	echo.ServeHTTP(secondAPIRec, secondAPIReq)
	require.Equal(t, http.StatusTooManyRequests, secondAPIRec.Code)

	for attempt := 1; attempt <= 2; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/session-import-jobs", nil)
		rec := httptest.NewRecorder()
		echo.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "bulk request %d should use its own bucket", attempt)
	}

	thirdBulkReq := httptest.NewRequest(http.MethodPost, "/api/v1/session-import-jobs", nil)
	thirdBulkRec := httptest.NewRecorder()
	echo.ServeHTTP(thirdBulkRec, thirdBulkReq)
	require.Equal(t, http.StatusTooManyRequests, thirdBulkRec.Code)
}

func TestNewEchoRejectsRequestsOutsideAllowedCIDRs(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.AllowedCIDRs = []string{"10.0.0.0/8"}

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
	})
	engine.RegisterRoute()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNewEchoAllowsRequestsWithinAllowedCIDRs(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.AllowedCIDRs = []string{"127.0.0.1/32"}

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
	})
	engine.RegisterRoute()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewEchoUsesTrustedProxyCIDRsForAllowlist(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.AllowedCIDRs = []string{"203.0.113.0/24"}
	conf.HTTP.TrustedProxyCIDRs = []string{"127.0.0.1/32"}

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
	})
	engine.RegisterRoute()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewEchoDoesNotApplyAdminAllowedCIDRsToHealthRoute(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")
	conf.HTTP.AdminAllowedCIDRs = []string{"10.0.0.0/8"}

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)

	echo, err := NewEcho(conf, slog.Default(), store)
	require.NoError(t, err)

	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
	})
	engine.RegisterRoute()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
