// Package catalog extracts and adapts the embedded model catalog from the
// upstream Codex CLI binary so DeepSeek models (deepseek-flash, deepseek-v4-pro,
// deepseek-chat, deepseek-reasoner) run natively with 1M context window,
// zero fallback metadata warnings, compatible tool definitions, and built-in
// Recap + Balance instructions.
//
// Project: CodexSeek by Jair Lima
// Author: Jair Lima
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeepSeekModelSpec describes a DeepSeek model to register in the catalog.
type DeepSeekModelSpec struct {
	Slug            string
	DisplayName     string
	Description     string
	InputModalities []string
	ContextWindow   int
	Priority        int
}

// DefaultModels returns the official DeepSeek models configured for CodexSeek by Jair Lima.
func DefaultModels() []DeepSeekModelSpec {
	return []DeepSeekModelSpec{
		{
			Slug:            "deepseek-flash",
			DisplayName:     "DeepSeek V4.1 Flash",
			Description:     "Ultra-fast 1M context coding and reasoning model by DeepSeek.",
			InputModalities: []string{"text", "image"},
			ContextWindow:   1048576,
			Priority:        1,
		},
		{
			Slug:            "deepseek-v4-pro",
			DisplayName:     "DeepSeek V4 Pro",
			Description:     "Frontier deep reasoning and software engineering model by DeepSeek (1M context).",
			InputModalities: []string{"text"},
			ContextWindow:   1048576,
			Priority:        2,
		},
		{
			Slug:            "deepseek-chat",
			DisplayName:     "DeepSeek Chat",
			Description:     "General purpose and coding model by DeepSeek.",
			InputModalities: []string{"text"},
			ContextWindow:   128000,
			Priority:        3,
		},
		{
			Slug:            "deepseek-reasoner",
			DisplayName:     "DeepSeek Reasoner",
			Description:     "Chain-of-thought reasoning model by DeepSeek.",
			InputModalities: []string{"text"},
			ContextWindow:   128000,
			Priority:        4,
		},
	}
}

// EnsureCatalog generates or updates the DeepSeek model catalog JSON file in
// ~/.codexseek/deepseek_catalog_<version>.json by extracting the exact schema
// from the installed Codex binary. This guarantees 100% forward compatibility
// whenever the upstream Codex CLI updates its internal Rust struct schema.
func EnsureCatalog(codexBinPath, codexVersion, stateDir string, forceRebuild bool) (string, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create state dir: %w", err)
	}

	safeVer := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, codexVersion)
	if safeVer == "" {
		safeVer = "latest"
	}

	// Schema revision v3 includes Recap + Balance instructions (with CODEXSEEK_BALANCE env & codexseek.exe balance --short)
	catalogPath := filepath.Join(stateDir, fmt.Sprintf("deepseek_catalog_%s_r3.json", safeVer))
	if !forceRebuild {
		if info, err := os.Stat(catalogPath); err == nil && info.Size() > 1024 {
			return catalogPath, nil
		}
	}

	rawCatalog, err := extractEmbeddedCatalogFromBinary(codexBinPath)
	if err != nil {
		rawCatalog = buildFallbackTemplate()
	}

	adaptedBytes, err := buildDeepSeekCatalogJSON(rawCatalog)
	if err != nil {
		return "", fmt.Errorf("failed to build DeepSeek catalog JSON: %w", err)
	}

	if err := os.WriteFile(catalogPath, adaptedBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write catalog file: %w", err)
	}

	latestPath := filepath.Join(stateDir, "deepseek_catalog.json")
	_ = os.WriteFile(latestPath, adaptedBytes, 0644)

	return catalogPath, nil
}

func extractEmbeddedCatalogFromBinary(binPath string) (map[string]any, error) {
	data, err := os.ReadFile(binPath)
	if err != nil {
		return nil, err
	}

	needle := []byte(`"slug": "gpt-6-astra"`)
	pos := bytes.Index(data, needle)
	if pos == -1 {
		needle = []byte(`"slug": "gpt-5.5"`)
		pos = bytes.Index(data, needle)
	}
	if pos == -1 {
		return nil, fmt.Errorf("embedded catalog marker not found in binary")
	}

	searchStart := pos - 200
	if searchStart < 0 {
		searchStart = 0
	}
	modelsMarker := bytes.LastIndex(data[searchStart:pos], []byte(`"models"`))
	if modelsMarker == -1 {
		return nil, fmt.Errorf("models array marker not found")
	}
	absModelsPos := searchStart + modelsMarker
	outerBrace := bytes.LastIndexByte(data[searchStart:absModelsPos], '{')
	if outerBrace == -1 {
		return nil, fmt.Errorf("outer opening brace not found")
	}
	start := searchStart + outerBrace

	depth := 0
	inString := false
	escaped := false
	end := -1
	for i := start; i < len(data); i++ {
		b := data[i]
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' && inString {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if !inString {
			if b == '{' {
				depth++
			} else if b == '}' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
	}

	if end == -1 {
		return nil, fmt.Errorf("matching closing brace not found")
	}

	var parsed map[string]any
	if err := json.Unmarshal(data[start:end], &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

const codexSeekRecapInstructions = `

## CodexSeek Recap & DeepSeek Balance Instructions (by Jair Lima)
Whenever the user invokes ` + "`$recap`" + `, ` + "`/recap`" + `, asks for a recap/resumo da sessão, or when finishing a multi-step operational task, include a concise single-line recap at the end in Brazilian Portuguese with the live DeepSeek account balance (available in the environment variable ` + "`$env:CODEXSEEK_BALANCE`" + ` or by running ` + "`codexseek.exe balance --short`" + `):
` + "`recap: <o que foi concluído>. Próximo passo: <ação concreta ou decisão pendente>. [Saldo DeepSeek: <valor USD>]`" + `
`

func buildDeepSeekCatalogJSON(upstream map[string]any) ([]byte, error) {
	modelsSlice, ok := upstream["models"].([]any)
	if !ok || len(modelsSlice) == 0 {
		return nil, fmt.Errorf("upstream catalog has no models array")
	}

	var baseTemplate map[string]any
	for _, item := range modelsSlice {
		if m, ok := item.(map[string]any); ok {
			if slug, _ := m["slug"].(string); slug == "gpt-5.5" {
				baseTemplate = m
				break
			}
		}
	}
	if baseTemplate == nil {
		baseTemplate, _ = modelsSlice[0].(map[string]any)
	}

	baseBytes, err := json.Marshal(baseTemplate)
	if err != nil {
		return nil, err
	}

	specs := DefaultModels()
	outModels := make([]any, 0, len(specs))

	for _, spec := range specs {
		var clone map[string]any
		if err := json.Unmarshal(baseBytes, &clone); err != nil {
			return nil, err
		}

		clone["slug"] = spec.Slug
		clone["display_name"] = spec.DisplayName
		clone["description"] = spec.Description
		clone["input_modalities"] = spec.InputModalities
		clone["context_window"] = spec.ContextWindow
		clone["max_context_window"] = spec.ContextWindow
		clone["priority"] = spec.Priority

		clone["apply_patch_tool_type"] = nil
		clone["tool_mode"] = nil
		clone["prefer_websockets"] = false
		clone["use_responses_lite"] = false
		clone["supports_search_tool"] = false
		clone["upgrade"] = nil
		clone["availability_nux"] = nil

		if mm, ok := clone["model_messages"].(map[string]any); ok && mm != nil {
			if tmpl, ok := mm["instructions_template"].(string); ok && tmpl != "" {
				tmpl = strings.ReplaceAll(
					tmpl,
					"You are Codex, a coding agent based on GPT-5.",
					fmt.Sprintf("You are CodexSeek (by Jair Lima), an advanced coding agent powered by %s (%s) on the Codex harness.", spec.DisplayName, spec.Slug),
				)
				tmpl = strings.ReplaceAll(
					tmpl,
					"You are Codex, an agent based on GPT-6.",
					fmt.Sprintf("You are CodexSeek (by Jair Lima), an advanced coding agent powered by %s (%s) on the Codex harness.", spec.DisplayName, spec.Slug),
				)
				mm["instructions_template"] = tmpl + codexSeekRecapInstructions
			}
		}

		outModels = append(outModels, clone)
	}

	result := map[string]any{
		"models": outModels,
	}
	return json.MarshalIndent(result, "", "  ")
}

func buildFallbackTemplate() map[string]any {
	return map[string]any{
		"models": []any{
			map[string]any{
				"slug":                              "gpt-5.5",
				"prefer_websockets":                 false,
				"support_verbosity":                 true,
				"default_verbosity":                 "low",
				"apply_patch_tool_type":             nil,
				"web_search_tool_type":              "text_and_image",
				"input_modalities":                  []string{"text", "image"},
				"supports_image_detail_original":    true,
				"truncation_policy":                 map[string]any{"mode": "tokens", "limit": 10000},
				"supports_parallel_tool_calls":      true,
				"tool_mode":                         nil,
				"multi_agent_version":               nil,
				"multi_agent_reasoning_effort":      nil,
				"use_responses_lite":                false,
				"supports_reasoning_effort_updates": false,
				"include_skills_usage_instructions": true,
				"include_apps_usage_instructions":   true,
				"include_plugin_usage_instructions": true,
				"guardian":                          nil,
				"node_repl_auto_review_required":    false,
				"node_repl_disabled":                false,
				"requires_sandboxed_review":         false,
				"auto_review_model_override":        nil,
				"model_specialty":                   nil,
				"context_window":                    1048576,
				"max_context_window":                1048576,
				"auto_compact_token_limit":          nil,
				"comp_hash":                         "2911",
				"default_reasoning_summary":         "none",
				"display_name":                      "DeepSeek V4.1 Flash",
				"description":                       "DeepSeek coding model.",
				"default_reasoning_level":           "medium",
				"supported_reasoning_levels": []any{
					map[string]any{"effort": "low", "description": "Fast responses"},
					map[string]any{"effort": "medium", "description": "Balanced reasoning"},
					map[string]any{"effort": "high", "description": "Deep reasoning"},
				},
				"shell_type":             "shell_command",
				"visibility":             "list",
				"minimal_client_version": "0.124.0",
				"supported_in_api":       true,
				"availability_nux":       nil,
				"upgrade":                nil,
				"priority":               1,
				"model_messages": map[string]any{
					"instructions_template":   "You are CodexSeek (by Jair Lima), a coding agent powered by DeepSeek." + codexSeekRecapInstructions,
					"persistent_instructions": nil,
					"tools":                   nil,
					"approvals":               nil,
					"collaboration_modes":     nil,
					"auto_review":             nil,
					"multi_agent":             nil,
					"permissions":             nil,
					"token_budget":            nil,
					"guardian_v2":             nil,
					"confirmation_policies":   nil,
				},
				"experimental_supported_tools": []any{},
				"available_in_plans":           []string{"free", "plus", "pro", "team", "enterprise"},
				"supports_search_tool":         false,
				"supports_experimental_context": false,
				"default_service_tier":         nil,
				"service_tiers": []any{
					map[string]any{"id": "priority", "name": "Fast", "description": "Fast"},
				},
				"additional_speed_tiers":              []string{"fast"},
				"supports_reasoning_summary_parameter": true,
				"supports_reasoning_summaries":         true,
			},
		},
	}
}
