# SA-Lathe-Shop-BE backend

Go + chi API and separate worker with PostgreSQL and private S3-compatible
local storage. Scaffolding phases 0-2, staff auth and UC1-UC9 (Phases 3-12) are complete.

The existing README.md has been preserved exactly.

- [System Architecture & Requirements](docs/000-requirements.md)
- [Backend Setup & Git Workflow](docs/000-setup.md)
- [Student Deployment to Railway](docs/000-deploy-railway.md)
- [Authentication & Staff Management](docs/000-auth.md)
- [UC1 — Customer Registration & Requests](docs/001-UC1.md)
- [UC2 — Quotation & Price Estimation](docs/002-UC2.md)
- [UC3 — Customer Quotation Decisions & Terms](docs/003-UC3.md)
- [UC4 — Deposit/PO Evidence & Job Opening](docs/004-UC4.md)
- [UC5 — Inventory Catalog & Material Plans](docs/005-UC5.md)
- [UC6 — Supplier Procurement Orders](docs/006-UC6.md)
- [UC7 — Material Receiving & Settlement](docs/007-UC7.md)
- [UC8 — Production Execution & Usage](docs/008-UC8.md)
- [UC9 — Delivery, Invoices & Free Rework](docs/009-UC9.md)
- [API Examples (PowerShell)](docs/010-api-examples.md)
- [Security & Permission Review](docs/011-security-review.md)
- [Performance & Latency Benchmarks](docs/012-performance.md)
- [OpenAPI 3.0 Specification](docs/openapi.yaml)
- [Guarded demo accounts and password-free local login](docs/015-demo-seed.md)
- [Frontend demo integration handoff](docs/016-frontend-demo-handoff.md)
- [PO/job/request workflow testing dataset](docs/017-demo-workflows.md)
- [System analysis digest](docs/018-system-analysis.md)
- [Source report to backend traceability](docs/019-source-traceability.md)
- [Printed document details](docs/020-printed-documents.md)
- [ER diagram vs implemented schema](docs/021-er-schema-comparison.md)
- [UC10 — Unit setting](docs/022-UC10.md)
- [UC11 — Add Worker](docs/023-UC11.md)
- [UC12 — Add Admin (not approved)](docs/024-UC12.md)
- [Use-case description alignment (UC1–UC12)](docs/025-uc-spec-alignment.md)
- [ER-aligned demo dataset handoff](docs/027-demo-dataset-handoff.md)
- [Architecture and open questions](AGENTS.md)
- [Phased plan](PLAN.md)

PowerShell quick start after configuring private values in a new local .env:

```powershell
./tasks.ps1 dev-up
./tasks.ps1 migrate-up
./tasks.ps1 bootstrap-admin
./tasks.ps1 run
```

GET /health is liveness. GET /health/ready checks PostgreSQL.
To run API and worker in Docker: docker compose --profile app up --build -d.
Staff/customer auth, request uploads and notification dispatch are available.
Quotation and customer-decision code passed real PostgreSQL/S3/regression/Docker verification.
Current binaries require migrations through 000012. UC7 receiving/claims for
Admin/Worker and Admin-only financial details/settlement/evidence are implemented
and verified. See [UC7 guide](docs/007-UC7.md). UC9 is implemented and verified:
staff deliveries/invoices, Admin terms/payment review/cash, customer bank evidence,
installments, free pre-acceptance rework and paid+accepted closure. Both full
UC1-UC9 PostgreSQL/private S3/Docker runs passed on 2026-10-20 and cleaned up their
resources. See [UC9 guide](docs/009-UC9.md). No production deployment
or existing database migration was performed; invoice API returns JSON, not PDFs.

UC8 is implemented and verified with all five production policies confirmed:
Worker start/actual use/time/completion and optional private photos, Admin actual
labor cost, consumption-aware plan reads and unused release. Current code needs
migrations through 000012 (UC8 itself introduced 000011). Build/vet/lint and combined real UC1-UC8 PostgreSQL/S3/
Docker verification passed on 2026-10-19 in sa-lathe-uc8-a664942c, with scoped
cleanup confirmed. No existing database was migrated or production deployment
performed. See [UC8 guide](docs/008-UC8.md).

Worker management now requires email and phone on create, validates email syntax,
supports international phone formatting and returns contacts on Worker reads.
Apply migration 000006 first; existing accounts remain intact. See
[Auth & Worker Contacts guide](docs/000-auth.md).
