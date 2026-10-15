# Phase 5 verification — 2026-10-15

Status: **implementation and full verification passed**. Push acknowledgement
is tracked in PLAN.md. UC3 and later phases remain unapproved.

## Passed checks

- `go build ./...`, `go vet ./...`, unit tests, HTTP parsing/permission checks.
- Exact whole-request 60% markup and upward satang rounding, string money JSON,
  minimum/overflow validation, no float arithmetic or VAT.
- Real PostgreSQL private drafts/cost redaction, company isolation, revision
  chain fixture, stale edit detection, concurrent sends and draft creation,
  send-vs-refusal race, minimum-price DB constraints, failed cost insert rollback,
  forced send/request-update rollback, refusal audit and inactive Worker denial.
- Staff auth and UC1 company/private-S3/inbox regression suites against separate
  fresh databases. These were actually executed, not skipped.
- Migrations 1-4, empty quotation rollback (4 down) and reapply.
- Nonroot API and worker Docker builds and DB readiness; real Docker HTTP
  customer registration/request creation, Worker price preview, draft/save/send,
  customer quote read without internal costs, worker inbox delivery and stop.

Executed `./scripts/verify-uc2.ps1`: PHASE5_VERIFICATION_PASSED, exit 0.
Isolated Compose project: `sa-lathe-uc2-0331d81e`. Its newly created containers,
network and PostgreSQL/S3 volumes were removed afterward. No existing user or
business database was migrated/deleted. Test secrets were process-only random
values, not files or commits. Real LINE delivery stayed disabled.

## Docker recovery (owner-authorized, non-destructive)

Docker Desktop 4.86.0 initially failed on inaccessible zero-byte runtime sockets:
dockerInference, then docker-secrets-engine/engine.sock. This matches the failure
class in [Docker's issue tracker](https://github.com/docker/desktop-feedback/issues/531).
The inference-only directory rename exposed the second stale socket. A graceful
stop timed out; only the failed Desktop/backend processes were closed after
confirming the engine had not started. Both socket-only parent directories were
validated and renamed aside, not deleted; the restarted engine responds as
29.7.2. No settings, WSL configuration, image/volume store or VHDX was reset.

Retained original runtime directories (relative to LOCALAPPDATA):

- Docker/run.stale-20261015-8bbeeb4f (first inference-only attempt).
- Docker/run.stale-20261015-69cf5889 (paired recovery).
- docker-secrets-engine.stale-20261015-b68bf411 (paired recovery).

These are retained IPC artifacts, not business-data backups. Do not restore them
while Docker runs. This recovery proves current engine availability, not a
permanent fix for the installed Desktop version; report recurrence rather than
automatically deleting sockets or resetting Docker. No diagnostic bundle was
uploaded externally.
