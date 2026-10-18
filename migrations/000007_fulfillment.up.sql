CREATE TABLE term_evidence (
 id uuid PRIMARY KEY,
 quotation_id uuid NOT NULL REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 attachment_id uuid NOT NULL UNIQUE REFERENCES request_attachment(id) ON DELETE RESTRICT,
 submitted_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 idempotency_key uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('deposit','po')),
 amount numeric(18,2) CHECK(amount>=0 AND amount<>'NaN'::numeric),
 paid_at timestamptz,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
 version bigint NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 reviewed_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 reviewed_at timestamptz,
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),
 UNIQUE(quotation_id,submitted_by,idempotency_key),
 UNIQUE(id,quotation_id,kind,status),
 CHECK((kind='deposit' AND amount IS NOT NULL AND paid_at IS NOT NULL AND paid_at<=submitted_at)
 OR (kind='po' AND amount IS NULL AND paid_at IS NULL)),
 CHECK((status='pending' AND version=1 AND reviewed_by IS NULL AND reviewed_at IS NULL AND reason='')
 OR (status<>'pending' AND version=2 AND reviewed_by IS NOT NULL AND reviewed_at>=submitted_at
 AND ((status='approved' AND reason='') OR (status='rejected' AND length(btrim(reason))>0))))
);
CREATE UNIQUE INDEX term_evidence_one_active ON term_evidence(quotation_id,kind) WHERE status IN ('pending','approved');
CREATE INDEX term_evidence_history ON term_evidence(quotation_id,submitted_at DESC,id);

CREATE TABLE job (
 id uuid PRIMARY KEY,
 quotation_id uuid NOT NULL UNIQUE REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 request_id uuid NOT NULL UNIQUE REFERENCES job_request(id) ON DELETE RESTRICT,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','in_progress','completed','cancelled')),
 deposit_amount numeric(18,2) NOT NULL CHECK(deposit_amount>=0 AND deposit_amount<>'NaN'::numeric),
 balance numeric(18,2) NOT NULL CHECK(balance>=0 AND balance<>'NaN'::numeric),
 deposit_evidence_id uuid,
 deposit_kind text NOT NULL DEFAULT 'deposit' CHECK(deposit_kind='deposit'),
 po_evidence_id uuid,
 po_kind text NOT NULL DEFAULT 'po' CHECK(po_kind='po'),
 evidence_status text NOT NULL DEFAULT 'approved' CHECK(evidence_status='approved'),
 due_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(deposit_evidence_id,quotation_id,deposit_kind,evidence_status) REFERENCES term_evidence(id,quotation_id,kind,status) ON DELETE RESTRICT,
 FOREIGN KEY(po_evidence_id,quotation_id,po_kind,evidence_status) REFERENCES term_evidence(id,quotation_id,kind,status) ON DELETE RESTRICT
);
CREATE TABLE uc4_cancellation (
 quotation_id uuid PRIMARY KEY REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 request_id uuid NOT NULL UNIQUE REFERENCES job_request(id) ON DELETE RESTRICT,
 canceled_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 reason text NOT NULL DEFAULT 'missing_required_evidence_at_deadline' CHECK(reason='missing_required_evidence_at_deadline')
);

-- Cross-table contract checks are enforced as well as application transaction guards.
CREATE FUNCTION validate_term_evidence() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t quotation_acceptance_terms; a request_attachment; r job_request;
BEGIN
 SELECT * INTO t FROM quotation_acceptance_terms WHERE quotation_id=NEW.quotation_id;
 SELECT jr.* INTO r FROM job_request jr JOIN quotation q ON q.request_id=jr.id WHERE q.id=NEW.quotation_id FOR UPDATE OF jr;
 IF TG_OP='UPDATE' THEN
  IF OLD.status<>'pending' OR NEW.status='pending' OR
   (NEW.id,NEW.quotation_id,NEW.attachment_id,NEW.submitted_by,NEW.idempotency_key,NEW.kind,NEW.amount,NEW.paid_at,NEW.submitted_at)
   IS DISTINCT FROM (OLD.id,OLD.quotation_id,OLD.attachment_id,OLD.submitted_by,OLD.idempotency_key,OLD.kind,OLD.amount,OLD.paid_at,OLD.submitted_at) THEN
   RAISE EXCEPTION 'Evidence audit is immutable' USING ERRCODE='23514';
  END IF;
 END IF;
 SELECT * INTO a FROM request_attachment WHERE id=NEW.attachment_id;
 IF r.status<>'ongoing' OR t.quotation_id IS NULL OR NEW.submitted_at>t.documents_due_at
 OR a.id IS NULL OR a.request_id<>r.id OR a.uploaded_by<>NEW.submitted_by OR a.state<>'ready'
 OR a.completed_at>NEW.submitted_at
 OR (NEW.kind='deposit' AND (NOT t.deposit_required OR NEW.amount<t.deposit_amount OR NEW.amount>t.final_price))
 OR (NEW.kind='po' AND NOT t.po_required) THEN
  RAISE EXCEPTION 'Evidence does not fulfill the accepted contract' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER term_evidence_contract BEFORE INSERT OR UPDATE ON term_evidence FOR EACH ROW EXECUTE FUNCTION validate_term_evidence();

CREATE FUNCTION validate_queued_job() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t quotation_acceptance_terms; r job_request; deposited numeric(18,2);
BEGIN
 SELECT * INTO t FROM quotation_acceptance_terms WHERE quotation_id=NEW.quotation_id;
 SELECT jr.* INTO r FROM job_request jr JOIN quotation q ON q.request_id=jr.id WHERE q.id=NEW.quotation_id FOR UPDATE OF jr;
 IF r.id IS NULL OR r.id<>NEW.request_id OR r.status<>'ongoing' OR NEW.status<>'queued'
 OR (t.deposit_required AND NEW.deposit_evidence_id IS NULL)
 OR (t.po_required AND NEW.po_evidence_id IS NULL)
 OR (NOT t.deposit_required AND NEW.deposit_evidence_id IS NOT NULL)
 OR (NOT t.po_required AND NEW.po_evidence_id IS NOT NULL)
 OR EXISTS(SELECT 1 FROM uc4_cancellation WHERE quotation_id=NEW.quotation_id) THEN
  RAISE EXCEPTION 'Job requirements are not fulfilled' USING ERRCODE='23514';
 END IF;
 SELECT amount INTO deposited FROM term_evidence WHERE id=NEW.deposit_evidence_id;
 IF NEW.deposit_amount<>COALESCE(deposited,0) OR NEW.balance<>t.final_price-COALESCE(deposited,0) THEN
  RAISE EXCEPTION 'Job payment snapshot is inconsistent' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER queued_job_contract BEFORE INSERT ON job FOR EACH ROW EXECUTE FUNCTION validate_queued_job();
COMMENT ON TABLE term_evidence IS 'UC4 immutable submission plus one staff review; ready private attachments are not automatically payments';
COMMENT ON TABLE job IS 'Exactly one queued job per accepted quotation/request, opened atomically after required evidence approvals';
COMMENT ON COLUMN job.deposit_amount IS 'Staff-verified actual deposit, not the minimum deposit in accepted terms';
COMMENT ON TABLE uc4_cancellation IS 'System deadline cancellation audit; timely complete pending evidence prevents expiry';
