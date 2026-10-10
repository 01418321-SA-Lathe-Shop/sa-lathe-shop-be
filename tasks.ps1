param([ValidateSet('run','worker','build','test','lint','migrate-up','migrate-down','migrate-version','dev-up','dev-down')][string]$Task='test')
$ErrorActionPreference='Stop'
Push-Location $PSScriptRoot
try {
  if(Test-Path -LiteralPath '.env') {
    foreach($line in Get-Content -LiteralPath '.env') {
      if($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
        $name=$Matches[1]; $value=$Matches[2].Trim()
        if($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))){$value=$value.Substring(1,$value.Length-2)}
        if([Environment]::GetEnvironmentVariable($name,'Process') -eq $null){[Environment]::SetEnvironmentVariable($name,$value,'Process')}
      }
    }
  }
  switch($Task) {
    'run' { go run ./cmd/api }
    'worker' { go run ./cmd/worker }
    'build' { go build ./... }
    'test' { go test ./... }
    'lint' {
      $files=@(Get-ChildItem cmd,internal -Filter '*.go' -File -Recurse | ForEach-Object {$_.FullName})
      $unformatted= & gofmt -l $files
      if($unformatted){throw "Run gofmt on: $unformatted"}
      go vet ./...
    }
    'dev-up' { docker compose up -d postgres s3 }
    'dev-down' { docker compose down }
    default {
      $dsn=$env:DATABASE_URL
      if(-not $dsn){
        $dbHost=if($env:DB_HOST){$env:DB_HOST}else{'localhost'}
        $dbPort=if($env:DB_PORT){$env:DB_PORT}else{'55432'}
        $dbUser=if($env:DB_USER){$env:DB_USER}else{'sa_lathe_shop'}
        $dbName=if($env:DB_NAME){$env:DB_NAME}else{'sa_lathe_shop'}
        $ssl=if($env:DB_SSLMODE){$env:DB_SSLMODE}else{'disable'}
        $dsn='postgres://'+[uri]::EscapeDataString($dbUser)+':'+[uri]::EscapeDataString($env:DB_PASSWORD)+'@'+$dbHost+':'+$dbPort+'/'+[uri]::EscapeDataString($dbName)+'?sslmode='+$ssl
      }
      $direction=switch($Task){'migrate-up'{'up'} 'migrate-down'{'down'} 'migrate-version'{'version'}}
      $argsList=@('-path','migrations','-database',$dsn,$direction)
      if($Task -eq 'migrate-down'){$argsList+='1'}
      go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 @argsList
    }
  }
  if($LASTEXITCODE -ne 0){throw "Task $Task failed with exit code $LASTEXITCODE"}
} finally { Pop-Location }
