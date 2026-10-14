# Implementation plan

## Phase 0 repo and docs — done
- [x] Inspect existing repository and remote; preserve existing README.md.
- [x] Read available schema and prior fully-read UC context; record missing originals.
- [x] Add ignore rules, environment template, AGENTS.md and setup documentation.
- [x] Create buildable module bootstrap so every commit builds.
- [ ] Push phase commits.
Decision: repository already initialized on main; README.md is untouched. Module and empty entrypoints are bootstrapped in phase 0 to satisfy build-at-every-commit. Problem: original UC Word attachment is missing; no sequence attachment is currently available. Phase push pending verification.

## Phase 1 project skeleton — done
- [x] Feature-oriented structure, API and worker entrypoints.
- [x] Config, structured logging, request ID/recovery/timeout/CORS.
- [x] Consistent JSON errors, graceful shutdown and GET /health.
- [x] Build, vet and HTTP/config tests.
- [ ] Push phase commits.
Decision: standard library env config/slog; chi/cors allowlist; middleware timeout produces JSON. Push blocked pending explicit remote confirmation requested by automatic approval review. Phase 0 local commits completed; remote is origin/main at Yoshi4489/SA-Lathe-Shop-BE.

## Phase 2 DB migrations and Docker — done
- [x] PostgreSQL pool and DB readiness endpoint.
- [x] golang-migrate and empty initial up/down migration.
- [x] API/worker Dockerfiles, PostgreSQL/private S3 Compose, PowerShell tasks.
- [x] Verify build/vet/tests/lint/API/DB, migrate up/down and container entrypoints.
- [x] Local batch commits; stop before auth/UC feature code.
- [ ] Phase push: awaiting explicit confirmation of GitHub destination.
Decision: pgx/v5 in repository only; standard library config/slog/validation; golang-migrate v4.19.0 pinned CLI. Local S3 switched to digest-pinned official SeaweedFS after MinIO registry denial. Official Go/Alpine ECR mirrors resolved Docker Hub timeouts. Verified liveness/readiness 200, DB-down readiness 503 with liveness 200, anonymous S3 listing 403, up/down/up migrations and graceful API/worker stop. Temporary verification containers and newly-created volumes were removed; no existing project files/data deleted. No secrets persisted or committed. See docs/VERIFICATION.md.

## Phase 3 auth and roles — done
- [x] Staff schema, Argon2id passwords, opaque revocable sessions and RBAC.
- [x] One-time first-Admin CLI with hidden password prompt; no default account.
- [x] Admin creates/lists/edits/deactivates/reactivates Workers; no role promotion.
- [x] Login/logout/me endpoints; expiry and DB-backed activity/revocation checks.
- [x] Service mock tests, HTTP tests and real PostgreSQL integration/race checks.
- [x] Build/vet/tests/lint, migrations up/down/up and Docker auth verified.
- [x] Update runbook and local 3-8-file buildable commit batches.
- [x] Customer login remains an open question for UC1; suppliers have no login.
- [ ] Push phase batches: exact GitHub destination confirmation remains pending.
Decision: Argon2id 64 MiB/3 passes/2 lanes; random 256-bit Bearer tokens, SHA-256 token digests only in PostgreSQL, configurable absolute TTL (default 12h). Login and Worker credential/activity changes serialize on the account row. Deactivation/password/username changes revoke sessions atomically; already-running requests may finish, subsequent requests reauthenticate. No session cache. Local login abuse limiter is bounded and trusts only the socket IP. See docs/AUTH.md and docs/AUTH_VERIFICATION.md. Phase 4 remains unapproved; current missing original UC/sequence files and customer authorization must be resolved before UC1 work.

## Phase 4 UC1 submit job request — todo
- [ ] Customer details, request validation, private attachments and notifications.
## Phase 5 UC2 estimate and quote — todo
- [ ] Draft/send estimates, queue review and rejection reason.
## Phase 6 UC3 verify customer terms — todo
- [ ] Accept/reject history, revisions, rejection threshold and pricing policy.
## Phase 7 UC4 deposit and company PO — todo
- [ ] Validate first payment and company PO, open job only when terms fulfilled.
## Phase 8 UC5 stock check and reservation — todo
- [ ] Required materials, partial reservations, shortages and row locking.
## Phase 9 UC6 order materials — todo
- [ ] Supplier orders/items and supplier rejection flow.
## Phase 10 UC7 receive and stock — todo
- [ ] Partial receipts, damage claims, moving average and supplier settlement.
## Phase 11 UC8 execute job — todo
- [ ] Stock issue, unused reservation reconciliation, photos and actual cost/time.
## Phase 12 UC9 deliver and settle — todo
- [ ] Self/third-party delivery, invoices, post-completion payments and returns.

Every future phase: update this file, test business rules, build before each 3-8-file commit, inspect status/log, push at phase end, record decisions/problems. Owner authorized Phase 3; UC1-UC9 remain unapproved. Customer login does not block staff authentication but must be resolved before customer-facing authorization.
