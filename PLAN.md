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

## Phase 2 DB migrations and Docker — in progress
- [ ] PostgreSQL pool and DB readiness endpoint.
- [ ] golang-migrate and empty initial up/down migration.
- [ ] API/worker Dockerfiles, PostgreSQL/MinIO Compose, PowerShell tasks.
- [ ] Verify build/vet/API/DB, migrate up/down and container entrypoints.
- [ ] Batch commits, phase push, stop for owner approval.

## Phase 3 auth and roles — todo
- [ ] Admin/Worker authentication, password hashing, revocable sessions and RBAC.
- [ ] Resolve customer login before customer-facing authorization.

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

Every future phase: update this file, test business rules, build before each 3-8-file commit, inspect status/log, push at phase end, record decisions/problems. No feature implementation authorized yet.
