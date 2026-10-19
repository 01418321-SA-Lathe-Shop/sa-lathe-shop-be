package postgres

import (
	"context"
	"encoding/json"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func writeProductionUsage(ctx context.Context, tx pgx.Tx, actor, id string, in p.Action) error {
	for _, line := range in.Lines {
		if _, e := readMaterial(ctx, tx, line.MaterialID, true); e != nil {
			return e
		}
	}
	for _, line := range in.Lines {
		current, e := readMaterial(ctx, tx, line.MaterialID, false)
		if e != nil {
			return e
		}
		var text string
		e = tx.QueryRow(ctx, `SELECT COALESCE((SELECT qty_reserved FROM material_reservation WHERE job_id=$1 AND material_id=$2),0)::numeric(18,6)::text`, id, line.MaterialID).Scan(&text)
		if e != nil {
			return e
		}
		own, e := m.ParseDecimal(text)
		if e != nil {
			return e
		}
		used, next, e := p.IssueEffect(current, own, *line.Quantity)
		if e != nil {
			return e
		}
		usageID := uuid.NewString()
		_, e = tx.Exec(ctx, `INSERT INTO job_material_usage(id,event_id,job_id,material_id,actor_id,material_name,unit_name,quantity,reservation_used,unit_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, usageID, in.IdempotencyKey, id, line.MaterialID, actor, current.Name, current.UnitName, line.Quantity.String(), used.String(), current.AverageCost.String())
		if e != nil {
			return productionError(e)
		}
		// The usage trigger already reduced the job's own reservation/aggregate/version.
		afterReservation, e := readMaterial(ctx, tx, line.MaterialID, false)
		if e != nil {
			return e
		}
		payload, _ := json.Marshal(map[string]string{"job_id": id, "usage_id": usageID})
		_, e = tx.Exec(ctx, `INSERT INTO stock_movement(id,material_id,actor_id,idempotency_key,mode,quantity,unit_cost,reason,expected_version,version,before_quantity,after_quantity,before_cost,after_cost,payload,job_usage_id) VALUES($1,$2,$3,$1,'job_issue',$4,$5,'Production usage',$6,$7,$8,$9,$5,$5,$10,$1)`, usageID, line.MaterialID, actor, line.Quantity.String(), current.AverageCost.String(), afterReservation.Version, afterReservation.Version+1, current.OnHand.String(), next.String(), payload)
		if e != nil {
			return productionError(e)
		}
	}
	return nil
}
