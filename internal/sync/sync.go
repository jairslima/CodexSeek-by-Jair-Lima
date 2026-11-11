// Package sync implements automated upstream synchronization (`codexseek sync-upstream`)
// and health verification (`codexseek doctor`), ensuring CodexSeek automatically absorbs
// all future releases of OpenAI Codex CLI while preserving its DeepSeek LLM integration.
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/catalog"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/config"
)

// SyncResult holds the outcome of an upstream check/update operation.
type SyncResult struct {
	PreviousVersion string `json:"previous_version"`
	CurrentVersion  string `json:"current_version"`
	LatestNPM       string `json:"latest_npm"`
	Updated         bool   `json:"updated"`
	CatalogPath     string `json:"catalog_path"`
	Timestamp       string `json:"timestamp"`
	LogFile         string `json:"log_file"`
}

// FetchLatestNPMVersion queries the official npm registry for the latest @openai/codex version.
func FetchLatestNPMVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://registry.npmjs.org/@openai/codex/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status HTTP inesperado do npm registry: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	return strings.TrimSpace(data.Version), nil
}

// RunSync checks if a newer version of @openai/codex is available, updates it if requested,
// rebuilds the DeepSeek model catalog JSON for the active binary, and writes an audit log.
func RunSync(checkOnly bool) (*SyncResult, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(stateDir, 0755)
	logPath := filepath.Join(stateDir, "sync-upstream.log")

	codexBin, err := config.FindUpstreamCodexBinary()
	prevVer := "none"
	if err == nil {
		prevVer = strings.TrimPrefix(config.GetUpstreamVersion(codexBin), "codex-cli ")
	}

	latestNPM, npmErr := FetchLatestNPMVersion()
	if npmErr != nil {
		latestNPM = prevVer
	}

	res := &SyncResult{
		PreviousVersion: prevVer,
		CurrentVersion:  prevVer,
		LatestNPM:       latestNPM,
		Updated:         false,
		Timestamp:       time.Now().Format(time.RFC3339),
		LogFile:         logPath,
	}

	if !checkOnly && latestNPM != "" && latestNPM != prevVer {
		fmt.Printf("[CodexSeek Sync] Nova versão upstream detectada: %s -> %s. Atualizando @openai/codex...\n", prevVer, latestNPM)
		cmd := exec.Command("npm", "install", "-g", "@openai/codex@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			appendLog(logPath, fmt.Sprintf("[%s] ERRO ao atualizar @openai/codex (%s -> %s): %v\n", res.Timestamp, prevVer, latestNPM, err))
			return res, fmt.Errorf("falha ao executar npm install -g @openai/codex@latest: %w", err)
		}
		res.Updated = true
	}

	// Re-resolve binary and regenerate DeepSeek catalog
	codexBin, err = config.FindUpstreamCodexBinary()
	if err != nil {
		return res, err
	}
	currVerRaw := config.GetUpstreamVersion(codexBin)
	res.CurrentVersion = strings.TrimPrefix(currVerRaw, "codex-cli ")

	catPath, err := catalog.EnsureCatalog(codexBin, currVerRaw, stateDir)
	if err != nil {
		return res, err
	}
	res.CatalogPath = catPath

	statusMsg := fmt.Sprintf("[%s] OK | Anterior: %s | Atual: %s | NPM Latest: %s | Atualizado: %v | Catalog: %s\n",
		res.Timestamp, res.PreviousVersion, res.CurrentVersion, res.LatestNPM, res.Updated, res.CatalogPath)
	appendLog(logPath, statusMsg)

	return res, nil
}

func appendLog(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

// RunDoctor performs a complete diagnostic of the CodexSeek installation,
// upstream Codex binary, model catalog, and DeepSeek API connectivity.
func RunDoctor() int {
	fmt.Printf("=== %s v%s — Diagnóstico de Saúde (Doctor) ===\n\n", config.AppName, config.AppVersion)
	exitCode := 0

	// 1. Upstream binary check
	codexBin, err := config.FindUpstreamCodexBinary()
	if err != nil {
		fmt.Printf("[FALHA] Binário upstream Codex CLI: %v\n", err)
		exitCode = 1
	} else {
		ver := config.GetUpstreamVersion(codexBin)
		fmt.Printf("[OK]    Motor Upstream Codex CLI: %s\n", ver)
		fmt.Printf("        Caminho Nativo: %s\n", codexBin)
	}

	// 2. State directory & Catalog check
	stateDir, err := config.StateDir()
	if err != nil {
		fmt.Printf("[FALHA] Diretório de estado (~/.codexseek): %v\n", err)
		exitCode = 1
	} else if codexBin != "" {
		ver := config.GetUpstreamVersion(codexBin)
		catPath, err := catalog.EnsureCatalog(codexBin, ver, stateDir)
		if err != nil {
			fmt.Printf("[FALHA] Catálogo de Modelos DeepSeek: %v\n", err)
			exitCode = 1
		} else {
			fmt.Printf("[OK]    Catálogo de Modelos DeepSeek: %s\n", catPath)
		}
	}

	// 3. API Key check (masked)
	apiKey, source, err := config.ResolveAPIKey()
	if err != nil {
		fmt.Printf("[FALHA] Credencial DeepSeek (%s): %v\n", config.EnvKeyName, err)
		exitCode = 1
	} else {
		fmt.Printf("[OK]    Credencial DeepSeek (%s): %s (Fonte: %s)\n", config.EnvKeyName, config.MaskKey(apiKey), source)
	}

	// 4. Live DeepSeek API check (/models)
	if apiKey != "" {
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", "https://api.deepseek.com/models", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[FALHA] Conectividade API DeepSeek: %v\n", err)
			exitCode = 1
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("[OK]    Conectividade API DeepSeek (https://api.deepseek.com/v1): HTTP 200 OK\n")
			} else {
				fmt.Printf("[FALHA] Conectividade API DeepSeek: HTTP %d\n", resp.StatusCode)
				exitCode = 1
			}
		}
	}

	// 5. Configured models summary
	fmt.Println("\nModelos DeepSeek pré-configurados no CodexSeek:")
	for _, m := range catalog.DefaultModels() {
		fmt.Printf("  - %-18s | %-20s | Contexto: %d tokens\n", m.Slug, m.DisplayName, m.ContextWindow)
	}

	fmt.Println("\nAtalhos rápidos de linha de comando:")
	fmt.Println("  codexseek --flash ...       -> Usa DeepSeek V4.1 Flash (padrão)")
	fmt.Println("  codexseek --pro ...         -> Usa DeepSeek V4 Pro")
	fmt.Println("  codexseek -m pro exec ...   -> Execução headless H2H com DeepSeek V4 Pro")
	fmt.Println("  codexseek sync-upstream     -> Sincroniza automaticamente com a última versão do Codex CLI")
	fmt.Println("  codexseek doctor            -> Executa este diagnóstico completo")

	return exitCode
}
