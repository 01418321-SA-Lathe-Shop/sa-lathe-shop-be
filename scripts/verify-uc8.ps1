$ErrorActionPreference='Stop'
# Fail promptly if Docker is stopped or stuck; do not touch test data first.
$probeOut=New-TemporaryFile
$probeErr=New-TemporaryFile
try {
 $probe=Start-Process -FilePath (Get-Command docker).Source -ArgumentList @('info','--format','{{.ServerVersion}}') -WindowStyle Hidden -PassThru -RedirectStandardOutput $probeOut.FullName -RedirectStandardError $probeErr.FullName
 if(-not $probe.WaitForExit(15000)){Stop-Process -Id $probe.Id -ErrorAction SilentlyContinue;throw 'Docker engine unavailable or stuck. Start a healthy Docker Desktop engine before verification.'}
 if($probe.ExitCode -ne 0){throw 'Docker engine unavailable. Start Docker Desktop before verification.'}
} finally {Remove-Item -LiteralPath $probeOut.FullName,$probeErr.FullName -ErrorAction SilentlyContinue}
$stagePath=Split-Path -Parent $PSScriptRoot
$savedEnvironment=@{}
foreach($key in @('DB_PASSWORD','DB_NAME','DB_USER','DB_PORT','S3_PORT','S3_BUCKET','S3_ACCESS_KEY','S3_SECRET_KEY','TEST_ADMIN_PASSWORD','TEST_WORKER_PASSWORD','PHASE4_DATABASE_URL','PHASE5_DATABASE_URL','PHASE6_DATABASE_URL','PHASE7_DATABASE_URL','PHASE8_DATABASE_URL','PHASE9_DATABASE_URL','PHASE10_DATABASE_URL','PHASE11_DATABASE_URL','TEST_DATABASE_URL','TEST_S3_ENDPOINT','S3_PUBLIC_ENDPOINT','API_PORT','LINE_ENABLED')){$savedEnvironment[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$emptyEnv=New-TemporaryFile
$projectName='sa-lathe-uc8-'+[guid]::NewGuid().ToString('N').Substring(0,8)
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
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL up 5
 if($LASTEXITCODE -ne 0){throw 'Pre-contact migrations failed'}
 # Seed only a legacy profile in this newly-created test DB, never a real account.
 docker compose @composeArgs exec -T postgres psql -v ON_ERROR_STOP=1 -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c "INSERT INTO person(id,name) VALUES('00000000-0000-0000-0000-000000000006','Legacy fixture'); INSERT INTO worker(person_id,employee_no) VALUES('00000000-0000-0000-0000-000000000006','LEGACY-6');"
 if($LASTEXITCODE -ne 0){throw 'Legacy fixture failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Phase 4 migrations failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL down 2
 if($LASTEXITCODE -ne 0){throw 'Empty migration rollback failed'}
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE4_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Migration reapply failed'}
 $legacyProbe=@'
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM person JOIN worker ON worker.person_id=person.id WHERE person.id='00000000-0000-0000-0000-000000000006' AND email IS NULL AND phone IS NULL) THEN
  RAISE EXCEPTION 'Legacy contacts were changed or deleted';
 END IF;
END $$;
UPDATE worker SET position='Legacy retained' WHERE person_id='00000000-0000-0000-0000-000000000006';
UPDATE person SET email='legacy@example.com',phone='+66812345678' WHERE id='00000000-0000-0000-0000-000000000006';
DELETE FROM worker WHERE person_id='00000000-0000-0000-0000-000000000006';
DELETE FROM person WHERE id='00000000-0000-0000-0000-000000000006';
'@
 $legacyProbe | docker compose @composeArgs exec -T postgres psql -v ON_ERROR_STOP=1 -U sa_lathe_shop -d sa_lathe_shop_phase4_test
 if($LASTEXITCODE -ne 0){throw 'Legacy Worker preservation/backfill probe failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase3_test;'
 if($LASTEXITCODE -ne 0){throw 'Staff regression DB creation failed'}
 $env:TEST_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase3_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:TEST_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Staff regression migration failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase5_test;'
 if($LASTEXITCODE -ne 0){throw 'Quotation test DB creation failed'}
 $env:PHASE5_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase5_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE5_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Quotation migrations failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase6_test;'
 if($LASTEXITCODE -ne 0){throw 'Decision test DB creation failed'}
 $env:PHASE6_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase6_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE6_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'Decision migrations failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase7_test;'
 if($LASTEXITCODE -ne 0){throw 'UC4 DB creation failed'}
 $env:PHASE7_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase7_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE7_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'UC4 migrations failed'}
 docker compose @composeArgs exec -T postgres psql -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase8_test;'
 if($LASTEXITCODE -ne 0){throw 'UC5 DB creation failed'}
 $env:PHASE8_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase8_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE8_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'UC5 migrations failed'}
 docker compose @composeArgs exec -T postgres psql -v ON_ERROR_STOP=1 -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase9_test;'
 if($LASTEXITCODE -ne 0){throw 'UC6 DB creation failed'}
 $env:PHASE9_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase9_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE9_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'UC6 migrations failed'}
 docker compose @composeArgs exec -T postgres psql -v ON_ERROR_STOP=1 -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase10_test;'
 if($LASTEXITCODE -ne 0){throw 'UC7 DB creation failed'}
 $env:PHASE10_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase10_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE10_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'UC7 migrations failed'}
 docker compose @composeArgs exec -T postgres psql -v ON_ERROR_STOP=1 -U sa_lathe_shop -d sa_lathe_shop_phase4_test -c 'CREATE DATABASE sa_lathe_shop_phase11_test;'
 if($LASTEXITCODE -ne 0){throw 'UC8 DB creation failed'}
 $env:PHASE11_DATABASE_URL='postgres://sa_lathe_shop:'+$env:DB_PASSWORD+'@'+$dbAddress+'/sa_lathe_shop_phase11_test?sslmode=disable'
 go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database $env:PHASE11_DATABASE_URL up
 if($LASTEXITCODE -ne 0){throw 'UC8 migrations failed'}
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
  $newRequest=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/requests') -Headers $headers -ContentType 'application/json' -Body $requestBody
  $workerLoginBody=@{username='uc1_worker';password=$env:TEST_ADMIN_PASSWORD}|ConvertTo-Json
  $workerLogin=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/auth/login') -ContentType 'application/json' -Body $workerLoginBody
  $workerHeaders=@{Authorization='Bearer '+$workerLogin.access_token}
  # Existing auth regression creates this Admin and Worker in the disposable DB.
  $adminBody=@{username='uc1_admin';password=$env:TEST_ADMIN_PASSWORD}|ConvertTo-Json
  $adminLogin=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/auth/login') -ContentType 'application/json' -Body $adminBody
  $adminHeaders=@{Authorization='Bearer '+$adminLogin.access_token}
  $workers=Invoke-RestMethod -Uri ($apiAddress+'/admin/workers') -Headers $adminHeaders
  $contactWorker=$workers.items|Where-Object username -eq 'uc1_worker'|Select-Object -First 1
  if($contactWorker.email -ne 'worker@example.com' -or $contactWorker.phone -ne '+66812345678'){throw 'Docker Worker contact read failed'}
  $contactBody=@{email='updated.worker@example.com';phone='+1 (202) 555-0123'}|ConvertTo-Json
  $updatedWorker=Invoke-RestMethod -Method Patch -Uri ($apiAddress+'/admin/workers/'+$contactWorker.id) -Headers $adminHeaders -ContentType 'application/json' -Body $contactBody
  if($updatedWorker.email -ne 'updated.worker@example.com' -or $updatedWorker.phone -ne '+12025550123'){throw 'Docker Worker contact update failed'}
  $estimateBody=@{material_cost='0.01';labor_cost='0.01'}|ConvertTo-Json
  $estimate=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/quotations/estimate') -Headers $workerHeaders -ContentType 'application/json' -Body $estimateBody
  if($estimate.minimum_price -ne '0.04'){throw 'Docker minimum price rounding failed'}
  $quoteBody=@{material_cost='800.00';labor_cost='200.00';final_price='1600.00'}|ConvertTo-Json
  $draft=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/requests/'+$newRequest.id+'/quotations') -Headers $workerHeaders -ContentType 'application/json' -Body $quoteBody
  $sendBody=@{expected_lock_version=$draft.lock_version}|ConvertTo-Json
  $sent=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/quotations/'+$draft.id+'/send') -Headers $workerHeaders -ContentType 'application/json' -Body $sendBody
  $publicQuote=Invoke-RestMethod -Uri ($apiAddress+'/quotations/'+$sent.id) -Headers $headers
  if($publicQuote.final_price -ne '1600.00' -or $null -ne $publicQuote.internal_costs){throw 'Docker quotation redaction failed'}
  $decisionBody=@{outcome='accept';expected_lock_version=$sent.lock_version}|ConvertTo-Json
  $decision=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/quotations/'+$sent.id+'/decision') -Headers $headers -ContentType 'application/json' -Body $decisionBody
  if($decision.outcome -ne 'accept' -or $decision.terms.deposit_amount -ne '800.00' -or $decision.terms.po_required){throw 'Docker acceptance terms failed'}
  $retry=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/quotations/'+$sent.id+'/decision') -Headers $headers -ContentType 'application/json' -Body $decisionBody
  if($retry.terms.documents_due_at -ne $decision.terms.documents_due_at){throw 'Docker replay reset deadline'}
  $decisionHistory=Invoke-RestMethod -Uri ($apiAddress+'/requests/'+$newRequest.id+'/quotation-decisions') -Headers $headers
  if($decisionHistory.items.Count -ne 1){throw 'Docker decision history failed'}
  $uploadBody=@{file_name='deposit.txt';content_type='text/plain';byte_size=14}|ConvertTo-Json
  $intent=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/requests/'+$newRequest.id+'/uploads') -Headers $headers -ContentType 'application/json' -Body $uploadBody
  $uploadHeaders=@{}
  foreach($property in $intent.headers.PSObject.Properties){$uploadHeaders[$property.Name]=[string]$property.Value}
  $null=Invoke-WebRequest -Method Put -Uri $intent.url -Headers $uploadHeaders -ContentType 'text/plain' -Body ([Text.Encoding]::UTF8.GetBytes('deposit proof!'))
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/uploads/'+$intent.attachment_id+'/complete') -Headers $headers
  $evidenceBody=@{idempotency_key=[guid]::NewGuid().ToString();attachment_id=$intent.attachment_id;kind='deposit';amount='1000.00';paid_at=[DateTime]::UtcNow.AddMinutes(-1).ToString('o')}|ConvertTo-Json
  $evidence=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/quotations/'+$sent.id+'/evidence') -Headers $headers -ContentType 'application/json' -Body $evidenceBody
  $pending=Invoke-RestMethod -Uri ($apiAddress+'/quotations/'+$sent.id+'/fulfillment') -Headers $headers
  if($null -ne $pending.job){throw 'Unreviewed upload opened a Docker job'}
  $reviewBody=@{outcome='approved';expected_version=1}|ConvertTo-Json
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/evidence/'+$evidence.id+'/review') -Headers $workerHeaders -ContentType 'application/json' -Body $reviewBody
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/evidence/'+$evidence.id+'/review') -Headers $workerHeaders -ContentType 'application/json' -Body $reviewBody
  $fulfilled=Invoke-RestMethod -Uri ($apiAddress+'/quotations/'+$sent.id+'/fulfillment') -Headers $headers
  if($fulfilled.job.status -ne 'queued' -or $fulfilled.job.deposit_amount -ne '1000.00' -or $fulfilled.job.balance -ne '600.00'){throw 'Docker queued job/actual deposit snapshot failed'}
  $unitBody=@{name='kg'}|ConvertTo-Json
  $unit=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/units') -Headers $adminHeaders -ContentType 'application/json' -Body $unitBody
  $materialBody=@{name='Docker S45C';unit_id=$unit.id}|ConvertTo-Json
  $material=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/materials') -Headers $adminHeaders -ContentType 'application/json' -Body $materialBody
  $stockBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$material.version;mode='addition';quantity='10.000000';unit_cost='20.000000';reason='Opening test stock'}|ConvertTo-Json
  $null=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/materials/'+$material.id+'/stock') -Headers $adminHeaders -ContentType 'application/json' -Body $stockBody
  $planBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=0;lines=@(@{material_id=$material.id;quantity='12.000000'})}|ConvertTo-Json -Depth 5
  $plan=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $planBody
  if($plan.lines[0].reserved -ne '10.000000' -or $plan.lines[0].shortage -ne '2.000000' -or $plan.ready){throw 'Docker partial reservation failed'}
  $null=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $planBody
  $stock=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $workerHeaders
  if($stock.items[0].qty_on_hand -ne '10.000000' -or $stock.items[0].qty_reserved -ne '10.000000' -or $stock.items[0].qty_available -ne '0.000000'){throw 'Docker reservation consumed/over-reserved physical stock'}
  $clearBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$plan.version;lines=@()}|ConvertTo-Json -Depth 5
  $null=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $clearBody
  $released=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $workerHeaders
  if($released.items[0].qty_reserved -ne '0.000000' -or $released.items[0].qty_on_hand -ne '10.000000'){throw 'Docker release failed'}
  $supplierBody=@{name='Docker Supplier';email='supplier@example.com';phone='+66812345678'}|ConvertTo-Json
  $supplier=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/suppliers') -Headers $adminHeaders -ContentType 'application/json' -Body $supplierBody
  $supplierSearch=Invoke-RestMethod -Uri ($apiAddress+'/suppliers?search=Docker') -Headers $adminHeaders
  if($supplierSearch.items.Count -ne 1){throw 'Docker supplier search failed'}
  $orderBody=@{idempotency_key=[guid]::NewGuid().ToString();supplier_id=$supplier.id;job_ids=@($fulfilled.job.id);lines=@(@{material_id=$material.id;quantity='25';unit_cost='0.335'})}|ConvertTo-Json -Depth 5
  $order=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders') -Headers $adminHeaders -ContentType 'application/json' -Body $orderBody
  if($order.total_amount -ne '8.38' -or $order.status -ne 'ordered' -or $order.job_ids.Count -ne 1){throw 'Docker flexible order/half-up price failed'}
  $orderReplay=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders') -Headers $adminHeaders -ContentType 'application/json' -Body $orderBody
  if($orderReplay.id -ne $order.id){throw 'Docker duplicate order created'}
  $acceptOrder=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=1;action='accept';reason='Supplier confirmed by phone'}|ConvertTo-Json
  $acceptedOrder=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders/'+$order.id+'/actions') -Headers $adminHeaders -ContentType 'application/json' -Body $acceptOrder
  if($acceptedOrder.supplier_response -ne 'accepted' -or $acceptedOrder.status -ne 'ordered'){throw 'Docker supplier acceptance failed'}
  # An open supplier commitment must not block Worker plan edits or receive stock.
  $flexPlan=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=2;lines=@(@{material_id=$material.id;quantity='1'})}|ConvertTo-Json -Depth 5
  $flex=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $flexPlan
  $clearFlex=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$flex.version;lines=@()}|ConvertTo-Json -Depth 5
  $null=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $clearFlex
  $cancelOrder=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=2;action='cancel';reason='Demo cancellation before receipt'}|ConvertTo-Json
  $cancelledOrder=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders/'+$order.id+'/actions') -Headers $adminHeaders -ContentType 'application/json' -Body $cancelOrder
  $orderRead=Invoke-RestMethod -Uri ($apiAddress+'/material-orders/'+$order.id) -Headers $adminHeaders
  $jobOrders=Invoke-RestMethod -Uri ($apiAddress+'/material-orders?job_id='+$fulfilled.job.id) -Headers $adminHeaders
  if($cancelledOrder.status -ne 'cancelled' -or $orderRead.events.Count -ne 3 -or $jobOrders.items.Count -ne 1){throw 'Docker order cancel/history/job search failed'}
  $stockAfterOrder=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $adminHeaders
  if($stockAfterOrder.items[0].qty_on_hand -ne '10.000000' -or $stockAfterOrder.items[0].qty_reserved -ne '0.000000' -or $stockAfterOrder.items[0].avg_cost -ne '20.000000'){throw 'Docker procurement modified stock'}
  $receiveOrderBody=@{idempotency_key=[guid]::NewGuid().ToString();supplier_id=$supplier.id;lines=@(@{material_id=$material.id;quantity='5';unit_cost='30'})}|ConvertTo-Json -Depth 5
  $receiveOrder=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders') -Headers $adminHeaders -ContentType 'application/json' -Body $receiveOrderBody
  $accept=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=1;action='accept';reason='Supplier confirmed'}|ConvertTo-Json
  $receiveOrder=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/material-orders/'+$receiveOrder.id+'/actions') -Headers $adminHeaders -ContentType 'application/json' -Body $accept
  $termsBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$receiveOrder.version;credit_days=7}|ConvertTo-Json
  $receiving=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/receiving/orders/'+$receiveOrder.id+'/credit-terms') -Headers $adminHeaders -ContentType 'application/json' -Body $termsBody
  $receiptBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$receiving.version;note='Mixed delivery';lines=@(@{material_id=$material.id;good='3';damaged='2'})}|ConvertTo-Json -Depth 5
  $receiving=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/receiving/orders/'+$receiveOrder.id+'/receipts') -Headers $workerHeaders -ContentType 'application/json' -Body $receiptBody
  if($receiving.status -ne 'partially_received' -or $receiving.claims.Count -ne 1 -or $null -ne $receiving.finance){throw 'Docker receipt/Worker finance redaction failed'}
  $claimBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$receiving.version;note='Supplier contacted for replacements'}|ConvertTo-Json
  $claimId=$receiving.claims[0].id
  $receiving=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/receiving/orders/'+$receiveOrder.id+'/claims/'+$claimId+'/submit') -Headers $workerHeaders -ContentType 'application/json' -Body $claimBody
  $replacementBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$receiving.version;note='Good replacements';lines=@(@{material_id=$material.id;good='2';damaged='0';replacement_claim_id=$claimId})}|ConvertTo-Json -Depth 5
  $receiving=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/receiving/orders/'+$receiveOrder.id+'/receipts') -Headers $workerHeaders -ContentType 'application/json' -Body $replacementBody
  if($receiving.status -ne 'received' -or $receiving.claims[0].status -ne 'resolved'){throw 'Docker replacement completion failed'}
  $settlementBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$receiving.version;method='cash';paid_at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json
  $settled=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/receiving/orders/'+$receiveOrder.id+'/settlement') -Headers $adminHeaders -ContentType 'application/json' -Body $settlementBody
  if($settled.finance.payment_status -ne 'paid' -or $settled.finance.settlement.amount -ne '150.00' -or $null -eq $settled.finance.payment_due_at){throw 'Docker supplier settlement/credit failed'}
  $receivedStock=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $adminHeaders
  $workerStock=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $workerHeaders
  if($receivedStock.items[0].qty_on_hand -ne '15.000000' -or $receivedStock.items[0].avg_cost -ne '23.333333' -or $null -ne $workerStock.items[0].avg_cost){throw 'Docker receipt stock/cost/privacy failed'}
  $currentPlan=Invoke-RestMethod -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders
  $productionPlan=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$currentPlan.version;lines=@(@{material_id=$material.id;quantity='5'})}|ConvertTo-Json -Depth 5
  $readyPlan=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/jobs/'+$fulfilled.job.id+'/materials') -Headers $workerHeaders -ContentType 'application/json' -Body $productionPlan
  if(-not $readyPlan.ready){throw 'Docker production plan not ready'}
  $startBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=0;expected_plan_version=$readyPlan.version}|ConvertTo-Json
  $production=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/production/jobs/'+$fulfilled.job.id+'/start') -Headers $workerHeaders -ContentType 'application/json' -Body $startBody
  $usageBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$production.version;note='Actual issue';lines=@(@{material_id=$material.id;quantity='2'})}|ConvertTo-Json -Depth 5
  $production=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/production/jobs/'+$fulfilled.job.id+'/usage') -Headers $workerHeaders -ContentType 'application/json' -Body $usageBody
  $timeBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$production.version;minutes=30;note='Machine time'}|ConvertTo-Json
  $production=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/production/jobs/'+$fulfilled.job.id+'/time') -Headers $workerHeaders -ContentType 'application/json' -Body $timeBody
  $completeBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$production.version}|ConvertTo-Json
  $production=Invoke-RestMethod -Method Post -Uri ($apiAddress+'/production/jobs/'+$fulfilled.job.id+'/complete') -Headers $workerHeaders -ContentType 'application/json' -Body $completeBody
  if($production.status -ne 'completed' -or $production.materials[0].released -ne '3.000000' -or $production.labor_minutes -ne 30 -or $null -ne $production.finance){throw 'Docker production completion/release/Worker redaction failed'}
  $laborBody=@{idempotency_key=[guid]::NewGuid().ToString();expected_version=$production.version;amount='150.00'}|ConvertTo-Json
  $cost=Invoke-RestMethod -Method Put -Uri ($apiAddress+'/production/jobs/'+$fulfilled.job.id+'/labor-cost') -Headers $adminHeaders -ContentType 'application/json' -Body $laborBody
  if($cost.finance.material_cost -ne '46.67' -or $cost.finance.total_cost -ne '196.67'){throw 'Docker actual production cost failed'}
  $finalStock=Invoke-RestMethod -Uri ($apiAddress+'/materials?search=S45C') -Headers $adminHeaders
  if($finalStock.items[0].qty_on_hand -ne '13.000000' -or $finalStock.items[0].qty_reserved -ne '0.000000' -or $finalStock.items[0].avg_cost -ne '23.333333'){throw 'Docker usage/release changed balances incorrectly'}
  $delivered=$false
  for($i=0;$i -lt 15;$i++){
   $inbox=Invoke-RestMethod -Uri ($apiAddress+'/notifications') -Headers $headers
   if($inbox.items.Count -ge 1){$delivered=$true;break};Start-Sleep -Seconds 1
  }
  if(-not $delivered){throw 'Worker did not deliver the Docker request notification'}
 } catch {throw 'Docker customer/request/worker flow failed; credentials withheld'}
 docker compose @composeArgs stop api worker
 if($LASTEXITCODE -ne 0){throw 'Graceful container shutdown failed'}
 Write-Output 'PHASE11_VERIFICATION_PASSED'
 Write-Output 'WORKER_CONTACTS_VERIFICATION_PASSED'
} finally {
 # This unique Compose project and its fresh test volumes are the exact targets.
 docker compose @composeArgs down -v
 Pop-Location
 Remove-Item -LiteralPath $emptyEnv.FullName
 foreach($key in $savedEnvironment.Keys){[Environment]::SetEnvironmentVariable($key,$savedEnvironment[$key],'Process')}
}
