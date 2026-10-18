# Phase 8 — UC5 stock checks and reservations

Owner authorized 2026-10-18. Clean develop and origin/develop at 0381004; main
86d5be1. Implemented and verified; publication is tracked in PLAN.md. UC6-UC9 remain unapproved. Keep Go/chi and the
Handler -> Service -> Repository boundary; PostgreSQL row locks, no cache/Redis.

## Source review

AGENTS.md, PLAN.md, requirements baseline, docs/01-backend.md UC5 and explicit
open questions, original ER stock/material/unit/reservation structures, UC4 API
and migration 000007 queued-job schema. Original Word/sequence files remain
unavailable; do not invent missing business rules from unrelated documents.

## Settled requirements

- Separate needed quantities, reserved quantities and actual material usage.
- Reserve available quantity even when insufficient; expose only the remaining
  job/material shortage for future UC6 procurement, without placing orders now.
- Available stock = physical on-hand minus all active reservations. A reservation
  is not consumption and must not reduce on-hand quantity. No negative stock or
  over-reservation under concurrent jobs, retries or plan changes.
- Lock affected rows inside transactions. Any physical balance change and stock
  movement/audit must commit together. No floating-point money or quantities.
- Do not start production or implement consumption, supplier receiving, defects,
  settlement, moving-average receiving or job completion in this phase.

## Owner-confirmed decisions

1. Persist an editable material requirement list per queued job, entered by a
   Worker, rather than ephemeral stock-check input. Owner confirmed; allocate as
   much as available, and show shortage = max(required - this job's reserved,0).
   Rechecks preserve prior reservations rather than counting them twice.
2. Admin maintains materials/units and initial/manual stock balances,
   with a mandatory reason and immutable adjustment audit;
   Workers search/read stock and manage job requirements/reservations only.
   Physical stock corrections may not go below active reservations. Supplier
   receiving and purchasing remain UC6/UC7, not hidden manual receiving here.
3. Workers may edit/release reservations while a job is queued:
   decreasing/removing a requirement releases the excess, increasing it reserves
   only available stock and reports shortage. No edits after production begins;
   actual usage/reconciliation is UC8. No silent job-status transition to production.

4. Owner also confirmed: stock additions require quantity/unit cost and recalculate
   weighted-average cost; count corrections retain the current average cost.

The owner replied yes to all four policies. Exact quantities and unit costs use
NUMERIC(18,6); quantities are never rounded silently. Weighted averages round
half-up to six decimal places. The Admin inventory permissions above are explicit,
not inferred from Worker-account management. Base-unit conversion is not in scope.

## Implementation groups after confirmation

1. Exact quantity parsing and pure allocation/shortage domain, validation and mocks.
   Propose base-unit decimal quantities; document technical precision, never
   round user quantities silently or guess conversion factors.
2. Material/unit catalog, persistent requirement/reservation and append-only audit
   schema; non-negative balances, unique job/material plan lines and safe FKs.
3. Request/job/material transaction locking in stable order; optimistic plan
   versions and idempotency; no physical stock decrement when reserving/releasing.
4. Authorized search/catalog and job plan/check/reserve/read APIs, plus only those
   Admin stock commands the owner approves. Customers cannot inspect shop stock.
5. Real PostgreSQL competing-job races, partial availability, release/edit/retry,
   rollback and authorization tests. Migration up/down and disposable Docker flow
   plus staff/Worker-contact/UC1-UC4 regression. No real LINE sends.
6. API guide, verification record, PLAN completion and separate buildable commits;
   push completed phase to develop, verify remote, stop before UC6. No main push.

## Result

Initial full verification passed in sa-lathe-uc5-cf5d13cb. Expanded final run
sa-lathe-uc5-1b78985e passed with PHASE8_VERIFICATION_PASSED, including identical
same-job retry races and non-mutating service canonicalization. Both projects'
fresh containers, volumes and networks were removed; no existing database or
real LINE recipient was touched. See UC5.md and UC5_VERIFICATION.md.
