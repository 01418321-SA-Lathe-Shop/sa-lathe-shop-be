# Backend setup

The original README.md is preserved. This document is the backend runbook.

Scaffolding only: no auth, uploads, notifications or UC implementations yet.
Requires Go 1.25+, Git, PowerShell 7+, Docker Desktop with Linux containers.

Dependencies: chi/v5 and chi/cors; pgx/v5; Goose v3. Standard library config,
log/slog and explicit validation. No ORM, Redis or cache.

Commands and Docker instructions are added as scaffolding phases complete.
