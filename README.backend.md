# SA-Lathe-Shop-BE backend

Go + chi API and separate worker with PostgreSQL and private S3-compatible
local storage. Scaffolding phases 0-2, staff auth and UC1-UC9 (Phases 3-12) are complete.

The existing README.md has been preserved exactly.

- [Setup and commands](docs/SETUP.md)
- [Staff auth API and first Admin setup](docs/AUTH.md)
- [Customer registration, company invitations, requests and files](docs/UC1.md)
- [Phase 4 verification](docs/UC1_VERIFICATION.md)
- [UC2 pricing and quotation API](docs/UC2.md)
- [Phase 5 verification and Docker recovery](docs/UC2_VERIFICATION.md)
- [UC3 customer decisions and acceptance terms](docs/UC3.md)
- [Phase 6 verification](docs/UC3_VERIFICATION.md)
- [UC4 deposit/PO verification and job opening](docs/UC4.md)
- [Phase 7 verification](docs/UC4_VERIFICATION.md)
- [UC5 catalog, stock and material reservations](docs/UC5.md)
- [Phase 8 verification](docs/UC5_VERIFICATION.md)
- [UC6 Admin supplier orders and flexible job references](docs/UC6.md)
- [Phase 9 verification](docs/UC6_VERIFICATION.md)
- [UC9 delivery, invoices, customer payments and returns](docs/UC9.md)
- [Phase 12 verification](docs/UC9_VERIFICATION.md)
- [Architecture and open questions](AGENTS.md)
- [Phased plan](PLAN.md)
- [Verification and commands run](docs/VERIFICATION.md)
- [Auth verification](docs/AUTH_VERIFICATION.md)

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
and verified. See [UC7 contract](docs/UC7.md), [plan](docs/UC7_PLAN.md) and
[verification record](docs/UC7_VERIFICATION.md). UC9 is implemented and verified:
staff deliveries/invoices, Admin terms/payment review/cash, customer bank evidence,
installments, free pre-acceptance rework and paid+accepted closure. Both full
UC1-UC9 PostgreSQL/private S3/Docker runs passed on 2026-10-20 and cleaned up their
resources. See the UC9 API/verification records above. No production deployment
or existing database migration was performed; invoice API returns JSON, not PDFs.

UC8 is implemented and verified with all five production policies confirmed:
Worker start/actual use/time/completion and optional private photos, Admin actual
labor cost, consumption-aware plan reads and unused release. Current code needs
migrations through 000012 (UC8 itself introduced 000011). Build/vet/lint and combined real UC1-UC8 PostgreSQL/S3/
Docker verification passed on 2026-10-19 in sa-lathe-uc8-a664942c, with scoped
cleanup confirmed. No existing database was migrated or production deployment
performed. See [UC8 API](docs/UC8.md), [plan](docs/UC8_PLAN.md) and
[verification record](docs/UC8_VERIFICATION.md).

Worker management now requires email and phone on create, validates email syntax,
supports international phone formatting and returns contacts on Worker reads.
Apply migration 000006 first; existing accounts remain intact. See
[Worker contact verification](docs/WORKER_CONTACTS_VERIFICATION.md).
