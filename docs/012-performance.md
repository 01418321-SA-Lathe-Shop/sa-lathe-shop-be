# Performance Testing, Latency Benchmarks & Phase 13 Quality Verification

## Current harness and repeatable synthetic baseline (Phase 15)

The original Phase 13 harness stopped timers in await order and did not consume
response bodies. Discard latency numbers produced by that version; they are not
a valid before/after baseline. The corrected harness uses independently timed
.NET async workers and drains bodies with an end-to-end request deadline.
`HeadersP95Ms` is distinct from the full-response p50/p95/p99. Percentiles include
successful full responses only; inspect Failures, StatusCounts and FailureCounts
alongside them. All-failed runs have null percentiles, not an apparent zero-ms win.

```powershell
# Local loopback regression tests, no application or DB required:
./scripts/test-benchmark.ps1

# Real API read, token supplied only in this process environment:
./scripts/benchmark-http.ps1 -Path '/fulfillment/jobs?limit=25&offset=0' -Concurrency 10 -AsJson

# Full disposable baseline. Requires Docker, Go 1.27.3 and PowerShell 7.
# Never accepts or migrates an existing application DB; creates fresh resources.
./scripts/measure-performance.ps1
```

The synthetic baseline seeds through real services/repositories and the private
S3 upload/inspection/freeze path, with database triggers enabled: 100 accounts
(1 Admin, 9 Workers, 90 company Customers), 1,000 jobs, 1,000 materials and 100
suppliers. Job states cover queued, active, completed, free rework and settled;
three active jobs carry 10/100/1,000 time entries. Seeding and password hashing
are excluded from read measurements. No fixture credentials/tokens are saved.

Each read scenario warms up, then executes three 100-request runs at concurrency
1/10/25. A separate 100-identity mix uses 500-ms think time. Record per-run full
latencies, status/failure counts, response bytes, throughput, client-issued SQL
count/wall time and PostgreSQL pool wait. Query wall time includes client row
consumption; batched statements are counted but their time is not included.
Sequential service-login timings are separate from HTTP login throughput.
The report also includes actual read-only EXPLAIN ANALYZE/BUFFERS JSON plans and
stored production-result bytes, toolchain/OS/architecture and Docker CPU/memory.

Results are written under ignored `bin/performance-sa-lathe-perf-*.json` only
after an executed, passing test produces a report. The unique test containers,
network and volumes are removed; existing databases, .env and Docker settings
are untouched. This fixture can take several minutes. A missing test URL skips
the Go test in the ordinary unit suite; the dedicated runner rejects skips.

Limitations: these are real loopback HTTP feature handlers and common middleware,
DB-backed authentication and PostgreSQL, not a deployed server/network SLA.
Request log output is discarded. The background worker and external LINE are
not part of the load. Initial results measure the unoptimized legacy response
path. Re-run on the same environment/dataset after each optimization, and do not
compare a race-instrumented run against a normal performance run.

The older sections below describe the historical Phase 13 deliverables.

### Measured legacy baseline: 2026-10-27

Fresh project `sa-lathe-perf-aa349ddd`, Go 1.27.3 windows/amd64, Docker allocation
8 CPUs / 8,212,668,416 bytes, connection pool 10. Application baseline c83409e
plus the Phase 15 test-only fixture/harness. The benchmark binary was compiled
before any Phase 16 edits; those edits do not affect these results. Fixture
seeding took 304.53 s; the complete test passed in 460.57 s. The raw local report
is ignored `bin/performance-sa-lathe-perf-aa349ddd.json`; no credentials are in it.

Full-response p95 below is the median of three 100-request runs after warm-up;
all measurements had zero failed requests. Values are milliseconds.

| Read, page size 25 | Concurrency 1 | 10 | 25 | SQL statements/request |
| --- | ---: | ---: | ---: | ---: |
| Liveness | 0.54 | 2.01 | 2.13 | 0 |
| Requests | 7.04 | 18.74 | 29.73 | 2 |
| Fulfillment jobs | 261.95 | 460.21 | 1021.91 | 152 |
| Production job list | 17.25 | 31.84 | 67.66 | 6 |
| Material substring search | 11.88 | 22.18 | 69.83 | 5 |
| Supplier substring search | 10.57 | 24.79 | 42.03 | 5 |

The 100-identity mix had p95 1962.94 / 2371.34 / 1801.65 ms in its three runs,
with database pool waits recorded. This is a pressure scenario, not evidence of
a deployed 100-user SLA. The same-process client/server and local Docker network
must remain explicit when comparing results.

Direct service usage writes (not HTTP), 1/10/100 material lines, issued
19/73/613 SQL statements respectively. Three observed durations were
47.97/28.04/42.58 ms, 93.08/101.65/101.53 ms and 796.80/797.20/715.25 ms.
The 10/100/1,000-time-entry jobs used 121,558 / 635,754 / 53,569,820 stored
production-result bytes; the first job includes nine subsequent write samples.
No old snapshot was removed to make these numbers smaller.

Representative EXPLAIN ANALYZE execution times were 0.86 ms for the production
list, 0.18 ms for material search, 0.14 ms for supplier search and 19.74 ms for
the 1,000-entry production document. These single-plan diagnostics do not include
HTTP/authentication/network overhead. They support prioritizing round trips and
history aggregation over speculative search indexes. Re-evaluate after the
round-trip fix rather than treating every sequential scan as a performance bug.

Verification also exposed and fixed a report-parser edge case: Go test2json
splits long output into 1-KiB chunks. The report parser reassembles original output
without inserted newlines, and regression tests cover fragments and skipped/
failed tests. The first run's report was rejected, not published as a success;
its disposable resources were cleaned up before the passing fresh rerun.

## 1. Local Latency and Query Measurements

These instructions guide repeatable performance and latency measurements for comparing backend code changes on the same machine, dataset, and configuration.

> [!WARNING]
> These measurements are for local comparative benchmarking only. They do not constitute an SLA, production capacity estimate, or hosting recommendation. Never run load tests against production databases. Always use a fresh disposable local database with synthetic fixtures.

---

## 2. HTTP Latency Benchmarking Harness

Start the local API and database using the setup runbook ([000-setup.md](000-setup.md)). Sign in and set the token in your current PowerShell process (`$env:LATHE_BENCH_TOKEN`).

Run the read-only benchmarking script:
```powershell
$env:LATHE_BENCH_TOKEN = '<local test bearer token>'
./scripts/benchmark-http.ps1 -BaseUrl 'http://localhost:8080' -Path '/fulfillment/jobs?limit=25&offset=0' -Requests 200 -Concurrency 10
```

### Measurement Guidelines
- **Baselines**: Use `/health` for an application process baseline, and an authenticated read route (e.g. `/requests?limit=25&offset=0` or `/fulfillment/jobs?limit=25&offset=0`) for database and business logic behavior.
- **Repeatability**: Repeat each test run at least three times after a warm-up run.
- **Metrics to Record**: Git commit, Go version, OS, Docker CPU/RAM allocation, dataset row counts, concurrency level, total requests, p50/p95/p99 round-trip times, and throughput (requests/sec).
- **Harness Scope**: The harness measures client-observed response round-trip time. It does not isolate network vs SQL latency and does not benchmark write mutations.

---

## 3. PostgreSQL Query Plan & Execution Time

To inspect query execution plans, extract the parameterized SQL from repository methods (e.g., `readFulfillment` and `FulfillmentJobs` in `internal/repository/postgres/delivery_read.go`) and execute in a read-only transaction:

```sql
BEGIN TRANSACTION READ ONLY;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS, FORMAT TEXT)
SELECT j.id, j.status, j.fulfillment_version, j.production_cycle
FROM job AS j
WHERE j.id = '00000000-0000-4000-8000-000000000001';
ROLLBACK;
```

### Cumulative Query Statistics (`pg_stat_statements`)
On disposable development databases where `pg_stat_statements` is available:

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

SELECT queryid, calls, round(mean_exec_time::numeric, 3) AS mean_ms,
       round(max_exec_time::numeric, 3) AS max_ms, rows,
       shared_blks_hit, shared_blks_read, left(query, 500) AS query
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
```

---

## 4. Phase 13 Quality & Verification Record (2026-10-23)

- **Scope**: Canonical OpenAPI contract publication, security review, CI setup, and performance measurement tooling.
- **Deliverables**:
  - `openapi.yaml`: Complete OpenAPI 3.0 specification covering 73 API path templates, authentication, and error schemas.
  - PowerShell API examples ([010-api-examples.md](010-api-examples.md)).
  - Comprehensive source-level security review ([011-security-review.md](011-security-review.md)).
  - `.github/workflows/ci.yml`: Automated CI workflow running formatting, build, vet, and unit test suites.
  - `scripts/benchmark-http.ps1`: Parallel HTTP latency test harness.
- **Checks Run**:
  - `gofmt -l`, `go build ./...`, `go vet ./...`, and `go test ./...` passed cleanly.
  - YAML syntax parsing for `openapi.yaml` and `.github/workflows/ci.yml` passed.
  - Benchmark harness passed syntax parsing and a 6-request smoke test against a loopback server (100% success).
