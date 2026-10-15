# UC3 — Customer quotation decisions

Phase 6 records customer decisions and immutable acceptance requirements. Apply
migration 000005 before deploying. No VAT, payment receipt, PO upload/verification,
job opening, expiry worker or new outbound notification event is added here.

## Access and endpoints

All endpoints require the existing Bearer session. Only active Customers may
decide. Any current active company member may decide for that company, not just
its owner or the request creator. Admin/Worker may read decision history but
cannot decide. Unrelated customer/company IDs are hidden with 404.

| Method | Path | Purpose |
|---|---|---|
| POST | `/quotations/{quotation_id}/decision` | Accept/reject latest sent quote |
| GET | `/quotations/{quotation_id}/decision` | Read saved decision and terms |
| GET | `/requests/{request_id}/quotation-decisions` | Paginated decision history |

POST body, using the lock version returned by the sent quotation:

```json
{"outcome":"accept","expected_lock_version":2}
```

Rejection uses `"outcome":"reject"` and an optional `reason` (trimmed, at most
2000 Unicode characters, no NUL). Acceptance cannot include a nonempty reason.
Actor IDs, timestamps, terms and unknown fields are rejected as input. History
accepts limit 1-100 and offset >=0. Customer responses never expose internal costs.

Missing/revoked session: 401; wrong role: 403; inaccessible/missing resource: 404;
invalid body: 400; stale version, conflicting decision or invalid state: 409.

## Atomic state and retry rules

The request row lock serializes decisions and quotation revisions. The transaction
rechecks current account/customer/membership activity and locks the sent quotation
and cost snapshot. It atomically updates quotation state/lock version, writes one
decision audit, saves acceptance terms if needed, and updates request state.

- Accept: quotation becomes approved; request remains ongoing.
- Reject: quotation becomes rejected, allowing a Worker revision. Count distinct
  customer rejection audits across all revisions of the request.
- Sixth rejection: request becomes canceled; further revision is blocked. Accepting
  after five rejections is allowed. Worker refusal does not count as a rejection.
- Exact replay by the same actor with the same outcome, normalized reason and
  original expected_lock_version returns the saved decision, without recounting
  rejection or changing deadlines. Another actor or changed payload returns 409.
- Historical replays may return a prior result after later revisions. The returned
  request_status and rejection_count describe the state after that decision, not
  the request's current state; fetch the request separately for current status.

## Acceptance terms

| Customer | Deposit | PO |
|---|---|---|
| Individual | 50%, rounded upward to a satang | No |
| Company, total <= 10000.00 | None | Yes |
| Company, total > 10000.00 | 50%, rounded upward to a satang | Yes |

The server captures accepted_at once and sets documents_due_at to exactly
336 hours later (14 days). Money remains decimal strings/NUMERIC(18,2).
Example: 1600.01 -> deposit_amount 800.01, balance_after_deposit 800.00.
The balance is a calculation, not confirmation that the deposit was paid.

Terms expose customer_type, final_price, deposit_required, po_required,
deposit_amount, balance_after_deposit, accepted_at and documents_due_at. They
are persisted snapshots, never recalculated using later customer/policy changes.
Rejected decisions have no terms. Decision responses also include quotation_id,
request_id, actor_id, outcome, optional reason, expected_lock_version, decided_at,
rejection_count and request_status.

## Storage and verification

quotation_decision has one row per quotation with actor/time/outcome/version and
request-level rejection sequence constraints. quotation_acceptance_terms references
only accepted decisions and enforces the threshold, exact rounded deposit and
deadline in PostgreSQL. No historical decisions are fabricated for legacy rows.
Migration rollback refuses when decisions exist; back up and prefer forward fixes.

```powershell
./tasks.ps1 migrate-up
go build ./...
go vet ./...
go test ./...
./scripts/verify-uc3.ps1
```

The script uses fresh isolated databases and Compose resources, restores process
configuration, disables real LINE delivery and cleans only resources it created.
See UC3_VERIFICATION.md. UC4-UC9 require separate authorization.
