// Package runner orchestrates the execution of the upstream Codex CLI binary
// configured as CodexSeek by Jair Lima, injecting the DeepSeek model provider,
// custom 1M-context model catalog, Windows sandbox adjustments, and H2H flags.
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/catalog"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/config"
)

// TransformArgs inspects the user-provided CLI arguments and injects the
// CodexSeek overlay flags (DeepSeek provider, catalog JSON, model alias resolution,
// and fast headless config isolation when running `exec`).
func TransformArgs(rawArgs []string, catalogPath string) []string {
	selectedModel := config.DefaultModel
	if envModel := strings.TrimSpace(os.Getenv("CODEXSEEK_MODEL")); envModel != "" {
		selectedModel = config.ResolveModelAlias(envModel)
	}

	cleaned := make([]string, 0, len(rawArgs))
	hasNoDaemon := false
	hasIgnoreUserConfig := false
	isExecSubcommand := false
	execSubcommandIdx := -1

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
		}

		cleaned = append(cleaned, arg)
	}

	// Build top-level overlay flags
	escapedCatalogPath := strings.ReplaceAll(catalogPath, `\`, `\\`)
	baseURL := config.DefaultBaseURL
	if customURL := strings.TrimSpace(os.Getenv("CODEXSEEK_BASE_URL")); customURL != "" {
		baseURL = customURL
	}

	overlayFlags := make([]string, 0, 18)
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
	)

	if runtime.GOOS == "windows" {
		// Ensure Windows workspace-write works cleanly without requiring Desktop MXC pipe
		overlayFlags = append(overlayFlags, "-c", `windows.sandbox="unelevated"`)
	}

	// If running non-interactively via `exec`, inject `--ignore-user-config` right after `exec`
	// unless the user explicitly passed `--use-codex-config` or `--ignore-user-config`.
	// This prevents 25-second hangs caused by Codex Desktop MCP plugins (`node_repl`) and
	// `approvals_reviewer = "auto_review"`.
	if isExecSubcommand && !hasIgnoreUserConfig && os.Getenv("CODEXSEEK_KEEP_USER_CONFIG") != "1" {
		withExecFlags := make([]string, 0, len(cleaned)+2)
		withExecFlags = append(withExecFlags, cleaned[:execSubcommandIdx+1]...)
		// Only add --ignore-user-config for `exec` / `e` (not `review` if unsupported)
		if cleaned[execSubcommandIdx] == "exec" || cleaned[execSubcommandIdx] == "e" {
			withExecFlags = append(withExecFlags, "--ignore-user-config")
		}
		withExecFlags = append(withExecFlags, cleaned[execSubcommandIdx+1:]...)
		cleaned = withExecFlags
	} else {
		// In interactive TUI mode, override service_tier and approvals_reviewer so user config doesn't conflict
		overlayFlags = append(overlayFlags,
			"-c", `approvals_reviewer="user"`,
		)
	}

	finalArgs := make([]string, 0, len(overlayFlags)+len(cleaned))
	finalArgs = append(finalArgs, overlayFlags...)
	finalArgs = append(finalArgs, cleaned...)
	return finalArgs
}

// Run executes the upstream Codex CLI binary with the CodexSeek overlay.
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
	catalogPath, err := catalog.EnsureCatalog(codexBin, upstreamVer, stateDir)
	if err != nil {
		return 1, fmt.Errorf("erro ao preparar catálogo de modelos DeepSeek: %w", err)
	}

	finalArgs := TransformArgs(rawArgs, catalogPath)

	cmd := exec.Command(codexBin, finalArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Inject DEEPSEEK_API_KEY into the child process environment in memory only
	env := os.Environ()
	filteredEnv := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if !strings.HasPrefix(kv, config.EnvKeyName+"=") {
			filteredEnv = append(filteredEnv, kv)
		}
	}
	filteredEnv = append(filteredEnv, fmt.Sprintf("%s=%s", config.EnvKeyName, apiKey))
	cmd.Env = filteredEnv

	// Forward termination signals cleanly
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

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
