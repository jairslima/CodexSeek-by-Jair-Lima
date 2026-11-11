// CodexSeek by Jair Lima — Harness OpenAI Codex CLI pré-otimizado para a API DeepSeek.
//
// Copyright (c) 2026 Jair Lima. Licensed under MIT / Apache-2.0.
// Upstream engine: OpenAI Codex CLI (https://github.com/openai/codex), Apache-2.0.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/catalog"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/config"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/runner"
	"github.com/jairslima/CodexSeek-by-Jair-Lima/internal/sync"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "-V", "--version", "version":
			codexBin, err := config.FindUpstreamCodexBinary()
			upstreamVer := "unknown"
			if err == nil {
				upstreamVer = config.GetUpstreamVersion(codexBin)
			}
			fmt.Printf("%s v%s (upstream engine: %s | default provider: DeepSeek)\n",
				config.AppName, config.AppVersion, upstreamVer)
			os.Exit(0)

		case "doctor":
			os.Exit(sync.RunDoctor())

		case "models":
			fmt.Printf("=== %s — Modelos DeepSeek Disponíveis ===\n", config.AppName)
			for _, m := range catalog.DefaultModels() {
				defTag := ""
				if m.Slug == config.DefaultModel {
					defTag = " [PADRÃO]"
				}
				fmt.Printf("  * %-18s (%s)%s\n    Janela de contexto: %d tokens | Modalidades: %v\n    %s\n\n",
					m.Slug, m.DisplayName, defTag, m.ContextWindow, m.InputModalities, m.Description)
			}
			os.Exit(0)

		case "sync-upstream", "update":
			checkOnly := false
			jsonOut := false
			for _, a := range args[1:] {
				if a == "--check" || a == "--dry-run" {
					checkOnly = true
				}
				if a == "--json" {
					jsonOut = true
				}
			}
			res, err := sync.RunSync(checkOnly)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Erro na sincronização upstream: %v\n", err)
				os.Exit(1)
			}
			if jsonOut {
				b, _ := json.MarshalIndent(res, "", "  ")
				fmt.Println(string(b))
			} else {
				fmt.Printf("=== %s — Sincronização Upstream Concluída ===\n", config.AppName)
				fmt.Printf("  Versão anterior : %s\n", res.PreviousVersion)
				fmt.Printf("  Versão atual    : %s\n", res.CurrentVersion)
				fmt.Printf("  Última no NPM   : %s\n", res.LatestNPM)
				fmt.Printf("  Atualizado agora: %v\n", res.Updated)
				fmt.Printf("  Catálogo gerado : %s\n", res.CatalogPath)
				fmt.Printf("  Log de auditoria: %s\n", res.LogFile)
			}
			os.Exit(0)

		case "-h", "--help":
			printBanner()
		}
	}

	exitCode, err := runner.Run(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CodexSeek] Erro: %v\n", err)
	}
	os.Exit(exitCode)
}

func printBanner() {
	fmt.Printf(`================================================================================
  %s v%s
  Harness OpenAI Codex CLI + LLM DeepSeek API (1M Context Window)
  Autor: Jair Lima | Licença: MIT & Apache-2.0
================================================================================
Atalhos exclusivos do CodexSeek:
  --flash                   Usa o modelo rápido DeepSeek V4.1 Flash (deepseek-flash, padrão)
  --pro                     Usa o modelo de raciocínio profundo DeepSeek V4 Pro (deepseek-v4-pro)
  -m <flash|pro|chat|r1>    Seleciona o modelo DeepSeek por alias curto ou slug oficial
  codexseek models          Lista os modelos DeepSeek pré-configurados e suas capacidades
  codexseek doctor          Executa diagnóstico completo (binário, catálogo, chave e API)
  codexseek sync-upstream   Atualiza automaticamente o motor Codex CLI e regenera o catálogo

Abaixo seguem os comandos e opções nativas do motor Codex CLI:
--------------------------------------------------------------------------------
`, config.AppName, config.AppVersion)
}
