package postgres

import (
	"context"
)

func (d *Database) ExpireUnfulfilled(ctx context.Context) (int, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	// SKIP LOCKED lets multiple workers cooperate. Keep request locks through the
	// state change and audit. Submission and reviews acquire these same locks.
	rows, err := tx.Query(ctx, `SELECT r.id::text,t.quotation_id::text FROM job_request r JOIN quotation q ON q.request_id=r.id JOIN quotation_acceptance_terms t ON t.quotation_id=q.id
 WHERE r.status='ongoing' AND t.documents_due_at<clock_timestamp()
 AND NOT EXISTS(SELECT 1 FROM job WHERE quotation_id=t.quotation_id)
 AND NOT ((NOT t.deposit_required OR EXISTS(SELECT 1 FROM term_evidence e WHERE e.quotation_id=t.quotation_id AND e.kind='deposit' AND e.status IN ('pending','approved') AND e.submitted_at<=t.documents_due_at))
 AND (NOT t.po_required OR EXISTS(SELECT 1 FROM term_evidence e WHERE e.quotation_id=t.quotation_id AND e.kind='po' AND e.status IN ('pending','approved') AND e.submitted_at<=t.documents_due_at)))
 ORDER BY t.documents_due_at,r.id LIMIT 50 FOR UPDATE OF r SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type target struct{ request, quote string }
	var targets []target
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.request, &t.quote); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, t := range targets {
		// Re-evaluate eligibility after the lock in a new statement/snapshot, so a
		// concurrent committed submission can never be canceled using stale reads.
		tag, err := tx.Exec(ctx, `UPDATE job_request r SET status='canceled',edited_at=clock_timestamp() FROM quotation_acceptance_terms t WHERE r.id=$1 AND t.quotation_id=$2 AND r.status='ongoing' AND t.documents_due_at<clock_timestamp()
  AND NOT EXISTS(SELECT 1 FROM job WHERE quotation_id=t.quotation_id)
  AND NOT ((NOT t.deposit_required OR EXISTS(SELECT 1 FROM term_evidence e WHERE e.quotation_id=t.quotation_id AND e.kind='deposit' AND e.status IN ('pending','approved') AND e.submitted_at<=t.documents_due_at)) AND (NOT t.po_required OR EXISTS(SELECT 1 FROM term_evidence e WHERE e.quotation_id=t.quotation_id AND e.kind='po' AND e.status IN ('pending','approved') AND e.submitted_at<=t.documents_due_at)))`, t.request, t.quote)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 1 {
			// M-04 (UC4 Except-Step 2 / Q4.E2): Also mark the quotation rejected on 14-day expiry.
			if _, err = tx.Exec(ctx, `UPDATE quotation SET status='rejected',edited_at=clock_timestamp(),lock_version=lock_version+1 WHERE id=$1`, t.quote); err != nil {
				return 0, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO uc4_cancellation(quotation_id,request_id) VALUES($1,$2)`, t.quote, t.request); err != nil {
				return 0, err
			}
			count++
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
