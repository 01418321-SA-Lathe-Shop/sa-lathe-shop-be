# Phase 7 UC4 verification — PASS

Full runs passed on 2026-10-18 (Asia/Bangkok). Initial isolated Compose project:
sa-lathe-uc4-5316ed1f. Expanded final run: sa-lathe-uc4-92badc31. Both returned
exit 0 with PHASE7_VERIFICATION_PASSED and WORKER_CONTACTS_VERIFICATION_PASSED.

## Commands

```powershell
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
./scripts/verify-uc4.ps1
```

The script runs uncached unit/HTTP and real PostgreSQL/S3 integration tests for
staff, required Worker contacts and UC1-UC4 using five fresh dedicated databases.
It applies migrations 1-7, verifies empty migration 7 down/up, preserves/backfills
a test-only legacy Worker profile, builds/runs API and worker in Docker, tests
the HTTP flow, and gracefully stops them. Real LINE sending stays disabled.

## Verified UC4 behavior

- Any active company member may submit their own finalized private upload.
  Wrong company, unready/another uploader's file and removed membership are denied.
- Upload alone is not a payment; company >10000 needs deposit AND PO approvals.
- Rounded lower bound and quoted upper bound use exact decimal/satang arithmetic.
  A full deposit leaves zero balance; actual approved amount does not rewrite
  the original minimum terms. Domain tests also retain zero-price contract behavior.
- Same-key concurrent submissions create one record; changed payload conflicts.
- Admin and Worker may review; Customer and inactive stale Worker cannot.
- Concurrent identical reviews replay one result; opposite reviewers/outcomes have
  one winner and one conflict. Exactly one queued job is created.
- Timely pending complete evidence is not expired and may be reviewed later.
  A direct review-vs-expiry race preserves that evidence and the queued job.
- Missing evidence after deadline is canceled with audit. Rejected required
  evidence no longer protects an expired request. Late submission is blocked.
- Rejected evidence can be replaced with a new submission before due; both records
  remain in history and final approval opens the job.
- Forced job constraint failure rolls back final approval. Forced cancellation
  audit failure rolls back request cancellation. Reviewed evidence is immutable.
- Status reads work with a one-connection pool. Real HTTP verifies sessions,
  roles, hidden unrelated IDs and strict unknown-field rejection.
- Real S3 evidence download requires a signed authorized link; unsigned object
  access returns 403, and another company cannot obtain the link.
- Populated migration 7 rollback refuses before dropping UC4 data.
- Docker customer upload/complete/submit, Worker approval/replay and fulfillment
  show one queued job with the actual 1000.00 deposit and 600.00 remaining balance.
- Earlier staff/session, Worker-contact, request/file/notification, quotation and
  customer-decision regressions pass. Existing inbox dispatch still works.

## Scope and safety

Only the test projects' newly-created containers, volumes and networks were
removed. No existing user database/volume was migrated, reset or deleted. Test
credentials were random process-only values and restored afterward, not committed.
Historical deadline fixtures are inserted only into the dedicated test database;
there is no production clock override or acceptance-term editing endpoint.

Production deployment must apply migration 000007 and run the separate worker for
expiry scanning. This is manual evidence approval, not bank/payment-provider
verification. No real payment, refund, new LINE event, final invoice or UC5 stock
feature was implemented/tested. Both API and worker need deployment monitoring.
Develop-only push and final acknowledgement are tracked in PLAN.md.
