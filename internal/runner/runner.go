// Package runner orchestrates the execution of the upstream Codex CLI binary
// configured as CodexSeek by Jair Lima, injecting the DeepSeek model provider,
// custom 1M-context model catalog, Windows sandbox adjustments, auto_recap,
// and H2H usage audit logging.
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/catalog"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/config"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/sync"
)

// UsageLogRecord represents a single execution entry in ~/.codexseek/h2h-usage.jsonl
// per Section 10 of the H2H Protocol specification.
type UsageLogRecord struct {
	Timestamp   string  `json:"timestamp"`
	Mode        string  `json:"mode"`
	Model       string  `json:"model"`
	WorkDir     string  `json:"workdir"`
	DurationSec float64 `json:"duration_sec"`
	ExitCode    int     `json:"exit_code"`
	PromptBrief string  `json:"prompt_brief,omitempty"`
}

// TransformArgs inspects the user-provided CLI arguments and injects the
// CodexSeek overlay flags (DeepSeek provider, catalog JSON, model alias resolution,
// tui.auto_recap=true, and fast headless config isolation when running `exec`).
func TransformArgs(rawArgs []string, catalogPath string) ([]string, string, string) {
	selectedModel := config.DefaultModel
	if envModel := strings.TrimSpace(os.Getenv("CODEXSEEK_MODEL")); envModel != "" {
		selectedModel = config.ResolveModelAlias(envModel)
	}

	cleaned := make([]string, 0, len(rawArgs))
	hasNoDaemon := false
	hasIgnoreUserConfig := false
	isExecSubcommand := false
	execSubcommandIdx := -1
	mode := "interactive"

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]

		switch {
		case arg == "--pro":
			selectedModel = config.ProModel
			continue
		case arg == "--flash":
			selectedModel = config.DefaultModel
			continue
		case arg == "-m" || arg == "--model":
			if i+1 < len(rawArgs) {
				selectedModel = config.ResolveModelAlias(rawArgs[i+1])
				i++
				continue
			}
		case strings.HasPrefix(arg, "--model="):
			val := strings.TrimPrefix(arg, "--model=")
			selectedModel = config.ResolveModelAlias(val)
			continue
		case strings.HasPrefix(arg, "-m="):
			val := strings.TrimPrefix(arg, "-m=")
			selectedModel = config.ResolveModelAlias(val)
			continue
		case arg == "--no-daemon":
			hasNoDaemon = true
		case arg == "--ignore-user-config":
			hasIgnoreUserConfig = true
		case (arg == "exec" || arg == "e" || arg == "review") && !isExecSubcommand:
			isExecSubcommand = true
			execSubcommandIdx = len(cleaned)
			mode = arg
		}

		cleaned = append(cleaned, arg)
	}

	escapedCatalogPath := strings.ReplaceAll(catalogPath, `\`, `\\`)
	baseURL := config.DefaultBaseURL
	if customURL := strings.TrimSpace(os.Getenv("CODEXSEEK_BASE_URL")); customURL != "" {
		baseURL = customURL
	}

	selfExe, _ := os.Executable()
	escapedSelfExe := strings.ReplaceAll(selfExe, `\`, `\\`)

	overlayFlags := make([]string, 0, 24)
	if !hasNoDaemon {
		overlayFlags = append(overlayFlags, "--no-daemon")
	}

	overlayFlags = append(overlayFlags,
		"-m", selectedModel,
		"-c", `model_provider="deepseek"`,
		"-c", fmt.Sprintf(`model_catalog_json="%s"`, escapedCatalogPath),
		"-c", `model_providers.deepseek.name="DeepSeek"`,
		"-c", fmt.Sprintf(`model_providers.deepseek.base_url="%s"`, baseURL),
		"-c", fmt.Sprintf(`model_providers.deepseek.env_key="%s"`, config.EnvKeyName),
		"-c", `model_providers.deepseek.wire_api="responses"`,
		"-c", `tui.auto_recap=true`,
	)

	if selfExe != "" {
		overlayFlags = append(overlayFlags,
			"-c", fmt.Sprintf(`notify=["%s", "turn-ended"]`, escapedSelfExe),
		)
	}

	if runtime.GOOS == "windows" {
		overlayFlags = append(overlayFlags, "-c", `windows.sandbox="unelevated"`)
	}

	if isExecSubcommand && !hasIgnoreUserConfig && os.Getenv("CODEXSEEK_KEEP_USER_CONFIG") != "1" {
		withExecFlags := make([]string, 0, len(cleaned)+2)
		withExecFlags = append(withExecFlags, cleaned[:execSubcommandIdx+1]...)
		if cleaned[execSubcommandIdx] == "exec" || cleaned[execSubcommandIdx] == "e" {
			withExecFlags = append(withExecFlags, "--ignore-user-config")
		}
		withExecFlags = append(withExecFlags, cleaned[execSubcommandIdx+1:]...)
		cleaned = withExecFlags
	} else {
		overlayFlags = append(overlayFlags,
			"-c", `approvals_reviewer="user"`,
		)
	}

	finalArgs := make([]string, 0, len(overlayFlags)+len(cleaned))
	finalArgs = append(finalArgs, overlayFlags...)
	finalArgs = append(finalArgs, cleaned...)
	return finalArgs, selectedModel, mode
}

// Run executes the upstream Codex CLI binary with the CodexSeek overlay and logs H2H usage.
func Run(rawArgs []string) (int, error) {
	codexBin, err := config.FindUpstreamCodexBinary()
	if err != nil {
		return 1, err
	}

	apiKey, _, err := config.ResolveAPIKey()
	if err != nil {
		return 1, err
	}

	stateDir, err := config.StateDir()
	if err != nil {
		return 1, err
	}

	upstreamVer := config.GetUpstreamVersion(codexBin)
	catalogPath, err := catalog.EnsureCatalog(codexBin, upstreamVer, stateDir, false)
	if err != nil {
		return 1, fmt.Errorf("erro ao preparar catálogo de modelos DeepSeek: %w", err)
	}

	// Pre-fetch and cache DeepSeek balance before entering the Codex sandbox
	cachedBalanceSummary := ""
	if bal, balErr := sync.FetchDeepSeekBalance(apiKey); balErr == nil {
		cachedBalanceSummary = sync.FormatBalanceSummary(bal)
	}

	finalArgs, selectedModel, mode := TransformArgs(rawArgs, catalogPath)

	cmd := exec.Command(codexBin, finalArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	filteredEnv := make([]string, 0, len(env)+3)
	for _, kv := range env {
		if !strings.HasPrefix(kv, config.EnvKeyName+"=") && !strings.HasPrefix(kv, "CODEXSEEK_BALANCE=") {
			filteredEnv = append(filteredEnv, kv)
		}
	}
	filteredEnv = append(filteredEnv, fmt.Sprintf("%s=%s", config.EnvKeyName, apiKey))
	if cachedBalanceSummary != "" {
		filteredEnv = append(filteredEnv, fmt.Sprintf("CODEXSEEK_BALANCE=%s", cachedBalanceSummary))
	}
	cmd.Env = filteredEnv

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
	defer signal.Stop(sigChan)

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime).Seconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		} else {
			exitCode = 1
		}
	}

	// Record H2H usage audit entry
	recordUsageLog(stateDir, mode, selectedModel, rawArgs, duration, exitCode)

	return exitCode, err
}

func recordUsageLog(stateDir, mode, model string, rawArgs []string, durationSec float64, exitCode int) {
	wd, _ := os.Getwd()
	brief := ""
	for i, a := range rawArgs {
		if (a == "-C" || a == "--cd") && i+1 < len(rawArgs) {
			wd = rawArgs[i+1]
		}
	}
	if len(rawArgs) > 0 {
		last := rawArgs[len(rawArgs)-1]
		if !strings.HasPrefix(last, "-") && last != "exec" && last != "e" {
			brief = last
			if len([]rune(brief)) > 120 {
				brief = string([]rune(brief)[:120]) + "..."
			}
		}
	}

	rec := UsageLogRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		Mode:        mode,
		Model:       model,
		WorkDir:     wd,
		DurationSec: float64(int(durationSec*100)) / 100,
		ExitCode:    exitCode,
		PromptBrief: brief,
	}

	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	logFile := filepath.Join(stateDir, "h2h-usage.jsonl")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
