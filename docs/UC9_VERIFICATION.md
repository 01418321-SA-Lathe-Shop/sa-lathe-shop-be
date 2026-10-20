# Phase 12 UC9 verification — PASSED

Verified on 2026-10-20. First complete isolated run sa-lathe-uc9-98304677 and
expanded final run sa-lathe-uc9-ace1e2f1 both exited 0. Each emitted
UC9_DOCKER_FLOW_PASSED, PHASE12_VERIFICATION_PASSED and
WORKER_CONTACTS_VERIFICATION_PASSED. Both runs removed all four containers, both
fresh volumes and their unique networks; process environment was restored.
Build, vet and ./tasks.ps1 lint pass. No existing database/runtime data changed,
production migration/deployment performed or real LINE message sent.

Initial attempt sa-lathe-uc9-cc5f3096 stopped before migrations because the Go
module proxy timed out; its resources were removed. GOPROXY=off also cannot
resolve this Go command's deprecation lookup. Successful runs used the same pinned
v4.19.0 tool through the existing file module-cache proxy, only for that process,
with no global Go config change, version change or disabled checksum verification.

## Commands

```powershell
go build ./...
go vet ./...
./tasks.ps1 lint
./scripts/verify-uc9.ps1
```

When the online Go proxy was unavailable, the actual verifier invocation used:

```powershell
$previousProxy = [Environment]::GetEnvironmentVariable('GOPROXY', 'Process')
try {
    $cachePath = Join-Path (go env GOMODCACHE) 'cache/download/'
    $env:GOPROXY = ([uri]$cachePath).AbsoluteUri
    ./scripts/verify-uc9.ps1
} finally {
    [Environment]::SetEnvironmentVariable('GOPROXY', $previousProxy, 'Process')
}
```

This requires the pinned tool/dependencies already cached; it is not a new default
or a replacement for normal dependency downloads. Actual local cache was under
C:/Users/win/go/pkg/mod/cache/download. Missing cache still fails explicitly.

The verifier creates only a unique sa-lathe-uc9-* Compose project and ten fresh
dedicated databases, with random process-only credentials and an empty temporary
Compose environment. It applies migrations 1-12, tests empty 12/11/10 down/up,
preserves/backfills legacy Worker contacts and runs uncached real PostgreSQL/S3
tests through UC9. API/worker Docker builds then run customer-to-production,
invoice/delivery/return/rework/redelivery/bank review/final acceptance and inbox
dispatch, followed by graceful stop and scoped containers/volumes/network cleanup.
Existing databases, .env, Docker runtime folders and real LINE are never targets.

## Passed UC9 checks

- Exact explicit invoice terms, one invoice, customer/company scope and Worker
  financial/evidence privacy; no re-crediting the deposit.
- Return with immutable cycle history, explicit extra use/time, re-completion,
  redelivery and no stock restoration or invoice/payment changes.
- Private S3 bank proof/unsigned denial, pending versus approved receipts,
  installments, concurrent same-key cash receipt, no overpayment and paid+accepted
  request closure. Zero balance needs no fake payment.
- Competing approvals, forced audit rollback, immutable history, closure guard,
  no UC9 LINE and recipient scoping; strict real authenticated HTTP, one-connection
  reads, guarded populated rollback and all earlier-phase regressions.
- Any active company member accepts/returns; removed membership and disabled
  account fail live access. Removed members get no deferred UC9 notifications.
- Two return cycles retain three production-cycle identities without changing
  original invoice or physical stock. Concurrent accept/return has one winner.
- Competing pending payments cannot overpay even with a refreshed version. Admin
  can reject an outstanding pending proof after cash settles/closes the request;
  that rejection changes no approved amounts. Same-key replay keeps old snapshot.
- Forced outbox insertion failure restores receipt, event and version together.

Phase 12 implementation and real integration verification are complete. Production
deployment and latency/load SLAs are not part of these functional checks. Real
LINE, post-acceptance warranty/refunds, partial shipments, PDF rendering and payment
gateway execution are not implemented or claimed; invoice data is served as JSON.
