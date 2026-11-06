package postgres

import (
	"context"
	"errors"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"
)

var _ p.Repository = (*Database)(nil)
var _ p.ExpiryRepository = (*Database)(nil)

func fulfillmentError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, q.ErrNotFound) {
		return req.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return p.ErrConflict
		case "23514", "23503", "23502", "22P02", "22001":
			return req.ErrInvalidInput
		}
	}
	return err
}

const evidenceFields = `e.id::text,e.quotation_id::text,e.submitted_by::text,e.idempotency_key::text,e.attachment_id::text,e.kind,e.amount::text,e.paid_at,e.status,e.version,e.submitted_at,e.reviewed_by::text,e.reviewed_at,e.reason`

// ER split: deposit and purchase_order are separate tables. kind is validated before any write, so the
// table name below is never user input. Purchase orders carry no amount or paid_at.
func evidenceTable(kind string) (table, fields string) {
	if kind == "deposit" {
		return "deposit", evidenceFields
	}
	return "purchase_order", strings.Replace(evidenceFields, "e.amount::text,e.paid_at", "NULL::text,NULL::timestamptz", 1)
}

func evidenceRow(row pgx.Row) (p.Evidence, error) {
	var e p.Evidence
	var amount *string
	err := row.Scan(&e.ID, &e.QuotationID, &e.SubmittedBy, &e.IdempotencyKey, &e.AttachmentID, &e.Kind, &amount, &e.PaidAt, &e.Status, &e.Version, &e.SubmittedAt, &e.ReviewedBy, &e.ReviewedAt, &e.Reason)
	if err != nil {
		return e, fulfillmentError(err)
	}
	if amount != nil {
		v, err := q.ParseAmount(*amount)
		if err != nil {
			return e, err
		}
		e.Amount = &v
	}
	e.SubmittedAt = e.SubmittedAt.UTC()
	if e.PaidAt != nil {
		v := e.PaidAt.UTC()
		e.PaidAt = &v
	}
	if e.ReviewedAt != nil {
		v := e.ReviewedAt.UTC()
		e.ReviewedAt = &v
	}
	return e, nil
}
func sameSubmission(e p.Evidence, in p.Submission) bool {
	if e.Kind != in.Kind || e.AttachmentID != in.AttachmentID || (e.Amount == nil) != (in.Amount == nil) || (e.PaidAt == nil) != (in.PaidAt == nil) {
		return false
	}
	return (e.Amount == nil || e.Amount.String() == in.Amount.String()) && (e.PaidAt == nil || e.PaidAt.Equal(*in.PaidAt))
}
func acceptedTerms(ctx context.Context, tx pgx.Tx, id string) (q.Terms, error) {
	value := q.Decision{QuotationID: id, Outcome: "accept"}
	if err := loadTerms(ctx, tx, &value); err != nil {
		return q.Terms{}, fulfillmentError(err)
	}
	return *value.Terms, nil
}
func (d *Database) SubmitEvidence(ctx context.Context, actor, id string, in p.Submission) (p.Evidence, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Evidence{}, err
	}
	defer tx.Rollback(ctx)
	var requestID, status string
	err = tx.QueryRow(ctx, `SELECT r.id::text,r.status FROM quotation q JOIN job_request r ON r.id=q.request_id JOIN customer_member m ON m.customer_id=r.customer_id JOIN account a ON a.id=m.account_id JOIN quotation_acceptance_terms t ON t.quotation_id=q.id WHERE q.id=$2 AND a.id=$1 AND a.role='customer' AND a.is_active AND q.status='approved' FOR UPDATE OF r FOR SHARE OF a,m`, actor, id).Scan(&requestID, &status)
	if err != nil {
		return p.Evidence{}, fulfillmentError(err)
	}
	old, err := evidenceRow(tx.QueryRow(ctx, `SELECT `+evidenceFields+` FROM term_evidence e WHERE e.quotation_id=$1 AND e.submitted_by=$2 AND e.idempotency_key=$3`, id, actor, in.IdempotencyKey))
	if err == nil {
		if !sameSubmission(old, in) {
			return p.Evidence{}, p.ErrConflict
		}
		return old, tx.Commit(ctx)
	}
	if !errors.Is(err, req.ErrNotFound) {
		return p.Evidence{}, err
	}
	terms, err := acceptedTerms(ctx, tx, id)
	if err != nil {
		return p.Evidence{}, err
	}
	if err = p.ValidateSubmission(terms, in); err != nil {
		return p.Evidence{}, err
	}
	var now time.Time
	var hasJob bool
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp(),EXISTS(SELECT 1 FROM job WHERE quotation_id=$1)`, id).Scan(&now, &hasJob); err != nil {
		return p.Evidence{}, err
	}
	if status != "ongoing" || hasJob || now.After(terms.DocumentsDueAt) {
		return p.Evidence{}, p.ErrConflict
	}
	if in.PaidAt != nil && in.PaidAt.After(now) {
		return p.Evidence{}, req.ErrInvalidInput
	}
	var available bool
	err = tx.QueryRow(ctx, `SELECT true FROM request_attachment WHERE id=$1 AND request_id=$2 AND uploaded_by=$3 AND state='ready' AND completed_at<=$4 FOR SHARE`, in.AttachmentID, requestID, actor, now).Scan(&available)
	if err != nil {
		return p.Evidence{}, fulfillmentError(err)
	}
	var amount any
	if in.Amount != nil {
		amount = in.Amount.String()
	}
	table, fields := evidenceTable(in.Kind)
	insert := `INSERT INTO ` + table + ` AS e(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,amount,paid_at,submitted_at) VALUES($1,$2,$3,$4,$5,$6,$7::numeric,$8,$9) RETURNING ` + fields
	args := []any{uuid.NewString(), id, in.AttachmentID, actor, in.IdempotencyKey, in.Kind, amount, in.PaidAt, now}
	if table == "purchase_order" {
		insert = `INSERT INTO purchase_order AS e(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,submitted_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING ` + fields
		args = []any{args[0], id, in.AttachmentID, actor, in.IdempotencyKey, in.Kind, now}
	}
	e, err := evidenceRow(tx.QueryRow(ctx, insert, args...))
	if err != nil {
		return p.Evidence{}, err
	}
	return e, tx.Commit(ctx)
}
func (d *Database) ReviewEvidence(ctx context.Context, actor, id string, in p.Review) (p.Evidence, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return p.Evidence{}, err
	}
	defer tx.Rollback(ctx)
	var quoteID, status string
	err = tx.QueryRow(ctx, `SELECT e.quotation_id::text,r.status FROM term_evidence e JOIN quotation q ON q.id=e.quotation_id JOIN job_request r ON r.id=q.request_id JOIN account a ON a.id=$1 WHERE e.id=$2 AND a.is_active AND a.role IN ('admin','worker') FOR UPDATE OF r FOR SHARE OF a`, actor, id).Scan(&quoteID, &status)
	if err != nil {
		return p.Evidence{}, fulfillmentError(err)
	}
	var kind string
	if err = tx.QueryRow(ctx, `SELECT kind FROM term_evidence WHERE id=$1`, id).Scan(&kind); err != nil {
		return p.Evidence{}, fulfillmentError(err)
	}
	table, fields := evidenceTable(kind)
	e, err := evidenceRow(tx.QueryRow(ctx, `SELECT `+fields+` FROM `+table+` e WHERE e.id=$1 FOR UPDATE`, id))
	if err != nil {
		return e, err
	}
	if e.Status != "pending" {
		if e.ReviewedBy == nil || *e.ReviewedBy != actor || e.Status != in.Outcome || e.Reason != in.Reason || in.ExpectedVersion != 1 {
			return p.Evidence{}, p.ErrConflict
		}
		return e, tx.Commit(ctx)
	}
	if status != "ongoing" || e.Version != in.ExpectedVersion {
		return p.Evidence{}, p.ErrConflict
	}
	if err = p.ValidateReview(in); err != nil {
		return p.Evidence{}, err
	}
	e, err = evidenceRow(tx.QueryRow(ctx, `UPDATE `+table+` AS e SET status=$2,version=2,reviewed_by=$3,reviewed_at=clock_timestamp(),reason=$4 WHERE id=$1 RETURNING `+fields, id, in.Outcome, actor, in.Reason))
	if err != nil {
		return e, err
	}
	if in.Outcome == "approved" {
		if err = openFulfilledJob(ctx, tx, quoteID); err != nil {
			return p.Evidence{}, err
		}
	}
	return e, tx.Commit(ctx)
}
func openFulfilledJob(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO job(id,quotation_id,request_id,deposit_amount,balance,deposit_evidence_id,po_evidence_id,due_at)
 SELECT $2,t.quotation_id,r.id,COALESCE(d.amount,0),t.final_price-COALESCE(d.amount,0),d.id,po.id,r.deadline
 FROM quotation_acceptance_terms t JOIN quotation q ON q.id=t.quotation_id JOIN job_request r ON r.id=q.request_id
 LEFT JOIN deposit d ON d.quotation_id=t.quotation_id AND d.status='approved'
 LEFT JOIN purchase_order po ON po.quotation_id=t.quotation_id AND po.status='approved'
 WHERE t.quotation_id=$1 AND r.status='ongoing' AND (NOT t.deposit_required OR d.id IS NOT NULL) AND (NOT t.po_required OR po.id IS NOT NULL)
 AND NOT EXISTS(SELECT 1 FROM uc4_cancellation WHERE quotation_id=t.quotation_id)
 ON CONFLICT(quotation_id) DO NOTHING`, id, uuid.NewString())
	return fulfillmentError(err)
}
func (d *Database) GetFulfillment(ctx context.Context, actor, id string) (p.Fulfillment, error) {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return p.Fulfillment{}, err
	}
	defer tx.Rollback(ctx)
	f := p.Fulfillment{QuotationID: id, Evidence: []p.Evidence{}}
	err = tx.QueryRow(ctx, `SELECT r.status FROM quotation q JOIN job_request r ON r.id=q.request_id JOIN quotation_acceptance_terms t ON t.quotation_id=q.id WHERE q.id=$2 AND `+accessibleRequest, actor, id).Scan(&f.RequestStatus)
	if err != nil {
		return f, fulfillmentError(err)
	}
	f.Terms, err = acceptedTerms(ctx, tx, id)
	if err != nil {
		return f, err
	}
	rows, err := tx.Query(ctx, `SELECT `+evidenceFields+` FROM term_evidence e WHERE e.quotation_id=$1 ORDER BY e.submitted_at DESC,e.id LIMIT 100`, id)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		e, err := evidenceRow(rows)
		if err != nil {
			rows.Close()
			return f, err
		}
		f.Evidence = append(f.Evidence, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return f, err
	}
	var j p.Job
	var deposit, balance string
	err = tx.QueryRow(ctx, `SELECT id::text,quotation_id::text,request_id::text,status,deposit_amount::text,balance::text,due_at,created_at FROM job WHERE quotation_id=$1`, id).Scan(&j.ID, &j.QuotationID, &j.RequestID, &j.Status, &deposit, &balance, &j.DueAt, &j.CreatedAt)
	if err == nil {
		j.DepositAmount, err = q.ParseAmount(deposit)
		if err != nil {
			return f, err
		}
		j.Balance, err = q.ParseAmount(balance)
		if err != nil {
			return f, err
		}
		j.CreatedAt = j.CreatedAt.UTC()
		f.Job = &j
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return f, err
	}
	err = tx.QueryRow(ctx, `SELECT canceled_at FROM uc4_cancellation WHERE quotation_id=$1`, id).Scan(&f.CanceledAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return f, err
	}
	return f, tx.Commit(ctx)
}
