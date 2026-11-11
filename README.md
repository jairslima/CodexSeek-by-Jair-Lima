# CodexSeek by Jair Lima

<p align="center">
  <strong>O poder do harness OpenAI Codex CLI unido nativamente aos modelos de 1M de contexto da DeepSeek API.</strong><br/>
  <em>100% Open Source • Sincronização Upstream Automática • Zero Exposição de Chaves</em>
</p>

---

## O que é o CodexSeek?

O **CodexSeek by Jair Lima** (`codexseek`) é um fork/overlay open source projetado sobre o motor do [OpenAI Codex CLI](https://github.com/openai/codex) (licenciado sob Apache 2.0), pré-configurado e otimizado para operar nativamente com a **API da DeepSeek** (`https://api.deepseek.com/v1` via protocolo `Responses`).

Com o **CodexSeek**, você mantém toda a ergonomia, velocidade em Rust, sandbox de arquivos, execução de shell e suporte a MCP do Codex CLI, mas utilizando os modelos de última geração da DeepSeek:

| Atalho / Flag | Slug Oficial | Nome do Modelo | Janela de Contexto | Modalidades | Uso Recomendado |
|---|---|---|---|---|---|
| `--flash` *(padrão)* | `deepseek-flash` | **DeepSeek V4.1 Flash** | **1.048.576 tokens** | Texto e Imagem | Execução ultrarrápida no dia a dia e automação |
| `--pro` | `deepseek-v4-pro` | **DeepSeek V4 Pro** | **1.048.576 tokens** | Texto | Raciocínio profundo, arquitetura e refatoração complexa |
| `-m chat` | `deepseek-chat` | **DeepSeek Chat** | 128.000 tokens | Texto | Uso geral e codificação leve |
| `-m r1` | `deepseek-reasoner` | **DeepSeek Reasoner** | 128.000 tokens | Texto | Cadeia de pensamento dedicada |

---

## Principais Diferenciais

1. **Sincronização Upstream Automática (`codexseek sync-upstream`):**
   - Diferente de forks estáticos que quebram a cada atualização da OpenAI, o CodexSeek utiliza **Extração Dinâmica de Catálogo (`internal/catalog`)**.
   - Sempre que a OpenAI lança uma nova versão do `@openai/codex`, o comando `codexseek sync-upstream` atualiza o motor base, extrai o novo schema diretamente do binário nativo e reconstrói automaticamente o catálogo de modelos da DeepSeek sem conflitos de código e sem perder nenhuma customização.
2. **Coexistência Limpa com o Codex Oficial:**
   - Você pode continuar usando o comando `codex` (conectado à sua conta ChatGPT/OpenAI) e o comando `codexseek` (conectado à sua conta pré-paga da DeepSeek API) lado a lado na mesma máquina, sem que um sobrescreva as configurações do outro.
3. **Zero Avisos de Metadados e Tool Calling Nativo:**
   - Registra nativamente os modelos da DeepSeek no `model_catalog_json` com janela de 1M tokens e ajusta `apply_patch_tool_type = null`, permitindo que a DeepSeek execute comandos e crie/edite arquivos na primeira tentativa em menos de 600ms.
4. **Segurança Absoluta de Credenciais:**
   - Nenhuma chave de API fica gravada no código. O `codexseek` lê a variável `DEEPSEEK_API_KEY` do ambiente ou de `~/.codexseek/.env` (ou `~/.api_keys/credentials.env`) apenas em memória no momento da execução.

---

## Pré-requisitos e Instalação

### 1. Instalar o motor base do OpenAI Codex CLI
```bash
npm install -g @openai/codex@latest
```

### 2. Configurar sua chave da DeepSeek API
Defina a variável de ambiente `DEEPSEEK_API_KEY` no seu sistema ou salve-a no arquivo local `~/.codexseek/.env`:

```bash
# Linux / macOS
export DEEPSEEK_API_KEY="<SUA_CHAVE_DEEPSEEK>"

# Windows (PowerShell)
$env:DEEPSEEK_API_KEY="<SUA_CHAVE_DEEPSEEK>"
```

Ou crie o arquivo `~/.codexseek/.env`:
```env
DEEPSEEK_API_KEY=<SUA_CHAVE_DEEPSEEK>
```

### 3. Compilar e Instalar o Binário Nativo (Go)
```bash
git clone https://github.com/jairslima/CodexSeek-by-Jair-Lima.git
cd CodexSeek-by-Jair-Lima

# Windows (instala em C:\Users\<usuario>\bin\codexseek.exe)
go build -ldflags="-s -w" -o "$HOME/bin/codexseek.exe" ./cmd/codexseek

# Linux / macOS
go build -ldflags="-s -w" -o /usr/local/bin/codexseek ./cmd/codexseek
```

---

## Como Usar

### Diagnóstico de Saúde (`doctor`)
Verifica o binário nativo do Codex, o catálogo JSON da DeepSeek, a chave mascarada e a conectividade ao vivo com a API da DeepSeek:
```bash
codexseek doctor
```

### Listar Modelos Configurados (`models`)
```bash
codexseek models
```

### Modo Interativo (TUI)
```bash
# Inicia com DeepSeek V4.1 Flash (padrão)
codexseek

# Inicia com DeepSeek V4 Pro
codexseek --pro
```

### Modo Headless Não-Interativo (`exec` / Protocolo H2H)
Ideal para automação, scripts e delegação entre agentes (Harness-to-Harness):
```bash
# Usando DeepSeek V4.1 Flash (rápido)
codexseek exec --skip-git-repo-check -s workspace-write -C "/caminho/do/projeto" "Refatore o módulo X e rode os testes"

# Usando DeepSeek V4 Pro (raciocínio pesado)
codexseek --pro exec --skip-git-repo-check -s workspace-write -C "/caminho/do/projeto" "Analise a arquitetura e corrija o bug de concorrência"
```

### Atualização Automática com o Codex Upstream (`sync-upstream`)
```bash
# Verifica se há versão nova do Codex CLI sem instalar
codexseek sync-upstream --check

# Atualiza o motor Codex CLI para a última versão e regenera o catálogo DeepSeek automaticamente
codexseek sync-upstream
```

---

## Licença e Atribuição

- **CodexSeek by Jair Lima** é distribuído sob a licença **MIT** (`Copyright (c) 2026 Jair Lima`).
- O motor base **OpenAI Codex CLI** (`@openai/codex`) é software livre da OpenAI distribuído sob a licença **Apache License 2.0**. Veja os arquivos [LICENSE](LICENSE) e [NOTICE](NOTICE) para os termos completos.
- Projeto independente criado por **Jair Lima**, sem afiliação oficial com a OpenAI ou com a DeepSeek.
