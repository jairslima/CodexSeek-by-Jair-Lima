# PROJECT.md: CodexSeek by Jair Lima

## 1. Nome e Descrição
**CodexSeek by Jair Lima** (`codexseek`) é um fork/overlay open source de alta performance que une o harness completo do **OpenAI Codex CLI** (sandbox, shell, MCP, sessões e modo headless `exec`) ao poder dos modelos da **API da DeepSeek** (`deepseek-flash` / DeepSeek V4.1 Flash e `deepseek-v4-pro` / DeepSeek V4 Pro, com janela de contexto de 1.048.576 tokens).

Foi projetado com arquitetura de **Superfície Mínima / Overlay Dinâmico**, permitindo absorver todas as atualizações futuras do Codex CLI automaticamente (`codexseek sync-upstream`) sem conflitos de merge, sem perder as customizações da DeepSeek e sem jamais expor a chave de API do autor.

---

## 2. Stack Tecnológica
- **Linguagem Principal (Binário Nativo):** Go 1.22+ (`cmd/codexseek`, `internal/catalog`, `internal/config`, `internal/runner`, `internal/sync`)
- **Launcher Multiplataforma NPM:** Node.js ESM (`npm/bin/codexseek.mjs`)
- **Motor Upstream:** OpenAI Codex CLI (`@openai/codex`, binário nativo Rust invocado diretamente sem wrapper `.cmd`)
- **Provedor LLM Padrão:** DeepSeek API (`https://api.deepseek.com/v1` via protocolo `wire_api = "responses"`)
- **CI/CD:** GitHub Actions (`.github/workflows/ci.yml` e `.github/workflows/sync-upstream.yml`)

---

## 3. Estrutura de Pastas
```
C:\Users\jairs\Antigravity\CodexSeek-by-Jair-Lima\
├── cmd\codexseek\main.go               # CLI principal do CodexSeek by Jair Lima
├── internal\
│   ├── catalog\catalog.go              # Extrator do schema binário do Codex e gerador do catálogo DeepSeek 1M
│   ├── config\config.go                # Resolução segura de DEEPSEEK_API_KEY, mascaramento e localização do codex.exe
│   ├── config\config_test.go           # Testes unitários de aliases e mascaramento de segredos
│   ├── runner\runner.go                # Orquestrador de processo, flags de overlay e sinais de sistema
│   ├── runner\runner_test.go           # Testes unitários de transformação de argumentos
│   └── sync\sync.go                    # Sincronizador automático com NPM (@openai/codex@latest) e comando doctor
├── npm\
│   ├── package.json                    # Pacote NPM público multiplataforma
│   └── bin\codexseek.mjs               # Entrypoint Node.js ESM
├── scripts\
│   └── pre-commit-secret-scan.ps1      # Auditoria automática contra exposição de chaves antes de commits
├── .github\workflows\
│   ├── ci.yml                          # Pipeline multiplataforma (Windows, Linux, macOS) + Secret Gate
│   └── sync-upstream.yml               # Cron job diário de sincronização com @openai/codex
├── UPSTREAM_VERSION.txt                # Versão homologada do motor upstream
├── LICENSE                             # Licença MIT (Jair Lima) + Notice Apache 2.0 (OpenAI)
├── NOTICE                              # Atribuição legal Apache 2.0
├── SPEC.md                             # Especificação técnica detalhada
├── PROJECT.md                          # Este documento de continuidade
├── GEMINI.md                           # Resumo técnico para IAs
└── README.md                           # Documentação pública do repositório
```

---

## 4. Comandos de Build, Test, Run e Deploy

### Testes Unitários
```powershell
cd "C:\Users\jairs\Antigravity\CodexSeek-by-Jair-Lima"
go test ./... -v
```

### Build e Deploy Global (`C:\Users\jairs\bin\codexseek.exe`)
Seguindo a regra global de deploy do autor, o executável compilado fica instalado diretamente em `C:\Users\jairs\bin\codexseek.exe` (e nunca é executado a partir de pastas temporárias `dist`/`build` do projeto):
```powershell
cd "C:\Users\jairs\Antigravity\CodexSeek-by-Jair-Lima"
go build -ldflags="-s -w" -o "C:\Users\jairs\bin\codexseek.exe" ./cmd/codexseek
```

### Execução e Diagnóstico
```powershell
codexseek --version
codexseek doctor
codexseek models
codexseek sync-upstream
codexseek exec --skip-git-repo-check -s workspace-write -C "C:\caminho\projeto" "Tarefa"
codexseek --pro exec --skip-git-repo-check -s workspace-write -C "C:\caminho\projeto" "Tarefa complexa"
```

---

## 5. Local de Instalação e Atalhos
- **Executável Oficial no PATH:** `C:\Users\jairs\bin\codexseek.exe`
- **Função Nativa nos Perfis PowerShell:** Registrada em `C:\Users\jairs\Documents\PowerShell\Microsoft.PowerShell_profile.ps1` e `C:\Users\jairs\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1` chamando diretamente `C:\Users\jairs\bin\codexseek.exe` (sem `.cmd`).
- **Diretório de Estado Isolado:** `C:\Users\jairs\.codexseek\` (contém o catálogo `deepseek_catalog.json` e o log de auditoria `sync-upstream.log`).

---

## 6. Estado Atual e Próximos Passos
- **Estado Atual (2026-11-10):** Concluído, testado e homologado em produção. Sincronização automática validada em tempo real atualizando o motor upstream de `0.167.0` para `0.167.1` e regenerando o catálogo `deepseek_catalog_codex-cli_0.167.1.json`. Repositório público publicado no GitHub e integrado ao `SPEC.md` do Protocolo H2H.
- **Problemas Conhecidos:** Nenhum. O atraso de ~25s que ocorria ao carregar plugins do Codex Desktop foi 100% eliminado no modo `exec` via injeção automática de `--ignore-user-config` + `-c windows.sandbox="unelevated"`.
