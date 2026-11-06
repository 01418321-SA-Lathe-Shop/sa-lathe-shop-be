CREATE OR REPLACE FUNCTION er_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.due_date := NEW.due_at;
  RETURN NEW;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM demo_workflow_run WHERE fixture = 'workflows-v3') THEN
    RAISE EXCEPTION 'Refusing to remove workflows-v3 provenance; preserve the ER-complete fixture';
  END IF;
END $$;

ALTER TABLE demo_workflow_run DROP CONSTRAINT demo_workflow_run_fixture_check;
ALTER TABLE demo_workflow_run ADD CONSTRAINT demo_workflow_run_fixture_check
  CHECK (fixture IN ('workflows-v1', 'workflows-v2'));

ALTER TABLE job
  DROP COLUMN actual_time,
  DROP COLUMN estimated_time;
