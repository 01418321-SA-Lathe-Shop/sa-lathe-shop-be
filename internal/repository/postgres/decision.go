package postgres

import (
	"context"
	"errors"
	"time"

	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	"github.com/jackc/pgx/v5"
)

var _ q.DecisionRepository = (*Database)(nil)

type decisionQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const decisionFields = `d.quotation_id::text,d.request_id::text,d.actor_id::text,d.outcome,d.reason,d.expected_lock_version,d.decided_at,d.rejection_count,d.request_status`

func decisionRow(row pgx.Row) (q.Decision, error) {
	var value q.Decision
	err := row.Scan(&value.QuotationID, &value.RequestID, &value.ActorID, &value.Outcome, &value.Reason, &value.ExpectedLockVersion, &value.DecidedAt, &value.RejectionCount, &value.RequestStatus)
	return value, quoteError(err)
}
func loadTerms(ctx context.Context, query decisionQuery, value *q.Decision) error {
	if value.Outcome != "accept" {
		return nil
	}
	var kind, price, deposit string
	var required, po bool
	var accepted, due time.Time
	err := query.QueryRow(ctx, `SELECT customer_type,final_price::text,deposit_amount::text,deposit_required,po_required,accepted_at,documents_due_at FROM quotation_acceptance_terms WHERE quotation_id=$1`, value.QuotationID).Scan(&kind, &price, &deposit, &required, &po, &accepted, &due)
	if err != nil {
		return err
	}
	amount, err := q.ParseAmount(price)
	if err != nil {
		return err
	}
	depositAmount, err := q.ParseAmount(deposit)
	if err != nil {
		return err
	}
	balance, err := amount.Subtract(depositAmount)
	if err != nil {
		return err
	}
	// Read the recorded contract, not today's acceptance policy. The balance is
	// derived from stored amounts, never a claim that a deposit was paid.
	value.Terms = &q.Terms{CustomerType: kind, FinalPrice: amount, DepositRequired: required, PORequired: po, DepositAmount: depositAmount, BalanceAfterDeposit: balance, AcceptedAt: accepted, DocumentsDueAt: due}
	return nil
}
func (d *Database) DecideQuotation(ctx context.Context, actor, id string, in q.DecisionInput) (q.Decision, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return q.Decision{}, err
	}
	defer tx.Rollback(ctx)
	var requestID, requestStatus, kind string
	// Any current active member may decide. The membership owner flag is irrelevant.
	err = tx.QueryRow(ctx, `SELECT r.id::text,r.status,c.customer_type FROM quotation q JOIN job_request r ON r.id=q.request_id JOIN customer c ON c.id=r.customer_id JOIN customer_member m ON m.customer_id=c.id JOIN account a ON a.id=m.account_id WHERE q.id=$2 AND a.id=$1 AND a.role='customer' AND a.is_active FOR UPDATE OF r FOR SHARE OF a,m,c`, actor, id).Scan(&requestID, &requestStatus, &kind)
	if err != nil {
		return q.Decision{}, quoteError(err)
	}
	existing, err := decisionRow(tx.QueryRow(ctx, `SELECT `+decisionFields+` FROM quotation_decision d WHERE d.quotation_id=$1`, id))
	if err == nil {
		if existing.ActorID != actor || existing.Outcome != in.Outcome || existing.Reason != in.Reason || existing.ExpectedLockVersion != in.ExpectedLockVersion {
			return q.Decision{}, q.ErrConflict
		}
		if err = loadTerms(ctx, tx, &existing); err != nil {
			return q.Decision{}, err
		}
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, q.ErrNotFound) {
		return q.Decision{}, err
	}
	value, err := quoteRow(tx.QueryRow(ctx, `SELECT `+quoteFields+` FROM quotation q JOIN quotation_cost c ON c.quotation_id=q.id WHERE q.id=$1 FOR UPDATE OF q,c`, id))
	if err != nil {
		return q.Decision{}, err
	}
	if requestStatus != "ongoing" || value.Status != "sent" || value.LockVersion != in.ExpectedLockVersion {
		return q.Decision{}, q.ErrConflict
	}
	var latest int64
	err = tx.QueryRow(ctx, `SELECT max(version) FROM quotation WHERE request_id=$1`, requestID).Scan(&latest)
	if err != nil {
		return q.Decision{}, err
	}
	if latest != value.Version {
		return q.Decision{}, q.ErrConflict
	}
	var previousCount int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM quotation_decision WHERE request_id=$1 AND outcome='reject'`, requestID).Scan(&previousCount)
	if err != nil {
		return q.Decision{}, err
	}
	quoteStatus, nextStatus, count, err := q.DecisionEffect(in.Outcome, previousCount)
	if err != nil {
		return q.Decision{}, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return q.Decision{}, err
	}
	result := q.Decision{QuotationID: id, RequestID: requestID, ActorID: actor, Outcome: in.Outcome, Reason: in.Reason, ExpectedLockVersion: in.ExpectedLockVersion, DecidedAt: now, RejectionCount: count, RequestStatus: nextStatus}
	if in.Outcome == "accept" {
		terms, err := q.AcceptanceTerms(kind, value.FinalPrice, now)
		if err != nil {
			return q.Decision{}, err
		}
		result.Terms = &terms
	}
	_, err = tx.Exec(ctx, `UPDATE quotation SET status=$2,lock_version=lock_version+1,edited_by=$3,edited_at=$4 WHERE id=$1`, id, quoteStatus, actor, now)
	if err != nil {
		return q.Decision{}, quoteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO quotation_decision(quotation_id,request_id,actor_id,outcome,reason,expected_lock_version,decided_at,rejection_count,request_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, requestID, actor, in.Outcome, in.Reason, in.ExpectedLockVersion, now, count, nextStatus)
	if err != nil {
		return q.Decision{}, quoteError(err)
	}
	if terms := result.Terms; terms != nil {
		_, err = tx.Exec(ctx, `INSERT INTO quotation_acceptance_terms(quotation_id,customer_type,final_price,deposit_required,po_required,deposit_amount,accepted_at,documents_due_at) VALUES($1,$2,$3::numeric,$4,$5,$6::numeric,$7,$8)`, id, kind, terms.FinalPrice.String(), terms.DepositRequired, terms.PORequired, terms.DepositAmount.String(), terms.AcceptedAt, terms.DocumentsDueAt)
		if err != nil {
			return q.Decision{}, quoteError(err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE job_request SET status=$2,edited_at=$3 WHERE id=$1`, requestID, nextStatus, now)
	if err != nil {
		return q.Decision{}, err
	}
	return result, tx.Commit(ctx)
}
func (d *Database) GetDecision(ctx context.Context, actor, id string) (q.Decision, error) {
	value, err := decisionRow(d.pool.QueryRow(ctx, `SELECT `+decisionFields+` FROM quotation_decision d JOIN job_request r ON r.id=d.request_id WHERE d.quotation_id=$2 AND `+accessibleRequest, actor, id))
	if err != nil {
		return value, err
	}
	err = loadTerms(ctx, d.pool, &value)
	return value, err
}
func (d *Database) ListDecisions(ctx context.Context, actor, requestID string, limit, offset int) ([]q.Decision, error) {
	var accessible bool
	err := d.pool.QueryRow(ctx, `SELECT true FROM job_request r WHERE r.id=$2 AND `+accessibleRequest, actor, requestID).Scan(&accessible)
	if err != nil {
		return nil, quoteError(err)
	}
	rows, err := d.pool.Query(ctx, `SELECT `+decisionFields+` FROM quotation_decision d JOIN job_request r ON r.id=d.request_id WHERE r.id=$2 AND `+accessibleRequest+` ORDER BY d.decided_at DESC,d.quotation_id LIMIT $3 OFFSET $4`, actor, requestID, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]q.Decision, 0)
	for rows.Next() {
		value, err := decisionRow(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Release the result-set connection before loading terms (also works with a
	// one-connection pool). All decision/terms rows are immutable after commit.
	for i := range result {
		if err = loadTerms(ctx, d.pool, &result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}
