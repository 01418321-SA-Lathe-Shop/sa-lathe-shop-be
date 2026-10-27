# Performance Testing, Latency Benchmarks & Phase 13 Quality Verification

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
