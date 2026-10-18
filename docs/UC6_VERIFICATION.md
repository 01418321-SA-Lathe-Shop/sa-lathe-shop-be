# Phase 9 UC6 verification — PASS

Initial full isolated run sa-lathe-uc6-85f10f70 passed with exit 0,
PHASE9_VERIFICATION_PASSED and WORKER_CONTACTS_VERIFICATION_PASSED. Its fresh
containers, volumes and network were removed. Final expanded run
sa-lathe-uc6-bd002774 also completed with exit 0 and both pass markers, then its
fresh containers/volumes/network were removed. Verified 2026-10-18
(Asia/Bangkok). No existing database, local .env or real LINE recipient was used.

## Commands

```powershell
go build ./...
go vet ./...
go test ./internal/procurement ./internal/repository/postgres
./tasks.ps1 lint
./scripts/verify-uc6.ps1
```

The full script applies migrations 1-9 to seven fresh dedicated databases, checks
empty migration 9 down/up and preserves the earlier legacy Worker contact fixture.
It runs uncached real PostgreSQL/S3 integration for all prior staff/UC1-UC5 flows,
builds/runs API and worker in Docker and gracefully stops/cleans those resources.
Credentials are private random process values restored afterward, not committed.

## Coverage

- Exact half-up line pricing, sum-of-rounded-lines, sub-satang boundaries, zero
  quantity rejection, product overflow and total-only overflow; no float math.
- Pure service/mocking checks Admin-only operations, contact validation, sorted
  canonical commands, untouched caller slices and no-job general replenishment.
- Real private S3/UC1-UC4 flows create two valid queued jobs for consolidated orders.
- Concurrent identical placements yield one order/event; changed payload with the
  same retry key conflicts. New keys intentionally allow additional replenishment.
- Multi-job lines exceeding all shortages work. Placed orders never block Worker
  requirement edits/releases. Procurement leaves physical/reserved stock and
  average cost unchanged.
- Supplier/material master renames preserve original order contact/name snapshots.
  Inactive suppliers/materials block new placement. Historical order reads and
  original retries remain available to an active Admin.
- Recorded acceptance leaves the order ordered, not received. Rejection cancels
  with reason. Explicit cancellation after acceptance retains original response.
  Exact retries return original snapshots after later state changes.
- Opposite concurrent responses at one version produce one winner/one conflict
  and one resulting event. Per-job order filtering resolves many-job references.
- Failed placement rolls back child rows/audit and leaves its retry key reusable.
  Forced event insertion failure restores the previous header/version/status.
- SQL rejects header-total edits, line edits, job-link deletion/late insertion and
  audit rewriting. A forged header total fails its deferred commit check and leaves
  no orphan item. Populated migration rollback refuses before dropping data.
- One-connection order-list reads complete without pool recursion/deadlock.
- Real HTTP sessions deny all procurement reads/writes to Worker/Customer, reject
  unauthenticated access, stale supplier versions, invalid contacts/unknown fields,
  malformed pagination and invalid order transitions. Disabled Admin sessions and
  stale direct-service principals cannot access procurement.
- Docker HTTP creates/searches a supplier, places/replays an excess order, records
  acceptance, edits/releases a linked Worker plan, cancels and reads history, then
  verifies unchanged physical/reserved stock and average cost.
- Earlier auth, Worker contacts, company membership, private file permissions,
  quotation pricing/decisions, evidence/expiry, inventory races and inbox worker
  regressions pass. No new real LINE message or supplier integration was exercised.

## Boundaries

Only UC6 procurement is implemented. There is no receiving/payment endpoint,
incoming allocation, consumption or supplier delivery automation. UC7 must extend
receipt states and use the same order lock to reject cancellation after any receipt.
No performance/throughput/production security certification is claimed from these
functional tests. Migration 000009 must be applied before the current API.

Publication is develop-only; PLAN.md records the verified remote acknowledgement.
