package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/dto/response"
)

func TestRunDaemonEnableStatusDisable(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("AGENTOPT_HOME", filepath.Join(root, ".agentopt"))
	t.Setenv("HOME", home)

	launchctlLog := filepath.Join(root, "launchctl.log")
	launchctlStub := filepath.Join(root, "launchctl-stub.sh")
	require.NoError(t, os.WriteFile(launchctlStub, []byte(`#!/bin/sh
set -eu
printf '%s %s
' "$1" "$*" >> "$AGENTOPT_LAUNCHCTL_LOG"
exit 0
`), 0o755))
	t.Setenv("AGENTOPT_LAUNCHCTL_BIN", launchctlStub)
	t.Setenv("AGENTOPT_LAUNCHCTL_LOG", launchctlLog)

	require.NoError(t, saveState(state{
		ServerURL:   "http://127.0.0.1:8082",
		APIToken:    "token-daemon",
		OrgID:       "org-1",
		UserID:      "user-1",
		WorkspaceID: "project-1",
	}))

	enableOutput := captureStdout(t, func() {
		require.NoError(t, runDaemon([]string{"enable", "--collect-interval", "45m", "--sync-interval", "20s", "--recent", "2", "--snapshot-mode", "skip"}))
	})

	var enabled daemonStatusResp
	require.NoError(t, json.Unmarshal([]byte(enableOutput), &enabled))
	require.True(t, enabled.Enabled)
	require.True(t, enabled.Loaded)
	require.True(t, enabled.CollectorReady)
	require.True(t, enabled.CollectorLive)
	require.True(t, enabled.SyncReady)
	require.True(t, enabled.SyncLive)
	require.NotNil(t, enabled.Metadata)
	require.Equal(t, 2700, enabled.Metadata.CollectIntervalSeconds)
	require.Equal(t, 20, enabled.Metadata.SyncIntervalSeconds)
	require.Equal(t, 2, enabled.Metadata.Recent)
	require.Equal(t, "skip", enabled.Metadata.SnapshotMode)
	require.FileExists(t, enabled.Metadata.CollectPlistPath)
	require.FileExists(t, enabled.Metadata.CollectScriptPath)
	require.FileExists(t, enabled.Metadata.SyncPlistPath)
	require.FileExists(t, enabled.Metadata.SyncScriptPath)

	collectScriptData, err := os.ReadFile(enabled.Metadata.CollectScriptPath)
	require.NoError(t, err)
	require.Contains(t, string(collectScriptData), "collect")
	require.Contains(t, string(collectScriptData), "--snapshot-mode")
	require.Contains(t, string(collectScriptData), "skip")

	syncScriptData, err := os.ReadFile(enabled.Metadata.SyncScriptPath)
	require.NoError(t, err)
	require.Contains(t, string(syncScriptData), "sync")
	require.Contains(t, string(syncScriptData), "--watch")
	require.Contains(t, string(syncScriptData), "--interval")
	require.Contains(t, string(syncScriptData), "20s")

	statusOutput := captureStdout(t, func() {
		require.NoError(t, runDaemon([]string{"status"}))
	})
	var status daemonStatusResp
	require.NoError(t, json.Unmarshal([]byte(statusOutput), &status))
	require.True(t, status.Enabled)
	require.True(t, status.Loaded)
	require.NotNil(t, status.Metadata)
	require.Equal(t, enabled.Metadata.CollectLabel, status.Metadata.CollectLabel)
	require.Equal(t, enabled.Metadata.SyncLabel, status.Metadata.SyncLabel)

	disableOutput := captureStdout(t, func() {
		require.NoError(t, runDaemon([]string{"disable"}))
	})
	var disabled daemonDisableResp
	require.NoError(t, json.Unmarshal([]byte(disableOutput), &disabled))
	require.True(t, disabled.Disabled)
	require.NoFileExists(t, enabled.Metadata.CollectPlistPath)
	require.NoFileExists(t, enabled.Metadata.CollectScriptPath)
	require.NoFileExists(t, enabled.Metadata.SyncPlistPath)
	require.NoFileExists(t, enabled.Metadata.SyncScriptPath)

	logData, err := os.ReadFile(launchctlLog)
	require.NoError(t, err)
	require.Contains(t, string(logData), "load")
	require.Contains(t, string(logData), "list")
	require.Contains(t, string(logData), "unload")
}

func TestRunDaemonEnableBootstrapsExistingSessions(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	codexHome := filepath.Join(root, ".codex")
	sessionPath := filepath.Join(codexHome, "sessions", "2026", "03", "12", "bootstrap.jsonl")
	t.Setenv("AGENTOPT_HOME", filepath.Join(root, ".agentopt"))
	t.Setenv("HOME", home)

	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0o755))
	require.NoError(t, os.WriteFile(sessionPath, []byte(strings.Join([]string{
		`{"timestamp":"2026-03-12T08:00:00Z","type":"session_meta","payload":{"id":"codex-session-bootstrap","timestamp":"2026-03-12T08:00:00Z","model_provider":"openai"}}`,
		`{"timestamp":"2026-03-12T08:00:00Z","type":"turn_context","payload":{"model":"gpt-5.4"}}`,
		`{"timestamp":"2026-03-12T08:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"## My request for Codex:\nBootstrap my existing sessions."}}`,
		`{"timestamp":"2026-03-12T08:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":600,"cached_input_tokens":100,"output_tokens":120,"reasoning_output_tokens":40,"total_tokens":720}}}}`,
		`{"timestamp":"2026-03-12T08:00:03Z","type":"event_msg","payload":{"type":"agent_message","message":"The daemon can bootstrap existing sessions before background collection starts."}}`,
	}, "\n")+"\n"), 0o644))

	launchctlLog := filepath.Join(root, "launchctl.log")
	launchctlStub := filepath.Join(root, "launchctl-stub.sh")
	require.NoError(t, os.WriteFile(launchctlStub, []byte(`#!/bin/sh
set -eu
printf '%s %s
' "$1" "$*" >> "$AGENTOPT_LAUNCHCTL_LOG"
exit 0
`), 0o755))
	t.Setenv("AGENTOPT_LAUNCHCTL_BIN", launchctlStub)
	t.Setenv("AGENTOPT_LAUNCHCTL_LOG", launchctlLog)

	uploaded := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/session-summaries":
			uploaded++
			require.NoError(t, json.NewEncoder(w).Encode(envelope{
				Code: 0,
				Data: mustJSONRawMessage(t, response.SessionIngestResp{
					SessionID:           "codex-session-bootstrap",
					ProjectID:           "project-bootstrap",
					RecommendationCount: 0,
				}),
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	require.NoError(t, saveState(state{
		ServerURL:   server.URL,
		APIToken:    "token-daemon",
		OrgID:       "org-1",
		UserID:      "user-1",
		WorkspaceID: "project-bootstrap",
	}))

	enableOutput := captureStdout(t, func() {
		require.NoError(t, runDaemon([]string{
			"enable",
			"--bootstrap-recent", "1",
			"--collect-interval", "45m",
			"--sync-interval", "20s",
			"--codex-home", codexHome,
			"--snapshot-mode", "skip",
		}))
	})

	var enabled daemonStatusResp
	require.NoError(t, json.Unmarshal([]byte(enableOutput), &enabled))
	require.NotNil(t, enabled.Bootstrap)
	require.Equal(t, "skipped", enabled.Bootstrap.SnapshotStatus)
	require.Equal(t, "uploaded", enabled.Bootstrap.SessionStatus)
	require.Equal(t, 1, enabled.Bootstrap.SessionUploaded)
	require.NotNil(t, enabled.Metadata)
	require.Equal(t, 1, enabled.Metadata.BootstrapRecent)
	require.Equal(t, codexHome, enabled.Metadata.CodexHome)
	require.Equal(t, 1, uploaded)

	captureStdout(t, func() {
		require.NoError(t, runDaemon([]string{"disable"}))
	})
}
