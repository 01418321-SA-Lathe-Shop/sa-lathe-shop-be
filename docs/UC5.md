# UC5 — Material requirements, stock and reservations

> UC7 permission clarification (2026-10-19): Worker HTTP inventory search now
> omits avg_cost; Worker movement reads omit costs, private payload and free-form
> stock-adjustment reasons. Quantities/plans remain accessible. Admin retains
> financial fields. This prevents the existing endpoints bypassing Admin-only
> supplier finance. No reservation/business calculation changes; see UC7.md.

Apply migration 000008 before deploying the API. Existing UC4 jobs receive
material_plan_version=0; no stock or historical material usage is fabricated.
Keep the separate worker running for the existing UC4/notification functions.

## Permissions and catalog

- Admin creates units/materials, renames or deactivates/reactivates materials,
  and records stock additions/count corrections with a mandatory reason.
- Workers search/read inventory and manage queued-job material plans/reservations.
  Admin can inspect plans but does not inherit the Worker write role.
- Customers and Suppliers cannot read shop inventory, costs or material plans.
- Each material has one immutable base unit. Unit names are created/listed;
  no conversion factors, unit replacement, unit deletion or destructive catalog
  deletion are exposed. Use a new descriptive material when the base unit differs.
- Names are trimmed, Unicode, max 50 characters for units/200 for materials;
  names are unique case-insensitively within each catalog. Inactive materials
  remain visible for audit and release, but cannot be newly planned or added to.

## API

> UC8 compatibility: plan lines now include consumed reservation quantity. For
> in-progress jobs shortage excludes actual reservation consumption; ready is
> queued-only. Completed jobs do not request replenishment for released leftovers.
> Use /production/jobs/{id} for actual use and released quantities. The current
> API therefore needs migration 000011; full UC7/UC8 integration remains pending.

All routes require Bearer authentication; successful commands return 200 JSON.
Unknown fields, multiple JSON values and floating-point numeric quantities are
rejected. No user-supplied actor ID, balance, average cost or reservation total.

| Method | Path | Behavior |
|---|---|---|
| POST | /units | Admin: create `{ "name":"kg" }` |
| GET | /units | Staff: list units, up to 1000 |
| POST | /materials | Admin: create `{ "name":"S45C steel", "unit_id":"uuid" }` |
| PATCH | /materials/{id} | Admin: name and/or is_active, plus expected_version |
| GET | /materials?search=S45C&limit=50&offset=0 | Staff: live catalog/stock search |
| POST | /materials/{id}/stock | Admin: addition or absolute count correction |
| GET | /materials/{id}/movements?limit=50&offset=0 | Staff: movement audit |
| PUT | /jobs/{job_id}/materials | Worker: replace plan and allocate available stock |
| POST | /jobs/{job_id}/materials/reserve | Worker: top up saved requirements |
| GET | /jobs/{job_id}/materials | Staff: current plan/reservations/shortages |

Search is a case-insensitive literal substring of material name, not SQL wildcard
or fuzzy search. Pagination: limit 1-100, offset >=0. Material responses contain
id, name, unit_id/unit_name, is_active, version, qty_on_hand, qty_reserved,
qty_available and avg_cost. No application stock/status cache is used.

## Exact quantities and valuation

Quantity and per-base-unit cost use NUMERIC(18,6), range 0 through
999999999999.999999. Send decimal strings, at most six fractional digits; excess
precision is rejected, never silently rounded. Responses use six decimal places.
These per-unit costs are separate from the existing two-decimal quotation money.

Stock addition body:

```json
{"idempotency_key":"generated-uuid","expected_version":1,"mode":"addition","quantity":"10.500000","unit_cost":"20.000000","reason":"Opening stock count"}
```

Replace placeholder UUIDs with real values. Addition quantity must be positive;
unit_cost is required and may be zero. Average cost is recalculated as:

`(old_on_hand * old_avg_cost + added_qty * unit_cost) / (old_on_hand + added_qty)`

Use exact integer/big-integer arithmetic; round the resulting average half-up to
six decimals. PostgreSQL independently checks the same effect. No float or VAT.
Use additions with actual cost for priced opening stock, not an unpriced correction.

Count correction body uses mode=correction and quantity is the absolute counted
on-hand balance, including zero. Omit unit_cost; the current average cost is
retained, including when correcting the balance to zero. Corrections cannot go
below total reserved stock. This endpoint is not supplier receiving or payment.

Stock movements record actor, reason, normalized command, idempotency key,
before/after quantity and cost, original expected_version, resulting version
and timestamp. The ledger and physical balance change are atomic and immutable.
Catalog changes also record before/after name/activity, actor, version and time.

## Save, release and reserve

Worker saves the full desired material list for a queued job:

```json
{"idempotency_key":"generated-uuid","expected_version":0,"lines":[{"material_id":"material-uuid","quantity":"12.000000"}]}
```

Expected version is the job's material-plan version, initially 0, not a material's
stock version (initially 1). Each line has a positive quantity; duplicate material
IDs are rejected. Maximum 500 lines per command, within the normal request size
limit. Omitted/null lines are invalid to prevent accidental clearing. Explicit
`"lines":[]` releases the whole plan; omitting a previous material releases it.

For each material:

- Available = on_hand - all active reservations.
- New job reservation = min(required, previous job reservation + available).
- Shortage = required - new job reservation, never hidden by partial allocation.
- Reductions release only the excess. Increases reserve only what remains free.
- Reserving/releasing NEVER changes physical on-hand quantity or average cost.

The response contains job_id, resulting version, ready and lines with material_id,
required, reserved and shortage. Ready means a nonempty plan has no shortages;
it does not start production or mark a job in progress. Empty plans are not ready.
Requirements, reservations and actual consumption remain separate facts.

After new stock becomes available, explicitly recheck the saved plan:

```json
{"idempotency_key":"new-generated-uuid","expected_version":1}
```

POST this to /jobs/{id}/materials/reserve. It reads stored requirements, does not
trust client replacement lines, and preserves prior reservations without double
counting. Stock addition does not automatically reorder/allocate across waiting
jobs; no unconfirmed production priority policy is inferred.

Only queued jobs with ongoing requests accept new plan edits/rechecks. Once
production starts, these commands return 409; UC8 will handle usage/reconciliation.
Existing inactive material requirements may be retained or reduced/released,
but cannot be increased or topped up until the material is reactivated.

## Concurrency, retries and errors

Plan transactions lock request, job, then all affected material rows in sorted
UUID order. Stock commands lock the material. DB triggers maintain the reserved
aggregate in the same transaction and reject over-reservation/bare balance writes.
Optimistic versions prevent stale overwrites; a failed audit rolls back changes.

An idempotency key is scoped to a material for stock commands and a job for plan
commands. Exact same actor/canonical payload replays the original result, even
after newer commands or job status changes; a changed actor/payload gets 409.
Plan line order and decimal formatting normalize before comparison. Replays do
not reallocate stock or return today's inventory: GET provides current state.
Use a new key and the latest expected_version for an intentional new operation.

401: absent/revoked session; 403: wrong role/inactive account; 404: missing job or
material; 400: invalid body, precision or reference; 409: duplicate name/command,
stale version, inactive allocation, non-queued job or below-reservation correction.
Real DB constraints provide additional guards; private SQL/errors are not exposed.

## Persistence and boundaries

Migration 8 adds unit, material, material_catalog_audit, stock_movement,
job_material_requirement, material_reservation and job_material_command.
Removed plan lines retain zero-quantity identity; released reservations remain
auditable. Commands preserve complete actor/payload/result snapshots. There is
no actual-consumption table or production mutation in UC5.

Rollback refuses once inventory/catalog/command data exists. Back up and use a
planned forward migration; never force/drop populated inventory data. No previous
user database was migrated during tests. Apply through 000008 before deployment.

Supplier orders/receiving/payment/defects, unit conversions, automatic scheduling,
production usage and new outbound notification events are not implemented here.
Future load targets and large-catalog search tuning require measurement; no
unmeasured latency/throughput benchmark is claimed.

```powershell
./tasks.ps1 migrate-up
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
./scripts/verify-uc5.ps1
```

The verifier uses fresh isolated databases and disposable Docker/S3 resources,
disables real LINE delivery and cleans only its own resources. See UC5_VERIFICATION.md.
