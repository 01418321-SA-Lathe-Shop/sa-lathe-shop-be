$ErrorActionPreference='Stop'
# Always create a fresh labelled container; never read .env or target an existing DB.
$repo=Split-Path -Parent $PSScriptRoot
$nonce=[guid]::NewGuid().ToString('N')
$container='sa-lathe-seed-'+$nonce.Substring(0,8)
$keys=@('POSTGRES_PASSWORD','SEED_TEST_DATABASE_URL','SEED_TEST_PASSWORD','SEED_TEST_CONFIRM')
$saved=@{}
foreach($key in $keys){$saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$created=$false
Push-Location $repo
try {
 $env:POSTGRES_PASSWORD=[guid]::NewGuid().ToString('N')
 $env:SEED_TEST_PASSWORD=[guid]::NewGuid().ToString('N')
 $env:SEED_TEST_CONFIRM='disposable'
 docker run -d --name $container --label "sa-lathe.seed-verification=$nonce" --env POSTGRES_PASSWORD --env POSTGRES_USER=seed --env POSTGRES_DB=sa_lathe_shop_seed_test -p '127.0.0.1::5432' postgres:17-alpine | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Disposable PostgreSQL did not start'}
 $created=$true
 $ready=$false
 for($i=0;$i -lt 40;$i++){
  docker exec $container pg_isready -U seed -d sa_lathe_shop_seed_test *> $null
  if($LASTEXITCODE -eq 0){$ready=$true;break}
  Start-Sleep -Seconds 1
 }
 if(-not $ready){throw 'Disposable PostgreSQL not ready'}
 $address=(docker port $container 5432/tcp).Trim()
 $env:SEED_TEST_DATABASE_URL='postgres://seed:'+$env:POSTGRES_PASSWORD+'@'+$address+'/sa_lathe_shop_seed_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:SEED_TEST_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Disposable migrations failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:SEED_TEST_DATABASE_URL down 1
 if($LASTEXITCODE -ne 0){throw 'Empty provenance migration rollback failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:SEED_TEST_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Provenance migration reapply failed'}
 go test ./internal/repository/postgres -run 'TestIdentityTransactionIntegration|TestDemoSeedIntegration' -count=1 -race -v
 if($LASTEXITCODE -ne 0){throw 'Real seed integration tests failed'}
 Write-Output 'DEMO_SEED_VERIFICATION_PASSED (fresh PostgreSQL; no existing data changed)'
} finally {
 if($created){
  $label=(docker inspect --format '{{ index .Config.Labels "sa-lathe.seed-verification" }}' $container).Trim()
  if($LASTEXITCODE -eq 0 -and $label -eq $nonce){docker rm -f -v $container | Out-Null}
  else {Write-Warning 'Container ownership check failed; cleanup refused'}
 }
 foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}
 Pop-Location
}
