-- Durable local demo operation journal, never business data or credentials.
CREATE TABLE demo_workflow_run (
 id uuid PRIMARY KEY,
 dataset_id uuid NOT NULL REFERENCES demo_seed_dataset(id) ON DELETE RESTRICT,
 fixture text NOT NULL CHECK(fixture='workflows-v1'),
 status text NOT NULL DEFAULT 'building' CHECK(status IN ('building','complete')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 UNIQUE(dataset_id,fixture),
 CHECK((status='complete')=(completed_at IS NOT NULL))
);
CREATE TABLE demo_workflow_step (
 run_id uuid NOT NULL REFERENCES demo_workflow_run(id) ON DELETE RESTRICT,
 step_key text NOT NULL,
 actor text NOT NULL,
 method text NOT NULL CHECK(method IN ('POST','PUT','UPLOAD','GET')),
 path text NOT NULL,
 request jsonb NOT NULL CHECK(jsonb_typeof(request)='object'),
 safe_retry boolean NOT NULL,
 result jsonb CHECK(jsonb_typeof(result)='object'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 PRIMARY KEY(run_id,step_key),
 CHECK((result IS NULL)=(completed_at IS NULL))
);
COMMENT ON TABLE demo_workflow_step IS 'Seed-owned operation results/UUID roots and exact safe retry input; never tokens, passwords, signed URLs or file bytes. No destructive reset.';
