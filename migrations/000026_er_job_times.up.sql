-- Complete the two JOB field names present in the supplied ER reference.
-- The current model has no separate scheduler estimate, so due_at is the only
-- durable estimated completion timestamp. completed_at is the actual completion
-- timestamp and is cleared by the existing rework transition.
ALTER TABLE job
  ADD COLUMN estimated_time timestamptz,
  ADD COLUMN actual_time timestamptz;

ALTER TABLE demo_workflow_run DROP CONSTRAINT demo_workflow_run_fixture_check;
ALTER TABLE demo_workflow_run ADD CONSTRAINT demo_workflow_run_fixture_check
  CHECK (fixture IN ('workflows-v1', 'workflows-v2', 'workflows-v3'));

UPDATE job
SET estimated_time = due_at,
    actual_time = CASE WHEN status = 'completed' THEN completed_at END;

CREATE OR REPLACE FUNCTION er_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.due_date := NEW.due_at;
  NEW.estimated_time := NEW.due_at;
  NEW.actual_time := CASE WHEN NEW.status = 'completed' THEN NEW.completed_at END;
  RETURN NEW;
END $$;
