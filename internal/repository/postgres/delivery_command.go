package postgres

import (
	"context"
	"encoding/json"
	"errors"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
)

func fulfillmentRole(role, kind string) bool {
	switch kind {
	case "terms", "cash", "review":
		return role == "admin"
	case "invoice", "ship":
		return role == "admin" || role == "worker"
	case "accept", "return", "bank":
		return role == "customer"
	}
	return false
}
func (d *Database) ApplyFulfillment(ctx context.Context, actor, id string, in f.Action) (f.State, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return f.State{}, e
	}
	defer tx.Rollback(ctx)
	role, e := fulfillmentActor(ctx, tx, actor, id)
	if e != nil {
		return f.State{}, e
	}
	if !fulfillmentRole(role, in.Kind) {
		return f.State{}, req.ErrForbidden
	}
	payload, e := json.Marshal(struct {
		JobID string   `json:"job_id"`
		Input f.Action `json:"input"`
	}{id, in})
	if e != nil {
		return f.State{}, e
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,12))`, in.IdempotencyKey); e != nil {
		return f.State{}, e
	}
	var same bool
	var data []byte
	e = tx.QueryRow(ctx, `SELECT actor_id=$2 AND command=$3::jsonb,result FROM fulfillment_event WHERE id=$1`, in.IdempotencyKey, actor, payload).Scan(&same, &data)
	if e == nil {
		if !same {
			return f.State{}, f.ErrConflict
		}
		var v f.State
		if e = json.Unmarshal(data, &v); e != nil {
			return v, e
		}
		return f.Redact(v, role), tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return f.State{}, e
	}
	var locked string
	e = tx.QueryRow(ctx, `SELECT r.id::text FROM job j JOIN job_request r ON r.id=j.request_id WHERE j.id=$1 FOR UPDATE OF r`, id).Scan(&locked)
	if e != nil {
		return f.State{}, customerFulfillmentError(e)
	}
	if e = tx.QueryRow(ctx, `SELECT id::text FROM job WHERE id=$1 FOR UPDATE`, id).Scan(&locked); e != nil {
		return f.State{}, customerFulfillmentError(e)
	}
	v, e := readFulfillment(ctx, tx, id, role)
	if e != nil {
		return v, e
	}
	if e = f.CanApply(v, in); e != nil {
		return v, e
	}
	if v.Invoice != nil {
		if e = tx.QueryRow(ctx, `SELECT id::text FROM invoice WHERE job_id=$1 FOR UPDATE`, id).Scan(&locked); e != nil {
			return v, e
		}
	}
	if e = writeFulfillment(ctx, tx, actor, id, in, v); e != nil {
		return v, customerFulfillmentError(e)
	}
	if in.Kind != "terms" {
		if _, e = tx.Exec(ctx, `UPDATE job SET fulfillment_version=fulfillment_version+1 WHERE id=$1`, id); e != nil {
			return v, customerFulfillmentError(e)
		}
	}
	v, e = readFulfillment(ctx, tx, id, role)
	if e != nil {
		return v, e
	}
	if f.ShouldClose(v) {
		if _, e = tx.Exec(ctx, `UPDATE job_request SET status='completed',edited_at=clock_timestamp() WHERE id=$1`, v.RequestID); e != nil {
			return v, customerFulfillmentError(e)
		}
		v.RequestStatus = "completed"
	}
	event := f.Event{ID: in.IdempotencyKey, ActorID: actor, Action: in.Kind, Version: v.Version}
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&event.CreatedAt); e != nil {
		return v, e
	}
	v.Events = append(v.Events, event)
	result, e := json.Marshal(v)
	if e != nil {
		return v, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO fulfillment_event(id,job_id,actor_id,action,version,command,result,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, event.ID, id, actor, in.Kind, v.Version, payload, result, event.CreatedAt)
	if e != nil {
		return v, customerFulfillmentError(e)
	}
	if in.Kind == "return" {
		pv, e := readProduction(ctx, tx, id, false)
		if e != nil {
			return v, e
		}
		pe := p.Event{ID: in.IdempotencyKey, ActorID: actor, Action: "reopen", Version: pv.Version, CreatedAt: event.CreatedAt}
		pv.Events = append(pv.Events, pe)
		snapshot, e := json.Marshal(pv)
		if e != nil {
			return v, e
		}
		_, e = tx.Exec(ctx, `INSERT INTO production_event(id,job_id,actor_id,action,version,command,result,created_at) VALUES($1,$2,$3,'reopen',$4,$5,$6,$7)`, in.IdempotencyKey, id, actor, pv.Version, payload, snapshot, event.CreatedAt)
		if e != nil {
			return v, customerFulfillmentError(e)
		}
	}
	if e = fulfillmentNotice(ctx, tx, v, in); e != nil {
		return v, e
	}
	return v, customerFulfillmentError(tx.Commit(ctx))
}

func writeFulfillment(ctx context.Context, tx pgx.Tx, actor, id string, in f.Action, v f.State) error {
	var e error
	switch in.Kind {
	case "terms":
		_, e = tx.Exec(ctx, `UPDATE job SET invoice_credit_days=$2,fulfillment_version=fulfillment_version+1 WHERE id=$1`, id, *in.CreditDays)
	case "invoice":
		if v.CreditDays == nil {
			if _, e = tx.Exec(ctx, `UPDATE job SET invoice_credit_days=0 WHERE id=$1 AND invoice_credit_days IS NULL`, id); e != nil {
				return e
			}
		}
		_, e = tx.Exec(ctx, `INSERT INTO invoice(id,job_id,amount,credit_days,issued_at,due_at) SELECT $1,id,balance,COALESCE(invoice_credit_days,0),t,t+COALESCE(invoice_credit_days,0)*interval '24 hours' FROM job CROSS JOIN (SELECT clock_timestamp() t)x WHERE id=$2`, in.IdempotencyKey, id)
	case "ship":
		shippingCost := "0.00"
		if in.ShippingCost != nil {
			shippingCost = in.ShippingCost.String()
		}
		_, e = tx.Exec(ctx, `INSERT INTO delivery(id,job_id,cycle,method,shipping_cost,receiver_name) SELECT $1,id,production_cycle,$3,$4,$5 FROM job WHERE id=$2`, in.IdempotencyKey, id, in.Method, shippingCost, in.ReceiverName)
	case "accept", "return":
		last := v.Deliveries[len(v.Deliveries)-1]
		status := "delivered"
		if in.Kind == "return" {
			status = "returned"
		}
		_, e = tx.Exec(ctx, `UPDATE delivery SET status=$2,decided_by=$3,decided_at=clock_timestamp(),decision_event_id=$4,reason=$5 WHERE id=$1`, last.ID, status, actor, in.IdempotencyKey, in.Reason)
		if e != nil {
			return e
		}
		if in.Kind == "return" {
			_, e = tx.Exec(ctx, `INSERT INTO production_return(id,delivery_id,job_id,cycle,prior_started_at,prior_completed_at) SELECT $1,$2,id,production_cycle,started_at,completed_at FROM job WHERE id=$3`, in.IdempotencyKey, last.ID, id)
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `UPDATE job SET status='in_progress',production_cycle=production_cycle+1,production_version=production_version+1,completed_at=NULL WHERE id=$1`, id)
		}
	case "bank", "cash":
		var proof any
		if in.EvidenceID != "" {
			proof = in.EvidenceID
		}
		status := "pending"
		var reviewed any
		var reviewEvent any
		if in.Kind == "cash" {
			status = "approved"
			reviewed = actor
			reviewEvent = in.IdempotencyKey
		}
		_, e = tx.Exec(ctx, `INSERT INTO payment(id,invoice_id,submitted_by,method,amount,paid_at,evidence_id,status,reviewed_by,reviewed_at,review_event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,CASE WHEN $8='approved' THEN clock_timestamp() ELSE NULL END,$10)`, in.IdempotencyKey, v.Invoice.ID, actor, in.Kind, in.Amount.String(), in.PaidAt, proof, status, reviewed, reviewEvent)
		if e != nil {
			return e
		}
		if in.Kind == "cash" {
			_, e = tx.Exec(ctx, `INSERT INTO payment_allocation(payment_id,invoice_id,amount) VALUES($1,$2,$3)`, in.IdempotencyKey, v.Invoice.ID, in.Amount.String())
		}
	case "review":
		var amount string
		var status string
		e = tx.QueryRow(ctx, `SELECT amount::text,status FROM payment WHERE id=$1 AND invoice_id=$2 FOR UPDATE`, in.PaymentID, v.Invoice.ID).Scan(&amount, &status)
		if e != nil {
			return e
		}
		if status != "pending" {
			return f.ErrConflict
		}
		_, e = tx.Exec(ctx, `UPDATE payment SET status=$2,reviewed_by=$3,reviewed_at=clock_timestamp(),review_event_id=$4,reason=$5 WHERE id=$1`, in.PaymentID, in.Outcome, actor, in.IdempotencyKey, in.Reason)
		if e != nil {
			return e
		}
		if in.Outcome == "approved" {
			_, e = tx.Exec(ctx, `INSERT INTO payment_allocation(payment_id,invoice_id,amount) VALUES($1,$2,$3)`, in.PaymentID, v.Invoice.ID, amount)
		}
	default:
		return req.ErrInvalidInput
	}
	return e
}
