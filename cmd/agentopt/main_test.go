package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/liushuangls/go-server-template/dto/response"
)

func TestReadOptionalJSONMapMissingFile(t *testing.T) {
	var out map[string]any
	exists, err := readOptionalJSONMap(filepath.Join(t.TempDir(), "missing.json"), &out)
	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, out)
}

func TestApplyBackupRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	backup := applyBackup{
		ApplyID:        "apply-1",
		ProjectID:      "project-1",
		FilePath:       "/tmp/config.json",
		OriginalExists: true,
		OriginalJSON: map[string]any{
			"baseline": true,
		},
	}

	require.NoError(t, saveApplyBackup(backup))

	loaded, err := loadApplyBackup("apply-1")
	require.NoError(t, err)
	require.Equal(t, backup.ApplyID, loaded.ApplyID)
	require.Equal(t, backup.FilePath, loaded.FilePath)
	require.Equal(t, backup.OriginalJSON["baseline"], loaded.OriginalJSON["baseline"])

	require.NoError(t, deleteApplyBackup("apply-1"))

	_, err = loadApplyBackup("apply-1")
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(root, "applies", "apply-1.json"))
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}

func TestAPIClientAddsTokenHeader(t *testing.T) {
	client := newAPIClient("http://example.com", "test-token")
	require.Equal(t, "test-token", client.token)
}

func TestExecuteLocalApplyCreatesBackupAndWritesConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	target := filepath.Join(root, "config.json")
	err := os.WriteFile(target, []byte("{\"baseline\":true}\n"), 0o644)
	require.NoError(t, err)

	result, err := executeLocalApply(state{ProjectID: "project-1"}, "apply-1", []response.PatchPreviewItem{
		{
			FilePath: target,
			SettingsUpdates: map[string]any{
				"shell_profile": "safe",
			},
		},
	}, "")
	require.NoError(t, err)
	require.Equal(t, target, result.FilePath)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Contains(t, string(data), "\"baseline\": true")
	require.Contains(t, string(data), "\"shell_profile\": \"safe\"")

	backup, err := loadApplyBackup("apply-1")
	require.NoError(t, err)
	require.True(t, backup.OriginalExists)
	require.Equal(t, true, backup.OriginalJSON["baseline"])
}

func TestExecuteLocalApplyAppendsTextFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	target := filepath.Join(root, "AGENTS.md")
	err := os.WriteFile(target, []byte("# Existing\n"), 0o644)
	require.NoError(t, err)

	result, err := executeLocalApply(state{ProjectID: "project-1"}, "apply-text", []response.PatchPreviewItem{
		{
			FilePath:       "AGENTS.md",
			Operation:      "append_block",
			ContentPreview: "\n## AgentOpt\n- safe rollout\n",
		},
	}, target)
	require.NoError(t, err)
	require.Equal(t, target, result.FilePath)
	require.Contains(t, result.AppliedText, "AgentOpt")

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Contains(t, string(data), "# Existing")
	require.Contains(t, string(data), "safe rollout")
}

func TestPreflightLocalApplyRejectsUnsafeTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	result, err := preflightLocalApply(state{ProjectID: "project-1"}, "apply-unsafe", []response.PatchPreviewItem{
		{
			FilePath:        ".ssh/config",
			Operation:       "merge_patch",
			SettingsUpdates: map[string]any{"unsafe": true},
		},
	}, "")
	require.NoError(t, err)
	require.False(t, result.Allowed)
	require.Equal(t, "file_scope", result.Guard)
}

func TestPreflightLocalApplyRejectsMultipleSteps(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	_, err := preflightLocalApply(state{ProjectID: "project-1"}, "apply-multi", []response.PatchPreviewItem{
		{FilePath: ".codex/config.json", Operation: "merge_patch"},
		{FilePath: "AGENTS.md", Operation: "append_block"},
	}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly 1 step")
}

func TestRunSyncRejectsInvalidIntervalInWatchMode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPT_HOME", root)

	err := saveState(state{
		ServerURL: "http://127.0.0.1:8082",
		APIToken:  "token",
		OrgID:     "org-1",
		UserID:    "user-1",
		ProjectID: "project-1",
	})
	require.NoError(t, err)

	err = runSync([]string{"--watch", "--interval", "0s"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "greater than zero")
}
