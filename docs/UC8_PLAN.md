# Phase 11 — UC8 production, actual usage and completion

Authorized to begin 2026-10-19, planning checkpoint at 9e46118. All five policies
are confirmed and production implementation is in progress, not yet verified.
UC7 remains in progress: its code/build/vet/unit/lint checks exist, but real
PostgreSQL/S3/Docker verification is pending the documented runtime recovery
approval. UC8 build/vet/unit/lint pass; real integration tests have not run.
No Docker folders/processes, production stock or existing database are changed.
Do not mark UC7 or UC8 complete, deploy dependent stock workflows or start UC9.

## Sources and existing contracts

Reviewed AGENTS.md, PLAN.md, UC7_VERIFICATION.md, UC5.md, backend guide UC8 and
stock/status sections, job migration 000007, material domain and plan repository.
The original detailed Word/sequence files remain unavailable as already recorded.

The guide says Worker starts/continues production, records actual material usage,
photos and actual cost/time, then completes the job. UC9 customer returns may
resume production later, but UC9 return/delivery rules are not authorized here.

Current job statuses: queued, in_progress, completed, cancelled. Current UC5
requirements/reservations are separate from actual consumption. ready means a
nonempty plan without shortages, not permission to silently consume the plan.
Queued plan writes already lock request -> job -> sorted materials and are frozen
once production starts. Current reservation triggers allow only queued changes;
UC8 must extend them safely for issue/release, not bypass or disable guards.

Physical on-hand, active reserved balance, usage ledger and job status/history
must remain consistent in one transaction. Quantity/per-unit cost precision is
NUMERIC(18,6); exact arithmetic only. Preserve reservations belonging to other
jobs, audit actor/time and original command/results, and protect against duplicate
issues and competing production/plan/stock commands. No automatic material use.

Supplier/inventory financial fields remain Admin-only per UC7 confirmation.
Workers need operational quantities and time, not purchase prices/valuation.
Customer quotation pricing under UC2 is unchanged. Job photos must be private
S3 objects with keys/metadata and authorized links, separate from customer request
drawings and Admin-only supplier payment evidence. Reuse file validation/policies.

## Owner confirmations received

The owner answered yes to all five production proposals. Any active Worker may
start/log/complete any job; Admin monitors and enters actual labor cost, not
production actions. Require a nonempty fully reserved plan and current version.
Starting does not consume stock. Explicit actual incremental use consumes own
reservation first, then free stock; extra/unplanned materials are allowed without
using other jobs' reservations. Complete releases leftovers, photos optional.
Workers enter actual whole labor minutes; material valuation uses issue-time cost,
and Admin enters labor cost without an assumed rate. See UC8.md.

### Historical questions (resolved by the five confirmations)

1. Permissions/scope: keep production mutations Worker-only for any active Worker,
   without assignment restrictions, and Admin can monitor/read costs? Existing
   job plans have this Worker-write/Admin-read pattern; do not invent assignment
   or Admin write permissions without confirmation.
2. Starting: require a nonempty, fully reserved plan before queued -> in_progress?
   Starting changes status but does not consume stock. Clarify if partial-start
   or material-free jobs are needed; these are not defined by the guide.
3. Usage: proposed explicit incremental entries deduct physical stock when logged,
   consuming the job's own reservation first. Earlier owner direction permits
   extra use if stock remains. Confirm extra or previously unplanned material may
   use only genuinely free stock, never another job's reservation. Preserve cost
   at each actual issue, not future average changes or quoted estimated cost.
4. Completion: proposed release all remaining unconsumed reservations atomically
   and mark job completed. They were never physically consumed, so release must
   not add them to on-hand a second time. Are completion photos required or optional?
   No quantity auto-fill or invented full-plan consumption is allowed.
5. Actual time/cost: proposed Worker enters actual labor minutes explicitly,
   material cost derives from issue-time valuation and is Admin-only, and Admin
   enters actual labor cost rather than an assumed hourly rate. Confirm this model
   or provide the intended clock/time/rate rules. Do not fabricate labor rates,
   edit history or automatically equate elapsed wall time with actual work.

The confirmations above supersede the proposals. No rejection/rework, cancellation after
consumption, inventory returns, adjustments to historical usage, photo-sharing
policy, supplier message or automatic scheduling is implied. Resolve any need for
these exceptions explicitly; UC9 return handling belongs to that later phase.

## Planned implementation groups after confirmation

1. Confirmed lifecycle/usage/time/cost domain and mock tests, with Worker-safe DTOs.
2. Versioned production audit, actual material use/valuation/time and private photo
   metadata schema, with protective rollback and restrictive references.
3. Repository-only SQL: request/job/sorted-material row locks, exact issue valuation,
   own-reservation-first usage, free-stock checks, idempotency and atomic completion.
4. Authenticated job detail/start/usage/time/photo/completion endpoints and only
   confirmed Admin actual-cost entry; retain the existing Go/chi layered structure.
5. Service/HTTP tests plus real PostgreSQL concurrency and forced failure rollback;
   private S3 integrity/permissions, migrations and full UC1-UC8 Docker regression.
6. Document API and verification evidence, update PLAN, commit logical buildable
   groups while working and push develop only. Verify the remote; never main.

## Verification dependency

Existing Docker socket recovery approval is still outstanding. Do not rename,
delete, reset or repair runtime folders solely because the owner said next phase.
Any approved recovery must preserve images/volumes/settings/databases and retained
socket artifacts. UC7's unexecuted migrations/integration and every earlier stock
flow must actually pass before claiming UC8 end-to-end readiness.
