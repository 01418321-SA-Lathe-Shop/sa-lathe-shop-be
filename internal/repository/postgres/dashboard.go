package postgres

import (
	"context"

	d "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/dashboard"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/failure"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
)

var _ d.Repository = (*Database)(nil)

func (db *Database) dashboardRead(ctx context.Context, actor string) (pgx.Tx, error) {
	tx, e := db.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, failure.Wrap("dashboard.begin", e)
	}
	var allowed bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account WHERE id=$1 AND is_active AND role IN ('worker','admin'))`, actor).Scan(&allowed)
	if e != nil || !allowed {
		tx.Rollback(ctx)
		if e != nil {
			return nil, failure.Wrap("dashboard.actor", e)
		}
		return nil, req.ErrForbidden
	}
	return tx, nil
}

// Select the first event over all history, not the first event inside a window.
const dashboardFirstCompletion = `WITH first_completion AS (
 SELECT job_id,MIN(created_at) AS first_completed_at FROM production_event
 WHERE action='complete' AND created_at<=$1 GROUP BY job_id
) `

func (db *Database) DashboardOverview(ctx context.Context, actor string, w d.Window) (d.Overview, error) {
	v := d.Overview{}
	tx, e := db.dashboardRead(ctx, actor)
	if e != nil {
		return v, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='queued'),count(*) FILTER(WHERE status='in_progress'),
 count(*) FILTER(WHERE status='completed'),count(*) FILTER(WHERE status='cancelled'),count(*)
 FROM job WHERE created_at>=$1 AND created_at<=$2`, w.From, w.AsOf).
		Scan(&v.Counts.Queued, &v.Counts.InProgress, &v.Counts.Completed, &v.Counts.Cancelled, &v.Counts.Total)
	if e != nil {
		return v, failure.Wrap("dashboard.counts", e)
	}
	e = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM job_request WHERE status='pending'),
 (SELECT count(*) FROM job WHERE status='queued'),
 (SELECT count(*) FROM material_order WHERE supplier_response='accepted' AND status NOT IN ('received','cancelled'))`).
		Scan(&v.Queue.PendingRequests, &v.Queue.QueuedJobs, &v.Queue.ReceivingOrders)
	if e != nil {
		return v, failure.Wrap("dashboard.queue", e)
	}
	rows, e := tx.Query(ctx, dashboardFirstCompletion+`SELECT to_char(first_completed_at AT TIME ZONE 'Asia/Bangkok','YYYY-MM-DD'),count(*)
 FROM first_completion WHERE first_completed_at>=$2 GROUP BY 1 ORDER BY 1`, w.AsOf, w.HeatmapFrom)
	if e != nil {
		return v, failure.Wrap("dashboard.activity", e)
	}
	for rows.Next() {
		var day d.Day
		if e = rows.Scan(&day.Date, &day.Count); e != nil {
			rows.Close()
			return v, failure.Wrap("dashboard.day", e)
		}
		v.Heatmap.Days = append(v.Heatmap.Days, day)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, failure.Wrap("dashboard.days", e)
	}
	e = tx.QueryRow(ctx, dashboardFirstCompletion+`SELECT count(*) FROM job j LEFT JOIN first_completion f ON f.job_id=j.id
 WHERE f.job_id IS NULL AND (j.status='completed' OR j.completed_at IS NOT NULL OR
 EXISTS(SELECT 1 FROM production_return p WHERE p.job_id=j.id AND p.prior_completed_at IS NOT NULL))
 AND j.created_at<=$1`, w.AsOf).Scan(&v.Heatmap.MissingHistoryJobs)
	if e != nil {
		return v, failure.Wrap("dashboard.coverage", e)
	}
	return v, tx.Commit(ctx)
}

const dashboardJobFilter = ` FROM job j JOIN job_request r ON r.id=j.request_id
 LEFT JOIN first_completion f ON f.job_id=j.id
 WHERE j.created_at<=$1 AND (($3::text='' AND j.created_at>=$2 AND ($4::text='' OR j.status=$4))
 OR ($3::text<>'' AND to_char(f.first_completed_at AT TIME ZONE 'Asia/Bangkok','YYYY-MM-DD')=$3))`

func (db *Database) DashboardJobs(ctx context.Context, actor string, f d.Filter) (d.Jobs, error) {
	v := d.Jobs{Items: []d.Job{}}
	tx, e := db.dashboardRead(ctx, actor)
	if e != nil {
		return v, e
	}
	defer tx.Rollback(ctx)
	args := []any{f.AsOf, f.From, f.FirstCompletedOn, f.Status}
	e = tx.QueryRow(ctx, dashboardFirstCompletion+`SELECT count(*)`+dashboardJobFilter, args...).Scan(&v.Total)
	if e != nil {
		return v, failure.Wrap("dashboard.jobs_total", e)
	}
	rows, e := tx.Query(ctx, dashboardFirstCompletion+`SELECT j.id::text,j.request_id::text,r.no_request,r.description,j.status,
 j.created_at,j.started_at,j.completed_at,f.first_completed_at`+dashboardJobFilter+` ORDER BY j.created_at DESC,j.id DESC LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if e != nil {
		return v, failure.Wrap("dashboard.jobs", e)
	}
	for rows.Next() {
		var job d.Job
		if e = rows.Scan(&job.JobID, &job.RequestID, &job.RequestNumber, &job.Description, &job.Status, &job.CreatedAt, &job.StartedAt, &job.CompletedAt, &job.FirstCompletedAt); e != nil {
			rows.Close()
			return v, failure.Wrap("dashboard.job", e)
		}
		v.Items = append(v.Items, job)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, failure.Wrap("dashboard.job_rows", e)
	}
	return v, tx.Commit(ctx)
}
