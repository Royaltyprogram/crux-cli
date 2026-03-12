package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDaemonCollectInterval = 30 * time.Minute
	defaultDaemonSyncInterval    = 15 * time.Second
)

type daemonMetadata struct {
	Platform               string    `json:"platform"`
	BootstrapRecent        int       `json:"bootstrap_recent"`
	Recent                 int       `json:"recent"`
	Tool                   string    `json:"tool"`
	CodexHome              string    `json:"codex_home,omitempty"`
	SnapshotMode           string    `json:"snapshot_mode"`
	SnapshotFile           string    `json:"snapshot_file,omitempty"`
	ProfileID              string    `json:"profile_id"`
	CollectIntervalSeconds int       `json:"collect_interval_seconds"`
	SyncIntervalSeconds    int       `json:"sync_interval_seconds"`
	SyncReasoningEffort    string    `json:"sync_reasoning_effort,omitempty"`
	CollectLabel           string    `json:"collect_label"`
	CollectPlistPath       string    `json:"collect_plist_path"`
	CollectScriptPath      string    `json:"collect_script_path"`
	CollectLogPath         string    `json:"collect_log_path"`
	SyncLabel              string    `json:"sync_label"`
	SyncPlistPath          string    `json:"sync_plist_path"`
	SyncScriptPath         string    `json:"sync_script_path"`
	SyncLogPath            string    `json:"sync_log_path"`
	EnabledAt              time.Time `json:"enabled_at"`
}

type daemonStatusResp struct {
	Enabled        bool            `json:"enabled"`
	Loaded         bool            `json:"loaded"`
	CollectorReady bool            `json:"collector_ready"`
	CollectorLive  bool            `json:"collector_live"`
	SyncReady      bool            `json:"sync_ready"`
	SyncLive       bool            `json:"sync_live"`
	Bootstrap      *collectRunResp `json:"bootstrap,omitempty"`
	Metadata       *daemonMetadata `json:"metadata,omitempty"`
}

type daemonDisableResp struct {
	Disabled     bool   `json:"disabled"`
	CollectLabel string `json:"collect_label"`
	SyncLabel    string `json:"sync_label"`
}

func runDaemon(args []string) error {
	if len(args) == 0 {
		return errors.New("daemon requires a subcommand: enable, disable, status")
	}

	switch args[0] {
	case "enable":
		return runDaemonEnable(args[1:])
	case "disable":
		return runDaemonDisable(args[1:])
	case "status":
		return runDaemonStatus(args[1:])
	default:
		return fmt.Errorf("unknown daemon subcommand %q", args[0])
	}
}

func runDaemonEnable(args []string) error {
	if err := ensureLaunchdPlatform(); err != nil {
		return err
	}

	fs := flag.NewFlagSet("daemon enable", flag.ContinueOnError)
	collectInterval := fs.Duration("collect-interval", defaultDaemonCollectInterval, "background session collection interval")
	syncInterval := fs.Duration("sync-interval", defaultDaemonSyncInterval, "pending-apply poll interval for automatic local sync")
	bootstrapRecent := fs.Int("bootstrap-recent", 0, "upload this many existing local Codex sessions once before enabling the daemon")
	recent := fs.Int("recent", 1, "number of recent local Codex session JSONL files to upload per collect run")
	tool := fs.String("tool", "codex", "tool name")
	codexHome := fs.String("codex-home", "", "override Codex home used for automatic session collection")
	snapshotMode := fs.String("snapshot-mode", collectSnapshotModeChanged, "snapshot upload mode: changed, always, skip")
	snapshotFile := fs.String("snapshot-file", "", "snapshot JSON file path")
	profileID := fs.String("profile-id", "default", "profile identifier for snapshot uploads")
	syncReasoningEffort := fs.String("codex-reasoning-effort", os.Getenv("AGENTOPT_CODEX_REASONING_EFFORT"), "Codex reasoning effort for automatic local apply (minimal, low, medium, high, xhigh)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *collectInterval <= 0 {
		return errors.New("daemon enable --collect-interval must be greater than zero")
	}
	if *syncInterval <= 0 {
		return errors.New("daemon enable --sync-interval must be greater than zero")
	}
	if *recent < 1 {
		return errors.New("daemon enable --recent must be at least 1")
	}
	if *bootstrapRecent < 0 {
		return errors.New("daemon enable --bootstrap-recent must be zero or greater")
	}
	resolvedSnapshotMode, err := parseCollectSnapshotMode(*snapshotMode)
	if err != nil {
		return err
	}
	resolvedReasoningEffort, err := parseCodexReasoningEffort(*syncReasoningEffort)
	if err != nil {
		return err
	}

	st, err := loadWorkspaceState()
	if err != nil {
		return err
	}
	meta, err := buildDaemonMetadata(st, *bootstrapRecent, *collectInterval, *syncInterval, *recent, *tool, *codexHome, resolvedSnapshotMode, *snapshotFile, *profileID, resolvedReasoningEffort)
	if err != nil {
		return err
	}
	var bootstrapResp *collectRunResp
	if *bootstrapRecent > 0 {
		client := newAPIClient(st.ServerURL, st.APIToken)
		resp, err := runCollectOnce(st, client, *snapshotFile, *profileID, *tool, "", *codexHome, *bootstrapRecent, resolvedSnapshotMode)
		if err != nil {
			return err
		}
		bootstrapResp = &resp
	}

	for _, path := range []string{
		filepath.Dir(meta.CollectScriptPath),
		filepath.Dir(meta.CollectLogPath),
		filepath.Dir(meta.CollectPlistPath),
		filepath.Dir(meta.SyncScriptPath),
		filepath.Dir(meta.SyncLogPath),
		filepath.Dir(meta.SyncPlistPath),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}

	collectScript, err := renderDaemonCollectScript(*meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(meta.CollectScriptPath, []byte(collectScript), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(meta.CollectPlistPath, []byte(renderIntervalLaunchdPlist(meta.CollectLabel, meta.CollectScriptPath, meta.CollectIntervalSeconds, meta.CollectLogPath)), 0o644); err != nil {
		return err
	}

	syncScript, err := renderDaemonSyncScript(*meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(meta.SyncScriptPath, []byte(syncScript), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(meta.SyncPlistPath, []byte(renderKeepAliveLaunchdPlist(meta.SyncLabel, meta.SyncScriptPath, meta.SyncLogPath)), 0o644); err != nil {
		return err
	}

	if err := saveDaemonMetadata(*meta); err != nil {
		return err
	}

	_ = runLaunchctl("unload", meta.CollectPlistPath)
	_ = runLaunchctl("unload", meta.SyncPlistPath)
	if err := runLaunchctl("load", "-w", meta.CollectPlistPath); err != nil {
		return err
	}
	if err := runLaunchctl("load", "-w", meta.SyncPlistPath); err != nil {
		return err
	}

	return prettyPrint(daemonStatusResp{
		Enabled:        true,
		Loaded:         true,
		CollectorReady: true,
		CollectorLive:  true,
		SyncReady:      true,
		SyncLive:       true,
		Bootstrap:      bootstrapResp,
		Metadata:       meta,
	})
}

func runDaemonDisable(args []string) error {
	if err := ensureLaunchdPlatform(); err != nil {
		return err
	}

	fs := flag.NewFlagSet("daemon disable", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := loadWorkspaceState()
	if err != nil {
		return err
	}
	meta, _ := loadDaemonMetadata(st)

	collectLabel := daemonCollectLabel(st)
	syncLabel := daemonSyncLabel(st)
	collectPlistPath, err := daemonCollectPlistPath(st)
	if err != nil {
		return err
	}
	collectScriptPath, err := daemonCollectScriptPath(st)
	if err != nil {
		return err
	}
	syncPlistPath, err := daemonSyncPlistPath(st)
	if err != nil {
		return err
	}
	syncScriptPath, err := daemonSyncScriptPath(st)
	if err != nil {
		return err
	}
	if meta != nil {
		collectLabel = meta.CollectLabel
		syncLabel = meta.SyncLabel
		collectPlistPath = meta.CollectPlistPath
		collectScriptPath = meta.CollectScriptPath
		syncPlistPath = meta.SyncPlistPath
		syncScriptPath = meta.SyncScriptPath
	}

	_ = runLaunchctl("unload", collectPlistPath)
	_ = runLaunchctl("unload", syncPlistPath)
	_ = os.Remove(collectPlistPath)
	_ = os.Remove(collectScriptPath)
	_ = os.Remove(syncPlistPath)
	_ = os.Remove(syncScriptPath)
	_ = os.Remove(daemonMetadataPath(st))

	return prettyPrint(daemonDisableResp{
		Disabled:     true,
		CollectLabel: collectLabel,
		SyncLabel:    syncLabel,
	})
}

func runDaemonStatus(args []string) error {
	if err := ensureLaunchdPlatform(); err != nil {
		return err
	}

	fs := flag.NewFlagSet("daemon status", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := loadWorkspaceState()
	if err != nil {
		return err
	}
	meta, err := loadDaemonMetadata(st)
	if err != nil {
		return err
	}
	if meta == nil {
		return prettyPrint(daemonStatusResp{})
	}

	collectorReady := fileExists(meta.CollectPlistPath) && fileExists(meta.CollectScriptPath)
	syncReady := fileExists(meta.SyncPlistPath) && fileExists(meta.SyncScriptPath)
	collectorLive := false
	syncLive := false
	if collectorReady {
		collectorLive = launchdLoaded(meta.CollectLabel)
	}
	if syncReady {
		syncLive = launchdLoaded(meta.SyncLabel)
	}

	return prettyPrint(daemonStatusResp{
		Enabled:        collectorReady && syncReady,
		Loaded:         collectorLive && syncLive,
		CollectorReady: collectorReady,
		CollectorLive:  collectorLive,
		SyncReady:      syncReady,
		SyncLive:       syncLive,
		Metadata:       meta,
	})
}

func buildDaemonMetadata(st state, bootstrapRecent int, collectInterval, syncInterval time.Duration, recent int, tool, codexHome, snapshotMode, snapshotFile, profileID, syncReasoningEffort string) (*daemonMetadata, error) {
	collectPlistPath, err := daemonCollectPlistPath(st)
	if err != nil {
		return nil, err
	}
	collectScriptPath, err := daemonCollectScriptPath(st)
	if err != nil {
		return nil, err
	}
	collectLogPath, err := daemonCollectLogPath(st)
	if err != nil {
		return nil, err
	}
	syncPlistPath, err := daemonSyncPlistPath(st)
	if err != nil {
		return nil, err
	}
	syncScriptPath, err := daemonSyncScriptPath(st)
	if err != nil {
		return nil, err
	}
	syncLogPath, err := daemonSyncLogPath(st)
	if err != nil {
		return nil, err
	}

	return &daemonMetadata{
		Platform:               "launchd",
		BootstrapRecent:        bootstrapRecent,
		Recent:                 recent,
		Tool:                   tool,
		CodexHome:              codexHome,
		SnapshotMode:           snapshotMode,
		SnapshotFile:           snapshotFile,
		ProfileID:              profileID,
		CollectIntervalSeconds: durationToLaunchdSeconds(collectInterval),
		SyncIntervalSeconds:    durationToLaunchdSeconds(syncInterval),
		SyncReasoningEffort:    syncReasoningEffort,
		CollectLabel:           daemonCollectLabel(st),
		CollectPlistPath:       collectPlistPath,
		CollectScriptPath:      collectScriptPath,
		CollectLogPath:         collectLogPath,
		SyncLabel:              daemonSyncLabel(st),
		SyncPlistPath:          syncPlistPath,
		SyncScriptPath:         syncScriptPath,
		SyncLogPath:            syncLogPath,
		EnabledAt:              time.Now().UTC(),
	}, nil
}

func renderDaemonCollectScript(meta daemonMetadata) (string, error) {
	command, err := resolveBackgroundBaseCommand()
	if err != nil {
		return "", err
	}

	var lines []string
	lines = append(lines, "#!/bin/sh", "set -eu")
	if root := strings.TrimSpace(os.Getenv("AGENTOPT_HOME")); root != "" {
		lines = append(lines, "export AGENTOPT_HOME="+shellQuote(root))
	}
	if command.Workdir != "" {
		lines = append(lines, "cd "+shellQuote(command.Workdir))
	}

	args := append([]string{command.Program}, command.Args...)
	args = append(args,
		"collect",
		"--recent", strconv.Itoa(meta.Recent),
		"--tool", meta.Tool,
		"--snapshot-mode", meta.SnapshotMode,
		"--profile-id", meta.ProfileID,
	)
	if meta.CodexHome != "" {
		args = append(args, "--codex-home", meta.CodexHome)
	}
	if meta.SnapshotFile != "" {
		args = append(args, "--snapshot-file", meta.SnapshotFile)
	}

	lines = append(lines, "exec "+joinShellArgs(args))
	return strings.Join(lines, "\n") + "\n", nil
}

func renderDaemonSyncScript(meta daemonMetadata) (string, error) {
	command, err := resolveBackgroundBaseCommand()
	if err != nil {
		return "", err
	}

	var lines []string
	lines = append(lines, "#!/bin/sh", "set -eu")
	if root := strings.TrimSpace(os.Getenv("AGENTOPT_HOME")); root != "" {
		lines = append(lines, "export AGENTOPT_HOME="+shellQuote(root))
	}
	if command.Workdir != "" {
		lines = append(lines, "cd "+shellQuote(command.Workdir))
	}

	args := append([]string{command.Program}, command.Args...)
	args = append(args,
		"sync",
		"--watch",
		"--interval", (time.Duration(meta.SyncIntervalSeconds) * time.Second).String(),
	)
	if meta.SyncReasoningEffort != "" {
		args = append(args, "--codex-reasoning-effort", meta.SyncReasoningEffort)
	}

	lines = append(lines, "exec "+joinShellArgs(args))
	return strings.Join(lines, "\n") + "\n", nil
}

func renderIntervalLaunchdPlist(label, scriptPath string, intervalSeconds int, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StartInterval</key>
  <integer>%d</integer>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(label), xmlEscape(scriptPath), intervalSeconds, xmlEscape(logPath), xmlEscape(logPath))
}

func renderKeepAliveLaunchdPlist(label, scriptPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(label), xmlEscape(scriptPath), xmlEscape(logPath), xmlEscape(logPath))
}

func daemonCollectLabel(st state) string {
	return "com.agentopt.daemon.collect." + sanitizeID(st.workspaceID())
}

func daemonSyncLabel(st state) string {
	return "com.agentopt.daemon.sync." + sanitizeID(st.workspaceID())
}

func daemonCollectPlistPath(st state) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", daemonCollectLabel(st)+".plist"), nil
}

func daemonCollectScriptPath(st state) (string, error) {
	root, err := agentoptHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "daemon", daemonCollectLabel(st)+".sh"), nil
}

func daemonCollectLogPath(st state) (string, error) {
	root, err := agentoptHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "logs", daemonCollectLabel(st)+".log"), nil
}

func daemonSyncPlistPath(st state) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", daemonSyncLabel(st)+".plist"), nil
}

func daemonSyncScriptPath(st state) (string, error) {
	root, err := agentoptHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "daemon", daemonSyncLabel(st)+".sh"), nil
}

func daemonSyncLogPath(st state) (string, error) {
	root, err := agentoptHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "logs", daemonSyncLabel(st)+".log"), nil
}

func daemonMetadataPath(st state) string {
	root, err := agentoptHomeDir()
	if err != nil {
		return filepath.Join(".agentopt", "daemon", sanitizeID(st.workspaceID())+".json")
	}
	return filepath.Join(root, "daemon", sanitizeID(st.workspaceID())+".json")
}

func saveDaemonMetadata(meta daemonMetadata) error {
	workspaceID := strings.TrimPrefix(meta.CollectLabel, "com.agentopt.daemon.collect.")
	path := filepath.Join(filepath.Dir(meta.CollectScriptPath), sanitizeID(workspaceID)+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func loadDaemonMetadata(st state) (*daemonMetadata, error) {
	path := daemonMetadataPath(st)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var meta daemonMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
