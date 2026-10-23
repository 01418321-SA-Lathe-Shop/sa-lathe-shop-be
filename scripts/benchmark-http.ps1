param(
  [string]$BaseUrl = 'http://localhost:8080',
  [Parameter(Mandatory=$true)][string]$Path,
  [ValidateRange(1,10000)][int]$Requests = 200,
  [ValidateRange(1,100)][int]$Concurrency = 10,
  [string]$BearerToken = $env:LATHE_BENCH_TOKEN
)
$ErrorActionPreference = 'Stop'
if (-not $Path.StartsWith('/')) { throw 'Path must begin with /.' }
$uri = [Uri]::new($BaseUrl.TrimEnd('/') + $Path)
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(30)
if ($BearerToken) {
  $client.DefaultRequestHeaders.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer', $BearerToken)
}
$samples = [System.Collections.Generic.List[double]]::new()
$failures = 0
$watch = [System.Diagnostics.Stopwatch]::StartNew()
try {
  for ($start = 0; $start -lt $Requests; $start += $Concurrency) {
    $count = [Math]::Min($Concurrency, $Requests - $start)
    $batch = [System.Collections.Generic.List[object]]::new()
    for ($i = 0; $i -lt $count; $i++) {
      $timer = [System.Diagnostics.Stopwatch]::StartNew()
      $task = $client.GetAsync($uri, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead)
      $batch.Add([pscustomobject]@{ Timer = $timer; Task = $task })
    }
    foreach ($item in $batch) {
      try {
        $response = $item.Task.GetAwaiter().GetResult()
        $item.Timer.Stop()
        $samples.Add($item.Timer.Elapsed.TotalMilliseconds)
        if (-not $response.IsSuccessStatusCode) { $failures++ }
        $response.Dispose()
      } catch {
        $item.Timer.Stop()
        $samples.Add($item.Timer.Elapsed.TotalMilliseconds)
        $failures++
      }
    }
  }
} finally {
  $watch.Stop()
  $client.Dispose()
}
$sorted = @($samples | Sort-Object)
function Get-Percentile([double[]]$Values, [double]$Percentile) {
  $index = [Math]::Max(0, [Math]::Ceiling($Values.Count * $Percentile) - 1)
  return $Values[$index]
}
$success = $Requests - $failures
[pscustomobject]@{
  Endpoint = $uri.AbsolutePath
  Requests = $Requests
  Concurrency = $Concurrency
  Failures = $failures
  SuccessPercent = [Math]::Round(100.0 * $success / $Requests, 2)
  P50Ms = [Math]::Round((Get-Percentile $sorted 0.50), 2)
  P95Ms = [Math]::Round((Get-Percentile $sorted 0.95), 2)
  P99Ms = [Math]::Round((Get-Percentile $sorted 0.99), 2)
  MaxMs = [Math]::Round($sorted[-1], 2)
  RequestsPerSecond = [Math]::Round($Requests / $watch.Elapsed.TotalSeconds, 2)
  ElapsedSeconds = [Math]::Round($watch.Elapsed.TotalSeconds, 2)
} | Format-List
