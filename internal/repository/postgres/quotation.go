package postgres

import (
	"context"
	"errors"

	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ q.Repository = (*Database)(nil)

const quoteFields = `q.id::text,q.quotation_no,q.request_id::text,q.version,q.previous_quotation_id::text,q.status,q.lock_version,c.material_cost::text,c.labor_cost::text,c.final_price::text,q.created_by::text,q.created_at,q.edited_at,q.sent_at`

func quoteRow(row pgx.Row) (q.Quotation, error) {
	var v q.Quotation
	var material, labor, price string
	err := row.Scan(&v.ID, &v.Number, &v.RequestID, &v.Version, &v.PreviousID, &v.Status, &v.LockVersion, &material, &labor, &price, &v.CreatedBy, &v.CreatedAt, &v.EditedAt, &v.SentAt)
	if err != nil {
		return v, quoteError(err)
	}
	v.MaterialCost, err = q.ParseAmount(material)
	if err != nil {
		return v, err
	}
	v.LaborCost, err = q.ParseAmount(labor)
	if err != nil {
		return v, err
	}
	v.FinalPrice, err = q.ParseAmount(price)
	return v, err
}
func quoteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return q.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return q.ErrConflict
		case "23514", "22003", "22P02":
			return q.ErrInvalidInput
		}
	}
	return err
}
func lockQuoteRequest(ctx context.Context, tx pgx.Tx, actor, requestID string) (string, error) {
	var active bool
	err := tx.QueryRow(ctx, `SELECT is_active FROM account WHERE id=$1 AND role='worker' FOR SHARE`, actor).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
		return "", q.ErrForbidden
	}
	if err != nil {
		return "", err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM job_request WHERE id=$1 FOR UPDATE`, requestID).Scan(&status)
	return status, quoteError(err)
}
func (d *Database) CreateDraft(ctx context.Context, actor string, in q.Draft) (q.Quotation, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return q.Quotation{}, err
	}
	defer tx.Rollback(ctx)
	status, err := lockQuoteRequest(ctx, tx, actor, in.RequestID)
	if err != nil {
		return q.Quotation{}, err
	}
	if status != "pending" && status != "ongoing" {
		return q.Quotation{}, q.ErrConflict
	}
	var previous *string
	version := int64(1)
	var previousID, previousStatus string
	var previousVersion int64
	err = tx.QueryRow(ctx, `SELECT id::text,status,version FROM quotation WHERE request_id=$1 ORDER BY version DESC LIMIT 1`, in.RequestID).Scan(&previousID, &previousStatus, &previousVersion)
	if err == nil {
		if previousStatus != "rejected" {
			return q.Quotation{}, q.ErrConflict
		}
		previous = &previousID
		version = previousVersion + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return q.Quotation{}, err
	} else if status != "pending" {
		return q.Quotation{}, q.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO quotation(id,request_id,version,previous_quotation_id,created_by,edited_by) VALUES($1,$2,$3,$4,$5,$5)`, in.ID, in.RequestID, version, previous, actor)
	if err != nil {
		return q.Quotation{}, quoteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO quotation_cost(quotation_id,material_cost,labor_cost,final_price) VALUES($1,$2::numeric,$3::numeric,$4::numeric)`, in.ID, in.MaterialCost.String(), in.LaborCost.String(), in.FinalPrice.String())
	if err != nil {
		return q.Quotation{}, quoteError(err)
	}
	value, err := quoteRow(tx.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id WHERE q.id=$1`, in.ID))
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func lockedQuote(ctx context.Context, tx pgx.Tx, actor, id string) (q.Quotation, error) {
	var requestID string
	err := tx.QueryRow(ctx, `SELECT request_id::text FROM quotation WHERE id=$1`, id).Scan(&requestID)
	if err != nil {
		return q.Quotation{}, quoteError(err)
	}
	status, err := lockQuoteRequest(ctx, tx, actor, requestID)
	if err != nil {
		return q.Quotation{}, err
	}
	if status != "pending" && status != "ongoing" {
		return q.Quotation{}, q.ErrConflict
	}
	return quoteRow(tx.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id WHERE q.id=$1 FOR UPDATE OF q,c`, id))
}
func (d *Database) UpdateDraft(ctx context.Context, actor string, in q.Draft) (q.Quotation, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return q.Quotation{}, err
	}
	defer tx.Rollback(ctx)
	value, err := lockedQuote(ctx, tx, actor, in.ID)
	if err != nil {
		return value, err
	}
	if value.Status != "draft" || value.LockVersion != in.ExpectedLockVersion {
		return q.Quotation{}, q.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE quotation_cost SET material_cost=$2::numeric,labor_cost=$3::numeric,final_price=$4::numeric WHERE quotation_id=$1`, in.ID, in.MaterialCost.String(), in.LaborCost.String(), in.FinalPrice.String())
	if err != nil {
		return q.Quotation{}, quoteError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE quotation SET lock_version=lock_version+1,edited_by=$2,edited_at=now() WHERE id=$1`, in.ID, actor)
	if err != nil {
		return q.Quotation{}, quoteError(err)
	}
	value, err = quoteRow(tx.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id WHERE q.id=$1`, in.ID))
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func (d *Database) SendQuotation(ctx context.Context, actor, id string, expected int64) (q.Quotation, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return q.Quotation{}, err
	}
	defer tx.Rollback(ctx)
	value, err := lockedQuote(ctx, tx, actor, id)
	if err != nil {
		return value, err
	}
	if value.Status != "draft" || value.LockVersion != expected {
		return q.Quotation{}, q.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE quotation SET status='sent',sent_at=now(),edited_at=now(),edited_by=$2,lock_version=lock_version+1 WHERE id=$1`, id, actor)
	if err != nil {
		return value, quoteError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE job_request SET status='ongoing',edited_at=now() WHERE id=$1`, value.RequestID)
	if err != nil {
		return value, err
	}
	value, err = quoteRow(tx.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id WHERE q.id=$1`, id))
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func (d *Database) RejectRequest(ctx context.Context, actor, requestID, reason string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	status, err := lockQuoteRequest(ctx, tx, actor, requestID)
	if err != nil {
		return err
	}
	if status != "pending" {
		return q.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO request_rejection(request_id,rejected_by,reason) VALUES($1,$2,$3)`, requestID, actor, reason)
	if err != nil {
		return quoteError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE job_request SET status='canceled',edited_at=now() WHERE id=$1`, requestID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const quoteVisible = ` AND (q.status<>'draft' OR EXISTS(SELECT 1 FROM account a WHERE a.id=$1 AND a.is_active AND a.role IN ('admin','worker')))`

func (d *Database) GetQuotation(ctx context.Context, actor, id string) (q.Quotation, error) {
	return quoteRow(d.pool.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id JOIN job_request r ON r.id=q.request_id WHERE q.id=$2 AND `+accessibleRequest+quoteVisible, actor, id))
}
func (d *Database) ListQuotations(ctx context.Context, actor, requestID string, limit, offset int) ([]q.Quotation, error) {
	// Distinguish accessible empty history from a hidden/missing request.
	var exists bool
	err := d.pool.QueryRow(ctx, `SELECT true FROM job_request r WHERE r.id=$2 AND `+accessibleRequest, actor, requestID).Scan(&exists)
	if err != nil {
		return nil, quoteError(err)
	}
	rows, err := d.pool.Query(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id JOIN job_request r ON r.id=q.request_id WHERE r.id=$2 AND `+accessibleRequest+quoteVisible+` ORDER BY q.version DESC LIMIT $3 OFFSET $4`, actor, requestID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]q.Quotation, 0)
	for rows.Next() {
		value, err := quoteRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
