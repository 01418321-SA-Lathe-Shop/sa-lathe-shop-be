# Phase 6 — UC3 customer quotation decisions (implemented and verified)

Authorized 2026-10-15, develop. Standing push destination is origin/develop for
all phases and follow-ups until an explicit instruction authorizes main.
Keep Go/chi, PostgreSQL and Handler -> Service -> Repository; no stack change.

## Confirmed rules

- Respond to a sent quotation, preserving sent prices and internal-cost privacy.
- Acceptance sets approved and records who/when; rejected quotations enable the
  next Worker revision. Do not overwrite prior quotes or decision history.
- Count distinct customer rejections per request across revisions, not per user
  or draft save. The sixth rejection cancels the request; prevent further revision.
- Individual: 50% deposit, no PO. Company <= THB 10,000: PO only. Company above
  THB 10,000: PO and 50% deposit. All prices are whole-request THB totals.
- Acceptance time plus 14 days determines the Deposit/PO deadline. Sending and
  replaying a request must not start/reset that clock.

## Owner confirmations

1. Any active company member may accept/reject the company's quotation. The
   decision actor need not own the company or have created the request.
2. Round 50% deposits upward to one satang: 1600.01 requires 800.01, leaving
   800.00 after that deposit. These are required terms, not payment receipts.

## Planned groups and verification

1. Scope/standing branch policy and decisions (this group).
2. Customer-decision domain, exact terms and boundary/replay unit tests.
3. Audit/terms migration and transactional repository; membership/activity and
   latest sent quotation checks, request row locking and atomic rejection count.
4. Decision/history routes with ownership, strict JSON and cost-redaction tests.
5. Disposable PostgreSQL concurrent accept/reject/replay tests, six-rejection
   revision cycle, threshold/deposit/deadline boundaries, earlier-phase regression
   and Docker flow, then API docs and end-of-phase push acknowledgement.

Commit each logical group as it completes; normally 3-8 related files, clear
Conventional Commit purpose and independently buildable snapshots. Run full
checks before the phase push; preserve separate commits on develop. No main push.

## Scope boundaries

Record acceptance requirements for UC4; do not receive/verify money, upload PO or
create jobs in Phase 6. Do not add an expiry-cancellation worker or repurpose UC1
LINE alerts without a defined event contract. Shared decision reads must retain
company isolation. Existing unavailable source documents are not replaced with
assumptions; use the reviewed guide/ER and direct owner clarifications.

## Result

All planned implementation and verification groups passed. See UC3.md for the
API contract and UC3_VERIFICATION.md for real database, race and Docker checks.
The disposable verification project was sa-lathe-uc3-f2ced38b; its test resources
were removed. Phase completion push is recorded in PLAN.md. Stop before UC4.
