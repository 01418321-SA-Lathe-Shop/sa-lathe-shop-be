# ER-aligned demo dataset handoff

This dataset is for a dedicated local or test database only. Never run the reset or seed commands against production. The supplied ER document is a comparison target; migrations and UC rules remain the executable contract.

## Schema result

Migrations `000023`–`000026` expose all 27 ER table names and all field names listed in `er_diagram_table_structure.md`. Migration `000026` supplies the two names that were previously missing on `job`: `estimated_time` mirrors `due_at`, and `actual_time` mirrors `completed_at` while the job is complete. Some fields are synchronized compatibility columns because the application keeps more detailed operational tables and immutable audit history. Run `scripts/check-er-schema.sql`; zero result rows means no required table or field name is missing.

## Demo coverage

The `workflows-v3` fixture creates six role accounts, realistic Thai customer/shop data, 14 manufacturing requests, 13 materials and conversions, six suppliers and purchase orders, receiving damage/claims, production usage/time, deliveries, invoices and customer payments. The request scenarios cover request-only, draft quotation, rejected/revised quotation, accepted waiting for evidence, pending deposit, queued shortage, queued ready, in production, production complete, shipped with a pending bank payment, fully settled/accepted, rejected bank payment, returned delivery/rework, and a shop-rejected request.

Supplier orders cover pending, accepted/received, cancelled after supplier rejection, partially received and fully damaged. Defect records cover pending claim and submitted claim. The generated private PDFs are fictional Thai specification, PO, deposit and payment-evidence documents; none represents a real transaction.

Statuses that are immutable history rather than useful current cards remain visible in their audit records, such as rejected quotation decisions and evidence reviews. Unsupported ER-only states such as `preparing`/`in_transit` order items and `WRITTEN_OFF` defects are not fabricated because the confirmed UC rules provide no transition that can create them. Run `scripts/demo-status-matrix.sql` to list every current status and record count in a seeded database.

## Fresh database procedure

1. Back up the existing local test database.
2. Stop API and worker processes that use it.
3. Drop and recreate only `sa_lathe_shop_demo` or `sa_lathe_shop_seed_test`.
4. Set private database/S3 values in `.env`, set `DB_NAME` to the dedicated demo database, then migrate through `000026`:

```powershell
./tasks.ps1 migrate-up
```

5. Confirm the ER names. The command must print no rows:

```powershell
psql $env:SEED_DATABASE_URL -f scripts/check-er-schema.sql
```

6. Enable the guarded account seed and set six private demo passwords in the environment:

```powershell
$env:SEED_ENABLED='true'
$env:SEED_ENV='local'
$env:SEED_CONFIRM_DATABASE='sa_lathe_shop_demo'
$env:SEED_DATABASE_URL='postgres://USER:PASSWORD@localhost:55432/sa_lathe_shop_demo?sslmode=disable'
./scripts/seed-demo.ps1 -Action apply
```

7. Start the local API and worker with password-free local demo login enabled:

```powershell
$env:DB_NAME='sa_lathe_shop_demo'
$env:LOCAL_TEST_ACCOUNT_ENABLE='true'
docker compose --profile app up --build -d --force-recreate api worker
```

8. Seed workflows through real APIs and private object storage:

```powershell
$env:SEED_WORKFLOWS_ENABLE='true'
$env:SEED_WORKFLOW_API_ORIGIN='http://localhost:8080'
$env:SEED_WORKFLOW_S3_ORIGIN='http://localhost:19000'
go run ./cmd/seed-demo-workflows apply
go run ./cmd/seed-demo-workflows verify
psql $env:SEED_DATABASE_URL -f scripts/demo-status-matrix.sql
```

`apply` is resumable and checkpointed. A completed rerun is read-only and preserves tester changes. To start over, recreate the dedicated demo database and its private S3 test volume; do not manually delete selected workflow rows because audit and file ownership tables are intentionally linked.

### Bash / zsh equivalent

The following assumes the URL components are already URL-safe and the API/S3 local containers are available:

```bash
export SEED_ENABLED=true
export SEED_ENV=local
export SEED_CONFIRM_DATABASE=sa_lathe_shop_demo
export SEED_DATABASE_URL='postgres://USER:PASSWORD@localhost:55432/sa_lathe_shop_demo?sslmode=disable'
export SEED_PIM_PASSWORD='set-a-private-12-plus-character-password'
export SEED_ANAN_PASSWORD='set-a-private-12-plus-character-password'
export SEED_KITTIPONG_PASSWORD='set-a-private-12-plus-character-password'
export SEED_NIDA_PASSWORD='set-a-private-12-plus-character-password'
export SEED_PLOY_PASSWORD='set-a-private-12-plus-character-password'
export SEED_SOMCHAI_PASSWORD='set-a-private-12-plus-character-password'

go run ./cmd/seed-demo apply --profile accounts

export SEED_WORKFLOWS_ENABLE=true
export SEED_WORKFLOW_API_ORIGIN='http://localhost:8080'
export SEED_WORKFLOW_S3_ORIGIN='http://localhost:19000'
go run ./cmd/seed-demo-workflows apply
go run ./cmd/seed-demo-workflows verify
psql "$SEED_DATABASE_URL" -f scripts/demo-status-matrix.sql
```

Clear the six password variables from the shell after use. The account seed does not read passwords from `.env`, and committed files contain no password.

## Demo accounts

The guarded manifest contains `pim.admin`, `anan.staff`, `kittipong.staff`, `nida.company`, `ploy.company`, and `somchai`. Passwords come only from `SEED_*_PASSWORD` process variables or the secure prompts in `scripts/seed-demo.ps1`; they are never committed. With `LOCAL_TEST_ACCOUNT_ENABLE=true`, the frontend account picker can log into these six accounts without exposing their passwords.

## Verification evidence

Run `go run ./cmd/seed-demo-workflows verify` after seeding. It checks customer isolation, private PDF downloads, worker finance redaction, queued stock readiness, receiving/claim state, supplier credit terms, pending/rejected/settled customer payments, returned delivery/rework, cancelled/partial/damaged supplier orders, and paid/accepted request closure.
