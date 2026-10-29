# Real-backend workflow testing data

Owner authorized representative local workflow data on 2026-10-29. This extends
the six accounts; it does not replace frontend mock mode, add accounts or alter
UC1-UC9 rules. No production or original `sa_lathe_shop` data is targeted.

## Scenarios

All request descriptions start `[DEMO rN]`; catalog/suppliers use `DEMO` names.
Deadline dates are real future instants relative to the first seed run, not
backdated audit events. Nida and Ploy share the company; Somchai is isolated.

| Key | Customer | Initial state / tester action |
| --- | --- | --- |
| r1 | Somchai | Pending request and private specification; Worker estimates |
| r2 | Nida | Draft quotation; Worker edits/sends |
| r3 | Ploy | Rejected first quotation and revised sent quotation; Customer decides |
| r4 | Nida | THB18,000 accepted; submitted PO pending review, deposit missing |
| r5 | Somchai | THB2,400 accepted; THB1,200 deposit pending review, no job yet |
| r6 | Nida | THB8,000 PO approved; queued job, SUS304 shortage10 |
| r7 | Ploy | THB6,400 PO approved; queued job, S45C5 fully reserved |
| r8 | Somchai | THB4,800, deposit2,400 approved; running job, S45C2 used, time recorded |
| r9 | Nida | THB9,600 PO approved; completed job/costs, no invoice yet |
| r10 | Somchai | THB7,200, deposit3,600; completed, invoiced, third-party delivery; bank3,600 pending Admin review |
| r11 | Ploy | THB6,400, PO-only; completed/invoiced/self-delivered, cash paid and accepted; request closed |
| o1 | Admin | SUS304100 supplier order awaiting external acceptance; references r6, excess stock permitted |
| o2 | Admin/Worker | S45C20 accepted: good15/damaged5; replacement claim submitted, no payment |
| o3 | Admin/Worker | AL6061 10 fully received, 14-day credit, unpaid |

Totals: 11 requests, 11 quotations (including the r3 revision), 6 jobs,
5 customer PO submissions, 3 deposits, 2 invoices, 2 customer balance payments,
3 units, 6 materials, 3 suppliers, 3 supplier orders, 2 receipts and 1 claim.
There are 20 actual private test documents: 11 specifications, 5 POs,
3 deposit proofs and 1 balance-payment proof. These are supported TXT files,
clearly labelled DEMO / TEST ONLY, not real bank receipts, production drawings
or rendered invoice/PO PDFs. Signed downloads open the actual stored files.

## Safety and ownership

- Explicit CLI only; never called by API/worker startup. No new HTTP endpoint.
- Requires existing account provenance, an active matching identity manifest,
  loopback API/S3 origins, guarded local/test DB URL/name and opt-ins below.
- Requires clean migration14. Account seeding accepts clean13 or14; no schema
  change to business rules or demo login. Migration14 adds only an operation journal.
- Repository validates actual DB, then login checks all six real API account UUIDs
  against that DB; wrong-target/name matches are never adopted.
- Session advisory lock permits only one workflow seed process per DB. Normal
  API/worker remain operational. Do not intentionally edit seed scenarios during
  their initial construction; a version conflict stops the seed, never forces it.
- All business writes go through existing HTTP handlers/services/repositories:
  permissions, pricing, acceptance, PO/deposit approvals, reservations, stock
  ledger, production, invoices/payments, audit and outbox are real.
- Money/quantities are exact decimal strings, IDs are real UUIDs and instants UTC.
  No VAT, no SQL business inserts/status edits, no direct reservation/stock patch.
- Files are uploaded to real signed private storage and completed through APIs.
  Only metadata/digests/UUIDs enter the journal; no file bytes, passwords, session
  tokens, signed URLs or storage headers. Tokens remain in memory and are logged out.
- `demo_workflow_run` binds to the identity dataset. `demo_workflow_step` records
  exact requests and safe response/resource UUID roots for each named operation.
  Request/job children remain traceable via normal FKs and audited responses.

## Commands

Supply private process environment; this command does not read `.env` or infer
its DB from `DATABASE_URL`. Do not print/commit the private connection URL.

```powershell
$env:SEED_ENABLED='true'
$env:SEED_ENV='local'
$env:SEED_DATABASE_URL='<private loopback PostgreSQL URL for sa_lathe_shop_demo>'
$env:SEED_CONFIRM_DATABASE='sa_lathe_shop_demo'
$env:SEED_WORKFLOWS_ENABLE='true'
$env:SEED_WORKFLOW_API_ORIGIN='http://localhost:8080'
$env:SEED_WORKFLOW_S3_ORIGIN='http://localhost:19000'
go run ./cmd/seed-demo-workflows plan
go run ./cmd/seed-demo-workflows apply
go run ./cmd/seed-demo-workflows verify
```

Operator prerequisites: migrate the dedicated demo DB through14; seed accounts;
API uses the same DB and `LOCAL_TEST_ACCOUNT_ENABLE=true`; private S3 is ready
and its browser/signed origin matches SEED_WORKFLOW_S3_ORIGIN; worker runs with
LINE disabled. No seed passwords are needed for this workflow command.

## Rerun, partial failure and retirement

Completed `apply` reads the journal without API logins/business writes: no
duplicates, password reset or state restoration over tester changes. `verify`
reads real current records and checks access/private files; it does not insist
that testers have preserved the initial business states. Both share the normal
10/minute/IP login limit; wait for its window when performing repeated verifies.

Before each operation, the pending request is durably committed. Completed
steps return recorded results. Pending idempotent commands retry the SAME
stored key and body, not new versions/keys. Requests, catalog creation, quote
creation/decisions and combined uploads do not all expose retry keys: an unknown
outcome stops with the step key and requires reconciliation. The command does
not guess whether a timeout committed or adopt a similar-looking record.

For an uncertain operation, retain the journal and inspect the exact actor,
path/body and real API/DB history before an operator recovery decision. No
automated reconcile/delete/reset is provided. Interrupted uploads can leave
private staging objects for an authorized retention process; never fake ready
state or silently remove evidence. Schema14 down refuses when run provenance exists.

There is no destructive workflow cleanup: immutable audits, receipts and stock
must stay consistent. Existing account cleanup refuses used dependencies.
Explicit account seed retirement revokes the six accounts/sessions and preserves
all workflow data; it is not a reset. Never TRUNCATE or cascade-delete the dataset.

## Verification

`./scripts/verify-demo-workflows.ps1` uses a uniquely named fresh Docker project,
new databases/volumes and process-only generated credentials. It verifies real
private upload/download/anonymous denial, all scenario transitions, stock/WAC,
pending-payment balance, paid/accepted closure, company isolation, Worker finance
redaction and Admin-only procurement. It checks duplicate-free reruns, preserving
an extra tester stock addition, and a separate readback after restarting only
the disposable API. It removes only its generated resources.

Unit/race tests cover disabled/mismatched guards, foreign object origin refusal,
completed no-op steps, exact idempotent replay and refusing ambiguous non-idempotent
checkpoint failures. They are not a substitute for the Docker verification.
The verifier is also in CI; do not claim hosted CI green without observing it.

On2026-10-29 disposable project `sa-lathe-workflows-99e7e3e9` passed, including
the additional tester stock entry remaining after rerun; its fresh resources
were removed. The separately running local `sa_lathe_shop_demo` then completed
all141 steps and passed private document/state/isolation readback and an exact
reused apply. API readiness remained200; original shop data was not migrated.
Two initial harness assumptions were corrected before live execution: automatic
API ports can change on restart, and full-receipt supplier credit is reported
as `pending` until settled. These are test fixes, not changed business rules.
The existing guarded account verifier also passed against schema14, including
cleanup refusal/retirement and normal/password-free login/revocation/default-off
route checks. The live demo readback confirms 11 requests, 6 jobs, 5 POs,
3 deposits and clean schema14. No full prior ten-suite UC regression or browser
UI test is claimed in this workflow-fixture follow-up.

Frontend can use existing live screens, private-download endpoints and all six
login buttons. Refresh real API data; never merge frontend mock rows into these
lists. No frontend code or browser-to-real-API verification is included here.
