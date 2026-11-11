// Package config resolves the DeepSeek API credential, model aliases,
// state directory (~/.codexseek), and upstream Codex binary path without
// ever hardcoding or exposing any secret in source code or logs.
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	AppName        = "CodexSeek by Jair Lima"
	AppVersion     = "1.1.0"
	DefaultModel   = "deepseek-flash"
	ProModel       = "deepseek-v4-pro"
	DefaultBaseURL = "https://api.deepseek.com/v1"
	EnvKeyName     = "DEEPSEEK_API_KEY"
)

// ModelAliases maps friendly shortcut names (-m pro, -m flash, etc.) to official DeepSeek model slugs.
var ModelAliases = map[string]string{
	"flash":          "deepseek-flash",
	"deepseek-flash": "deepseek-flash",
	"v4-flash":       "deepseek-flash",
	"pro":            "deepseek-v4-pro",
	"deepseek-pro":   "deepseek-v4-pro",
	"deepseek-v4-pro": "deepseek-v4-pro",
	"v4-pro":         "deepseek-v4-pro",
	"chat":           "deepseek-chat",
	"deepseek-chat":  "deepseek-chat",
	"reasoner":       "deepseek-reasoner",
	"r1":             "deepseek-reasoner",
}

// ResolveModelAlias translates a shortcut model name to its canonical DeepSeek slug.
func ResolveModelAlias(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if canonical, ok := ModelAliases[trimmed]; ok {
		return canonical
	}
	return input
}

// StateDir returns the path to ~/.codexseek where CodexSeek stores its
// generated model catalog, sync logs, and optional user .env file.
func StateDir() (string, error) {
	if custom := os.Getenv("CODEXSEEK_HOME"); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codexseek"), nil
}

// MaskKey masks an API key following Jair Lima's security rule:
// first 8 chars + "..." + last 4 chars. Never prints a full key.
func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		if len(key) == 0 {
			return "<não configurada>"
		}
		return "********"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// ResolveAPIKey searches for DEEPSEEK_API_KEY in:
//  1. Current process environment (DEEPSEEK_API_KEY)
//  2. ~/.codexseek/.env or ~/.codexseek/credentials.env
//  3. ~/.api_keys/credentials.env (standard local vault)
//  4. WSL environment fallback on Windows (if available)
//
// Returns the key, its source label, or an error if not found.
func ResolveAPIKey() (string, string, error) {
	if val := strings.TrimSpace(os.Getenv(EnvKeyName)); val != "" {
		return val, "environment (" + EnvKeyName + ")", nil
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".codexseek", ".env"),
			filepath.Join(home, ".codexseek", "credentials.env"),
			filepath.Join(home, ".api_keys", "credentials.env"),
		}
		for _, path := range candidates {
			if val := readKeyFromEnvFile(path, EnvKeyName); val != "" {
				return val, path, nil
			}
		}
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("wsl", "-e", "bash", "-lc", `echo -n "$DEEPSEEK_API_KEY"`)
		if out, err := cmd.Output(); err == nil {
			if val := strings.TrimSpace(string(out)); val != "" {
				return val, "WSL (~/.bashrc)", nil
			}
		}
	}

	return "", "", fmt.Errorf(
		"variável %s não encontrada.\nConfigure com:\n  export %s=\"sua-chave\" (Linux/macOS)\n  $env:%s=\"sua-chave\" (PowerShell)\n  ou salve em ~/.codexseek/.env",
		EnvKeyName, EnvKeyName, EnvKeyName,
	)
}

func readKeyFromEnvFile(filePath, keyName string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k != keyName {
			continue
		}
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		if v != "" {
			return v
		}
	}
	return ""
}

// FindUpstreamCodexBinary locates the native Rust executable of the upstream
// OpenAI Codex CLI directly, bypassing .cmd wrappers on Windows to prevent
// temporary file / process leaks in PowerShell pipelines.
func FindUpstreamCodexBinary() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("CODEXSEEK_UPSTREAM_BIN")); custom != "" {
		if info, err := os.Stat(custom); err == nil && !info.IsDir() {
			return custom, nil
		}
	}

	home, _ := os.UserHomeDir()

	if runtime.GOOS == "windows" {
		// 1. Check npm global vendor binary directly (.exe, not .cmd!)
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		archTriple := "x86_64-pc-windows-msvc"
		pkgArch := "codex-win32-x64"
		if runtime.GOARCH == "arm64" {
			archTriple = "aarch64-pc-windows-msvc"
			pkgArch = "codex-win32-arm64"
		}

		candidates := []string{
			filepath.Join(appData, "npm", "node_modules", "@openai", "codex", "node_modules", "@openai", pkgArch, "vendor", archTriple, "bin", "codex.exe"),
			filepath.Join(appData, "npm", "node_modules", "@openai", pkgArch, "vendor", archTriple, "bin", "codex.exe"),
			filepath.Join(appData, "npm", "node_modules", "@openai", "codex", "vendor", archTriple, "bin", "codex.exe"),
		}

		// Also check LocalAppData OpenAI Codex bin directory
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" && home != "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		if localAppData != "" {
			matches, _ := filepath.Glob(filepath.Join(localAppData, "OpenAI", "Codex", "bin", "*", "codex.exe"))
			candidates = append(candidates, matches...)
		}

		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, nil
			}
		}
	} else {
		// Linux / macOS native binary locations inside npm global root
		cmd := exec.Command("npm", "root", "-g")
		if out, err := cmd.Output(); err == nil {
			npmRoot := strings.TrimSpace(string(out))
			var pkgArch, archTriple string
			if runtime.GOOS == "darwin" {
				if runtime.GOARCH == "arm64" {
					pkgArch = "codex-darwin-arm64"
					archTriple = "aarch64-apple-darwin"
				} else {
					pkgArch = "codex-darwin-x64"
					archTriple = "x86_64-apple-darwin"
				}
			} else {
				if runtime.GOARCH == "arm64" {
					pkgArch = "codex-linux-arm64"
					archTriple = "aarch64-unknown-linux-musl"
				} else {
					pkgArch = "codex-linux-x64"
					archTriple = "x86_64-unknown-linux-musl"
				}
			}
			candidates := []string{
				filepath.Join(npmRoot, "@openai", "codex", "node_modules", "@openai", pkgArch, "vendor", archTriple, "bin", "codex"),
				filepath.Join(npmRoot, "@openai", pkgArch, "vendor", archTriple, "bin", "codex"),
			}
			for _, c := range candidates {
				if info, err := os.Stat(c); err == nil && !info.IsDir() {
					return c, nil
				}
			}
		}
	}

	// Fallback to PATH lookup
	if p, err := exec.LookPath("codex"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("binário do OpenAI Codex CLI não encontrado. Instale com: npm install -g @openai/codex@latest")
}

// GetUpstreamVersion returns the version string reported by `codex --version`.
func GetUpstreamVersion(binPath string) string {
	cmd := exec.Command(binPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
