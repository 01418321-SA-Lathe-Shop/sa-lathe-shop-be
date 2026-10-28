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

Implementation and verification commands will be added when S1-S3 are complete.
