# Phase 5 verification — 2026-10-15

Status: **in progress, not complete, not pushed**.

Passed locally: exact satang/pricing tests, quotation service authorization and
cost redaction tests, HTTP malformed/spoofed-body checks, existing unit suite,
and builds before each focused implementation commit. No float arithmetic or
VAT added. Database integration tests compile but are skipped without an
explicit disposable database connection; this is not an integration pass.

Pending: real PostgreSQL constraints/transactions/concurrency, migration up/down,
all staff/customer/S3 regressions against services and Docker API/worker flow.
The prepared script is scripts/verify-uc2.ps1; it now checks Docker readiness
with a bounded probe before creating any test resources.

Observed environment blocker: Docker Desktop startup fails while initializing
its Inference manager's local dockerInference socket (file inaccessible / invalid
listener path). CLI start reported starting but did not produce a healthy engine;
the installed per-user launcher reproduced the failure. No factory reset,
Docker file deletion, WSL configuration change or data deletion was attempted.
Only this task's waiting Docker CLI probes/test calls were cancelled.

Prepared real tests cover whole-request prices, private drafts, customer cost
redaction, cross-company denial, stale edits, concurrent sends/draft creation,
send-vs-refusal races, DB-enforced minimum prices and rollback of both parent and
cost inserts, forced send/request-update rollback, refusal audit and inactive
Worker protection. These cases require a healthy engine before being certified.

Next: start/repair Docker Desktop without resetting data, rerun the verification
script, fix any integration failures, finish documentation/checklists, then push
the separate phase commits to origin/develop and verify the tip. main must remain
unchanged. UC3 and later phases remain unapproved.
