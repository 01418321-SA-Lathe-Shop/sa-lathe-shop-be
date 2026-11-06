package postgres

import (
	"context"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/jackc/pgx/v5"
)

func fulfillmentNotice(ctx context.Context, tx pgx.Tx, v f.State, in f.Action) error {
	kind, audience := "", ""
	var target any
	switch in.Kind {
	case "ship":
		kind, audience = "delivery", "customer"
	case "accept", "return":
		kind, audience = "inspection", "admin"
	case "bank":
		kind, audience = "payment", "admin"
	case "review":
		kind, audience = "review", "target"
		var id string
		if e := tx.QueryRow(ctx, `SELECT submitted_by::text FROM payment WHERE id=$1`, in.PaymentID).Scan(&id); e != nil {
			return e
		}
		target = id
	default:
		return nil
	}
	_, e := tx.Exec(ctx, `INSERT INTO outbox(id,request_id,channel,kind,audience,target_account_id) VALUES($1,$2,'in_system',$3,$4,$5)`, in.IdempotencyKey, v.RequestID, kind, audience, target)
	return e
}
