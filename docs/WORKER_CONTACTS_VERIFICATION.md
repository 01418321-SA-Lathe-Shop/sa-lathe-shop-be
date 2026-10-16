# Required Worker contacts — verification

Owner confirmed Admin creates Workers with required email/phone, valid email
syntax, international phone support and non-unique contacts. Existing accounts
are preserved without fabricated backfills. API details: AUTH.md.

## Results

PASS: go build ./..., go vet ./..., go test ./... and
./scripts/verify-uc3.ps1 (exit 0). Both PHASE6_VERIFICATION_PASSED and
WORKER_CONTACTS_VERIFICATION_PASSED were observed. Disposable project:
sa-lathe-uc3-4a16ab7c. All newly-created containers, volumes and network were
removed. No existing database, user volume or real LINE recipient was touched.

- Unit/service and HTTP tests reject missing contacts, invalid email/phone,
  blank/null PATCH fields and unknown fields; Admin permissions remain enforced.
- Valid international and local numbers normalize without losing leading zeros.
- Worker create/detail/list/PATCH return contacts; omitted fields are retained.
- Real PostgreSQL permits duplicate email/phone across Workers, rejects invalid
  formats/removal and rolls back failed creation without orphan person rows.
- Migration 6 up/down/up preserves a pre-migration contact-less Worker fixture;
  Admin-compatible backfill succeeds without fabricated data. Populated shared
  contacts prevent rollback of the non-unique policy before schema changes occur.
- Staff session/revocation tests and full UC1 private S3, UC2 quotation and UC3
  decision/concurrency tests pass against fresh isolated databases.
- Docker API readiness, Admin Worker list/PATCH, customer quotation acceptance,
  immutable replay, inbox worker dispatch and graceful shutdown pass.

## Deploy

Run ./tasks.ps1 migrate-up before deploying the new API (migration 000006).
Old Workers may return null contacts until Admin fills them through PATCH; new
Workers always require both fields. No existing login/session is invalidated.
Format CHECK constraints are NOT VALID for legacy data but enforce future
writes; repair any pre-existing malformed values when updating that person.
Do not force a populated down migration or delete shared contact data. Production
contact ownership verification and message delivery are not implemented here.

Logical implementation commits precede this report. Develop-only push status is
recorded in PLAN.md. No UC4/payment/PO/job feature was added.
