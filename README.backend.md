# SA-Lathe-Shop-BE backend

Go + chi API and separate worker with PostgreSQL and private S3-compatible
local storage. Scaffolding phases 0-2, staff auth and UC1-UC5 (Phases 3-8) are complete.

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
Apply migrations through 000008 before deploying the current API/worker. UC6-UC9 remain unapproved.

Worker management now requires email and phone on create, validates email syntax,
supports international phone formatting and returns contacts on Worker reads.
Apply migration 000006 first; existing accounts remain intact. See
[Worker contact verification](docs/WORKER_CONTACTS_VERIFICATION.md).
