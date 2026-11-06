-- ER alignment, phase 1: rename three tables to their ER entity names and split term_evidence into
-- DEPOSIT and PURCHASE_ORDER. Behaviour, constraints and data are unchanged. A read-only view named
-- term_evidence (union of the two) keeps the old read queries working.

ALTER TABLE customer_payment RENAME TO payment;
ALTER TABLE customer_address RENAME TO address;
ALTER TABLE material_claim RENAME TO material_defect_log;

-- ER attributes with an unambiguous source.
ALTER TABLE address ADD COLUMN edited_at timestamptz;
ALTER TABLE material_reservation ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE material_reservation ADD CONSTRAINT material_reservation_id_key UNIQUE (id);

-- Trigger functions that mention a renamed or split table by name.
CREATE OR REPLACE FUNCTION public.check_customer_payment()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE p payment;
BEGIN
 SELECT * INTO p FROM payment WHERE id=NEW.id;
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
 SELECT amount INTO deposited FROM deposit WHERE id=NEW.deposit_evidence_id;
 IF NEW.deposit_amount<>COALESCE(deposited,0) OR NEW.balance<>t.final_price-COALESCE(deposited,0) THEN
  RAISE EXCEPTION 'Job payment snapshot is inconsistent' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $function$;

CREATE OR REPLACE FUNCTION public.guard_receipt_line()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE remaining numeric; parent_order uuid; c material_defect_log;
BEGIN
 SELECT order_id INTO parent_order FROM material_receipt WHERE id=NEW.receipt_id;
 IF NOT EXISTS(SELECT 1 FROM material_receipt WHERE id=NEW.receipt_id AND created_tx=txid_current()) THEN RAISE EXCEPTION 'Receipt is already finalized' USING ERRCODE='23514'; END IF;
 IF parent_order IS DISTINCT FROM NEW.order_id THEN RAISE EXCEPTION 'Receipt order mismatch' USING ERRCODE='23514'; END IF;
 IF NEW.replacement_claim_id IS NULL THEN
  SELECT i.qty_ordered-COALESCE((SELECT sum(good+damaged) FROM material_receipt_line l WHERE l.order_id=NEW.order_id AND l.material_id=NEW.material_id AND replacement_claim_id IS NULL),0)
   INTO remaining FROM material_order_item i WHERE i.order_id=NEW.order_id AND i.material_id=NEW.material_id;
 ELSE
  SELECT cl.* INTO c FROM material_defect_log cl JOIN material_receipt_line src ON src.id=cl.source_line_id
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
 IF EXISTS(SELECT 1 FROM material_defect_log c JOIN material_receipt_line src ON src.id=c.source_line_id WHERE src.order_id=NEW.order_id AND c.replaced<>(SELECT COALESCE(sum(l.good),0) FROM material_receipt_line l WHERE l.replacement_claim_id=c.id)) THEN
  RAISE EXCEPTION 'Claim totals must reconcile with good replacements' USING ERRCODE='23514'; END IF;
 IF NOT EXISTS(SELECT 1 FROM material_receipt_line WHERE receipt_id=NEW.id) OR EXISTS(
  SELECT 1 FROM material_receipt_line l WHERE l.receipt_id=NEW.id AND
  ((l.good>0 AND NOT EXISTS(SELECT 1 FROM stock_movement s WHERE s.receipt_line_id=l.id))
   OR (l.replacement_claim_id IS NULL AND l.damaged>0 AND NOT EXISTS(SELECT 1 FROM material_defect_log c WHERE c.source_line_id=l.id AND c.quantity=l.damaged)))) THEN
  RAISE EXCEPTION 'Receipt requires complete ledger/defect audit' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $function$;

CREATE OR REPLACE FUNCTION public.guard_payment_allocation()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE p payment; total numeric; received numeric;
BEGIN
 SELECT amount INTO total FROM invoice WHERE id=NEW.invoice_id FOR UPDATE;
 SELECT * INTO p FROM payment WHERE id=NEW.payment_id;
 SELECT COALESCE(sum(amount),0) INTO received FROM payment_allocation WHERE invoice_id=NEW.invoice_id;
 IF p.status<>'approved' OR p.invoice_id<>NEW.invoice_id OR p.amount<>NEW.amount OR received+NEW.amount>total THEN RAISE EXCEPTION 'Exact approved payment allocation required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $function$;


-- DEPOSIT and PURCHASE_ORDER replace term_evidence (kind deposit / po).
CREATE TABLE deposit (
 id uuid PRIMARY KEY,
 quotation_id uuid NOT NULL REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 attachment_id uuid NOT NULL UNIQUE REFERENCES request_attachment(id) ON DELETE RESTRICT,
 submitted_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 idempotency_key uuid NOT NULL,
 kind text NOT NULL DEFAULT 'deposit' CHECK(kind='deposit'),
 amount numeric(18,2) NOT NULL CHECK(amount>=0 AND amount<>'NaN'::numeric),
 paid_at timestamptz NOT NULL,
 percent numeric(7,2),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
 version bigint NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 reviewed_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 reviewed_at timestamptz,
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),
 UNIQUE(quotation_id,submitted_by,idempotency_key),
 UNIQUE(id,quotation_id,kind,status),
 CHECK(paid_at<=submitted_at),
 CHECK((status='pending' AND version=1 AND reviewed_by IS NULL AND reviewed_at IS NULL AND reason='')
 OR (status<>'pending' AND version=2 AND reviewed_by IS NOT NULL AND reviewed_at>=submitted_at
 AND ((status='approved' AND reason='') OR (status='rejected' AND length(btrim(reason))>0))))
);
CREATE TABLE purchase_order (
 id uuid PRIMARY KEY,
 quotation_id uuid NOT NULL REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 attachment_id uuid NOT NULL UNIQUE REFERENCES request_attachment(id) ON DELETE RESTRICT,
 submitted_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 idempotency_key uuid NOT NULL,
 kind text NOT NULL DEFAULT 'po' CHECK(kind='po'),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
 version bigint NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 reviewed_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 reviewed_at timestamptz,
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),
 UNIQUE(quotation_id,submitted_by,idempotency_key),
 UNIQUE(id,quotation_id,kind,status),
 CHECK((status='pending' AND version=1 AND reviewed_by IS NULL AND reviewed_at IS NULL AND reason='')
 OR (status<>'pending' AND version=2 AND reviewed_by IS NOT NULL AND reviewed_at>=submitted_at
 AND ((status='approved' AND reason='') OR (status='rejected' AND length(btrim(reason))>0))))
);
CREATE UNIQUE INDEX deposit_one_active ON deposit(quotation_id) WHERE status IN ('pending','approved');
CREATE INDEX deposit_history ON deposit(quotation_id,submitted_at DESC,id);
CREATE UNIQUE INDEX purchase_order_one_active ON purchase_order(quotation_id) WHERE status IN ('pending','approved');
CREATE INDEX purchase_order_history ON purchase_order(quotation_id,submitted_at DESC,id);

INSERT INTO deposit(id,quotation_id,attachment_id,submitted_by,idempotency_key,amount,paid_at,percent,status,version,submitted_at,reviewed_by,reviewed_at,reason)
SELECT e.id,e.quotation_id,e.attachment_id,e.submitted_by,e.idempotency_key,e.amount,e.paid_at,
       CASE WHEN t.final_price>0 THEN round(e.amount/t.final_price*100,2) END,e.status,e.version,e.submitted_at,e.reviewed_by,e.reviewed_at,e.reason
FROM term_evidence e JOIN quotation_acceptance_terms t ON t.quotation_id=e.quotation_id WHERE e.kind='deposit';
INSERT INTO purchase_order(id,quotation_id,attachment_id,submitted_by,idempotency_key,status,version,submitted_at,reviewed_by,reviewed_at,reason)
SELECT id,quotation_id,attachment_id,submitted_by,idempotency_key,status,version,submitted_at,reviewed_by,reviewed_at,reason
FROM term_evidence WHERE kind='po';

ALTER TABLE job DROP CONSTRAINT job_deposit_evidence_id_quotation_id_deposit_kind_evidence_fkey;
ALTER TABLE job DROP CONSTRAINT job_po_evidence_id_quotation_id_po_kind_evidence_status_fkey;
DROP TABLE term_evidence;
DROP FUNCTION validate_term_evidence();
ALTER TABLE job ADD CONSTRAINT job_deposit_evidence_id_quotation_id_deposit_kind_evidence_fkey
 FOREIGN KEY(deposit_evidence_id,quotation_id,deposit_kind,evidence_status) REFERENCES deposit(id,quotation_id,kind,status) ON DELETE RESTRICT;
ALTER TABLE job ADD CONSTRAINT job_po_evidence_id_quotation_id_po_kind_evidence_status_fkey
 FOREIGN KEY(po_evidence_id,quotation_id,po_kind,evidence_status) REFERENCES purchase_order(id,quotation_id,kind,status) ON DELETE RESTRICT;

-- Same contract checks as the old term_evidence trigger, plus the cross-table uniqueness the single table gave.
CREATE FUNCTION validate_deposit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t quotation_acceptance_terms; a request_attachment; r job_request;
BEGIN
 SELECT * INTO t FROM quotation_acceptance_terms WHERE quotation_id=NEW.quotation_id;
 SELECT jr.* INTO r FROM job_request jr JOIN quotation q ON q.request_id=jr.id WHERE q.id=NEW.quotation_id FOR UPDATE OF jr;
 IF TG_OP='UPDATE' THEN
  IF OLD.status<>'pending' OR NEW.status='pending' OR
   (NEW.id,NEW.quotation_id,NEW.attachment_id,NEW.submitted_by,NEW.idempotency_key,NEW.kind,NEW.amount,NEW.paid_at,NEW.percent,NEW.submitted_at)
   IS DISTINCT FROM (OLD.id,OLD.quotation_id,OLD.attachment_id,OLD.submitted_by,OLD.idempotency_key,OLD.kind,OLD.amount,OLD.paid_at,OLD.percent,OLD.submitted_at) THEN
   RAISE EXCEPTION 'Evidence audit is immutable' USING ERRCODE='23514';
  END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM purchase_order o WHERE o.attachment_id=NEW.attachment_id OR (o.quotation_id=NEW.quotation_id AND o.submitted_by=NEW.submitted_by AND o.idempotency_key=NEW.idempotency_key)) THEN
   RAISE EXCEPTION 'Evidence attachment or idempotency key already used' USING ERRCODE='23505';
  END IF;
  IF t.final_price>0 THEN NEW.percent := round(NEW.amount/t.final_price*100,2); END IF;
 END IF;
 SELECT * INTO a FROM request_attachment WHERE id=NEW.attachment_id;
 IF r.status<>'ongoing' OR t.quotation_id IS NULL OR NEW.submitted_at>t.documents_due_at
 OR a.id IS NULL OR a.request_id<>r.id OR a.uploaded_by<>NEW.submitted_by OR a.state<>'ready'
 OR a.completed_at>NEW.submitted_at
 OR NOT t.deposit_required OR NEW.amount<t.deposit_amount OR NEW.amount>t.final_price THEN
  RAISE EXCEPTION 'Evidence does not fulfill the accepted contract' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER deposit_contract BEFORE INSERT OR UPDATE ON deposit FOR EACH ROW EXECUTE FUNCTION validate_deposit();

CREATE FUNCTION validate_purchase_order() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t quotation_acceptance_terms; a request_attachment; r job_request;
BEGIN
 SELECT * INTO t FROM quotation_acceptance_terms WHERE quotation_id=NEW.quotation_id;
 SELECT jr.* INTO r FROM job_request jr JOIN quotation q ON q.request_id=jr.id WHERE q.id=NEW.quotation_id FOR UPDATE OF jr;
 IF TG_OP='UPDATE' THEN
  IF OLD.status<>'pending' OR NEW.status='pending' OR
   (NEW.id,NEW.quotation_id,NEW.attachment_id,NEW.submitted_by,NEW.idempotency_key,NEW.kind,NEW.submitted_at)
   IS DISTINCT FROM (OLD.id,OLD.quotation_id,OLD.attachment_id,OLD.submitted_by,OLD.idempotency_key,OLD.kind,OLD.submitted_at) THEN
   RAISE EXCEPTION 'Evidence audit is immutable' USING ERRCODE='23514';
  END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM deposit d WHERE d.attachment_id=NEW.attachment_id OR (d.quotation_id=NEW.quotation_id AND d.submitted_by=NEW.submitted_by AND d.idempotency_key=NEW.idempotency_key)) THEN
   RAISE EXCEPTION 'Evidence attachment or idempotency key already used' USING ERRCODE='23505';
  END IF;
 END IF;
 SELECT * INTO a FROM request_attachment WHERE id=NEW.attachment_id;
 IF r.status<>'ongoing' OR t.quotation_id IS NULL OR NEW.submitted_at>t.documents_due_at
 OR a.id IS NULL OR a.request_id<>r.id OR a.uploaded_by<>NEW.submitted_by OR a.state<>'ready'
 OR a.completed_at>NEW.submitted_at
 OR NOT t.po_required THEN
  RAISE EXCEPTION 'Evidence does not fulfill the accepted contract' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER purchase_order_contract BEFORE INSERT OR UPDATE ON purchase_order FOR EACH ROW EXECUTE FUNCTION validate_purchase_order();

-- Read-only compatibility view for queries that look evidence up without knowing its kind.
CREATE VIEW term_evidence AS
SELECT id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,amount,paid_at,status,version,submitted_at,reviewed_by,reviewed_at,reason FROM deposit
UNION ALL
SELECT id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,NULL::numeric(18,2),NULL::timestamptz,status,version,submitted_at,reviewed_by,reviewed_at,reason FROM purchase_order;
COMMENT ON TABLE deposit IS 'ER DEPOSIT: customer deposit evidence plus one staff review';
COMMENT ON TABLE purchase_order IS 'ER PURCHASE_ORDER: customer PO evidence plus one staff review';
COMMENT ON VIEW term_evidence IS 'Read-only union of deposit and purchase_order; write to the real tables';
