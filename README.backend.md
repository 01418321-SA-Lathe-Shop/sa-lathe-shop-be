# SA-Lathe-Shop-BE backend

Go + chi API and separate worker with PostgreSQL and private S3-compatible
local storage. Scaffolding phases 0-2, staff auth and UC1-UC6 (Phases 3-9) are complete.

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
UC1-UC6 baseline uses migrations through 000009; the UC7 code below needs 000010.
UC7 is now authorized and implementation is in progress: receiving/claims for
Admin/Worker, financial details/settlement/evidence for Admin only. Its code needs
migration 000010 and pending real PostgreSQL/S3/Docker verification before phase
completion. See [UC7 contract](docs/UC7.md), [plan](docs/UC7_PLAN.md) and
[verification status](docs/UC7_VERIFICATION.md). UC9 remains unapproved.

UC8 implementation is also in progress with all five production policies confirmed:
Worker start/actual use/time/completion and optional private photos, Admin actual
labor cost, consumption-aware plan reads and unused release. Current code needs
migrations through 000011. Build/vet/unit/lint pass; do not deploy or mark complete
before real UC7/UC8 PostgreSQL/S3/Docker verification. See [UC8 API](docs/UC8.md),
[plan](docs/UC8_PLAN.md) and [verification gate](docs/UC8_VERIFICATION.md).

Worker management now requires email and phone on create, validates email syntax,
supports international phone formatting and returns contacts on Worker reads.
Apply migration 000006 first; existing accounts remain intact. See
[Worker contact verification](docs/WORKER_CONTACTS_VERIFICATION.md).
