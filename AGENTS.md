# SA-Lathe-Shop-BE

Current task authorization (2026-10-28): implement approved demo-account seed
phases S0-S3 before resuming remediation. Six compatible frontend fictional
accounts only; omit legacy.staff (missing required contacts). Dedicated guarded
local/test databases only; no existing shop migration, hosted seed, deployment,
workflow fixtures, S3 provisioning, real LINE or frontend changes. Add ownership
metadata and reuse existing auth services/Argon2id. Never adopt unrelated accounts,
reset passwords on rerun or seed sessions. Publish buildable groups to develop.
See docs/015-demo-seed.md and PLAN.md. S4-S5 workflows remain optional/unapproved.

Current remediation authorization (2026-10-27): owner approved PLAN.md phases
14-21. Preserve existing UC1-UC9 APIs, business rules and exact retries; add
opt-in compact modes rather than breaking clients. Use trusted proxies only
when explicitly configured, benchmark before claiming performance gains, and
retain Go/chi/PostgreSQL/private-S3/outbox without new runtime infrastructure.
Phase 14 patches Go to 1.27.3 and x/text to v0.41.0, pins both builder image
references and adds source/binary vulnerability gates plus response/retry
characterization tests. Phase 15 now has corrected measured benchmarks and an
unconditional real DB/S3 integration CI job; all ten required suites must execute
and pass. The completed synthetic baseline is in docs/012-performance.md. Phase
16 error/validation work is implemented and unit/race verified; transaction helper extraction will accompany
the Phase 17 production-query changes to avoid rewriting those steps twice.
See docs/013-remediation-verification.md for results.
Phase 17 now implements bulk scoped fulfillment, focused production validation
and batched material issues with unchanged locks/triggers/retries. Real workflows
and paired measurements pass; preserve host-load caveats, not a production SLA.
Phases 18-21 remain authorized but unimplemented. Next is opt-in compact responses
and history pagination without breaking legacy APIs or pruning immutable snapshots.
Continue subsequent approved phases in order, with buildable logical commits
and a verified origin/develop push at every phase end. No main push, existing
database migration, production deployment or real LINE delivery is authorized.
Older phase-specific notes below are historical context, not permission to
change the confirmed business contracts.

Current authorization (2026-10-20): owner confirmed all seven UC9 proposals.
Phase 12 implementation is authorized; see docs/UC9_PLAN.md for the confirmed
delivery, invoice/installment/evidence, free rework, closure and inbox policies.
This supersedes all older UC9-unapproved/planning-only notes below. UC9 is now
implemented, real PostgreSQL/S3/Docker verified and published to develop at
c487fa2; main remains 86d5be1. See docs/UC9_VERIFICATION.md. Current binaries need
migrations through 000012; no existing database was migrated by verification.
Keep develop-first grouped commits; no permission to push main or deploy.

Made-to-order lathe shop backend. Phases 3-12 (auth and UC1-UC9) are implemented and verified. The combined PostgreSQL/private S3/Docker run sa-lathe-uc8-a664942c passed on 2026-10-19 and cleaned up its fresh resources; see docs/UC7_VERIFICATION.md and docs/UC8_VERIFICATION.md. UC9 is implemented under the current authorization above. UC6 is Admin-only, including supplier management and procurement reads/history. One order has one supplier, several materials and flexible zero/many job references (owner option A); allow excess quantities/general replenishment, no fixed allocations, shortage cap, incoming coverage or UC5 plan blocking. Placed quantities/prices/references are immutable; cancel/recreate unreceived orders to change. Admin records external supplier acceptance/rejection; rejection cancels the unreceived order with reason. Round each line half-up to two decimals, then sum; NUMERIC(18,6) inputs and NUMERIC(18,2) THB totals, no VAT. UC6 procurement never changes physical stock, reservations or average cost. UC5 policies remain confirmed: Worker-editable queued-job material plans with partial allocation/shortages; Admin catalog and audited stock management; queued release/reallocation only; additions recalculate weighted-average cost and count corrections preserve it. UC4 rules remain confirmed: both Admin/Worker verify evidence; a single deposit may be the rounded 50% minimum through 100%; timely complete evidence may be reviewed later; missing evidence expires; exactly one queued job opens after approvals. Confirmed quotation pricing: minimum = (material + labor) * 1.60 for the entire requested quantity, two decimals, minimum rounded upward to a satang, no VAT. Any active company member may accept/reject company quotations. Round a 50% deposit upward to a satang.

See docs/AUTH.md and docs/UC1.md through docs/UC8.md. Customers self-register; company users join by one-use invitation and share company request reads and quotation decisions. Current active membership, not ownership, authorizes company decisions/evidence submissions. PDF, DOC/DOCX, XLS/XLSX, TXT, JPEG/PNG only, maximum 100 MB. LINE alerts target one configured Admin user; inbox notifications are per-account. Never infer company membership from a name or tax ID alone. UC3 terms remain immutable; exact retries never reset deadlines. UC4 binds private ready files to deposit/PO submissions, records one staff review and opens a queued job atomically after all required approvals. UC5 uses live transactional stock, exact quantities and row-locked job/material plans. Reservations do not consume physical stock. UC7 and UC8 policies and real integration checks are confirmed. Final invoicing/UC9 is confirmed and implemented.

## Stack

UC8 owner confirmations: all five policies in docs/UC8_PLAN.md are approved. Any active Worker may start/log/complete without assignment restrictions; Admin monitors and enters actual labor cost only. Start requires nonempty fully reserved plan and current plan version, no automatic use. Actual incremental use consumes own reservation first then free stock, including extra/unplanned material; never steal other reservations. Completion releases unused reservations without adding physical stock; photos optional. Manual whole labor minutes, issue-time material valuation and Admin-entered actual labor cost, no assumed rate. Migration 000011 and production APIs/private photos passed build/vet/unit/lint and real PostgreSQL/S3/Docker verification. Preserve existing layers and the confirmed UC9 contract. Apply migrations through 000012 before current binaries; verification did not migrate any existing database or authorize deployment.

UC8 authorization/verification update (2026-10-19): owner authorized UC8 after the UC7 checkpoint and confirmed all five policies. The later requested Docker retry passed full UC7/UC8 integration; this supersedes earlier pending-verification and stop-before-UC8 wording. Docker responded without runtime repair. Do not infer approval to rename/delete/reset runtime folders; preserve retained socket artifacts, settings and existing data. UC9 is implemented under the current authorization above.

UC7 owner confirmations (2026-10-19): Admin and Worker receive and manage replacement claims; supplier prices, stock costs, credit/payment details and payment evidence remain Admin-only. Accepted orders only; partial receipt, no excess. Damaged units stay outstanding until good replacements arrive; replacement only, no refund/write-off. Full good receipt sets the immutable credit starting time. Admin enters credit days per order, no default; one full cash/bank settlement after full receipt. Bank proof required, cash proof/photo optional. No automatic UC5 allocation. Implementation and database/S3/Docker verification are complete. See docs/UC7.md and docs/UC7_VERIFICATION.md. The earlier planning questions are resolved, not permission to broaden Worker financial access.

- Go, net/http, github.com/go-chi/chi/v5.
- UC9 migration 000012: one immutable post-deposit invoice; Admin-only explicit terms/review/cash, staff delivery/invoice, customer inspection/bank evidence; exact installment allocations, audited free pre-acceptance rework cycles, accepted+paid request closure, scoped in-system outbox. Verification passed twice on 2026-10-20. No new LINE/reminders/refunds/partial delivery, post-acceptance warranty, carrier API, payment gateway or invoice PDF renderer. Preserve current private-file/role boundaries; see docs/UC9.md.
- PostgreSQL with pgx/v5 isolated behind repository implementations. Money: NUMERIC, never float. Stock reservations use transactions and row locking.
- golang-migrate SQL migrations, pinned Go tool. Migrations 000002 (staff), 000003 (customer/UC1), 000004 (quotations/UC2) and 000005 (decisions/UC3) are implemented. Apply migrations before deploying binaries that use them.
- Migration 000006 implements required Worker contacts. Admin-created Workers need valid email and phone; contacts are non-unique and not login identifiers. Reuse person.email/phone; never fake legacy backfills or disable existing accounts. See docs/AUTH.md for exact syntax, nullable legacy responses and rollback limits.
- Migration 000007 implements UC4 evidence, jobs and system cancellation audit. Apply before both API and worker. Deadline scanning runs in the separate worker: protect timely complete pending/approved submissions; missing/rejected evidence expires after 336 hours. Do not change historical terms or silently extend deadlines.
- Migration 000008 implements UC5 inventory. Quantities/per-base-unit costs use NUMERIC(18,6), never float. Average costs round half-up to six places only on additions; never silently round entered quantities. Request/job/material lock ordering, optimistic versions and idempotency protect reservations. Reuse the base unit; do not invent conversion factors or consumption transitions.
- Migration 000009 implements UC6 supplier/audit, immutable order/item/job-reference and command/event records. Live Admin checks, retry-key transaction locks and order row locks protect commands. Retries return original snapshots; GET reads current state. No stock effect, new outbound event or receiving/payment code. UC7 must extend receipt states and prohibit cancellation after any receipt under the same order lock. See docs/UC6.md.
- Migration 000010 adds UC7 partial/good/damaged receipts, replacement-only claims, linked audited stock movements, explicit credit terms, one full supplier settlement and Admin-only order evidence. Migration 000011 adds UC8 lifecycle, actual usage/time/cost, consumption-aware reservations, stock-issue audit and private photos. Apply both before current binaries. Combined isolated PostgreSQL/S3/Docker verification passed; UC9 is implemented under the current authorization above.
- S3-compatible private object storage (SeaweedFS locally, pinned official GHCR image digest). Store object keys and metadata only; signed upload/download URLs. Evidence never public. MinIO registries were unavailable during setup, so local provider was replaced without changing the S3 contract.
- Admin and Worker auth: Argon2id password hashing, opaque Bearer sessions checked in PostgreSQL on every protected request, and role permissions. Suppliers have no accounts. First Admin is provisioned through cmd/api -bootstrap-admin, never a public registration route.
- PostgreSQL transactional outbox and separate worker. No Redis, broker, or cache.
- Standard library environment config, log/slog, JSON parsing/validation. Configuration and external dependencies behind small interfaces.

## Architecture and layout

cmd/api and cmd/worker are composition roots. Feature packages under internal/{auth,quotation,job,material,procurement,delivery,payment}; each feature has handler, service, repository interfaces, and domain types. PostgreSQL implementations live under internal/repository/postgres. Services accept interfaces for mock testing.

Handlers parse/validate requests and write responses. Services implement business rules. Only repositories execute SQL or import the DB driver. Infrastructure connection bootstrapping belongs in the PostgreSQL repository package; entrypoints may compose it. Entities never access DB. Service transaction orchestration calls repository interfaces; no DB types escape into services.

internal/config: environment config. internal/platform/httpserver: shared HTTP support. internal/health: liveness/readiness. migrations: SQL. docs: requirements and setup. tasks.ps1: Windows tasks.

## Conventions

Use gofmt, explicit errors, contexts and cancellation. No global DB handles. snake_case SQL and JSON names. timestamptz for instants; durations are not timestamps. Monetary arithmetic must be exact. Private file access requires authorization before signing. No credentials in logs. Transactions must atomically persist stock/payment changes and associated events. Use parameterized SQL, consistent JSON errors and idempotency for later financial/stock commands.

## Run and verify

See docs/000-setup.md. PowerShell: ./tasks.ps1 test; ./tasks.ps1 lint; ./tasks.ps1 run; ./tasks.ps1 migrate-up. Build: go build ./...; vet: go vet ./.... Docker Compose requires private values from a local .env. Do not commit .env or local data.

API reference: docs/openapi.yaml is the route/authentication contract; docs/010-api-examples.md contains PowerShell request and private-upload examples. Read the detailed UC guide before changing a payload or role rule. Security review evidence and deployment-dependent follow-ups are in docs/011-security-review.md. For repeatable local read latency and SQL plan measurements, use docs/012-performance.md and scripts/benchmark-http.ps1; never benchmark against production or claim an SLA from laptop results. GitHub CI runs formatting, build, vet and the default test suite without secrets. Database/S3 integration verification remains the dedicated scripts/verify-uc*.ps1 workflow and requires fresh disposable resources.

## Commit and push rules

These are mandatory for every phase, not optional cleanup at the end:

1. Plan reasonable commit groups and commit completed logical units as work progresses. Each commit has one purpose, normally 3-8 related files. Keep dependencies in buildable order; include the tests needed to explain that change.
2. Never collect a whole multi-feature phase or several phases into one giant commit and push. Do not split cohesive changes artificially just to increase the commit count. A small documentation-only follow-up may be one focused commit.
3. Use short Conventional Commit messages describing what changed and why. Avoid vague messages such as update, changes or done. Make each group understandable, independently reviewable and easy to locate with git log or git bisect.
4. Run go build ./... before every commit and verify that the committed snapshot itself builds, not merely a working tree containing uncommitted dependencies. Run focused tests for each group and the full required checks at phase completion.
5. At the end of EVERY completed phase, update PLAN.md, review the staged diff, git status and git log --oneline, then push that phase's separate commits to the authorized branch. Do not accumulate completed phases locally. Standing owner instruction for every phase and follow-up: always push to origin/develop first, until the owner explicitly asks to push main. Permission to start/finish a phase is not permission to push main.
6. Verify the remote branch tip after pushing and report the branch, commit range and test results. If a push is blocked, report the reason and pending commits; never claim the phase was pushed. Never force-push or rewrite published history without explicit authorization.

Never include unrelated changes, secrets, outputs or local data. If no remote exists, ask the owner for its URL; never invent one. Preserve pre-existing user files and changes. PLAN.md must record phase status, decisions and problems. See docs/000-setup.md for examples.

## Open questions

- Customer self-registration and invitation-only company joining/shared request access are confirmed. Invitation lifetime defaults to 24h, owner-only issuance; production business identity verification and membership-transfer policy remain future work.
- Staff frontend session persistence/cookie strategy and trusted proxy handling require deployment/frontend decisions. Current auth uses opaque Authorization Bearer tokens, not cookies.
- Pricing confirmed: 60% markup on total material+labor costs, whole-request totals, two decimal THB amounts; round minimum upward to the next satang. No float arithmetic or VAT.
- Cancel on the sixth quotation rejection (more than five), confirmed by owner.
- Deposit/PO deadline is quotation acceptance time plus 14 days, confirmed by owner. Do not extend this rule to invoice terms without confirmation.
- LINE targets only the configured Admin user, never a group or broadcast. In-system notifications are per-user. Credentials remain private environment variables; real LINE delivery is disabled by default and was not tested against a real recipient.
- Hosting, expected load, performance targets, production malware scanning, bucket lifecycle/retention and failed-outbox monitoring remain deployment decisions. File allowlist and 100 MB limit are confirmed.
- UC4/UC9 cardinality is confirmed: one job per approved quotation/request and one invoice per job for actual post-deposit balance. Admin sets explicit credit days; customer installments require Admin approval, no overpayment. Accept plus full payment closes the request. See docs/UC9.md for return/evidence/notification permissions.
- Original Word/sequence files remain unavailable; Phase 4 used docs/01-backend.md, the original ER and direct owner clarifications. Resolve later-UC ambiguities before their phases; do not use unrelated project documents.

## Confirmed rules

Individual: first payment is 50% deposit, no PO. Company <= THB 10,000: PO only; above: PO plus 50% deposit. Company issues and uploads PO. Payment can follow job completion. Admin manages workers. Supplier payment usually settles once. Delivery records self/third-party method only; no tracking integration. Stock requirements, reservations and actual usage are separate concepts; unused stock remains available after reconciliation.
