# Backend setup

The original README.md is preserved. This document is the backend runbook.

Scaffolding only: no auth, uploads, notifications or UC implementations yet.
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
5. ./tasks.ps1 run (in another terminal).
6. Invoke-RestMethod http://localhost:8080/health
7. Invoke-RestMethod http://localhost:8080/health/ready

Tasks read .env as data (never execute it); explicit process variables win.
Direct go run uses process variables only. Use ./tasks.ps1 worker in a separate
terminal for the idle worker skeleton. It checks DB and waits for shutdown;
outbox schema and notification dispatch are future features.

PostgreSQL listens on localhost:55432; SeaweedFS S3 on localhost:19000.
The local S3 provider creates the configured private bucket with supplied
credentials. Signed upload/download implementations are future feature phases.
MinIO registry images were unavailable; the official SeaweedFS GHCR image is
pinned by digest. Docker Go/Alpine images use the public ECR official-image
mirror because Docker Hub blob downloads timed out on this machine.

Docker applications: docker compose --profile app up --build -d
Stop: docker compose down (keeps data volumes).
Build/test/lint: ./tasks.ps1 build; ./tasks.ps1 test; ./tasks.ps1 lint.
Migrations: ./tasks.ps1 migrate-up; ./tasks.ps1 migrate-version;
./tasks.ps1 migrate-down (one migration only; avoid production rollback).
The pinned golang-migrate CLI v4.19.0 runs with the postgres build tag; the first
run fetches the tool. The initial migration intentionally creates no business
tables. Only the migration tool's tracking table exists.

/health is liveness; /health/ready checks PostgreSQL with a bounded timeout and
returns HTTP 503 if unavailable. API can start while DB is unavailable. Worker
requires DB on startup. HTTP logs omit query strings and credentials.
Shutdown uses SIGTERM in containers or Ctrl+C locally.

Hosting is undecided. DB SSL disable is for local development only; supply your
environment's connection/TLS settings when a hosting target is selected.
