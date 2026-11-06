package postgres

import (
	"context"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

var _ p.PhotoRepository = (*Database)(nil)

const jobPhotoFields = `id::text,job_id::text,uploaded_by::text,file_name,content_type,byte_size,staging_key,object_key,state,expires_at`

func photoTargets(v *p.Photo) []any {
	return []any{&v.ID, &v.JobID, &v.UploadedBy, &v.Name, &v.ContentType, &v.Size, &v.StagingKey, &v.ObjectKey, &v.State, &v.ExpiresAt}
}
func (d *Database) CreateJobPhoto(ctx context.Context, v p.Photo) error {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = inventoryActor(ctx, tx, v.UploadedBy, "worker"); e != nil {
		return e
	}
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM job WHERE id=$1 AND status='in_progress' FOR SHARE`, v.JobID).Scan(&locked); e != nil {
		return productionError(e)
	}
	_, e = tx.Exec(ctx, `INSERT INTO job_photo(id,job_id,uploaded_by,file_name,content_type,byte_size,staging_key,object_key,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.JobID, v.UploadedBy, v.Name, v.ContentType, v.Size, v.StagingKey, v.ObjectKey, v.ExpiresAt)
	if e != nil {
		return productionError(e)
	}
	return tx.Commit(ctx)
}
func (d *Database) BeginJobPhoto(ctx context.Context, actor, id string) (p.Photo, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return p.Photo{}, e
	}
	defer tx.Rollback(ctx)
	if e = inventoryActor(ctx, tx, actor, "worker"); e != nil {
		return p.Photo{}, e
	}
	var locked string
	e = tx.QueryRow(ctx, `SELECT j.id::text FROM job j JOIN job_photo f ON f.job_id=j.id WHERE f.id=$1 AND f.uploaded_by=$2 AND j.status='in_progress' FOR SHARE OF j`, id, actor).Scan(&locked)
	if e != nil {
		return p.Photo{}, productionError(e)
	}
	var v p.Photo
	e = tx.QueryRow(ctx, `UPDATE job_photo SET state='checking' WHERE id=$1 AND uploaded_by=$2 AND state='pending' AND expires_at>clock_timestamp() RETURNING `+jobPhotoFields, id, actor).Scan(photoTargets(&v)...)
	if e != nil {
		return v, productionError(e)
	}
	return v, tx.Commit(ctx)
}
func (d *Database) FinishJobPhoto(ctx context.Context, actor, id string, success bool) error {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if success {
		if e = inventoryActor(ctx, tx, actor, "worker"); e != nil {
			return e
		}
		var locked string
		e = tx.QueryRow(ctx, `SELECT j.id::text FROM job j JOIN job_photo f ON f.job_id=j.id WHERE f.id=$1 AND f.uploaded_by=$2 AND j.status='in_progress' FOR SHARE OF j`, id, actor).Scan(&locked)
		if e != nil {
			return productionError(e)
		}
	}
	state := "failed"
	if success {
		state = "ready"
	}
	tag, e := tx.Exec(ctx, `UPDATE job_photo SET state=$3,completed_at=CASE WHEN $3='ready' THEN now() ELSE NULL END WHERE id=$1 AND uploaded_by=$2 AND state='checking'`, id, actor, state)
	if e != nil {
		return productionError(e)
	}
	if tag.RowsAffected() != 1 {
		return req.ErrNotFound
	}
	return tx.Commit(ctx)
}
func (d *Database) ReadJobPhoto(ctx context.Context, actor, id string) (p.Photo, error) {
	tx, e := d.pool.Begin(ctx)
	if e != nil {
		return p.Photo{}, e
	}
	defer tx.Rollback(ctx)
	var role string
	if e = tx.QueryRow(ctx, `SELECT role FROM account WHERE id=$1 AND is_active FOR SHARE`, actor).Scan(&role); e != nil {
		return p.Photo{}, productionError(e)
	}
	if role != "admin" && role != "worker" && role != "customer" {
		return p.Photo{}, req.ErrForbidden
	}
	var v p.Photo
	// Customers see only photos of jobs on a customer account they belong to; others get 404.
	e = tx.QueryRow(ctx, `SELECT `+jobPhotoFields+` FROM job_photo WHERE id=$1 AND state='ready' AND ($3<>'customer' OR EXISTS(SELECT 1 FROM job j JOIN job_request r ON r.id=j.request_id JOIN customer_member m ON m.customer_id=r.customer_id WHERE j.id=job_photo.job_id AND m.account_id=$2))`, id, actor, role).Scan(photoTargets(&v)...)
	if e != nil {
		return v, productionError(e)
	}
	return v, tx.Commit(ctx)
}
