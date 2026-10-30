$ErrorActionPreference='Stop'
# Local demo only: rebuild Docker with .env, migrate + seed sa_lathe_shop_demo, start API with demo login.
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
 if(-not (Test-Path .env)){throw 'Create .env first (copy .env.example, set DB_PASSWORD/S3_ACCESS_KEY/S3_SECRET_KEY)'}
 # Compose reads .env itself; load it here too for migrate/seed. Existing process values win.
 foreach($line in Get-Content .env){
  if($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$' -and -not [Environment]::GetEnvironmentVariable($Matches[1],'Process')){
   [Environment]::SetEnvironmentVariable($Matches[1],$Matches[2].Trim().Trim('"',"'"),'Process')
  }
 }
 $demo='sa_lathe_shop_demo'
 $dbUser=if($env:DB_USER){$env:DB_USER}else{'sa_lathe_shop'}
 $dbPort=if($env:DB_PORT){$env:DB_PORT}else{'55432'}
 # Dedicated demo DB + demo login; compose picks these up from the process environment.
 $env:DB_NAME=$demo; $env:LOCAL_TEST_ACCOUNT_ENABLE='true'

 docker compose up -d --wait postgres s3
 if($LASTEXITCODE){throw 'postgres/s3 failed to start'}
 $exists=docker compose exec -T postgres psql -U $dbUser -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$demo'"
 if(-not $exists){docker compose exec -T postgres psql -U $dbUser -d postgres -c "CREATE DATABASE $demo"; if($LASTEXITCODE){throw 'create db failed'}}

 ./tasks.ps1 migrate-up   # uses DB_NAME=demo from process env
 if($LASTEXITCODE){throw 'migrate failed'}

 $env:SEED_ENABLED='true'; $env:SEED_ENV='local'; $env:SEED_CONFIRM_DATABASE=$demo
 $env:SEED_DATABASE_URL='postgres://'+[uri]::EscapeDataString($dbUser)+':'+[uri]::EscapeDataString($env:DB_PASSWORD)+"@localhost:${dbPort}/${demo}?sslmode=disable"
 ./scripts/seed-demo.ps1 -Action apply   # prompts for passwords on first run only
 ./scripts/seed-demo.ps1 -Action status

 docker compose --profile app up --build -d --force-recreate api worker
 if($LASTEXITCODE){throw 'app start failed'}
 Write-Host "Done: API http://localhost:$(if($env:API_PORT){$env:API_PORT}else{'8080'}) using $demo"
} finally {
 Remove-Item Env:SEED_DATABASE_URL -ErrorAction SilentlyContinue
 Pop-Location
}
