# Phase 3 staff authentication

Phase 4 update: customer self-registration and login are now implemented; see
UC1.md. Migration 000003 renames `staff_account` to `account`, preserving staff
IDs/hashes/sessions. The staff rules below remain applicable; the original
Phase 3-only customer/registration scope statement is historical.

Admin and Worker accounts only. Customer login remains undecided. Suppliers
have no accounts. No public registration or Admin-promotion API exists.

## Setup

Configure private DB values in a local .env, then in PowerShell:

```powershell
./tasks.ps1 dev-up
./tasks.ps1 migrate-up
./tasks.ps1 bootstrap-admin
./tasks.ps1 run
```

bootstrap-admin prompts for username, display name and a hidden password. It
runs cmd/api -bootstrap-admin and clears its temporary credential environment.
It creates the first Admin and exits. Repeating it refuses to overwrite/create
another Admin. Concurrent bootstrap is serialized by a PostgreSQL transaction
advisory lock. Migrations do not seed accounts or passwords.

For trusted noninteractive setup, provide BOOTSTRAP_ADMIN_USERNAME,
BOOTSTRAP_ADMIN_NAME and BOOTSTRAP_ADMIN_PASSWORD in private process environment
and run go run ./cmd/api -bootstrap-admin. Never place bootstrap passwords in
source, command-line arguments, Compose files or .env.example.

## API

| Method | Path | Permission |
| --- | --- | --- |
| POST | /auth/login | Username/password; rate limited |
| GET | /auth/me | Active Admin or Worker session |
| POST | /auth/logout | Active session; revokes it; HTTP 204 |
| GET | /admin/workers?limit=50&offset=0 | Admin; limit 1-100 |
| POST | /admin/workers | Admin; creates Worker only |
| GET | /admin/workers/{account_id} | Admin |
| PATCH | /admin/workers/{account_id} | Admin; edits/deactivates/reactivates Worker |

Use Content-Type: application/json for request bodies. Worker management uses
account id, not person id. Unknown JSON fields (including role) are rejected.
Requests are capped at 8 KiB and multiple JSON values are rejected.

Login request shape:

```json
{"username":"your.username","password":"your-private-password"}
```

Login returns access_token, token_type (Bearer), expires_at and account.
Send Authorization: Bearer <access_token> on protected requests. Credentials
are not accepted in URL parameters or cookies. Protected/token responses use
Cache-Control: no-store. Keep browser tokens in memory; do not persist them in
localStorage or print them. Use HTTPS in deployment. Frontend session persistence
and any future cookie/CSRF design remain decisions for frontend integration.

### Required Worker contacts — implemented

Admin Worker management must match the confirmed frontend requirement: **name,
email and phone are all required**. This adds email and phone; it does not remove
the existing username, password or employee_no requirements. Admin may add/edit,
deactivate/reactivate and change Worker passwords; no deletion or role promotion.

The implemented contract is:

| Operation | Behavior |
| --- | --- |
| POST /admin/workers | Require username, password, name, employee_no, email and phone; position remains optional |
| PATCH /admin/workers/{account_id} | Accept a subset of username, password, name, employee_no, email, phone, position and is_active. Omitted contact fields retain their values; supplied name/email/phone must not be blank or removed |
| Worker list/detail/create/update responses | Include email and phone with the Worker profile so contact edits round-trip through the API |

Admin-created Workers require both email and phone. Missing, blank or malformed
contacts return 400. PATCH preserves omitted contacts but rejects explicit null
or blank contact fields, including when another valid field is supplied. Unknown
JSON fields remain rejected. Roles and existing session-revocation rules are
unchanged; Worker/Customer accounts cannot manage staff.

Email: trim outer whitespace, validate a single bare address with Go net/mail,
maximum 254 bytes; no display names, lists or CR/LF. Preserve address case.
Phone: allow local and international numbers, optional leading +, 7-15 ASCII
digits; accept spaces, hyphens, parentheses and dots as display separators and
remove them on save. Maximum raw length 64 bytes; extensions are not supported.
This is syntax validation, not email/phone ownership or deliverability verification.
Neither email nor phone is unique or used as a login identifier.

Create example (replace the demo password):

```json
{"username":"worker.one","password":"replace-with-private-password","name":"Worker","employee_no":"EMP-1","email":"worker@example.com","phone":"+66 81 234 5678"}
```

Worker list/detail/create/PATCH responses include email and phone. Phone in the
example is returned as +66812345678. Login/auth-me account DTOs are unchanged.
Apply migration 000006 before deployment. It reuses person.email/phone and removes
their obsolete unique indexes, without adding duplicate contact columns.

Legacy Workers are preserved, including their sessions and nullable contacts;
responses expose missing contacts as null. Admin can fill them using PATCH.
New Worker inserts require contacts in the database, and populated Worker
contacts cannot be cleared. Nullable NOT VALID format constraints avoid scanning
or inventing old data and check new/updated rows; pre-existing malformed contacts
must be corrected when that person is next written. Existing Workers are not
silently disabled or forced to use fabricated contact details.

Migration 6 down restores old uniqueness only if there are no shared contacts;
otherwise it refuses before changing the schema. Back up first and prefer a
planned forward fix. Never discard duplicates to make rollback succeed.

Usernames are case-insensitive ASCII, normalized lowercase, 3-64 characters
using letters, digits, _, . and -. Employee numbers are normalized uppercase,
1-30 characters using the same symbols. Names support Unicode, 1-200 characters;
position max 100. Passwords have at least 12 Unicode characters and max 128 UTF-8
bytes, are not trimmed, and use random-salt Argon2id (64 MiB, 3 passes, 2 lanes).
Hasher concurrency is bounded at two per API process. Password hashing uses
golang.org/x/crypto/argon2 behind a service interface, following OWASP's
[password storage guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html).

## Sessions and permissions

Session tokens contain 32 crypto-random bytes; PostgreSQL stores SHA-256 digests
only, never raw tokens. Password hashes never appear in JSON. Database lookup on
every protected request checks account activity, session expiry and revocation;
there is no auth/status cache. AUTH_SESSION_TTL defaults to 12h, max 168h; expiry
is absolute. Multiple sessions are allowed. Logout revokes the current session.
Deactivation, username change and password reset revoke all Worker sessions in
the same transaction. Reactivation does not restore revoked sessions.

Already-running requests may finish; every subsequent request must reauthenticate.
Login rechecks username, password-hash snapshot and activity under a row lock so
concurrent credential/account changes cannot leave a new usable session behind.
The design follows [OWASP session guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html).

Service methods enforce Admin permissions even when called outside HTTP; repository
operations use parameterized SQL and transactions. Role checks live in middleware
and services. Only repository implementations import pgx or execute SQL.

Login allows 10 attempts/minute/socket-peer IP per API process. The limiter's map
is bounded; X-Forwarded-For is not trusted. Configure trusted proxy handling and
deployment-wide abuse protection once hosting is chosen; the limiter is not a
cache of application data. Invalid/disabled/nonexistent accounts share a generic
401. JSON validation returns 400, permission failures 403, missing Worker 404,
duplicate username/employee number 409, and private server details remain hidden.

## Schema and tests

Migration 000002 creates person, staff_account, admin, worker and auth_session.
staff_account.is_active is the single source of staff access status. Future UC
tables can reference person/worker without tying business data to sessions.
The down migration deletes staff and session records; back up before rollback.

```powershell
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
```

Integration tests skip unless TEST_DATABASE_URL points to an isolated migrated
database named sa_lathe_shop_phase3_test with no existing Admin. Supply random
TEST_ADMIN_PASSWORD and TEST_WORKER_PASSWORD through process environment, then:

```powershell
go test -count=1 -v ./internal/repository/postgres
```

Tests cover one-time/concurrent bootstrap, real HTTP permissions, malformed input,
generic credential failures, logout, expiry, deactivation/reactivation, credential
reset races, duplicate-record rollback and absence of raw credentials in storage.
Tests do not truncate or overwrite an existing DB.

Full disposable contact and UC1-UC3 regression: `./scripts/verify-uc3.ps1`.
See WORKER_CONTACTS_VERIFICATION.md for results and deployment boundaries.
