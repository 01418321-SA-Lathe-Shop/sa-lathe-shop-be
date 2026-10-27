package postgres

import (
	"context"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
)

// Frozen pre-optimization read algorithm, test-only. It enables alternating
// before/after runs against exactly the same synthetic rows, pool and host load.
type legacyFulfillmentReads struct{ *Database }

func (d *legacyFulfillmentReads) FulfillmentJobs(ctx context.Context, actor string, limit, offset int) ([]f.State, error) {
	rows, err := d.pool.Query(ctx, `SELECT j.id::text FROM job j JOIN job_request r ON r.id=j.request_id WHERE `+accessibleRequest+` ORDER BY j.created_at DESC,j.id LIMIT $2 OFFSET $3`, actor, limit, offset)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []f.State{}
	for _, id := range ids {
		value, err := d.FulfillmentState(ctx, actor, id)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
