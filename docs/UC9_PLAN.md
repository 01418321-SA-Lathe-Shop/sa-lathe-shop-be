# Phase 12 — UC9 delivery, invoicing, customer payments and returns

Owner requested the next phase on 2026-10-19. Started from clean develop at
1f1eb08, with UC7/UC8 real PostgreSQL/S3/Docker verification complete. UC9 planning
was authorized. On 2026-10-20 the owner answered yes to all seven decisions;
implementation is authorized. Push develop only, never main.

## Sources reviewed

AGENTS.md, PLAN.md, docs/01-backend.md, docs/requirements.md, original full
Mermaid ER attachment, migration 000007 job/deposit contract, migration 000011
production guards and current production domain. The original detailed Word
attachment remains unavailable; no current sequence document is supplied.
Direct owner decisions override the older ER and guide proposals.

## Existing contracts to preserve

- Go/chi Handler -> Service -> Repository; only repositories execute SQL.
- Exact NUMERIC money, THB/no VAT, timestamptz, private S3 object keys/metadata.
- Job price and verified actual deposit are immutable contract snapshots.
  job.balance is the opening post-deposit balance, not a mutable receivables cache.
- Production completion is not customer acceptance or final payment.
- Supplier settlement is separate from customer receipts. Do not reuse UC7's
  one-full-supplier-payment rule to prohibit customer installments.
- Customer access uses current individual ownership/company membership, never
  names or tax IDs. Preserve Admin-only internal cost and supplier evidence.
- UC8 currently forbids completed -> in_progress and preserves completion time.
  A customer return needs a deliberate audited extension, not a raw status UPDATE.
- Issued material cannot reappear in stock on a customer return. Further work
  appends usage/time/cost audit; no rewriting old production or payment records.

## Owner-confirmed decisions (2026-10-20)

1. Roles: Admin/Worker issue invoices and record delivery; Customer submits bank
   evidence; Admin alone approves/rejects final payments and records cash receipts.
   Workers see customer invoice/balance needed for delivery, never internal costs.
2. Cardinality/amount: one invoice per job for accepted price minus actual approved
   deposit; one whole-job shipment per attempt, with new attempts after return.
   No partial shipments, cross-job consolidated invoices, additional delivery fee
   or price changes. Fully prepaid/zero-balance jobs need no fabricated payment.
3. Customer terms: installments allowed, positive amounts up to remaining unpaid
   balance, one invoice per receipt, no unallocated credit/refund/overpayment.
   Admin explicitly chooses credit days when issuing an invoice, from issue time;
   no default and no reuse of the UC4 acceptance-plus-14-days deadline.
4. Evidence: bank transfer and cash only; bank requires ready private proof, cash
   proof optional. Customer bank submissions do not reduce balance until Admin
   approval. Pending submissions reserve no paid amount; approval must atomically
   recheck outstanding balance so competing payments cannot overpay.
5. Inspection/rework: individual owner or any active company member accepts/returns
   an outstanding shipment; no staff proxy acceptance. Return requires a reason,
   before acceptance, and atomically reopens the same job for free rework.
   Keep original invoice/payments, no price change/refund or raw-material return.
   Subsequent usage is explicit free-stock issue, no automatic re-reservation of
   the old plan; retain production-cycle history and cumulative costs/minutes.
   Redelivery requires that job to complete again. Post-acceptance warranty is
   outside this proposed first version, not silently disallowed forever.
6. Closure: job_request becomes completed only when the latest delivery is
   accepted AND invoice is fully paid, in either order. Delivery before final
   payment is allowed; payment alone never implies acceptance. Full prepayment
   satisfies the payment condition without an artificial receipt.
7. Notifications: in-system delivery notice to current customer members and
   inspection/payment-submission alerts to active Admins; review result to the
   submitting customer. No new LINE event, overdue reminder, timing assumption
   or carrier integration in this phase. Existing configured Admin LINE is unchanged.

All seven proposals above are confirmed. Admin enters terms separately before
Admin/Worker issues the invoice, so Worker invoice permissions do not grant the
ability to choose/change financial terms. No default credit period is inferred.

## Implementation sequence after confirmation

1. Exact invoice/balance/payment and delivery/inspection rules, role-safe domain
   views and mock tests; preserve price/deposit snapshots and zero balances.
2. Migration 000012: proposed delivery/inspection history, invoice, customer
   payment submission/review/allocation, private evidence and immutable command
   audit. Final names/relationships depend on owner decisions. Extend production
   cycles/status guards only for the confirmed return workflow.
3. Repository transactions: current account/membership checks, deterministic
   request -> job -> invoice/payment locks and sorted material locks if needed,
   idempotent original-result replay, optimistic versions, guarded closure and
   atomic outbox events. Do not perform S3 or notification network calls in SQL TX.
4. Feature-oriented delivery/invoice/customer-payment services and thin handlers;
   reuse accepted private upload validation without exposing request drawings,
   supplier evidence or staff-only production photos to unrelated roles.
5. Proposed API groups: staff job delivery/invoice reads and commands; customer
   delivery decisions; scoped invoice/payment reads, private evidence upload/
   completion/download, bank submission and Admin review/cash receipt. Publish
   concrete endpoints/error/status contracts only after decisions are confirmed.
6. Real isolated PostgreSQL/S3 tests: concurrent invoices/accept-return/reviews,
   partial/full/zero balances, duplicate commands and stale memberships, no
   overpayment, atomic audit/notification rollback, privacy, repeated rework and
   no duplicate deposit credit. Full prior UC1-UC8 regression and Docker API/worker
   end-to-end, readiness, migration up/down guards and scoped resource cleanup.
7. Build/vet/lint, runbook and verified PLAN status. Commit logical buildable
   groups while working; push develop and verify its remote tip, never main.

Implementation and real PostgreSQL/S3/Docker verification are complete on
2026-10-20; both full runs passed and scoped cleanup was observed. Implementation
batches ed09788 through c487fa2 are published to develop; main unchanged. See
UC9_VERIFICATION.md for exact projects/checks and PLAN.md for completed checklist.
No production deployment or existing database migration was performed, and no
new cloud/service, invoice PDF renderer or performance SLA was inferred.
