package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// Database owns infrastructure connections. No driver types cross this boundary.
type Database struct {
	pool       *pgxpool.Pool
	identityTx pgx.Tx
}

func Open(ctx context.Context, dsn string, maxConnections int32) (*Database, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConnections
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Database{pool: pool}, nil
}

func (d *Database) Check(ctx context.Context) error { return d.pool.Ping(ctx) }
func (d *Database) Close()                          { d.pool.Close() }
