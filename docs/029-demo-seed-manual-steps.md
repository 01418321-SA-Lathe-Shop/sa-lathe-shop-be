# Manual demo seed steps (macOS / zsh)

These commands recreate a dedicated local demo database and install the complete `workflows-v3` dataset. Run them from a terminal. They delete the selected demo database and its project-local S3 files after creating backups; never use them against production or shared environments.

## 1. Open the backend repository and update `develop`

```bash
cd '/path/to/sa-lathe-shop-be'
git switch develop
git pull --ff-only origin develop
git status --short
```

Stop if `git status --short` shows changes you do not recognize.

## 2. Prepare `.env`

Create `.env` only if it does not already exist:

```bash
test -f .env || cp .env.example .env
```

Open `.env` and set at least the local PostgreSQL and S3 secrets required by `docker-compose.yml`. Do not commit `.env`.

Load it into the current shell and choose the dedicated database:

```bash
set -a
source .env
set +a

export DB_USER="${DB_USER:-sa_lathe_shop}"
export DB_PORT="${DB_PORT:-55432}"
export DB_NAME='sa_lathe_shop_demo'
export TARGET_DB="$DB_NAME"
export LOCAL_TEST_ACCOUNT_ENABLE='true'
```

The safety check below must print `sa_lathe_shop_demo`:

```bash
printf '%s\n' "$TARGET_DB"
test "$TARGET_DB" = 'sa_lathe_shop_demo'
```

If you intentionally use the other guarded local database, replace both assignments with `sa_lathe_shop_seed_test`.

## 3. Start local PostgreSQL and S3

```bash
docker compose up -d --wait postgres s3
docker compose ps
```

## 4. Back up the existing local demo data

```bash
export BACKUP_DIR="$(mktemp -d /tmp/sa-lathe-backup.XXXXXX)"

if docker compose exec -T postgres \
  psql -U "$DB_USER" -d postgres -tAc \
  "SELECT 1 FROM pg_database WHERE datname='$TARGET_DB'" | grep -q 1; then
  docker compose exec -T postgres \
    pg_dump -Fc -U "$DB_USER" -d "$TARGET_DB" \
    > "$BACKUP_DIR/$TARGET_DB.dump"
fi

docker compose exec -T s3 tar -czf - -C /data . \
  > "$BACKUP_DIR/s3-data.tgz"

ls -lh "$BACKUP_DIR"
```

Keep the printed `/tmp/sa-lathe-backup.*` path until the new dataset is verified. The dump and archive can contain private local test data and must never be committed.

## 5. Stop application processes and clear only the project S3 volume

Resolve and validate the exact Compose-owned S3 volume before deletion:

```bash
docker compose --profile app stop api worker

export S3_CONTAINER="$(docker compose ps -q s3)"
export S3_IMAGE="$(docker inspect "$S3_CONTAINER" --format '{{.Config.Image}}')"
export S3_VOLUME="$(docker inspect "$S3_CONTAINER" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}')"

printf 'S3 container: %s\nS3 volume: %s\n' "$S3_CONTAINER" "$S3_VOLUME"
case "$S3_VOLUME" in
  *_s3-data) ;;
  *) echo 'Refusing to clear an unexpected volume' >&2; exit 1 ;;
esac

docker compose stop s3
docker run --rm --entrypoint sh \
  -v "$S3_VOLUME:/data" "$S3_IMAGE" \
  -c 'find /data -mindepth 1 -delete'
docker compose up -d --wait s3
```

## 6. Recreate only the guarded demo database

```bash
test "$TARGET_DB" = 'sa_lathe_shop_demo' || \
  test "$TARGET_DB" = 'sa_lathe_shop_seed_test'

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 \
  -U "$DB_USER" -d postgres \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$TARGET_DB' AND pid<>pg_backend_pid()" \
  -c "DROP DATABASE IF EXISTS \"$TARGET_DB\"" \
  -c "CREATE DATABASE \"$TARGET_DB\" OWNER \"$DB_USER\""
```

## 7. Build the guarded database URL and apply migrations

The password is URL-encoded without printing it:

```bash
export ENCODED_DB_PASSWORD="$(python3 -c 'import os, urllib.parse; print(urllib.parse.quote(os.environ["DB_PASSWORD"], safe=""))')"
export SEED_DATABASE_URL="postgres://${DB_USER}:${ENCODED_DB_PASSWORD}@localhost:${DB_PORT}/${TARGET_DB}?sslmode=disable"

go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 \
  -path migrations -database "$SEED_DATABASE_URL" up
```

Confirm migration 26 and verify every ER table/field name:

```bash
psql "$SEED_DATABASE_URL" -Atc 'SELECT version,dirty FROM schema_migrations'
psql "$SEED_DATABASE_URL" -f scripts/check-er-schema.sql
```

Expected migration output is `26|f`. The schema checker must return zero rows.

## 8. Set guarded seed variables and private demo passwords

Replace every placeholder with a private password between 12 and 128 UTF-8 bytes. These passwords remain in the current shell only.

```bash
export SEED_ENABLED='true'
export SEED_ENV='local'
export SEED_CONFIRM_DATABASE="$TARGET_DB"

export SEED_PIM_PASSWORD='replace-with-private-password-1'
export SEED_ANAN_PASSWORD='replace-with-private-password-2'
export SEED_KITTIPONG_PASSWORD='replace-with-private-password-3'
export SEED_NIDA_PASSWORD='replace-with-private-password-4'
export SEED_PLOY_PASSWORD='replace-with-private-password-5'
export SEED_SOMCHAI_PASSWORD='replace-with-private-password-6'
```

## 9. Seed the six accounts

```bash
go run ./cmd/seed-demo apply --profile accounts
```

Expected result: `state` is `ready`, `accounts` is `6`, and `owned_records` is `12`.

## 10. Start API and worker on the demo database

`DB_NAME` is already set to the guarded target in this shell:

```bash
docker compose --profile app up -d --build --force-recreate api worker

for attempt in {1..60}; do
  curl -fsS http://localhost:8080/health/ready && break
  sleep 1
done

curl -fsS http://localhost:8080/health/ready
```

Expected response contains `"status":"ok"` and `"database":"ok"`.

## 11. Seed all realistic workflows and files

```bash
export SEED_WORKFLOWS_ENABLE='true'
export SEED_WORKFLOW_API_ORIGIN='http://localhost:8080'
export SEED_WORKFLOW_S3_ORIGIN='http://localhost:19000'

go run ./cmd/seed-demo-workflows plan
go run ./cmd/seed-demo-workflows apply
```

Expected final result: `steps` is `235`.

## 12. Run read-only verification

Restarting the local API clears only the in-memory local-test login limiter; it does not delete data.

```bash
docker compose restart api

for attempt in {1..60}; do
  curl -fsS http://localhost:8080/health/ready >/dev/null && break
  sleep 1
done

go run ./cmd/seed-demo-workflows verify
psql "$SEED_DATABASE_URL" -f scripts/check-er-schema.sql
psql "$SEED_DATABASE_URL" -f scripts/demo-status-matrix.sql
```

Confirm the fixture and summary counts:

```bash
psql "$SEED_DATABASE_URL" -At -F '|' <<'SQL'
SELECT fixture,status,count(*)
FROM demo_workflow_run
GROUP BY fixture,status;

SELECT
  (SELECT count(*) FROM job_request),
  (SELECT count(*) FROM job),
  (SELECT count(*) FROM material_order),
  (SELECT count(*) FROM invoice),
  (SELECT count(*) FROM payment);
SQL
```

Expected output includes:

```text
workflows-v3|complete|1
14|8|6|4|3
```

## 13. Clear seed secrets from the shell

```bash
unset ENCODED_DB_PASSWORD SEED_DATABASE_URL
unset SEED_ENABLED SEED_ENV SEED_CONFIRM_DATABASE SEED_WORKFLOWS_ENABLE
unset SEED_WORKFLOW_API_ORIGIN SEED_WORKFLOW_S3_ORIGIN
unset SEED_PIM_PASSWORD SEED_ANAN_PASSWORD SEED_KITTIPONG_PASSWORD
unset SEED_NIDA_PASSWORD SEED_PLOY_PASSWORD SEED_SOMCHAI_PASSWORD
```

The application remains running with the completed local demo dataset. Use the local-test account picker in the frontend while `LOCAL_TEST_ACCOUNT_ENABLE=true`.
