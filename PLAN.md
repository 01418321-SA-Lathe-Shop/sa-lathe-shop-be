# Implementation plan

## Phase 0 repo and docs — done
- [x] Inspect existing repository and remote; preserve existing README.md.
- [x] Read available schema and prior fully-read UC context; record missing originals.
- [x] Add ignore rules, environment template, AGENTS.md and setup documentation.
- [x] Create buildable module bootstrap so every commit builds.
- [x] Push phase commits (verified before Phase 4).
Decision: repository already initialized on main; README.md is untouched. Module and empty entrypoints are bootstrapped in phase 0 to satisfy build-at-every-commit. Problem: original UC Word attachment is missing; no sequence attachment is currently available. Phase push pending verification.

## Phase 1 project skeleton — done
- [x] Feature-oriented structure, API and worker entrypoints.
- [x] Config, structured logging, request ID/recovery/timeout/CORS.
- [x] Consistent JSON errors, graceful shutdown and GET /health.
- [x] Build, vet and HTTP/config tests.
- [x] Push phase commits (verified before Phase 4).
Decision: standard library env config/slog; chi/cors allowlist; middleware timeout produces JSON. Push blocked pending explicit remote confirmation requested by automatic approval review. Phase 0 local commits completed; remote is origin/main at Yoshi4489/SA-Lathe-Shop-BE.

## Phase 2 DB migrations and Docker — done
- [x] PostgreSQL pool and DB readiness endpoint.
- [x] golang-migrate and empty initial up/down migration.
- [x] API/worker Dockerfiles, PostgreSQL/private S3 Compose, PowerShell tasks.
- [x] Verify build/vet/tests/lint/API/DB, migrate up/down and container entrypoints.
- [x] Local batch commits; stop before auth/UC feature code.
- [x] Phase push: completed to the confirmed organization repository before Phase 4.
Decision: pgx/v5 in repository only; standard library config/slog/validation; golang-migrate v4.19.0 pinned CLI. Local S3 switched to digest-pinned official SeaweedFS after MinIO registry denial. Official Go/Alpine ECR mirrors resolved Docker Hub timeouts. Verified liveness/readiness 200, DB-down readiness 503 with liveness 200, anonymous S3 listing 403, up/down/up migrations and graceful API/worker stop. Temporary verification containers and newly-created volumes were removed; no existing project files/data deleted. No secrets persisted or committed. See docs/VERIFICATION.md.

## Phase 3 auth and roles — done
- [x] Staff schema, Argon2id passwords, opaque revocable sessions and RBAC.
- [x] One-time first-Admin CLI with hidden password prompt; no default account.
- [x] Admin creates/lists/edits/deactivates/reactivates Workers; no role promotion.
- [x] Login/logout/me endpoints; expiry and DB-backed activity/revocation checks.
- [x] Service mock tests, HTTP tests and real PostgreSQL integration/race checks.
- [x] Build/vet/tests/lint, migrations up/down/up and Docker auth verified.
- [x] Update runbook and local 3-8-file buildable commit batches.
- [x] Customer login was an open question at Phase 3 completion, resolved in Phase 4; suppliers have no login.
- [x] Push phase batches: completed before Phase 4.
Decision: Argon2id 64 MiB/3 passes/2 lanes; random 256-bit Bearer tokens, SHA-256 token digests only in PostgreSQL, configurable absolute TTL (default 12h). Login and Worker credential/activity changes serialize on the account row. Deactivation/password/username changes revoke sessions atomically; already-running requests may finish, subsequent requests reauthenticate. No session cache. Local login abuse limiter is bounded and trusts only the socket IP. See docs/AUTH.md and docs/AUTH_VERIFICATION.md. Phase 4 remains unapproved; current missing original UC/sequence files and customer authorization must be resolved before UC1 work.

## Phase 4 UC1 submit job request — done
- [x] Owner authorized Phase 4 on develop on 2026-10-14; working tree inspected and clean.
- [x] Read docs/01-backend.md, requirements baseline, attached ER schema and existing auth contracts.
- [x] Record owner decisions: customer login required; sixth rejection cancels; Deposit/PO deadline is acceptance plus 14 days; private object keys and metadata only.
- [x] Confirm customer account provisioning and company account cardinality: self-registration; multiple users per company.
- [x] Confirm upload categories (documents/images) and notification channels (LINE/in-system).
- [x] Implement UC1 request domain/service, mock tests and authenticated routes.
- [x] Confirm exact file allowlist, invitation-only company access, and LINE Admin user target.
- [x] Customer accounts and ownership authorization, without weakening staff sessions.
- [x] Request validation, private signed uploads/downloads, content checks and transactional outbox.
- [x] Per-user inbox and separate outbox worker, LINE Admin-only push with stable retry keys.
- [x] Mock/unit/HTTP and real PostgreSQL/S3 integration, staff regression, migration down/up and Docker checks.
- [x] Update runbook and verification documentation.
- [x] Create nine logical 3-8-file commits; independently build each exact commit tree.
- [x] Push completed implementation to develop only; remote 451a0e8 verified on 2026-10-15, main unchanged at 86d5be1.
Decision: customer self-registration, owner-issued one-use company invitations and company-shared request reads. Input follows ER: selected customer ID verified against membership, description, positive int64 qty, material_spec and optional future requested deadline. Text bounds 10,000/2,000 Unicode characters; no artificial quantity business cap. Final evidence uses distinct private keys and ETag-conditional copy after inspection. Requests/outbox and inbox/acknowledgement are atomic; LINE targets only the configured Admin user and is disabled by default. Real LINE messages were not sent. See docs/UC1.md and docs/UC1_VERIFICATION.md. Original detailed UC/sequence files remain unavailable; reviewed guide, ER and owner decisions cover this phase. Stop before UC2.
## Phase 5 UC2 estimate and quote — done
- [x] Owner authorized the next phase on develop on 2026-10-15; clean branch and current workflow rules reviewed.
- [x] Read the backend guide and requirements; identify quotation transaction/status rules and unresolved pricing basis.
- [x] Confirm 60% total-cost markup, whole-request totals and upward satang rounding.
- [x] Exact money domain and pricing unit tests, no float arithmetic or VAT.
- [x] Quotation/revision/cost schema and transactional repository with real DB constraints/rollback/concurrency verification.
- [x] Worker draft/edit/send and request rejection with reason; restrict customer reads to their company and hide internal cost breakdowns.
- [x] Handler validation/errors, API wiring, HTTP auth and company-scoped cost-redaction checks.
- [x] Full regression, disposable PostgreSQL/S3 integration, migration down/up, Docker API/worker and quotation flow.
- [x] Recover Docker non-destructively after owner authorization: preserve stale socket directories, no settings/WSL changes or existing data deletion.
- [x] Logical buildable commits during work; runbook and verification record updated.
- [x] Verify end-of-phase push to develop only: d2b94ba confirmed on origin/develop, main remains 86d5be1.
Decision: minimum price = (material+labor)*1.60 for the entire quantity, rounded upward to a satang. Exact decimal strings/NUMERIC(18,2), no VAT. Full checks passed on 2026-10-15 in isolated project sa-lathe-uc2-0331d81e; test resources cleaned up. Docker's runtime-socket blocker was recovered by reversible directory preservation; engine 29.7.2 responds. See docs/UC2.md and docs/UC2_VERIFICATION.md. Existing Docker data and settings were not deleted/reset. Quotation send publishes in-system only, with no unconfirmed outbound delivery. No UC3 customer decisions, rejection threshold handling, deposits, PO or job opening are implemented; the 14-day clock remains acceptance-based in a later phase. Push verification is the final pending handoff step; stop before UC3.
## Phase 6 UC3 verify customer terms — done
- [x] Owner authorized Phase 6 and reaffirmed develop-first pushes on 2026-10-15; clean develop branch inspected.
- [x] Read backend guide, UC2 API, current quotation/revision schema and commit workflow.
- [x] Confirm company quotation-decision authority and odd-satang 50% deposit rounding.
- [x] Decision domain/terms snapshot and focused tests, preserving existing cost redaction.
- [x] One-decision-per-quotation audit, request-scoped rejection count, sixth-rejection cancellation and row-lock transactions.
- [x] Acceptance time/deadline and required Deposit/PO terms; no payment or job-opening implementation.
- [x] Authenticated customer decision/history endpoints, strict validation and replay/concurrency guards.
- [x] Real PostgreSQL races, boundary amounts, migration/Docker and earlier-phase regression checks.
- [x] Update docs, preserve logical buildable commits, push to develop and verify remote tip; never push main without a new explicit request.
Decision: any active company member may accept/reject; round 50% deposits upward to a satang. Count distinct customer rejections per request across revisions; the sixth cancels. Deposit/PO deadline is acceptance plus 336 hours, never reset by replay. Build/vet/unit, real PostgreSQL races/rollback/boundaries, earlier-phase regressions and Docker flow passed in disposable project sa-lathe-uc3-f2ced38b; its test resources were cleaned up. See docs/UC3.md and docs/UC3_VERIFICATION.md. No payment, PO upload, job opening or new LINE event was implemented. The previous publication hold is resolved: origin/develop was independently observed at 7e10423, containing the UC3 implementation plus Worker-contact requirement documentation; main remained 86d5be1. Stop before UC4.
## Frontend Worker contact follow-up — done

- [x] Owner reconfirmed sixth-rejection cancellation; remove stale fifth-rejection wording from docs/01-backend.md flow/status/business-rule/UC3 sections.
- [x] Record required Worker name/email/phone and confirmed Admin add/edit/deactivate/reactivate/password-change permissions in docs/AUTH.md and docs/requirements.md.
- [x] Preserve existing username/password/employee_no requirements and optional position; implement the required contact contract without widening staff permissions.
- [x] Implement and verify Worker email/phone input, persistence, response and required-field validation before full frontend Worker-management integration.
- [x] Owner confirmed Admin-created Workers require both fields, email syntax validation, international phone support and non-unique contacts.
- [x] Preserve legacy nullable contacts/accounts without fake data; Admin may backfill through PATCH. Reject null/blank contact removal; retain omitted fields.
- [x] Reuse person contacts; migration 000006 removes obsolete unique indexes and checks new Worker inserts. Guard rollback when shared contacts exist.
- [x] Build/vet/unit/HTTP, real PostgreSQL constraints/rollback, legacy migration up/down/up and Docker UC1-UC3 regression all passed.
- [x] Push separate buildable contact commits to develop and verify remote tip; main remains unauthorized.

The earlier documentation-only gap is now implemented as an owner-authorized auth follow-up, not UC4. Phone formatting is normalized to optional + and 7-15 digits; email uses net/mail, maximum 254 bytes. Neither is unique or ownership-verified. Contact changes do not alter existing password/session rules. See docs/AUTH.md.

Verification: full isolated project sa-lathe-uc3-4a16ab7c passed with PHASE6_VERIFICATION_PASSED and WORKER_CONTACTS_VERIFICATION_PASSED; resources cleaned up, real LINE disabled, no existing database touched. Publication hold was resolved by observing 7e10423 already on origin/develop. Focused follow-up commits 34fb391 through ce5a889 were pushed to origin/develop, with ce5a889 verified remotely; main remained 86d5be1. This acknowledgement is a documentation-only follow-up pushed to the same branch. See docs/WORKER_CONTACTS_VERIFICATION.md.

## Phase 7 UC4 deposit and company PO — done
- [x] Owner authorized the next phase on 2026-10-18; inspect clean develop at bdf3f75 and read repository rules.
- [x] Review backend guide, original ER, acceptance terms, private upload infrastructure and API/worker composition roots.
- [x] Confirm verification roles, permitted deposit amounts, deadline behavior and job cardinality; do not invent financial rules.
- [x] Add exact deposit/PO evidence domain, validation, audit and private signed upload/download handling.
- [x] Add PostgreSQL migration and transactional repository; lock request/terms/evidence and prevent duplicate job opening under retries/races.
- [x] Add authenticated scoped submission/review/read routes and queued job creation when all confirmed requirements are fulfilled.
- [x] Add confirmed deadline handling; do not cancel timely complete pending evidence or add unconfirmed LINE events.
- [x] Verify service/HTTP rules, PostgreSQL races/rollback, S3 evidence privacy, migrations and full earlier-phase/Docker regression.
- [x] Update documentation, commit focused buildable groups, push to develop only and verify remote tip.
Decision: owner confirmed both Admin/Worker review, one deposit between the rounded 50% minimum and 100%, later review of timely complete evidence, automatic cancellation for missing evidence, and exactly one queued job per approved quotation/request. This overrides the old ER's one-to-many job relationship. Reuse persisted UC3 terms and private ready attachments; technical upload completion is not payment verification. Migration 000007 adds term_evidence, job and uc4_cancellation with constraints and immutable review audit. Request row locks serialize retries, opposite reviews, expiry and job opening. Actual approved deposit determines the job balance, not the minimum deposit. Both isolated runs passed; expanded final project sa-lathe-uc4-92badc31 returned PHASE7_VERIFICATION_PASSED, and its test containers/volumes/network were removed. See docs/UC4_VERIFICATION.md. No real LINE messages or existing database changes. Focused commits 57f3555 through 754210b were pushed to origin/develop and 754210b was verified remotely; main remained 86d5be1. This acknowledgement is a separate documentation-only develop follow-up. Stop before UC5. Original detailed Word/sequence sources remain unavailable.
## Phase 8 UC5 stock check and reservation — done
- [x] Owner authorized the next phase on 2026-10-18; inspect clean develop and verified remote tip 0381004.
- [x] Read repository rules, UC5 guide/open questions, original stock ER and current queued-job schema.
- [x] Confirm persisted editable job requirements, master-data/stock-adjustment permissions, queued release/edit behavior and weighted-average addition/correction policy.
- [x] Exact quantity domain, shortage/allocation policy and mock tests; no floats or speculative unit conversion.
- [x] Material/unit/requirement/reservation and audit migration, with non-negative available stock and FK/uniqueness guards.
- [x] Transactional repository: lock jobs and material rows in deterministic order; prevent over-reservation and duplicate retries.
- [x] Authorized material search, job material-plan/stock-check/reservation endpoints and approved Admin catalog/stock commands.
- [x] Verify competing jobs, partial stock, retries, plan reductions/releases, rollback, permission isolation and earlier-phase/Docker regression.
- [x] Update API/verification docs; focused buildable commits, develop-only push and verified remote tip.
Decision: owner confirmed all four inventory policies. Requirements, active reservations and actual consumption remain separate. Worker plans partially allocate free stock and expose shortages without consuming on-hand quantity; reductions/releases and explicit top-ups are queued-only. Admin additions require quantity/unit cost and recompute average cost; absolute count corrections preserve cost and may not undercut reservations. Migration 000008 uses NUMERIC(18,6), database aggregate/ledger guards and immutable audits. Canonical same-key retries replay original snapshots; GET reads current state. No unit conversion, supplier order/receipt, production consumption or new LINE event was added. Both isolated runs passed; final project sa-lathe-uc5-1b78985e returned PHASE8_VERIFICATION_PASSED, then its fresh containers/volumes/network were removed. See docs/UC5_VERIFICATION.md. Focused implementation commits e9fb13a through 6e81e63 were pushed to origin/develop and 6e81e63 was verified remotely; main remained 86d5be1. This acknowledgement is a separate documentation-only develop follow-up. Stop before UC6.
## Phase 9 UC6 order materials — done
- [x] Owner authorized UC6 on 2026-10-18; inspect clean develop at 61984c3 and current repository rules.
- [x] Review UC6 guide/role contradiction, original supplier/order ER and UC5 live shortage/reservation contract.
- [x] Confirm ordering roles, supplier-master ownership, flexible job linkage (option A), supplier responses and price rounding.
- [x] Exact order totals/quantity validation and role/domain/mock tests; retain base units and no VAT.
- [x] Supplier/order/item/audit migration and transactional repository with idempotency and concurrent duplicate-command prevention.
- [x] Admin-only supplier search/management, order creation/read/history and supplier acceptance/rejection recording; no Supplier login.
- [x] Flexible many-job references: allow excess orders; preserve Worker UC5 edits without incoming allocations, shortage caps or physical-stock changes.
- [x] Verify PostgreSQL races, rejection/cancel/reorder, stale roles/versions, rollback, migrations and earlier-phase/Docker regression.
- [x] API/verification documentation, focused buildable commits, develop-only push and verified remote tip.
Decision: owner confirmed Admin-only procurement, many-job/excess orders, recorded acceptance/rejection, immutable prices/quantities and half-up per-line totals. Subsequent A confirmation selects flexible references without fixed allocation or UC5 plan blocking. Supplier remains external/no account. Migration 000009 adds supplier/audit and immutable material_order/item/job/event records; six-place quantities/costs and two-place THB totals, no VAT. Retry-key locks, row locks, expected versions and immutable original result snapshots prevent duplicate commands, not intentional replenishment with new keys. New orders preserve supplier/material/unit snapshots. No physical stock/average-cost/reservation change, receipt/payment or new LINE event. No UC5 plan code was changed. Both isolated runs passed; final expanded sa-lathe-uc6-bd002774 returned PHASE9_VERIFICATION_PASSED, then its fresh containers/volumes/network were removed. See docs/UC6_PLAN.md, docs/UC6.md and docs/UC6_VERIFICATION.md. No unresolved implementation blocker; original detailed Word/sequence sources remain unavailable as previously recorded. Focused buildable implementation commits dddda1f through 258cb33 were pushed to origin/develop and 258cb33 was verified remotely; main remained 86d5be1. This acknowledgement is a separate documentation-only develop follow-up. Stop before UC7.
## Phase 10 UC7 receive and stock — in progress
- [x] Owner authorized the next phase on 2026-10-19; inspect clean develop at ab0c224 and read repository rules.
- [x] Review UC7 guide, original ER, immutable UC6 orders, UC5 stock-ledger guards and request-scoped private upload infrastructure.
- [x] Confirm receipt/claim/payment roles, eligible orders and excess quantities, damaged-item settlement, payment methods/timing and evidence requirements.
- [x] Exact receipt/claim/payment domain and service tests; retain base-unit quantities, immutable order pricing, THB/no VAT and six-place weighted-average costing.
- [x] Add migration/repositories for audited partial receipts, good-stock movements, separate defects and approved supplier settlement rules.
- [x] Transactional order/material locking, idempotency, optimistic versions and receipt-aware cancellation protection; preserve independent UC5 plans.
- [x] Authorized receiving/history/claim/payment endpoints and order-scoped private evidence; Worker-safe inventory views, no Supplier account.
- [ ] Real PostgreSQL races/rollback, stock/cost reconciliation, private S3, migration and earlier-phase/Docker regressions.
- [ ] Update API/verification docs, commit focused buildable groups, push develop only and verify remote tip.
Decision: owner confirmed Admin/Worker receiving and claims, Admin-only supplier prices/credit/payment/evidence, accepted partial deliveries/no excess, replacement-only damage, full good receipt before payment, explicit per-order credit days counted from full receipt and one full cash/bank settlement. Bank evidence required; cash photo/proof optional. Implemented domain, migration 000010, transaction repositories, private order evidence and API groups; no UC5 auto-reservation or UC8/UC9. Build/vet/unit/lint pass. Real PostgreSQL/S3/Docker verification is not yet run: the availability probe stopped before resources were created, and starting Docker exposed the known stale IPC socket crash. Recovery approval was requested before renaming the specific socket-only folders; no data/settings reset. Original Word/sequence source availability is unchanged. See docs/UC7_PLAN.md, docs/UC7.md and docs/UC7_VERIFICATION.md. Do not mark this phase complete until actual integration succeeds. Stop before UC8.
## Phase 11 UC8 execute job — in progress
- [x] Owner asked to begin the next phase on 2026-10-19; inspect clean develop at 070265d.
- [x] Read repository rules, UC8 guide, job schema, live plan/reservation API, stock-lock hierarchy and pending UC7 verification status.
- [x] Confirm production roles/scope, start prerequisites, incremental usage/extra-material rules, completion/photo requirements and actual time/labor-cost entry.
- [x] Domain/service tests and versioned job lifecycle commands; no inferred assignments, labor rates or automatic plan consumption.
- [x] Audited usage/cost snapshots, stock-issue ledger and reservation reconciliation migration/repository transactions.
- [x] Authorized job/start/usage/time/photo/completion APIs with Worker financial redaction and private job-scoped files.
- [ ] Execute real ready/start, issue/costing, competing-job, duplicate-command, completion race, leftover/photo/rollback checks; prepared in the dedicated integration suite, not yet run.
- [ ] Resolve the existing UC7 verification gate, then run real PostgreSQL/S3/migrations and full earlier-phase/UC8 Docker regression; do not count skipped tests as success.
- [ ] Update API/verification docs, commit focused buildable groups, push develop only and verify remote; stop before UC9.
Decision: owner confirmed all five production rules. Worker-only operations for any active Worker, Admin monitoring/labor cost, fully reserved nonempty start, explicit own-reservation-first/free-stock actual use, unused release, optional photos and manual minutes without assumed rate. Implemented domain policies, migration 000011, audited usage/reservation/ledger transactions, APIs, Admin-only cost and private job photos in buildable commit groups. Build/vet/unit/lint pass. The prepared real verifier stopped at its bounded Docker probe before creating any test resources; integration tests have not run. UC7 remains in progress with Docker recovery approval pending; production confirmations are not approval for OS repair. No existing database migrated or UC9 work. See UC8.md and UC8_VERIFICATION.md. Phase 11 remains in progress and is not deployment-ready; publish as an explicitly incomplete develop checkpoint, not a completed phase.
## Phase 12 UC9 deliver and settle — todo
- [ ] Self/third-party delivery, invoices, post-completion payments and returns.

Every future phase: follow the mandatory AGENTS.md commit/push policy and docs/GIT_WORKFLOW.md. Commit focused buildable groups as work progresses, with clear purpose and relevant tests; never one giant phase commit. Update this plan, run full checks, review status/log and push each completed phase to develop first, verifying the remote tip. Main requires a new explicit owner instruction. Record blockers instead of silently leaving a finished phase unpushed. Phase 10 / UC7 is implemented but not integration-verified; Phase 11 / UC8 is now authorized with production-policy confirmation pending. UC9 remains unapproved. Historical push-pending notes in Phases 0-3 were resolved before Phase 4. Company decision authority is confirmed for any current active company member.
