DROP VIEW term_evidence;
ALTER TABLE job DROP CONSTRAINT job_deposit_evidence_id_quotation_id_deposit_kind_evidence_fkey;
ALTER TABLE job DROP CONSTRAINT job_po_evidence_id_quotation_id_po_kind_evidence_status_fkey;
ALTER TABLE material_reservation DROP CONSTRAINT material_reservation_id_key;
ALTER TABLE material_reservation DROP COLUMN id;
ALTER TABLE address DROP COLUMN edited_at;
ALTER TABLE address RENAME TO customer_address;
ALTER TABLE payment RENAME TO customer_payment;
ALTER TABLE material_defect_log RENAME TO material_claim;
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
CREATE OR REPLACE FUNCTION public.check_customer_payment()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE p customer_payment;
BEGIN
 SELECT * INTO p FROM customer_payment WHERE id=NEW.id;
 IF (p.status='approved') IS DISTINCT FROM EXISTS(SELECT 1 FROM payment_allocation WHERE payment_id=p.id AND invoice_id=p.invoice_id AND amount=p.amount) THEN RAISE EXCEPTION 'Payment allocation audit required' USING ERRCODE='23514'; END IF;
 IF NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=p.id AND actor_id=p.submitted_by AND action=p.method) OR (p.review_event_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=p.review_event_id AND actor_id=p.reviewed_by AND action IN ('review','cash'))) THEN RAISE EXCEPTION 'Payment command audit required' USING ERRCODE='23514'; END IF;RETURN NULL;
END $function$;

CREATE OR REPLACE FUNCTION public.validate_queued_job()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
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
END $function$;

CREATE OR REPLACE FUNCTION public.guard_receipt_line()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE remaining numeric; parent_order uuid; c material_claim;
BEGIN
 SELECT order_id INTO parent_order FROM material_receipt WHERE id=NEW.receipt_id;
 IF NOT EXISTS(SELECT 1 FROM material_receipt WHERE id=NEW.receipt_id AND created_tx=txid_current()) THEN RAISE EXCEPTION 'Receipt is already finalized' USING ERRCODE='23514'; END IF;
 IF parent_order IS DISTINCT FROM NEW.order_id THEN RAISE EXCEPTION 'Receipt order mismatch' USING ERRCODE='23514'; END IF;
 IF NEW.replacement_claim_id IS NULL THEN
  SELECT i.qty_ordered-COALESCE((SELECT sum(good+damaged) FROM material_receipt_line l WHERE l.order_id=NEW.order_id AND l.material_id=NEW.material_id AND replacement_claim_id IS NULL),0)
   INTO remaining FROM material_order_item i WHERE i.order_id=NEW.order_id AND i.material_id=NEW.material_id;
 ELSE
  SELECT cl.* INTO c FROM material_claim cl JOIN material_receipt_line src ON src.id=cl.source_line_id
   WHERE cl.id=NEW.replacement_claim_id AND src.order_id=NEW.order_id AND src.material_id=NEW.material_id;
  IF c.id IS NULL OR c.status<>'claimed' THEN RAISE EXCEPTION 'Replacement requires an open claimed defect' USING ERRCODE='23514'; END IF;
  SELECT c.quantity-COALESCE(sum(good),0) INTO remaining FROM material_receipt_line WHERE replacement_claim_id=c.id;
 END IF;
 IF remaining IS NULL OR NEW.good+NEW.damaged>remaining THEN RAISE EXCEPTION 'Excess delivery' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $function$;

CREATE OR REPLACE FUNCTION public.check_material_receipt()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM material_order_event e JOIN material_order o ON o.id=e.order_id WHERE e.id=NEW.id AND e.order_id=NEW.order_id AND e.action='receive' AND e.version=o.version) THEN
  RAISE EXCEPTION 'Receipt requires its current order event' USING ERRCODE='23514'; END IF;
 IF EXISTS(SELECT 1 FROM material_claim c JOIN material_receipt_line src ON src.id=c.source_line_id WHERE src.order_id=NEW.order_id AND c.replaced<>(SELECT COALESCE(sum(l.good),0) FROM material_receipt_line l WHERE l.replacement_claim_id=c.id)) THEN
  RAISE EXCEPTION 'Claim totals must reconcile with good replacements' USING ERRCODE='23514'; END IF;
 IF NOT EXISTS(SELECT 1 FROM material_receipt_line WHERE receipt_id=NEW.id) OR EXISTS(
  SELECT 1 FROM material_receipt_line l WHERE l.receipt_id=NEW.id AND
  ((l.good>0 AND NOT EXISTS(SELECT 1 FROM stock_movement s WHERE s.receipt_line_id=l.id))
   OR (l.replacement_claim_id IS NULL AND l.damaged>0 AND NOT EXISTS(SELECT 1 FROM material_claim c WHERE c.source_line_id=l.id AND c.quantity=l.damaged)))) THEN
  RAISE EXCEPTION 'Receipt requires complete ledger/defect audit' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $function$;

CREATE OR REPLACE FUNCTION public.guard_payment_allocation()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE p customer_payment; total numeric; received numeric;
BEGIN
 SELECT amount INTO total FROM invoice WHERE id=NEW.invoice_id FOR UPDATE;
 SELECT * INTO p FROM customer_payment WHERE id=NEW.payment_id;
 SELECT COALESCE(sum(amount),0) INTO received FROM payment_allocation WHERE invoice_id=NEW.invoice_id;
 IF p.status<>'approved' OR p.invoice_id<>NEW.invoice_id OR p.amount<>NEW.amount OR received+NEW.amount>total THEN RAISE EXCEPTION 'Exact approved payment allocation required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $function$;

ALTER TABLE term_evidence DISABLE TRIGGER term_evidence_contract;
INSERT INTO term_evidence(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,amount,paid_at,status,version,submitted_at,reviewed_by,reviewed_at,reason)
SELECT id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,amount,paid_at,status,version,submitted_at,reviewed_by,reviewed_at,reason FROM deposit
UNION ALL
SELECT id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,NULL,NULL,status,version,submitted_at,reviewed_by,reviewed_at,reason FROM purchase_order;
ALTER TABLE term_evidence ENABLE TRIGGER term_evidence_contract;
DROP TABLE deposit;
DROP TABLE purchase_order;
DROP FUNCTION validate_deposit();
DROP FUNCTION validate_purchase_order();
ALTER TABLE job ADD CONSTRAINT job_deposit_evidence_id_quotation_id_deposit_kind_evidence_fkey
 FOREIGN KEY(deposit_evidence_id,quotation_id,deposit_kind,evidence_status) REFERENCES term_evidence(id,quotation_id,kind,status) ON DELETE RESTRICT;
ALTER TABLE job ADD CONSTRAINT job_po_evidence_id_quotation_id_po_kind_evidence_status_fkey
 FOREIGN KEY(po_evidence_id,quotation_id,po_kind,evidence_status) REFERENCES term_evidence(id,quotation_id,kind,status) ON DELETE RESTRICT;
COMMENT ON TABLE term_evidence IS 'UC4 immutable submission plus one staff review; ready private attachments are not automatically payments';
