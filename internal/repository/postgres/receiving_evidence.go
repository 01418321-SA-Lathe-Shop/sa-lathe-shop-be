package postgres

import (
	"context"
	r "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/receiving"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

var _ r.EvidenceRepository = (*Database)(nil)

const orderEvidenceFields = `id::text,order_id::text,uploaded_by::text,file_name,content_type,byte_size,staging_key,object_key,state,expires_at`

func evidenceTargets(v *r.Evidence) []any {
	return []any{&v.ID, &v.OrderID, &v.UploadedBy, &v.Name, &v.ContentType, &v.Size, &v.StagingKey, &v.ObjectKey, &v.State, &v.ExpiresAt}
}
func (d *Database) CreateOrderEvidence(ctx context.Context, v r.Evidence) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = receivingActor(ctx, tx, v.UploadedBy, true); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO order_evidence(id,order_id,uploaded_by,file_name,content_type,byte_size,staging_key,object_key,expires_at) SELECT $1,id,$3,$4,$5,$6,$7,$8,$9 FROM material_order WHERE id=$2 AND supplier_response='accepted' AND status<>'cancelled'`, v.ID, v.OrderID, v.UploadedBy, v.Name, v.ContentType, v.Size, v.StagingKey, v.ObjectKey, v.ExpiresAt)
	if err != nil {
		return procurementError(err)
	}
	if tag.RowsAffected() != 1 {
		return req.ErrNotFound
	}
	return tx.Commit(ctx)
}
func (d *Database) BeginOrderEvidence(ctx context.Context, actor, id string) (r.Evidence, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return r.Evidence{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = receivingActor(ctx, tx, actor, true); err != nil {
		return r.Evidence{}, err
	}
	var v r.Evidence
	err = tx.QueryRow(ctx, `UPDATE order_evidence SET state='checking' WHERE id=$1 AND uploaded_by=$2 AND state='pending' AND expires_at>clock_timestamp() RETURNING `+orderEvidenceFields, id, actor).Scan(evidenceTargets(&v)...)
	if err != nil {
		return v, procurementError(err)
	}
	return v, tx.Commit(ctx)
}
func (d *Database) FinishOrderEvidence(ctx context.Context, actor, id string, success bool) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if success {
		if _, err = receivingActor(ctx, tx, actor, true); err != nil {
			return err
		}
	}
	state := "failed"
	if success {
		state = "ready"
	}
	tag, err := tx.Exec(ctx, `UPDATE order_evidence SET state=$3,completed_at=CASE WHEN $3='ready' THEN now() ELSE NULL END WHERE id=$1 AND uploaded_by=$2 AND state='checking'`, id, actor, state)
	if err != nil {
		return procurementError(err)
	}
	if tag.RowsAffected() != 1 {
		return req.ErrNotFound
	}
	return tx.Commit(ctx)
}
func (d *Database) ReadOrderEvidence(ctx context.Context, actor, id string) (r.Evidence, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return r.Evidence{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = receivingActor(ctx, tx, actor, true); err != nil {
		return r.Evidence{}, err
	}
	var v r.Evidence
	err = tx.QueryRow(ctx, `SELECT `+orderEvidenceFields+` FROM order_evidence WHERE id=$1 AND state='ready'`, id).Scan(evidenceTargets(&v)...)
	if err != nil {
		return v, procurementError(err)
	}
	return v, tx.Commit(ctx)
}
