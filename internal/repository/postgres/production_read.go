package postgres

import (
	"context"
	"encoding/json"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	"github.com/jackc/pgx/v5"
)

var _ p.Repository = (*Database)(nil)

const productionSummary = `j.id::text,j.request_id::text,j.status,j.production_version,j.material_plan_version,j.started_at,j.completed_at`

func (d *Database) ProductionJobs(ctx context.Context, actor string, limit, offset int) ([]p.Summary, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if _, e = receivingActor(ctx, tx, actor, false); e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `SELECT `+productionSummary+` FROM job j WHERE status<>'cancelled' ORDER BY created_at DESC,id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if e != nil {
		return nil, e
	}
	values := []p.Summary{}
	for rows.Next() {
		var v p.Summary
		if e = rows.Scan(&v.JobID, &v.RequestID, &v.Status, &v.Version, &v.PlanVersion, &v.StartedAt, &v.CompletedAt); e != nil {
			rows.Close()
			return nil, e
		}
		values = append(values, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	return values, tx.Commit(ctx)
}

const productionDocument = `jsonb_build_object('job_id',j.id,'request_id',j.request_id,'status',j.status,'version',j.production_version,'material_plan_version',j.material_plan_version,'started_at',j.started_at,'completed_at',j.completed_at,
 'materials',COALESCE((SELECT jsonb_agg(jsonb_build_object('material_id',m.id,'name',m.name,'unit_name',u.name,'required',COALESCE(q.required_qty,0)::numeric(18,6)::text,'reserved',COALESCE(r.qty_reserved,0)::numeric(18,6)::text,'reservation_consumed',COALESCE(r.qty_consumed,0)::numeric(18,6)::text,
 'used',COALESCE((SELECT sum(quantity) FROM job_material_usage x WHERE x.job_id=j.id AND x.material_id=m.id),0)::numeric(18,6)::text,
 'released',(CASE WHEN j.status='completed' THEN COALESCE(q.required_qty,0)-COALESCE(r.qty_consumed,0) ELSE 0 END)::numeric(18,6)::text) ORDER BY m.id)
 FROM material m JOIN unit u ON u.id=m.unit_id LEFT JOIN job_material_requirement q ON q.job_id=j.id AND q.material_id=m.id LEFT JOIN material_reservation r ON r.job_id=j.id AND r.material_id=m.id
 WHERE q.required_qty>0 OR EXISTS(SELECT 1 FROM job_material_usage x WHERE x.job_id=j.id AND x.material_id=m.id)),'[]'::jsonb),
 'usage',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',x.id,'material_id',x.material_id,'material_name',x.material_name,'unit_name',x.unit_name,'quantity',x.quantity::text,'reservation_used',x.reservation_used::text,'actor_id',x.actor_id,'created_at',x.created_at)
 ||CASE WHEN $2::boolean THEN jsonb_build_object('unit_cost',x.unit_cost::text,'total_amount',x.total_amount::text) ELSE '{}'::jsonb END ORDER BY x.created_at,x.id) FROM job_material_usage x WHERE x.job_id=j.id),'[]'::jsonb),
 'time_entries',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'actor_id',t.actor_id,'minutes',t.minutes,'note',t.note,'created_at',t.created_at) ORDER BY t.created_at,t.id) FROM job_time_entry t WHERE t.job_id=j.id),'[]'::jsonb),
 'labor_minutes',COALESCE((SELECT sum(minutes) FROM job_time_entry WHERE job_id=j.id),0),
 'events',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',e.id,'actor_id',e.actor_id,'action',e.action,'version',e.version,'created_at',e.created_at) ORDER BY e.version) FROM production_event e WHERE e.job_id=j.id),'[]'::jsonb),
 'photos',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',f.id,'job_id',f.job_id,'uploaded_by',f.uploaded_by,'file_name',f.file_name,'content_type',f.content_type,'byte_size',f.byte_size,'state',f.state,'expires_at',f.expires_at) ORDER BY f.created_at,f.id) FROM job_photo f WHERE f.job_id=j.id AND f.state='ready'),'[]'::jsonb),
 'finance',CASE WHEN $2::boolean THEN jsonb_build_object('material_cost',COALESCE((SELECT sum(total_amount) FROM job_material_usage WHERE job_id=j.id),0)::numeric(18,2)::text,'labor_cost',j.actual_labor_cost::text,
 'total_cost',(j.actual_labor_cost+COALESCE((SELECT sum(total_amount) FROM job_material_usage WHERE job_id=j.id),0))::numeric(18,2)::text) ELSE NULL END)`

func readProduction(ctx context.Context, tx pgx.Tx, id string, finance bool) (p.State, error) {
	var v p.State
	var data []byte
	e := tx.QueryRow(ctx, `SELECT `+productionDocument+` FROM job j WHERE j.id=$1`, id, finance).Scan(&data)
	if e != nil {
		return v, productionError(e)
	}
	e = json.Unmarshal(data, &v)
	return v, e
}
func (d *Database) ProductionState(ctx context.Context, actor, id string) (p.State, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return p.State{}, e
	}
	defer tx.Rollback(ctx)
	finance, e := receivingActor(ctx, tx, actor, false)
	if e != nil {
		return p.State{}, e
	}
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM job WHERE id=$1 FOR SHARE`, id).Scan(&locked); e != nil {
		return p.State{}, productionError(e)
	}
	v, e := readProduction(ctx, tx, id, finance)
	if e != nil {
		return v, e
	}
	return v, tx.Commit(ctx)
}
