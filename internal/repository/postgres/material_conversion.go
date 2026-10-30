package postgres

import (
	"context"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const conversionFields = `c.id::text,c.material_id::text,c.unit_id::text,u.name,c.factor::text,c.is_active,c.created_by::text,c.created_at,c.deactivated_by::text,c.deactivated_at`

func conversionRow(row pgx.Row) (m.Conversion, error) {
	var value m.Conversion
	var factor string
	err := row.Scan(&value.ID, &value.MaterialID, &value.UnitID, &value.UnitName, &factor, &value.Active, &value.CreatedBy, &value.CreatedAt, &value.DeactivatedBy, &value.DeactivatedAt)
	if err != nil {
		return value, inventoryError(err)
	}
	value.Factor, err = m.ParseDecimal(factor)
	return value, err
}
func readConversion(ctx context.Context, tx pgx.Tx, material, id string) (m.Conversion, error) {
	return conversionRow(tx.QueryRow(ctx, `SELECT `+conversionFields+` FROM unit_conversion c JOIN unit u ON u.id=c.unit_id WHERE c.material_id=$1 AND c.id=$2`, material, id))
}
func (d *Database) ListConversions(ctx context.Context, actor, material string) ([]m.Conversion, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	if _, err = readMaterial(ctx, tx, material, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+conversionFields+` FROM unit_conversion c JOIN unit u ON u.id=c.unit_id WHERE c.material_id=$1 ORDER BY c.created_at,c.id LIMIT 1000`, material)
	if err != nil {
		return nil, err
	}
	values := []m.Conversion{}
	for rows.Next() {
		v, err := conversionRow(rows)
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
func (d *Database) CreateConversion(ctx context.Context, actor, material string, in m.ConversionInput) (m.Conversion, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Conversion{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Conversion{}, err
	}
	if _, err = readMaterial(ctx, tx, material, false); err != nil {
		return m.Conversion{}, err
	}
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO unit_conversion(id,material_id,unit_id,factor,created_by) VALUES($1,$2,$3,$4,$5)`, id, material, in.UnitID, in.Factor.String(), actor)
	if err != nil {
		return m.Conversion{}, inventoryError(err)
	}
	value, err := readConversion(ctx, tx, material, id)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

// A repeated deactivation returns the original record, so a lost response is safe to retry.
func (d *Database) DeactivateConversion(ctx context.Context, actor, material, id string) (m.Conversion, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Conversion{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Conversion{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE unit_conversion SET is_active=false,deactivated_by=$3,deactivated_at=clock_timestamp() WHERE material_id=$1 AND id=$2 AND is_active`, material, id, actor)
	if err != nil {
		return m.Conversion{}, inventoryError(err)
	}
	value, err := readConversion(ctx, tx, material, id)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
