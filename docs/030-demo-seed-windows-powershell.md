# Manual demo seed steps (Windows / PowerShell 7)

These commands recreate a dedicated local demo database and install the complete `workflows-v3` dataset. Run them in PowerShell 7 (`pwsh`) from Windows Terminal. They delete the selected demo database and its project-local S3 files after creating backups; never use them against production or shared environments.

Requirements:

- PowerShell 7 or later (`$PSVersionTable.PSVersion.Major` must be at least `7`).
- Docker Desktop is running with Linux containers.
- Git and Go are available in `PATH`.
- The backend repository has a valid local `.env` file.

## 1. Open the backend repository and update `develop`

```powershell
$PSVersionTable.PSVersion
if ($PSVersionTable.PSVersion.Major -lt 7) {
  throw 'PowerShell 7 or later is required. Open pwsh, not Windows PowerShell 5.1.'
}

Set-Location 'C:\path\to\sa-lathe-shop-be'
git switch develop
git pull --ff-only origin develop
git status --short
```

Stop if `git status --short` shows changes you do not recognize.

## 2. Prepare and load `.env`

Create `.env` only if it does not already exist:

```powershell
if (-not (Test-Path .env)) {
  Copy-Item .env.example .env
  throw 'Created .env. Edit its PostgreSQL and S3 secrets, then run this step again.'
}
```

Load simple `KEY=VALUE` entries into the current PowerShell process. Existing process variables take precedence:

```powershell
foreach ($line in Get-Content .env) {
  if ($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
    $key = $Matches[1]
    $value = $Matches[2].Trim().Trim('"', "'")
    if (-not [Environment]::GetEnvironmentVariable($key, 'Process')) {
      [Environment]::SetEnvironmentVariable($key, $value, 'Process')
    }
  }
}

if (-not $env:DB_PASSWORD) { throw 'DB_PASSWORD is missing from .env' }
if (-not $env:DB_USER) { $env:DB_USER = 'sa_lathe_shop' }
if (-not $env:DB_PORT) { $env:DB_PORT = '55432' }
if (-not $env:API_PORT) { $env:API_PORT = '8080' }
if (-not $env:S3_PORT) { $env:S3_PORT = '19000' }

$targetDb = 'sa_lathe_shop_demo'
$env:DB_NAME = $targetDb
$env:LOCAL_TEST_ACCOUNT_ENABLE = 'true'

if ($targetDb -notin @('sa_lathe_shop_demo', 'sa_lathe_shop_seed_test')) {
  throw 'Refusing an unguarded database name'
}
if ($env:DB_USER -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
  throw 'DB_USER contains unsupported characters'
}

Write-Host "Target local database: $targetDb"
```

Use `sa_lathe_shop_seed_test` instead only when that is the intended dedicated local target.

## 3. Start local PostgreSQL and S3

```powershell
docker compose up -d --wait postgres s3
if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL or S3 failed to start' }
docker compose ps

$pgContainer = (docker compose ps -q postgres).Trim()
$s3Container = (docker compose ps -q s3).Trim()
if (-not $pgContainer -or -not $s3Container) {
  throw 'Could not resolve the local PostgreSQL/S3 containers'
}
```

## 4. Back up the existing local demo data

The backups are written under the Windows temporary directory and never into Git:

```powershell
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$backupDir = Join-Path $env:TEMP "sa-lathe-backup-$stamp"
New-Item -ItemType Directory -Path $backupDir | Out-Null

$exists = (docker compose exec -T postgres psql `
  -U $env:DB_USER -d postgres -tAc `
  "SELECT 1 FROM pg_database WHERE datname='$targetDb'").Trim()

if ($exists -eq '1') {
  $dbBackupInContainer = "/tmp/$targetDb-$stamp.dump"
  docker compose exec -T postgres pg_dump -Fc `
    -U $env:DB_USER -d $targetDb -f $dbBackupInContainer
  if ($LASTEXITCODE -ne 0) { throw 'Database backup failed' }
  docker cp "${pgContainer}:$dbBackupInContainer" `
    (Join-Path $backupDir "$targetDb.dump")
  if ($LASTEXITCODE -ne 0) { throw 'Could not copy the database backup' }
}

$s3BackupInContainer = "/tmp/s3-data-$stamp.tgz"
docker compose exec -T s3 tar -czf $s3BackupInContainer -C /data .
if ($LASTEXITCODE -ne 0) { throw 'S3 backup failed' }
docker cp "${s3Container}:$s3BackupInContainer" `
  (Join-Path $backupDir 's3-data.tgz')
if ($LASTEXITCODE -ne 0) { throw 'Could not copy the S3 backup' }

Write-Host "Backups saved to: $backupDir"
Get-ChildItem $backupDir
```

Keep the printed backup directory until the new dataset is verified. It can contain private local test data and must never be committed.

## 5. Stop the app and clear only the project S3 volume

Resolve and validate the exact Compose-owned volume before deleting its contents:

```powershell
docker compose --profile app stop api worker

$s3Image = (docker inspect $s3Container --format '{{.Config.Image}}').Trim()
$s3Volume = (docker inspect $s3Container --format `
  '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}').Trim()

Write-Host "S3 container: $s3Container"
Write-Host "S3 volume: $s3Volume"

if (-not $s3Volume.EndsWith('_s3-data')) {
  throw 'Refusing to clear an unexpected Docker volume'
}

docker compose stop s3
if ($LASTEXITCODE -ne 0) { throw 'Could not stop S3' }

docker run --rm --entrypoint sh `
  -v "${s3Volume}:/data" $s3Image `
  -c 'find /data -mindepth 1 -delete'
if ($LASTEXITCODE -ne 0) { throw 'Could not clear the validated S3 volume' }

docker compose up -d --wait s3
if ($LASTEXITCODE -ne 0) { throw 'Could not restart S3' }
```

## 6. Recreate only the guarded demo database

```powershell
if ($targetDb -notin @('sa_lathe_shop_demo', 'sa_lathe_shop_seed_test')) {
  throw 'Refusing an unguarded database name'
}

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 `
  -U $env:DB_USER -d postgres `
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$targetDb' AND pid<>pg_backend_pid()" `
  -c "DROP DATABASE IF EXISTS $targetDb" `
  -c "CREATE DATABASE $targetDb OWNER $($env:DB_USER)"
if ($LASTEXITCODE -ne 0) { throw 'Demo database recreation failed' }
```

## 7. Apply migrations through version 26

Build the guarded database URL without printing the password:

```powershell
$encodedPassword = [uri]::EscapeDataString($env:DB_PASSWORD)
$env:SEED_DATABASE_URL = `
  "postgres://$($env:DB_USER):${encodedPassword}@localhost:$($env:DB_PORT)/${targetDb}?sslmode=disable"

go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 `
  -path migrations -database $env:SEED_DATABASE_URL up
if ($LASTEXITCODE -ne 0) { throw 'Database migration failed' }
```

Copy the ER checker into PostgreSQL and execute it without requiring `psql` on Windows:

```powershell
docker cp scripts/check-er-schema.sql `
  "${pgContainer}:/tmp/check-er-schema.sql"

docker compose exec -T postgres psql -U $env:DB_USER -d $targetDb `
  -Atc 'SELECT version,dirty FROM schema_migrations'

docker compose exec -T postgres psql -U $env:DB_USER -d $targetDb `
  -f /tmp/check-er-schema.sql
```

Expected migration output is `26|f`. The ER checker must return zero missing rows.

## 8. Set guarded seed variables and private passwords

Replace each placeholder with a private password between 12 and 128 UTF-8 bytes. The values remain only in the current PowerShell process.

```powershell
$env:SEED_ENABLED = 'true'
$env:SEED_ENV = 'local'
$env:SEED_CONFIRM_DATABASE = $targetDb

$env:SEED_PIM_PASSWORD = 'replace-with-private-password-1'
$env:SEED_ANAN_PASSWORD = 'replace-with-private-password-2'
$env:SEED_KITTIPONG_PASSWORD = 'replace-with-private-password-3'
$env:SEED_NIDA_PASSWORD = 'replace-with-private-password-4'
$env:SEED_PLOY_PASSWORD = 'replace-with-private-password-5'
$env:SEED_SOMCHAI_PASSWORD = 'replace-with-private-password-6'
```

## 9. Seed the six demo accounts

```powershell
go run ./cmd/seed-demo apply --profile accounts
if ($LASTEXITCODE -ne 0) { throw 'Account seed failed' }
```

Expected result: `state` is `ready`, `accounts` is `6`, and `owned_records` is `12`.

## 10. Start API and worker on the demo database

```powershell
docker compose --profile app up -d --build --force-recreate api worker
if ($LASTEXITCODE -ne 0) { throw 'API or worker failed to start' }

$apiOrigin = "http://localhost:$($env:API_PORT)"
$s3Origin = "http://localhost:$($env:S3_PORT)"

$ready = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
  try {
    $health = Invoke-RestMethod "$apiOrigin/health/ready" -TimeoutSec 3
    if ($health.status -eq 'ok' -and $health.database -eq 'ok') {
      $ready = $true
      break
    }
  } catch {}
  Start-Sleep -Seconds 1
}
if (-not $ready) { throw 'API readiness check failed' }

Invoke-RestMethod "$apiOrigin/health/ready"
```

## 11. Seed all workflows and private files

```powershell
$env:SEED_WORKFLOWS_ENABLE = 'true'
$env:SEED_WORKFLOW_API_ORIGIN = $apiOrigin
$env:SEED_WORKFLOW_S3_ORIGIN = $s3Origin

go run ./cmd/seed-demo-workflows plan
if ($LASTEXITCODE -ne 0) { throw 'Workflow plan failed' }

go run ./cmd/seed-demo-workflows apply
if ($LASTEXITCODE -ne 0) { throw 'Workflow seed failed' }
```

Expected final result: `steps` is `235`.

## 12. Run read-only verification

Restarting the local API clears only the in-memory local-test login limiter; it does not delete data.

```powershell
docker compose restart api
if ($LASTEXITCODE -ne 0) { throw 'API restart failed' }

$ready = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
  try {
    $health = Invoke-RestMethod "$apiOrigin/health/ready" -TimeoutSec 3
    if ($health.status -eq 'ok' -and $health.database -eq 'ok') {
      $ready = $true
      break
    }
  } catch {}
  Start-Sleep -Seconds 1
}
if (-not $ready) { throw 'Restarted API readiness check failed' }

go run ./cmd/seed-demo-workflows verify
if ($LASTEXITCODE -ne 0) { throw 'Workflow verification failed' }

docker cp scripts/demo-status-matrix.sql `
  "${pgContainer}:/tmp/demo-status-matrix.sql"

docker compose exec -T postgres psql -U $env:DB_USER -d $targetDb `
  -f /tmp/check-er-schema.sql

docker compose exec -T postgres psql -U $env:DB_USER -d $targetDb `
  -f /tmp/demo-status-matrix.sql
```

Confirm the fixture and record totals:

```powershell
docker compose exec -T postgres psql -At -F '|' `
  -U $env:DB_USER -d $targetDb `
  -c 'SELECT fixture,status,count(*) FROM demo_workflow_run GROUP BY fixture,status' `
  -c 'SELECT (SELECT count(*) FROM job_request),(SELECT count(*) FROM job),(SELECT count(*) FROM material_order),(SELECT count(*) FROM invoice),(SELECT count(*) FROM payment)'
```

Expected output includes:

```text
workflows-v3|complete|1
14|8|6|4|3
```

## 13. Clear seed secrets from PowerShell

```powershell
$secretNames = @(
  'SEED_DATABASE_URL',
  'SEED_ENABLED',
  'SEED_ENV',
  'SEED_CONFIRM_DATABASE',
  'SEED_WORKFLOWS_ENABLE',
  'SEED_WORKFLOW_API_ORIGIN',
  'SEED_WORKFLOW_S3_ORIGIN',
  'SEED_PIM_PASSWORD',
  'SEED_ANAN_PASSWORD',
  'SEED_KITTIPONG_PASSWORD',
  'SEED_NIDA_PASSWORD',
  'SEED_PLOY_PASSWORD',
  'SEED_SOMCHAI_PASSWORD'
)
foreach ($name in $secretNames) {
  [Environment]::SetEnvironmentVariable($name, $null, 'Process')
}
$encodedPassword = $null
```

The application remains running with the completed local demo dataset. Use the local-test account picker in the frontend while `LOCAL_TEST_ACCOUNT_ENABLE=true`.
