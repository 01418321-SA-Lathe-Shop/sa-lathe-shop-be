# SA-Lathe-Shop-BE backend

Go + chi API and separate worker with PostgreSQL and private S3-compatible
local storage. Scaffolding phases 0-2 and Phase 3 staff auth are complete.

The existing README.md has been preserved exactly.

- [Setup and commands](docs/SETUP.md)
- [Staff auth API and first Admin setup](docs/AUTH.md)
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
Staff auth is available; notification dispatch and UC1-UC9 are future phases.
