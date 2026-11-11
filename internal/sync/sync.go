// Package sync implements automated upstream synchronization (`codexseek sync-upstream`),
// DeepSeek account balance inspection (`codexseek balance`), session recap + balance
// (`codexseek recap`), and health verification (`codexseek doctor`).
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package sync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	CleanedTempDirs int    `json:"cleaned_temp_dirs"`
	CatalogPath     string `json:"catalog_path"`
	Timestamp       string `json:"timestamp"`
	LogFile         string `json:"log_file"`
}

// BalanceInfo represents a currency balance item returned by GET https://api.deepseek.com/user/balance.
type BalanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

// BalanceResponse represents the full payload from GET https://api.deepseek.com/user/balance.
type BalanceResponse struct {
	IsAvailable  bool          `json:"is_available"`
	BalanceInfos []BalanceInfo `json:"balance_infos"`
}

// FetchDeepSeekBalance queries the official DeepSeek API balance endpoint.
// Whenever a live fetch succeeds, it caches the result in ~/.codexseek/last-balance.json.
// If the HTTP call fails (for instance, inside Codex's network-restricted sandbox proxy),
// it seamlessly falls back to the cached balance in ~/.codexseek/last-balance.json.
func FetchDeepSeekBalance(apiKey string) (*BalanceResponse, error) {
	stateDir, _ := config.StateDir()
	cachePath := ""
	if stateDir != "" {
		_ = os.MkdirAll(stateDir, 0755)
		cachePath = filepath.Join(stateDir, "last-balance.json")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://api.deepseek.com/user/balance", nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, doErr := client.Do(req)
		if doErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if body, readErr := io.ReadAll(resp.Body); readErr == nil {
					var bal BalanceResponse
					if json.Unmarshal(body, &bal) == nil {
						if cachePath != "" {
							_ = os.WriteFile(cachePath, body, 0644)
						}
						return &bal, nil
					}
				}
			} else {
				err = fmt.Errorf("HTTP %d ao consultar saldo DeepSeek", resp.StatusCode)
			}
		} else {
			err = doErr
		}
	}

	// Fallback to cached balance when network is blocked by sandbox
	if cachePath != "" {
		if cachedBytes, readErr := os.ReadFile(cachePath); readErr == nil {
			var cachedBal BalanceResponse
			if json.Unmarshal(cachedBytes, &cachedBal) == nil && len(cachedBal.BalanceInfos) > 0 {
				return &cachedBal, nil
			}
		}
	}
	return nil, err
}

// FormatBalanceSummary returns a concise human-readable string of the DeepSeek balance.
func FormatBalanceSummary(bal *BalanceResponse) string {
	if bal == nil || len(bal.BalanceInfos) == 0 {
		return "indisponível"
	}
	parts := make([]string, 0, len(bal.BalanceInfos))
	for _, info := range bal.BalanceInfos {
		parts = append(parts, fmt.Sprintf("%s %s (recarga: %s, bônus: %s)",
			info.TotalBalance, info.Currency, info.ToppedUpBalance, info.GrantedBalance))
	}
	status := "ativo"
	if !bal.IsAvailable {
		status = "esgotado/indisponível"
	}
	return fmt.Sprintf("%s [%s]", strings.Join(parts, " | "), status)
}

// CleanOrphanedNPMDirs removes stale `.codex-*` temporary directories left behind by npm on Windows
// so `npm install -g @openai/codex@latest` never triggers EPERM cleanup warnings.
func CleanOrphanedNPMDirs() int {
	if runtime.GOOS != "windows" {
		return 0
	}
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	if appData == "" && home != "" {
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	if appData == "" {
		return 0
	}
	openaiScopeDir := filepath.Join(appData, "npm", "node_modules", "@openai")
	entries, err := os.ReadDir(openaiScopeDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".codex-") {
			fullPath := filepath.Join(openaiScopeDir, entry.Name())
			if err := os.RemoveAll(fullPath); err == nil {
				removed++
			}
		}
	}
	return removed
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

// RunSync checks if a newer version of @openai/codex is available, cleans orphaned npm temp dirs,
// updates @openai/codex if needed, rebuilds the DeepSeek model catalog JSON, and writes an audit log.
func RunSync(checkOnly bool) (*SyncResult, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(stateDir, 0755)
	logPath := filepath.Join(stateDir, "sync-upstream.log")

	cleanedCount := CleanOrphanedNPMDirs()

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
		CleanedTempDirs: cleanedCount,
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
		_ = CleanOrphanedNPMDirs()
		res.Updated = true
	}

	codexBin, err = config.FindUpstreamCodexBinary()
	if err != nil {
		return res, err
	}
	currVerRaw := config.GetUpstreamVersion(codexBin)
	res.CurrentVersion = strings.TrimPrefix(currVerRaw, "codex-cli ")

	catPath, err := catalog.EnsureCatalog(codexBin, currVerRaw, stateDir, res.Updated)
	if err != nil {
		return res, err
	}
	res.CatalogPath = catPath

	statusMsg := fmt.Sprintf("[%s] OK | Anterior: %s | Atual: %s | NPM Latest: %s | Atualizado: %v | TempLimpos: %d | Catalog: %s\n",
		res.Timestamp, res.PreviousVersion, res.CurrentVersion, res.LatestNPM, res.Updated, res.CleanedTempDirs, res.CatalogPath)
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

// HandleTurnEndedNotify handles the `notify = ["codexseek.exe", "turn-ended", "<json>"]` callback
// invoked by the Codex engine at the end of every turn, logging a live recap + DeepSeek balance
// snapshot to ~/.codexseek/last-recap.json and ~/.codexseek/recap-history.log.
func HandleTurnEndedNotify(payloadJSON string) int {
	stateDir, err := config.StateDir()
	if err != nil {
		return 0
	}
	_ = os.MkdirAll(stateDir, 0755)

	balText := "indisponível"
	if apiKey, _, err := config.ResolveAPIKey(); err == nil && apiKey != "" {
		if bal, err := FetchDeepSeekBalance(apiKey); err == nil {
			balText = FormatBalanceSummary(bal)
		}
	}

	var event map[string]any
	_ = json.Unmarshal([]byte(payloadJSON), &event)

	lastMsg, _ := event["last-assistant-message"].(string)
	cwd, _ := event["cwd"].(string)
	threadID, _ := event["thread-id"].(string)

	firstLine := strings.TrimSpace(lastMsg)
	if idx := strings.IndexAny(firstLine, "\r\n"); idx != -1 {
		firstLine = strings.TrimSpace(firstLine[:idx])
	}
	if len([]rune(firstLine)) > 160 {
		firstLine = string([]rune(firstLine)[:160]) + "..."
	}

	recapSnap := map[string]any{
		"timestamp":        time.Now().Format(time.RFC3339),
		"thread_id":        threadID,
		"cwd":              cwd,
		"summary":          firstLine,
		"deepseek_balance": balText,
	}
	if b, err := json.MarshalIndent(recapSnap, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(stateDir, "last-recap.json"), b, 0644)
	}

	logLine := fmt.Sprintf("[%s] Saldo DeepSeek: %s | Dir: %s | Resumo: %s\n",
		time.Now().Format("2006-01-02 15:04:05"), balText, cwd, firstLine)
	appendLog(filepath.Join(stateDir, "recap-history.log"), logLine)
	return 0
}

// RunRecap displays the consolidated session recap (latest tasks, H2H usage history,
// and live DeepSeek account balance).
func RunRecap() int {
	fmt.Printf("=== %s v%s — Recap da Sessão & Saldo DeepSeek ===\n\n", config.AppName, config.AppVersion)

	apiKey, _, err := config.ResolveAPIKey()
	balText := "indisponível"
	if err == nil && apiKey != "" {
		if bal, err := FetchDeepSeekBalance(apiKey); err == nil {
			balText = FormatBalanceSummary(bal)
		}
	}
	fmt.Printf("💰 Saldo Atual DeepSeek API : %s\n", balText)

	stateDir, _ := config.StateDir()
	usageFile := filepath.Join(stateDir, "h2h-usage.jsonl")
	lines := readLastLines(usageFile, 5)

	if len(lines) > 0 {
		fmt.Println("\n📋 Últimas Execuções Registradas (H2H / CLI):")
		for _, l := range lines {
			var rec struct {
				Timestamp   string  `json:"timestamp"`
				Mode        string  `json:"mode"`
				Model       string  `json:"model"`
				WorkDir     string  `json:"workdir"`
				DurationSec float64 `json:"duration_sec"`
				ExitCode    int     `json:"exit_code"`
				PromptBrief string  `json:"prompt_brief"`
			}
			if json.Unmarshal([]byte(l), &rec) == nil {
				ts := rec.Timestamp
				if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
					ts = t.Format("02/01 15:04:05")
				}
				brief := rec.PromptBrief
				if brief == "" {
					brief = "(sessão interativa TUI)"
				}
				fmt.Printf("  • [%s] %-16s | %.1fs | exit %d | %s\n    Tarefa: %s\n",
					ts, rec.Model, rec.DurationSec, rec.ExitCode, rec.WorkDir, brief)
			}
		}
	}

	// Also check latest Codex rollout session summary if present
	if latestSession := findLatestCodexRolloutSummary(); latestSession != "" {
		fmt.Printf("\n🔄 Última Resposta na Sessão Ativa:\n  %s\n", latestSession)
	}

	fmt.Printf("\n📌 Recap Consolidado:\n")
	fmt.Printf("  recap: Motor CodexSeek operacional (Codex CLI + DeepSeek 1M) | Saldo DeepSeek: %s.\n", balText)
	return 0
}

func readLastLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var all []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			all = append(all, t)
		}
	}
	if len(all) > n {
		return all[len(all)-n:]
	}
	return all
}

func findLatestCodexRolloutSummary() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	sessRoot := filepath.Join(home, ".codex", "sessions")
	var files []struct {
		path string
		mod  time.Time
	}
	_ = filepath.WalkDir(sessRoot, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".jsonl") {
			if info, err := d.Info(); err == nil {
				files = append(files, struct {
					path string
					mod  time.Time
				}{p, info.ModTime()})
			}
		}
		return nil
	})
	if len(files) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.After(files[j].mod)
	})
	lines := readLastLines(files[0].path, 30)
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], `"role":"assistant"`) || strings.Contains(lines[i], `"type":"message"`) {
			var obj map[string]any
			if json.Unmarshal([]byte(lines[i]), &obj) == nil {
				if payload, ok := obj["payload"].(map[string]any); ok {
					if content, ok := payload["content"].([]any); ok {
						for _, c := range content {
							if cm, ok := c.(map[string]any); ok {
								if txt, ok := cm["text"].(string); ok && strings.TrimSpace(txt) != "" {
									clean := strings.TrimSpace(txt)
									if len([]rune(clean)) > 200 {
										clean = string([]rune(clean)[:200]) + "..."
									}
									return clean
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// RunBalance prints the current DeepSeek account balance.
func RunBalance(shortOnly, jsonOut bool) int {
	apiKey, _, err := config.ResolveAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CodexSeek] Erro: %v\n", err)
		return 1
	}
	bal, err := FetchDeepSeekBalance(apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CodexSeek] Erro ao consultar saldo: %v\n", err)
		return 1
	}
	if jsonOut {
		b, _ := json.MarshalIndent(bal, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if shortOnly {
		fmt.Println(FormatBalanceSummary(bal))
		return 0
	}
	fmt.Printf("=== %s — Saldo da Conta DeepSeek API ===\n", config.AppName)
	fmt.Printf("  Disponível para uso : %v\n", bal.IsAvailable)
	for _, info := range bal.BalanceInfos {
		fmt.Printf("  Moeda               : %s\n", info.Currency)
		fmt.Printf("  Saldo Total         : %s %s\n", info.TotalBalance, info.Currency)
		fmt.Printf("  Saldo Recarregado   : %s %s\n", info.ToppedUpBalance, info.Currency)
		fmt.Printf("  Saldo Bônus/Granted : %s %s\n", info.GrantedBalance, info.Currency)
	}
	return 0
}

// RunDoctor performs a complete diagnostic of the CodexSeek installation,
// upstream Codex binary, model catalog, DeepSeek API connectivity, and live account balance.
func RunDoctor() int {
	fmt.Printf("=== %s v%s — Diagnóstico de Saúde (Doctor) ===\n\n", config.AppName, config.AppVersion)
	exitCode := 0

	codexBin, err := config.FindUpstreamCodexBinary()
	if err != nil {
		fmt.Printf("[FALHA] Binário upstream Codex CLI: %v\n", err)
		exitCode = 1
	} else {
		ver := config.GetUpstreamVersion(codexBin)
		fmt.Printf("[OK]    Motor Upstream Codex CLI: %s\n", ver)
		fmt.Printf("        Caminho Nativo: %s\n", codexBin)
	}

	stateDir, err := config.StateDir()
	if err != nil {
		fmt.Printf("[FALHA] Diretório de estado (~/.codexseek): %v\n", err)
		exitCode = 1
	} else if codexBin != "" {
		ver := config.GetUpstreamVersion(codexBin)
		catPath, err := catalog.EnsureCatalog(codexBin, ver, stateDir, false)
		if err != nil {
			fmt.Printf("[FALHA] Catálogo de Modelos DeepSeek: %v\n", err)
			exitCode = 1
		} else {
			fmt.Printf("[OK]    Catálogo de Modelos DeepSeek: %s\n", catPath)
		}
	}

	apiKey, source, err := config.ResolveAPIKey()
	if err != nil {
		fmt.Printf("[FALHA] Credencial DeepSeek (%s): %v\n", config.EnvKeyName, err)
		exitCode = 1
	} else {
		fmt.Printf("[OK]    Credencial DeepSeek (%s): %s (Fonte: %s)\n", config.EnvKeyName, config.MaskKey(apiKey), source)
	}

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

		if bal, err := FetchDeepSeekBalance(apiKey); err == nil {
			fmt.Printf("[OK]    Saldo da Conta DeepSeek: %s\n", FormatBalanceSummary(bal))
		} else {
			fmt.Printf("[AVISO] Não foi possível ler o saldo da conta DeepSeek: %v\n", err)
		}
	}

	fmt.Println("\n[OK]    Modo Recap Automático (tui.auto_recap + /recap + codexseek recap): ATIVO")

	fmt.Println("\nModelos DeepSeek pré-configurados no CodexSeek:")
	for _, m := range catalog.DefaultModels() {
		fmt.Printf("  - %-18s | %-20s | Contexto: %d tokens\n", m.Slug, m.DisplayName, m.ContextWindow)
	}

	fmt.Println("\nAtalhos rápidos de linha de comando:")
	fmt.Println("  codexseek --flash ...       -> Usa DeepSeek V4.1 Flash (padrão)")
	fmt.Println("  codexseek --pro ...         -> Usa DeepSeek V4 Pro")
	fmt.Println("  codexseek balance (ou saldo)-> Mostra o saldo atual da conta DeepSeek em USD")
	fmt.Println("  codexseek recap             -> Mostra o recap da sessão, histórico H2H e saldo DeepSeek")
	fmt.Println("  codexseek update            -> Limpa temporários e sincroniza com a última versão do Codex CLI")
	fmt.Println("  codexseek doctor            -> Executa este diagnóstico completo")

	return exitCode
}
