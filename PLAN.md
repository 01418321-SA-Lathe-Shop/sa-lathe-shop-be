# Implementation plan

## Self-service password change (2026-11-03) — done

- [x] `POST /auth/password` for every active role: verify current password (Argon2id), account password rule plus differs-from-current and no-username, rate limited per IP.
- [x] Replace hash under an account row lock; revoke every other session, keep the caller's. No migration.
- [x] Unit/HTTP tests; docs/000-auth.md and openapi.yaml updated; committed `7cf805a` to develop.

Verification: go build ./..., go vet (auth, postgres), full default `go test ./...` pass. Real PostgreSQL check through the
local Docker API against the demo DB: wrong current password 400, success 204, other session 401, old password rejected,
new password accepted. Local API container afterwards rebuilt from 77d3568 with its previous demo settings.

## Password-free local demo login (2026-10-29) — done

- [x] Inspect auth/session composition, seed provenance and current configuration.
- [x] Add explicitly gated password-free login with normal sessions and role checks.
- [x] Require dedicated demo/test DB and exact active seed identity/manifest.
- [x] Add default-off configuration, Docker wiring and regression tests.
- [x] Complete disposable real PostgreSQL/API verification and full race/build/vet.
- [x] Update API/frontend handoff and commit logical buildable groups to develop.

Verification: go build ./..., go vet ./... and uncached full race suite pass.
The disposable seed verifier ran both required real PostgreSQL suites (not
skipped), including password-free sessions for all six identities, rejection of
unowned matching names, inactive/role-drifted accounts, bad manifest and retired
datasets. Its independent API process verified normal password login still
works, six password-free logins with /auth/me, Admin-only gate and logout/401.
Restart with flag=false proved endpoint 404. It emitted
LOCAL_TEST_ACCOUNT_VERIFICATION_PASSED and DEMO_SEED_VERIFICATION_PASSED; only
the fresh labelled verification resources were removed. No browser/frontend,
shared DB, deployment or full UC/S3 regression is claimed for this follow-up.
Publication uses separate service/repository/tests, configuration/runtime
verification, and documentation commits; verify origin/develop at final handoff.

No frontend changes, persistent/shared seed, existing DB migration, deployment
or workflow fixtures. Flag is a process startup setting. Turning it off removes
the shortcut after restart, not existing sessions or ordinary password login.

## Approved demo account seeding (2026-10-28)

Owner approved the recommended S0-S3 account-only scope. This task precedes
remaining remediation phases; it does not authorize optional workflow fixtures.

### S0 scope and safety contract — done
- [x] Inspect both repositories, current schema/auth/domain services and mock data.
- [x] Record six compatible accounts; legacy.staff is excluded, not fabricated.
- [x] Separate account seeding from workflows, storage and real API verification.
- [x] Define local/test guards, ownership, no password resets and cleanup refusal.

### S1 ownership and identity transaction support — done
- [x] Add provenance migration without changing business schema or audit guards.
- [x] Bind existing identity repositories to a transaction using savepoints.
- [x] Verify migration and existing auth compatibility; publish logical groups.

Verification: build and focused auth/repository tests passed. Disposable
PostgreSQL 17 migration 1-13, empty 13 down/up and real nested identity rollback
passed under race detection. The test creates Admin/Worker/company/invited
member through existing services, then proves the outer rollback leaves all
identity tables and sessions empty. Only the uniquely labelled test container
and its fresh anonymous volume were removed. No existing DB was changed.

### S2 guarded account command — done
- [x] Add explicit plan/apply/status and private process-only credential inputs.
- [x] Create six accounts/memberships and provenance in one transaction.
- [x] Verify reruns/conflicts/atomic rollback without creating sessions.
- [x] Update documentation and publish logical groups.

Verification: build/vet and seed/auth unit tests pass. Real PostgreSQL tests
cover late invalid password and injected provenance failures (no partial rows),
unowned matching username refusal, six valid Argon2id identities, concurrent
reruns without reading/resetting credentials, drift and actual database guard.
The initial combined test run exposed a test-order assumption, not an auth bug:
the fresh-DB rollback test now executes first in a separate invocation, with
explicit non-skip gates. No credentials or hashes are printed by the command.
No persistent demo DB or hosted service was provisioned; verification is disposable.

### S3 integration verification and safe cleanup — done
- [x] Verify real HTTP login, role gates and company isolation on disposable PostgreSQL.
- [x] Implement dry-run cleanup, dependency refusal and explicit retirement.
- [x] Document commands, results and frontend handoff; publish verified groups.

Verification: the dedicated verifier passes both non-skipped real PostgreSQL
suites under race detection, actual CLI plan/apply/rerun and a separate API
process with six role logins, membership matching, logout/revocation and CORS.
Fresh test-only requests prove company sharing/cross-company isolation, role
gates and atomic outbox writes; these are not persistent workflow fixtures.
Cleanup proves dry-run rollback, failure rollback, unowned membership/FK and
unknown cascade refusal, plus retirement without password changes or deleting
business data. Populated provenance refuses downgrade. Full UC1-UC9 regression
sa-lathe-uc9-2eacc129 passed all ten real suites, Docker HTTP flow, private S3,
migrations including 13 and graceful stop. Disposable resources were removed.
Initial standalone verifier assumed preflight 204; actual chi/cors returns
valid 200, confirmed in request logs. Only the assertion was corrected.
Build/vet/default race and source vulnerability checks pass (zero reachable;
20 unused-module advisories remain informational). CI now includes the seed
verifier; hosted GitHub execution has not been observed. No frontend/browser
run, shared/hosted DB provisioning, deployment, existing DB migration, real
LINE delivery or S4-S5 fixtures occurred. See docs/015-demo-seed.md and
docs/016-frontend-demo-handoff.md. Stop after this handoff.

### S4-S5 workflow fixtures/private objects — done (authorized 2026-10-29)
- [x] Owner requested related test data for the seeded demo identities.
- [x] Add guarded command and DB-backed workflow checkpoints/ownership roots.
- [x] Build requests/quotes/PO/deposit/jobs/stock/orders through real HTTP APIs.
- [x] Provision clearly marked private test documents, never fake ready URLs.
- [x] Verify disposable real workflow, rerun/failure/isolation/financial/stock behavior.
- [x] Seed local demo only, preserve existing tester data and document logical develop groups.

Account identity provenance remains separate. Completed reruns never restore
fixture states over tester actions. Partial non-idempotent uncertain writes
must stop for reconciliation rather than create duplicates. No destructive
workflow reset/cleanup is offered; immutable business audit remains intact.

Verification: build/vet/full uncached race suite pass. Final disposable project
sa-lathe-workflows-99e7e3e9 passed 141 real API steps, 20 private document
uploads/downloads/anonymous denial, stock/WAC and payment/closure checks,
company isolation and role redaction. Completed rerun preserved an additional
tester stock entry without changing account/session/request/job/step counts.
Independent post-restart readback passed and fresh resources were removed.
Earlier harness failures were corrected: a dynamic API port changes on restart;
received supplier credit has existing payment_status=pending, not unpaid.
No business contract was changed for either assertion. Unit tests cover exact
idempotent replay, checkpoint ambiguity refusal and foreign storage origins.

The separate running local sa_lathe_shop_demo was explicitly migrated13->14
(metadata tables only), seeded and verified through localhost:8080. All141
steps completed; repeat apply reports reuse. Original sa_lathe_shop, frontend
files and real LINE remain untouched; existing Docker services stay running.
No browser UI or hosted CI pass is claimed. See docs/017-demo-workflows.md.
The existing account verifier also passed on schema14: real identity rollback,
seed/cleanup/retirement tests, six normal and password-free API logins, role
gates, revocation and default-off endpoint checks. Live demo readback confirmed
11 requests, 6 jobs, 5 term_evidence POs, 3 deposits and clean schema14.
Final publication is grouped on develop; remote tip is verified at handoff.

No existing DB is targeted. Never run automatically, TRUNCATE, reset a DB,
disable triggers or remove immutable audits. Passwords/tokens/hashes stay out
of output and provenance. Seeded accounts use normal live frontend login.

## Approved remediation programme (2026-10-27)

Owner approved phases 14-21. Preserve UC1-UC9 business rules, current API
responses and exact retries; add opt-in compact responses rather than breaking
existing clients. Remain on develop, with independently buildable logical
commits and a verified push at each phase completion. No main push, deployment,
existing-database migration or real LINE delivery is authorized.

### Phase 14 security patches and regression safeguards — done
- [x] Align module, development, CI and container builds on patched Go 1.27.3.
- [x] Upgrade x/text to v0.41.0; its minimum requirement also selects x/sync v0.22.0.
- [x] Pin verified builder image digest and add pinned source/binary vulnerability checks.
- [x] Characterize current response shapes, financial redaction and exact retries.
- [x] Build/vet/unit/race tests, real DB/S3 workflows and container/binary checks pass.
- [x] Separate patch, regression-test and security-CI commits created.
- [x] Push final phase checkpoint to develop and independently verify the remote tip.

Verification: Go 1.27.3 build/vet and full race/coverage suite passed. Source,
Windows API/worker and actual Linux container binary scans reported zero
reachable vulnerabilities (unused-module advisories remain informational, not
a claim of no vulnerabilities anywhere). Both Docker images built. Isolated
sa-lathe-uc9-b06047c3 passed real UC1-UC9 PostgreSQL/private-S3 tests, full JSON
retry equality, migrations/down-up, Docker HTTP flow and graceful shutdown;
only its fresh containers/network/volumes were cleaned up. No existing DB,
real LINE recipient, main branch or deployment was changed. See
docs/013-remediation-verification.md. Published dc035d8 through b8ab9b5 to
origin/develop; independently observed remote b8ab9b56dd52238a1134f7f264a0fc99b29cf55b.

### Phase 15 measurements and real integration CI — done
- [x] Correct per-request benchmark timing, body consumption and bounded concurrency; test the harness.
- [x] Add unconditional disposable PostgreSQL/private-S3 CI with explicit non-skip verification.
- [x] Baseline 100 synthetic accounts, 1,000 jobs/materials, 100 suppliers and long histories.
- [x] Measure concurrency 1/10/25, 100-user mix and 1/10/100-line service writes; publish evidence.
- [x] Build/vet/unit/harness checks pass from independently exported b71d1cd snapshot.
- [x] Push completed phase groups to develop and verify remote tip: 5a1f83a947cefd48f8ae0ad7eaeb4a94dd3b20f1.

Verification: fresh sa-lathe-perf-aa349ddd passed in 460.57 s, zero request
failures, report retained under ignored bin/. Measured fulfillment page size 25:
152 SQL statements/request, median p95 460.21 ms at concurrency 10 and 1021.91 ms
at 25. The 1000-entry job has 53,569,820 stored snapshot bytes. Search SQL is
below 0.2 ms here; no speculative index was added. See docs/012-performance.md.
Strict integration rerun sa-lathe-uc9-6d466554 passed all ten real suites under
race/coverage plus Docker HTTP/inbox/cleanup. An earlier isolated HTTP attempt
failed without a location; safe line/status diagnostics were added, the failure
did not reproduce, and no unconfirmed root cause is claimed. Initial baseline
report parsing rejected fragmented test2json output; parser regression tests and
the fresh successful rerun resolved that tooling issue. All disposable resources
were cleaned up; no existing DB or real LINE changed. Linux GitHub execution
awaits the develop push; local equivalent checks ran on Windows.

### Phase 16 errors, validation and shared HTTP support — done
- [x] Distinguish missing rows from infrastructure failures; sanitized request-correlated logging.
- [x] Share strict JSON/pagination primitives, preserving 8/64-KiB limits and public errors.
- [x] Reject NUL/invalid UTF-8 in request, fulfillment and profile text before persistence.
- [x] Unit/failure-injection tests, build/vet/full race suite and exact staged-tree checks pass.
- [x] Push phase checkpoint to develop and verify remote 012a4d8a8972b3c7c500ab95eb35bf1736db0283.

Groups: b20c9e8 (text), 1de7395 (safe diagnostics), b82c6ab (shared HTTP),
5e6ce0c (authorization error preservation). Every staged tree was exported and
independently built/tested before committing. Full build/vet/race checks pass.
Timeout/connection/missing actor/membership fault rows are covered without a DB.
Full real DB/S3/container regression runs again with Phase 17 query changes.
Transaction-step extraction accompanies that production rewrite to avoid
rewriting the same command twice, as recorded before Phase 16.

### Phase 17 database read/write efficiency — done
- [x] Extract focused command-state/material-read/batch steps; retain repository-only SQL.
- [x] Bulk fulfillment pages; real tests enforce one repository statement and list/detail equality for scoped roles.
- [x] Focus production validation reads and build legacy final state once with safe redaction.
- [x] Bulk locked material/reservation reads and batched usage/ledger writes; preserve all triggers.
- [x] Paired baseline, real Docker workflow, race/harness/vet and source/binary vulnerability checks pass.
- [x] Exported 5d1f9d3/4af3d05/b589b21/d0308c1/76e7e56 snapshots independently build.
- [x] Push phase checkpoint; independently verified origin/develop at 373708880dc2ae14154a7f826666b1cd9dced394.

Code groups: 5d1f9d3 (bulk fulfillment), 4af3d05 (focused command state),
b589b21 (batched usage and rollback/trace tests). Default race tests pass and
all ten real DB/S3 suites passed in sa-lathe-uc9-6280d46e. Its Docker flow then
failed at supplier settlement fixture line 244 (HTTP 400). The fixture used an
independent host clock while the DB enforces full_received_at <= paid_at <= now;
it now uses the returned server full_received_at boundary, with no production
guard change. Final sa-lathe-uc9-c335c3fb passed all ten suites and full Docker
flow, with scoped cleanup. Paired sa-lathe-perf-60ea3b78 passed with identical
bodies and zero failures: legacy/optimized median p95 417.14/17.22 ms at
concurrency 1, 2782.27/64.52 ms at 10 and 1870.54/201.69 ms at 25. Fulfillment
HTTP uses 2 statements at page sizes 25/50 versus 152 previously at 25;
100-material usage uses 215 versus 613. Earlier uncontrolled results are retained,
not presented as clean comparisons. Captured host counters show 77% CPU and
547 MiB free RAM, so no normal-load or production SLA is claimed. Final source/
API/worker scans find zero reachable advisories, with unused-module warnings
retained as informational. No index or pool-size change
is justified by the original small search-query timings. Legacy snapshot growth
remains for Phase 18's explicitly opt-in receipt mode.

### Phase 18 compact responses and bounded history — todo
- [ ] Add opt-in summary reads and role-scoped paginated production/fulfillment histories.
- [ ] Add response=receipt commands with immutable compact retry records and result-kind migration.
- [ ] Preserve legacy fingerprints/snapshots; different retry mode conflicts; compatibility-first rollout.
- [ ] Verify bounded receipt sizes and legacy regression; document retained legacy growth cost.

### Phase 19 login fairness and trusted proxies — todo
- [ ] Separate coarse IP guard from username/IP failed-login guard; preserve Argon2/session policy.
- [ ] Separate registration/join/invitation limits, bounded counters and successful-login reset.
- [ ] Opt-in trusted proxy CIDRs; ignore untrusted forwarding and test spoofing/shared-NAT access.

### Phase 20 upload budget and independent outbox channels — todo
- [ ] Share cancellation-aware inspection/freezing pipeline and two-slot process-wide budget.
- [ ] Preserve private file lifecycle/permissions and test all cleanup/failure cases.
- [ ] Independent inbox/LINE claim loops, set-based inbox inserts and safe lag/failure logs.
- [ ] Verify leases, retries, stalled LINE isolation and graceful shutdown.

### Phase 21 contracts, documentation and closure — todo
- [ ] Concrete role-specific OpenAPI schemas and automated route/response contract coverage.
- [ ] Current setup/auth/security/performance/Railway docs and migration/configuration guidance.
- [ ] Full UC1-UC9 regression and before/after benchmark/SQL plans; per-finding closure report.

Programme defaults: no new runtime library, Redis, broker, ORM or framework;
no hosting-independent SLA. Use fresh synthetic environments only. Frontend
adoption, hosting, production deployment and malware-scanning infrastructure
remain outside scope. Full legacy snapshots are retained as a compatibility
cost until clients adopt opt-in receipt mode. Every phase must record its tests,
limitations, decisions, commit groups and publication result below.

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
## Phase 10 UC7 receive and stock — done
- [x] Owner authorized the next phase on 2026-10-19; inspect clean develop at ab0c224 and read repository rules.
- [x] Review UC7 guide, original ER, immutable UC6 orders, UC5 stock-ledger guards and request-scoped private upload infrastructure.
- [x] Confirm receipt/claim/payment roles, eligible orders and excess quantities, damaged-item settlement, payment methods/timing and evidence requirements.
- [x] Exact receipt/claim/payment domain and service tests; retain base-unit quantities, immutable order pricing, THB/no VAT and six-place weighted-average costing.
- [x] Add migration/repositories for audited partial receipts, good-stock movements, separate defects and approved supplier settlement rules.
- [x] Transactional order/material locking, idempotency, optimistic versions and receipt-aware cancellation protection; preserve independent UC5 plans.
- [x] Authorized receiving/history/claim/payment endpoints and order-scoped private evidence; Worker-safe inventory views, no Supplier account.
- [x] Real PostgreSQL races/rollback, stock/cost reconciliation, private S3, migration and earlier-phase/Docker regressions.
- [x] Update API/verification docs and commit focused buildable implementation/fix groups.
- [x] Push verification follow-up to develop only and verify remote tip: 1d31580 remotely confirmed; main unchanged at 86d5be1.
Decision: owner confirmed Admin/Worker receiving and claims, Admin-only supplier prices/credit/payment/evidence, accepted partial deliveries/no excess, replacement-only damage, full good receipt before payment, explicit per-order credit days counted from full receipt and one full cash/bank settlement. Bank evidence required; cash photo/proof optional. Domain, migration 000010, transaction repositories, private order evidence and API groups are implemented; no UC5 auto-reservation. On 2026-10-19 Docker responded to the requested retry without repair. The first database attempt found a PL/pgSQL CASE syntax error; focused fix ee0c33c parenthesizes that expression. Fresh combined verifier sa-lathe-uc8-a664942c passed all real UC7/UC8 PostgreSQL/S3 tests, migrations and Docker HTTP flows, emitted PHASE11_VERIFICATION_PASSED/WORKER_CONTACTS_VERIFICATION_PASSED and cleaned up its resources. Build/vet/lint pass. No existing database/runtime data changes or real LINE messages. Original Word/sequence availability is unchanged. See docs/UC7_VERIFICATION.md. UC8 was subsequently authorized and verified; UC9 remains unapproved. Only publication of this verification follow-up remains.
## Phase 11 UC8 execute job — done
- [x] Owner asked to begin the next phase on 2026-10-19; inspect clean develop at 070265d.
- [x] Read repository rules, UC8 guide, job schema, live plan/reservation API, stock-lock hierarchy and pending UC7 verification status.
- [x] Confirm production roles/scope, start prerequisites, incremental usage/extra-material rules, completion/photo requirements and actual time/labor-cost entry.
- [x] Domain/service tests and versioned job lifecycle commands; no inferred assignments, labor rates or automatic plan consumption.
- [x] Audited usage/cost snapshots, stock-issue ledger and reservation reconciliation migration/repository transactions.
- [x] Authorized job/start/usage/time/photo/completion APIs with Worker financial redaction and private job-scoped files.
- [x] Execute real ready/start, issue/costing, competing-job, duplicate-command, completion race, leftover/photo/rollback checks.
- [x] Resolve the UC7 verification gate and pass real PostgreSQL/S3/migrations plus full earlier-phase/UC8 Docker regression; integration environment explicitly configured, not skipped tests.
- [x] Update API/verification docs and commit focused buildable implementation/fix groups.
- [x] Push verification follow-up to develop only and verify remote: 1d31580 remotely confirmed; main unchanged at 86d5be1. Stop before UC9.
Decision: owner confirmed all five production rules. Worker-only operations for any active Worker, Admin monitoring/labor cost, fully reserved nonempty start, explicit own-reservation-first/free-stock actual use, unused release, optional photos and manual minutes without assumed rate. Domain policies, migration 000011, audited usage/reservation/ledger transactions, APIs, Admin-only cost and private photos are implemented. On 2026-10-19 full verifier sa-lathe-uc8-a664942c exited 0 after uncached real PostgreSQL/S3 regressions, migrations 1-11/empty 11+10 down/up, legacy contacts, Docker builds, readiness, customer-to-receiving-to-production API flow, worker inbox dispatch and graceful stop. Both success markers and scoped cleanup were observed. Build/vet/lint pass. No runtime repair, existing database migration, production deployment or UC9 work. See docs/UC8_VERIFICATION.md. Only publication of this verification follow-up remains.
## Phase 12 UC9 deliver and settle — done
- [x] Owner requested next phase on 2026-10-19; inspect clean develop at 1f1eb08 and repository rules.
- [x] Review guide/requirements, full original ER, job/deposit snapshots and production lifecycle guards.
- [x] Record proposed roles, invoice/delivery cardinality, credit/payment evidence, customer returns, closure and notifications in docs/UC9_PLAN.md.
- [x] Owner confirmed all seven business decisions on 2026-10-20; implement per docs/UC9_PLAN.md.
- [x] Domain policies, role-safe views and exact invoice/installment/balance tests.
- [x] Audited delivery/invoice/customer payment/private evidence schema and confirmed production return extension.
- [x] Transaction repositories with membership checks, idempotency, no overpayment and atomic closure/outbox.
- [x] Authenticated delivery/inspection/invoice/payment/evidence endpoints and confirmed notifications.
- [x] Real PostgreSQL/S3 concurrency/rollback/privacy, migration and full UC1-UC9 Docker regression.
- [x] Runbook/verification records and logical buildable implementation commits ed09788 through c487fa2, pushed to develop; c487fa2 independently confirmed remotely and main unchanged at 86d5be1. Final documentation is a separate develop follow-up.
Decision: owner confirmed all seven proposals on 2026-10-20. Implemented Admin-entered terms and staff-issued one-job invoice, whole-job delivery/inspection, customer bank submissions/Admin review/cash receipts, immutable allocations, free pre-acceptance same-job rework and paid+accepted closure. Private proof remains uploader/Admin only; Worker sees invoice balances, never receipts/proof/internal costs. Migration 000012 extends production via audited cycle history, not raw status resets. No automatic stock restoration, refunds, price changes, new LINE or reminders. Both isolated UC1-UC9 runs passed: initial sa-lathe-uc9-98304677 and expanded final sa-lathe-uc9-ace1e2f1 emitted UC9_DOCKER_FLOW_PASSED, PHASE12_VERIFICATION_PASSED and WORKER_CONTACTS_VERIFICATION_PASSED, exited 0 and cleaned up all scoped resources. Build/vet/lint pass. Standard Go proxy lookup timed out; the same pinned migration tool ran from the existing local module-cache proxy, with per-process restoration and no global configuration change. No existing database/runtime data changed, production deployment or real LINE sent. Original Word/sequence availability is unchanged. See docs/UC9.md and UC9_VERIFICATION.md. No further phase, PDF rendering, post-acceptance warranty or production deployment is inferred.

Publication confirmation (2026-10-19): separate fix ee0c33c and verification record 1d31580 were pushed to origin/develop, with 1d31580 independently observed remotely. This resolves the publication-only handoff notes above. Main remained 86d5be1. This final checklist acknowledgement is a small documentation-only follow-up on develop.

Every future phase: follow the mandatory AGENTS.md commit/push policy and docs/GIT_WORKFLOW.md. Commit focused buildable groups as work progresses, with clear purpose and relevant tests; never one giant phase commit. Update this plan, run full checks, review status/log and push each completed phase to develop first, verifying the remote tip. Main requires a new explicit owner instruction. Record blockers instead of silently leaving a finished phase unpushed. UC7/UC8 are implemented, integration-verified and published to develop. UC9 business policies are confirmed and implementation is authorized. Historical push-pending notes in Phases 0-3 were resolved before Phase 4. Company decision authority is confirmed for any current active company member.

## Phase 13 API contract, security review, CI and performance baseline — done
- [x] Record owner approval for documentation, cross-UC security/permission review, CI, and repeatable local latency/query measurement only; no new business behavior or deployment.
- [x] Publish an OpenAPI contract and runnable examples that match the registered UC1-UC9 routes, authentication, role boundaries, errors, and private evidence behavior.
- [x] Review authorization, data redaction, private evidence, transaction/idempotency and deployment-sensitive controls across UC1-UC9; document evidence, severity, and any safe in-scope correction separately.
- [x] Add CI for Go formatting, build, vet, and tests without requiring production secrets, existing databases, or real LINE delivery.
- [x] Add a repeatable local HTTP load scenario plus database query measurement instructions; state prerequisites and avoid inventing SLA/hosting assumptions.
- [x] Run CI-equivalent checks and validate API contract/examples and benchmark setup; preserve existing data and never deploy.
- [x] Update this phase record with outcomes/limitations, commit logical buildable groups, push to origin/develop, and independently verify the remote tip.
Decision: owner approved Phase 13 scope on 2026-10-23. Phase 12 / UC9 is the final business feature. Performance results are local diagnostics, not a production capacity claim. Security review found no confirmed authorization defect; proxy topology, malware scanning, and outbox/retention alerting remain deployment decisions. No main push or deployment was authorized.
Verification: gofmt, `go build ./...`, `go vet ./...`, and `go test ./...` passed with DB/S3 integration environment variables cleared. OpenAPI and CI YAML parsed successfully; the benchmark script passed PowerShell syntax parsing and a 6-request loopback smoke test (100% success) against a temporary HTTP server. This smoke test is not a shop API latency result. No SQL plan or full Docker integration run was performed during Phase 13; see docs/012-performance.md.

## Staff/Admin dashboard read-only reporting — implemented (2026-10-28)

- [x] Owner approved shared queue, Job-only creation-cohort states and first-completion-per-Job heatmap.
- [x] Add documented read models, Bangkok calendar/filter validation and summary tests.
- [x] Add authenticated read-only PostgreSQL aggregate and paginated joined list routes.
- [x] Verify repeatable-read/read-only repository, 125 jobs, cancelled inclusion, first completion/rework, privacy and permissions in a fresh PostgreSQL 18.3 minimal reporting fixture; not a full UC/S3 regression run.
- [x] Domain/contract 7f21b13 and HTTP/repository b806865 published separately to origin/develop after checks. No migration, deployment or existing UC command change.

Verification: Go build/vet/full default suite passed. Dedicated fresh PostgreSQL
18.3 reporting SQL fixture passed without skipped execution and was removed.
Frontend consumer adds a responsive shared-queue dashboard and contract mock;
the browser mock/fixture pass is not deployed Go/PostgreSQL/S3 verification.
Both read routes must be present in the running API before live reports work;
no production restart, existing database mutation or deployment was performed.

## Documentation consolidation and reformatting — done
- [x] Read and analyzed all documentation in `docs/`.
- [x] Identified duplicates and overlapping content across requirements, setup/workflow, auth, and UC1–UC9 specifications, plans, and verification records.
- [x] Consolidated 38 fragmented markdown documents into 15 unified, zero-loss, numbered documents matching the `number-name.md` convention (foundational docs `000-*`, use cases `001-UC1.md` through `009-UC9.md`, and reference docs `010-*` to `012-*`), preserving `openapi.yaml`.
- [x] Updated cross-references in `README.backend.md`, `AGENTS.md`, and `docs/openapi.yaml`.
- [x] Verified `go build ./...`, `go vet ./...`, `go test ./...`, and lint checks pass cleanly.

## Source report documentation alignment — done (2026-10-29)

- [x] Read the supplied Systems Analysis and Design Markdown and compare it with current backend requirements, auth rules, UC1–UC9 contracts, migrations, and demo fixture documentation.
- [x] Add a numbered system-analysis digest and a source-to-contract traceability map without changing API behavior or database schema.
- [x] Record source-only UC10–UC12 and legacy ER/query assumptions as reconciliation notes rather than implemented features.
- [x] State the numbered filename rule and add the new docs to `README.backend.md`.
- [x] Check cross-references, numbering, whitespace, and the staged diff; verify the documentation-only commit builds with `go build ./...`.

Decision: current owner-confirmed rules, backend UC/API documents, and migrations remain authoritative. The academic report's SQL/ER are historical and not executable PostgreSQL contracts. Existing filenames and links are preserved. No schema, API, seed data, or environment was changed.


## Per-material unit conversion — Phases A and B done (2026-10-30)

Status: owner asked to start this work on 2026-10-30, ahead of remediation phases 18-21. Phases A and B are implemented in the backend and frontend (frontend develop 986ee67). Phase C is not; it needs the money-rule decision below.

Design: stock stays in each material's one immutable base unit. Conversion happens only when a quantity is entered, so balances, reservations, average cost and the movement ledger keep their meaning.

Rules:
- Conversions are per material, for example `1 sheet = 12.5 kg` for one material. No global or chained conversions, and no density or size calculations.
- `quantity × factor` must fit NUMERIC(18,6) exactly; otherwise the request is rejected, never rounded.
- The server converts. Clients send `{unit_id, quantity}` and display the result. Omitting `unit_id` means base unit, so existing callers keep working.
- A factor is immutable once used. To correct one, deactivate it and add a new one; past records keep the factor they used.
- Average cost, cost reports and the usage chart stay per base unit.

### Phase A conversion list and page — done
- [x] Add migration 000015 `unit_conversion`: `material_id`, `unit_id`, `factor > 0`, `is_active`, creator and timestamp; one active row per material and unit; never the base unit.
- [x] Add `GET /materials/{id}/conversions` for staff, and create and deactivate for Admin only.
- [x] Frontend: Unit Conversion page, supported units on material details, and read-only converted balances.

Decision: the conversion row is its own audit, not `material_catalog_audit`. That table records name and active changes with a material version bump, which would make conversion edits conflict with concurrent stock commands. A trigger keeps factors immutable and rows undeletable. Seed guards now also accept version 15.

Verification: `go build`, `go vet` and the default test suite pass. A disposable Compose project ran 000015 up, down and up, then `TestMaterialIntegration` and `TestDemoSeedIntegration`, against real PostgreSQL and S3, and was removed. `scripts/verify-*.ps1` were not run because they need PowerShell 7, which is not installed here (5.1 strips their Docker arguments), so the Docker HTTP flows and the workflow fixture guard at version 15 are unverified. No existing database was migrated.

### Phase B entry in other units for stock and production — done
- [x] Accept the optional unit on stock adjustment, job material plans and production usage.
- [x] Add migration 000016: nullable `entered_qty`, `entered_unit_name` and `factor_used` on `stock_movement`, `job_material_requirement` and `job_material_usage`, with a check that entered x factor equals the base quantity.
- [x] Fingerprint the entered quantity and unit, and convert only after the retry check, so a retry cannot convert differently.
- [x] Frontend: unit selection on the `features/inventory` and `features/production` screens.

Frontend verification: typecheck, 197 tests and a production build pass. On a disposable stack with real API, PostgreSQL and S3 at version 16, all 141 demo workflow steps applied, and the browser exercised add and reject factor, stock entry with exact preview and blocked inexact amount, plan save and reservation, usage, and Worker cost redaction. Deactivating a factor was not clicked in the browser, and phone width was not checked. Reopening a saved plan restores the typed unit when it is still active at the same factor; otherwise the line opens in the base unit (verified in the browser).

Decisions: `material_reservation` gets no entered columns because a reservation may be partial and already follows its requirement. `unit_cost` on a stock addition stays per base unit, as the cost rule requires; an Admin entering 2 sheets still types the cost per kg. Sending the base unit's id is accepted and treated as base unit. Requests without `unit_id` keep byte-identical fingerprints and response shapes.

Verification: `go build`, `go vet` and the default suite pass. Disposable Compose projects ran 000016 up, down and up, then every repository integration suite on real PostgreSQL and S3 with no skips, except `TestDashboardIntegration`, which builds its own schema and touches no changed code. A new test covers rejected rounding, missing conversions, retries after deactivation, the same key with a different unit, plan and reservation records, usage and the ledger. `TestIdentityTransactionIntegration` must run before `TestDemoSeedIntegration` on the same DB, as the verifier does. `scripts/verify-*.ps1` still need PowerShell 7, so the Docker HTTP flows and the workflow fixture guard at version 16 are unverified, and `unit_id` is not exercised through HTTP. No existing database was migrated.

### Phase C purchasing and receiving — todo, separate decision
- [ ] Decide the money rule first: `material_order_item.total_amount` is `round(qty_ordered × unit_cost, 2)`, and dividing a supplier price by a factor may not fit 6 decimals, so totals could differ from the supplier bill.
- [ ] Likely approach: store the supplier's entered quantity and price on the order line, bill from those, and convert only when stock is received.
- [ ] Apply over-delivery and damaged-goods checks in one unit; update `features/procurement` and `features/receiving`.

Each phase also needs:
- [ ] One migration per phase, with nullable new columns so old rows stay valid.
- [ ] Update `docs/005-UC5.md`, `docs/019-source-traceability.md`, `docs/openapi.yaml` and frontend `docs/024-USECASE10-12.md`, which currently say conversion is unsupported.
- [ ] Integration tests covering rejected rounding and retries; update `scripts/verify-uc5.ps1` and `verify-uc8.ps1`, plus `verify-uc6.ps1` and `verify-uc7.ps1` in Phase C.

Known gap: based on the schema, routes and docs; not every write path was read, so the Phase B file list may grow.
