package postgres

import (
	"context"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
)

var _ f.EvidenceRepository = (*Database)(nil)

const customerEvidenceFields = `id::text,job_id::text,uploaded_by::text,file_name,content_type,byte_size,staging_key,object_key,state,expires_at`

func customerEvidenceTargets(v *f.Evidence) []any {
	return []any{&v.ID, &v.JobID, &v.UploadedBy, &v.Name, &v.ContentType, &v.Size, &v.StagingKey, &v.ObjectKey, &v.State, &v.ExpiresAt}
}
func customerEvidenceAccess(ctx context.Context, tx pgx.Tx, actor, job string) error {
	role, e := fulfillmentActor(ctx, tx, actor, job)
	if e != nil {
		return e
	}
	if role != "admin" && role != "customer" {
		return req.ErrForbidden
	}
	return nil
}
func (d *Database) CreateCustomerEvidence(ctx context.Context, v f.Evidence) error {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = customerEvidenceAccess(ctx, tx, v.UploadedBy, v.JobID); e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `INSERT INTO customer_evidence(id,job_id,uploaded_by,file_name,content_type,byte_size,staging_key,object_key,expires_at) SELECT $1,j.id,$3,$4,$5,$6,$7,$8,$9 FROM job j JOIN invoice i ON i.job_id=j.id JOIN job_request r ON r.id=j.request_id WHERE j.id=$2 AND r.status='ongoing'`, v.ID, v.JobID, v.UploadedBy, v.Name, v.ContentType, v.Size, v.StagingKey, v.ObjectKey, v.ExpiresAt)
	if e != nil {
		return customerFulfillmentError(e)
	}
	if tag.RowsAffected() != 1 {
		return f.ErrConflict
	}
	return tx.Commit(ctx)
}
func (d *Database) BeginCustomerEvidence(ctx context.Context, actor, id string) (f.Evidence, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return f.Evidence{}, e
	}
	defer tx.Rollback(ctx)
	var v f.Evidence
	e = tx.QueryRow(ctx, `SELECT `+customerEvidenceFields+` FROM customer_evidence WHERE id=$1 AND uploaded_by=$2`, id, actor).Scan(customerEvidenceTargets(&v)...)
	if e != nil {
		return v, customerFulfillmentError(e)
	}
	if e = customerEvidenceAccess(ctx, tx, actor, v.JobID); e != nil {
		return v, e
	}
	tag, e := tx.Exec(ctx, `UPDATE customer_evidence SET state='checking' WHERE id=$1 AND state='pending' AND expires_at>clock_timestamp()`, id)
	if e != nil {
		return v, e
	}
	if tag.RowsAffected() != 1 {
		return v, f.ErrConflict
	}
	v.State = "checking"
	return v, tx.Commit(ctx)
}
func (d *Database) FinishCustomerEvidence(ctx context.Context, actor, id string, success bool) error {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	state := "failed"
	if success {
		var job string
		if e = tx.QueryRow(ctx, `SELECT job_id::text FROM customer_evidence WHERE id=$1 AND uploaded_by=$2`, id, actor).Scan(&job); e != nil {
			return customerFulfillmentError(e)
		}
		if e = customerEvidenceAccess(ctx, tx, actor, job); e != nil {
			return e
		}
		state = "ready"
	}
	tag, e := tx.Exec(ctx, `UPDATE customer_evidence SET state=$3,completed_at=CASE WHEN $3='ready' THEN now() ELSE NULL END WHERE id=$1 AND uploaded_by=$2 AND state='checking'`, id, actor, state)
	if e != nil {
		return customerFulfillmentError(e)
	}
	if tag.RowsAffected() != 1 {
		return f.ErrConflict
	}
	return tx.Commit(ctx)
}
func (d *Database) ReadCustomerEvidence(ctx context.Context, actor, id string) (f.Evidence, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return f.Evidence{}, e
	}
	defer tx.Rollback(ctx)
	var v f.Evidence
	e = tx.QueryRow(ctx, `SELECT `+customerEvidenceFields+` FROM customer_evidence WHERE id=$1 AND state='ready'`, id).Scan(customerEvidenceTargets(&v)...)
	if e != nil {
		return v, customerFulfillmentError(e)
	}
	role, e := fulfillmentActor(ctx, tx, actor, v.JobID)
	if e != nil {
		return v, e
	}
	if role != "admin" && role != "worker" && (role != "customer" || actor != v.UploadedBy) {
		return v, req.ErrForbidden
	}
	return v, tx.Commit(ctx)
}
