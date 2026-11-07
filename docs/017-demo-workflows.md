# Real-backend workflow testing data

## Current ER-complete dataset (2026-11-06)

The current fixture is `workflows-v3`. It supersedes the v2 counts recorded in the historical sections below and is intended for a freshly recreated local/test database. It contains 14 requests, 8 jobs, 6 supplier orders, 4 invoices and 26 private PDFs across 235 checkpointed API operations. New current-state fixtures cover a rejected customer bank payment, returned delivery/rework, shop-rejected request, supplier-rejected/cancelled order, partial receipt and fully damaged receipt. See [ER-aligned demo dataset handoff](027-demo-dataset-handoff.md) for the current reset, seed and verification commands.

## Local realistic dataset refresh (2026-11-03)

The owner requested replacement of all visible sample business data and actual
PDF files. The active local API was connected to `sa_lathe_shop_seed_test`, not
`sa_lathe_shop_demo`. A verified custom-format backup of that test database was
saved outside the repository before it was recreated. This removed the prior v1
and v2 fixture rows and four tester-entered placeholder requests. The original
`sa_lathe_shop` database was untouched and contained no business rows.
The inactive `sa_lathe_shop_demo` was separately backed up, cleared of its old
v1 sample and migrated empty to schema19; the active API still uses
`sa_lathe_shop_seed_test`.

The refreshed active test database is clean schema19 with six newly seeded
local-test accounts and a completed `workflows-v2` run. It contains 11 Thai
manufacturing requests, 6 jobs, 3 material orders and 2 invoices. The fixture
uploads 20 item-specific, valid, private PDFs: 11 work specifications, 5 customer
POs, 3 deposit notices and 1 balance-payment notice. Every PDF has a proper PDF
structure and Thai text; its metadata is `application/pdf` and its ready object
was downloaded through the API's signed private URL and parsed successfully.
There are no TXT request attachments in the refreshed database. Payment
notices are clearly labelled as simulated records and are not bank-issued slips.
The fixture also saves the shop profile, both customer billing profiles,
both customer addresses (with a fictional contact name each),
quotation validity/notes and the structured details of all five customer POs
through the new printed-document APIs introduced by migration17.
The final local run completed 151 guarded API steps and verified all 20 private
PDF downloads against the embedded source files byte for byte.
The fixture PDFs are embedded under `internal/demoworkflow/assets/` so future
local/test runs do not depend on a separate file server. The normal fixture
command remains additive and never resets a database by itself. The one-time
database recreation was a separate owner-authorized local operation backed up
outside source control. The newly generated account passwords are stored in a
private local file, outside the repository; password-free local-test login stays
available while explicitly enabled.

The PDF source is `scripts/generate-workflow-pdfs.py`. To revise a document,
install ReportLab in a local Python environment and point
`LATHE_SEED_THAI_FONT` at a Noto Sans Thai TTF before running that script.
Review the regenerated pages visually and run the Go/PDF checks before
publishing the changed assets. The embedded documents are synthetic and
must not be presented as genuine bank or supplier issued documents.

Owner authorized representative local workflow data on 2026-10-29 and an
additional populated UI dataset on 2026-10-31. This extends the six accounts;
it does not restore frontend browser mocks, add accounts or alter UC1-UC9 rules.
No production or original `sa_lathe_shop` data is targeted.

## Scenarios

The current additive fixture is `workflows-v2`; request descriptions are
item-specific Thai production names. It can coexist
with a completed `workflows-v1` run, which remains immutable and is never reset.
Deadline dates are real future instants relative to the first seed run, not
backdated audit events. Nida and Ploy share the company; Somchai is isolated.

| Key | Customer | Initial state / tester action |
| --- | --- | --- |
| r1 | Somchai | Pending request and private specification; Worker estimates |
| r2 | Nida | Draft quotation; Worker edits/sends |
| r3 | Ploy | Rejected first quotation and revised sent quotation; Customer decides |
| r4 | Nida | THB18,000 accepted; submitted PO pending review, deposit missing |
| r5 | Somchai | THB2,400 accepted; THB1,200 deposit pending review, no job yet |
| r6 | Nida | THB8,000 PO approved; queued job, SUS304 25 mm plate shortage 10 kg |
| r7 | Ploy | THB6,400 PO approved; queued job, S45C Ø45 5 kg fully reserved |
| r8 | Somchai | THB4,800, deposit2,400 approved; running job, S45C Ø16 2 kg used, time recorded |
| r9 | Nida | THB9,600 PO approved; completed job/costs, no invoice yet |
| r10 | Somchai | THB7,200, deposit3,600; completed, invoiced, third-party delivery; bank3,600 pending Admin review |
| r11 | Ploy | THB6,400, PO-only; completed/invoiced/self-delivered, cash paid and accepted; request closed |
| o1 | Admin | SUS304 100 kg supplier order awaiting external acceptance; references r6, excess stock permitted |
| o2 | Admin/Worker | S45C Ø45 20 kg accepted: good 15/damaged 5 kg; replacement claim submitted, no payment |
| o3 | Admin/Worker | AL6061 10 kg fully received, 14-day credit, unpaid |

`workflows-v2` adds 11 requests, 11 quotations (including the r3 revision), 6 jobs,
5 customer PO submissions, 3 deposits, 2 invoices, 2 customer balance payments,
4 units, 13 materials with 15 per-material unit conversions, 3 suppliers, 3 supplier orders, 2 receipts and 1 claim.
There are 20 actual private PDF documents: 11 specifications, 5 customer POs,
3 deposit notices and 1 balance-payment notice. Signed downloads open the
actual stored PDF files. Quotation and invoice records remain structured system
data; this fixture does not add stored PDF exports for those records.
On a local demo database that already contains `workflows-v1`, applying v2 adds
this entire set without adopting, editing or deleting the earlier fixture or
tester-created records.

On 2026-10-31 the guarded local `sa_lathe_shop_demo` database was migrated through
schema17 and `workflows-v2` completed all 141 real API steps. The
existing incomplete v1 journal remained untouched. This local execution added
the full totals above; it did not reset any existing business record. A
read-only verification then passed all 141 recorded steps, private-document
checks, company isolation, Worker finance redaction, inventory reconciliation,
receiving states and pending/settled customer-payment states.

## Safety and ownership

- Explicit CLI only; never called by API/worker startup. No new HTTP endpoint.
- Requires existing account provenance, an active matching identity manifest,
  loopback API/S3 origins, guarded local/test DB URL/name and opt-ins below.
- The current v3 dataset requires clean migration26. Account seeding accepts clean13
  through26; no schema change weakens business rules or demo login. Migration14 adds
  the operation journal, and migration26 permits the separately owned `workflows-v3` key.
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

Operator prerequisites: migrate the dedicated demo DB through19; seed accounts;
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

## Materials and units (2026-11-04)

Customers, jobs, POs, quotations and prices are fictional. Materials, grades,
sizes and units follow Thai machine-shop stockists. Every material keeps stock
in กิโลกรัม (the unit steel is bought and priced in); alternative units are set
per material because weight per metre depends on section and density, so no
single kg-to-metre factor exists. Factors are theoretical weights rounded to the
gram: section mm² × length mm × density g/cm³ ÷ 1,000,000 (steel 7.85, SUS304
7.93, SUS316 7.98, AL6061 2.70, SAE660 bronze 8.93, C1100 copper 8.94, POM 1.41;
the SS400 angle uses the JIS table weight). A เส้น is a 6 m bar; the SS400
แผ่น is 4×8 ft (1219×2438 mm).

| Key | Material (base unit kg) | Opening stock / cost per kg | Alternative units |
| --- | --- | --- | --- |
| S45C | เหล็กเพลา S45C Ø45 มม. | 100 / 48 | 1 เมตร = 12.485, 1 เส้น = 74.909 |
| S45C16 | เหล็กเพลา S45C Ø16 มม. | 30 / 50 | 1 เมตร = 1.578, 1 เส้น = 9.470 |
| S45CPL | เหล็กแผ่น S45C หนา 40 มม. | 60 / 52 | — |
| SCM440 | เหล็กเพลา SCM440 Ø50 มม. | 0 | 1 เมตร = 15.413, 1 เส้น = 92.481 |
| SS400 | เหล็กแผ่น SS400 หนา 12 มม. (4×8 ฟุต) | 300 / 32 | 1 แผ่น = 279.955 |
| FLAT | เหล็กแบน SS400 50×6 มม. | 0 | 1 เมตร = 2.355, 1 เส้น = 14.130 |
| ANGLE | เหล็กฉาก SS400 50×50×5 มม. | 0 | 1 เมตร = 3.770, 1 เส้น = 22.620 |
| SUS304 | เหล็กกล้าไร้สนิม SUS304 แผ่นหนา 25 มม. | 0 | — |
| SUS316 | เหล็กกล้าไร้สนิม SUS316 เพลากลม Ø25 มม. | 0 | 1 เมตร = 3.917 |
| AL6061 | อะลูมิเนียม AL6061-T6 แผ่นหนา 20 มม. | 60 / 160 | — |
| BRONZE | ทองบรอนซ์ SAE660 เพลากลมตัน Ø40 มม. | 40 / 380 | 1 เมตร = 11.222 |
| COPPER | ทองแดง C1100 เพลากลม Ø20 มม. | 0 | 1 เมตร = 2.809 |
| POM | พลาสติกวิศวกรรม POM (อะซีทัล) แท่งกลม Ø50 มม. | 20 / 220 | 1 เมตร = 2.769 |

Jobs draw on stock that can make the part: r7 Ø40 bush from Ø45 bar, r8 Ø12
pin from Ø16 bar, r10 100×80×35 block from 40 mm plate, r6 insert from 25 mm
SUS304 plate, r9 15 mm plate from 20 mm AL6061, r11 Ø35 bush from Ø40 bronze.
A fresh v3 run's S45C Ø45 reconciliation is 112 kg on hand, 5 kg reserved, 48 THB/kg
average. A database that already completed an older fixture keeps its earlier
data: completed runs never rewrite tester changes. Recreate a dedicated local/test
database to install the complete v3 dataset without mixing fixture generations.

Verification on 2026-11-04 (disposable PostgreSQL/S3/API, migrations through
000022): every fixture step, the 15 conversions and the S45C Ø45 stock
reconciliation passed. `verify-demo-workflows.ps1` still stops on two checks
that predate this data change and need an owner decision, not a fixture edit:
commit db8e61c lets Workers read `/material-orders` (the verifier and UC6 docs
expect Admin-only 403), and now marks an order `received` when good + damaged
equals the ordered quantity (the verifier expects o2 `partially_received`).

Update 2026-11-07 (fresh `sa_lathe_shop_demo`, migrations through 000027): the
owner's UC6 change made Worker ordering intended, so the verifier now expects
200 for `anan.staff` and 403 for the three customers on `/material-orders`.
`apply` completed 235 steps and `verify` exited 0 (the o2 check passed); counts
were `14|8|6|4|3`, 13 materials and 26 private PDFs. Restart the API before
`verify` when `apply` ran in the same process lifetime: a second run in a row
hits the local-test login limiter (429).
