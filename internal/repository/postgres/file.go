package postgres

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/jackc/pgx/v5"
)

var _ file.Repository = (*Database)(nil)

const attachmentFields = `f.id::text,f.request_id::text,f.uploaded_by::text,f.file_name,f.content_type,f.byte_size,f.staging_key,f.object_key,f.state,f.expires_at`

func attachmentTargets(a *file.Attachment) []any {
	return []any{&a.ID, &a.RequestID, &a.UploadedBy, &a.Name, &a.ContentType, &a.Size, &a.StagingKey, &a.ObjectKey, &a.State, &a.ExpiresAt}
}
func (d *Database) CreateUpload(ctx context.Context, a file.Attachment) error {
	tag, err := d.pool.Exec(ctx, `INSERT INTO request_attachment(id,request_id,uploaded_by,file_name,content_type,byte_size,staging_key,object_key,expires_at)
 SELECT $2,r.id,$1,$3,$4,$5,$6,$7,$8 FROM job_request r WHERE r.id=$9 AND EXISTS(SELECT 1 FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.customer_id=r.customer_id AND m.account_id=$1 AND a.is_active AND a.role='customer')`, a.UploadedBy, a.ID, a.Name, a.ContentType, a.Size, a.StagingKey, a.ObjectKey, a.ExpiresAt, a.RequestID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return req.ErrNotFound
	}
	return nil
}
func (d *Database) BeginCompletion(ctx context.Context, accountID, id string) (file.Attachment, error) {
	var a file.Attachment
	err := d.pool.QueryRow(ctx, `UPDATE request_attachment f SET state='checking' FROM job_request r WHERE f.id=$2 AND f.request_id=r.id AND f.uploaded_by=$1 AND f.state='pending' AND f.expires_at>clock_timestamp() AND `+accessibleRequest+` RETURNING `+attachmentFields, accountID, id).Scan(attachmentTargets(&a)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, req.ErrNotFound
	}
	return a, err
}
func (d *Database) FinishCompletion(ctx context.Context, id string, success bool) error {
	state := "failed"
	if success {
		state = "ready"
	}
	tag, err := d.pool.Exec(ctx, `UPDATE request_attachment SET state=$2,completed_at=CASE WHEN $2='ready' THEN now() ELSE NULL END WHERE id=$1 AND state='checking'`, id, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return req.ErrNotFound
	}
	return nil
}
func (d *Database) GetAttachment(ctx context.Context, accountID, id string) (file.Attachment, error) {
	var a file.Attachment
	err := d.pool.QueryRow(ctx, `SELECT `+attachmentFields+` FROM request_attachment f JOIN job_request r ON r.id=f.request_id WHERE f.id=$2 AND f.state='ready' AND `+accessibleRequest, accountID, id).Scan(attachmentTargets(&a)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, req.ErrNotFound
	}
	return a, err
}
func (d *Database) ListAttachments(ctx context.Context, accountID, requestID string) ([]file.Attachment, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+attachmentFields+` FROM request_attachment f JOIN job_request r ON r.id=f.request_id WHERE r.id=$2 AND f.state='ready' AND `+accessibleRequest+` ORDER BY f.created_at,f.id`, accountID, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]file.Attachment, 0)
	for rows.Next() {
		var a file.Attachment
		if err = rows.Scan(attachmentTargets(&a)...); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
