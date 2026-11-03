package postgres

import (
	"context"
	"encoding/json"
	"errors"
	r "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/receiving"
	"github.com/jackc/pgx/v5"
)

func (d *Database) startReceiving(ctx context.Context, actor, id string, c r.Command, action string, input any, admin bool) (pgx.Tx, r.State, bool, bool, []byte, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, r.State{}, false, false, nil, err
	}
	fail := func(e error) (pgx.Tx, r.State, bool, bool, []byte, error) {
		tx.Rollback(ctx)
		return nil, r.State{}, false, false, nil, e
	}
	financial, err := receivingActor(ctx, tx, actor, admin)
	if err != nil {
		return fail(err)
	}
	payload, err := json.Marshal(struct {
		OrderID string `json:"order_id"`
		Action  string `json:"action"`
		Input   any    `json:"input"`
	}{id, action, input})
	if err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,9))`, c.IdempotencyKey); err != nil {
		return fail(err)
	}
	var data []byte
	var same bool
	err = tx.QueryRow(ctx, `SELECT result,actor_id=$2 AND command=$3::jsonb FROM material_order_event WHERE id=$1`, c.IdempotencyKey, actor, payload).Scan(&data, &same)
	if err == nil {
		if !same {
			return fail(r.ErrConflict)
		}
		var v r.State
		if err = json.Unmarshal(data, &v); err != nil {
			return fail(err)
		}
		if !financial {
			v.Finance = nil
		}
		return tx, v, true, financial, payload, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM material_order WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return fail(procurementError(err))
	}
	v, err := readReceiving(ctx, tx, id, financial)
	if err != nil {
		return fail(err)
	}
	if v.Version != c.ExpectedVersion || v.Status == "cancelled" {
		return fail(r.ErrConflict)
	}
	return tx, v, false, financial, payload, nil
}
func finishReceiving(ctx context.Context, tx pgx.Tx, actor, id string, c r.Command, action string, payload []byte, status string, credit *int, paymentStatus string, financial bool) (r.State, error) {
	_, err := tx.Exec(ctx, `UPDATE material_order SET version=version+1,status=$2,credit_days=COALESCE(credit_days,$3::integer),payment_status=COALESCE(NULLIF($4,''),payment_status),full_received_at=CASE WHEN $2 IN ('received','damaged') THEN COALESCE(full_received_at,clock_timestamp()) ELSE full_received_at END WHERE id=$1`, id, status, credit, paymentStatus)
	if err != nil {
		return r.State{}, procurementError(err)
	}
	v, err := readReceiving(ctx, tx, id, financial)
	if err != nil {
		return v, err
	}
	result, err := json.Marshal(v)
	if err != nil {
		return v, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO material_order_event(id,order_id,actor_id,action,version,command,result) VALUES($1,$2,$3,$4,$5,$6,$7)`, c.IdempotencyKey, id, actor, action, v.Version, payload, result)
	if err != nil {
		return v, procurementError(err)
	}
	return v, procurementError(tx.Commit(ctx))
}
func (d *Database) SetCredit(ctx context.Context, actor, id string, in r.CreditInput) (r.State, error) {
	tx, v, replay, financial, payload, err := d.startReceiving(ctx, actor, id, in.Command, "credit_terms", in, true)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if replay {
		return v, tx.Commit(ctx)
	}
	if v.Finance.CreditDays != nil {
		return v, r.ErrConflict
	}
	return finishReceiving(ctx, tx, actor, id, in.Command, "credit_terms", payload, v.Status, in.Days, "", financial)
}
func (d *Database) ClaimReplacement(ctx context.Context, actor, id, claimID string, in r.ClaimInput) (r.State, error) {
	input := struct {
		ClaimID string       `json:"claim_id"`
		Input   r.ClaimInput `json:"input"`
	}{claimID, in}
	tx, v, replay, financial, payload, err := d.startReceiving(ctx, actor, id, in.Command, "claim", input, false)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if replay {
		return v, tx.Commit(ctx)
	}
	tag, err := tx.Exec(ctx, `UPDATE material_claim c SET claimed_by=$3,claimed_at=clock_timestamp(),note=$4 FROM material_receipt_line l WHERE c.id=$2 AND l.id=c.source_line_id AND l.order_id=$1 AND c.status='pending_claim'`, id, claimID, actor, in.Note)
	if err != nil {
		return v, procurementError(err)
	}
	if tag.RowsAffected() != 1 {
		return v, r.ErrConflict
	}
	return finishReceiving(ctx, tx, actor, id, in.Command, "claim", payload, v.Status, nil, "", financial)
}
func (d *Database) SettleSupplier(ctx context.Context, actor, id string, in r.SettlementInput) (r.State, error) {
	tx, v, replay, financial, payload, err := d.startReceiving(ctx, actor, id, in.Command, "settle", in, true)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if replay {
		return v, tx.Commit(ctx)
	}
	if (v.Status != "received" && v.Status != "partially_received" && v.Status != "damaged") || v.Finance.Settlement != nil {
		return v, r.ErrConflict
	}
	var evidence any
	if in.EvidenceID != "" {
		evidence = in.EvidenceID
	}
	_, err = tx.Exec(ctx, `INSERT INTO supplier_settlement(order_id,event_id,actor_id,amount,method,paid_at,evidence_id) SELECT id,$2,$3,total_amount,$4,$5,$6 FROM material_order WHERE id=$1`, id, in.IdempotencyKey, actor, in.Method, in.PaidAt, evidence)
	if err != nil {
		return v, procurementError(err)
	}
	var defaultCredit *int
	if v.Finance.CreditDays == nil {
		zero := 0
		defaultCredit = &zero
	}
	return finishReceiving(ctx, tx, actor, id, in.Command, "settle", payload, v.Status, defaultCredit, "paid", financial)
}
