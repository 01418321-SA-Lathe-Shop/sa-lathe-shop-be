# No .env loading: this check needs source, binaries and the public Go vulnerability DB only.
$ErrorActionPreference = 'Stop'
Push-Location (Split-Path $PSScriptRoot -Parent)
try {
  function Invoke-CheckedGo([string[]]$Arguments) {
    & go @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Go security check failed (exit $LASTEXITCODE)." }
  }
  $binarySuffix = if ($IsWindows) { '.exe' } else { '' }
  New-Item -ItemType Directory -Path 'bin' -Force | Out-Null
  Invoke-CheckedGo @('build', '-trimpath', '-o', "bin/api$binarySuffix", './cmd/api')
  Invoke-CheckedGo @('build', '-trimpath', '-o', "bin/worker$binarySuffix", './cmd/worker')
  $scanner = 'golang.org/x/vuln/cmd/govulncheck@v1.9.0'
  Invoke-CheckedGo @('run', $scanner, './...')
  Invoke-CheckedGo @('run', $scanner, '-mode=binary', "bin/api$binarySuffix")
  Invoke-CheckedGo @('run', $scanner, '-mode=binary', "bin/worker$binarySuffix")
  Write-Output 'SECURITY_CHECKS_PASSED (source, api, worker)'
} finally {
  Pop-Location
}
