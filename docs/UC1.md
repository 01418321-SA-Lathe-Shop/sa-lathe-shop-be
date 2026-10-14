# Phase 4 — Customer registration and request submission

Approved scope: UC1 only, on `develop`. UC2–UC9 are not implemented.

## Run locally

Supply private DB/S3 credentials in an untracked `.env`. Keep LINE disabled while
testing. Apply all migrations before starting the new API/worker.

```powershell
./tasks.ps1 dev-up
./tasks.ps1 migrate-up
./tasks.ps1 bootstrap-admin # only when no Admin exists
./tasks.ps1 run
```

In a second terminal: `./tasks.ps1 worker`. Container alternative after migration:
`docker compose --profile app up --build -d api worker`.

`S3_ENDPOINT` is the API's internal storage address; `S3_PUBLIC_ENDPOINT` is the
browser-reachable signing origin. Compose uses `http://s3:8333` internally and
`http://localhost:19000` for links. Update the public origin when changing the
exposed port. Production requires HTTPS, a private bucket, restrictive storage
credentials and bucket CORS limited to the real frontend origin and signed
PUT/GET headers. CORS does not grant anonymous read access.

## Registration and company ownership

Public, rate-limited `POST /auth/register` accepts:

```json
{
  "username": "customer_one",
  "password": "replace-with-a-private-password",
  "name": "Customer name",
  "customer_type": "individual"
}
```

For a new company, use `customer_type: company` and `company_name`. The registrant
becomes the membership owner. Names do not merge companies or grant access;
duplicate names represent separate customer records, not verified business IDs.
For an existing company, use `invitation` instead of `company_name`.

Company owners issue invitations through `POST /customers/{customer_id}/invitations`.
Tokens are shown once, expire after 24 hours and can be used once; only SHA-256
digests are stored. Share privately. Normal members cannot issue invitations.
Invitation redemption locks the invitation row and registration rolls back as a
unit on failure. Existing customers join another company using
`POST /customers/join` with `{"invitation":"..."}`. `GET /customers` lists only
their memberships. No name-based joining or public company search exists.

Customer login/logout/me use existing `/auth/login`, `/auth/logout`, `/auth/me`.
The same Argon2id hashing and revocable sessions protect staff and customers.
Registration cannot set roles or active-state flags. Usernames are globally
unique. Email verification, password reset and membership transfer are not part
of this phase. Company members can read all requests in their company.

## Authenticated endpoints

| Method | Path | Access / behavior |
|---|---|---|
| GET | `/customers` | Own customer/company memberships |
| POST | `/customers/{customer_id}/invitations` | Company owner only |
| POST | `/customers/join` | Customer with a valid invitation |
| POST | `/requests` | Customer member of selected customer ID |
| GET | `/requests` | Own companies; Admin/Worker shop queue |
| GET | `/requests/{request_id}` | Same read boundary |
| POST | `/requests/{request_id}/uploads` | Customer member; create upload intent |
| POST | `/uploads/{attachment_id}/complete` | Original uploader only |
| GET | `/requests/{request_id}/attachments` | Authorized reader; ready files only |
| GET | `/attachments/{attachment_id}/download` | Authorized request reader |
| GET | `/notifications` | Own inbox only |
| PATCH | `/notifications/{notification_id}/read` | Own notification, idempotent |

Use `Authorization: Bearer <token>`. Lists take `limit` 1–100 (default 50) and
`offset` >=0. JSON parsing rejects unknown fields/extra values. Existing JSON
error format applies: 400 invalid input, 401 missing/revoked session, 403 denied
creation, 404 missing **or inaccessible** request/file, 409 uniqueness conflict.

Request input follows the ER:

```json
{
  "customer_id": "UUID returned by GET /customers",
  "description": "Rubber compression mould",
  "qty": 2,
  "material_spec": "S45C",
  "deadline": "2026-12-01T10:00:00+07:00"
}
```

Description: required, 1–10,000 Unicode characters; material specification:
optional, up to 2,000. Quantity: positive int64, no artificial business maximum.
Requested deadline: optional future instant, not a promised production due date.
Server sets creator, number, source `net`, status `pending` and timestamps.
Request and two notification outbox rows commit atomically. Later-use-case
decisions are recorded, not implemented: sixth quotation rejection cancels;
Deposit/PO deadline is quotation acceptance plus 14 days.

## Private uploads

POST an upload intent with `file_name`, `content_type`, `byte_size`, PUT the bytes
to the returned URL with returned headers, then POST completion. Supported types:
PDF, DOC/DOCX, XLS/XLSX, TXT, JPEG/PNG, max **100,000,000 bytes per file**.
MIME type must match the extension. Upload/download links expire in 10 minutes;
do not persist or log them. State flow: `pending -> checking -> ready/failed`.
Failed completion needs a new intent; a crashed `checking` intent is unavailable
and also needs a fresh intent. Only the original uploader may complete it.

Validation checks exact size and content format, using temporary disk and at
most two concurrent validators per API process. TXT uses streaming UTF-8 checks;
images check dimensions/header; OOXML checks format entries and rejects VBA;
legacy DOC/XLS checks OLE signature. This is **not malware scanning**. Download
links force attachment/octet-stream. Production needs a separate malware policy
before staff open untrusted documents.

Validated staging objects are copied to distinct final keys with an ETag
precondition. A still-valid upload link cannot replace finalized evidence. Only
keys and metadata enter PostgreSQL; no file bytes or permanent public URLs.
Unauthorized customers cannot get links; final objects deny anonymous reads.
Use a lifecycle rule for `staging/` objects after at least one day, never for
final evidence. DB/S3 failures can leave a private orphan final object; reconcile
operationally. This is not a distributed transaction or permission to delete
business evidence.

## Notification delivery

Worker claims outbox rows with `FOR UPDATE SKIP LOCKED`, 60-second leases and
claim tokens. Inbox insertion and event acknowledgement commit together;
`(event_id,account_id)` prevents duplicate inbox rows. New-request notifications
go to each active Admin/Worker's own inbox, plus confirmation to the submitting
customer. Unrelated customers receive nothing. Read flags are per-user.

LINE uses only the configured **Admin user**, never groups/broadcast/client-sent
recipients. Private environment configuration:

```text
LINE_ENABLED=false
LINE_CHANNEL_ACCESS_TOKEN=
LINE_ADMIN_USER_ID=
```

Enable only when ready to send real alerts. The `U...` Admin user must be reachable
by the configured Official Account. Messages contain request number/ID only, no
customer names, evidence or signed links. Stable event UUID is `X-Line-Retry-Key`.
409 is accepted only with LINE's accepted-request header. Disabled events remain
pending without spending attempts. Failures back off exponentially, and after
10 failures mark the outbox row failed for operator inspection. Inbox delivery
does not depend on LINE success. Dispatch is at-least-once, not guaranteed
end-user receipt. Monitor/reconcile failed rows before production.

References: [LINE messages](https://developers.line.biz/en/docs/messaging-api/sending-messages/),
[LINE retry keys](https://developers.line.biz/en/docs/messaging-api/retrying-api-request/),
[AWS Go S3](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_s3_code_examples.html).

## Migration and test contract

Migration 000003 renames `staff_account` to `account` in place, preserving staff
UUIDs, hashes and sessions. Apply before deploying the new binary. Down migration
refuses rollback when customer data exists; use backups and a planned forward
fix for populated databases. Never force dirty production migrations.

Run `go build ./...`, `go vet ./...`, `go test ./...`.
Real UC1 integration tests require a **fresh disposable** migrated database
`sa_lathe_shop_phase4_test` via `PHASE4_DATABASE_URL`, private
`TEST_ADMIN_PASSWORD`, `TEST_S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, and a
disposable S3 bucket `sa-lathe-shop-private`. Tests refuse existing accounts,
never truncate user tables, and never send real LINE messages. Staff regressions
retain the separate `TEST_DATABASE_URL` / `sa_lathe_shop_phase3_test` contract.
