package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSaveStateWritesSecureTokenSchema(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CRUX_HOME", root)

	accessExpiresAt := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	refreshExpiresAt := time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC)

	require.NoError(t, saveState(state{
		ServerURL:        "https://example.com",
		AccessToken:      "access-token",
		RefreshToken:     "refresh-token",
		TokenType:        "Bearer",
		AccessExpiresAt:  &accessExpiresAt,
		RefreshExpiresAt: &refreshExpiresAt,
		OrgID:            "org-1",
		UserID:           "user-1",
		AgentID:          "agent-1",
		WorkspaceID:      "project-1",
	}))

	statePath := filepath.Join(root, "state.json")
	info, err := os.Stat(statePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	raw, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "access-token", payload["access_token"])
	require.Equal(t, "refresh-token", payload["refresh_token"])
	require.Equal(t, "Bearer", payload["token_type"])
	require.NotContains(t, payload, "api_token")
}

func TestLoadStateReadsLegacyAPIToken(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CRUX_HOME", root)

	statePath := filepath.Join(root, "state.json")
	require.NoError(t, os.WriteFile(statePath, []byte(`{
  "server_url": "https://example.com",
  "api_token": "legacy-token",
  "org_id": "org-1",
  "user_id": "user-1",
  "agent_id": "agent-1"
}
`), 0o644))

	st, err := loadState()
	require.NoError(t, err)
	require.Equal(t, "legacy-token", st.accessToken())
	require.Equal(t, "legacy-token", st.AccessToken)

	info, err := os.Stat(statePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
