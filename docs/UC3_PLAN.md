# Phase 6 — UC3 customer quotation decisions (in progress)

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

## Questions to confirm before implementing policy

1. Company users can read shared requests, but who can accept/reject: only the
   company owner, any company member, or only the request creator? Shared reads
   alone are not permission to bind the company to a price.
2. For a price of 1600.01, exact 50% is 800.005. May the required deposit round
   upward to 800.01, with the later remaining balance 800.00? Prior approval of
   upward rounding was for minimum quotation price, not deposits.

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
