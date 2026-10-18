# Phase 10 UC7 verification — IN PROGRESS

Build, vet, unit/service/privacy tests and ./tasks.ps1 lint pass. Ordinary
go test ./... passes, but dedicated PostgreSQL/S3 integration is skipped unless
its environment is configured. Do not treat those skips as integration success.

The first ./scripts/verify-uc7.ps1 attempt stopped at its Docker availability
probe before any test resource/data creation. The per-user Docker Desktop was
stopped. Starting it exposed the previously-recorded Docker 4.86.0 stale IPC
socket failure at Docker/run/dockerInference. The socket is a zero-byte reparse
point; the paired docker-secrets-engine/engine.sock is the same artifact type.
No factory reset, socket deletion, settings/WSL/volume/image/VHDX change was made.
Owner approval was requested before repeating the earlier directory-rename recovery.

## Verification command

```powershell
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
./scripts/verify-uc7.ps1
```

The script uses a unique sa-lathe-uc7-* Compose project, random private process
credentials, an empty temporary environment and eight fresh dedicated databases.
It applies migrations 1-10, tests empty migration 10 down/up, preserves legacy
contacts, runs all uncached staff/UC1-UC7 PostgreSQL/S3 tests and builds/runs API
and worker. It removes only its own fresh test resources and restores process
values. No existing database, local .env or real LINE recipient is used.

## Added checks (execution pending the Docker recovery)

- Admin/Worker receiving and claims, Worker finance/evidence denial and explicit
  allowlist redaction in legacy inventory/movement HTTP responses.
- Same-key concurrent mixed receipts create one shipment/ledger/defect. Excess
  quantities and changed retry payloads fail without duplicate stock.
- Original damaged quantities remain outstanding; claim-before-replacement, bad
  replacement without extra liability, good-only completion and exact moving cost.
- Original replay result after subsequent replacement/payment; current GET state.
- Full receipt timestamp and explicit credit days determine due time without reset.
- Bank evidence required; cash evidence optional; borrowed/failed order evidence
  cannot settle. Bad image content fails private S3 completion.
- One full payment only after full good receipt, bank settlement and cash race.
- Receipt-versus-cancel race, including cancellation denial after mixed receipt.
- Forced event failure restores stock, average cost, receipt, claim and order version.
- Append-only history, populated rollback refusal, one-connection read and disabled
  Worker denial; full earlier-phase stock/evidence/auth/inbox regression.
- Deferred receipt checks require the current matching order event and reconcile
  every claim's replaced total against good replacement rows.
- Docker mixed delivery -> claim -> good replacement -> credit/full cash settlement,
  exact physical stock/average and Worker financial redaction.

Do not mark Phase 10 done or claim PHASE10_VERIFICATION_PASSED until the complete
script exits successfully and cleanup is confirmed. Stop before UC8.
