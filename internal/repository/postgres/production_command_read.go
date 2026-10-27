package postgres

import (
	"context"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"github.com/jackc/pgx/v5"
)

// Validation never needs usage/time/event/photo histories. The request and job
// locks are already held by the caller, preserving the existing write hierarchy.
func readProductionCommandState(ctx context.Context, tx pgx.Tx, id, kind string) (p.State, *m.Plan, error) {
	var value p.State
	err := tx.QueryRow(ctx, `SELECT `+productionSummary+` FROM job j WHERE j.id=$1`, id).Scan(&value.JobID, &value.RequestID, &value.Status, &value.Version, &value.PlanVersion, &value.StartedAt, &value.CompletedAt, &value.Cycle)
	if err != nil {
		return value, nil, productionError(err)
	}
	if kind == "labor" {
		var text string
		err = tx.QueryRow(ctx, `SELECT COALESCE(sum(total_amount),0)::numeric(18,2)::text FROM job_material_usage WHERE job_id=$1`, id).Scan(&text)
		if err != nil {
			return value, nil, productionError(err)
		}
		cost, err := q.ParseAmount(text)
		if err != nil {
			return value, nil, err
		}
		value.Finance = &p.Finance{MaterialCost: cost}
	}
	if kind != "start" {
		return value, nil, nil
	}
	plan, err := loadPlan(ctx, tx, id)
	if err != nil {
		return value, nil, err
	}
	for _, line := range plan.Lines {
		value.Materials = append(value.Materials, p.MaterialState{MaterialID: line.MaterialID, Required: line.Required, Reserved: line.Reserved})
	}
	return value, &plan, nil
}

func workerProductionSnapshot(value p.State) p.State {
	value.Finance = nil
	for i := range value.Usage {
		value.Usage[i].UnitCost = nil
		value.Usage[i].Total = nil
	}
	return value
}
