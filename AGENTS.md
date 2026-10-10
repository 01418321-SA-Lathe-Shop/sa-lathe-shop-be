# SA-Lathe-Shop-BE

Made-to-order lathe shop backend. Current authorized scope: scaffolding phases 0-2 only. Stop after phase 2; obtain the owner's go-ahead before auth or UC features.

## Stack

- Go, net/http, github.com/go-chi/chi/v5.
- PostgreSQL with pgx/v5 isolated behind repository implementations. Money: NUMERIC, never float. Stock reservations use transactions and row locking.
- golang-migrate SQL migrations, pinned Go tool. Empty initial migration only until feature schema approval.
- S3-compatible private object storage (SeaweedFS locally, pinned official GHCR image digest). Store object keys and metadata only; signed upload/download URLs. Evidence never public. MinIO registries were unavailable during setup, so local provider was replaced without changing the S3 contract.
- Admin and Worker auth first: password hashing, revocable sessions, role permissions. Suppliers have no accounts.
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

At each completed phase, make separate logical commits containing 3-8 related files. Use short Conventional Commit messages with purpose. Run go build ./... before every commit. Never include unrelated changes, secrets, outputs or local data. Inspect git status and git log --oneline before each phase push. Push only after that phase completes. If no remote exists, ask the owner for its URL; never invent one. Preserve all files that predate this work. PLAN.md must be updated at the end of each phase with checklists, status, decisions and problems.

## Open questions

- Customer login and customer authorization are undecided.
- Profit target 60%: markup on materials, markup on total costs, or margin on sale price?
- Rejection limit: owner's wording says more than five rejections (sixth); confirm cancellation semantics before implementing.
- No deposit/PO deadline is defined; do not implement timed cancellation.
- Notification channel/provider, template and delivery policy are undecided.
- Hosting, expected load, performance targets and file size/type limits are undecided.
- Whether one approved quotation can create multiple jobs/invoices requires confirmation.
- Original Word use-case attachment is no longer present at its supplied path. Obtain current requirements/sequence documents before feature implementation; do not use unrelated project documents.

## Confirmed rules

Individual: first payment is 50% deposit, no PO. Company <= THB 10,000: PO only; above: PO plus 50% deposit. Company issues and uploads PO. Payment can follow job completion. Admin manages workers. Supplier payment usually settles once. Delivery records self/third-party method only; no tracking integration. Stock requirements, reservations and actual usage are separate concepts; unused stock remains available after reconciliation.
