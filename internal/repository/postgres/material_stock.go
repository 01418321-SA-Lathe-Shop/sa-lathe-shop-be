package postgres

import (
	"context"
	"encoding/json"
	"errors"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const movementFields = `id::text,material_id::text,actor_id::text,idempotency_key::text,mode,quantity::text,unit_cost::text,reason,expected_version,version,before_quantity::text,after_quantity::text,before_cost::text,after_cost::text,created_at`

func movementRow(row pgx.Row) (m.Movement, error) {
	var v m.Movement
	var qty, before, after, oldCost, newCost string
	var cost *string
	err := row.Scan(&v.ID, &v.MaterialID, &v.ActorID, &v.IdempotencyKey, &v.Mode, &qty, &cost, &v.Reason, &v.ExpectedVersion, &v.Version, &before, &after, &oldCost, &newCost, &v.CreatedAt)
	if err != nil {
		return v, inventoryError(err)
	}
	parsed, err := m.ParseDecimal(qty)
	if err != nil {
		return v, err
	}
	v.Quantity = &parsed
	for _, pair := range []struct {
		text   string
		target *m.Decimal
	}{{before, &v.BeforeQuantity}, {after, &v.AfterQuantity}, {oldCost, &v.BeforeCost}, {newCost, &v.AfterCost}} {
		value, err := m.ParseDecimal(pair.text)
		if err != nil {
			return v, err
		}
		*pair.target = value
	}
	if cost != nil {
		parsed, err := m.ParseDecimal(*cost)
		if err != nil {
			return v, err
		}
		v.UnitCost = &parsed
	}
	v.CreatedAt = v.CreatedAt.UTC()
	return v, nil
}
func (d *Database) ChangeStock(ctx context.Context, actor, id string, in m.StockInput) (m.Movement, error) {
	if err := m.ValidateStock(in); err != nil {
		return m.Movement{}, err
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Movement{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Movement{}, err
	}
	material, err := readMaterial(ctx, tx, id, true)
	if err != nil {
		return m.Movement{}, err
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return m.Movement{}, err
	}
	var same bool
	err = tx.QueryRow(ctx, `SELECT actor_id=$3::uuid AND payload=$4::jsonb FROM stock_movement WHERE material_id=$1 AND idempotency_key=$2`, id, in.IdempotencyKey, actor, payload).Scan(&same)
	if err == nil {
		if !same {
			return m.Movement{}, m.ErrConflict
		}
		value, err := movementRow(tx.QueryRow(ctx, `SELECT `+movementFields+` FROM stock_movement WHERE material_id=$1 AND idempotency_key=$2`, id, in.IdempotencyKey))
		if err != nil {
			return value, err
		}
		return value, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m.Movement{}, err
	}
	if material.Version != in.ExpectedVersion || (!material.Active && in.Mode == "addition") {
		return m.Movement{}, m.ErrConflict
	}
	qty, cost, err := m.StockEffect(material, in)
	if err != nil {
		return m.Movement{}, err
	}
	var unitCost any
	if in.UnitCost != nil {
		unitCost = in.UnitCost.String()
	}
	value, err := movementRow(tx.QueryRow(ctx, `INSERT INTO stock_movement(id,material_id,actor_id,idempotency_key,mode,quantity,unit_cost,reason,expected_version,version,before_quantity,after_quantity,before_cost,after_cost,payload) VALUES($1,$2,$3,$4,$5,$6::numeric,$7::numeric,$8,$9,$10,$11::numeric,$12::numeric,$13::numeric,$14::numeric,$15::jsonb) RETURNING `+movementFields, uuid.NewString(), id, actor, in.IdempotencyKey, in.Mode, in.Quantity.String(), unitCost, in.Reason, material.Version, material.Version+1, material.OnHand.String(), qty.String(), material.AverageCost.String(), cost.String(), payload))
	if err != nil {
		return value, inventoryError(err)
	}
	return value, tx.Commit(ctx)
}
func (d *Database) ListMovements(ctx context.Context, actor, id string, limit, offset int) ([]m.Movement, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	if _, err = readMaterial(ctx, tx, id, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+movementFields+` FROM stock_movement WHERE material_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	values := []m.Movement{}
	for rows.Next() {
		v, err := movementRow(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		values = append(values, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return values, tx.Commit(ctx)
}
