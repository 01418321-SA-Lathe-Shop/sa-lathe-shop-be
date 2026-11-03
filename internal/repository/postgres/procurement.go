package postgres

import (
	"context"
	"encoding/json"
	"errors"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

var _ p.Repository = (*Database)(nil)

func procurementError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return req.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505", "40001", "40P01":
			return p.ErrConflict
		case "23514", "23503", "23502", "22003", "22P02", "22001":
			return req.ErrInvalidInput
		}
	}
	return err
}

const supplierFields = `id::text,name,email,phone,is_active,version`

func supplierRow(row pgx.Row) (p.Supplier, error) {
	var v p.Supplier
	err := row.Scan(&v.ID, &v.Name, &v.Email, &v.Phone, &v.Active, &v.Version)
	return v, procurementError(err)
}
func (d *Database) CreateSupplier(ctx context.Context, actor string, in p.SupplierInput) (p.Supplier, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Supplier{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return p.Supplier{}, err
	}
	v, err := supplierRow(tx.QueryRow(ctx, `INSERT INTO supplier(id,name,email,phone,created_by) VALUES($1,$2,$3,$4,$5) RETURNING `+supplierFields, uuid.NewString(), in.Name, in.Email, in.Phone, actor))
	if err != nil {
		return v, err
	}
	after, _ := json.Marshal(v)
	_, err = tx.Exec(ctx, `INSERT INTO supplier_audit(id,supplier_id,actor_id,after_value) VALUES($1,$2,$3,$4)`, uuid.NewString(), v.ID, actor, after)
	if err != nil {
		return v, procurementError(err)
	}
	return v, procurementError(tx.Commit(ctx))
}
func (d *Database) UpdateSupplier(ctx context.Context, actor, id string, in p.SupplierUpdate) (p.Supplier, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Supplier{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return p.Supplier{}, err
	}
	old, err := supplierRow(tx.QueryRow(ctx, `SELECT `+supplierFields+` FROM supplier WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return old, err
	}
	if old.Version != in.ExpectedVersion {
		return p.Supplier{}, p.ErrConflict
	}
	v, err := supplierRow(tx.QueryRow(ctx, `UPDATE supplier SET name=$2,email=$3,phone=$4,is_active=$5,version=version+1,edited_at=now() WHERE id=$1 RETURNING `+supplierFields, id, in.Name, in.Email, in.Phone, *in.Active))
	if err != nil {
		return v, err
	}
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(v)
	_, err = tx.Exec(ctx, `INSERT INTO supplier_audit(id,supplier_id,actor_id,before_value,after_value) VALUES($1,$2,$3,$4,$5)`, uuid.NewString(), id, actor, before, after)
	if err != nil {
		return v, procurementError(err)
	}
	return v, procurementError(tx.Commit(ctx))
}
func (d *Database) ListSuppliers(ctx context.Context, actor, term string, limit, offset int) ([]p.Supplier, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+supplierFields+` FROM supplier WHERE strpos(lower(name),lower($1))>0 ORDER BY lower(name),id LIMIT $2 OFFSET $3`, term, limit, offset)
	if err != nil {
		return nil, err
	}
	values := []p.Supplier{}
	for rows.Next() {
		v, e := supplierRow(rows)
		if e != nil {
			rows.Close()
			return nil, e
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
func (d *Database) RecommendSuppliers(ctx context.Context, actor, materialID, excludeSupplierID string) ([]p.SupplierRecommendation, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT
			s.id::text, s.name, s.email, s.phone, s.is_active, s.version,
			COALESCE(hist.order_count, 0) > 0 AS has_history,
			hist.last_unit_cost::text,
			hist.last_ordered_at,
			COALESCE(hist.order_count, 0)
		FROM supplier s
		LEFT JOIN LATERAL (
			SELECT
				(ARRAY_AGG(moi.unit_cost ORDER BY mo.created_at DESC, mo.id DESC))[1] AS last_unit_cost,
				MAX(mo.created_at) AS last_ordered_at,
				COUNT(*)::int AS order_count
			FROM material_order_item moi
			JOIN material_order mo ON mo.id = moi.order_id
			WHERE mo.supplier_id = s.id
			  AND mo.status <> 'cancelled'
			  AND ($1 = '' OR moi.material_id = NULLIF($1, '')::uuid)
		) hist ON true
		WHERE s.is_active = TRUE
		  AND ($2 = '' OR s.id <> NULLIF($2, '')::uuid)
		ORDER BY has_history DESC, hist.last_ordered_at DESC NULLS LAST, lower(s.name), s.id
		LIMIT 100
	`, materialID, excludeSupplierID)
	if err != nil {
		return nil, err
	}
	values := []p.SupplierRecommendation{}
	for rows.Next() {
		var rec p.SupplierRecommendation
		var lastCost *string
		var lastOrderedAt *time.Time
		if err = rows.Scan(
			&rec.Supplier.ID, &rec.Supplier.Name, &rec.Supplier.Email, &rec.Supplier.Phone, &rec.Supplier.Active, &rec.Supplier.Version,
			&rec.HasHistory, &lastCost, &lastOrderedAt, &rec.OrderCount,
		); err != nil {
			rows.Close()
			return nil, err
		}
		if lastCost != nil {
			parsed, parseErr := m.ParseDecimal(*lastCost)
			if parseErr != nil {
				rows.Close()
				return nil, parseErr
			}
			rec.LastUnitCost = &parsed
		}
		rec.LastOrderedAt = lastOrderedAt
		values = append(values, rec)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return values, tx.Commit(ctx)
}
func (d *Database) ListShortages(ctx context.Context, actor string) ([]p.ShortageItem, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		WITH job_deficits AS (
			SELECT
				r.material_id,
				SUM(GREATEST(r.required_qty - COALESCE(s.qty_consumed, 0) - COALESCE(s.qty_reserved, 0), 0)) AS unreserved_req,
				ARRAY_AGG(DISTINCT j.id::text ORDER BY j.id::text) FILTER (
					WHERE GREATEST(r.required_qty - COALESCE(s.qty_consumed, 0) - COALESCE(s.qty_reserved, 0), 0) > 0
				) AS job_ids
			FROM job_material_requirement r
			JOIN job j ON j.id = r.job_id
			LEFT JOIN material_reservation s ON s.job_id = r.job_id AND s.material_id = r.material_id
			WHERE j.status IN ('queued', 'in_progress') AND r.required_qty > 0
			GROUP BY r.material_id
		),
		computed AS (
			SELECT
				m.id::text AS material_id,
				m.name AS material_name,
				m.unit_id::text AS unit_id,
				u.name AS unit_name,
				m.qty_on_hand,
				m.qty_reserved,
				GREATEST(
					COALESCE(jd.unreserved_req, 0) - GREATEST(m.qty_on_hand - m.qty_reserved, 0),
					GREATEST(m.qty_reserved - m.qty_on_hand, 0),
					0
				) AS shortage_qty,
				COALESCE(jd.job_ids, ARRAY[]::text[]) AS job_ids
			FROM material m
			JOIN unit u ON u.id = m.unit_id
			LEFT JOIN job_deficits jd ON jd.material_id = m.id
			WHERE m.is_active = TRUE
		)
		SELECT
			material_id,
			material_name,
			unit_id,
			unit_name,
			qty_on_hand::text,
			qty_reserved::text,
			(qty_reserved + shortage_qty)::text AS qty_required,
			shortage_qty::text,
			shortage_qty::text AS recommended_order_qty,
			job_ids
		FROM computed
		WHERE shortage_qty > 0
		ORDER BY shortage_qty DESC, lower(material_name), material_id
	`)
	if err != nil {
		return nil, err
	}
	values := []p.ShortageItem{}
	for rows.Next() {
		var item p.ShortageItem
		var hand, reserved, required, shortage, recommended string
		if err = rows.Scan(
			&item.MaterialID, &item.MaterialName, &item.UnitID, &item.UnitName,
			&hand, &reserved, &required, &shortage, &recommended, &item.JobIDs,
		); err != nil {
			rows.Close()
			return nil, err
		}
		for _, pair := range []struct {
			text   string
			target *m.Decimal
		}{
			{hand, &item.QtyOnHand},
			{reserved, &item.QtyReserved},
			{required, &item.QtyRequired},
			{shortage, &item.ShortageQty},
			{recommended, &item.RecommendedOrderQty},
		} {
			parsed, parseErr := m.ParseDecimal(pair.text)
			if parseErr != nil {
				rows.Close()
				return nil, parseErr
			}
			*pair.target = parsed
		}
		if item.JobIDs == nil {
			item.JobIDs = []string{}
		}
		values = append(values, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return values, tx.Commit(ctx)
}

// One statement gives a consistent order+items+links+audit snapshot, even with a one-connection pool.
const orderDocument = `jsonb_build_object(
 'id',o.id,'order_no',o.order_no,'supplier',o.supplier_snapshot,
 'status',o.status,'supplier_response',o.supplier_response,'version',o.version,
 'total_amount',o.total_amount::text,'expected_at',o.expected_at,'created_at',o.created_at,'created_by',o.created_by,
 'job_ids',COALESCE((SELECT jsonb_agg(j.job_id ORDER BY j.job_id) FROM material_order_job j WHERE j.order_id=o.id),'[]'::jsonb),
 'lines',COALESCE((SELECT jsonb_agg(jsonb_build_object('material_id',i.material_id,'material_name',i.material_name,'unit_id',i.unit_id,'unit_name',i.unit_name,'quantity',i.qty_ordered::text,'unit_cost',i.unit_cost::text,'total_amount',i.total_amount::text) ORDER BY i.material_id) FROM material_order_item i WHERE i.order_id=o.id),'[]'::jsonb),
 'events',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',e.id,'actor_id',e.actor_id,'action',e.action,'reason',e.reason,'version',e.version,'created_at',e.created_at) ORDER BY e.version) FROM material_order_event e WHERE e.order_id=o.id),'[]'::jsonb))`

func orderRow(row pgx.Row) (p.Order, error) {
	var data []byte
	var v p.Order
	if err := row.Scan(&data); err != nil {
		return v, procurementError(err)
	}
	err := json.Unmarshal(data, &v)
	return v, err
}
func readOrder(ctx context.Context, tx pgx.Tx, id string) (p.Order, error) {
	return orderRow(tx.QueryRow(ctx, `SELECT `+orderDocument+` FROM material_order o WHERE o.id=$1`, id))
}
func (d *Database) ReadOrder(ctx context.Context, actor, id string) (p.Order, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return p.Order{}, err
	}
	v, err := readOrder(ctx, tx, id)
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (d *Database) ListOrders(ctx context.Context, actor, jobID string, limit, offset int) ([]p.Order, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+orderDocument+` FROM material_order o WHERE ($1='' OR EXISTS(SELECT 1 FROM material_order_job j WHERE j.order_id=o.id AND j.job_id=NULLIF($1,'')::uuid)) ORDER BY o.created_at DESC,o.id DESC LIMIT $2 OFFSET $3`, jobID, limit, offset)
	if err != nil {
		return nil, err
	}
	values := []p.Order{}
	for rows.Next() {
		v, e := orderRow(rows)
		if e != nil {
			rows.Close()
			return nil, e
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
