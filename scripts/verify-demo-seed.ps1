$ErrorActionPreference='Stop'
# Always create a fresh labelled container; never read .env or target an existing DB.
$repo=Split-Path -Parent $PSScriptRoot
$nonce=[guid]::NewGuid().ToString('N')
$container='sa-lathe-seed-'+$nonce.Substring(0,8)
$keys=@('POSTGRES_PASSWORD','SEED_TEST_DATABASE_URL','SEED_TEST_PASSWORD','SEED_TEST_CONFIRM','SEED_ENABLED','SEED_ENV','SEED_DATABASE_URL','SEED_CONFIRM_DATABASE','SEED_API_BASE_URL','SEED_PIM_PASSWORD','SEED_ANAN_PASSWORD','SEED_KITTIPONG_PASSWORD','SEED_NIDA_PASSWORD','SEED_PLOY_PASSWORD','SEED_SOMCHAI_PASSWORD','DATABASE_URL','HTTP_ADDR','CORS_ORIGINS','LINE_ENABLED')
$saved=@{}
$keys += 'LOCAL_TEST_ACCOUNT_ENABLE'
foreach($key in $keys){$saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$created=$false
$apiProcess=$null
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
 # Identity rollback must run before the account dataset is committed. Separate
 # invocations make the fresh-DB contract explicit, not Go test file-order dependent.
 foreach($suite in @('TestIdentityTransactionIntegration','TestDemoSeedIntegration')){
  $lines=@(go test ./internal/repository/postgres -run "^$suite`$" -count=1 -race -json)
  $testExit=$LASTEXITCODE
  $events=@($lines | ForEach-Object {$_ | ConvertFrom-Json})
  $pass=@($events | Where-Object {$_.Test -eq $suite -and $_.Action -eq 'pass'})
  $skip=@($events | Where-Object {$_.Test -eq $suite -and $_.Action -eq 'skip'})
  if($testExit -ne 0 -or $pass.Count -ne 1 -or $skip.Count -ne 0){
   $events | Where-Object {$_.Action -eq 'output'} | ForEach-Object {$_.Output.TrimEnd()}
   throw "Required real suite $suite failed or did not execute"
  }
  Write-Output "$suite passed (real PostgreSQL, race enabled)"
 }
 # A second fresh DB verifies the actual CLI + independent API process, not
 # just direct services or httptest. No private objects/worker are needed for
 # account-only reads; no storage workflow verification is claimed here.
 docker exec $container psql -v ON_ERROR_STOP=1 -U seed -d sa_lathe_shop_seed_test -c 'CREATE DATABASE sa_lathe_shop_demo;' | Out-Null
 if($LASTEXITCODE -ne 0){throw 'CLI demo test DB creation failed'}
 $env:SEED_DATABASE_URL='postgres://seed:'+$env:POSTGRES_PASSWORD+'@'+$address+'/sa_lathe_shop_demo?sslmode=disable'
 $env:SEED_ENABLED='true';$env:SEED_ENV='test';$env:SEED_CONFIRM_DATABASE='sa_lathe_shop_demo'
 foreach($key in @('SEED_PIM_PASSWORD','SEED_ANAN_PASSWORD','SEED_KITTIPONG_PASSWORD','SEED_NIDA_PASSWORD','SEED_PLOY_PASSWORD','SEED_SOMCHAI_PASSWORD')){[Environment]::SetEnvironmentVariable($key,$env:SEED_TEST_PASSWORD,'Process')}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:SEED_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'CLI demo migrations failed'}
 ./scripts/seed-demo.ps1 plan | Out-Null
 ./scripts/seed-demo.ps1 apply | Out-Null
 ./scripts/seed-demo.ps1 apply | Out-Null
 $probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
 $probe.Start();$port=$probe.LocalEndpoint.Port;$probe.Stop()
 $env:DATABASE_URL=$env:SEED_DATABASE_URL;$env:HTTP_ADDR="127.0.0.1:$port";$env:CORS_ORIGINS='http://localhost:3000';$env:LINE_ENABLED='false'
 $env:SEED_API_BASE_URL="http://127.0.0.1:$port"
 $env:LOCAL_TEST_ACCOUNT_ENABLE='true'
 New-Item -ItemType Directory -Path bin -Force | Out-Null
 $suffix=if($IsWindows){'.exe'}else{''}
 $binary=Join-Path $repo "bin/seed-api-$nonce$suffix"
 go build -o $binary ./cmd/api
 if($LASTEXITCODE -ne 0){throw 'Verification API build failed'}
 $start=@{FilePath=$binary;PassThru=$true;RedirectStandardOutput=(Join-Path $repo "bin/seed-api-$nonce.out.log");RedirectStandardError=(Join-Path $repo "bin/seed-api-$nonce.err.log")}
 if($IsWindows){$start.WindowStyle='Hidden'}
 $apiProcess=Start-Process @start
 $ready=$false
 for($i=0;$i -lt 40;$i++){
  if($apiProcess.HasExited){throw 'Verification API exited; safe local logs retained in bin'}
  try{Invoke-RestMethod "$env:SEED_API_BASE_URL/health/ready" -TimeoutSec 2 | Out-Null;$ready=$true;break}catch{Start-Sleep -Milliseconds 250}
 }
 if(-not $ready){throw 'Verification API readiness failed'}
 $cors=Invoke-WebRequest "$env:SEED_API_BASE_URL/auth/login" -Method Options -Headers @{Origin='http://localhost:3000';'Access-Control-Request-Method'='POST';'Access-Control-Request-Headers'='content-type'}
 # Existing chi/cors returns 200 for preflight, not 204. Assert its actual
 # contract and allowed origin/method/header; do not change API behavior.
 $allowedOrigin=@($cors.Headers['Access-Control-Allow-Origin']) -join ','
 $allowedMethods=@($cors.Headers['Access-Control-Allow-Methods']) -join ','
 $allowedHeaders=@($cors.Headers['Access-Control-Allow-Headers']) -join ','
 if($cors.StatusCode -ne 200 -or $allowedOrigin -ne 'http://localhost:3000' -or $allowedMethods -notmatch 'POST' -or $allowedHeaders -notmatch 'Content-Type'){throw 'Real API CORS preflight failed'}
 go run ./cmd/seed-demo verify
 if($LASTEXITCODE -ne 0){throw 'Real CLI/API login and permission verification failed'}
 # Password-free login must produce the same DB-backed, revocable sessions.
 # Never print bearer tokens or HTTP bodies containing them.
 foreach($username in @('pim.admin','anan.staff','kittipong.staff','nida.company','ploy.company','somchai')){
  $body=@{username=$username} | ConvertTo-Json -Compress
  $login=Invoke-RestMethod "$env:SEED_API_BASE_URL/auth/local-test/login" -Method Post -ContentType 'application/json' -Body $body
  if($login.account.username -ne $username -or $login.token_type -ne 'Bearer' -or -not $login.access_token){throw 'Password-free login returned wrong identity/session'}
  $headers=@{Authorization=('Bearer '+$login.access_token)}
  $me=Invoke-RestMethod "$env:SEED_API_BASE_URL/auth/me" -Headers $headers
  if($me.id -ne $login.account.id){throw 'Password-free session is not backed by real authentication'}
  $gate=Invoke-WebRequest "$env:SEED_API_BASE_URL/admin/workers" -Headers $headers -SkipHttpErrorCheck
  $expected=if($username -eq 'pim.admin'){200}else{403}
  if($gate.StatusCode -ne $expected){throw 'Password-free login bypassed role checks'}
  Invoke-RestMethod "$env:SEED_API_BASE_URL/auth/logout" -Method Post -Headers $headers | Out-Null
  $revoked=Invoke-WebRequest "$env:SEED_API_BASE_URL/auth/me" -Headers $headers -SkipHttpErrorCheck
  if($revoked.StatusCode -ne 401){throw 'Password-free session could not be revoked'}
  $login=$null;$headers=$null
 }
 # Restart with flag OFF: endpoint must be absent, not merely hidden in UI.
 Stop-Process -Id $apiProcess.Id;$apiProcess.WaitForExit(5000) | Out-Null
 $env:LOCAL_TEST_ACCOUNT_ENABLE='false'
 $start.RedirectStandardOutput=Join-Path $repo "bin/seed-api-$nonce-disabled.out.log"
 $start.RedirectStandardError=Join-Path $repo "bin/seed-api-$nonce-disabled.err.log"
 $apiProcess=Start-Process @start
 $ready=$false
 for($i=0;$i -lt 40;$i++){
  if($apiProcess.HasExited){throw 'Disabled-mode API exited'}
  try{Invoke-RestMethod "$env:SEED_API_BASE_URL/health/ready" -TimeoutSec 2 | Out-Null;$ready=$true;break}catch{Start-Sleep -Milliseconds 250}
 }
 if(-not $ready){throw 'Disabled-mode API not ready'}
 $disabled=Invoke-WebRequest "$env:SEED_API_BASE_URL/auth/local-test/login" -Method Post -ContentType 'application/json' -Body '{"username":"pim.admin"}' -SkipHttpErrorCheck
 if($disabled.StatusCode -ne 404){throw 'Password-free endpoint exists with flag disabled'}
 Write-Output 'LOCAL_TEST_ACCOUNT_VERIFICATION_PASSED (six real logins, role gates, revocation, disabled endpoint)'
 Write-Output 'CLI plan/apply/rerun and independent real API login/CORS verification passed'
 Write-Output 'DEMO_SEED_VERIFICATION_PASSED (fresh PostgreSQL; no existing data changed)'
} finally {
 if($apiProcess -and -not $apiProcess.HasExited){Stop-Process -Id $apiProcess.Id -ErrorAction SilentlyContinue;$apiProcess.WaitForExit(5000) | Out-Null}
 if($created){
  $label=(docker inspect --format '{{ index .Config.Labels "sa-lathe.seed-verification" }}' $container).Trim()
  if($LASTEXITCODE -eq 0 -and $label -eq $nonce){docker rm -f -v $container | Out-Null}
  else {Write-Warning 'Container ownership check failed; cleanup refused'}
 }
 foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}
 Pop-Location
}
