# Staff/Admin production overview — read-only contract

Owner-approved 2026-10-28. Shared shop queue, no job assignment or new commands.
Both endpoints require a current active Worker/Admin Bearer session. Customer
access is forbidden. No financial or private-file data is returned.

## Reads

- `GET /dashboard/overview?range=30d`
- `GET /dashboard/jobs?range=30d&status=queued&limit=20&offset=0`
- `GET /dashboard/jobs?first_completed_on=2026-10-28&limit=20&offset=0`

`range`: today / 7d / 30d (default) / year. Days use Asia/Bangkok; today,
7/30 inclusive calendar days, year starts January 1. The creation cohort uses
`job.created_at` through response `as_of`; counts are **current** job states,
not historical status snapshots. Four mutually exclusive counts are queued,
in_progress, completed and cancelled. Their sum is total. Request cancellation
is not job cancellation. Queue counts are current all-date backlog, separately
labelled: pending requests, queued jobs, accepted unreceived supplier orders.

Heatmap is independent of range: rolling 12 calendar months through today,
starting the day after the clamped prior-year date. Find MIN(created_at) of
production_event action=complete **over all prior history before filtering**.
One job contributes once, even after free rework. Use date in Asia/Bangkok.
`job.completed_at` is current-cycle only and must not drive historical counts.

Overview returns `as_of`, `range`, `period {from,to}`, `counts`, `queue`, and
`heatmap {start_date,end_date,days:[{date,count}],history_complete,
missing_history_jobs,summary:{total,best_day,best_day_count,current_streak,
average_per_week}}`. Zero days are filled within the reporting window only.
Best-day ties use latest date; no completions means best_day=null. Streak ends
today, or yesterday if today is empty. Weekly average is total*7/day_count,
one-place decimal string. Incomplete audit history suppresses streak/average
(null), not unknown-as-zero. Summary total counts only known first completions.

Jobs returns `{as_of,items,total,limit,offset}`. Each item has job_id, request_id,
request_number, description, status, created_at, nullable started_at,
completed_at and first_completed_at. Normal mode filters creation cohort and
optional status. Date mode filters first completion and forbids range/status;
date must be within the heatmap window. Newest creation then UUID descending.
Pagination defaults 20, maximum 100; total is not the page length. Counts and
request_number are JSON int64 numbers (clients must preserve unsafe integers).

No new tables, migrations, events, assignments, production actions or changes
to existing UC endpoints. Reporting SQL uses one read-only repeatable-read
transaction per response with a current active-role check; rows and totals
share that snapshot. Lists fetch only the requested page, never per-job reads.

## Implementation and verification

Both routes are registered in cmd/api using the existing Bearer middleware.
No existing route, command, migration or business policy changed.

`go test ./internal/dashboard` covers Bangkok midnight, year/leap boundaries,
filter exclusion, role gates, tie-breaking, streak, empty and incomplete history.
`TestDashboardIntegration` runs only with a dedicated empty database named
sa_lathe_dashboard_test. It creates a minimal reporting fixture, not a shop
schema migration: 125 jobs across all four states, full filtered totals/pages,
multiple completion events/rework, an earlier completion outside the period,
missing history, financial-field exclusion and current active-role checks.
The fresh PostgreSQL 18.3 container run passed on 2026-10-28 and was removed.
This verifies reporting SQL, not a new full UC1–UC9/S3 workflow run or an SLA.

Run the SQL test with a private process-only `DASHBOARD_DATABASE_URL` pointed
at that disposable empty database; it refuses a different name or existing
public tables. Never point it at an existing shop database. Without that
explicit variable, the integration test is skipped by the default test suite.
