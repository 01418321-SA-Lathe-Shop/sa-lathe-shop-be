# Phase 8 — UC5 stock checks and reservations

Owner authorized 2026-10-18. Clean develop and origin/develop at 0381004; main
86d5be1. Started, not completed. UC6-UC9 remain unapproved. Keep Go/chi and the
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

## Decisions to confirm

1. Persist an editable material requirement list per queued job, entered by a
   Worker, rather than ephemeral stock-check input? Proposed yes; allocate as
   much as available, and show shortage = max(required - this job's reserved,0).
   Rechecks preserve prior reservations rather than counting them twice.
2. Who maintains materials/units and initial/manual stock balances? Proposed
   Admin manages these, with a mandatory reason and immutable adjustment audit;
   Workers search/read stock and manage job requirements/reservations only.
   Physical stock corrections may not go below active reservations. Supplier
   receiving and purchasing remain UC6/UC7, not hidden manual receiving here.
3. May Workers edit/release reservations while a job is queued? Proposed yes:
   decreasing/removing a requirement releases the excess, increasing it reserves
   only available stock and reports shortage. No edits after production begins;
   actual usage/reconciliation is UC8. No silent job-status transition to production.

These proposals are not confirmed yet. Admin's current ability to manage Worker
accounts does not imply unlimited inventory-write permissions. Any required
valuation/opening-cost policy must be explicit before implementing stock writes.

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
