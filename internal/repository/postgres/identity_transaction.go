package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// A private child Database can bind identity operations to one outer seed
// transaction. Existing methods use savepoints; API instances remain unbound.
type identityQueries interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (d *Database) identityQueries() identityQueries {
	if d.identityTx != nil {
		return d.identityTx
	}
	return d.pool
}

func (d *Database) identityBegin(ctx context.Context) (pgx.Tx, error) {
	if d.identityTx != nil {
		return d.identityTx.Begin(ctx)
	}
	return d.pool.Begin(ctx)
}
