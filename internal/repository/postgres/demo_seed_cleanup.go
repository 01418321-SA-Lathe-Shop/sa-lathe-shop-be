package postgres

import (
	"context"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
)

func (s *demoSeedStore) Cleanup(ctx context.Context, execute bool) error {
	var sessions bool
	e := s.identityTx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_session WHERE account_id IN
 (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account'))`, s.datasetID).Scan(&sessions)
	if e != nil {
		return e
	}
	// auth_session cascades on account deletion. Even revoked tester sessions
	// are not seed-created rows, so do not silently cascade-delete them.
	if sessions {
		return demoseed.ErrDependency
	}
	tx, e := s.identityTx.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	commands := []string{
		`DELETE FROM customer_invitation WHERE id IN (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='invitation')`,
		`DELETE FROM customer_member m USING demo_seed_record r WHERE r.dataset_id=$1 AND r.entity_type='member' AND m.account_id=r.record_id AND m.customer_id=r.parent_id`,
		`DELETE FROM customer WHERE id IN (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='customer')`,
		`DELETE FROM worker WHERE person_id IN (SELECT (identity->>'person_id')::uuid FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`,
		`DELETE FROM admin WHERE person_id IN (SELECT (identity->>'person_id')::uuid FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`,
		`DELETE FROM account WHERE id IN (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`,
		`DELETE FROM person WHERE id IN (SELECT (identity->>'person_id')::uuid FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`,
		`DELETE FROM demo_seed_record WHERE dataset_id=$1`,
		`DELETE FROM demo_seed_step WHERE dataset_id=$1`,
		`DELETE FROM demo_seed_dataset WHERE id=$1`,
	}
	for _, command := range commands {
		if _, e = tx.Exec(ctx, command, s.datasetID); e != nil {
			return demoseed.ErrDependency
		}
	}
	if _, e = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); e != nil {
		return demoseed.ErrDependency
	}
	if !execute {
		return tx.Rollback(ctx)
	}
	return tx.Commit(ctx)
}

func (s *demoSeedStore) Retire(ctx context.Context) error {
	// Explicit retirement revokes sessions but preserves every identity, audit,
	// business row and physical balance. It never touches unowned accounts.
	if _, e := s.identityTx.Exec(ctx, `UPDATE account SET is_active=false,edited_at=clock_timestamp() WHERE id IN
 (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`, s.datasetID); e != nil {
		return e
	}
	if _, e := s.identityTx.Exec(ctx, `UPDATE auth_session SET revoked_at=COALESCE(revoked_at,clock_timestamp()) WHERE account_id IN
 (SELECT record_id FROM demo_seed_record WHERE dataset_id=$1 AND entity_type='account')`, s.datasetID); e != nil {
		return e
	}
	if _, e := s.identityTx.Exec(ctx, `UPDATE demo_seed_record SET identity=jsonb_set(identity,'{is_active}','false'::jsonb) WHERE dataset_id=$1 AND entity_type='account'`, s.datasetID); e != nil {
		return e
	}
	_, e := s.identityTx.Exec(ctx, `UPDATE demo_seed_dataset SET status='retired' WHERE id=$1`, s.datasetID)
	return e
}
