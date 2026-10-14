# SA-Lathe-Shop-BE

Made-to-order lathe shop backend. The owner authorized Phase 4 (UC1 customer login and job request submission) on develop on 2026-10-14. Stop after Phase 4; UC2-UC9 remain unapproved.

Phases 3 and 4 are implemented and verified. See docs/AUTH.md and docs/UC1.md. Customers self-register; company users join by one-use invitation and share company requests. PDF, DOC/DOCX, XLS/XLSX, TXT, JPEG/PNG only, maximum 100 MB. LINE alerts target one configured Admin user; inbox notifications are per-account. Never infer company membership from a name or tax ID alone. Stop before UC2 unless separately authorized.

## Stack

- Go, net/http, github.com/go-chi/chi/v5.
- PostgreSQL with pgx/v5 isolated behind repository implementations. Money: NUMERIC, never float. Stock reservations use transactions and row locking.
- golang-migrate SQL migrations, pinned Go tool. Migrations 000002 (staff) and 000003 (customer/UC1) are implemented. Apply migrations before deploying binaries that use them.
- S3-compatible private object storage (SeaweedFS locally, pinned official GHCR image digest). Store object keys and metadata only; signed upload/download URLs. Evidence never public. MinIO registries were unavailable during setup, so local provider was replaced without changing the S3 contract.
- Admin and Worker auth: Argon2id password hashing, opaque Bearer sessions checked in PostgreSQL on every protected request, and role permissions. Suppliers have no accounts. First Admin is provisioned through cmd/api -bootstrap-admin, never a public registration route.
- PostgreSQL transactional outbox and separate worker. No Redis, broker, or cache.
- Standard library environment config, log/slog, JSON parsing/validation. Configuration and external dependencies behind small interfaces.

## Architecture and layout

cmd/api and cmd/worker are composition roots. Feature packages under internal/{auth,quotation,job,material,procurement,delivery,payment}; each feature has handler, service, repository interfaces, and domain types. PostgreSQL implementations live under internal/repository/postgres. Services accept interfaces for mock testing.

Handlers parse/validate requests and write responses. Services implement business rules. Only repositories execute SQL or import the DB driver. Infrastructure connection bootstrapping belongs in the PostgreSQL repository package; entrypoints may compose it. Entities never access DB. Service transaction orchestration calls repository interfaces; no DB types escape into services.

internal/config: environment config. internal/platform/httpserver: shared HTTP support. internal/health: liveness/readiness. migrations: SQL. docs: requirements and setup. tasks.ps1: Windows tasks.

## Conventions

Use gofmt, explicit errors, contexts and cancellation. No global DB handles. snake_case SQL and JSON names. timestamptz for instants; durations are not timestamps. Monetary arithmetic must be exact. Private file access requires authorization before signing. No credentials in logs. Transactions must atomically persist stock/payment changes and associated events. Use parameterized SQL, consistent JSON errors and idempotency for later financial/stock commands.

## Run and verify

See docs/SETUP.md. PowerShell: ./tasks.ps1 test; ./tasks.ps1 lint; ./tasks.ps1 run; ./tasks.ps1 migrate-up. Build: go build ./...; vet: go vet ./.... Docker Compose requires private values from a local .env. Do not commit .env or local data.

## Commit and push rules

These are mandatory for every phase, not optional cleanup at the end:

1. Plan reasonable commit groups and commit completed logical units as work progresses. Each commit has one purpose, normally 3-8 related files. Keep dependencies in buildable order; include the tests needed to explain that change.
2. Never collect a whole multi-feature phase or several phases into one giant commit and push. Do not split cohesive changes artificially just to increase the commit count. A small documentation-only follow-up may be one focused commit.
3. Use short Conventional Commit messages describing what changed and why. Avoid vague messages such as update, changes or done. Make each group understandable, independently reviewable and easy to locate with git log or git bisect.
4. Run go build ./... before every commit and verify that the committed snapshot itself builds, not merely a working tree containing uncommitted dependencies. Run focused tests for each group and the full required checks at phase completion.
5. At the end of EVERY completed phase, update PLAN.md, review the staged diff, git status and git log --oneline, then push that phase's separate commits to the authorized branch. Do not accumulate completed phases locally. Current destination is origin/develop; do not push main without explicit authorization.
6. Verify the remote branch tip after pushing and report the branch, commit range and test results. If a push is blocked, report the reason and pending commits; never claim the phase was pushed. Never force-push or rewrite published history without explicit authorization.

Never include unrelated changes, secrets, outputs or local data. If no remote exists, ask the owner for its URL; never invent one. Preserve pre-existing user files and changes. PLAN.md must record phase status, decisions and problems. See docs/GIT_WORKFLOW.md for examples.

## Open questions

- Customer self-registration and invitation-only company joining/shared request access are confirmed. Invitation lifetime defaults to 24h, owner-only issuance; production business identity verification and membership-transfer policy remain future work.
- Staff frontend session persistence/cookie strategy and trusted proxy handling require deployment/frontend decisions. Current auth uses opaque Authorization Bearer tokens, not cookies.
- Profit target 60%: markup on materials, markup on total costs, or margin on sale price?
- Cancel on the sixth quotation rejection (more than five), confirmed by owner.
- Deposit/PO deadline is quotation acceptance time plus 14 days, confirmed by owner. Do not extend this rule to invoice terms without confirmation.
- LINE targets only the configured Admin user, never a group or broadcast. In-system notifications are per-user. Credentials remain private environment variables; real LINE delivery is disabled by default and was not tested against a real recipient.
- Hosting, expected load, performance targets, production malware scanning, bucket lifecycle/retention and failed-outbox monitoring remain deployment decisions. File allowlist and 100 MB limit are confirmed.
- Whether one approved quotation can create multiple jobs/invoices requires confirmation.
- Original Word/sequence files remain unavailable; Phase 4 used docs/01-backend.md, the original ER and direct owner clarifications. Resolve later-UC ambiguities before their phases; do not use unrelated project documents.

## Confirmed rules

Individual: first payment is 50% deposit, no PO. Company <= THB 10,000: PO only; above: PO plus 50% deposit. Company issues and uploads PO. Payment can follow job completion. Admin manages workers. Supplier payment usually settles once. Delivery records self/third-party method only; no tracking integration. Stock requirements, reservations and actual usage are separate concepts; unused stock remains available after reconciliation.
