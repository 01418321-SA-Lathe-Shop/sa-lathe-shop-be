DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM demo_workflow_run) THEN
  RAISE EXCEPTION 'Workflow seed provenance exists; preserve it, do not downgrade';
 END IF;
END $$;
DROP TABLE demo_workflow_step;
DROP TABLE demo_workflow_run;
