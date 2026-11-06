# SQL console (Super Admin)

Arbitrary SQL against the application's PostgreSQL database, for the Super Admin only.
**Off by default**: set `SQL_CONSOLE_ENABLE=true`. When off, `/admin/sql/*` returns 404.

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/admin/sql/schema` | Tables/views in `public`: columns, PK/UQ/FK/CHECK constraints (with referenced table), indexes |
| POST | `/admin/sql/execute` | Body `{ "sql": "...", "confirm": false }` |

## Rules
- Bearer session required; account must be active, `admin` role **and** `is_super_admin` (403 otherwise).
- One statement per request (the extended protocol rejects more; a pre-check also reports 400 `statement_rejected`). Max 64 KB.
- Runs in a transaction with `statement_timeout` 10 s; any error rolls back. Results are PostgreSQL text output, capped at 1,000 rows (`truncated: true`).
- DROP, TRUNCATE, and UPDATE/DELETE with no `WHERE` keyword return 409 `confirmation_required` until resent with `confirm: true`. Statements starting with BEGIN/COMMIT/ROLLBACK/SAVEPOINT/SET/RESET/COPY are refused (400).
- Database errors return 422 `sql_error` with `message`, `sqlstate`, `position`, `detail`, `hint`.
- Each run logs account id, verb, destructive flag and outcome (never the SQL text).
- SQL is user input by design, so there is no parameter binding; protection is the role gate, the confirmation, and the DB role.

## Database role
`SQL_CONSOLE_DATABASE_URL` (optional) opens a separate pool, so the console can run as a least-privilege role. Without it the console uses the API's own connection, i.e. the owner role, and can alter any table including `account` and sessions.

## Known limits
Confirmation is a keyword heuristic (`WHERE true` passes; `DO` blocks and functions can still change data). `set_config()` can alter the timeout inside a statement. Only the `public` schema is shown.

## Tests
`go test ./internal/sqlconsole/` (guard + permission/confirmation handler tests). Real DB: create an empty database `sqlconsole_test`, then `SQLCONSOLE_DATABASE_URL=postgres://…/sqlconsole_test go test ./internal/repository/postgres -run TestSQLConsoleIntegration`.
