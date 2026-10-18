package postgres

import (
	"context"
	"encoding/json"
	r "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/receiving"
	"github.com/jackc/pgx/v5"
)

var _ r.Repository = (*Database)(nil)

func receivingActor(ctx context.Context, tx pgx.Tx, actor string, admin bool) (bool, error) {
	role := "staff"
	if admin {
		role = "admin"
	}
	if err := inventoryActor(ctx, tx, actor, role); err != nil {
		return false, err
	}
	var financial bool
	err := tx.QueryRow(ctx, `SELECT role='admin' FROM account WHERE id=$1`, actor).Scan(&financial)
	return financial, err
}

const receivingSummary = `o.id::text,o.order_no,o.supplier_snapshot->>'name',o.status,o.version,o.full_received_at`

func (d *Database) ReceivingOrders(ctx context.Context, actor string, limit, offset int) ([]r.Summary, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = receivingActor(ctx, tx, actor, false); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+receivingSummary+` FROM material_order o WHERE supplier_response='accepted' AND status<>'cancelled' ORDER BY created_at DESC,id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	values := []r.Summary{}
	for rows.Next() {
		var v r.Summary
		if err = rows.Scan(&v.OrderID, &v.OrderNo, &v.SupplierName, &v.Status, &v.Version, &v.FullReceivedAt); err != nil {
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

const receivingDocument = `jsonb_build_object('order_id',o.id,'order_no',o.order_no,'supplier_name',o.supplier_snapshot->>'name','status',o.status,'version',o.version,'full_received_at',o.full_received_at,
 'lines',COALESCE((SELECT jsonb_agg(jsonb_build_object('material_id',i.material_id,'material_name',i.material_name,'unit_name',i.unit_name,'ordered',i.qty_ordered::text,
  'good',COALESCE((SELECT sum(l.good) FROM material_receipt_line l WHERE l.order_id=o.id AND l.material_id=i.material_id),0)::numeric(18,6)::text,
  'damaged',COALESCE((SELECT sum(l.damaged) FROM material_receipt_line l WHERE l.order_id=o.id AND l.material_id=i.material_id),0)::numeric(18,6)::text,
  'outstanding',(i.qty_ordered-COALESCE((SELECT sum(l.good) FROM material_receipt_line l WHERE l.order_id=o.id AND l.material_id=i.material_id),0))::numeric(18,6)::text) ORDER BY i.material_id) FROM material_order_item i WHERE i.order_id=o.id),'[]'::jsonb),
 'receipts',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',rc.id,'actor_id',rc.actor_id,'note',rc.note,'created_at',rc.created_at,'lines',
  (SELECT jsonb_agg(jsonb_build_object('material_id',l.material_id,'good',l.good::text,'damaged',l.damaged::text,'replacement_claim_id',COALESCE(l.replacement_claim_id::text,'')) ORDER BY l.material_id,l.id) FROM material_receipt_line l WHERE l.receipt_id=rc.id)) ORDER BY rc.created_at,rc.id) FROM material_receipt rc WHERE rc.order_id=o.id),'[]'::jsonb),
 'claims',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',c.id,'receipt_id',l.receipt_id,'material_id',l.material_id,'quantity',c.quantity::text,'replaced',c.replaced::text,'status',c.status,'claimed_by',c.claimed_by,'claimed_at',c.claimed_at,'note',c.note) ORDER BY c.id) FROM material_claim c JOIN material_receipt_line l ON l.id=c.source_line_id WHERE l.order_id=o.id),'[]'::jsonb),
 'finance',CASE WHEN $2::boolean THEN jsonb_build_object('total_amount',o.total_amount::text,'credit_days',o.credit_days,'payment_due_at',o.full_received_at+o.credit_days*interval '24 hours',
  'payment_status',CASE WHEN EXISTS(SELECT 1 FROM supplier_settlement WHERE order_id=o.id) THEN 'paid' WHEN o.full_received_at IS NOT NULL AND o.credit_days IS NOT NULL THEN 'pending' ELSE 'unpaid' END,
  'settlement',(SELECT jsonb_build_object('actor_id',s.actor_id,'amount',s.amount::text,'method',s.method,'paid_at',s.paid_at,'evidence_id',s.evidence_id) FROM supplier_settlement s WHERE s.order_id=o.id)) ELSE NULL END)`

func readReceiving(ctx context.Context, tx pgx.Tx, id string, financial bool) (r.State, error) {
	var data []byte
	var v r.State
	err := tx.QueryRow(ctx, `SELECT `+receivingDocument+` FROM material_order o WHERE o.id=$1`, id, financial).Scan(&data)
	if err != nil {
		return v, procurementError(err)
	}
	err = json.Unmarshal(data, &v)
	return v, err
}
func (d *Database) ReceivingState(ctx context.Context, actor, id string) (r.State, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return r.State{}, err
	}
	defer tx.Rollback(ctx)
	financial, err := receivingActor(ctx, tx, actor, false)
	if err != nil {
		return r.State{}, err
	}
	var locked string
	err = tx.QueryRow(ctx, `SELECT id::text FROM material_order WHERE id=$1 AND supplier_response='accepted' AND status<>'cancelled' FOR SHARE`, id).Scan(&locked)
	if err != nil {
		return r.State{}, procurementError(err)
	}
	v, err := readReceiving(ctx, tx, id, financial)
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
