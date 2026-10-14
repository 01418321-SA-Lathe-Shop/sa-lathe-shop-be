package postgres

import (
	"context"
	"errors"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ req.Repository = (*Database)(nil)

const requestFields = `r.id::text,r.no_request,r.customer_id::text,r.created_by::text,r.description,r.qty,r.material_spec,r.deadline,r.source,r.status,r.created_at`

func requestTargets(r *req.Request) []any {
	return []any{&r.ID, &r.Number, &r.CustomerID, &r.CreatedBy, &r.Description, &r.Quantity, &r.MaterialSpec, &r.Deadline, &r.Source, &r.Status, &r.CreatedAt}
}
func (d *Database) Create(ctx context.Context, r req.Request) (req.Request, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return req.Request{}, err
	}
	defer tx.Rollback(ctx)
	var member bool
	err = tx.QueryRow(ctx, `SELECT true FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.customer_id=$1 AND a.id=$2 AND a.is_active AND a.role='customer' FOR SHARE OF a,m`, r.CustomerID, r.CreatedBy).Scan(&member)
	if errors.Is(err, pgx.ErrNoRows) {
		return req.Request{}, req.ErrForbidden
	}
	if err != nil {
		return req.Request{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO job_request(id,customer_id,created_by,description,qty,material_spec,deadline,source,status,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING no_request`, r.ID, r.CustomerID, r.CreatedBy, r.Description, r.Quantity, r.MaterialSpec, r.Deadline, r.Source, r.Status, r.CreatedAt).Scan(&r.Number)
	if err != nil {
		return req.Request{}, err
	}
	for _, channel := range []string{"in_system", "line"} {
		_, err = tx.Exec(ctx, `INSERT INTO outbox(id,request_id,channel) VALUES($1,$2,$3)`, uuid.NewString(), r.ID, channel)
		if err != nil {
			return req.Request{}, err
		}
	}
	return r, tx.Commit(ctx)
}

const accessibleRequest = `(EXISTS(SELECT 1 FROM account a WHERE a.id=$1 AND a.is_active AND a.role IN ('admin','worker')) OR EXISTS(SELECT 1 FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.account_id=$1 AND m.customer_id=r.customer_id AND a.is_active AND a.role='customer'))`

func (d *Database) GetOwned(ctx context.Context, accountID, id string) (req.Request, error) {
	var r req.Request
	err := d.pool.QueryRow(ctx, `SELECT `+requestFields+` FROM job_request r WHERE r.id=$2 AND `+accessibleRequest, accountID, id).Scan(requestTargets(&r)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, req.ErrNotFound
	}
	return r, err
}
func (d *Database) ListRequests(ctx context.Context, accountID string, limit, offset int) ([]req.Request, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+requestFields+` FROM job_request r WHERE `+accessibleRequest+` ORDER BY r.created_at DESC,r.id LIMIT $2 OFFSET $3`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]req.Request, 0)
	for rows.Next() {
		var r req.Request
		if err = rows.Scan(requestTargets(&r)...); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
