param([switch]$Demo)
. "$PSScriptRoot/env.ps1"
Import-ProjectEnv
if($Demo){$env:LLM_MODE='demo';$env:ASR_ENABLED='false';$env:TTS_ENABLED='false'}
Set-Location -LiteralPath $ProjectRoot
docker compose --env-file .env -f deployments/docker-compose.yml up -d --wait postgres redis
Assert-NativeSuccess '启动数据库与 Redis'
New-Item -ItemType Directory -Force -Path .local | Out-Null
$apiBinary = Join-Path $ProjectRoot '.local/api.exe'
Push-Location services/api
try { go build -o $apiBinary ./cmd/server; Assert-NativeSuccess '构建 Go API' } finally { Pop-Location }
$processes = @()
try {
    $processes += Start-Process -FilePath "$ProjectRoot/services/ai/.venv/Scripts/python.exe" -ArgumentList @('-m','uvicorn','emotional_agent.api.main:app','--host','127.0.0.1','--port','18001','--no-access-log') -WorkingDirectory $ProjectRoot -RedirectStandardOutput "$ProjectRoot/.local/ai.out.log" -RedirectStandardError "$ProjectRoot/.local/ai.err.log" -WindowStyle Hidden -PassThru
    $processes += Start-Process -FilePath $apiBinary -WorkingDirectory $ProjectRoot -RedirectStandardOutput "$ProjectRoot/.local/api.out.log" -RedirectStandardError "$ProjectRoot/.local/api.err.log" -WindowStyle Hidden -PassThru
    Write-Host '启动中：浏览器访问 http://localhost:5173。Ctrl+C 停止本次前台开发服务。日志在 .local/。'
    Push-Location apps/web
    try { npm run dev; Assert-NativeSuccess '启动 Web' } finally { Pop-Location }
} finally { foreach ($process in $processes) { Stop-ProjectProcessTree -ProcessId $process.Id } }
