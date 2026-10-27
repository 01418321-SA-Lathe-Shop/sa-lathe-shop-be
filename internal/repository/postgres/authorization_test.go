package postgres

import (
	"context"
	"errors"
	"testing"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
)

type actorTestRow struct {
	role string
	err  error
}

func (r actorTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = r.role
	return nil
}

type actorTestTransaction struct {
	pgx.Tx
	rows []actorTestRow
}

func (t *actorTestTransaction) QueryRow(context.Context, string, ...any) pgx.Row {
	row := t.rows[0]
	t.rows = t.rows[1:]
	return row
}

func TestFulfillmentAuthorizationPreservesInfrastructureFailures(t *testing.T) {
	infrastructure := errors.New("connection unavailable")
	for _, test := range []struct {
		name string
		rows []actorTestRow
		want error
	}{
		{"missing actor", []actorTestRow{{err: pgx.ErrNoRows}}, req.ErrForbidden},
		{"actor timeout", []actorTestRow{{err: context.DeadlineExceeded}}, context.DeadlineExceeded},
		{"actor connection", []actorTestRow{{err: infrastructure}}, infrastructure},
		{"missing membership", []actorTestRow{{role: "customer"}, {err: pgx.ErrNoRows}}, req.ErrNotFound},
		{"membership timeout", []actorTestRow{{role: "customer"}, {err: context.DeadlineExceeded}}, context.DeadlineExceeded},
		{"membership connection", []actorTestRow{{role: "customer"}, {err: infrastructure}}, infrastructure},
		{"unsupported role", []actorTestRow{{role: "supplier"}}, req.ErrForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := fulfillmentActor(context.Background(), &actorTestTransaction{rows: test.rows}, "actor", "job")
			if !errors.Is(err, test.want) {
				t.Fatal("authorization changed the underlying failure", err)
			}
		})
	}
}
