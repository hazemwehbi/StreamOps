$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
if (Test-Path -LiteralPath '.env') { throw '.env existe déjà ; conservez-le ou déplacez-le avant de générer de nouveaux identifiants.' }
function New-Token {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
}
$values = @{}
foreach ($key in @('POSTGRES_PASSWORD','APP_DB_PASSWORD','ADMIN_TOKEN','READ_TOKEN','METRICS_TOKEN')) { $values[$key] = New-Token }
$lines = foreach ($key in @('POSTGRES_PASSWORD','APP_DB_PASSWORD','ADMIN_TOKEN','READ_TOKEN','METRICS_TOKEN')) { "$key=$($values[$key])" }
$lines += @('HTTP_PORT=8080','DB_POOL_MAX=10','RATE_LIMIT_RPS=100','RATE_LIMIT_BURST=200','CORS_ORIGIN=','OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=','DATABASE_URL=')
[IO.File]::WriteAllLines((Join-Path $PWD '.env'), $lines, [Text.UTF8Encoding]::new($false))
New-Item -ItemType Directory -Force '.secrets' | Out-Null
[IO.File]::WriteAllText((Join-Path $PWD '.secrets/metrics_token'), $values['METRICS_TOKEN'], [Text.UTF8Encoding]::new($false))
Write-Host 'Fichiers .env et .secrets/metrics_token créés. Conservez-les confidentiels. Démarrez avec docker compose up --build -d.'
