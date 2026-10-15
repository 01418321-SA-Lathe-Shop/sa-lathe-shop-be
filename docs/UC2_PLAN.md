# Phase 5 — UC2 estimation and quotations (complete)

Authorized 2026-10-15 on develop. Sources: docs/01-backend.md, requirements.md,
existing UC1 code and direct owner decisions. Keep Go/chi, PostgreSQL NUMERIC,
Handler -> Service -> Repository, repository interfaces and existing private auth.

## Confirmed pricing decisions

Owner selected total-cost markup: `(material_cost + labor_cost) * 1.60`.
All amounts are totals for the entire requested quantity, not unit prices.
THB amounts have two decimal places; round the minimum upward to a satang.
The pricing questions are resolved.

Material-only markup and selling-price margin were not selected and must not be
used. JSON amounts are exact decimal strings; the implementation uses satang and
NUMERIC(18,2). A quoted final price may exceed the calculated minimum, not fall
below it. No formula choice, unit interpretation or rounding question remains.

Build/vet/unit, real PostgreSQL concurrency/rollback, staff/customer/S3 regression
and Docker flow checks passed. The Docker socket startup issue was recovered
without resetting/deleting data. See UC2_VERIFICATION.md for results and recovery
details. Separate implementation commits remain intact; push tracking is in PLAN.md.

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
catch-all commit or push main. Implementation and verification are complete;
the final push acknowledgement is tracked in PLAN.md.

## Explicit exclusions

UC3 customer acceptance/rejection, sixth-rejection cancellation, Deposit/PO
deadlines and verification, production jobs, stock/purchasing, delivery/payment.
Record confirmed future rules but do not add those later-phase behaviors.
