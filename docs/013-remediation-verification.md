# Remediation verification

## Phase 14 — security baseline and compatibility safeguards

Date: 2026-10-27 (Asia/Bangkok). Baseline: develop at 1a75cef.

### Changes

- Go module/development/CI baseline: 1.27.3. Local base installation was not
  overwritten; Go downloaded the required version into its toolchain cache.
- x/text v0.41.0, with x/sync v0.22.0 required by its module graph. Other direct
  runtime dependencies and Argon2id parameters are unchanged.
- Both builder images: official public.ecr.aws/docker/library/golang:1.27.3-alpine
  at index sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb.
- govulncheck pinned at v1.9.0; CI and scripts/check-security.ps1 scan source
  and the two built binaries. No .env loading or application DB access is
  needed for the security script.
- Production/fulfillment response tests freeze legacy keys, empty arrays,
  exact decimal strings, Worker redaction and full-state command responses.
- Real repository tests compare complete serialized original/replayed states
  after later production completion or payment review, not just version fields.

### Checks actually executed

| Check | Result |
| --- | --- |
| go mod tidy; go build ./... | Pass with Go 1.27.3; exported snapshots dc035d8/53fa00d/1b83205/b8ab9b5 also independently build |
| go vet ./... | Pass |
| go test ./internal/production ./internal/delivery | Pass |
| go test -race -cover ./... with integration DSNs cleared | Pass; DB tests correctly skipped in this unit-only run |
| scripts/check-security.ps1 | Pass: source and Windows API/worker, zero reachable advisories |
| Dockerfile.api and Dockerfile.worker builds | Pass |
| go version -m on actual Linux image binaries | Both Go 1.27.3; x/text v0.41.0 |
| govulncheck -mode=binary on both extracted Linux binaries | Pass: zero reachable advisories |
| scripts/verify-uc9.ps1, isolated sa-lathe-uc9-b06047c3 | Exit 0; real repository tests pass, including new replay equality assertions |
| Migration 1-12, empty 12/11/10 down and up, legacy Worker contacts | Pass in disposable databases |
| Docker authenticated UC1-UC9 flow, inbox worker, graceful stop | UC9_DOCKER_FLOW_PASSED, PHASE12_VERIFICATION_PASSED, WORKER_CONTACTS_VERIFICATION_PASSED |

The scanner also reports advisories in required modules whose vulnerable
packages/symbols are not imported/reachable (20 in the Windows/source scans,
19 in the Linux binary scans at verification time). These are not hidden or
described as confirmed remote exploits. A clean reachability scan does not
prove the absence of all vulnerabilities. Re-run against the current public
vulnerability database before release.

### Safety and limitations

All DB/S3 mutations occurred in freshly created verification resources. The
verifier removed its own containers, network and two volumes after success;
existing data/settings were preserved. Temporary image-inspection containers
never started and were removed. Ignored binaries and verification image tags
remain available locally. No real LINE message, main push or deployment.

No application query optimization or shop API latency claim is part of this
phase. Phase 15 measurement/integration-CI work and phases 16-21 remain pending.
Commit groups: dc035d8 (security baseline), 53fa00d (contracts/replay),
1b83205 (scanner automation), followed by this verification/checklist checkpoint.
The phase checkpoint must be pushed and its remote tip verified before claiming
publication; the agent handoff records the actual verified commit.

## Phase 15 — trustworthy benchmarks and real integration CI

Implemented in separate groups: 79df842 (independent full-response timings and
loopback tests), c83409e (unconditional strict real-integration CI), b71d1cd
(synthetic fixture, robust report parser and measured results), then this checkpoint.

- Loopback harness tests cover out-of-order completion, delayed/truncated bodies,
  timeout, HTTP failures and fragmented Go output; skipped/failed evidence is rejected.
- Fresh strict verifier sa-lathe-uc9-6d466554 exited 0 after all ten required real
  repository suites ran under race/coverage, migration checks, Docker HTTP/inbox
  flow and cleanup. One earlier HTTP flow failed without safe details; added
  line/status diagnostics and a rerun passed. Cause remains unconfirmed.
- Fresh baseline sa-lathe-perf-aa349ddd passed in 460.57 s with the approved
  100-account/1,000-job/1,000-material/100-supplier dataset and 10/100/1,000-entry
  histories, 1/10/25 concurrent reads, 100-identity mix and labelled service-write
  timings. See 012-performance.md for measured values and limitations.
- The first baseline's completed test report was rejected because test2json
  fragments long output. The parser now reassembles it correctly; a new full run
  passed and saved its report. No failed report was claimed as valid evidence.
- Exported b71d1cd independently passed build, vet, default tests and harness tests,
  without any prepared Phase 16 files. GitHub's Linux job is configured but its
  remote execution is not claimed before publication/CI observation.
- Both test projects cleaned their own fresh resources. Reports and snapshot
  archives are local/ignored, no credentials are saved and no existing DB was used.

The baseline confirms round-trip amplification and long-history aggregation;
it does not justify new search indexes at this dataset size or a production SLA.
Phase 16 may be prepared while the fixed compiled baseline runs; publish phase
checkpoints in order. Transaction-step extraction is grouped with Phase 17 to
avoid duplicating the same production rewrite. Phases 16-21 are not yet complete.

## Phase 16 — validation, error handling and shared HTTP support

Groups b20c9e8, 1de7395, b82c6ab and 5e6ce0c each passed independent exported
staged-tree build/focused tests before committing. Full build, vet and race suite
also passed with all test/performance DSNs cleared.

- Unicode/NUL validation protects request description/material spec, fulfillment
  receiver/reasons, names and worker positions. Existing contact/enum/file checks
  were retained; password policy is unchanged.
- Shared JSON/pagination retains auth's 8-KiB and feature 64-KiB limits, strict
  unknown-field/trailing-value rejection and existing public error envelopes.
- Internal failures log request ID, route, safe operation/category and validated
  SQLSTATE alongside the existing duration summary. Injected private errors,
  authorization headers and query content never appear in logs or responses.
- Only pgx.ErrNoRows maps to forbidden/missing in fulfillment authorization;
  timeout/connection causes survive wrapping. Fault-row tests cover both actor
  and membership lookup failures without an additional library or database.

No migration, permission, financial/stock rule, endpoint or external service was
changed. Real DB/S3/container regression is scheduled again with Phase 17.
Transaction-step extraction accompanies that query rewrite, not claimed as done
here. GitHub CLI is unavailable, so remote CI status was not observed. Publication
must be independently verified. Phases 17-21 remain pending.

## Phase 17 — database round trips (verification in progress)

Code groups: 5d1f9d3 (one-statement scoped fulfillment page), 4af3d05 (focused
production validation and one final legacy snapshot), b589b21 (bulk material/
reservation reads and batched issues/ledger), d0308c1 (server-timed test fixture).
No schema/index/pool-size change, removed constraint, cached stock, truncated
legacy response or rewritten historical retry was introduced.

The first workflow gate failed without a retained assertion; no cause is claimed
from that truncated output. A logged rerun passed all ten real repository suites
but caught HTTP 400 at supplier settlement fixture line 244. Its timestamp came
from the host while PostgreSQL enforces receipt_time <= paid_at <= DB_time.
The fixture now uses the server-returned full_received_at. Production validation
is unchanged. Final sa-lathe-uc9-c335c3fb passed all ten suites under race/coverage,
including list/detail equality, scoped access, single-connection reads, exact
replays, multi-material batch rollback, full Docker workflow and cleanup.
Default build/vet/race/harness checks also pass. These are correctness results,
not yet proof that every latency acceptance gate is satisfied.

The after-change synthetic run sa-lathe-perf-ce72311c passed with zero request
failures and records two SQL statements per fulfillment HTTP request for both
25/50-item pages (authentication plus one read), versus the original 152 at 25.
Usage service commands issue 17/35/215 statements for 1/10/100 materials versus
19/73/613. Trigger work still executes; these count client-issued statements.

Do not treat that run's raw timings as a clean before/after comparison: even
unchanged liveness p95 rose from 2.01 to 14.67 ms at concurrency 10, unchanged
search/list paths slowed, and seeding rose from 304.53 to 1030.22 s. Both runs
used the same Docker CPU/memory allocation but other verification work overlapped;
host/load drift is suspected, not precisely attributed. The report's base commit
is 012a4d8 plus the then-uncommitted Phase 17 patch, not unmodified 012a4d8.
Raw reports are retained locally under ignored bin/ for transparency.

A fresh run with no other verification jobs includes a frozen test-only legacy list and
optimized list over the same rows, pool, middleware and authentication. Full
response bytes must match; three repeats alternate legacy/optimized order at
concurrency 1/10/25. It passed as sa-lathe-perf-60ea3b78 in 650.72 s. Paired
legacy/optimized median p95 was 417.14/17.22 ms at concurrency 1,
2782.27/64.52 ms at 10 and 1870.54/201.69 ms at 25, with zero failures.
Aggregate host counters showed 77% CPU and 547 MiB free RAM, so these remain
local loaded-host diagnostics, not a normal-load or production SLA. Query counts
and complete response equality supply independent structural/contract evidence.
See 012-performance.md for timings, caveats and remaining legacy-history costs.
Both benchmark projects and all workflow resources cleaned up their own fresh
containers/volumes. Publication and independent snapshot checks must still be
recorded before the phase is claimed published. Final exported snapshots
5d1f9d3/4af3d05/b589b21/d0308c1/76e7e56 independently built. Vet, default race
and harness checks passed. Final source and Windows API/worker scans report zero
reachable advisories; unused-module reports remain informational. The paired
reference/report is committed in 76e7e56. Verify the checkpoint's remote tip after
pushing. Publication was independently confirmed at origin/develop
373708880dc2ae14154a7f826666b1cd9dced394; this acknowledgement is a focused
documentation follow-up on the same branch. Phases 18-21 remain pending.
