# Authentication & Staff Management

## 1. Authentication Architecture

Optional password-free local testing: only `LOCAL_TEST_ACCOUNT_ENABLE=true`
registers `POST /auth/local-test/login` with `{ "username": "pim.admin" }`.
Default OFF (404). Requires the dedicated seeded demo/test database and active
provenance-owned identity; returns the standard revocable Bearer session.
Normal password login and role/company authorization are unchanged. Never
expose an enabled API publicly. See [015-demo-seed.md](015-demo-seed.md) for
all six accounts, exact guards and disable/revocation behavior.

The system provides role-based authentication for **Admin**, **Worker**, and **Customer** accounts. Suppliers have no accounts and interact externally. Public registration is restricted to customers (`/auth/register`, UC1). Admin and Worker accounts are strictly provisioned by Admin.

### Credential & Session Specifications
- **Password Hashing**: OWASP-aligned Argon2id (64 MiB memory, 3 passes, 2 lanes) with random salt via `golang.org/x/crypto/argon2`. Hasher concurrency is bounded at 2 per API process. Passwords require min 12 Unicode characters and max 128 UTF-8 bytes.
- **Session Tokens**: 32 cryptographically random bytes transmitted via `Authorization: Bearer <token>`.
- **Digest Storage**: PostgreSQL stores only SHA-256 digests of active session tokens; raw tokens never enter storage or logs. Password hashes never appear in API responses.
- **Session Expiry & Revocation**: Configurable absolute TTL (default 12 hours via `AUTH_SESSION_TTL`, max 168 hours). Protected responses send `Cache-Control: no-store`. Every request verifies account active status and session validity directly in PostgreSQL with no caching.
- **Atomic Invalidation**: Account deactivation, password reset, or username change atomically revokes all active sessions for that account in the same transaction. Login serializes under an account row lock to prevent race conditions.
- **Rate Limiting**: Login allows at most 10 attempts per minute per socket IP address. Failed and disabled credentials share a generic 401 error.

### Admin Provisioning

Optional local/test fictional accounts use the guarded command described in
[015-demo-seed.md](015-demo-seed.md), reusing this same hashing and initial-Admin
policy. The seed does not add public account-provisioning routes or reset an
existing account. Frontend browser mock credentials are never real seed passwords.
The initial Admin account cannot be created via public HTTP. It is bootstrapped through the CLI:
```powershell
./tasks.ps1 bootstrap-admin
# Prompts for username, display name, and password securely.
# Concurrent execution is guarded by a PostgreSQL advisory lock.
```
For non-interactive setup, environment variables `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_NAME`, and `BOOTSTRAP_ADMIN_PASSWORD` can be provided.

---

## 2. Worker Management API

Only active **Admin** users can create and manage Worker accounts.

| Method | Path | Permission | Description |
|---|---|---|---|
| `POST` | `/auth/login` | Public (rate limited) | Authenticate with username and password |
| `GET` | `/auth/me` | Active Session | Returns current account and role profile |
| `POST` | `/auth/logout` | Active Session | Revokes the current session token (HTTP 204) |
| `POST` | `/auth/password` | Active Session (rate limited) | Change own password: `{current_password, new_password}` (HTTP 204) |
| `GET` | `/admin/workers?limit=50&offset=0` | Admin | List workers with pagination (limit 1–100) |
| `POST` | `/admin/workers` | Admin | Create a new worker account |
| `GET` | `/admin/workers/{account_id}` | Admin | Fetch worker profile |
| `PATCH` | `/admin/workers/{account_id}` | Admin | Edit worker, change password, or deactivate/reactivate |

### Self-Service Password Change (2026-11-03)
Every active role (Admin, Worker, Customer) may change only its own password. The
service verifies `current_password` with Argon2id, applies the normal password rule,
and also requires the new password to differ from the current one and not contain
the username. A wrong current password returns HTTP 400 `invalid_current_password`
(never 401, so the caller stays signed in). On success the hash is replaced under an
account row lock and every *other* session is revoked; the calling session stays
active. No migration. Admin resets of Worker passwords via `PATCH /admin/workers/{id}`
are unchanged.

### Required Worker Contacts & Validation (Migration 000006)
Admin worker creation requires all of the following fields:
- `username`: ASCII letters, digits, `_`, `.`, `-` (3–64 chars, normalized lowercase)
- `password`: min 12 Unicode characters, max 128 bytes
- `employee_no`: normalized uppercase, 1–30 chars
- `name`: Unicode, 1–200 chars
- `email`: Bare RFC 5322 address validated by Go `net/mail`, max 254 bytes
- `phone`: Local or international format, optional leading `+`, 7–15 digits (separators stripped on save)
- `position`: Optional, max 100 chars

```json
{
  "username": "worker.one",
  "password": "replace-with-a-secure-password",
  "name": "Technician Name",
  "employee_no": "EMP-001",
  "email": "worker1@example.com",
  "phone": "+66 81 234 5678",
  "position": "Senior Lathe Machinist"
}
```

### Contact Policies & Legacy Compatibility
1. **Non-Unique Contacts**: Email and phone numbers are not unique identifiers and are not used as login credentials.
2. **PATCH Updates**: Omitted fields retain current values. Supplied fields cannot be blank or null. An existing populated email or phone cannot be erased.
3. **Legacy Preservation**: Pre-existing workers without contacts are preserved with `null` contact fields until Admin supplies them via PATCH. Format constraints use `NOT VALID` checks to prevent breaking historical records.
4. **No Role Promotion / Deletion**: Admin cannot promote workers to Admin or delete accounts. Deactivation sets `is_active = false` and revokes all sessions.

---

## 3. Database Schema & Migrations

- **Migration 000002**: Creates `person`, `staff_account`, `admin`, `worker`, and `auth_session`. `is_active` defines account state.
- **Migration 000003**: Renames `staff_account` to `account` in place, adding customer types while preserving staff UUIDs, hashes, and active sessions.
- **Migration 000006**: Reuses `person.email` and `person.phone`, removes obsolete unique indexes, and enforces non-empty contact validation on new worker inserts.

---

## 4. Verification Records

### Phase 3 Staff Auth Verification (2026-10-14)
- **Status**: PASSED (`PHASE3_VERIFICATION_PASSED`).
- **Checks Verified**:
  - Argon2id hash verification and rejection of malformed/tampered hashes.
  - Concurrency: advisory locks ensure exactly one Admin is created on simultaneous bootstrap calls; duplicate employee numbers roll back cleanly.
  - Session revocation: logout, password reset, username change, and deactivation immediately block subsequent requests. Stale login snapshots cannot revive revoked sessions.
  - Docker flow: non-root API and worker containers build, start, authenticate, and gracefully shut down on SIGTERM.

### Worker Contacts Follow-up Verification
- **Status**: PASSED (`WORKER_CONTACTS_VERIFICATION_PASSED`).
- **Checks Verified**:
  - Validation: rejects missing contacts, malformed emails, and invalid phone numbers on worker creation.
  - Normalization: international numbers with `+` and country codes preserve leading digits while punctuation is stripped.
  - Non-uniqueness: identical email and phone shared across multiple workers permitted in PostgreSQL.
  - Rollback safety: migration 000006 up/down/up preserves pre-existing contact-less workers; populated shared contacts safely prevent destructive downgrade.
