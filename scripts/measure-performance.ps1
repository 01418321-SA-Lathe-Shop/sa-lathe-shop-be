# Synthetic local diagnostics only. Never takes an existing database URL.
$ErrorActionPreference = 'Stop'
Push-Location (Split-Path $PSScriptRoot -Parent)
$keys = @('DB_PASSWORD','DB_NAME','DB_USER','DB_PORT','S3_PORT','S3_BUCKET','S3_ACCESS_KEY','S3_SECRET_KEY','TEST_ADMIN_PASSWORD','TEST_S3_ENDPOINT','PERFORMANCE_DATABASE_URL','PERFORMANCE_CONFIRM_DISPOSABLE','LINE_ENABLED')
$saved = @{}
foreach ($key in $keys) { $saved[$key] = [Environment]::GetEnvironmentVariable($key,'Process') }
$emptyEnv = New-TemporaryFile
$project = 'sa-lathe-perf-' + [guid]::NewGuid().ToString('N').Substring(0,8)
$composeArgs = @('--env-file',$emptyEnv.FullName,'-p',$project)
try {
  docker info --format '{{.ServerVersion}}'
  if ($LASTEXITCODE -ne 0) { throw 'A healthy Docker engine is required.' }
  $env:DB_NAME = 'sa_lathe_shop_performance_test'
  $env:DB_USER = 'sa_lathe_shop'
  $env:DB_PASSWORD = [guid]::NewGuid().ToString('N')
  $env:DB_PORT = '0'
  $env:S3_PORT = '0'
  $env:S3_BUCKET = 'sa-lathe-shop-private'
  $env:S3_ACCESS_KEY = [guid]::NewGuid().ToString('N')
  $env:S3_SECRET_KEY = [guid]::NewGuid().ToString('N')
  $env:TEST_ADMIN_PASSWORD = [guid]::NewGuid().ToString('N')
  $env:LINE_ENABLED = 'false'
  docker compose @composeArgs up -d postgres s3
  if ($LASTEXITCODE -ne 0) { throw 'Disposable performance services failed to start.' }
  $ready = $false
  for ($attempt = 0; $attempt -lt 40; $attempt++) {
    docker compose @composeArgs exec -T postgres pg_isready -U sa_lathe_shop -d sa_lathe_shop_performance_test *> $null
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 1
  }
  if (-not $ready) { throw 'Disposable PostgreSQL was not ready.' }
  $dbAddress = (docker compose @composeArgs port postgres 5432).Trim()
  $s3Address = (docker compose @composeArgs port s3 8333).Trim()
  $env:PERFORMANCE_DATABASE_URL = 'postgres://sa_lathe_shop:' + $env:DB_PASSWORD + '@' + $dbAddress + '/sa_lathe_shop_performance_test?sslmode=disable'
  $env:TEST_S3_ENDPOINT = 'http://' + $s3Address
  $env:PERFORMANCE_CONFIRM_DISPOSABLE = 'yes'
  go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PERFORMANCE_DATABASE_URL up
  if ($LASTEXITCODE -ne 0) { throw 'Disposable performance migrations failed.' }
  $commit = (git rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0) { throw 'Cannot record baseline revision.' }
  $dockerResources = docker info --format '{{json .NCPU}} {{json .MemTotal}}'
  New-Item -ItemType Directory -Path 'bin' -Force | Out-Null
  $resultFile = Join-Path (Get-Location) ('bin/performance-' + $project + '.json')
  $testOutput = @(go test ./internal/repository/postgres -run '^TestPerformanceBaseline$' -count=1 -timeout 40m -json)
  $testExit = $LASTEXITCODE
  $events = @($testOutput | ForEach-Object { $_ | ConvertFrom-Json })
  . (Join-Path $PSScriptRoot 'Read-PerformanceReport.ps1')
  $report = Read-PerformanceReport -Events $events -ExitCode $testExit
  $report | Add-Member -NotePropertyName commit -NotePropertyValue $commit
  $report | Add-Member -NotePropertyName docker_cpu_and_memory_bytes -NotePropertyValue $dockerResources
  $report | Add-Member -NotePropertyName measured_at_utc -NotePropertyValue ([DateTime]::UtcNow.ToString('o'))
  $report | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath $resultFile -Encoding utf8
  Write-Output ('PERFORMANCE_BASELINE_PASSED: ' + $resultFile)
} finally {
  # Exact unique project created above; never remove another project's resources.
  docker compose @composeArgs down -v
  Remove-Item -LiteralPath $emptyEnv.FullName -ErrorAction SilentlyContinue
  foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key,$saved[$key],'Process') }
  Pop-Location
}
