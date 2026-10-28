# Demo accounts on the real backend

## Approved scope

Account-only S0-S3, approved 2026-10-28. This is real Go authentication and
PostgreSQL with fictional accounts, not an HTTP mock. No workflow records or
objects are seeded. S4-S5 workflow/private-object fixtures require later approval.

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

Run only while the dedicated demo API is stopped. The command locks application
tables against writes during short identity/provenance transactions and uses a
five-second lock timeout. Ordinary API instances never use this transaction seam.

## Verification so far

S1: fresh PostgreSQL 17 migrations 1-13 and empty provenance down/up passed;
service-created Admin/Worker/company/invited member rolled back with the outer
transaction, leaving no identity/session rows. S2 adds real creation, injected
late validation/capture failures, unrelated identity refusal, concurrent exact
reruns, hash preservation and identity/connected-database drift checks.
The verifier creates its own labelled container, never reads .env, and removes
only that container and its fresh anonymous volume. No existing DB is seeded.
