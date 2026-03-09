package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/require"

	"github.com/liushuangls/go-server-template/configs"
	"github.com/liushuangls/go-server-template/dto/request"
	"github.com/liushuangls/go-server-template/dto/response"
	"github.com/liushuangls/go-server-template/routes/controller"
	"github.com/liushuangls/go-server-template/service"
)

type testEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func TestRequireAPITokenProtectsAnalyticsAPI(t *testing.T) {
	conf := &configs.Config{}
	conf.App.APIToken = "secret-token"
	conf.App.StorePath = filepath.Join(t.TempDir(), "agentopt-store.json")

	echo, err := NewEcho(conf, slog.Default())
	require.NoError(t, err)

	store, err := service.NewAnalyticsStore(conf)
	require.NoError(t, err)
	analyticsSvc := service.NewAnalyticsService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})
	healthSvc := service.NewHealthService(service.Options{
		Config:         conf,
		AnalyticsStore: store,
	})

	engine := NewHttpEngine(Options{
		Router: echo,
		Conf:   conf,
		Health: controller.NewHealthRoute(controller.Options{HealthService: healthSvc}),
		Analytics: controller.NewAnalyticsRoute(controller.Options{
			AnalyticsService: analyticsSvc,
		}),
	})
	engine.RegisterRoute()

	healthReq := httptest.NewRequest(http.MethodGet, "/health?message=ok", nil)
	healthRec := httptest.NewRecorder()
	echo.ServeHTTP(healthRec, healthReq)
	require.Equal(t, http.StatusOK, healthRec.Code)

	payload, err := json.Marshal(request.RegisterAgentReq{
		OrgID:      "org-auth",
		UserID:     "user-auth",
		DeviceName: "mbp",
	})
	require.NoError(t, err)

	apiReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewReader(payload))
	apiReq = apiReq.WithContext(context.Background())
	apiReq.Header.Set("Content-Type", "application/json")
	apiRec := httptest.NewRecorder()
	echo.ServeHTTP(apiRec, apiReq)
	require.Equal(t, http.StatusUnauthorized, apiRec.Code)

	var env testEnvelope
	require.NoError(t, json.Unmarshal(apiRec.Body.Bytes(), &env))
	require.Equal(t, 1001, env.Code)

	apiReq = httptest.NewRequest(http.MethodPost, "/api/v1/agents/register", bytes.NewReader(payload))
	apiReq = apiReq.WithContext(context.Background())
	apiReq.Header.Set("Content-Type", "application/json")
	apiReq.Header.Set("X-AgentOpt-Token", "secret-token")
	apiRec = httptest.NewRecorder()
	echo.ServeHTTP(apiRec, apiReq)
	require.Equal(t, http.StatusOK, apiRec.Code)

	require.NoError(t, json.Unmarshal(apiRec.Body.Bytes(), &env))
	require.Equal(t, 0, env.Code)

	var data response.AgentRegistrationResp
	require.NoError(t, json.Unmarshal(env.Data, &data))
	require.Equal(t, "registered", data.Status)
}
