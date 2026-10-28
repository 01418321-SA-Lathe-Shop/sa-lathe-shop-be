package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	d "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/dashboard"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
)

// Dedicated disposable database only. This fixture tests reporting SQL, not UC
// mutations or S3; it intentionally does not alter any existing shop database.
func TestDashboardIntegration(t *testing.T) {
	dsn := os.Getenv("DASHBOARD_DATABASE_URL")
	if dsn == "" {
		t.Skip("set dedicated DASHBOARD_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/sa_lathe_dashboard_test" {
		t.Fatal("dedicated fresh reporting database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, e := Open(ctx, dsn, 4)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var tables int
	if e = db.pool.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public'`).Scan(&tables); e != nil || tables != 0 {
		t.Fatal("fresh database required; existing data unchanged", e)
	}
	_, e = db.pool.Exec(ctx, `CREATE TABLE account(id uuid PRIMARY KEY,role text,is_active boolean);
 CREATE TABLE job_request(id uuid PRIMARY KEY,no_request bigint,description text,status text);
 CREATE TABLE job(id uuid PRIMARY KEY,request_id uuid,status text,created_at timestamptz,started_at timestamptz,completed_at timestamptz);
 CREATE TABLE production_event(job_id uuid,action text,created_at timestamptz);
 CREATE TABLE production_return(job_id uuid,prior_completed_at timestamptz);
 CREATE TABLE material_order(status text,supplier_response text);`)
	if e != nil {
		t.Fatal(e)
	}
	worker, admin, customer := uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, e = db.pool.Exec(ctx, `INSERT INTO account VALUES($1,'worker',true),($2,'admin',true),($3,'customer',true)`, worker, admin, customer)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	w, _ := d.NewWindow("30d", now)
	var first string
	for i := 0; i < 125; i++ {
		id, request := uuid.NewString(), uuid.NewString()
		status := []string{"queued", "in_progress", "completed", "cancelled"}[i%4]
		_, e = db.pool.Exec(ctx, `INSERT INTO job_request VALUES($1,$2,$3,'ongoing')`, request, i+1, "งานกลึงทดสอบ")
		if e == nil {
			_, e = db.pool.Exec(ctx, `INSERT INTO job VALUES($1,$2,$3,$4,NULL,NULL)`, id, request, status, now.Add(-time.Hour))
		}
		if e != nil {
			t.Fatal(e)
		}
		if i == 1 {
			first = id // currently in progress after rework; two completion events
			_, e = db.pool.Exec(ctx, `INSERT INTO production_event VALUES($1,'complete',$2),($1,'complete',$3)`, id, now.Add(-time.Hour), now.Add(-time.Minute))
			if e != nil {
				t.Fatal(e)
			}
		}
		if status == "completed" {
			_, e = db.pool.Exec(ctx, `INSERT INTO production_event VALUES($1,'complete',$2)`, id, now.Add(-time.Hour))
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	service := d.NewService(db)
	a := auth.Account{ID: worker, Role: auth.RoleWorker, Active: true}
	v, e := service.Overview(ctx, a, "30d")
	if e != nil || v.Counts.Total != 125 || v.Counts.Cancelled != 31 || v.Counts.Queued != 32 || v.Heatmap.Summary.Total != 32 || !v.Heatmap.HistoryComplete {
		t.Fatalf("overview: %+v %v", v, e)
	}
	list, e := service.Jobs(ctx, a, "30d", "cancelled", "", 20, 20)
	if e != nil || list.Total != 31 || len(list.Items) != 11 || list.Items[0].Status != "cancelled" {
		t.Fatal(list, e)
	}
	// First completion before the heatmap stays excluded after later re-completion.
	_, e = db.pool.Exec(ctx, `INSERT INTO production_event VALUES($1,'complete',$2)`, first, w.HeatmapFrom.Add(-time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	v, e = service.Overview(ctx, a, "30d")
	if e != nil || v.Heatmap.Summary.Total != 31 {
		t.Fatal(v.Heatmap, e)
	}
	day, e := service.Jobs(ctx, a, "", "", now.In(time.FixedZone("Asia/Bangkok", 25200)).Add(-time.Hour).Format("2006-01-02"), 100, 0)
	if e != nil || day.Total != 31 {
		t.Fatal(day, e)
	}
	// Missing completion history cannot silently become a zero/complete history.
	_, e = db.pool.Exec(ctx, `DELETE FROM production_event WHERE job_id=(SELECT id FROM job WHERE status='completed' LIMIT 1)`)
	if e != nil {
		t.Fatal(e)
	}
	v, e = service.Overview(ctx, a, "30d")
	if e != nil || v.Heatmap.HistoryComplete || v.Heatmap.MissingHistoryJobs != 1 || v.Heatmap.Summary.CurrentStreak != nil {
		t.Fatal(v.Heatmap, e)
	}
	for _, id := range []string{worker, admin} {
		rows, e := db.DashboardJobs(ctx, id, d.Filter{Window: w, Limit: 20})
		data, _ := json.Marshal(rows)
		if e != nil || strings.Contains(string(data), "cost") || strings.Contains(string(data), "finance") || strings.Contains(string(data), "actor_id") {
			t.Fatal(string(data), e)
		}
	}
	if _, e = db.DashboardOverview(ctx, customer, w); !errors.Is(e, req.ErrForbidden) {
		t.Fatal(e)
	}
	_, e = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, worker)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.DashboardJobs(ctx, worker, d.Filter{Window: w, Limit: 20}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal(e)
	}
}
