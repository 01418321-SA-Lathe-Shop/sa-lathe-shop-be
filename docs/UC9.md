# UC9 — Delivery, customer invoices, installments and free rework

Owner confirmed all seven policies on 2026-10-20. Apply migration 000012 before
current API and worker binaries. Implementation verification status is recorded
in UC9_VERIFICATION.md; never treat skipped integration tests as a real DB pass.

## Roles and lifecycle

- Admin sets invoice credit days exactly once on a production-completed job.
  There is no default. Worker cannot set terms; Admin/Worker may subsequently
  issue the invoice. This separates the two confirmed permissions.
- One invoice per job, exactly job.balance (accepted price minus actual approved
  deposit). This is the opening balance snapshot, never recomputed using minimum
  deposit or production costs. No VAT, new shipping fee or price amendments.
- Invoice issue time plus explicit credit_days * 24 hours is the due instant.
  This is separate from the UC4 acceptance-plus-336-hour submission deadline.
  Invoice amount/terms/date/number are immutable; amounts are exact decimal strings.
- Admin/Worker records one whole-job shipment for each completed production cycle.
  method is self or third_party, with required receiver_name (1-200 characters).
  Shipping records dispatch, not customer acceptance. There is no carrier API,
  partial shipment, shipping charge or inferred physical delivery confirmation.
- Individual owner/current company member inspects the latest shipping delivery:
  accept -> delivered, or return -> returned with a required reason. No staff proxy.
  Acceptance is final for this phase; post-acceptance warranty is not implemented.
- Return atomically snapshots the completed production cycle, reopens the same job
  as in_progress and increments production/fulfillment versions. Old completion
  events and cycle timestamps remain immutable. Original started_at is retained,
  current completed_at clears until rework completes; production_cycle advances.
  No raw material is restored or automatically reserved. Further use consumes
  genuine free stock under UC8; previous costs/minutes accumulate, no guessed rates.
  Re-completion is required before redelivery; invoice and payments remain unchanged.
- Request closes only when latest delivery is accepted and invoice is fully paid,
  in either order. Delivery before payment is allowed. Zero balances need no fake
  receipt; payment alone never implies acceptance. Request closure is DB-guarded.

## Customer payments and private evidence

Customers submit positive bank amounts no larger than the current remaining
balance, ready private evidence and paid_at no later than submission. Proof is
not payment approval. Admin approves/rejects, and only approval creates an exact
immutable payment allocation. Rejection requires a reason, approval no reason.
Admin may record positive cash receipts; photo/proof is optional. Bank proof is
required and must belong to the submitting customer and this job. Each proof is
single-use; failed/rejected attempts need a new upload. No split receipt across
invoices, unallocated credit, overpayment, refunds, discounts or payment gateway.

Pending submissions do not reserve paid amounts. Concurrent approvals recheck
balance while invoice/request/job locks are held; stale versions conflict. A
submission that no longer fits can be rejected, including after final closure,
but cannot be partially approved or silently trimmed. Invoice paid/balance/status
are derived only from approved allocations (unpaid/partial/paid), not stored caches.

Customer/Admin uploads use the existing document/image allowlist, 100 MB maximum,
short-lived signed links, content inspection and ETag-conditional immutable copy.
Only object keys/metadata are in PostgreSQL. Worker never sees payment records or
proof links; invoice balances are visible for delivery. Proof download is limited
to the uploader with current customer access or Admin. Other company members may
inspect the invoice/payment status but not download another member's private proof.
Supplier evidence, request drawings and staff production photos remain separate.

## API

All routes require existing Bearer sessions; responses use Cache-Control: no-store.
All commands require UUID idempotency_key and current expected_version from the
fulfillment response (not production.version). Exact same-key/actor/payload retry
returns the original snapshot after current authorization; changed payload conflicts.
Use GET for current state. UUIDs must be canonical, nonzero values.

| Method/path | Role | Body beyond command fields |
| --- | --- | --- |
| GET /fulfillment/jobs?limit=25&offset=0 | Staff/scoped Customer | Paginated items |
| GET /fulfillment/jobs/{job_id} | Staff/scoped Customer | Full current state |
| PUT /fulfillment/jobs/{job_id}/credit-terms | Admin | credit_days, integer 0-36500 |
| POST /fulfillment/jobs/{job_id}/invoice | Admin/Worker | None |
| POST /fulfillment/jobs/{job_id}/deliveries | Admin/Worker | method, receiver_name |
| POST /fulfillment/jobs/{job_id}/inspection | Scoped Customer | outcome accept/return; reason for return |
| POST /fulfillment/jobs/{job_id}/payments/bank | Scoped Customer | amount string, paid_at, evidence_id |
| POST /fulfillment/jobs/{job_id}/payments/cash | Admin | amount string, paid_at, optional evidence_id |
| POST /fulfillment/jobs/{job_id}/payments/review | Admin | payment_id, outcome approved/rejected, reason if rejected |
| POST /fulfillment/jobs/{job_id}/evidence/uploads | Customer/Admin | file_name, content_type, byte_size; no command fields |
| POST /customer-evidence/{evidence_id}/complete | Uploader | No body; start new intent on failure |
| GET /customer-evidence/{evidence_id}/download | Uploader/Admin | Short-lived signed download |

All successful operations return 200; strict JSON rejects unknown fields. Existing
JSON error envelope: 400 invalid_input, 403 forbidden, 404 not_found (including
unrelated customer scope), 409 conflict for state/version/key conflicts; DB
constraint violations map to 400. Unexpected failures are 500 with no SQL/secrets.

State includes job/request statuses, fulfillment version, production cycle, original
deposit/opening balance, invoice, delivery attempts, rework_history and event metadata.
Customer/Admin also receive payment history; Worker does not. Internal cost/stock
valuation and private object keys never appear. Rework history contains cycle,
original start/completion, reopen instant and delivery reference.

## Transactions, audit and notifications

Lock current active account/membership, retry key, request, job, invoice/payment;
production retains request -> job -> sorted-material locking. Return also creates
a customer-authorized production reopen event without bypassing UC8 guards.
Invoice/payment/delivery/allocation/closure/audit/outbox writes commit together.
No S3 operation, LINE send or network notification runs inside these transactions.
Original command/result audits are append-only; populated migration rollback refuses
to destroy records/evidence and requires a deliberate backup/recovery procedure.

Separate worker dispatches in-system events after commit: shipment to current
customer members; customer inspection/bank submission to active Admins; payment
review result to the submitting customer if still an active member. No UC9 LINE,
automatic overdue reminders or real LINE delivery in tests. Delivery retries use
existing leased outbox and unique (event,account) inbox entries. Pending/error
monitoring and production retention/scanning remain deployment decisions.
