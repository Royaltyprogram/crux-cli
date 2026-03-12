package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type backgroundBaseCommand struct {
	Program string
	Args    []string
	Workdir string
}

func ensureLaunchdPlatform() error {
	if runtime.GOOS == "darwin" {
		return nil
	}
	if strings.TrimSpace(os.Getenv("AGENTOPT_LAUNCHCTL_BIN")) != "" {
		return nil
	}
	return errors.New("daemon is currently supported on macOS launchd only")
}

func launchdLoaded(label string) bool {
	return runLaunchctl("list", label) == nil
}

func runLaunchctl(args ...string) error {
	bin := strings.TrimSpace(os.Getenv("AGENTOPT_LAUNCHCTL_BIN"))
	if bin == "" {
		bin = "launchctl"
	}
	cmd := exec.Command(bin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, text)
	}
	return nil
}

func resolveBackgroundBaseCommand() (backgroundBaseCommand, error) {
	executable, err := os.Executable()
	if err == nil && shouldUseStableExecutable(executable) {
		return backgroundBaseCommand{Program: executable}, nil
	}
	if installed, ok := findInstalledAgentopt(); ok {
		return backgroundBaseCommand{Program: installed}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return backgroundBaseCommand{}, err
	}
	repoRoot, ok := detectRepoRoot(cwd)
	if ok {
		return backgroundBaseCommand{
			Program: "go",
			Args:    []string{"run", "./cmd/agentopt"},
			Workdir: repoRoot,
		}, nil
	}
	return backgroundBaseCommand{}, errors.New("unable to infer a stable agentopt command for daemon; run from the repo root or use a built agentopt binary")
}

func findInstalledAgentopt() (string, bool) {
	if resolved, err := exec.LookPath("agentopt"); err == nil && shouldUseStableExecutable(resolved) {
		return filepath.Clean(resolved), true
	}

	var candidates []string
	if binDir := strings.TrimSpace(os.Getenv("AGENTOPT_BIN_DIR")); binDir != "" {
		candidates = append(candidates, filepath.Join(binDir, "agentopt"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "agentopt"))
	}
	for _, candidate := range candidates {
		if shouldUseStableExecutable(candidate) && fileExists(candidate) {
			return filepath.Clean(candidate), true
		}
	}
	return "", false
}

func detectRepoRoot(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "cmd", "agentopt", "main.go")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func shouldUseStableExecutable(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	base := strings.ToLower(filepath.Base(cleaned))
	if !strings.Contains(base, "agentopt") {
		return false
	}
	tempRoot := filepath.Clean(os.TempDir())
	if isWithinRoot(tempRoot, cleaned) {
		return false
	}
	return !strings.Contains(cleaned, string(os.PathSeparator)+"go-build")
}

func durationToLaunchdSeconds(interval time.Duration) int {
	seconds := int(interval / time.Second)
	if interval%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func agentoptHomeDir() (string, error) {
	path, err := stateFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func joinShellArgs(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
