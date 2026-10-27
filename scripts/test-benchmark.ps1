$ErrorActionPreference = 'Stop'
if (-not ('Lathe.Benchmark.TestServer' -as [type])) {
  Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
namespace Lathe.Benchmark {
    public sealed class TestServer : IDisposable {
        readonly TcpListener listener = new TcpListener(IPAddress.Loopback, 0);
        readonly CancellationTokenSource stop = new CancellationTokenSource();
        readonly string mode;
        int index;
        public string Address { get; private set; }
        public TestServer(string mode) {
            this.mode = mode;
            listener.Start();
            Address = "http://127.0.0.1:" + ((IPEndPoint)listener.LocalEndpoint).Port;
            _ = Serve();
        }
        async Task Serve() {
            try {
                while (!stop.IsCancellationRequested) {
                    var client = await listener.AcceptTcpClientAsync(stop.Token);
                    _ = Handle(client, Interlocked.Increment(ref index));
                }
            } catch (OperationCanceledException) {} catch (ObjectDisposedException) {}
        }
        async Task Handle(TcpClient client, int number) {
            try {
                using (client)
                using (var stream = client.GetStream()) {
                    // Headers contain no request body; consume until CRLF CRLF.
                    int matched = 0;
                    byte[] marker = {13, 10, 13, 10};
                    byte[] single = new byte[1];
                    while (matched < marker.Length) {
                        if (await stream.ReadAsync(single, 0, 1, stop.Token) == 0) return;
                        matched = single[0] == marker[matched] ? matched + 1 : 0;
                    }
                    if (mode == "reorder") await Task.Delay(number == 1 ? 500 : 15, stop.Token);
                    var status = mode == "error" ? "500 Internal Server Error" : "200 OK";
                    var declared = mode == "truncated" ? "10" : "4";
                    var headers = Encoding.ASCII.GetBytes("HTTP/1.1 " + status + "\r\nContent-Length: " + declared + "\r\nConnection: close\r\n\r\n");
                    await stream.WriteAsync(headers, 0, headers.Length, stop.Token);
                    if (mode == "body") await Task.Delay(300, stop.Token);
                    if (mode == "timeout") await Task.Delay(5000, stop.Token);
                    var body = Encoding.ASCII.GetBytes("test");
                    await stream.WriteAsync(body, 0, body.Length, stop.Token);
                }
            } catch (OperationCanceledException) {} catch (IOException) {} catch (ObjectDisposedException) {}
        }
        public void Dispose() { stop.Cancel(); listener.Stop(); }
    }
}
'@
}

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw $Message }
}
foreach ($mode in @('reorder','body','error','truncated','timeout')) {
  $server = [Lathe.Benchmark.TestServer]::new($mode)
  try {
    $requests = if ($mode -eq 'reorder') { 2 } else { 1 }
    $seconds = if ($mode -eq 'timeout') { 1 } else { 5 }
    $report = & (Join-Path $PSScriptRoot 'benchmark-http.ps1') -BaseUrl $server.Address -Path '/fixture' -Requests $requests -Concurrency 2 -TimeoutSeconds $seconds -BearerToken '' -IncludeSamples
    Assert-True ($report.Samples.Count -eq $requests) 'Some benchmark attempts were lost.'
    switch ($mode) {
      'reorder' {
        $times = @($report.Samples.FullResponseMs | Sort-Object)
        Assert-True ($report.Failures -eq 0) 'Reordered requests failed.'
        Assert-True ($times[0] -lt $times[1] * 0.6) 'Fast request incorrectly includes the earlier slow request wait.'
        Assert-True ($report.ResponseBytes -eq 8) 'Bodies were not drained/counting is wrong.'
      }
      'body' {
        Assert-True ($report.Failures -eq 0) 'Delayed body request failed.'
        Assert-True (($report.Samples[0].FullResponseMs - $report.Samples[0].HeadersMs) -ge 200) 'Reported latency stopped at headers.'
      }
      'error' {
        Assert-True ($report.Failures -eq 1 -and $report.StatusCounts['500'] -eq 1) 'HTTP error omitted from failure/status counts.'
        Assert-True ($null -eq $report.P95Ms) 'Failed responses must not be mixed into successful latency percentiles.'
      }
      'truncated' {
        Assert-True ($report.Failures -eq 1) 'Truncated body incorrectly counted as success.'
      }
      'timeout' {
        Assert-True ($report.Failures -eq 1 -and $report.FailureCounts['timeout'] -eq 1) 'Body read timeout was not enforced.'
      }
    }
  } finally { $server.Dispose() }
}
. (Join-Path $PSScriptRoot 'Read-PerformanceReport.ps1')
$payload = @{name=('long JSON ' * 1000);jobs=1000} | ConvertTo-Json -Compress
$line = "    fixture.go:1: PERFORMANCE_BASELINE_JSON=$payload`n"
$chunks = @()
for ($offset = 0; $offset -lt $line.Length; $offset += 97) {
  $chunks += [pscustomobject]@{Action='output';Test='TestPerformanceBaseline';Output=$line.Substring($offset,[Math]::Min(97,$line.Length-$offset))}
}
$passEvent = [pscustomobject]@{Action='pass';Test='TestPerformanceBaseline'}
$parsed = Read-PerformanceReport -Events @($chunks + $passEvent) -ExitCode 0
Assert-True ($parsed.jobs -eq 1000 -and $parsed.name.Length -eq 10000) 'Fragmented Go JSON report was corrupted.'
foreach ($outcome in @('skip','fail')) {
  $rejected = $false
  try { $null = Read-PerformanceReport -Events @($chunks + [pscustomobject]@{Action=$outcome;Test='TestPerformanceBaseline'}) -ExitCode 0 } catch { $rejected = $true }
  Assert-True $rejected 'Skipped or failed performance test was accepted as evidence.'
}
Write-Output 'BENCHMARK_HARNESS_TESTS_PASSED'
