# Phase 6 verification

Result: PASS. `go build ./...`, `go vet ./...`, `go test ./...` and
`./scripts/verify-uc3.ps1` completed successfully. Script exit code 0 and marker
PHASE6_VERIFICATION_PASSED were observed. Docker API/worker images built and ran.

Disposable Compose project: sa-lathe-uc3-f2ced38b. All of its containers, network
and newly-created database/S3 volumes were removed afterward. No existing
business database or user volumes were migrated/deleted. Real LINE delivery
remained disabled; credentials were random, process-local and not committed.

## Verified coverage

- Migrations 1-5 up, empty migration 5 down/up, then isolated integration DBs.
- Staff auth, UC1 customer/company/private S3/notification and UC2 regressions.
- Non-owner company member acceptance; unauthenticated, wrong-role, unrelated
  customer, inactive account and removed-membership denial.
- Company threshold 10000.00 vs 10000.01 and individual odd-satang rounding.
- Exact acceptance timestamp/deadline (+336 hours); stored terms survive later
  customer-type changes; history works with a one-connection pool.
- Concurrent opposite decisions have one winner; exact concurrent retries do not
  duplicate audits, recount rejections or reset deadlines. Conflicts return 409.
- Six distinct revisions/rejections cancel the request and prevent another
  revision. Acceptance after five rejections remains possible.
- Forced terms-write failure rolls back quotation/audit/request changes together.
- Strict HTTP input, cost privacy, real Docker register/login/request/quote/send/
  accept/replay/history and existing worker notification processing.
- Graceful API/worker stop and scoped test-resource cleanup.

## Boundaries

This validates terms, not payment settlement, PO verification or job creation.
No new outbound quotation event or deadline-expiry behavior is assumed. Worker
email/phone integration remains a separately documented gap, not part of UC3.
Develop-first commits and push verification are tracked in PLAN.md; main is not
an authorized push destination. Production hosting/load targets remain undecided.
