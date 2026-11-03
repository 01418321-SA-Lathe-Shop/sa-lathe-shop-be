-- Allow a second additive demo workflow set while preserving the immutable v1 journal.
ALTER TABLE demo_workflow_run DROP CONSTRAINT demo_workflow_run_fixture_check;
ALTER TABLE demo_workflow_run ADD CONSTRAINT demo_workflow_run_fixture_check
 CHECK(fixture IN ('workflows-v1','workflows-v2'));
