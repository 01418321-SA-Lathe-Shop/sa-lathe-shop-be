# Phase 10 UC7 verification — PASSED

Verified on 2026-10-19 by the combined ./scripts/verify-uc8.ps1 run in isolated
project sa-lathe-uc8-a664942c. It exited 0 with PHASE11_VERIFICATION_PASSED and
WORKER_CONTACTS_VERIFICATION_PASSED. That verifier sets PHASE10_DATABASE_URL and
PHASE11_DATABASE_URL and runs all uncached PostgreSQL/S3 tests, including UC7,
then the UC7 receipt/claim/settlement Docker HTTP workflow and UC8 production.
The standalone UC7 script was not rerun; no PHASE10 marker is claimed.

Build, vet and ./tasks.ps1 lint also pass. All four test containers, both fresh
volumes and the network were removed by the verifier. No existing database was
migrated, local .env read, Docker runtime folder repaired or real LINE message sent.

The first ./scripts/verify-uc7.ps1 attempt stopped at its Docker availability
probe before any test resource/data creation. The per-user Docker Desktop was
stopped. Starting it exposed the previously-recorded Docker 4.86.0 stale IPC
socket failure at Docker/run/dockerInference. The socket is a zero-byte reparse
point; the paired docker-secrets-engine/engine.sock is the same artifact type.
No factory reset, socket deletion, settings/WSL/volume/image/VHDX change was made.
Owner approval was requested before repeating the earlier directory-rename recovery.

The first retry reached migration 000010 and exposed a PL/pgSQL syntax error in
the receipt-status CASE comparison. Parenthesizing that expression fixed it;
the fresh complete rerun applied migrations 1-11 and passed empty migration
11/10 down/up, all earlier regression tests and private S3 checks.

## Commands run

```powershell
go build ./...
go vet ./...
./tasks.ps1 lint
./scripts/verify-uc8.ps1 # internally: go test ./... -count=1 with dedicated DBs
```

The combined script uses nine fresh dedicated databases, random process-only
credentials and an empty temporary Compose environment. It preserves/backfills
legacy Worker contacts, builds/runs API and worker and restores process values.
See UC8_VERIFICATION.md for the combined run and its exact success markers.

## Passed checks

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

UC7's real integration gate is resolved by the successful combined run. UC9
remains unapproved; verification does not authorize a production deployment.
