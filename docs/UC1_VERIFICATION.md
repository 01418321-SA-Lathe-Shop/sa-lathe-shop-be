# Phase 4 verification — 2026-10-15 (Asia/Bangkok)

Scope: UC1 only. Changes target develop, not main.

Passed:

- `go build ./...`, `go vet ./...`, `go test ./...`.
- Unit tests for registration/role enforcement, invitation digests, request
  validation and access guards, upload allowlist/content/size and finalization,
  LINE's exact recipient/retry key/duplicate acknowledgement, integration config.
- Real PostgreSQL migrations 1/2/3, empty Phase 4 rollback and reapply.
- Existing staff auth regression suite against a separate fresh database under
  the new account table name (Admin/Worker/session behavior preserved).
- HTTP registration, login, company owner invitation, coworker registration,
  invitation replay denial with no orphan person, shared company reads,
  cross-company reads/lists denied, forged customer creation denied, staff queue.
- Real private S3 signed PUT, file completion, authorized signed GET, outsider
  denial, anonymous GET 403 and finalized-file protection after staging overwrite.
- Disguised PDF and disallowed SVG rejection; other members cannot complete the
  uploader's intent; one intent cannot complete twice.
- Transactional request/outbox creation, worker inbox delivery and per-user
  read/list isolation; disabled LINE preserves retry budget.
- Nonroot API and worker Docker builds, API DB readiness, real Docker HTTP
  registration/login/submission and separate-process inbox delivery, orderly stop.

Disposable Compose projects used:

- `sa-lathe-uc1-781657a2` — first PostgreSQL/S3 integration pass.
- `sa-lathe-uc1-e0775fce` — full regression and Docker flow.

Only these projects' newly created containers, networks and volumes were removed.
No user database was migrated or deleted. Test credentials were random process
environment values, not files or commits. No real LINE account was contacted;
LINE HTTP behavior was tested against a local mock server. Signed links and
tokens are not printed by the test suite.

Repeat with `./scripts/verify-uc1.ps1` (Docker required). It creates fresh isolated
services and runs the real integration suite and Docker flow; it never uses a
local .env or existing database. Failed tests clean up their own fresh volumes.

Operational limitations: no malware scanner or email verification in this phase;
LINE needs private credentials and an eligible Admin recipient; S3 CORS, private
policy, lifecycle and failed-outbox monitoring must be set for production. See
UC1.md. Phase 5 remains unapproved.
