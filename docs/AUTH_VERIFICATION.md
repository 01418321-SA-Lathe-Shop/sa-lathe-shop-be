# Phase 3 verification

Completed 2026-10-14 Asia/Bangkok on Windows/PowerShell and Docker Desktop Linux.

## Passed checks

- go build ./..., go vet ./..., go test ./..., ./tasks.ps1 lint.
- Service tests use repository/hasher doubles without importing a DB driver.
- Argon2id verifies correct passwords, rejects wrong/malformed hashes, honors
  pre-cancelled contexts and enforces password character/byte limits.
- HTTP validation rejects oversized bodies, unknown role fields, multiple JSON
  values, invalid pagination and missing credentials. Login throttling is bounded.
- Real PostgreSQL tests: concurrent initial Admin creation permits exactly one;
  duplicate employee numbers roll back the person/account transaction.
- Admin can create/list/edit Workers; Worker cannot manage staff; Admin accounts
  cannot be targeted as Workers and role promotion is rejected.
- Invalid/nonexistent/disabled credentials share a generic error.
- Logout, expiry, Worker deactivation, password reset and username change block
  subsequent authenticated requests. Reactivation cannot restore revoked tokens.
- Stale login snapshots after deactivation/password reset/username change cannot
  create sessions. Account row locking serializes these operations.
- Session storage uses digests; responses do not contain password_hash.
- Trusted CLI created the first Admin and refused another bootstrap.
- Auth migration up/down/up was verified on populated disposable test data.
- API/worker Docker images built; container API login, current Admin, Worker list
  and logout/revoked-token HTTP 401 were verified. Graceful shutdown still works.

## Commands used

```powershell
go get github.com/google/uuid@v1.6.0 golang.org/x/crypto@v0.43.0
go mod tidy
gofmt -w cmd internal
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
docker compose -p <temporary-auth-project> up -d --wait postgres
./tasks.ps1 migrate-up
go test -count=1 -v ./internal/repository/postgres
go run ./cmd/api -bootstrap-admin
./tasks.ps1 migrate-down
./tasks.ps1 migrate-version
./tasks.ps1 migrate-up
docker compose -p <temporary-auth-project> up --build -d api worker
docker compose -p <temporary-auth-project> stop -t 15 api worker
docker compose -p <temporary-auth-project> down --volumes
```

HTTP verification used Invoke-RestMethod/Invoke-WebRequest against /health/ready,
/auth/login, /auth/me, /admin/workers and /auth/logout. Tokens and passwords were
kept in process variables and not printed. The successful public responses were
checked without exposing credentials in the verification transcript.

TEST_DATABASE_URL targeted only sa_lathe_shop_phase3_test. A separate
sa_lathe_shop_bootstrap_test database tested the CLI. All passwords were generated
randomly into process environment; no production/user accounts were seeded and
no .env was created or changed. Only the two temporary Compose projects' newly
created containers, networks and volumes were removed. Test records are discarded.

The original README.md remains unchanged. Only auth-phase files and maintained
project docs/config were changed. Local commits follow 3-8 related files and build
before every commit. Push remains pending the earlier destination confirmation;
no GitHub write occurred during this phase. No GitHub CLI was available to resolve
remote ownership through read-only account checks.
