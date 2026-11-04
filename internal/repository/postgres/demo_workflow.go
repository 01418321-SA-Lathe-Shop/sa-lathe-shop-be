package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
	wf "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoworkflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowRepository struct{ db *Database }

func NewWorkflowRepository(db *Database) *WorkflowRepository { return &WorkflowRepository{db} }

type workflowStore struct {
	conn  *pgxpool.Conn
	runID string
}

func (r *WorkflowRepository) Within(ctx context.Context, c wf.Config, create bool, fn func(wf.Store, []auth.Account, bool, time.Time) error) error {
	conn, e := r.db.pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	var locked bool
	if e = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(814031914)`).Scan(&locked); e != nil {
		return e
	}
	if !locked {
		return wf.ErrGuard
	}
	defer func() {
		unlock, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlock, `SELECT pg_advisory_unlock(814031914)`)
	}()
	var accounts []auth.Account
	e = NewDemoSeedRepository(r.db).Within(ctx, c.Seed, func(store demoseed.Store) error {
		state, e := store.Inspect(ctx)
		if e != nil || state.State != "ready" {
			return wf.ErrGuard
		}
		for _, spec := range demoseed.Accounts() {
			a, e := store.FindAccount(ctx, spec.Username)
			if e != nil {
				return e
			}
			a.PasswordHash = ""
			accounts = append(accounts, a)
		}
		return nil
	})
	if e != nil {
		return e
	}
	var version int
	var dirty bool
	if e = conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); e != nil || (version < 19 || version > 22) || dirty {
		return wf.ErrGuard
	}
	if create {
		_, e = conn.Exec(ctx, `INSERT INTO public.demo_workflow_run(id,dataset_id,fixture) SELECT $1,id,$3 FROM public.demo_seed_dataset WHERE name=$2 AND status='active' ON CONFLICT(dataset_id,fixture) DO NOTHING`, uuid.NewString(), demoseed.DatasetName, wf.FixtureName)
		if e != nil {
			return e
		}
	}
	s := &workflowStore{conn: conn}
	var status string
	var started time.Time
	e = conn.QueryRow(ctx, `SELECT r.id::text,r.status,r.created_at FROM public.demo_workflow_run r JOIN public.demo_seed_dataset d ON d.id=r.dataset_id WHERE d.name=$1 AND d.status='active' AND r.fixture=$2`, demoseed.DatasetName, wf.FixtureName).Scan(&s.runID, &status, &started)
	if errors.Is(e, pgx.ErrNoRows) {
		return wf.ErrGuard
	}
	if e != nil {
		return e
	}
	return fn(s, accounts, status == "complete", started)
}
func (s *workflowStore) Prepare(ctx context.Context, op wf.Operation) (wf.Step, error) {
	raw, e := json.Marshal(op.Body)
	if e != nil {
		return wf.Step{}, e
	}
	tag, e := s.conn.Exec(ctx, `INSERT INTO public.demo_workflow_step(run_id,step_key,actor,method,path,request,safe_retry) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, s.runID, op.Key, op.Actor, op.Method, op.Path, raw, op.SafeRetry)
	if e != nil {
		return wf.Step{}, e
	}
	step := wf.Step{Existing: tag.RowsAffected() == 0}
	var body, result []byte
	e = s.conn.QueryRow(ctx, `SELECT actor,method,path,request,safe_retry,result FROM public.demo_workflow_step WHERE run_id=$1 AND step_key=$2`, s.runID, op.Key).Scan(&step.Actor, &step.Method, &step.Path, &body, &step.SafeRetry, &result)
	if e != nil {
		return step, e
	}
	step.Key = op.Key
	if step.Actor != op.Actor || step.Method != op.Method || step.Path != op.Path || step.SafeRetry != op.SafeRetry {
		return step, wf.ErrGuard
	}
	if e = json.Unmarshal(body, &step.Body); e != nil {
		return step, e
	}
	if len(result) > 0 {
		e = json.Unmarshal(result, &step.Result)
	}
	return step, e
}
func (s *workflowStore) Complete(ctx context.Context, key string, result wf.Object) error {
	raw, e := json.Marshal(result)
	if e != nil {
		return e
	}
	tag, e := s.conn.Exec(ctx, `UPDATE public.demo_workflow_step SET result=$3,completed_at=clock_timestamp() WHERE run_id=$1 AND step_key=$2 AND result IS NULL`, s.runID, key, raw)
	if e == nil && tag.RowsAffected() != 1 {
		return wf.ErrGuard
	}
	return e
}
func (s *workflowStore) Finish(ctx context.Context) error {
	tag, e := s.conn.Exec(ctx, `UPDATE public.demo_workflow_run r SET status='complete',completed_at=clock_timestamp() WHERE r.id=$1 AND NOT EXISTS(SELECT 1 FROM public.demo_workflow_step s WHERE s.run_id=r.id AND s.result IS NULL)`, s.runID)
	if e == nil && tag.RowsAffected() != 1 {
		return wf.ErrGuard
	}
	return e
}
func (s *workflowStore) Results(ctx context.Context) (map[string]wf.Object, error) {
	rows, e := s.conn.Query(ctx, `SELECT step_key,result FROM public.demo_workflow_step WHERE run_id=$1 AND result IS NOT NULL`, s.runID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := map[string]wf.Object{}
	for rows.Next() {
		var key string
		var raw []byte
		if e = rows.Scan(&key, &raw); e != nil {
			return nil, e
		}
		var v wf.Object
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		result[key] = v
	}
	return result, rows.Err()
}
