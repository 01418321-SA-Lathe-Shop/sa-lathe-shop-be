param([ValidateSet('plan','apply','status','verify')][string]$Action='plan')
$ErrorActionPreference='Stop'
# Deliberately does not read .env. Private values must be process environment.
$prompted=@()
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
 if($Action -eq 'apply' -or $Action -eq 'verify'){
  $statusJson=go run ./cmd/seed-demo status
  if($LASTEXITCODE -ne 0){throw 'Seed preflight refused the target; no changes made'}
  $status=$statusJson | ConvertFrom-Json
  if($Action -eq 'verify' -and $status.status.state -ne 'ready'){throw 'Only an active, seeded dataset can be verified'}
  if(($Action -eq 'apply' -and $status.status.state -eq 'uninitialized') -or $Action -eq 'verify'){
   foreach($spec in $status.manifest){
    if(-not [Environment]::GetEnvironmentVariable($spec.password_env,'Process')){
     $secure=Read-Host "Private password for $($spec.username) (12+ characters, <=128 UTF-8 bytes)" -AsSecureString
     $ptr=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
     try {[Environment]::SetEnvironmentVariable($spec.password_env,[Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr),'Process');$prompted+=$spec.password_env}
     finally {[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr);$secure.Dispose()}
    }
   }
  }
 }
 go run ./cmd/seed-demo $Action --profile accounts
 if($LASTEXITCODE -ne 0){throw 'Seed command failed; see safe diagnostic above'}
} finally {
 foreach($key in $prompted){[Environment]::SetEnvironmentVariable($key,$null,'Process')}
 Pop-Location
}
