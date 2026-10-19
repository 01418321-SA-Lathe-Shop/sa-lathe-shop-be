# Phase 11 UC8 verification — IN PROGRESS

Build, vet, unit/domain/HTTP-guard/photo/privacy tests and ./tasks.ps1 lint pass.
Ordinary go test ./... passes; database integration is skipped without dedicated
environment variables, so this is NOT evidence of real SQL/S3/Docker success.

Attempted ./scripts/verify-uc8.ps1 on 2026-10-19. Its bounded 15-second Docker
availability probe failed: engine unavailable or stuck. It stopped before test
services/databases/volumes were created. No existing database was migrated, no
Docker runtime folder was renamed/deleted, and no socket recovery was inferred
from the five production-policy confirmations. UC7's recovery approval/verification
gate remains open. Do not deploy or mark either phase complete.

## Commands

```powershell
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
./scripts/verify-uc8.ps1
```

The full verifier is prepared for nine fresh dedicated databases with migrations
1–11, empty migration 11 and 10 down/up, legacy Worker contacts, all earlier real
PostgreSQL/S3 regressions, API/worker Docker builds and customer-to-production HTTP
flow. It uses a random sa-lathe-uc8-* project, empty temporary Compose environment,
random process-only credentials, disabled real LINE and scoped cleanup/restore.
Existing volumes, .env, settings and data are not targets.

## Added integration checks — NOT YET EXECUTED

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

Do not claim PHASE11_VERIFICATION_PASSED or resolve UC7's gate until the complete
real verifier runs and scoped cleanup is observed. No new runtime repair authority
or UC9 implementation is granted by these test descriptions.
