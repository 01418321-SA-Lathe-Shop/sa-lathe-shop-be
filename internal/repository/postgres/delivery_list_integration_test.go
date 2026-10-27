package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyFulfillmentListContract(t *testing.T, ctx context.Context, db *Database, actors []auth.Account) {
	t.Helper()
	trace := &performanceTrace{}
	config := db.pool.Config()
	config.MaxConns = 1
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := f.NewService(&Database{pool: pool})
	for _, actor := range actors {
		before := trace.count.Load()
		values, err := service.List(ctx, actor, 50, 0)
		if err != nil {
			t.Fatal(err)
		}
		if count := trace.count.Load() - before; count != 1 {
			t.Fatalf("list must use one statement independent of page size, got %d", count)
		}
		for _, value := range values {
			// Any over-broad list scope fails the independently authorized detail read.
			detail, err := service.Get(ctx, actor, value.JobID)
			if err != nil {
				t.Fatal("list leaked an inaccessible job", err)
			}
			listedJSON, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			detailJSON, err := json.Marshal(detail)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(listedJSON, detailJSON) {
				t.Fatal("bulk list changed legacy detail fields or role redaction")
			}
		}
	}
}
