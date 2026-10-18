# Phase 9 — UC6 supplier material orders

Authorized 2026-10-18. Inspected clean develop at 61984c3. Implemented and verified;
UC7-UC9 remain unapproved. Preserve Go/chi, PostgreSQL, exact decimal
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

## Owner-confirmed replacement for the earlier proposals

1. Admin only: create/manage supplier masters, place/read/manage orders and record
   responses/cancellations/history. Workers retain UC5, not procurement access.
2. Option A, flexible job references: one supplier, multiple lines and zero/many
   job links. Allow over-ordering/general replenishment. No fixed job allocations,
   shortage caps, incoming coverage or blocking Worker plan changes.
3. Admin records the external supplier's acceptance/rejection. Rejection cancels
   the entire unreceived order with mandatory reason/audit. No Supplier login or
   automatic supplier email/LINE delivery is assumed.
4. Placed quantities/prices/references are immutable; cancel/recreate to change.
   A recorded supplier response cannot be replaced by a second response; use
   explicit cancellation if an accepted commitment must change. UC7 must extend
   receipt states and atomically prohibit cancellation after any receipt using
   the same order lock. There is no receiving endpoint in UC6.
5. Exact quantity * unit_cost, half-up each line to two decimals, sum rounded
   lines. THB/no VAT. Reject overflow; keep six-decimal input quantities/costs.

These policies replace the old Worker/single-job/shortage-only proposals. Supplier master name is required;
contacts can be optional and validated if supplied, without inferring uniqueness
or account ownership. Do not invent actual purchasing prices or receipt dates.

## Implemented groups after confirmation

1. Confirmed domain/roles, exact line/order pricing, canonical commands and tests.
2. Supplier master, immutable placed orders/items and actor/time/reason audits;
   schema supports later UC7 without pretending receiving/payment exists now.
3. Stable idempotency/version guards and transactional cancel. Flexible references
   never mutate reservations, on-hand stock, average costs or UC5 plan policy.
4. Supplier/order/status/history APIs and only the approved UC5 integration;
   no production transition, stock addition or new outbound notification event.
5. Real PostgreSQL duplicate-command/response races, multi-job excess orders,
   rollback, permissions, migrations and disposable Docker flow with full earlier
   regression. Disable real LINE and preserve existing databases/user data.
6. API/verification docs and PLAN completion; focused independently buildable
   commits, push to develop and verify remote. Stop before UC7; no main push.
