# Especificação Técnica (SPEC.md): CodexSeek by Jair Lima

**Projeto:** CodexSeek by Jair Lima  
**Versão:** 1.0.0  
**Autor:** Jair Lima  
**Licença:** MIT (código próprio do CodexSeek) e Apache 2.0 (atribuição ao motor upstream OpenAI Codex CLI)  
**Repositório:** `https://github.com/jairslima/CodexSeek-by-Jair-Lima` (Público por solicitação explícita do autor)

---

## 1. Objetivo

Construir e manter um fork/overlay nativo e público chamado **CodexSeek by Jair Lima** (`codexseek`), que utiliza a infraestrutura completa de agente local do **OpenAI Codex CLI** (sandbox, execução de shell, MCP, sessões, TUI e modo headless `exec`) conectada de forma nativa e pré-configurada à **API da DeepSeek** (`deepseek-flash` / DeepSeek V4.1 Flash e `deepseek-v4-pro` / DeepSeek V4 Pro, com janela de contexto de 1.048.576 tokens), sendo capaz de incorporar todas as versões futuras do Codex CLI automaticamente sem perder suas customizações e sem jamais expor a chave de API do autor.

---

## 2. Escopo

### Dentro do Escopo
1. **Binário Nativo Global (`codexseek.exe`):** Compilado em Go e instalado em `C:\Users\jairs\bin\codexseek.exe` (sem depender de wrappers `.cmd` intermediários).
2. **Pacote NPM Multiplataforma (`npm/`):** Launcher ESM em Node.js para usuários em Windows, Linux e macOS.
3. **Gerador Automático de Catálogo DeepSeek (`internal/catalog`):** Extração dinâmica do schema JSON embutido no binário do `codex` e síntese automática de `~/.codexseek/deepseek_catalog_<versao>.json` com:
   - `deepseek-flash` (DeepSeek V4.1 Flash, 1M tokens, texto e imagem, padrão)
   - `deepseek-v4-pro` (DeepSeek V4 Pro, 1M tokens, raciocínio profundo)
   - `deepseek-chat` (DeepSeek Chat, 128k tokens)
   - `deepseek-reasoner` (DeepSeek Reasoner, 128k tokens)
   - `apply_patch_tool_type = null` e `tool_mode = null` (garantindo chamadas de função JSON universais em vez da gramática `freeform` proprietária da OpenAI).
4. **Resolução Segura de Credenciais (`internal/config`):** Leitura em memória da variável `DEEPSEEK_API_KEY` a partir do ambiente, `~/.codexseek/.env` ou `~/.api_keys/credentials.env`, com mascaramento obrigatório (`8 primeiros caracteres + "..." + 4 últimos`) em qualquer diagnóstico.
5. **Sincronização Upstream Automatizada (`codexseek sync-upstream`):** Verificação e atualização automática do pacote `@openai/codex@latest` via NPM e reconstrução imediata do catálogo de modelos compatível com a versão recém-instalada, além de workflow diário no GitHub Actions (`.github/workflows/sync-upstream.yml`).
6. **Integração ao Protocolo H2H:** Manifesto oficial `codexseek` documentado para delegação síncrona entre harnesses.

### Fora do Escopo
- Modificar ou sobrescrever o arquivo `~/.codex/config.toml` usado pelo `codex` original da OpenAI (o `codex` e o `codexseek` coexistem na mesma máquina sem interferência mútua).
- Armazenar chaves de API dentro do repositório Git.

---

## 3. Arquitetura

```
CodexSeek-by-Jair-Lima/
├── cmd/
│   └── codexseek/
│       └── main.go               # Ponto de entrada CLI (flags --pro, --flash, doctor, models, sync-upstream)
├── internal/
│   ├── catalog/
│   │   └── catalog.go            # Extrator do schema binário do Codex e gerador do catálogo DeepSeek 1M
│   ├── config/
│   │   ├── config.go             # Resolução de DEEPSEEK_API_KEY, mascaramento e localização do codex.exe nativo
│   │   └── config_test.go        # Testes unitários de segurança de chave e aliases
│   ├── runner/
│   │   ├── runner.go             # Injeção do overlay DeepSeek (/v1/responses) e isolamento de execução
│   │   └── runner_test.go        # Testes unitários de transformação de argumentos
│   └── sync/
│       └── sync.go               # Motor de atualização automática (@openai/codex@latest) e diagnóstico doctor
├── npm/
│   ├── package.json              # Manifesto NPM multiplataforma
│   └── bin/
│       └── codexseek.mjs         # Launcher Node.js ESM
├── scripts/
│   └── pre-commit-secret-scan.ps1 # Scanner de proteção contra vazamento de chaves
├── .github/
│   └── workflows/
│       ├── ci.yml                # Testes, scanner de segredos e compilação Linux/Windows/macOS
│       └── sync-upstream.yml     # Monitoramento e sincronização diária com @openai/codex
├── go.mod
├── LICENSE                       # MIT (Jair Lima) + Notice Apache 2.0 (OpenAI)
├── NOTICE
├── SPEC.md
├── PROJECT.md
├── GEMINI.md
└── README.md
```

---

## 4. Decisões Técnicas Fundamentais

1. **Descoberta do Suporte Nativo a `/v1/responses` na API DeepSeek:**
   - Versões recentes do Codex CLI (`>= 0.160.0`) descontinuaram `wire_api = "chat"` e exigem `wire_api = "responses"`.
   - Validamos empiricamente que `https://api.deepseek.com/v1/responses` responde nativamente ao protocolo `Responses` da OpenAI com `HTTP 200 OK`, incluindo blocos de raciocínio (`reasoning_text`) e chamadas de ferramentas (`FunctionCall`).
2. **Eliminação de Avisos de Metadados via `model_catalog_json` Dinâmico:**
   - Em vez de sofrer com `warning: Model metadata for deepseek-flash not found`, o módulo `internal/catalog` lê o catálogo JSON diretamente do binário `codex.exe` instalado, clona a estrutura exata esperada pela versão atual do Rust e registra os modelos da DeepSeek com `context_window = 1048576`.
3. **Execução Headless Rápida (`--ignore-user-config` + `windows.sandbox="unelevated"`):**
   - O `~/.codex/config.toml` do Codex Desktop carrega dezenas de plugins e um pipe `node_repl` com timeout de 120s, além de `approvals_reviewer = "auto_review"`. No modo `codexseek exec`, o runner injeta `--ignore-user-config` e `-c windows.sandbox="unelevated"`, permitindo escrita em `workspace-write` em menos de 1 segundo por comando sem tocar na configuração do Codex Desktop.

---

## 5. Critérios de Aceite

- [x] `codexseek --version` exibe a versão do CodexSeek by Jair Lima e a versão do motor upstream Codex CLI.
- [x] `codexseek doctor` valida binário nativo, catálogo JSON, chave mascarada e conectividade HTTP 200 com a API DeepSeek.
- [x] `codexseek sync-upstream` atualiza automaticamente o motor `@openai/codex` quando há versão nova no NPM e regenera o catálogo DeepSeek sem intervenção manual.
- [x] `codexseek --flash exec ...` e `codexseek --pro exec ...` executam tarefas reais de leitura e escrita de arquivos sem erros e sem avisos de metadados.
- [x] Repositório público no GitHub com licença MIT + Apache 2.0 Notice e zero chaves ou dados sensíveis no histórico Git.

---

## 6. Instruções Operacionais

```powershell
# Diagnóstico completo da instalação
codexseek doctor

# Listar modelos DeepSeek disponíveis
codexseek models

# Executar tarefa rápida (DeepSeek V4.1 Flash - padrão)
codexseek exec --skip-git-repo-check -s workspace-write -C "C:\caminho\projeto" "Sua tarefa"

# Executar tarefa de raciocínio profundo (DeepSeek V4 Pro)
codexseek --pro exec --skip-git-repo-check -s workspace-write -C "C:\caminho\projeto" "Sua tarefa complexa"

# Sincronizar com a versão mais nova do OpenAI Codex CLI
codexseek sync-upstream
```
