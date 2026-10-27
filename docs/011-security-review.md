# UC1–UC9 Security & Permission Review

Reviewed the registered handlers, feature service guards, PostgreSQL authorization/transaction paths, file lifecycle, role-redaction code, migrations and integration tests on 2026-10-23. This is a source review of the current `develop` implementation, not a penetration test or deployment certification.

---

## 1. Security Controls & Implementation Evidence

| Security Area | Evidence in Implementation | Assessment |
|---|---|---|
| **Authentication** | `internal/auth`: Argon2id passwords, opaque random session tokens, stored token digests, revocable DB-backed sessions checked on protected requests; customer registration is separate from Admin bootstrap. | Controls are present. Token is a Bearer secret; API uses no browser cookie by default. |
| **Account & Role Gates** | Auth middleware rejects inactive/unknown roles; feature services independently enforce roles. Admin-only worker management, procurement, stock correction, supplier finance, invoice terms/cash/review and actual labor cost are guarded. | Handler guard and service-level checks are both used where applicable. |
| **Customer / Company Ownership** | Request, quotation decision, deposit evidence and fulfillment repositories verify current active membership under transaction locks; cross-company inaccessible IDs are returned as 404 not found. Invitation is owner-issued/one-use. | Membership is checked at use time; client-provided customer IDs are not trusted. |
| **Internal Financial Data Redaction** | Quotation `staffView` omits internal costs for customers; material/movement explicit allowlists omit cost from Worker views; delivery redaction clears receipt details for Workers; receiving strips finance. | Explicit allowlists/redactors reduce accidental field leakage. Regression tests cover representative cases. |
| **Files & Object Storage** | S3 keys/metadata only; short-lived signed links (10 min); private buckets; MIME and byte signature checks, size limits (100 MB), staging/final object separation, conditional freeze using ETag, and scope checks before download. | No public evidence route found. Malware scanning and retention/lifecycle remain deployment decisions. |
| **Financial / Stock Transactions** | Repository transactions lock the relevant request/job/invoice/material/order rows; idempotency and expected versions protect retries; audit and outbox events are atomic with state changes. UC9 enforces allocation $\le$ invoice total. | No unsafe bypass identified in reviewed code paths; PostgreSQL integration tests exercise races and rollback. |
| **Notifications & Errors** | Inbox reads are per-account and UC9 recipients are scoped to current members/active Admins; errors return generic internal failures rather than DB details. | No customer payment proof or amount in notification titles. Real LINE is disabled during verification. |
| **HTTP Boundary** | JSON decoders reject unknown fields and enforce body limits; CORS uses configured allowlist; request IDs, timeout, recovery and structured request logging are installed. | No credentials in logs. Keep production origins and proxy configuration explicit. |

---

## 2. Findings & Operational Follow-Ups

| Severity | Finding | Action / Evidence |
|---|---|---|
| **Medium** *(Deployment-Dependent)* | Login throttling is in-process and uses the socket peer address. Multiple API replicas do not share its counters; reverse-proxy client identity is not trusted by default. | Keep the API directly protected at the edge or add a trusted-proxy-aware/shared limiter when deployment topology is selected. No proxy header should be trusted until proxy CIDRs are defined. |
| **Medium** *(Deployment-Dependent)* | Upload content is checked by allowlist/signature and bounded size (100 MB), but there is no third-party malware scanning service. | Enable antivirus/malware scanning and quarantine before production if the risk model requires it; keep files private and untrusted until the scan policy is decided. |
| **Low** *(Operational)* | Failed outbox delivery and database/S3 health need operational alerting and retention policies. | Choose alert ownership, retry limits, object retention, and backup/restore checks with hosting provider. Not implemented as an in-app product feature. |
| **Low** *(Performance)* | `FulfillmentJobs` reads a page of IDs and then loads each item through a separate transaction (`internal/repository/postgres/delivery_read.go`). | Page size is capped at 100; benchmark this list route before considering a snapshot/join redesign. Avoid denormalizing status or balances without measurements. |

No confirmed authorization defect required changing business behavior in this review. The deployment-dependent items remain explicit open decisions. See [012-performance.md](012-performance.md) for repeatable local benchmarks.
