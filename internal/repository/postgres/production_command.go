package postgres

import (
	"context"
	"encoding/json"
	"errors"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func productionError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return req.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		switch pe.Code {
		case "23505", "40001", "40P01":
			return p.ErrConflict
		case "23514", "23503", "23502", "22003", "22P02", "22001":
			return req.ErrInvalidInput
		}
	}
	return e
}
func (d *Database) ApplyProduction(ctx context.Context, actor, id string, in p.Action) (p.State, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return p.State{}, e
	}
	defer tx.Rollback(ctx)
	role := "worker"
	if in.Kind == "labor" {
		role = "admin"
	}
	if e = inventoryActor(ctx, tx, actor, role); e != nil {
		return p.State{}, e
	}
	finance := role == "admin"
	payload, e := json.Marshal(struct {
		JobID string   `json:"job_id"`
		Input p.Action `json:"input"`
	}{id, in})
	if e != nil {
		return p.State{}, e
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,11))`, in.IdempotencyKey); e != nil {
		return p.State{}, e
	}
	var same bool
	var data []byte
	e = tx.QueryRow(ctx, `SELECT actor_id=$2 AND command=$3::jsonb,result FROM production_event WHERE id=$1`, in.IdempotencyKey, actor, payload).Scan(&same, &data)
	if e == nil {
		if !same {
			return p.State{}, p.ErrConflict
		}
		var v p.State
		if e = json.Unmarshal(data, &v); e != nil {
			return v, e
		}
		if !finance {
			v.Finance = nil
			for i := range v.Usage {
				v.Usage[i].UnitCost = nil
				v.Usage[i].Total = nil
			}
		}
		return v, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return p.State{}, e
	}
	var requestStatus string
	e = tx.QueryRow(ctx, `SELECT r.status FROM job j JOIN job_request r ON r.id=j.request_id WHERE j.id=$1 FOR UPDATE OF r`, id).Scan(&requestStatus)
	if e != nil {
		return p.State{}, productionError(e)
	}
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM job WHERE id=$1 FOR UPDATE`, id).Scan(&locked); e != nil {
		return p.State{}, productionError(e)
	}
	v, e := readProduction(ctx, tx, id, finance)
	if e != nil {
		return v, e
	}
	if v.Version != in.ExpectedVersion || requestStatus != "ongoing" {
		return v, p.ErrConflict
	}
	status, e := p.NextStatus(v, in)
	if e != nil {
		return v, e
	}
	switch in.Kind {
	case "start":
		if v.Status != "queued" || v.PlanVersion != in.PlanVersion {
			return v, p.ErrConflict
		}
		plan, e := loadPlan(ctx, tx, id)
		if e != nil {
			return v, e
		}
		if !plan.Ready {
			return v, p.ErrConflict
		}
		for _, line := range plan.Lines {
			if _, e = readMaterial(ctx, tx, line.MaterialID, true); e != nil {
				return v, e
			}
		}
		status = "in_progress"
	case "use":
		if v.Status != "in_progress" {
			return v, p.ErrConflict
		}
		if e = writeProductionUsage(ctx, tx, actor, id, in); e != nil {
			return v, e
		}
	case "time":
		if v.Status != "in_progress" {
			return v, p.ErrConflict
		}
		_, e = tx.Exec(ctx, `INSERT INTO job_time_entry(id,job_id,actor_id,minutes,note) VALUES($1,$2,$3,$4,$5)`, in.IdempotencyKey, id, actor, in.Minutes, in.Note)
		if e != nil {
			return v, productionError(e)
		}
	case "complete":
		if v.Status != "in_progress" {
			return v, p.ErrConflict
		}
		rows, e := tx.Query(ctx, `SELECT material_id::text FROM material_reservation WHERE job_id=$1 AND qty_reserved>0 ORDER BY material_id`, id)
		if e != nil {
			return v, e
		}
		ids := []string{}
		for rows.Next() {
			var mid string
			if e = rows.Scan(&mid); e != nil {
				rows.Close()
				return v, e
			}
			ids = append(ids, mid)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return v, e
		}
		for _, mid := range ids {
			if _, e = readMaterial(ctx, tx, mid, true); e != nil {
				return v, e
			}
		}
		_, e = tx.Exec(ctx, `UPDATE material_reservation SET qty_reserved=0 WHERE job_id=$1 AND qty_reserved>0`, id)
		if e != nil {
			return v, productionError(e)
		}
		status = "completed"
	case "labor":
		if v.Status != "in_progress" && v.Status != "completed" {
			return v, p.ErrConflict
		}
		if _, e = p.SumAmounts(v.Finance.MaterialCost, *in.Amount); e != nil {
			return v, e
		}
	default:
		return v, req.ErrInvalidInput
	}
	var labor any
	if in.Amount != nil {
		labor = in.Amount.String()
	}
	_, e = tx.Exec(ctx, `UPDATE job SET status=$2,production_version=production_version+1,started_at=CASE WHEN $3='start' THEN clock_timestamp() ELSE started_at END,completed_at=CASE WHEN $3='complete' THEN clock_timestamp() ELSE completed_at END,actual_labor_cost=CASE WHEN $3='labor' THEN $4::numeric ELSE actual_labor_cost END WHERE id=$1`, id, status, in.Kind, labor)
	if e != nil {
		return v, productionError(e)
	}
	// Validate exact aggregate monetary bounds even for a Worker command, then only
	// persist/return the role-safe snapshot. No prices pass through Worker DTOs.
	full, e := readProduction(ctx, tx, id, true)
	if e != nil {
		return v, e
	}
	if finance {
		v = full
	} else {
		v, e = readProduction(ctx, tx, id, false)
		if e != nil {
			return v, e
		}
	}
	event := p.Event{ID: in.IdempotencyKey, ActorID: actor, Action: in.Kind, Version: v.Version}
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&event.CreatedAt); e != nil {
		return v, e
	}
	v.Events = append(v.Events, event)
	result, e := json.Marshal(v)
	if e != nil {
		return v, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO production_event(id,job_id,actor_id,action,version,command,result,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.IdempotencyKey, id, actor, in.Kind, v.Version, payload, result, event.CreatedAt)
	if e != nil {
		return v, productionError(e)
	}
	return v, productionError(tx.Commit(ctx))
}
