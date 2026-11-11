# Resumo Técnico (GEMINI.md): CodexSeek by Jair Lima

Este documento fornece o contexto técnico completo do projeto **CodexSeek by Jair Lima** para garantir continuidade imediata por qualquer inteligência artificial ou harness do ecossistema H2H.

## 1. Visão Geral da Arquitetura
O **CodexSeek by Jair Lima** (`codexseek`) transforma o **OpenAI Codex CLI** (`@openai/codex`, licenciado sob Apache 2.0) em um agente de engenharia de software alimentado nativamente pela **API da DeepSeek** (`https://api.deepseek.com/v1`), preservando 100% da compatibilidade com todas as versões futuras do Codex CLI.

Em vez de manter um fork estático do código Rust que sofreria conflitos constantes de merge, o CodexSeek opera por **Injeção Dinâmica de Catálogo e Overlay de Configuração**:
1. **Localização Direta do Binário PE/ELF (`internal/config`):** No Windows, localiza e invoca diretamente o binário nativo `codex.exe` dentro de `@openai/codex-win32-x64/vendor/...`, evitando wrappers `.cmd` que causam vazamento de recursos em pipelines do PowerShell.
2. **Extração e Síntese de Catálogo (`internal/catalog`):** Lê em tempo real o JSON de metadados de modelos embutido dentro do próprio binário `codex.exe` instalado, clona a estrutura exata exigida pela versão corrente do Codex e gera `~/.codexseek/deepseek_catalog_<versao>.json` com os modelos da DeepSeek (`deepseek-flash`, `deepseek-v4-pro`, `deepseek-chat`, `deepseek-reasoner`), janela de 1.048.576 tokens e `apply_patch_tool_type: null` (usando Function Calling JSON padrão).
3. **Protocolo `Responses` Nativo (`internal/runner`):** Configura `wire_api = "responses"` apontando para `https://api.deepseek.com/v1`, aproveitando o suporte nativo da API da DeepSeek ao endpoint `/v1/responses` da OpenAI.
4. **Isolamento de Execução Headless:** No subcomando `exec`, injeta automaticamente `--ignore-user-config` e `-c windows.sandbox="unelevated"`, evitando que os plugins e o pipe `node_repl` do Codex Desktop atrasem a inicialização ou que `approvals_reviewer = "auto_review"` tente chamar modelos da OpenAI durante sessões da DeepSeek.
5. **Sincronização Upstream Automatizada (`internal/sync`):** O comando `codexseek sync-upstream` consulta o registro oficial do NPM (`@openai/codex/latest`), atualiza o pacote global quando há versão nova e reconstrói imediatamente o catálogo de modelos para o novo binário, registrando auditoria em `~/.codexseek/sync-upstream.log`.

## 2. Segurança de Credenciais
- O código-fonte nunca armazena chaves de API.
- A variável `DEEPSEEK_API_KEY` é resolvida em memória a partir do ambiente do sistema, de `~/.codexseek/.env` ou do cofre local `C:\Users\jairs\.api_keys\credentials.env`.
- Qualquer exibição de diagnóstico (`codexseek doctor`) aplica mascaramento estrito (`8 primeiros caracteres + "..." + 4 últimos`).

## 3. Comandos Principais
- `codexseek doctor`: Diagnóstico completo de saúde e conectividade HTTP 200 com a DeepSeek.
- `codexseek models`: Lista os modelos DeepSeek configurados no catálogo.
- `codexseek sync-upstream`: Atualiza o motor Codex CLI para a última versão e regenera o catálogo.
- `codexseek exec --skip-git-repo-check -s workspace-write -C "<dir>" "<tarefa>"`: Execução headless H2H com `deepseek-flash`.
- `codexseek --pro exec --skip-git-repo-check -s workspace-write -C "<dir>" "<tarefa>"`: Execução headless H2H com `deepseek-v4-pro`.
