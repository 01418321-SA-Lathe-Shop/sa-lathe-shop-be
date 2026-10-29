$ErrorActionPreference='Stop'
$repo=Split-Path -Parent $PSScriptRoot
$project='sa-lathe-workflows-'+[guid]::NewGuid().ToString('N').Substring(0,8)
$emptyEnv=New-TemporaryFile
$keys=@('DB_NAME','DB_USER','DB_PASSWORD','DB_PORT','API_PORT','S3_PORT','S3_ACCESS_KEY','S3_SECRET_KEY','S3_BUCKET','S3_PUBLIC_ENDPOINT','LOCAL_TEST_ACCOUNT_ENABLE','LINE_ENABLED','CORS_ORIGINS','SEED_ENABLED','SEED_ENV','SEED_DATABASE_URL','SEED_CONFIRM_DATABASE','SEED_WORKFLOWS_ENABLE','SEED_WORKFLOW_API_ORIGIN','SEED_WORKFLOW_S3_ORIGIN','SEED_PIM_PASSWORD','SEED_ANAN_PASSWORD','SEED_KITTIPONG_PASSWORD','SEED_NIDA_PASSWORD','SEED_PLOY_PASSWORD','SEED_SOMCHAI_PASSWORD')
$saved=@{};foreach($key in $keys){$saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$compose=@('--env-file',$emptyEnv.FullName,'-p',$project,'--profile','app')
Push-Location $repo
try{
 $env:DB_NAME='sa_lathe_shop_demo';$env:DB_USER='fixture';$env:DB_PASSWORD=[guid]::NewGuid().ToString('N')
 $env:DB_PORT='0';$env:API_PORT='0';$env:S3_PORT='0';$env:S3_ACCESS_KEY=[guid]::NewGuid().ToString('N');$env:S3_SECRET_KEY=[guid]::NewGuid().ToString('N');$env:S3_BUCKET='sa-lathe-shop-private'
 $env:LOCAL_TEST_ACCOUNT_ENABLE='true';$env:LINE_ENABLED='false';$env:CORS_ORIGINS='http://localhost:3000'
 docker compose @compose up -d postgres s3
 if($LASTEXITCODE -ne 0){throw 'Fresh workflow infrastructure failed'}
 $ready=$false;for($i=0;$i -lt 40;$i++){docker compose @compose exec -T postgres pg_isready -U fixture -d sa_lathe_shop_demo *> $null;if($LASTEXITCODE -eq 0){$ready=$true;break};Start-Sleep -Seconds 1}
 if(-not $ready){throw 'Fresh workflow database not ready'}
 $dbAddress=(docker compose @compose port postgres 5432).Trim()
 $s3Address=(docker compose @compose port s3 8333).Trim()
 $env:SEED_DATABASE_URL='postgres://fixture:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_demo?sslmode=disable'
 $env:SEED_ENABLED='true';$env:SEED_ENV='test';$env:SEED_CONFIRM_DATABASE='sa_lathe_shop_demo';$env:SEED_WORKFLOWS_ENABLE='true'
 $env:S3_PUBLIC_ENDPOINT='http://'+$s3Address;$env:SEED_WORKFLOW_S3_ORIGIN=$env:S3_PUBLIC_ENDPOINT
 $output=@(go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:SEED_DATABASE_URL up 2>&1)
 if($LASTEXITCODE -ne 0){throw 'Fresh workflow migrations failed (private details withheld)'}
 foreach($key in @('SEED_PIM_PASSWORD','SEED_ANAN_PASSWORD','SEED_KITTIPONG_PASSWORD','SEED_NIDA_PASSWORD','SEED_PLOY_PASSWORD','SEED_SOMCHAI_PASSWORD')){[Environment]::SetEnvironmentVariable($key,[guid]::NewGuid().ToString('N'),'Process')}
 go run ./cmd/seed-demo apply | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Fresh workflow accounts failed'}
 docker compose @compose up -d --build api worker
 if($LASTEXITCODE -ne 0){throw 'Fresh workflow API/worker build failed'}
 $apiAddress=(docker compose @compose port api 8080).Trim();$env:SEED_WORKFLOW_API_ORIGIN='http://'+$apiAddress
 $ready=$false;for($i=0;$i -lt 40;$i++){try{Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/health/ready') -TimeoutSec 3 | Out-Null;$ready=$true;break}catch{Start-Sleep -Milliseconds 500}}
 if(-not $ready){throw 'Fresh workflow API not ready'}
 go run ./cmd/seed-demo-workflows apply
 if($LASTEXITCODE -ne 0){throw 'Fresh workflow fixture failed'}
 $leaked=docker compose @compose exec -T postgres psql -U fixture -d sa_lathe_shop_demo -At -c "SELECT EXISTS(SELECT 1 FROM demo_workflow_step WHERE request ? 'test_document' OR request::text LIKE '%access_token%' OR result::text LIKE '%access_token%' OR result::text LIKE '%X-Amz-%');"
 if($LASTEXITCODE -ne 0 -or $leaked.Trim() -ne 'f'){throw 'Workflow journal contains file content or credentials'}
 # Simulate a legitimate tester stock addition through the real Admin API.
 # Completed seed reruns must preserve it, not restore the fixture baseline.
 $login=Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/auth/local-test/login') -Method Post -ContentType 'application/json' -Body '{"username":"pim.admin"}'
 $headers=@{Authorization=('Bearer '+$login.access_token)}
 $catalog=Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/materials?search=DEMO%20NBR') -Headers $headers
 $material=$catalog.items | Where-Object {$_.name -eq 'DEMO NBR'} | Select-Object -First 1
 if(-not $material){throw 'Tester progress fixture material missing'}
 $body=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$material.version;mode='addition';quantity='1';unit_cost='60';reason='TEST ONLY tester progress preservation'} | ConvertTo-Json
 $null=Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/materials/'+$material.id+'/stock') -Method Post -ContentType 'application/json' -Headers $headers -Body $body
 $before=docker compose @compose exec -T postgres psql -U fixture -d sa_lathe_shop_demo -At -c "SELECT (SELECT count(*) FROM job_request)||':'||(SELECT count(*) FROM job)||':'||(SELECT count(*) FROM demo_workflow_step)||':'||(SELECT count(*) FROM auth_session);"
 go run ./cmd/seed-demo-workflows apply
 if($LASTEXITCODE -ne 0){throw 'Workflow rerun failed'}
 $after=docker compose @compose exec -T postgres psql -U fixture -d sa_lathe_shop_demo -At -c "SELECT (SELECT count(*) FROM job_request)||':'||(SELECT count(*) FROM job)||':'||(SELECT count(*) FROM demo_workflow_step)||':'||(SELECT count(*) FROM auth_session);"
 if($before.Trim() -ne $after.Trim()){throw 'Completed rerun duplicated records or sessions'}
 $catalog=Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/materials?search=DEMO%20NBR') -Headers $headers
 $changed=$catalog.items | Where-Object {$_.id -eq $material.id} | Select-Object -First 1
 if($changed.qty_on_hand -ne '21.000000'){throw 'Completed rerun overwrote tester progress'}
 Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/auth/logout') -Method Post -Headers $headers | Out-Null
 $login=$null;$headers=$null
 # API restart only in this disposable project gives the separate verify run
 # a fresh normal login limiter; production login limits are never weakened.
 docker compose @compose restart api | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Fresh API restart failed'}
 $apiAddress=(docker compose @compose port api 8080).Trim();$env:SEED_WORKFLOW_API_ORIGIN='http://'+$apiAddress
 $ready=$false;for($i=0;$i -lt 40;$i++){try{Invoke-RestMethod ($env:SEED_WORKFLOW_API_ORIGIN+'/health/ready') -TimeoutSec 3 | Out-Null;$ready=$true;break}catch{Start-Sleep -Milliseconds 500}}
 if(-not $ready){throw 'Restarted workflow API not ready'}
 go run ./cmd/seed-demo-workflows verify
 if($LASTEXITCODE -ne 0){throw 'Workflow readback/private file/isolation verification failed'}
 Write-Output 'DEMO_WORKFLOWS_VERIFICATION_PASSED (real PostgreSQL/private S3/API/worker; exact completed rerun)'
} finally {
 # This generated project owns only fresh resources; never target the running demo.
 docker compose @compose down -v | Out-Null
 Remove-Item -LiteralPath $emptyEnv.FullName
 foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}
 Pop-Location
}
