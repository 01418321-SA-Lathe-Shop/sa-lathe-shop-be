param(
  [string]$BaseUrl = 'http://localhost:8080',
  [Parameter(Mandatory=$true)][string]$Path,
  [ValidateRange(1,10000)][int]$Requests = 200,
  [ValidateRange(1,100)][int]$Concurrency = 10,
  [string]$BearerToken = $env:LATHE_BENCH_TOKEN,
  [ValidateRange(1,300)][int]$TimeoutSeconds = 30,
  [switch]$IncludeSamples,
  [switch]$AsJson
)
$ErrorActionPreference = 'Stop'
if (-not $Path.StartsWith('/') -or $Path.StartsWith('//')) { throw 'Path must begin with one /.' }
$uri = [Uri]::new($BaseUrl.TrimEnd('/') + $Path)
if (-not $uri.IsAbsoluteUri -or $uri.Scheme -notin @('http','https') -or $uri.UserInfo) {
  throw 'BaseUrl must be an absolute HTTP(S) URL without credentials.'
}
if ($BearerToken -match '[\x00-\x20\x7f]') { throw 'Bearer token must not contain whitespace/control characters.' }
if (-not ('Lathe.Benchmark.Runner' -as [type])) {
  Add-Type -Path (Join-Path $PSScriptRoot 'HttpBenchmark.cs')
}
$result = [Lathe.Benchmark.Runner]::RunAsync($uri.AbsoluteUri, $BearerToken, $Requests, $Concurrency, $TimeoutSeconds).GetAwaiter().GetResult()
$successful = @($result.Samples | Where-Object Success)
$sorted = @($successful.FullResponseMs | Sort-Object)
$headers = @($successful.HeadersMs | Sort-Object)
function Get-Percentile([double[]]$Values, [double]$Percentile) {
  if ($Values.Count -eq 0) { return $null }
  $index = [Math]::Max(0, [Math]::Ceiling($Values.Count * $Percentile) - 1)
  return [Math]::Round($Values[$index], 2)
}
$statuses = [ordered]@{}
foreach ($group in ($result.Samples | Group-Object Status)) { $statuses[$group.Name] = $group.Count }
$errors = [ordered]@{}
foreach ($group in ($result.Samples | Where-Object { -not $_.Success } | Group-Object FailureCategory)) { $errors[$group.Name] = $group.Count }
$report = [pscustomobject]@{
  Endpoint = $uri.AbsolutePath
  Requests = $Requests
  Concurrency = $Concurrency
  Failures = $Requests - $successful.Count
  SuccessPercent = [Math]::Round(100.0 * $successful.Count / $Requests, 2)
  LatencyPopulation = 'successful full responses only; includes body drain'
  P50Ms = Get-Percentile $sorted 0.50
  P95Ms = Get-Percentile $sorted 0.95
  P99Ms = Get-Percentile $sorted 0.99
  MaxMs = Get-Percentile $sorted 1
  HeadersP95Ms = Get-Percentile $headers 0.95
  ResponseBytes = ($result.Samples | Measure-Object Bytes -Sum).Sum
  StatusCounts = $statuses
  FailureCounts = $errors
  RequestsPerSecond = [Math]::Round($Requests / $result.ElapsedSeconds, 2)
  ElapsedSeconds = [Math]::Round($result.ElapsedSeconds, 2)
}
if ($IncludeSamples) { $report | Add-Member -NotePropertyName Samples -NotePropertyValue $result.Samples }
if ($AsJson) { $report | ConvertTo-Json -Depth 8 } else { $report }
