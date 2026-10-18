package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Serialize a globally unique retry key before checking or writing it. Hash collisions
// only serialize unrelated commands; the UUID and full jsonb still decide identity.
func orderRetry(ctx context.Context, tx pgx.Tx, key, actor string, command []byte) (p.Order, bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,9))`, key); err != nil {
		return p.Order{}, false, err
	}
	var data []byte
	var same bool
	err := tx.QueryRow(ctx, `SELECT result,(actor_id=$2 AND command=$3::jsonb) FROM material_order_event WHERE id=$1`, key, actor, command).Scan(&data, &same)
	if errors.Is(err, pgx.ErrNoRows) {
		return p.Order{}, false, nil
	}
	if err != nil {
		return p.Order{}, false, err
	}
	if !same {
		return p.Order{}, true, p.ErrConflict
	}
	var v p.Order
	err = json.Unmarshal(data, &v)
	return v, true, err
}
func writeOrderEvent(ctx context.Context, tx pgx.Tx, key, actor, action, reason string, command []byte, v p.Order) (p.Order, error) {
	event := p.Event{ID: key, ActorID: actor, Action: action, Reason: reason, Version: v.Version}
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&event.CreatedAt); err != nil {
		return v, err
	}
	v.Events = append(v.Events, event)
	data, err := json.Marshal(v)
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO material_order_event(id,order_id,actor_id,action,reason,version,command,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, key, v.ID, actor, action, reason, v.Version, command, data)
	return v, procurementError(err)
}
func (d *Database) PlaceOrder(ctx context.Context, actor string, in p.OrderInput) (p.Order, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return p.Order{}, err
	}
	command, err := json.Marshal(struct {
		Action string       `json:"action"`
		Input  p.OrderInput `json:"input"`
	}{"place", in})
	if err != nil {
		return p.Order{}, err
	}
	v, replayed, err := orderRetry(ctx, tx, in.IdempotencyKey, actor, command)
	if err != nil {
		return v, err
	}
	if replayed {
		return v, tx.Commit(ctx)
	}
	supplier, err := supplierRow(tx.QueryRow(ctx, `SELECT `+supplierFields+` FROM supplier WHERE id=$1 FOR SHARE`, in.SupplierID))
	if err != nil {
		return v, err
	}
	if !supplier.Active {
		return v, p.ErrConflict
	}
	_, total, err := p.Totals(in.Lines)
	if err != nil {
		return v, err
	}
	v = p.Order{ID: uuid.NewString(), Supplier: supplier, JobIDs: in.JobIDs, Lines: []p.Line{}, Events: []p.Event{}, Total: total, Status: "ordered", SupplierResponse: "pending", Version: 1, ExpectedAt: in.ExpectedAt, CreatedBy: actor}
	v.OrderNo = "MO-" + v.ID
	// References need no plan/reservation locks or quantities. FKs protect job identity.
	for _, id := range in.JobIDs {
		_, err = tx.Exec(ctx, `INSERT INTO material_order_job(order_id,job_id) VALUES($1,$2)`, v.ID, id)
		if err != nil {
			return v, procurementError(err)
		}
	}
	for _, line := range in.Lines {
		// Shared material locks in sorted order prevent concurrent deactivation/rename
		// during snapshot creation; purchasing never updates stock or locks a job plan.
		var name, unitID, unitName string
		var active bool
		err = tx.QueryRow(ctx, `SELECT m.name,m.unit_id::text,u.name,m.is_active FROM material m JOIN unit u ON u.id=m.unit_id WHERE m.id=$1 FOR SHARE OF m`, line.MaterialID).Scan(&name, &unitID, &unitName, &active)
		if err != nil {
			return v, procurementError(err)
		}
		if !active {
			return v, p.ErrConflict
		}
		value, err := p.PriceLine(*line.Quantity, *line.UnitCost)
		if err != nil {
			return v, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO material_order_item(order_id,material_id,material_name,unit_id,unit_name,qty_ordered,unit_cost) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, line.MaterialID, name, unitID, unitName, line.Quantity.String(), line.UnitCost.String())
		if err != nil {
			return v, procurementError(err)
		}
		v.Lines = append(v.Lines, p.Line{MaterialID: line.MaterialID, MaterialName: name, UnitID: unitID, UnitName: unitName, Quantity: *line.Quantity, UnitCost: *line.UnitCost, Total: value})
	}
	snapshot, _ := json.Marshal(supplier)
	err = tx.QueryRow(ctx, `INSERT INTO material_order(id,order_no,supplier_id,supplier_snapshot,total_amount,expected_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, v.ID, v.OrderNo, supplier.ID, snapshot, v.Total.String(), in.ExpectedAt, actor).Scan(&v.CreatedAt)
	if err != nil {
		return v, procurementError(err)
	}
	v, err = writeOrderEvent(ctx, tx, in.IdempotencyKey, actor, "place", "", command, v)
	if err != nil {
		return v, err
	}
	return v, procurementError(tx.Commit(ctx))
}
func (d *Database) ActOnOrder(ctx context.Context, actor, id string, in p.ActionInput) (p.Order, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return p.Order{}, err
	}
	command, err := json.Marshal(struct {
		OrderID string        `json:"order_id"`
		Input   p.ActionInput `json:"input"`
	}{id, in})
	if err != nil {
		return p.Order{}, err
	}
	v, replayed, err := orderRetry(ctx, tx, in.IdempotencyKey, actor, command)
	if err != nil {
		return v, err
	}
	if replayed {
		return v, tx.Commit(ctx)
	}
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM material_order WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return v, procurementError(err)
	}
	v, err = readOrder(ctx, tx, id)
	if err != nil {
		return v, err
	}
	if v.Version != in.ExpectedVersion || v.Status != "ordered" {
		return v, p.ErrConflict
	}
	switch in.Action {
	case "accept", "reject":
		if v.SupplierResponse != "pending" {
			return v, p.ErrConflict
		}
		if in.Action == "accept" {
			v.SupplierResponse = "accepted"
		} else {
			v.SupplierResponse = "rejected"
			v.Status = "cancelled"
		}
	case "cancel":
		v.Status = "cancelled"
	default:
		return v, req.ErrInvalidInput
	}
	v.Version++
	tag, err := tx.Exec(ctx, `UPDATE material_order SET status=$2,supplier_response=$3,version=$4 WHERE id=$1 AND version=$5`, id, v.Status, v.SupplierResponse, v.Version, in.ExpectedVersion)
	if err != nil {
		return v, procurementError(err)
	}
	if tag.RowsAffected() != 1 {
		return v, fmt.Errorf("%w: version", p.ErrConflict)
	}
	v, err = writeOrderEvent(ctx, tx, in.IdempotencyKey, actor, in.Action, in.Reason, command, v)
	if err != nil {
		return v, err
	}
	return v, procurementError(tx.Commit(ctx))
}
