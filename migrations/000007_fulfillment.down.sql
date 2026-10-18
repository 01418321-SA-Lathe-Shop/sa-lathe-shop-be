DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM term_evidence) OR EXISTS(SELECT 1 FROM job) OR EXISTS(SELECT 1 FROM uc4_cancellation) THEN
  RAISE EXCEPTION 'UC4 data exists; backup and use a forward migration';
 END IF;
END $$;
DROP TRIGGER queued_job_contract ON job;
DROP FUNCTION validate_queued_job();
DROP TRIGGER term_evidence_contract ON term_evidence;
DROP FUNCTION validate_term_evidence();
DROP TABLE job;
DROP TABLE uc4_cancellation;
DROP TABLE term_evidence;
