param([string]$Python = 'python')
. "$PSScriptRoot/env.ps1"
Set-Location -LiteralPath $ProjectRoot
if (-not (Test-Path -LiteralPath '.env')) {
    $content = Get-Content -LiteralPath '.env.example' -Raw -Encoding utf8
    $jwt = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
    $service = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
    $dbPassword = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24)).ToLowerInvariant()
    $content = $content.Replace('replace-with-a-different-random-jwt-secret-at-least-32-characters',$jwt).Replace('replace-with-a-random-service-token-at-least-32-characters',$service).Replace('local-development-change-me',$dbPassword)
    Set-Content -LiteralPath '.env' -Value $content -Encoding utf8
}
if (-not (Test-Path -LiteralPath 'services/ai/.venv/Scripts/python.exe')) {
    & $Python -m venv services/ai/.venv
    Assert-NativeSuccess '创建 venv'
}
Push-Location services/ai
try {
    & ./.venv/Scripts/python.exe -m pip install -r requirements-dev.lock; Assert-NativeSuccess '安装 Python 依赖'
    & ./.venv/Scripts/python.exe -m pip install --no-deps -e .; Assert-NativeSuccess '安装 AI 项目'
} finally { Pop-Location }
Push-Location services/api
try { go mod download; Assert-NativeSuccess '安装 Go 依赖' } finally { Pop-Location }
Push-Location apps/web
try { npm ci; Assert-NativeSuccess '安装前端依赖' } finally { Pop-Location }
Write-Host '准备完成。运行 ./scripts/dev.ps1，或运行 Docker Compose 全栈启动。'
