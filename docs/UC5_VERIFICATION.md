# Phase 8 UC5 verification — PASS

Verified 2026-10-18 (Asia/Bangkok). Initial full isolated run:
sa-lathe-uc5-cf5d13cb. Expanded final run: sa-lathe-uc5-1b78985e. Both completed
with exit 0, PHASE8_VERIFICATION_PASSED and WORKER_CONTACTS_VERIFICATION_PASSED.

## Commands

```powershell
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
./scripts/verify-uc5.ps1
```

The script applies migrations 1-8 in six fresh dedicated test databases, checks
empty migration 8 down/up, preserves the earlier legacy Worker contact fixture,
runs uncached real PostgreSQL/S3 integration with all previous staff/UC1-UC4
regressions, builds/runs API and worker in Docker and gracefully stops them.
Real LINE delivery is disabled. Additional service/mocking tests also passed.

## Verified inventory behavior

- Exact decimal parsing rejects negatives, excess precision/exponents/NaN and
  out-of-range values. Half-up weighted-average calculations agree with PostgreSQL;
  large products use big integers, not floats/overflow-prone integer products.
- Admin catalog creation, case-insensitive unit uniqueness and literal substring
  material search work; unknown unit-change JSON fields are rejected.
- Concurrent identical stock additions create one movement and one balance change.
  Changed-key payload retries conflict; count corrections preserve average cost.
- Two competing queued jobs requesting 15 each against 20 stock reserve only 20
  in total, without changing physical on-hand quantity.
- Reducing one plan releases excess; another job's explicit recheck can top up.
  Replays do not reserve twice. Explicit clear releases the whole plan.
- Expanded requirements partially reserve availability and expose the remainder.
- Opposite input ordering across multi-material concurrent plans does not deadlock;
  fractional stock is not over-reserved.
- Concurrent exact same-job plan retries create one command/version/allocation.
  Service canonicalization does not mutate the caller's shared input slice.
- Stale plan versions cannot overwrite reservations. Corrections below active
  reservations fail without creating ledger entries or changing balances.
- Forced plan-audit failure rolls back requirements, reservations, material totals
  and job version. Forced movement-ledger failure rolls back stock and costing.
- Test-only transition to in_progress freezes edits/rechecks; no production endpoint
  is introduced. Inactive material cannot be newly planned, but may be released.
- Bare physical stock updates and movement-audit edits are rejected. Stored
  reserved aggregates reconcile exactly with reservation rows.
- Populated migration rollback refuses before destroying inventory. Reads work
  with a one-connection pool.
- Real HTTP checks sessions, Customer stock denial, Worker catalog/stock denial,
  Admin plan-write denial, search, plan reads and stale inactive Worker denial.
- Docker flow creates a unit/material, adds stock, partially reserves 10 of 12,
  reports shortage 2, replays safely, then releases stock without consuming it.
- Existing private S3, customer/company, staff/Worker-contact, pricing, quotation,
  evidence/review/expiry and inbox worker regressions pass.

## Safety and boundaries

Only each verification project's newly-created containers, volumes and network
were removed afterward. No existing user database or volume was migrated/deleted.
Test credentials were random process-only values, restored afterward, never
committed. Test jobs are created through real UC1-UC4/private S3 flows. A direct
test-only job status change exercises the UC5 production lock without claiming UC8.

Apply migration 000008 before deploying the current API. No supplier order/receipt,
payment, unit conversion, production consumption, automatic scheduling or new LINE
event is implemented here. No unmeasured throughput/latency claim is made.
Develop-only commit/push acknowledgement is recorded in PLAN.md.
