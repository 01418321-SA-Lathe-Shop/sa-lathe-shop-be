$ErrorActionPreference='Stop'
$stagePath=Split-Path -Parent $PSScriptRoot
$savedEnvironment=@{}
foreach($key in @('DB_PASSWORD','DB_NAME','DB_USER','DB_PORT','S3_PORT','S3_BUCKET','S3_ACCESS_KEY','S3_SECRET_KEY','TEST_ADMIN_PASSWORD','TEST_WORKER_PASSWORD','PHASE4_DATABASE_URL','TEST_DATABASE_URL','TEST_S3_ENDPOINT','S3_PUBLIC_ENDPOINT','API_PORT','LINE_ENABLED')){$savedEnvironment[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$emptyEnv=New-TemporaryFile
$projectName='sa-lathe-uc1-'+[guid]::NewGuid().ToString('N').Substring(0,8)
$env:DB_PASSWORD=[guid]::NewGuid().ToString('N')
$env:DB_NAME='sa_lathe_shop_phase4_test'
$env:DB_USER='sa_lathe_shop'
$env:DB_PORT='0'
$env:S3_PORT='0'
$env:S3_BUCKET='sa-lathe-shop-private'
$composeArgs=@('--env-file',$emptyEnv.FullName,'-p',$projectName)
$env:S3_ACCESS_KEY=[guid]::NewGuid().ToString('N')
$env:S3_SECRET_KEY=[guid]::NewGuid().ToString('N')
$env:TEST_ADMIN_PASSWORD=[guid]::NewGuid().ToString('N')
$env:TEST_WORKER_PASSWORD=[guid]::NewGuid().ToString('N')
Push-Location $stagePath
try {
 docker compose @composeArgs up -d postgres s3
 if($LASTEXITCODE -ne 0){throw 'Test services failed to start'}
 for($i=0;$i -lt 40;$i++){
  docker compose @composeArgs exec -T postgres pg_isready -U sa_lathe_shop -d sa_lathe_shop_phase4_test *> $null
  if($LASTEXITCODE -eq 0){break};Start-Sleep -Seconds 1
 }
 $dbAddress=(docker compose @composeArgs port postgres 5432).Trim()
 $s3Address=(docker compose @composeArgs port s3 8333).Trim()
 $env:PHASE4_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase4_test?sslmode=disable'
 $env:TEST_S3_ENDPOINT='http://'+$s3Address
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Phase 4 migrations failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL down 1
 if($LASTEXITCODE -ne 0){throw 'Empty migration rollback failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Migration reapply failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase3_test;'
 if($LASTEXITCODE -ne 0){throw 'Staff regression DB creation failed'}
 $env:TEST_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase3_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:TEST_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Staff regression migration failed'}
 go test ./... -count=1
 if($LASTEXITCODE -ne 0){throw 'Real PostgreSQL/S3 integration failed'}
 $env:S3_PUBLIC_ENDPOINT=$env:TEST_S3_ENDPOINT
 $env:API_PORT='0'
 $env:LINE_ENABLED='false'
 docker compose @composeArgs --profile app build api worker
 if($LASTEXITCODE -ne 0){throw 'API/worker Docker build failed'}
 docker compose @composeArgs --profile app up -d api worker
 if($LASTEXITCODE -ne 0){throw 'API/worker failed to start'}
 $apiAddress='http://'+(docker compose @composeArgs port api 8080).Trim()
 $ready=$false
 for($i=0;$i -lt 30;$i++){
  try{$health=Invoke-RestMethod -Uri ($apiAddress+'/health/ready');$ready=$true;break}catch{Start-Sleep -Seconds 1}
 }
 if(-not $ready){throw 'Docker API readiness failed'}
 try {
  $registration=@{username='docker_customer';password=$env:TEST_ADMIN_PASSWORD;name='Docker Customer';customer_type='individual'}|ConvertTo-Json
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/auth/register') -ContentType 'application/json' -Body $registration
  $loginBody=@{username='docker_customer';password=$env:TEST_ADMIN_PASSWORD}|ConvertTo-Json
  $login=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/auth/login') -ContentType 'application/json' -Body $loginBody
  $headers=@{Authorization='Bearer '+$login.access_token}
  $profiles=Invoke-RestMethod -Uri ($apiAddress+'/customers') -Headers $headers
  $requestBody=@{customer_id=$profiles.items[0].id;description='Docker verification request';qty=1;material_spec='S45C'}|ConvertTo-Json
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/requests') -Headers $headers -ContentType 'application/json' -Body $requestBody
  $delivered=$false
  for($i=0;$i -lt 15;$i++){
   $inbox=Invoke-RestMethod -Uri ($apiAddress+'/notifications') -Headers $headers
   if($inbox.items.Count -ge 1){$delivered=$true;break};Start-Sleep -Seconds 1
  }
  if(-not $delivered){throw 'Worker did not deliver the Docker request notification'}
 } catch {throw 'Docker customer/request/worker flow failed; credentials withheld'}
 docker compose @composeArgs stop api worker
 if($LASTEXITCODE -ne 0){throw 'Graceful container shutdown failed'}
 Write-Output 'PHASE4_VERIFICATION_PASSED'
} finally {
 # This unique Compose project and its fresh test volumes are the exact targets.
 docker compose @composeArgs down -v
 Pop-Location
 Remove-Item -LiteralPath $emptyEnv.FullName
 foreach($key in $savedEnvironment.Keys){[Environment]::SetEnvironmentVariable($key,$savedEnvironment[$key],'Process')}
}
