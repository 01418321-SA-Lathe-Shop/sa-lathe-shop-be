# UC1-UC9 security and permission review

Reviewed the registered handlers, feature service guards, PostgreSQL authorization/transaction paths, file lifecycle, role-redaction code, migrations and integration tests on 2026-10-23. This is a source review of the current `develop` implementation, not a penetration test or deployment certification.

| Area | Evidence in implementation | Assessment |
|---|---|---|
| Authentication | `internal/auth`: Argon2id passwords, opaque random session tokens, stored token digests, revocable DB-backed sessions checked on protected requests; customer registration is separate from Admin bootstrap. | Controls are present. Token is a Bearer secret; API uses no browser cookie by default. |
| Account and role gates | Auth middleware rejects inactive/unknown roles; feature services independently enforce roles. Admin-only worker management, procurement, stock correction, supplier finance, invoice terms/cash/review and actual labor cost are guarded. | Handler guard and service-level checks are both used where applicable. |
| Customer/company ownership | Request, quotation decision, deposit evidence and fulfillment repositories verify current active membership under transaction locks; cross-company inaccessible IDs are generally returned as not found. Invitation is owner-issued/one-use. | Membership is checked at use time; client-provided customer IDs are not trusted. |
| Internal financial data | Quotation `staffView` omits internal costs for customers; material/movement explicit allowlists omit cost from Worker views; delivery redaction clears receipt details for Workers; receiving strips finance. | Explicit allowlists/redactors reduce accidental field leakage. Regression tests cover representative cases. |
| Files | S3 keys/metadata only; short-lived signed links; private buckets; MIME and byte signature checks, size limits, staging/final object separation, conditional freeze using ETag, and scope checks before download. | No public evidence route found. Malware scanning and retention/lifecycle remain deployment decisions. |
| Financial/stock commands | Repository transactions lock the relevant request/job/invoice/material/order rows; idempotency and expected versions protect retries; audit and outbox events are atomic with state changes. UC9 enforces allocation <= invoice total. | No unsafe bypass identified in reviewed code paths; PostgreSQL integration tests exercise races and rollback. |
| Notifications and errors | Inbox reads are per-account and UC9 recipients are scoped to current members/active Admins; errors return generic internal failures rather than DB details. | No customer payment proof or amount in notification titles. Real LINE is disabled during verification. |
| HTTP boundary | JSON decoders reject unknown fields and enforce body limits; CORS uses configured allowlist; request IDs, timeout, recovery and structured request logging are installed. | No credentials are intended in logs. Keep production origins and proxy configuration explicit. |

## Findings and follow-up

| Severity | Finding | Action / evidence |
|---|---|---|
| Medium, deployment-dependent | Login throttling is in-process and uses the socket peer address. Multiple API replicas do not share its counters; reverse-proxy client identity is not trusted by default. | Keep the API directly protected at the edge or add a trusted-proxy-aware/shared limiter when deployment topology is selected. No proxy header should be trusted until proxy CIDRs are defined. No current authorization bypass found. |
| Medium, deployment-dependent | Upload content is checked by allowlist/signature and bounded size, but there is no malware scanning service. | Enable scanning/quarantine before production if the risk model requires it; keep files private and untrusted until the scan policy is decided. |
| Low, operational | Failed outbox delivery and database/S3 health need operational alerting/retention policy. | Choose alert ownership, retry limits, object retention, and backup/restore checks with hosting. Not implemented as a product feature in this phase. |
| Low, performance | `FulfillmentJobs` reads a page of IDs and then loads each item through a separate transaction (`internal/repository/postgres/delivery_read.go`). | Page size is capped at 100; benchmark this list route before considering a snapshot/join redesign. Avoid denormalizing status or balances without measurements. |

No confirmed authorization defect required changing business behavior in this review. The deployment-dependent items remain explicit open decisions, not claims that the service is production-ready under an unknown hosting topology. See `AGENTS.md` open questions and `PERFORMANCE.md` for repeatable measurements.
