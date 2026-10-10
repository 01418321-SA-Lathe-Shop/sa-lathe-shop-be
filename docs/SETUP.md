# Backend setup

The original README.md is preserved. This document is the backend runbook.

Scaffolding only: no auth, uploads, notifications or UC implementations yet.
Requires Go 1.25+, Git, PowerShell 7+, Docker Desktop with Linux containers.

Dependencies: chi/v5 and chi/cors; pgx/v5; golang-migrate v4. Standard library config,
log/slog and explicit validation. No ORM, Redis or cache.

Environment is loaded from process variables. API: go run ./cmd/api. Worker:
go run ./cmd/worker. GET /health returns JSON status ok. Unknown routes and
failures return {"error":{"code":"...","message":"...","request_id":"..."}}.
