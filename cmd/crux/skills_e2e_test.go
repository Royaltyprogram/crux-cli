package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/dto/request"
	"github.com/Royaltyprogram/aiops/dto/response"
	"github.com/Royaltyprogram/aiops/routes"
	"github.com/Royaltyprogram/aiops/routes/controller"
	"github.com/Royaltyprogram/aiops/service"
)

func TestRunCollectEndToEndManagedSkillSetReflectsInDashboardAPI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AUTOSKILLS_HOME", root)

	repoPath := filepath.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	conf, server := newManagedSkillSetEndToEndServer(t)
	defer server.Close()

	client := &apiClient{
		baseURL: server.URL,
		token:   conf.App.APIToken,
		http:    server.Client(),
	}

	st, projectResp := registerManagedSkillSetEndToEndWorkspace(t, client, repoPath)
	require.NoError(t, saveState(st))

	codexHome := filepath.Join(root, ".codex")
	writeCodexSessionFixture(t, filepath.Join(codexHome, "sessions", "2026", "03", "17", "latest.jsonl"), time.Date(2026, 3, 17, 8, 0, 0, 0, time.UTC), []string{
		`{"timestamp":"2026-03-17T08:00:00Z","type":"session_meta","payload":{"id":"codex-session-skill-e2e","timestamp":"2026-03-17T08:00:00Z","model_provider":"openai"}}`,
		`{"timestamp":"2026-03-17T08:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"## My request for Codex:\nStart with concrete file discovery before summarizing control flow."}}`,
		`{"timestamp":"2026-03-17T08:00:02Z","type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"**Checking current control flow and test expectations before patching.**"}]}}`,
		`{"timestamp":"2026-03-17T08:00:03Z","type":"event_msg","payload":{"type":"agent_message","message":"I will start with file discovery and keep the patch narrow."}}`,
		`{"timestamp":"2026-03-17T08:00:04Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":700,"cached_input_tokens":180,"output_tokens":220,"reasoning_output_tokens":40,"total_tokens":920}}}}`,
	})

	firstOutput := captureStdout(t, func() {
		require.NoError(t, runCollect([]string{"--codex-home", codexHome, "--snapshot-mode", "skip"}))
	})
	var firstCollect collectRunResp
	require.NoError(t, json.Unmarshal([]byte(firstOutput), &firstCollect))
	require.NotNil(t, firstCollect.SkillSet)

	readyBundle := waitForManagedSkillSetBundleReady(t, client, projectResp.ProjectID)
	require.Equal(t, "ready", readyBundle.Status)
	require.NotEmpty(t, readyBundle.Version)
	require.NotEmpty(t, readyBundle.CompiledHash)
	require.NotEmpty(t, readyBundle.Files)

	secondOutput := captureStdout(t, func() {
		require.NoError(t, runCollect([]string{"--codex-home", codexHome, "--snapshot-mode", "skip"}))
	})
	var secondCollect collectRunResp
	require.NoError(t, json.Unmarshal([]byte(secondOutput), &secondCollect))
	require.NotNil(t, secondCollect.SkillSet)
	require.Contains(t, []string{"synced", "unchanged"}, secondCollect.SkillSet.Status)

	liveBundlePath := filepath.Join(codexHome, "skills", managedSkillBundleName)
	skillIndex, err := os.ReadFile(filepath.Join(liveBundlePath, "SKILL.md"))
	require.NoError(t, err)
	require.Contains(t, string(skillIndex), "AutoSkills Personal Skill Set")

	currentState, _, err := loadSkillSetState()
	require.NoError(t, err)
	require.Equal(t, readyBundle.Version, currentState.AppliedVersion)
	require.Equal(t, readyBundle.CompiledHash, currentState.AppliedHash)

	overview := fetchDashboardOverview(t, client, st.OrgID)
	require.Equal(t, st.OrgID, overview.OrgID)
	require.Equal(t, 1, overview.TotalSessions)
	require.Greater(t, overview.ActiveReports, 0)

	bundleAfterSync := fetchManagedSkillSetBundle(t, client, projectResp.ProjectID)
	require.NotNil(t, bundleAfterSync.ClientState)
	require.Equal(t, readyBundle.Version, bundleAfterSync.ClientState.AppliedVersion)
	require.Equal(t, readyBundle.CompiledHash, bundleAfterSync.ClientState.AppliedHash)
	require.Contains(t, []string{"synced", "unchanged"}, bundleAfterSync.ClientState.SyncStatus)
	require.NotEmpty(t, bundleAfterSync.DeploymentHistory)

	modifiedDoc := localBundleDocumentPath(t, liveBundlePath, readyBundle.Files)
	require.NoError(t, os.WriteFile(modifiedDoc, []byte("# Modified locally\n\n- keep this local customization\n"), 0o644))

	conflictOutput := captureStdout(t, func() {
		require.NoError(t, runCollect([]string{"--codex-home", codexHome, "--snapshot-mode", "skip"}))
	})
	var conflictCollect collectRunResp
	require.NoError(t, json.Unmarshal([]byte(conflictOutput), &conflictCollect))
	require.NotNil(t, conflictCollect.SkillSet)
	require.Equal(t, "conflict", conflictCollect.SkillSet.Status)
	require.Contains(t, conflictCollect.SkillSet.Error, "modified locally")
	require.Contains(t, conflictCollect.SkillSet.Error, "autoskills skills resolve")

	currentState, _, err = loadSkillSetState()
	require.NoError(t, err)
	require.Equal(t, "conflict", currentState.LastSyncStatus)
	require.Contains(t, currentState.LastError, "autoskills skills resolve")

	bundleAfterConflict := fetchManagedSkillSetBundle(t, client, projectResp.ProjectID)
	require.NotNil(t, bundleAfterConflict.ClientState)
	require.Equal(t, "conflict", bundleAfterConflict.ClientState.SyncStatus)
	require.Equal(t, readyBundle.Version, bundleAfterConflict.ClientState.AppliedVersion)
	require.Equal(t, readyBundle.CompiledHash, bundleAfterConflict.ClientState.AppliedHash)
	require.Contains(t, bundleAfterConflict.ClientState.LastError, "modified locally")
	require.NotEmpty(t, bundleAfterConflict.DeploymentHistory)
	require.Equal(t, "skillset_sync_failed", bundleAfterConflict.DeploymentHistory[0].EventType)
	require.Contains(t, bundleAfterConflict.DeploymentHistory[0].Summary, "modified locally")

	resp, err := server.Client().Get(server.URL + "/dashboard")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "AutoSkills")
	require.Contains(t, string(body), "Current policy documents")
}

func newManagedSkillSetEndToEndServer(t *testing.T) (*configs.Config, *httptest.Server) {
	t.Helper()

	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/responses", r.URL.Path)
		require.Equal(t, "Bearer test-openai-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "output": [
    {
      "type": "message",
      "content": [
        {
          "type": "output_text",
          "text": "{\"schema_version\":\"report-feedback.v1\",\"reports\":[{\"kind\":\"llm-workflow-review\",\"title\":\"Reduce repeated workflow recap before implementation\",\"summary\":\"The uploaded raw queries show the user spending too many turns on control-flow recap and verification setup before the actual patch work starts.\",\"user_intent\":\"The user wants a small, validated fix and keeps narrowing scope before implementation.\",\"model_interpretation\":\"The model appears to understand the request as repo-orientation and verification planning first, then patching.\",\"reason\":\"Recent raw queries repeatedly ask for current behavior summaries, exact checks, and narrow patch scope before implementation begins.\",\"explanation\":\"The report should call out that the user is compensating for missing default repo discovery and verification structure.\",\"expected_benefit\":\"Less repeated steering and faster transition from orientation to implementation.\",\"risk\":\"Low. Observational feedback only.\",\"expected_impact\":\"Fewer exploratory turns and clearer first useful responses.\",\"confidence\":\"high\",\"strengths\":[\"The user consistently asks for narrow patch scope.\",\"Verification intent is explicit before risky edits.\"],\"frictions\":[\"Repo discovery is repeated across sessions.\",\"Verification setup often arrives only after extra recap turns.\"],\"next_steps\":[\"Start with concrete file discovery before summarizing control flow.\",\"List targeted verification immediately after locating the fix.\"],\"score\":0.82,\"evidence\":[\"repeated control-flow recap\",\"repeated verification prompts\"]}]}"
        }
      ]
    }
  ]
}`))
		require.NoError(t, err)
	}))
	t.Cleanup(openAIServer.Close)

	conf := &configs.Config{
		App: configs.App{
			Mode:      "local",
			APIToken:  "skillset-e2e-token",
			StorePath: filepath.Join(t.TempDir(), "crux-store.json"),
		},
		OpenAI: configs.OpenAI{
			APIKey:         "test-openai-key",
			BaseURL:        openAIServer.URL + "/v1",
			ResponsesModel: "gpt-5.4",
		},
	}

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

	return conf, httptest.NewServer(echo)
}

func registerManagedSkillSetEndToEndWorkspace(t *testing.T, client *apiClient, repoPath string) (state, response.ProjectRegistrationResp) {
	t.Helper()

	var issuedToken response.CLITokenIssueResp
	require.NoError(t, client.doJSON(http.MethodPost, "/api/v1/auth/cli-tokens", request.IssueCLITokenReq{
		Label: "skill-e2e-device",
	}, &issuedToken))

	enrollmentClient := &apiClient{
		baseURL: client.baseURL,
		token:   issuedToken.Token,
		http:    client.http,
	}

	var loginResp response.CLILoginResp
	require.NoError(t, enrollmentClient.doJSON(http.MethodPost, "/api/v1/auth/cli/login", request.CLILoginReq{
		DeviceName: "skill-e2e-device",
		Hostname:   "skill-e2e.local",
		Platform:   "darwin/arm64",
		CLIVersion: "0.1.0-test",
		Tools:      []string{"codex"},
	}, &loginResp))

	deviceClient := &apiClient{
		baseURL: client.baseURL,
		token:   loginResp.AccessToken,
		http:    client.http,
	}

	var projectResp response.ProjectRegistrationResp
	require.NoError(t, deviceClient.doJSON(http.MethodPost, "/api/v1/projects/register", request.RegisterProjectReq{
		OrgID:       loginResp.OrgID,
		AgentID:     loginResp.AgentID,
		Name:        "skill-e2e-workspace",
		RepoHash:    "skill-e2e-repo-hash",
		RepoPath:    repoPath,
		LanguageMix: map[string]float64{"go": 1},
		DefaultTool: "codex",
	}, &projectResp))

	return state{
		ServerURL:        client.baseURL,
		AccessToken:      loginResp.AccessToken,
		RefreshToken:     loginResp.RefreshToken,
		TokenType:        loginResp.TokenType,
		AccessExpiresAt:  cloneTime(loginResp.AccessExpiresAt),
		RefreshExpiresAt: cloneTime(loginResp.RefreshExpiresAt),
		OrgID:            loginResp.OrgID,
		UserID:           loginResp.UserID,
		AgentID:          loginResp.AgentID,
		DeviceName:       "skill-e2e-device",
		Hostname:         "skill-e2e.local",
		WorkspaceID:      projectResp.ProjectID,
	}, projectResp
}

func waitForManagedSkillSetBundleReady(t *testing.T, client *apiClient, projectID string) response.SkillSetBundleResp {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		bundle := fetchManagedSkillSetBundle(t, client, projectID)
		if bundle.Status == "ready" && strings.TrimSpace(bundle.Version) != "" && len(bundle.Files) > 0 {
			return bundle
		}
		time.Sleep(50 * time.Millisecond)
	}

	bundle := fetchManagedSkillSetBundle(t, client, projectID)
	require.Equal(t, "ready", bundle.Status)
	require.NotEmpty(t, bundle.Version)
	require.NotEmpty(t, bundle.Files)
	return bundle
}

func fetchManagedSkillSetBundle(t *testing.T, client *apiClient, projectID string) response.SkillSetBundleResp {
	t.Helper()

	var bundle response.SkillSetBundleResp
	require.NoError(t, client.doJSON(
		http.MethodGet,
		"/api/v1/skill-sets/latest?project_id="+url.QueryEscape(projectID),
		nil,
		&bundle,
	))
	return bundle
}

func fetchDashboardOverview(t *testing.T, client *apiClient, orgID string) response.DashboardOverviewResp {
	t.Helper()

	var overview response.DashboardOverviewResp
	require.NoError(t, client.doJSON(
		http.MethodGet,
		"/api/v1/dashboard/overview?org_id="+url.QueryEscape(orgID),
		nil,
		&overview,
	))
	return overview
}

func localBundleDocumentPath(t *testing.T, liveBundlePath string, files []response.SkillSetFileResp) string {
	t.Helper()

	for _, file := range files {
		path := strings.TrimSpace(file.Path)
		if path == "" || path == "SKILL.md" || path == "00-manifest.json" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			return filepath.Join(liveBundlePath, filepath.FromSlash(path))
		}
	}
	t.Fatalf("managed skill bundle did not include a modifiable markdown document")
	return ""
}
