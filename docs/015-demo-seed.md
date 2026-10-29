# Demo accounts on the real backend

Update 2026-10-29: representative workflow/private-document fixtures are now
implemented as a separate explicitly enabled command. See
[017-demo-workflows.md](017-demo-workflows.md). Identity seeding remains
account-only; clean migration13 or14 is supported. Workflow seeding needs14.
Earlier S0-S3 verification records below describe the account-only scope at
that time, not a prohibition on the later owner-authorized workflow command.

## Password-free shortcut (2026-10-29)

After applying migrations through 000013 and explicitly seeding the dedicated
demo DB, set `LOCAL_TEST_ACCOUNT_ENABLE=true` in the API process environment
(or local Docker Compose `.env`) and restart/recreate the API. Default is false;
only exact lowercase true enables it. Invalid values fail startup. The API
requires DB name `sa_lathe_shop_demo` or `sa_lathe_shop_seed_test`, also checked
against the actual connected database inside the login transaction. Do not
rename an existing business DB to circumvent this guard.

`POST /auth/local-test/login` accepts JSON `{"username":"pim.admin"}` with
Content-Type application/json and no password. The same works for the other
five usernames below. Result is the normal `access_token`, `token_type`,
`expires_at`, `account` response. Use Authorization Bearer for protected APIs,
GET /auth/me and POST /auth/logout as usual. No role, UUID or password overrides
are accepted. Backend role checks and company scope are unchanged.

Only an active account with matching registered UUID, username, role, safe
identity snapshot and completed matching seed manifest is eligible; matching
names alone, unrelated accounts, inactive/drifted accounts and retired datasets
are refused with 401. Missing migrations/storage failures fail closed. There is
no auto-seed, password reset, fake session or business-rule bypass. Successful
shortcut logins create real sessions, so subsequent destructive cleanup is
refused; use explicit retirement to deactivate a used demo dataset.

The endpoint is NOT registered when false/unset (404); a hidden frontend button
is not the security boundary. It uses the existing ten-attempts/minute/IP limiter
and no-store responses. A startup warning records enablement, never tokens.

IMPORTANT: anyone who can reach an enabled API can sign in as demo Admin and
exercise its real permissions. Keep this API private/local to trusted testers;
CORS and the variable are not public-access protection. Never enable against
production. Turning the variable off and restarting prevents NEW shortcut
logins; already issued sessions still last until normal logout/revocation/TTL,
and seeded accounts still support ordinary password login. Use seed retirement
to deactivate all six accounts and revoke their sessions when testing ends.

Seed creation still requires private strong passwords; the shortcut means
testers need not know or type them. No frontend change was made here. See
016-frontend-demo-handoff.md for the client contract.

## Approved scope

Account-only S0-S3, approved 2026-10-28. This is real Go authentication and
PostgreSQL with fictional accounts, not an HTTP mock. No workflow records or
objects are seeded by the identity command. S4-S5 workflow/private-object
fixtures were separately authorized on 2026-10-29; see 017-demo-workflows.md.

| Username | Role | Profile |
| --- | --- | --- |
| pim.admin | admin | Pim Rattana, initial Admin |
| anan.staff | worker | Anan Srisuk, ST-001, Lathe operator |
| kittipong.staff | worker | Kittipong Meesuk, ST-002, CNC programmer |
| nida.company | customer | Nida Wongsuwan, company owner |
| ploy.company | customer | Ploy Charoen, same company, non-owner |
| somchai | customer | Somchai Jaidee, individual owner |

Company: Siam Precision Parts Co., Ltd. Frontend legacy.staff is deliberately
omitted: new Workers require email and phone. Never bypass the current contacts
trigger or insert a pre-migration account to recreate a legacy fixture.

## Safety contract

The proposed command requires a dedicated SEED_DATABASE_URL, explicit opt-in,
local/test environment, exact database-name confirmation and loopback host.
It must verify the connected database and clean migration version. The first
run requires an empty application dataset; unrelated accounts/Admins or business
records cause refusal. It never adopts accounts by username/email or resets
existing seeded passwords. No normal API/worker startup calls the seed.

Existing auth/customer services and Argon2id create identities. Ownership and
all accounts/memberships commit together; no sessions or access tokens are
seeded. Passwords come from private process environment, never CLI arguments,
fixtures, logs or committed files. Reruns validate registered identities and
skip them, preserving hashes and tester progress.

Use additive provenance metadata, not fields in business tables. Cleanup is
limited to exact seed-owned IDs and must refuse unowned dependencies or
immutable audit history. Full workflow deletion is not supported by the current
append-only schema. No broad DELETE, TRUNCATE, trigger disabling or DB reset.

## Frontend

Use the normal /login form and NEXT_PUBLIC_API_BASE_URL with the real API.
Existing mock shortcut buttons remain explicitly browser-only; no live-to-mock
fallback. Session Bearer tokens remain memory-only. Share passwords privately,
never through NEXT_PUBLIC variables or frontend source.

## Running the account seed

Create a dedicated empty local PostgreSQL database named `sa_lathe_shop_demo`
with private credentials. Apply migrations through **000013** using the existing
migration tool, explicitly targeting that new DB. The seed command never creates
a database or applies migrations. Normal API/worker binaries do not need the
provenance tables; only the seed command requires clean version 13 exactly.

Supply these **process-only** variables, without printing the connection string:

| Variable | Required value |
| --- | --- |
| SEED_ENABLED | true |
| SEED_ENV | local (or test for disposable verification) |
| SEED_DATABASE_URL | Private PostgreSQL URL for the dedicated loopback DB |
| SEED_CONFIRM_DATABASE | sa_lathe_shop_demo |
| SEED_PIM_PASSWORD | Private Pim password |
| SEED_ANAN_PASSWORD | Private Anan password |
| SEED_KITTIPONG_PASSWORD | Private Kittipong password |
| SEED_NIDA_PASSWORD | Private Nida password |
| SEED_PLOY_PASSWORD | Private Ploy password |
| SEED_SOMCHAI_PASSWORD | Private Somchai password |

The helper securely prompts for missing passwords only on first application.
It does not load .env, and clears values it prompted for afterward. Privately
supplied process variables remain the operator's responsibility to clear.
Do not put actual passwords into .env.example, source, shell arguments/history,
frontend public configuration or the handoff. Do not use the browser mock password.

```powershell
./scripts/seed-demo.ps1 plan
./scripts/seed-demo.ps1 apply
./scripts/seed-demo.ps1 status
# Equivalent non-interactive command, using private process credentials:
go run ./cmd/seed-demo apply --profile accounts
```

Task aliases are `./tasks.ps1 seed-plan`, `seed-apply`, `seed-status` and
`seed-verify`. Unlike normal tasks, these intentionally skip .env loading.

## Real API verification and cleanup

Start the API using normal private configuration pointed at the *same* demo
database. Set `SEED_API_BASE_URL` to its loopback origin, such as
`http://127.0.0.1:8080`, then run:

```powershell
./scripts/seed-demo.ps1 verify
# Or, with six existing passwords already in private process environment:
go run ./cmd/seed-demo verify --api-base "<loopback-api-origin>"

# No deletion unless --execute is explicitly supplied:
go run ./cmd/seed-demo cleanup --dry-run
go run ./cmd/seed-demo cleanup --execute
# Used datasets can instead be explicitly disabled, retaining all data:
go run ./cmd/seed-demo retire --execute
```

Verify checks real logins, /auth/me, role-gated reads, customer profile boundaries,
logout and revocation. API identity UUIDs must match the seed target; matching
usernames on another database are not enough. It never creates business rows.
Tokens/response bodies are withheld. A failed run may leave a session until
expiry if even best-effort logout cannot reach the server. Observe the existing
ten-logins-per-minute IP limiter; wait before repeating, do not disable it.

Cleanup simulates exact registered deletions inside a savepoint, rolls back by
default, and checks FKs before reporting eligibility. Even revoked tester sessions
prevent deletion because account deletion would otherwise cascade them. Other
dependencies/identity drift also cause refusal, with the whole operation rolled
back. No trigger or audit guard is disabled. Retirement explicitly deactivates
only the six seed-owned accounts and revokes their sessions, preserving hashes,
memberships, business rows and provenance; rerun cannot reactivate them.
Stop the dedicated API before apply/cleanup/retire. Normal API verification must
run with it started. Retirement is not a stock/payment/workflow reset.

Plan/status validate the connected target and report only public manifest/status
metadata. Apply commits six accounts, two customer profiles, three memberships,
one consumed invitation and twelve ownership bindings atomically. It generates
no sessions, files, jobs, stock or payments. Reruns skip the existing dataset;
password variables are not read or used to reset it. Identity drift is an error.

First initialization refuses *any* existing application rows, including an
unrelated first Admin. Reruns permit tester-created records without adopting
them. Environment mismatch, changed manifest, missing ownership records and
wrong/dirty migration state are errors. Do not edit provenance to resolve them.

The local/test guard accepts only loopback URL hosts and the exact demo or
disposable test database names. Host/service/search-path URL overrides are
rejected; the actual connected database is checked again. These are accidental
misuse guards, not protection against a database administrator or a deliberately
mislabelled production copy. Remote Railway seeding is not supported/authorized.

Mutation commands run only while the dedicated demo API is stopped. The command locks application
tables against writes during short identity/provenance transactions and uses a
five-second lock timeout. Ordinary API instances never use this transaction seam.

## Verification record — 2026-10-28

S1: fresh PostgreSQL 17 migrations 1-13 and empty provenance down/up passed;
service-created Admin/Worker/company/invited member rolled back with the outer
transaction, leaving no identity/session rows. S2 adds real creation, injected
late validation/capture failures, unrelated identity refusal, concurrent exact
reruns, hash preservation and identity/connected-database drift checks.
The verifier creates its own labelled container, never reads .env, and removes
only that container and its fresh anonymous volume. No existing DB is seeded.

S3 passed non-skipped real PostgreSQL identity/seed suites under race detection,
including unowned membership and unknown cascading-FK refusal, atomic dry-run/
failure rollback, populated downgrade refusal, retirement and session revocation.
Real HTTP fixtures verify company sharing, individual/cross-company isolation,
Worker/Admin boundaries and request/outbox consistency. A second fresh DB runs
the actual CLI plan/apply/rerun plus an independently started API process: all
six logins, exact UUID/role checks, company ownership, logout, revocation and
CORS pass. The first CORS assertion expected 204; existing chi/cors returns
valid 200. Request logs confirmed it and only the verifier was corrected.

Full existing UC1-UC9 regression `sa-lathe-uc9-2eacc129` passed all ten required
real suites, private S3, API/worker Docker builds/HTTP flows and graceful stop,
with migrations through 13. Build, vet and default race tests pass. The pinned
source vulnerability scan reports zero reachable vulnerabilities; twenty
unused-module advisories are informational, not a claim of no advisories.
CI includes the seed verifier, but hosted GitHub results have not been observed.

No frontend browser verification, persistent demo database, hosted API, actual
workflow seed, S3 provisioning for demo attachments, existing DB migration or
real LINE occurred. Verification containers/volumes were removed; ignored local
build/log artifacts remain in bin. Operator setup is still required before
frontend testers can use seeded accounts. Frontend handoff: [016-frontend-demo-handoff.md](016-frontend-demo-handoff.md).
