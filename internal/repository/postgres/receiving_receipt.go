package postgres

import (
	"context"
	"encoding/json"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	r "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/receiving"
	"github.com/google/uuid"
)

func (d *Database) Receive(ctx context.Context, actor, id string, in r.ReceiptInput) (r.State, error) {
	tx, v, replay, financial, payload, err := d.startReceiving(ctx, actor, id, in.Command, "receive", in, false)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if replay {
		return v, tx.Commit(ctx)
	}
	if v.Status != "ordered" && v.Status != "partially_received" {
		return v, r.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO material_receipt(id,order_id,actor_id,note) VALUES($1,$2,$3,$4)`, in.IdempotencyKey, id, actor, in.Note)
	if err != nil {
		return v, procurementError(err)
	}
	// Canonical service order is material ID, then claim ID. Lock each material once
	// before inserts, preserving UC5's sorted-material lock ordering without job locks.
	previous := ""
	for _, line := range in.Lines {
		if line.MaterialID != previous {
			if _, err = readMaterial(ctx, tx, line.MaterialID, true); err != nil {
				return v, err
			}
			previous = line.MaterialID
		}
	}
	for _, line := range in.Lines {
		lineID := uuid.NewString()
		var replacement any
		if line.ReplacementClaimID != "" {
			replacement = line.ReplacementClaimID
		}
		_, err = tx.Exec(ctx, `INSERT INTO material_receipt_line(id,receipt_id,order_id,material_id,good,damaged,replacement_claim_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, lineID, in.IdempotencyKey, id, line.MaterialID, line.Good.String(), line.Damaged.String(), replacement)
		if err != nil {
			return v, procurementError(err)
		}
		if line.ReplacementClaimID == "" && !line.Damaged.IsZero() {
			_, err = tx.Exec(ctx, `INSERT INTO material_claim(id,source_line_id,quantity) VALUES($1,$2,$3)`, uuid.NewString(), lineID, line.Damaged.String())
			if err != nil {
				return v, procurementError(err)
			}
		}
		if !line.Good.IsZero() {
			current, e := readMaterial(ctx, tx, line.MaterialID, false)
			if e != nil {
				return v, e
			}
			var price string
			err = tx.QueryRow(ctx, `SELECT unit_cost::text FROM material_order_item WHERE order_id=$1 AND material_id=$2`, id, line.MaterialID).Scan(&price)
			if err != nil {
				return v, procurementError(err)
			}
			cost, e := m.ParseDecimal(price)
			if e != nil {
				return v, e
			}
			next, e := current.OnHand.Add(*line.Good)
			if e != nil {
				return v, e
			}
			average, e := m.WeightedAverage(current.OnHand, current.AverageCost, *line.Good, cost)
			if e != nil {
				return v, e
			}
			audit, _ := json.Marshal(map[string]string{"receipt_id": in.IdempotencyKey, "receipt_line_id": lineID})
			_, err = tx.Exec(ctx, `INSERT INTO stock_movement(id,material_id,actor_id,idempotency_key,mode,quantity,unit_cost,reason,expected_version,version,before_quantity,after_quantity,before_cost,after_cost,payload,receipt_line_id) VALUES($1,$2,$3,$1,'purchase_receive',$4,$5,'Supplier receipt',$6,$7,$8,$9,$10,$11,$12,$1)`, lineID, line.MaterialID, actor, line.Good.String(), price, current.Version, current.Version+1, current.OnHand.String(), next.String(), current.AverageCost.String(), average.String(), audit)
			if err != nil {
				return v, procurementError(err)
			}
		}
		if line.ReplacementClaimID != "" {
			_, err = tx.Exec(ctx, `UPDATE material_claim SET replaced=(SELECT COALESCE(sum(good),0) FROM material_receipt_line WHERE replacement_claim_id=$1) WHERE id=$1`, line.ReplacementClaimID)
			if err != nil {
				return v, procurementError(err)
			}
		}
	}
	var complete bool
	err = tx.QueryRow(ctx, `SELECT bool_and(i.qty_ordered=COALESCE((SELECT sum(good) FROM material_receipt_line l WHERE l.order_id=i.order_id AND l.material_id=i.material_id),0)) FROM material_order_item i WHERE i.order_id=$1`, id).Scan(&complete)
	if err != nil {
		return v, err
	}
	status := "partially_received"
	if complete {
		status = "received"
	}
	return finishReceiving(ctx, tx, actor, id, in.Command, "receive", payload, status, nil, financial)
}
