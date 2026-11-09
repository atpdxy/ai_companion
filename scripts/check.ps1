param([switch]$Integration)
. "$PSScriptRoot/env.ps1"
Set-Location -LiteralPath $ProjectRoot
$pythonExecutable = Join-Path $ProjectRoot 'services/ai/.venv/Scripts/python.exe'
& $pythonExecutable scripts/generate_contracts.py
Assert-NativeSuccess '生成接口契约'
Push-Location services/ai
try {
    & ./.venv/Scripts/ruff.exe check src tests; Assert-NativeSuccess 'Ruff'
    & ./.venv/Scripts/ruff.exe format --check src tests; Assert-NativeSuccess 'Python 格式'
    & ./.venv/Scripts/mypy.exe src; Assert-NativeSuccess 'mypy'
    & ./.venv/Scripts/pytest.exe -q; Assert-NativeSuccess 'Python tests'
} finally { Pop-Location }
Push-Location services/api
try {
    $unformatted = gofmt -l cmd internal migrations
    if ($unformatted) { throw "Go 文件未格式化：$unformatted" }
    go vet ./...; Assert-NativeSuccess 'go vet'
    go test ./...; Assert-NativeSuccess 'Go tests'
    if (Get-Command golangci-lint -ErrorAction SilentlyContinue) { golangci-lint run; Assert-NativeSuccess 'golangci-lint' }
} finally { Pop-Location }
Push-Location apps/web
try {
    npm run lint; Assert-NativeSuccess 'ESLint'
    npm run format:check; Assert-NativeSuccess 'Prettier'
    npm run build; Assert-NativeSuccess 'Web build'
} finally { Pop-Location }
if ($Integration) {
    Import-ProjectEnv
    $env:TEST_DATABASE_URL = $env:DATABASE_URL
    Push-Location services/api
    try { go test ./internal/infrastructure -count=1; Assert-NativeSuccess 'PostgreSQL integration' } finally { Pop-Location }
    & $pythonExecutable -m pytest tests/test_e2e.py -q; Assert-NativeSuccess 'HTTP integration'
    & $pythonExecutable tests/browser_smoke.py; Assert-NativeSuccess 'Browser E2E'
    & $pythonExecutable tests/browser_companions.py; Assert-NativeSuccess 'Companion browser E2E'
    & $pythonExecutable tests/browser_voice.py; Assert-NativeSuccess 'Voice browser E2E'
}
