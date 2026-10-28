package postgres

import (
	"context"
	"errors"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
	"github.com/jackc/pgx/v5"
)

var _ auth.LocalTestRepository = (*Database)(nil)

func (d *Database) CreateLocalTestSession(ctx context.Context, session auth.Session, username string) (auth.Account, error) {
	var role string
	for _, spec := range demoseed.Accounts() {
		if spec.Username == username {
			role = spec.Role
			break
		}
	}
	if role == "" {
		return auth.Account{}, auth.ErrUnauthenticated
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return auth.Account{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=public; SET LOCAL lock_timeout='5s'`); err != nil {
		return auth.Account{}, err
	}
	// Check the actual connected DB, not just a URL or environment label.
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT current_database() IN ('sa_lathe_shop_demo','sa_lathe_shop_seed_test')`).Scan(&allowed); err != nil {
		return auth.Account{}, err
	}
	if !allowed {
		return auth.Account{}, auth.ErrUnauthenticated
	}
	var datasetID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.demo_seed_dataset
 WHERE name=$1 AND fixture_version=$2 AND status='active' AND environment IN ('local','test') FOR SHARE`, demoseed.DatasetName, demoseed.FixtureVersion).Scan(&datasetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Account{}, auth.ErrUnauthenticated
	}
	if err != nil {
		return auth.Account{}, err
	}
	// Lock the ownership record and account through insertion. The safe identity
	// projection detects role/profile/identity drift without comparing passwords.
	a, err := accountRow(tx.QueryRow(ctx, `WITH current_identity AS (`+seedIdentities+`)
 SELECT `+accountFields+` FROM public.account a JOIN public.person p ON p.id=a.person_id
 JOIN public.demo_seed_record r ON r.record_id=a.id AND r.dataset_id=$1 AND r.entity_type='account'
 JOIN current_identity c ON c.entity_type='account' AND c.record_id=a.id AND c.identity=r.identity
 WHERE a.username=$2 AND a.role=$3 AND a.is_active AND r.record_key='account:'||a.username
 AND EXISTS(SELECT 1 FROM public.demo_seed_step WHERE dataset_id=$1 AND step_key='accounts-v1' AND manifest_digest=$4)
 FOR UPDATE OF a FOR SHARE OF r`, datasetID, username, role, demoseed.ManifestDigest()))
	if errors.Is(err, auth.ErrNotFound) {
		return auth.Account{}, auth.ErrUnauthenticated
	}
	if err != nil {
		return auth.Account{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.auth_session(id,account_id,token_hash,created_at,expires_at) VALUES($1,$2,$3,$4,$5)`, session.ID, a.ID, session.TokenHash, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return auth.Account{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return auth.Account{}, err
	}
	a.PasswordHash = ""
	return a, nil
}
