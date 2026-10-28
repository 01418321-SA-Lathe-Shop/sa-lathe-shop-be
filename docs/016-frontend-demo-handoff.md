# Frontend handoff: fictional accounts, real backend

## What exists

Backend develop provides a guarded account-only seed command, migration 000013
for provenance and real HTTP verification. It reuses existing Argon2id, opaque
Bearer sessions and permissions. A later opt-in password-free endpoint is
documented below; normal login remains unchanged.
No workflows, attachments, stock, invoices or payments are seeded. Empty lists
after first login are expected, not a reason to switch to mock data.

The six usernames are pim.admin (Admin), anan.staff / kittipong.staff (Worker),
nida.company (company owner), ploy.company (same-company member) and somchai
(individual). legacy.staff is omitted because it lacks required new-Worker
contacts. Account UUIDs and passwords are not frontend constants.

## Operator prerequisite

An operator must prepare a dedicated local demo database, apply migrations
through 000013 and run the explicit account seed with privately supplied
passwords per docs/015-demo-seed.md. Start the real API against that same DB.
Disposable verification proves the implementation; it does NOT leave a shared
or hosted demo database/API available. No existing DB has been migrated/seeded,
and no Railway deployment or remote seed is authorized by this work.

For uploads, independently configure real private S3-compatible storage and
bucket CORS before testing file workflows. The account-only seed needs no S3
objects. Start the normal worker for actual outbox notifications/expiry, keeping
LINE disabled unless separately authorized. No real LINE delivery was tested.

## Frontend task

Read frontend AGENTS.md, docs/02-frontend.md and current auth/session client;
read backend docs/000-auth.md, 000-setup.md, 015-demo-seed.md and openapi.yaml.
Old AUTH.md/SETUP.md/UC*.md paths were consolidated into numbered docs.

1. Work on develop; preserve existing unrelated changes and logical commit rules.
2. Configure NEXT_PUBLIC_API_BASE_URL for the operator-provided API. It is public
   and build-time; no DB credentials, S3 keys or real seed passwords belong in it.
3. Normal /login -> mode live -> POST /auth/login -> GET /auth/me still works.
   For an explicitly enabled local/test target, add a separate password-free
   demo selector using POST /auth/local-test/login as described below. Keep
   browser mock shortcuts separate and explicit.
4. Preserve memory-only sessions, server roles, expiry/logout and error handling.
   No localStorage/sessionStorage tokens, role bypass or live-to-mock fallback.
5. Clarify login/help text only if needed; never hard-code real demo passwords
   or turn browser mock shortcuts into hidden credentialed live calls. UI changes
   require the frontend DESIGN.md maintenance procedure.
6. Test Customer/Worker/Admin navigation using privately shared credentials.
   Nida and Ploy share company scope; Somchai must not see their records.
   Respect Admin-only procurement and Worker-only production/quotation actions.
7. Verify real browser network requests and CORS. Test offline/error behavior
   without falling back to mock. Reload still requires login by current design.
8. Distinguish browser-mock tests, backend real HTTP tests and browser-to-real-API
   verification. Do not claim the latter if the operator target is unavailable.

No additional backend endpoints or dependency upgrades are needed. Do not run seeds,
migrations, deploy, or change backend rules as part of the frontend handoff
without separate authority. Use the existing UC1-UC9 screens to create real
test data after account login; persistent workflow fixtures remain deferred.

## Acceptance

### Password-free selector contract (2026-10-29)

The backend operator explicitly enables LOCAL_TEST_ACCOUNT_ENABLE=true and
restarts the API connected to a seeded dedicated demo/test DB. Do not send this
variable as a request field/header or assume frontend environment enables the
server. Backend default false/unset means the route does not exist (404).

Add clearly labelled "Local test accounts" buttons/selector for the six usernames
listed above, only in an explicitly configured frontend local-test UI. Keep
ordinary password login and explicit browser mock mode separate. The selector
must use the REAL transport, never mock routing:

```http
POST /auth/local-test/login
Content-Type: application/json

{"username":"pim.admin"}
```

No password, role, account ID or extra properties. Response is the normal
access_token/token_type/expires_at/account contract in openapi.yaml. Put the
opaque token in the existing memory-only live session, call GET /auth/me and
use server role navigation. Normal logout must POST /auth/logout. Do not
hard-code passwords, fabricate tokens/roles, persist tokens in browser storage,
or fall back to browser mocks on 401/404/429/network errors. Show a useful
disabled/unseeded or rate-limit error instead; never auto-create seed accounts.

Verify Admin/Worker/Customer buttons, Nida/Ploy shared company and Somchai
isolation, unauthorized actions, logout, disabled endpoint and offline behavior
against an actual running API. Backend verification is not frontend browser
verification; this change did not modify frontend code or leave an API running.

Only trusted local testers may reach an enabled backend: password-free Admin
is intentional privileged access, NOT safe public demo authentication. Turning
off the flag and restarting removes new shortcut logins; existing sessions
need normal revocation/expiry. Seed retirement disables the used demo identities.

### Existing acceptance checks

- Correct role route and Connected API mode after normal login.
- Private credentials never appear in source, logs, browser storage or URLs.
- Company scope and staff/admin boundaries match server responses.
- Empty/loading/error states use real responses; no borrowed mock records.
- Mock mode remains explicit and isolated; its reset cannot affect PostgreSQL.
- Report actual tested API revision/target, checks and remaining limitations.
