package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ notification.Repository = (*Database)(nil)

func (d *Database) ClaimEvent(ctx context.Context) (notification.Event, error) {
	var e notification.Event
	err := d.pool.QueryRow(ctx, `WITH next AS (SELECT id FROM outbox WHERE completed_at IS NULL AND failed_at IS NULL AND available_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<clock_timestamp()) ORDER BY available_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE outbox o SET lease_token=$1,lease_until=clock_timestamp()+interval '60 seconds' FROM next n,job_request r WHERE o.id=n.id AND r.id=o.request_id RETURNING o.id::text,o.request_id::text,o.lease_token::text,o.channel,o.attempts,r.no_request`, uuid.NewString()).Scan(&e.ID, &e.RequestID, &e.LeaseToken, &e.Channel, &e.Attempts, &e.RequestNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, notification.ErrEmpty
	}
	return e, err
}
func (d *Database) DeliverInbox(ctx context.Context, e notification.Event) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	var kind, audience string
	var target *string
	err = tx.QueryRow(ctx, `SELECT id::text,kind,audience,target_account_id::text FROM outbox WHERE id=$1 AND lease_token=$2 AND completed_at IS NULL FOR UPDATE`, e.ID, e.LeaseToken).Scan(&id, &kind, &audience, &target)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT a.id::text FROM account a JOIN job_request r ON r.id=$1 WHERE a.is_active AND (
 ($2='request' AND (a.role IN ('admin','worker') OR a.id=r.created_by)) OR
 ($2='admin' AND a.role='admin') OR
 ($2 IN ('customer','target') AND a.role='customer' AND EXISTS(SELECT 1 FROM customer_member m WHERE m.account_id=a.id AND m.customer_id=r.customer_id) AND ($2='customer' OR a.id=$3::uuid)))`, e.RequestID, audience, target)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	title := fmt.Sprintf("New request #%d", e.RequestNumber)
	switch kind {
	case "delivery":
		title = fmt.Sprintf("Delivery ready for inspection — request #%d", e.RequestNumber)
	case "inspection":
		title = fmt.Sprintf("Customer inspection recorded — request #%d", e.RequestNumber)
	case "payment":
		title = fmt.Sprintf("Customer payment awaiting review — request #%d", e.RequestNumber)
	case "review":
		title = fmt.Sprintf("Payment review completed — request #%d", e.RequestNumber)
	}
	for _, id := range ids {
		_, err = tx.Exec(ctx, `INSERT INTO notification(id,account_id,request_id,event_id,title) VALUES($1,$2,$3,$4,$5) ON CONFLICT(event_id,account_id) DO NOTHING`, uuid.NewString(), id, e.RequestID, e.ID, title)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE outbox SET completed_at=now(),lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2`, e.ID, e.LeaseToken)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (d *Database) CompleteEvent(ctx context.Context, e notification.Event) error {
	tag, err := d.pool.Exec(ctx, `UPDATE outbox SET completed_at=now(),lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND completed_at IS NULL`, e.ID, e.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return notification.ErrEmpty
	}
	return nil
}
func (d *Database) RetryEvent(ctx context.Context, e notification.Event, disabled bool) error {
	attempts := e.Attempts + 1
	delay := time.Duration(1<<min(attempts, 10)) * time.Second
	if disabled {
		attempts = e.Attempts
		delay = time.Minute
	}
	_, err := d.pool.Exec(ctx, `UPDATE outbox SET attempts=$3,available_at=$4,failed_at=CASE WHEN $3>=10 THEN now() ELSE NULL END,lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND completed_at IS NULL`, e.ID, e.LeaseToken, attempts, time.Now().UTC().Add(delay))
	return err
}
func (d *Database) ListNotifications(ctx context.Context, accountID string, limit, offset int) ([]notification.Item, error) {
	rows, err := d.pool.Query(ctx, `SELECT n.id::text,n.request_id::text,n.title,n.created_at,n.read_at FROM notification n JOIN account a ON a.id=n.account_id WHERE a.id=$1 AND a.is_active ORDER BY n.created_at DESC,n.id LIMIT $2 OFFSET $3`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]notification.Item, 0)
	for rows.Next() {
		var n notification.Item
		if err = rows.Scan(&n.ID, &n.RequestID, &n.Title, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}
func (d *Database) ReadNotification(ctx context.Context, accountID, id string) error {
	tag, err := d.pool.Exec(ctx, `UPDATE notification n SET read_at=COALESCE(read_at,now()) FROM account a WHERE n.id=$2 AND n.account_id=a.id AND a.id=$1 AND a.is_active`, accountID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return notification.ErrNotFound
	}
	return nil
}
