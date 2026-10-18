# UC4 — Deposit/company PO and queued job

Apply migration 000007 before API/worker deployment. Both Admin and Worker may
review evidence; only Customers submit it. Current active company membership
controls access, not who created the request. Supplier has no account.

## Flow

1. Customer reads accepted terms from GET /quotations/{id}/fulfillment.
2. Upload the evidence through POST /requests/{request_id}/uploads, signed PUT,
   then POST /uploads/{attachment_id}/complete. The existing file allowlist,
   100 MB limit, content inspection and frozen private S3 object rules apply.
3. Submit the ready attachment explicitly as deposit or PO evidence. The attachment
   must belong to that request and submitting account; it cannot be reused for
   another evidence record. A ready drawing is not automatically payment proof.
4. Admin/Worker reads the signed private evidence download and approves/rejects.
   This is manual verification, not bank/provider validation or money movement.
5. The final required approval atomically opens exactly one queued job. The job
   contains actual approved deposit and remaining balance, plus the request's
   optional requested deadline. Production/stock/invoicing are later phases.

## Endpoints

All require Bearer authentication. Paths use quotation/evidence IDs, not request
IDs except the existing upload flow. Customer JSON contains no internal costs.

| Method | Path | Role |
|---|---|---|
| POST | /quotations/{quotation_id}/evidence | Customer member |
| POST | /evidence/{evidence_id}/review | Admin or Worker |
| GET | /quotations/{quotation_id}/fulfillment | Scoped Customer, Admin, Worker |
| GET | /attachments/{attachment_id}/download | Existing scoped signed download |

Deposit body (amount is a decimal string, never a floating-point JSON number):

```json
{"idempotency_key":"a-generated-uuid","attachment_id":"ready-upload-uuid","kind":"deposit","amount":"1000.00","paid_at":"2026-10-18T03:00:00Z"}
```

Replace the placeholder UUIDs and time with actual values. PO body omits amount
and paid_at and sets kind to po. paid_at is the customer's declared receipt time,
cannot be in the future, and does not establish payment until staff approval.
The server records submitted_at and reviewed_at. Monetary/timestamp fields on
the job and accepted terms are never client-controlled.

Approval: `{"outcome":"approved","expected_version":1}`.
Rejection: `{"outcome":"rejected","reason":"Incorrect company PO","expected_version":1}`.
Reason is trimmed, 1-2000 Unicode characters for rejection; approval has no reason.
Evidence starts pending/version 1 and ends approved or rejected/version 2.
The submission and its single review are retained; no edit/delete/re-review API.
Before deadline, replace rejected evidence using a fresh upload and idempotency key.

Responses include evidence ID, quotation ID, submitting account, idempotency key,
attachment ID, kind, amount/paid_at when relevant, status/version, submitted_at,
reviewer/time and rejection reason. Fulfillment returns terms, current request
status, latest 100 evidence records, optional queued job and cancellation time.
File URLs/bytes are not stored in these records; signed links remain short-lived.

## Terms and deadline

- Individual: deposit, no PO. Company <=10000.00: PO only. Above: both.
- A single deposit must be between the stored rounded-up 50% minimum and total.
  No overpayment, partial-deposit aggregation or unnecessary deposit/PO kinds.
- Job balance = quoted total minus actual approved deposit, not minus the minimum.
  A 100% deposit leaves 0.00. UC3 terms remain unchanged snapshots.
- Existing zero-price contracts accept zero-amount confirmation for manual review,
  without pretending positive money was paid or imposing a new minimum price.
- Complete file finalization AND evidence submission must finish by documents_due_at
  (accepted_at +336 hours). Starting an upload before due is insufficient.
- All required kinds must have timely pending/approved evidence. Staff may review
  that complete set later; pending staff review does not cause cancellation.
- Missing/rejected required evidence after the deadline causes system cancellation;
  a late replacement is not allowed and does not reset the acceptance clock.
- Separate worker scans on startup and every 15 seconds in batches of at most 50,
  using request locks/SKIP LOCKED. Cancellation and its audit commit together.
  Transient DB failure retries next scan; worker health/operations monitoring is
  still a production deployment responsibility. Run the worker for timely expiry.

## Retry, permissions and transaction rules

Same submitting account + quotation + idempotency_key and equivalent normalized
payload returns the same evidence record (including its current review status).
Changed payload is 409; a different key cannot bypass one pending/approved record
per quotation/kind. A reviewed record replay requires the original reviewer,
outcome, normalized reason and expected_version=1. Other actors or outcomes get 409.
Exact replays remain readable after deadline/cancellation if current access allows.
Timestamp precision is normalized to PostgreSQL microseconds before submission.

Each mutation rechecks current activity/role/membership under transaction locks.
Reviews, submissions, expiry and job opening serialize on the request row; unique
constraints prevent duplicate active evidence and jobs. A failed final job insert
rolls back that approval; a failed cancellation audit rolls back cancellation.
Repeatable-read status reads return one consistent snapshot and support pool size 1.

401: missing/revoked session; 403: wrong role; 404: inaccessible/missing resource
or another user's/unready upload; 400: invalid body/amount/time; 409: stale,
duplicate or closed/deadline state. JSON rejects unknown fields and extra values.

## Storage, migration and boundaries

term_evidence unifies deposit and PO submissions with kind-specific CHECKs and
immutable review audit; job references the approved records through composite FKs.
uc4_cancellation records why/when the system canceled. Existing attachment keys
and file metadata remain in request_attachment. No duplicated public file URLs.
Migration down refuses if any UC4 records exist; back up and use forward fixes.
No historical payments or jobs are guessed/backfilled for accepted quotations.

Existing UC1 inbox/LINE behavior is unchanged; this phase adds no new outbound
event/recipient contract. No invoice settlement, refunds, gateway, override,
job splitting, stock reservation or UC5-UC9 behavior is implemented here.

Verification: ./scripts/verify-uc4.ps1 uses fresh isolated PostgreSQL/S3/Docker
resources and disables real LINE delivery. See UC4_VERIFICATION.md for results.
