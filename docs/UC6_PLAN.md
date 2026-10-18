# Phase 9 — UC6 supplier material orders

Authorized 2026-10-18. Inspected clean develop at 61984c3. Phase started, not
completed; UC7-UC9 remain unapproved. Preserve Go/chi, PostgreSQL, exact decimal
arithmetic, Handler -> Service -> Repository and develop-only publication.

## Sources and settled scope

Read AGENTS.md, PLAN.md, requirements baseline, docs/01-backend.md UC6 and its
explicit role/master-data questions, original supplier/material-order ER, and
UC5.md shortage/reservation/stock contract. Detailed Word/sequence files remain
unavailable; do not invent missing rules from unrelated documents.

Supplier has no login. A supplier order has one supplier and several material
lines. Existing quantities/unit costs use NUMERIC(18,6), money never floats, no
VAT. The request's own customer PO evidence is not the supplier purchase order.
Orders and supplier rejection must not change physical stock or average cost.
Receiving, damage claims and supplier payment belong to UC7, not this phase.

## Policies to confirm before affected implementation

1. Ordering permissions: guide says Worker, earlier owner summary says Admin.
   Proposed both active Admin and Worker may create/read orders and record
   responses/cancel eligible orders; Admin alone creates/edits/deactivates supplier
   master records. Customers cannot inspect supplier/procurement information.
2. Job linkage/quantity: proposed one queued job per order (one supplier, multiple
   material lines); order only the uncovered shortage for that job/material:
   max(required - reserved - unreceived quantities on existing live orders,0).
   Allow splitting the shortage across suppliers/orders, but no excess general
   replenishment or cross-job consolidated order in this first version.
3. Supplier response: staff contacts the external supplier outside this API and
   records acceptance/rejection. Proposed rejection cancels the entire unreceived
   order with mandatory reason and audit, releasing incoming-order coverage.
   No Supplier account, auto-email/LINE delivery or assumed supplier confirmation.
4. Editing/reduction: proposed immutable placed-order quantities/prices; cancel
   and recreate an unreceived order to change them. Block plan reductions/top-ups
   that would leave existing open order coverage greater than remaining shortage
   until those orders are canceled. Never silently cancel supplier commitments.
   Cancel only before any receipt; partial-receipt exceptions are UC7 questions.
5. Totals: proposed exact quantity * unit_cost, round each line half-up to one
   satang, then sum the rounded lines for NUMERIC(18,2) order total (THB, no VAT).
   Reject overflows; retain six-decimal unit costs/quantities without rounding input.

These are proposals, not confirmed requirements. Supplier master name is required;
contacts can be optional and validated if supplied, without inferring uniqueness
or account ownership. Do not invent actual purchasing prices or receipt dates.

## Planned implementation groups after confirmation

1. Confirmed domain/roles, exact line/order pricing, canonical commands and tests.
2. Supplier master, immutable placed orders/items and actor/time/reason audits;
   schema supports later UC7 without pretending receiving/payment exists now.
3. Request/job/material lock ordering compatible with UC5, outstanding-order
   coverage checks, stable idempotency/version guards and transactional cancel.
4. Supplier/order/status/history APIs and only the approved UC5 integration;
   no production transition, stock addition or new outbound notification event.
5. Real PostgreSQL duplicate-order/plan-vs-order races, split orders/rejections,
   rollback, permissions, migrations and disposable Docker flow with full earlier
   regression. Disable real LINE and preserve existing databases/user data.
6. API/verification docs and PLAN completion; focused independently buildable
   commits, push to develop and verify remote. Stop before UC7; no main push.
