# AI handoff: install the ER-complete demo dataset

Copy this entire document into an AI coding-agent chat opened at the backend repository. The agent must inspect the repository and adapt commands to the local operating system; it must not blindly run production-destructive commands.

## Task for the AI agent

Prepare the SA Lathe Shop application with the transferable `workflows-v3` demo dataset.

Repository expectations:

- Backend repository: `sa-lathe-shop-be`, branch `develop`.
- Frontend repository: `sa-lathe-shop-fe`, branch `develop`.
- Required backend commit or later: `d9a41a1`.
- The authoritative setup guide is `docs/027-demo-dataset-handoff.md`.
- The database must be migrated through `000026_er_job_times`.
- Use only a dedicated local/test database named `sa_lathe_shop_demo` or `sa_lathe_shop_seed_test`.

### Safety requirements

1. Never target production, staging, a remote database, or real customer data.
2. Before deleting anything, confirm that the database host is loopback/local Docker and that the exact database name is one of the two allowed names above.
3. Back up the existing local/test PostgreSQL database and private S3 test volume before resetting them. Report the backup paths.
4. Do not commit passwords, access keys, `.env` contents, database dumps, signed URLs, or generated login tokens.
5. Do not insert business records or statuses directly with SQL. Migrations may change schema, account seeding may write its owned identity rows, and workflow data must be created through the existing API fixture runner.
6. Do not fabricate unsupported ER-only states. In particular, do not manually create `preparing`/`in_transit` material-order items or `WRITTEN_OFF` defects when no application transition supports them.
7. Preserve unrelated local changes. Stop and ask the owner if the target is ambiguous or appears non-local.

### Required procedure

1. Confirm both repositories are on `develop`, inspect their working trees, and pull with fast-forward only. Do not overwrite uncommitted work.
2. Read:
   - `docs/027-demo-dataset-handoff.md`
   - `docs/017-demo-workflows.md`
   - `scripts/check-er-schema.sql`
   - `scripts/demo-status-matrix.sql`
3. Confirm the backend contains migration `000026_er_job_times` and fixture name `workflows-v3`.
4. Start local PostgreSQL and private S3 services using the repository's Docker Compose configuration.
5. Back up the existing dedicated local/test database and S3 volume.
6. Stop the API and worker before resetting the database.
7. Recreate only the confirmed dedicated demo/test database and its test S3 data. Apply migrations 1–26 using the repository migration command.
8. Run `scripts/check-er-schema.sql`. Continue only if it returns zero missing rows.
9. Seed the six demo accounts with private 12–128 byte passwords supplied through process environment variables:

```text
SEED_PIM_PASSWORD
SEED_ANAN_PASSWORD
SEED_KITTIPONG_PASSWORD
SEED_NIDA_PASSWORD
SEED_PLOY_PASSWORD
SEED_SOMCHAI_PASSWORD
```

10. Start the API and worker with local-test login enabled. Wait for `GET /health/ready` to report that the database is ready.
11. Set the guarded seed variables for the exact local database:

```bash
export SEED_ENABLED=true
export SEED_ENV=local
export SEED_CONFIRM_DATABASE=sa_lathe_shop_demo
export SEED_DATABASE_URL='postgres://USER:URL_ENCODED_PASSWORD@localhost:55432/sa_lathe_shop_demo?sslmode=disable'
export SEED_WORKFLOWS_ENABLE=true
export SEED_WORKFLOW_API_ORIGIN='http://localhost:8080'
export SEED_WORKFLOW_S3_ORIGIN='http://localhost:19000'
```

Use `sa_lathe_shop_seed_test` instead if that is the confirmed local target. The value of `SEED_CONFIRM_DATABASE` must exactly match the database in `SEED_DATABASE_URL`.

12. Seed accounts, then workflows:

```bash
go run ./cmd/seed-demo apply --profile accounts
go run ./cmd/seed-demo-workflows plan
go run ./cmd/seed-demo-workflows apply
```

PowerShell users may use `./scripts/seed-demo.ps1 -Action apply` for accounts after setting the guarded variables. Never place the passwords in a committed file.

13. Restart the local API once before the standalone verification so the normal local-test login limiter has a fresh process. Then run:

```bash
go run ./cmd/seed-demo-workflows verify
psql "$SEED_DATABASE_URL" -f scripts/check-er-schema.sql
psql "$SEED_DATABASE_URL" -f scripts/demo-status-matrix.sql
```

14. Run the backend tests and build:

```bash
go test ./...
go build ./...
```

15. Clear all six password variables and other seed-only secrets from the process environment when finished.

### Expected result

The completed database must contain exactly one completed `workflows-v3` run with 235 checkpointed operations. A fresh dataset contains:

- 6 role accounts: Admin, two Workers, two company members, and one individual customer.
- 14 realistic manufacturing requests and 8 jobs.
- 13 materials with realistic Thai names, stock, costs, units, and conversions.
- 6 suppliers/material orders.
- 4 invoices and 3 customer payment records.
- 26 private fictional PDF documents.
- Current states covering request pending/ongoing/completed/canceled; quotation draft/sent/rejected/approved; job queued/in-progress/completed; delivery shipping/delivered/returned; payment pending/rejected/approved; supplier order ordered/cancelled/partially received/received/damaged; and defect pending-claim/claimed.

The following SQL summary should return `14|8|6|4|3`:

```sql
SELECT
  (SELECT count(*) FROM job_request),
  (SELECT count(*) FROM job),
  (SELECT count(*) FROM material_order),
  (SELECT count(*) FROM invoice),
  (SELECT count(*) FROM payment);
```

`scripts/check-er-schema.sql` must return zero rows. `go run ./cmd/seed-demo-workflows verify` must exit successfully and report 235 steps.

### Final report required from the AI agent

Report only claims backed by executed checks. Include:

- Exact database name and confirmation that it is local/test.
- Backup locations.
- Migration version.
- ER schema check result.
- Fixture name, completion status, and step count.
- Counts for requests, jobs, material orders, invoices, payments, and private PDFs.
- Status-matrix output or a concise summary.
- Test/build results and API readiness.
- Anything that could not be executed and why.
- Git commit/push status. Do not push unless the owner explicitly authorizes it.
