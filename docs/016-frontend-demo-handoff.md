# Frontend handoff: fictional accounts, real backend

## What exists

Backend develop provides a guarded account-only seed command, migration 000013
for provenance and real HTTP verification. It reuses existing Argon2id, opaque
Bearer sessions and permissions. No API contracts or login endpoints changed.
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
3. Use normal /login -> mode live -> POST /auth/login -> GET /auth/me. Existing
   components already support this. Keep explicit mock shortcuts browser-only.
4. Preserve memory-only sessions, server roles, expiry/logout and error handling.
   No localStorage/sessionStorage tokens, role bypass or live-to-mock fallback.
5. Clarify login/help text only if needed; never hard-code real demo passwords
   or turn mock shortcut buttons into credentialed live shortcuts. UI changes
   require the frontend DESIGN.md maintenance procedure.
6. Test Customer/Worker/Admin navigation using privately shared credentials.
   Nida and Ploy share company scope; Somchai must not see their records.
   Respect Admin-only procurement and Worker-only production/quotation actions.
7. Verify real browser network requests and CORS. Test offline/error behavior
   without falling back to mock. Reload still requires login by current design.
8. Distinguish browser-mock tests, backend real HTTP tests and browser-to-real-API
   verification. Do not claim the latter if the operator target is unavailable.

No new feature endpoints or dependency upgrades are needed. Do not run seeds,
migrations, deploy, or change backend rules as part of the frontend handoff
without separate authority. Use the existing UC1-UC9 screens to create real
test data after account login; persistent workflow fixtures remain deferred.

## Acceptance

- Correct role route and Connected API mode after normal login.
- Private credentials never appear in source, logs, browser storage or URLs.
- Company scope and staff/admin boundaries match server responses.
- Empty/loading/error states use real responses; no borrowed mock records.
- Mock mode remains explicit and isolated; its reset cannot affect PostgreSQL.
- Report actual tested API revision/target, checks and remaining limitations.
