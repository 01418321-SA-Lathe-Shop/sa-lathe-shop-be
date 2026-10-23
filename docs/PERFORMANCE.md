# Local latency and query measurements

These measurements are for comparing changes on the same machine, dataset and configuration. They are not an SLA, production capacity estimate, or recommendation for a host. Do not run load against production. Use a fresh disposable local database with synthetic data and a normal authenticated account.

## HTTP latency

Start the local API and database using the existing setup guide. Sign in and set the token only in the current PowerShell process (`$env:LATHE_BENCH_TOKEN`). Use the committed read-only harness:

```powershell
$env:LATHE_BENCH_TOKEN = '<local test bearer token>'
./scripts/benchmark-http.ps1 -BaseUrl 'http://localhost:8080' -Path '/fulfillment/jobs?limit=25&offset=0' -Requests 200 -Concurrency 10
```

Use `/health` for a process baseline and an authenticated, populated read route such as `/requests?limit=25&offset=0` or `/fulfillment/jobs?limit=25&offset=0` for application/database behavior. Repeat each run at least three times after warm-up; record commit, Go version, OS, Docker resources, row counts, endpoint, concurrency, request count, p50/p95/p99, failures and achieved requests/second. The harness records response-header round-trip time from the local client; it does not isolate network, server, or SQL time and does not benchmark writes.

Keep concurrency modest on a laptop. A local result includes PowerShell/.NET, Docker and database scheduling overhead. Use identical fixture size, migration version, pool settings and warm-up when comparing two commits. Never compare results from a cold first run with a warmed run as if only code changed.

## PostgreSQL query time

To inspect a query, take the exact parameterized SELECT from the repository method being measured (for example `readFulfillment` and `FulfillmentJobs` in `internal/repository/postgres/delivery_read.go`), substitute only synthetic fixture UUIDs, and run it on the disposable local database:

```sql
BEGIN TRANSACTION READ ONLY;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS, FORMAT TEXT)
SELECT j.id, j.status, j.fulfillment_version, j.production_cycle
FROM job AS j
WHERE j.id = '00000000-0000-4000-8000-000000000001';
ROLLBACK;
```

This example measures a primary-key job lookup, not the full fulfillment response. For a meaningful route comparison, measure the exact query and correlated subqueries used by the repository method; retain the plan output with fixture row counts and PostgreSQL version. `EXPLAIN ANALYZE` executes the statement, so use read-only `SELECT` statements only. Do not run arbitrary update/delete statements or change indexes on an existing database as part of a benchmark.

For cumulative query statistics, PostgreSQL's optional `pg_stat_statements` extension must already be enabled by the local instance. On a disposable database where it is available:

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SELECT queryid, calls, round(mean_exec_time::numeric, 3) AS mean_ms,
       round(max_exec_time::numeric, 3) AS max_ms, rows,
       shared_blks_hit, shared_blks_read, left(query, 500) AS query
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
```

Some PostgreSQL builds require `shared_preload_libraries` and a restart before the extension can collect data. Do not alter the existing Compose database or user data to enable it; use a separate disposable instance if configuration is needed. Reset statistics only on that disposable instance and only when a before/after comparison requires it. Record mean/max execution time and buffer hits/reads alongside HTTP p95; a fast SQL plan does not guarantee a fast endpoint.

No universal latency target is set because expected users, data size, workload mix and hosting region remain undecided.
