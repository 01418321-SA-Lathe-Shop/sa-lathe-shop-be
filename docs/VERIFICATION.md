# Scaffolding verification

Date: 2026-10-11 Asia/Bangkok. Host: Windows/PowerShell; Docker Desktop Linux.

## Results

- go build ./...: passed, including before each local commit.
- go vet ./...: passed.
- go test ./... and ./tasks.ps1 lint: passed.
- API and worker Docker images built and both ran in Compose.
- GET /health: HTTP 200, {"status":"ok"}.
- GET /health/ready with PostgreSQL: HTTP 200, database ok.
- PostgreSQL stopped: readiness 503, liveness still 200.
- S3 anonymous list request: HTTP 403 (private credentials required).
- Empty initial migration: up to version 1, down, up again passed.
- SIGTERM: API logged graceful shutdown; worker logged stopped.
- Original README.md unchanged; no previous files removed.

## Commands executed

Repository inventory used Get-ChildItem, git status, git remote -v and git log.
Existing repository already initialized; no git init was necessary. Module
bootstrap was generated using go mod init github.com/Yoshi4489/SA-Lathe-Shop-BE
and go mod edit -go 1.25.0, then copied as a new file to the target folder.

```powershell
go get github.com/go-chi/chi/v5 github.com/go-chi/cors
go get github.com/jackc/pgx/v5/pgxpool
go mod tidy
gofmt -w cmd internal
go build ./...
go vet ./...
go test ./...
./tasks.ps1 lint
docker build -f Dockerfile.api -t sa-lathe-shop-be-api:scaffold .
docker build -f Dockerfile.worker -t sa-lathe-shop-be-worker:scaffold .
docker compose -p sa-lathe-shop-be-check-20261011 --profile app up --build -d
Invoke-RestMethod http://localhost:18080/health
Invoke-RestMethod http://localhost:18080/health/ready
Invoke-WebRequest http://localhost:19010/ -SkipHttpErrorCheck
./tasks.ps1 migrate-up
./tasks.ps1 migrate-version
./tasks.ps1 migrate-down
./tasks.ps1 migrate-up
docker compose -p sa-lathe-shop-be-check-20261011 stop postgres
Invoke-WebRequest http://localhost:18080/health/ready -SkipHttpErrorCheck
Invoke-RestMethod http://localhost:18080/health
docker compose -p sa-lathe-shop-be-check-20261011 start postgres
docker compose -p sa-lathe-shop-be-check-20261011 ps
docker compose -p sa-lathe-shop-be-check-20261011 stop -t 15 api worker
docker compose -p sa-lathe-shop-be-check-20261011 logs --tail 6 api worker
docker compose -p sa-lathe-shop-be-check-20261011 down --volumes
git status --short
git log --oneline
```

Credentials for verification were randomly generated process variables, never
printed or saved. Alternate test ports: API 18080, PostgreSQL 55439, S3 19010.
Cleanup removed only this temporary project's containers and two newly-created
test volumes; no existing volumes/files were targeted. Test data is discarded.

MinIO Docker Hub/Quay pulls failed; Docker Hub blob downloads also timed out.
Successful alternatives: official SeaweedFS GHCR image (pinned digest) and
Go/Alpine public ECR official-image mirrors. PostgreSQL uses postgres:17-alpine.

Git: logical 3-8-file local commits, each preceded by a successful build.
Remote exists at git@github.com:Yoshi4489/SA-Lathe-Shop-BE.git on main.
Automatic approval review rejected pushing without destination-specific human
confirmation. Push is pending; no remote writes occurred. Once confirmed, push
phase 0 then phase 1 then phase 2 commit tips separately, checking status/log
before each push rather than sending all phases in one push.

Original UC Word path is missing; available ER schema was read fully. Earlier
fully-read UC context and owner's final stack were recorded in requirements.md.
Obtain current missing requirements/sequence originals before feature work.
