# Pre-commit & Repository Secret Scanner — CodexSeek by Jair Lima
# Verifica que nenhum arquivo rastreado ou preparado contém chaves de API, tokens ou dados sensíveis.

$ErrorActionPreference = "Stop"
Write-Host "=== CodexSeek Security Scanner (by Jair Lima) ===" -ForegroundColor Cyan

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$forbiddenPatterns = @(
    "sk-[a-zA-Z0-9]{20,}",
    "ghp_[a-zA-Z0-9]{20,}",
    "gho_[a-zA-Z0-9]{20,}",
    "DEEPSEEK_API_KEY\s*=\s*['`"]?sk-"
)

$files = Get-ChildItem -Path $repoRoot -Recurse -File | Where-Object {
    $_.FullName -notmatch "\\\.git\\" -and
    $_.Extension -notin @(".exe", ".dll", ".so", ".dylib", ".png", ".jpg", ".ico")
}

$violations = 0
foreach ($file in $files) {
    # Skip unit test dummy string in config_test.go
    if ($file.Name -eq "config_test.go") { continue }
    $content = Get-Content -LiteralPath $file.FullName -Raw -ErrorAction SilentlyContinue
    if (-not $content) { continue }
    foreach ($pat in $forbiddenPatterns) {
        if ($content -match $pat) {
            Write-Host "[ALERTA DE SEGURANÇA] Padrão sensível detectado em: $($file.FullName)" -ForegroundColor Red
            $violations++
        }
    }
}

if ($violations -gt 0) {
    Write-Host "FALHA: $violations violação(ões) encontrada(s). Commit bloqueado." -ForegroundColor Red
    exit 1
}

Write-Host "[OK] Zero segredos ou chaves de API encontrados nos arquivos do repositório." -ForegroundColor Green
exit 0
