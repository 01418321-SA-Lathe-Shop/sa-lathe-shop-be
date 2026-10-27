using System;
using System.Diagnostics;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Threading;
using System.Threading.Tasks;

namespace Lathe.Benchmark
{
    public sealed class Sample
    {
        public int Index { get; set; }
        public int Status { get; set; }
        public double? HeadersMs { get; set; }
        public double FullResponseMs { get; set; }
        public long Bytes { get; set; }
        public string FailureCategory { get; set; }
        public bool Success { get { return FailureCategory == null; } }
    }

    public sealed class Result
    {
        public Sample[] Samples { get; set; }
        public double ElapsedSeconds { get; set; }
    }

    // .NET tasks perform timing at completion, never in PowerShell's ordered await loop.
    public static class Runner
    {
        public static async Task<Result> RunAsync(string address, string bearer, int requests, int concurrency, int timeoutSeconds)
        {
            var samples = new Sample[requests];
            using (var handler = new HttpClientHandler { MaxConnectionsPerServer = concurrency, AllowAutoRedirect = false })
            using (var client = new HttpClient(handler))
            {
                client.Timeout = TimeSpan.FromSeconds(timeoutSeconds);
                if (!String.IsNullOrEmpty(bearer))
                    client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", bearer);
                var elapsed = Stopwatch.StartNew();
                int next = -1;
                var workers = new Task[Math.Min(concurrency, requests)];
                for (int worker = 0; worker < workers.Length; worker++)
                {
                    workers[worker] = Task.Run(async () =>
                    {
                        var buffer = new byte[64 * 1024];
                        while (true)
                        {
                            int index = Interlocked.Increment(ref next);
                            if (index >= requests) return;
                            var sample = new Sample { Index = index };
                            var timer = Stopwatch.StartNew();
                            try
                            {
                                // Timeout covers draining the body too; HttpClient's timeout
                                // alone ends at headers when ResponseHeadersRead is selected.
                                using (var deadline = new CancellationTokenSource(TimeSpan.FromSeconds(timeoutSeconds)))
                                using (var response = await client.GetAsync(address, HttpCompletionOption.ResponseHeadersRead, deadline.Token).ConfigureAwait(false))
                                {
                                    sample.HeadersMs = timer.Elapsed.TotalMilliseconds;
                                    sample.Status = (int)response.StatusCode;
                                    using (var body = await response.Content.ReadAsStreamAsync().ConfigureAwait(false))
                                    {
                                        int count;
                                        while ((count = await body.ReadAsync(buffer, 0, buffer.Length, deadline.Token).ConfigureAwait(false)) != 0)
                                            sample.Bytes += count;
                                    }
                                    if (!response.IsSuccessStatusCode) sample.FailureCategory = "http_status";
                                }
                            }
                            catch (OperationCanceledException) { sample.FailureCategory = "timeout"; }
                            catch (Exception error) { sample.FailureCategory = error.GetType().Name; }
                            finally
                            {
                                timer.Stop();
                                sample.FullResponseMs = timer.Elapsed.TotalMilliseconds;
                                samples[index] = sample;
                            }
                        }
                    });
                }
                await Task.WhenAll(workers).ConfigureAwait(false);
                elapsed.Stop();
                return new Result { Samples = samples, ElapsedSeconds = elapsed.Elapsed.TotalSeconds };
            }
        }
    }
}
