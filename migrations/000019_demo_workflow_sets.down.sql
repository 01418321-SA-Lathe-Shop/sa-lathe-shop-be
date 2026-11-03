DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM demo_workflow_run WHERE fixture='workflows-v2') THEN
  RAISE EXCEPTION 'Refusing to remove workflows-v2 provenance; preserve the additive fixture';
 END IF;
END $$;
ALTER TABLE demo_workflow_run DROP CONSTRAINT demo_workflow_run_fixture_check;
ALTER TABLE demo_workflow_run ADD CONSTRAINT demo_workflow_run_fixture_check
 CHECK(fixture='workflows-v1');
