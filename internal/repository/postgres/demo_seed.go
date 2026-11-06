package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Only the CLI composes this repository. No API route exposes seed operations.
type DemoSeedRepository struct{ db *Database }

func NewDemoSeedRepository(db *Database) *DemoSeedRepository { return &DemoSeedRepository{db} }

type demoSeedStore struct {
	*Database
	datasetID   string
	environment string
}

func (r *DemoSeedRepository) Within(ctx context.Context, cfg demoseed.Config, fn func(demoseed.Store) error) error {
	// Recheck even when a caller constructs Config without LoadConfig.
	_, e := demoseed.LoadConfig(func(k string) string {
		switch k {
		case "SEED_ENABLED":
			return "true"
		case "SEED_ENV":
			return cfg.Environment
		case "SEED_CONFIRM_DATABASE":
			return cfg.Database
		case "SEED_DATABASE_URL":
			return cfg.DatabaseURL
		}
		return ""
	})
	if e != nil {
		return e
	}
	tx, e := r.db.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SET LOCAL search_path=public; SET LOCAL lock_timeout='5s'`); e != nil {
		return e
	}
	var name string
	if e = tx.QueryRow(ctx, `SELECT current_database()`).Scan(&name); e != nil {
		return e
	}
	if name != cfg.Database {
		return demoseed.ErrGuard
	}
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(814031913)`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `LOCK TABLE schema_migrations IN SHARE MODE`); e != nil {
		return demoseed.ErrGuard
	}
	var version int
	var dirty bool
	if e = tx.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty); e != nil || (version < 13 || version > 24) || dirty {
		return demoseed.ErrGuard
	}
	// Local operator command only. Freeze public tables against writes so the
	// emptiness check, ownership capture and later cleanup cannot race API writes.
	tables, e := seedTables(ctx, tx)
	if e != nil {
		return e
	}
	for _, table := range tables {
		if _, e = tx.Exec(ctx, "LOCK TABLE "+pgx.Identifier{"public", table}.Sanitize()+" IN SHARE ROW EXCLUSIVE MODE"); e != nil {
			return e
		}
	}
	store := &demoSeedStore{Database: &Database{pool: r.db.pool, identityTx: tx}, environment: cfg.Environment}
	if e = fn(store); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func seedTables(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, e := tx.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename<>'schema_migrations' ORDER BY tablename`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// Explicit safe projection: never include password_hash or invitation/session
// digests. Composite memberships are bound by both UUIDs, not by a name match.
const seedIdentities = `
 SELECT 'account:'||a.username AS record_key,'account' AS entity_type,a.id AS record_id,NULL::uuid AS parent_id,
 jsonb_build_object('id',a.id,'person_id',p.id,'username',a.username,'name',p.name,'role',a.role,'is_active',a.is_active,
 'email',p.email,'phone',p.phone,'admin_profile',EXISTS(SELECT 1 FROM admin x WHERE x.person_id=p.id),
 'worker_profile',CASE WHEN w.person_id IS NULL THEN NULL ELSE jsonb_build_object('employee_no',w.employee_no,'position',w.position) END) AS identity
 FROM account a JOIN person p ON p.id=a.person_id LEFT JOIN worker w ON w.person_id=p.id
 UNION ALL
 SELECT 'customer:'||id,'customer',id,NULL::uuid,jsonb_build_object('id',id,'customer_type',customer_type,'name',name) FROM customer
 UNION ALL
 SELECT 'member:'||account_id||':'||customer_id,'member',account_id,customer_id,
 jsonb_build_object('account_id',account_id,'customer_id',customer_id,'is_owner',is_owner) FROM customer_member
 UNION ALL
 SELECT 'invitation:'||id,'invitation',id,NULL::uuid,
 jsonb_build_object('id',id,'customer_id',customer_id,'created_by',created_by,'expires_at',expires_at,'accepted_at',accepted_at) FROM customer_invitation
`

func (s *demoSeedStore) Inspect(ctx context.Context) (demoseed.Status, error) {
	var status, environment, digest string
	var version int
	e := s.identityTx.QueryRow(ctx, `SELECT id::text,status,environment,fixture_version FROM demo_seed_dataset WHERE name=$1`, demoseed.DatasetName).Scan(&s.datasetID, &status, &environment, &version)
	if errors.Is(e, pgx.ErrNoRows) {
		if e = s.requireEmpty(ctx); e != nil {
			return demoseed.Status{}, e
		}
		return demoseed.Status{State: "uninitialized"}, nil
	}
	if e != nil {
		return demoseed.Status{}, e
	}
	if environment != s.environment || version != demoseed.FixtureVersion {
		return demoseed.Status{}, demoseed.ErrConflict
	}
	if e = s.identityTx.QueryRow(ctx, `SELECT manifest_digest FROM demo_seed_step WHERE dataset_id=$1 AND step_key='accounts-v1'`, s.datasetID).Scan(&digest); e != nil || digest != demoseed.ManifestDigest() {
		return demoseed.Status{}, demoseed.ErrConflict
	}
	var accounts, records int
	var drift bool
	e = s.identityTx.QueryRow(ctx, `WITH current_identity AS (`+seedIdentities+`)
 SELECT count(*) FILTER(WHERE r.entity_type='account'),count(*),COALESCE(bool_or(c.identity IS DISTINCT FROM r.identity),false)
 FROM demo_seed_record r LEFT JOIN current_identity c ON c.entity_type=r.entity_type AND c.record_id=r.record_id AND c.parent_id IS NOT DISTINCT FROM r.parent_id
 WHERE r.dataset_id=$1`, s.datasetID).Scan(&accounts, &records, &drift)
	if e != nil {
		return demoseed.Status{}, e
	}
	if drift || accounts != 6 || records != 12 {
		return demoseed.Status{}, demoseed.ErrConflict
	}
	state := "ready"
	if status == "retired" {
		state = "retired"
	}
	return demoseed.Status{State: state, Accounts: accounts, Records: records}, nil
}

func (s *demoSeedStore) requireEmpty(ctx context.Context) error {
	tables, e := seedTables(ctx, s.identityTx)
	if e != nil {
		return e
	}
	for _, table := range tables {
		var occupied bool
		if e = s.identityTx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+pgx.Identifier{"public", table}.Sanitize()+" LIMIT 1)").Scan(&occupied); e != nil {
			return e
		}
		if occupied {
			return fmt.Errorf("%w: initial database must have no application records", demoseed.ErrConflict)
		}
	}
	return nil
}

func (s *demoSeedStore) Initialize(ctx context.Context) error {
	if e := s.requireEmpty(ctx); e != nil {
		return e
	}
	s.datasetID = uuid.NewString()
	_, e := s.identityTx.Exec(ctx, `INSERT INTO demo_seed_dataset(id,name,fixture_version,environment) VALUES($1,$2,$3,$4)`, s.datasetID, demoseed.DatasetName, demoseed.FixtureVersion, s.environment)
	return e
}

func (s *demoSeedStore) Capture(ctx context.Context) error {
	tag, e := s.identityTx.Exec(ctx, `INSERT INTO demo_seed_record(dataset_id,record_key,entity_type,record_id,parent_id,identity) SELECT $1,x.* FROM (`+seedIdentities+`) x`, s.datasetID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 12 {
		return demoseed.ErrConflict
	}
	_, e = s.identityTx.Exec(ctx, `INSERT INTO demo_seed_step(dataset_id,step_key,manifest_digest) VALUES($1,'accounts-v1',$2)`, s.datasetID, demoseed.ManifestDigest())
	return e
}
