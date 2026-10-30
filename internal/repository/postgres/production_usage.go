package postgres

import (
	"context"
	"encoding/json"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func usageMaterials(ctx context.Context, tx pgx.Tx, ids []string, lock bool) (map[string]m.Material, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF m"
	}
	rows, err := tx.Query(ctx, `SELECT `+materialFields+` FROM material m JOIN unit u ON u.id=m.unit_id WHERE m.id=ANY($1::uuid[]) ORDER BY m.id`+suffix, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]m.Material, len(ids))
	for rows.Next() {
		value, err := materialRow(rows)
		if err != nil {
			return nil, err
		}
		values[value.ID] = value
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(values) != len(ids) {
		return nil, req.ErrNotFound
	}
	return values, nil
}

func productionBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	results := tx.SendBatch(ctx, batch)
	for range batch.Len() {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return productionError(err)
		}
	}
	return productionError(results.Close())
}

func writeProductionUsage(ctx context.Context, tx pgx.Tx, actor, id string, in p.Action) error {
	ids := make([]string, len(in.Lines))
	for i, line := range in.Lines {
		ids[i] = line.MaterialID
	}
	// Request/job locks are held. SQL acquires material locks in UUID order.
	materials, err := usageMaterials(ctx, tx, ids, true)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT material_id::text,qty_reserved::text FROM material_reservation WHERE job_id=$1 AND material_id=ANY($2::uuid[])`, id, ids)
	if err != nil {
		return err
	}
	reserved := map[string]m.Decimal{}
	for rows.Next() {
		var material, text string
		if err = rows.Scan(&material, &text); err != nil {
			rows.Close()
			return err
		}
		quantity, err := m.ParseDecimal(text)
		if err != nil {
			rows.Close()
			return err
		}
		reserved[material] = quantity
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	usageIDs := make([]string, len(in.Lines))
	nextQuantities := make([]m.Decimal, len(in.Lines))
	baseQuantities := make([]m.Decimal, len(in.Lines))
	entered := make([]m.Entered, len(in.Lines))
	usageBatch := &pgx.Batch{}
	for i, line := range in.Lines {
		current := materials[line.MaterialID]
		baseQuantities[i], entered[i], err = convertEntered(ctx, tx, current, line.UnitID, *line.Quantity)
		if err != nil {
			return err
		}
		used, next, err := p.IssueEffect(current, reserved[line.MaterialID], baseQuantities[i])
		if err != nil {
			return err
		}
		usageIDs[i], nextQuantities[i] = uuid.NewString(), next
		enteredQty, enteredUnit, factor := enteredArgs(entered[i])
		usageBatch.Queue(`INSERT INTO job_material_usage(id,event_id,job_id,material_id,actor_id,material_name,unit_name,quantity,reservation_used,unit_cost,entered_qty,entered_unit_name,factor_used) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::numeric,$12,$13::numeric)`, usageIDs[i], in.IdempotencyKey, id, line.MaterialID, actor, current.Name, current.UnitName, baseQuantities[i].String(), used.String(), current.AverageCost.String(), enteredQty, enteredUnit, factor)
	}
	if err = productionBatch(ctx, tx, usageBatch); err != nil {
		return err
	}
	// Preserve triggers and read actual post-reservation versions, never infer them.
	after, err := usageMaterials(ctx, tx, ids, false)
	if err != nil {
		return err
	}
	ledgerBatch := &pgx.Batch{}
	for i, line := range in.Lines {
		current, version := materials[line.MaterialID], after[line.MaterialID].Version
		enteredQty, enteredUnit, factor := enteredArgs(entered[i])
		payload, err := json.Marshal(map[string]string{"job_id": id, "usage_id": usageIDs[i]})
		if err != nil {
			return err
		}
		ledgerBatch.Queue(`INSERT INTO stock_movement(id,material_id,actor_id,idempotency_key,mode,quantity,unit_cost,reason,expected_version,version,before_quantity,after_quantity,before_cost,after_cost,payload,job_usage_id,entered_qty,entered_unit_name,factor_used) VALUES($1,$2,$3,$1,'job_issue',$4,$5,'Production usage',$6,$7,$8,$9,$5,$5,$10,$1,$11::numeric,$12,$13::numeric)`, usageIDs[i], line.MaterialID, actor, baseQuantities[i].String(), current.AverageCost.String(), version, version+1, current.OnHand.String(), nextQuantities[i].String(), payload, enteredQty, enteredUnit, factor)
	}
	return productionBatch(ctx, tx, ledgerBatch)
}
