# Phase 11 UC8 verification — PASSED

Verified on 2026-10-19 in disposable project sa-lathe-uc8-a664942c using
./scripts/verify-uc8.ps1. Exit 0; PHASE11_VERIFICATION_PASSED and
WORKER_CONTACTS_VERIFICATION_PASSED were observed, followed by successful cleanup
of all four containers, both fresh volumes and the network. No existing data or
runtime directories were changed, and no real LINE message was sent.

Build, vet, unit/domain/HTTP-guard/photo/privacy tests and ./tasks.ps1 lint pass.
The full verifier explicitly set dedicated database/S3 environment variables and
ran go test ./... -count=1; the real UC7 and UC8 integration suites ran, not skips.

Earlier attempt: ./scripts/verify-uc8.ps1 on 2026-10-19 had a bounded Docker
availability probe failed: engine unavailable or stuck. It stopped before test
services/databases/volumes were created. No existing database was migrated, no
Docker runtime folder was renamed/deleted, and no socket recovery was inferred
from the five production-policy confirmations. UC7's recovery approval/verification
gate was open at that checkpoint. The later successful retry supersedes that gate;
no Docker runtime recovery/rename was needed or performed during this retry.

Retry history: sa-lathe-uc8-7406e12b reached the database and found a migration
000010 receipt-status CASE syntax error. Its fresh resources were cleaned up.
Parenthesizing that PL/pgSQL expression fixed it, then the complete fresh run
sa-lathe-uc8-a664942c passed migrations 1-11, empty 11/10 down/up, legacy Worker
preservation, all earlier regressions and the Docker API/worker workflow.

## Commands

```powershell
go build ./...
go vet ./...
./tasks.ps1 lint
./scripts/verify-uc8.ps1 # includes go test ./... -count=1 against real PostgreSQL/S3
```

The full verifier ran against nine fresh dedicated databases with migrations
1–11, empty migration 11 and 10 down/up, legacy Worker contacts, all earlier real
PostgreSQL/S3 regressions, API/worker Docker builds and customer-to-production HTTP
flow. It uses a random sa-lathe-uc8-* project, empty temporary Compose environment,
random process-only credentials, disabled real LINE and scoped cleanup/restore.
Existing volumes, .env, settings and data are not targets.

## Passed integration checks

- Real UC1–UC4/private S3 flows create valid production jobs.
- Empty/partial plans cannot start; a full current plan starts without stock use.
  Started plans cannot be edited through UC5.
- Own-reservation-first usage, extra/unplanned free stock, no theft of another
  job's reservation, exact per-issue valuation and private Admin actual cost.
- Manual minute totals, completion without required photos, unused-reservation
  release without increasing physical stock and versioned labor-cost revisions.
- Concurrent same-key issue is one ledger entry; original replay after completion;
  changed input conflicts. Usage-versus-completion has one winner.
- Two production jobs competing for free stock preserve the other's reservation.
- Forced event failure after issue restores usage, stock, reserved totals, version
  and audit. Money overflow and immutable history writes fail.
- Valid private JPEG/PNG path, staff download/customer denial, one-connection read,
  HTTP financial redaction/role guards and disabled Worker denial.
- Populated migration rollback refusal and reserved-cache reconciliation.
- UC5's old raw test-only status update is replaced with a real ready-plan/start
  command because the new job lifecycle guard intentionally rejects raw transitions.
- Docker UC7 mixed receipt/replacement/full settlement followed by UC8 reserve,
  start, incremental use, time, release/completion and Admin actual-cost entry.

UC7 and UC8 implementation verification is complete. No production database was
migrated or deployment performed. UC9 remains unapproved. Performance/load targets
are still unspecified; these functional/concurrency checks are not a latency SLA.
