# Phase 5 — UC2 estimation and quotations (in progress)

Authorized 2026-10-15 on develop. Sources: docs/01-backend.md, requirements.md,
existing UC1 code and direct owner decisions. Keep Go/chi, PostgreSQL NUMERIC,
Handler -> Service -> Repository, repository interfaces and existing private auth.

## Confirmed pricing decisions

Owner selected total-cost markup: `(material_cost + labor_cost) * 1.60`.
All amounts are totals for the entire requested quantity, not unit prices.
THB amounts have two decimal places; round the minimum upward to a satang.
The alternatives below are historical options, not remaining blockers.

The owner confirmed a 60% profit target, but not its calculation basis. Do not
select any of these incompatible minimum-price rules without confirmation:

- Total-cost markup: `(material_cost + labor_cost) * 1.60`.
- Material-cost markup: `material_cost * 1.60 + labor_cost`.
- Selling-price margin: `(material_cost + labor_cost) / 0.40`.

Also confirm whether material_cost/labor_cost/final_price are whole-request
totals covering qty, or per-unit values. The ER has qty on JOB_REQUEST but does
not establish the quotation money basis. Precision/rounding was left open in
the guide: proposed THB values to two decimal places, exact arithmetic, minimum
price rounded upward to a satang so rounding cannot undercut the minimum.

## Planned implementation

1. Exact amount parsing and confirmed pricing rules, with tests for zero,
   invalid/negative input, decimal precision, rounding and minimum thresholds.
2. Quotation and cost tables linked to requests; revision chain/history,
   author/timestamps and constraints. Serialize updates per request. Draft save
   and sending update quotation/cost/request atomically; sending sets ongoing.
3. Worker APIs for saving/editing drafts, sending and rejecting a request with
   a reason. Drafts/internal costs stay staff-only; sent customer-facing prices
   are readable only through existing company membership rules. Sent price
   snapshots must not be silently overwritten.
4. HTTP validation/error mapping, authorized routes and tests. Any new outbound
   notification event/recipient contract must be agreed before implementation;
   do not repurpose UC1 new-request events to claim quotations were delivered.
5. Real PostgreSQL concurrency/transaction tests, existing-phase regressions,
   migrations and Docker checks; document API usage and operational limitations.

## Commit groups

- Scope/decision tracking (this documentation group).
- Pricing domain + focused tests.
- Schema/repository + transactional tests.
- Routes/authorization + HTTP tests.
- Wiring/integration verification + runbook.

Commit completed logical units as work progresses, normally 3-8 related files,
with each exact snapshot buildable. Build before each commit; run full checks
and push separate commits to develop at phase completion. Do not squash into a
catch-all commit or push main. Phase is currently incomplete; no phase push yet.

## Explicit exclusions

UC3 customer acceptance/rejection, sixth-rejection cancellation, Deposit/PO
deadlines and verification, production jobs, stock/purchasing, delivery/payment.
Record confirmed future rules but do not add those later-phase behaviors.
