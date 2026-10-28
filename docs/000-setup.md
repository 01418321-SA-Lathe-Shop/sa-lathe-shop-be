# Backend Setup, Verification & Git Workflow

## 1. Local Development Runbook

### Prerequisites
- Go 1.27.3 (security baseline; module, CI and Docker builders use the same version)
- Git
- PowerShell 7+
- Docker Desktop with Linux containers

### Dependencies
- Web routing: `github.com/go-chi/chi/v5` and `github.com/go-chi/cors`
- PostgreSQL driver: `github.com/jackc/pgx/v5/pgxpool`
- Database migrations: `golang-migrate/migrate/v4` (pinned CLI v4.19.0)
- Password hashing: `golang.org/x/crypto/argon2`
- Configuration & logging: Go standard library `os` and `log/slog`

### Port Allocations
- **API Server**: `http://localhost:8080`
- **PostgreSQL**: `localhost:55432` (database `sa_lathe_shop`)
- **SeaweedFS S3**: `localhost:19000` (internal API endpoint: `http://s3:8333`)

> Production hosting remains undecided. For student deployment to Railway (no local Docker required), see [000-deploy-railway.md](000-deploy-railway.md).

---

## 2. Quickstart Step-by-Step

```powershell
# 1. Prepare environment variables
Copy-Item .env.example .env -ErrorAction SilentlyContinue
# Edit .env to set DB_PASSWORD, S3_ACCESS_KEY (min 3 chars), S3_SECRET_KEY (min 8 chars)

# 2. Start PostgreSQL and SeaweedFS S3
./tasks.ps1 dev-up

# 3. Apply database migrations
./tasks.ps1 migrate-up

# 4. Bootstrap the initial Admin user (one-time interactive command)
./tasks.ps1 bootstrap-admin

# 5. Start the API server
./tasks.ps1 run

# 6. In a separate terminal, start the background outbox/scanner worker
./tasks.ps1 worker

# 7. Check health endpoints
Invoke-RestMethod http://localhost:8080/health        # Liveness (200 OK)
Invoke-RestMethod http://localhost:8080/health/ready  # Readiness (checks DB connection)
```

### Docker Compose Applications
To run both the API and worker inside Docker containers:
```powershell
docker compose --profile app up --build -d
# Stop containers while preserving data volumes:
docker compose down
```

### Common Helper Tasks
- **Build**: `./tasks.ps1 build` (`go build ./...`)
- **Vet**: `go vet ./...`
- **Test**: `./tasks.ps1 test` (`go test ./...`)
- **Lint**: `./tasks.ps1 lint` (`gofmt -l cmd internal`)
- **Migrations**:
  - Check current version: `./tasks.ps1 migrate-version`
  - Migrate up: `./tasks.ps1 migrate-up`
  - Migrate down (single step): `./tasks.ps1 migrate-down`

---

For optional fictional accounts on a dedicated real PostgreSQL demo database,
see [015-demo-seed.md](015-demo-seed.md). The account-only command requires clean
migration 000013, explicit local/test guards and private process passwords. It
does not bootstrap on startup, seed workflows, provision storage, or target an
existing shop/hosted database. Normal API/worker features still use business
migrations through 000012; 000013 adds only operator seed provenance.

## 3. Scaffolding Verification Record

Initial scaffolding and infrastructure verification was conducted on **2026-10-11 (Asia/Bangkok)**:
- `go build ./...`, `go vet ./...`, `go test ./...` and `./tasks.ps1 lint` passed cleanly.
- API and worker Docker images built and ran successfully via Compose.
- Health checks verified: `/health` returns 200 `{"status":"ok"}`; `/health/ready` returns 200 with DB up and 503 when DB is stopped while liveness remains 200.
- S3 private access verified: anonymous bucket listing returns 403 Forbidden.
- Initial migrations verified: version 1 up, down, and up again passed without errors.
- Graceful shutdown verified on SIGTERM for both API and worker.
- Alternative local image mirrors: Official SeaweedFS GHCR image (digest pinned) and Go/Alpine public ECR mirrors were used to prevent Docker Hub timeouts.

---

## 4. Disposable Integration Verification Architecture

The test suite enforces strict separation between unit tests and real integration tests:
- `go test ./...` runs unit tests and in-memory mock tests. Real integration tests against PostgreSQL and S3 are skipped when dedicated test environment variables are absent.
- Full end-to-end integration is automated via dedicated phase scripts:
  - `./scripts/verify-uc1.ps1` through `./scripts/verify-uc9.ps1`
- GitHub CI also runs `verify-uc9.ps1` as an unconditional separate integration
  job with disposable PostgreSQL/private S3, race/coverage tests and Docker HTTP
  workflows. The verifier checks actual JSON test outcomes: all ten top-level
  repository suites must execute and pass; absent/skipped suites fail the gate.
  Normal unit-only runs retain their opt-in integration behavior. Repository
  branch-protection settings are not changed by adding a workflow job.
- Each script provisions an isolated Docker Compose project (e.g. `sa-lathe-uc*-<hash>`), sets up multiple fresh databases, executes real migrations, runs uncached concurrency and rollback tests, tests Docker HTTP workflows, and cleans up all temporary containers, networks, and volumes upon completion. No local `.env` or existing database data is ever modified.

---

## 5. Mandatory Phase Commit & Push Workflow

Applies to every phase and follow-up. Authorized branch: `develop`.

### During Implementation
1. **Logical Groups**: Plan 3–8 related files per commit. Do not accumulate an entire phase into one giant commit.
2. **Independent Builds**: Every committed snapshot must compile (`go build ./...`) and pass relevant focused tests.
3. **Conventional Commits**: Use descriptive messages explaining what changed and why:
   - `feat: validate quotation pricing inputs`
   - `feat: persist quotation revisions atomically`
   - `feat: expose authorized quotation endpoints`
   - `docs: document quotation API and test results`
4. Avoid vague messages like `update`, `changes`, or `done`. Never squash logical commits into a single catch-all commit before pushing.

### End of Completed Phase Checklist
1. Update `PLAN.md` with completed items, decisions, and any known limitations.
2. Run full checks: `go build ./...`, `go vet ./...`, `go test ./...`, and `./tasks.ps1 lint`.
3. Run the phase integration script (e.g., `./scripts/verify-uc*.ps1`).
4. Inspect `git status` and `git diff` to ensure no temporary data, secrets, or `.env` files are staged.
5. Push logical commits to `origin/develop`:
   ```powershell
   git push origin develop
   ```
6. Verify the remote tip:
   ```powershell
   git status
   git log -n 5 --oneline
   ```
7. Never push to `main` or force-push without explicit owner authorization.
