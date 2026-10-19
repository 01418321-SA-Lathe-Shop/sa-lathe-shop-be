# Backend setup

The original README.md is preserved. This document is the backend runbook.

Staff auth and UC1-UC6 are implemented; see AUTH.md and UC1.md through UC6.md.
UC7-UC9 remain unapproved. Apply all migrations through 000009 before running
the current API and worker. Supplier orders are Admin-only and do not receive stock.
Requires Go 1.25+, Git, PowerShell 7+, Docker Desktop with Linux containers.

Dependencies: chi/v5 and chi/cors; pgx/v5; golang-migrate v4. Standard library config,
log/slog and explicit validation. No ORM, Redis or cache.

Environment is loaded from process variables. API: go run ./cmd/api. Worker:
go run ./cmd/worker. GET /health returns JSON status ok. Unknown routes and
failures return {"error":{"code":"...","message":"...","request_id":"..."}}.

## Local development in PowerShell

1. Copy-Item .env.example .env (only if .env does not already exist).
2. Edit .env: supply private DB_PASSWORD, S3_ACCESS_KEY (at least 3 characters)
   and S3_SECRET_KEY (at least 8 characters). No committed passwords exist.
3. ./tasks.ps1 dev-up
4. ./tasks.ps1 migrate-up
5. ./tasks.ps1 bootstrap-admin (one-time, hidden password prompt).
6. ./tasks.ps1 run (in another terminal).
7. Invoke-RestMethod http://localhost:8080/health
8. Invoke-RestMethod http://localhost:8080/health/ready

Tasks read .env as data (never execute it); explicit process variables win.
Direct go run uses process variables only. Use ./tasks.ps1 worker in a separate
terminal for PostgreSQL outbox/inbox notification dispatch and UC4 expiry scans.
Real LINE delivery is disabled by default; configure it only with approval for
the specific Admin recipient. There is no new outbound UC6 event.

PostgreSQL listens on localhost:55432; SeaweedFS S3 on localhost:19000.
The local S3 provider creates the configured private bucket with supplied
credentials. UC1/UC4 evidence uses authorized private signed upload/download URLs;
the database contains object keys/metadata, never file bytes or permanent links.
MinIO registry images were unavailable; the official SeaweedFS GHCR image is
pinned by digest. Docker Go/Alpine images use the public ECR official-image
mirror because Docker Hub blob downloads timed out on this machine.

Docker applications: docker compose --profile app up --build -d
Stop: docker compose down (keeps data volumes).
Build/test/lint: ./tasks.ps1 build; ./tasks.ps1 test; ./tasks.ps1 lint.
Migrations: ./tasks.ps1 migrate-up; ./tasks.ps1 migrate-version;
./tasks.ps1 migrate-down (one migration only; avoid production rollback).
The pinned golang-migrate CLI v4.19.0 runs with the postgres build tag; the first
run fetches the tool. Migration 000001 is intentionally empty; migration 000002
adds staff identity and revocable sessions. Migrations 000003-000009 add UC1-UC6.
Never run down against populated business data without a planned recovery path;
procurement down refuses to discard suppliers, commitments or audit history.

/health is liveness; /health/ready checks PostgreSQL with a bounded timeout and
returns HTTP 503 if unavailable. API can start while DB is unavailable. Worker
requires DB on startup. HTTP logs omit query strings and credentials.
Shutdown uses SIGTERM in containers or Ctrl+C locally.

Hosting is undecided. DB SSL disable is for local development only; supply your
environment's connection/TLS settings when a hosting target is selected.

## Disposable full verification

UC8 is implemented as an unverified checkpoint. The current API requires migrations
through 000011. ./scripts/verify-uc8.ps1 prepares nine fresh databases and regression
through UC8, including empty migration 11/10 rollback and real private photo tests.
Its Docker availability probe failed before resource creation; no deployment-ready
claim. UC7 verification and runtime recovery approval remain pending. See UC8.md
and UC8_VERIFICATION.md. Do not use skipped go tests as proof of SQL/S3 readiness.

UC7 implementation is in progress. It adds migration 000010 and
./scripts/verify-uc7.ps1 (eight fresh test databases). See UC7_VERIFICATION.md for
the current Docker recovery/verification status before deploying its API. Worker
inventory endpoints now redact costs; Admin retains them. UC8-UC9 remain unapproved.

```powershell
go build ./...
go vet ./...
./tasks.ps1 lint
./scripts/verify-uc6.ps1
```

verify-uc6.ps1 uses a unique Compose project, random process-only credentials,
an empty temporary Compose environment and seven fresh dedicated test databases.
It does not read local .env or migrate existing data. It verifies migrations,
real PostgreSQL/S3 tests, earlier auth/UC regressions, Docker API/worker and UC6
HTTP behavior, then removes only that project's new containers/volumes/network.
LINE is disabled for the Docker flow; previous process values are restored.
See UC6_VERIFICATION.md. Ordinary go test ./... skips database integration when
the named dedicated test environment variables are absent.
