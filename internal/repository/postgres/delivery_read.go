package postgres

import (
	"context"
	"encoding/json"
	"errors"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/failure"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ f.Repository = (*Database)(nil)

func customerFulfillmentError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return req.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		switch pe.Code {
		case "23505", "40001", "40P01":
			return f.ErrConflict
		case "23514", "23503", "23502", "22003", "22P02", "22001":
			return req.ErrInvalidInput
		}
	}
	return e
}
func fulfillmentActor(ctx context.Context, tx pgx.Tx, actor, job string) (string, error) {
	var role string
	e := tx.QueryRow(ctx, `SELECT role FROM account WHERE id=$1 AND is_active FOR SHARE`, actor).Scan(&role)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", req.ErrForbidden
	}
	if e != nil {
		return "", failure.Wrap("fulfillment.actor", e)
	}
	if role == "customer" {
		var id string
		e = tx.QueryRow(ctx, `SELECT m.account_id::text FROM customer_member m JOIN job_request r ON r.customer_id=m.customer_id JOIN job j ON j.request_id=r.id WHERE j.id=$2 AND m.account_id=$1 FOR SHARE OF m`, actor, job).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			return "", req.ErrNotFound
		}
		if e != nil {
			return "", failure.Wrap("fulfillment.membership", e)
		}
	} else if role != "admin" && role != "worker" {
		return "", req.ErrForbidden
	}
	return role, nil
}

const fulfillmentDocument = `jsonb_build_object('job_id',j.id,'request_id',j.request_id,'job_status',j.status,'request_status',r.status,'version',j.fulfillment_version,'cycle',j.production_cycle,'credit_days',j.invoice_credit_days,'opening_balance',j.balance::text,'deposit',j.deposit_amount::text,
 'rework_history',COALESCE((SELECT jsonb_agg(jsonb_build_object('cycle',pr.cycle,'started_at',pr.prior_started_at,'completed_at',pr.prior_completed_at,'reopened_at',pr.reopened_at,'delivery_id',pr.delivery_id) ORDER BY pr.cycle) FROM production_return pr WHERE pr.job_id=j.id),'[]'::jsonb),
 'invoice',(SELECT jsonb_build_object('id',i.id,'number','INV-'||i.invoice_no::text,'amount',i.amount::text,'paid',p.paid::numeric(18,2)::text,'balance',(i.amount-p.paid)::numeric(18,2)::text,'status',CASE WHEN i.amount=p.paid THEN 'paid' WHEN p.paid>0 THEN 'partial' ELSE 'unpaid' END,'credit_days',i.credit_days,'issued_at',i.issued_at,'due_at',i.due_at) FROM invoice i CROSS JOIN LATERAL(SELECT COALESCE(sum(amount),0) AS paid FROM payment_allocation WHERE invoice_id=i.id)p WHERE i.job_id=j.id),
 'deliveries',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'cycle',d.cycle,'method',d.method,'receiver_name',d.receiver_name,'status',d.status,'shipped_at',d.shipped_at,'decided_at',d.decided_at,'decided_by',d.decided_by,'reason',d.reason) ORDER BY d.cycle) FROM delivery d WHERE d.job_id=j.id),'[]'::jsonb),
 'payments',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',p.id,'submitted_by',p.submitted_by,'method',p.method,'amount',p.amount::text,'paid_at',p.paid_at,'status',p.status,'evidence_id',p.evidence_id,'reason',p.reason,'reviewed_by',p.reviewed_by) ORDER BY p.submitted_at,p.id) FROM payment p JOIN invoice i ON i.id=p.invoice_id WHERE i.job_id=j.id),'[]'::jsonb),
 'photos',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',ph.id,'file_name',ph.file_name,'content_type',ph.content_type,'byte_size',ph.byte_size,'state',ph.state) ORDER BY ph.created_at,ph.id) FROM job_photo ph WHERE ph.job_id=j.id AND ph.state='ready'),'[]'::jsonb),
 'events',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',e.id,'actor_id',e.actor_id,'action',e.action,'version',e.version,'created_at',e.created_at) ORDER BY e.version) FROM fulfillment_event e WHERE e.job_id=j.id),'[]'::jsonb))`

func readFulfillment(ctx context.Context, tx pgx.Tx, id, role string) (f.State, error) {
	var v f.State
	var data []byte
	e := tx.QueryRow(ctx, `SELECT `+fulfillmentDocument+` FROM job j JOIN job_request r ON r.id=j.request_id WHERE j.id=$1`, id).Scan(&data)
	if e != nil {
		return v, customerFulfillmentError(e)
	}
	e = json.Unmarshal(data, &v)
	return v, e
}
func (d *Database) FulfillmentState(ctx context.Context, actor, id string) (f.State, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return f.State{}, e
	}
	defer tx.Rollback(ctx)
	role, e := fulfillmentActor(ctx, tx, actor, id)
	if e != nil {
		return f.State{}, e
	}
	var locked string
	e = tx.QueryRow(ctx, `SELECT r.id::text FROM job j JOIN job_request r ON r.id=j.request_id WHERE j.id=$1 FOR SHARE OF r`, id).Scan(&locked)
	if e != nil {
		return f.State{}, customerFulfillmentError(e)
	}
	e = tx.QueryRow(ctx, `SELECT id::text FROM job WHERE id=$1 FOR SHARE`, id).Scan(&locked)
	if e != nil {
		return f.State{}, customerFulfillmentError(e)
	}
	v, e := readFulfillment(ctx, tx, id, role)
	if e != nil {
		return v, e
	}
	return v, tx.Commit(ctx)
}
func (d *Database) FulfillmentJobs(ctx context.Context, actor string, limit, offset int) ([]f.State, error) {
	// One statement supplies one MVCC snapshot for authorization, page and all
	// child histories. Paginate before building documents; never open a per-job
	// transaction or hold read locks while a client consumes a list response.
	rows, e := d.pool.Query(ctx, fulfillmentListQuery, actor, limit, offset)
	if e != nil {
		return nil, failure.Wrap("fulfillment.list", e)
	}
	defer rows.Close()
	result := []f.State{}
	for rows.Next() {
		var data []byte
		if e = rows.Scan(&data); e != nil {
			return nil, e
		}
		var value f.State
		if e = json.Unmarshal(data, &value); e != nil {
			return nil, e
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

// SQL rewriting is confined to this constant template, never user text. Share
// the exact legacy document allowlist with detail/command responses.
var fulfillmentListQuery = `WITH viewer AS MATERIALIZED (SELECT role FROM account WHERE id=$1 AND is_active),
 page AS MATERIALIZED (SELECT j.* FROM job j JOIN job_request r ON r.id=j.request_id
 WHERE ` + accessibleRequest + ` ORDER BY j.created_at DESC,j.id LIMIT $2 OFFSET $3)
 SELECT ` + fulfillmentDocument + `
 FROM page j JOIN job_request r ON r.id=j.request_id CROSS JOIN viewer ORDER BY j.created_at DESC,j.id`
