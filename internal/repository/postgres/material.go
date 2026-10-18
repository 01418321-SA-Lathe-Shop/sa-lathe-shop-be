package postgres

import (
	"context"
	"errors"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ m.Repository = (*Database)(nil)

func inventoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return req.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return m.ErrConflict
		case "23514", "23503", "23502", "22003", "22P02", "22001":
			return req.ErrInvalidInput
		}
	}
	return err
}
func inventoryActor(ctx context.Context, tx pgx.Tx, actor, role string) error {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT true FROM account WHERE id=$1 AND is_active AND (role=$2 OR ($2='staff' AND role IN ('admin','worker'))) FOR SHARE`, actor, role).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return req.ErrForbidden
	}
	return err
}

const materialFields = `m.id::text,m.name,m.unit_id::text,u.name,m.is_active,m.version,m.qty_on_hand::text,m.qty_reserved::text,(m.qty_on_hand-m.qty_reserved)::text,m.avg_cost::text`

func materialRow(row pgx.Row) (m.Material, error) {
	var value m.Material
	var hand, reserved, available, cost string
	err := row.Scan(&value.ID, &value.Name, &value.UnitID, &value.UnitName, &value.Active, &value.Version, &hand, &reserved, &available, &cost)
	if err != nil {
		return value, inventoryError(err)
	}
	for _, pair := range []struct {
		text   string
		target *m.Decimal
	}{{hand, &value.OnHand}, {reserved, &value.Reserved}, {available, &value.Available}, {cost, &value.AverageCost}} {
		v, err := m.ParseDecimal(pair.text)
		if err != nil {
			return value, err
		}
		*pair.target = v
	}
	return value, nil
}
func readMaterial(ctx context.Context, tx pgx.Tx, id string, lock bool) (m.Material, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF m"
	}
	return materialRow(tx.QueryRow(ctx, `SELECT `+materialFields+` FROM material m JOIN unit u ON u.id=m.unit_id WHERE m.id=$1`+suffix, id))
}
func (d *Database) CreateUnit(ctx context.Context, actor string, in m.UnitInput) (m.Unit, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Unit{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Unit{}, err
	}
	value := m.Unit{ID: uuid.NewString(), Name: in.Name}
	_, err = tx.Exec(ctx, `INSERT INTO unit(id,name,created_by) VALUES($1,$2,$3)`, value.ID, value.Name, actor)
	if err != nil {
		return value, inventoryError(err)
	}
	return value, tx.Commit(ctx)
}
func (d *Database) ListUnits(ctx context.Context, actor string) ([]m.Unit, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name FROM unit ORDER BY lower(name),id LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	values := []m.Unit{}
	for rows.Next() {
		var v m.Unit
		if err = rows.Scan(&v.ID, &v.Name); err != nil {
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
func (d *Database) CreateMaterial(ctx context.Context, actor string, in m.MaterialInput) (m.Material, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Material{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Material{}, err
	}
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO material(id,name,unit_id,created_by) VALUES($1,$2,$3,$4)`, id, in.Name, in.UnitID, actor)
	if err != nil {
		return m.Material{}, inventoryError(err)
	}
	value, err := readMaterial(ctx, tx, id, false)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func (d *Database) UpdateMaterial(ctx context.Context, actor, id string, in m.MaterialUpdate) (m.Material, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Material{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "admin"); err != nil {
		return m.Material{}, err
	}
	before, err := readMaterial(ctx, tx, id, true)
	if err != nil {
		return before, err
	}
	if before.Version != in.ExpectedVersion {
		return m.Material{}, m.ErrConflict
	}
	name, active := before.Name, before.Active
	if in.Name != nil {
		name = *in.Name
	}
	if in.Active != nil {
		active = *in.Active
	}
	_, err = tx.Exec(ctx, `UPDATE material SET name=$2,is_active=$3,version=version+1,edited_at=clock_timestamp() WHERE id=$1`, id, name, active)
	if err != nil {
		return m.Material{}, inventoryError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO material_catalog_audit(id,material_id,actor_id,before_name,after_name,before_active,after_active,expected_version,version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuid.NewString(), id, actor, before.Name, name, before.Active, active, before.Version, before.Version+1)
	if err != nil {
		return m.Material{}, err
	}
	value, err := readMaterial(ctx, tx, id, false)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func (d *Database) SearchMaterials(ctx context.Context, actor, term string, limit, offset int) ([]m.Material, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+materialFields+` FROM material m JOIN unit u ON u.id=m.unit_id WHERE strpos(lower(m.name),lower($1))>0 ORDER BY lower(m.name),m.id LIMIT $2 OFFSET $3`, term, limit, offset)
	if err != nil {
		return nil, err
	}
	values := []m.Material{}
	for rows.Next() {
		v, err := materialRow(rows)
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
