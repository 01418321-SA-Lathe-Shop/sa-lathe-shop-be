ALTER TABLE job ADD COLUMN fulfillment_version bigint NOT NULL DEFAULT 0 CHECK(fulfillment_version>=0);
ALTER TABLE job ADD COLUMN invoice_credit_days integer CHECK(invoice_credit_days BETWEEN 0 AND 36500);
ALTER TABLE job ADD COLUMN production_cycle bigint NOT NULL DEFAULT 1 CHECK(production_cycle>0);
CREATE TABLE fulfillment_event (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN ('terms','invoice','ship','accept','return','bank','cash','review')),
 version bigint NOT NULL CHECK(version>0),command jsonb NOT NULL,result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(job_id,version)
);
CREATE TABLE invoice (
 id uuid PRIMARY KEY REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 job_id uuid NOT NULL UNIQUE REFERENCES job(id) ON DELETE RESTRICT,
 invoice_no bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 amount numeric(18,2) NOT NULL CHECK(amount>=0 AND amount<>'NaN'::numeric),
 credit_days integer NOT NULL CHECK(credit_days BETWEEN 0 AND 36500),
 issued_at timestamptz NOT NULL DEFAULT clock_timestamp(),due_at timestamptz NOT NULL,
 CHECK(due_at=issued_at+credit_days*interval '24 hours')
);
CREATE TABLE delivery (
 id uuid PRIMARY KEY REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,cycle bigint NOT NULL CHECK(cycle>0),
 method text NOT NULL CHECK(method IN ('self','third_party')),
 receiver_name text NOT NULL CHECK(length(btrim(receiver_name)) BETWEEN 1 AND 200),
 status text NOT NULL DEFAULT 'shipping' CHECK(status IN ('shipping','delivered','returned')),
 shipped_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 decided_at timestamptz,decided_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 decision_event_id uuid UNIQUE REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),UNIQUE(job_id,cycle),
 CHECK((status='shipping' AND decided_at IS NULL AND decided_by IS NULL AND decision_event_id IS NULL AND reason='') OR
 (status<>'shipping' AND decided_at>=shipped_at AND decided_by IS NOT NULL AND decision_event_id IS NOT NULL AND
 ((status='delivered' AND reason='') OR (status='returned' AND length(btrim(reason))>0))))
);
CREATE TABLE production_return (
 id uuid PRIMARY KEY REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 delivery_id uuid NOT NULL UNIQUE REFERENCES delivery(id) ON DELETE RESTRICT,
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,cycle bigint NOT NULL CHECK(cycle>0),
 prior_started_at timestamptz NOT NULL,prior_completed_at timestamptz NOT NULL,
 reopened_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(job_id,cycle),
 CHECK(prior_completed_at>=prior_started_at AND reopened_at>=prior_completed_at)
);
CREATE TABLE customer_evidence (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 uploaded_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 file_name text NOT NULL CHECK(length(file_name) BETWEEN 1 AND 200),
 content_type text NOT NULL CHECK(content_type IN ('application/pdf','application/msword','application/vnd.openxmlformats-officedocument.wordprocessingml.document','application/vnd.ms-excel','application/vnd.openxmlformats-officedocument.spreadsheetml.sheet','text/plain','image/jpeg','image/png')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 100000000),
 staging_key text NOT NULL UNIQUE,object_key text NOT NULL UNIQUE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','checking','ready','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL,completed_at timestamptz,
 CHECK(expires_at>created_at AND staging_key<>object_key),CHECK((state='ready')=(completed_at IS NOT NULL))
);
CREATE INDEX customer_evidence_job_idx ON customer_evidence(job_id,created_at,id);
CREATE TRIGGER customer_evidence_guard BEFORE UPDATE OR DELETE ON customer_evidence FOR EACH ROW EXECUTE FUNCTION guard_order_evidence();
CREATE TABLE customer_payment (
 id uuid PRIMARY KEY REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 invoice_id uuid NOT NULL REFERENCES invoice(id) ON DELETE RESTRICT,
 submitted_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 method text NOT NULL CHECK(method IN ('bank','cash')),
 amount numeric(18,2) NOT NULL CHECK(amount>0 AND amount<>'NaN'::numeric),paid_at timestamptz NOT NULL,
 evidence_id uuid UNIQUE REFERENCES customer_evidence(id) ON DELETE RESTRICT,
 status text NOT NULL CHECK(status IN ('pending','approved','rejected')),
 submitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),reviewed_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 reviewed_at timestamptz,review_event_id uuid UNIQUE REFERENCES fulfillment_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),
 CHECK(paid_at<=submitted_at),CHECK(method<>'bank' OR evidence_id IS NOT NULL),
 CHECK((status='pending' AND method='bank' AND reviewed_by IS NULL AND reviewed_at IS NULL AND review_event_id IS NULL AND reason='') OR
 (status<>'pending' AND reviewed_by IS NOT NULL AND reviewed_at>=submitted_at AND review_event_id IS NOT NULL AND
 ((status='approved' AND reason='') OR(status='rejected' AND length(btrim(reason))>0)))),
 CHECK(method<>'cash' OR (status='approved' AND submitted_by=reviewed_by))
);
CREATE INDEX customer_payment_invoice_idx ON customer_payment(invoice_id,submitted_at,id);
CREATE TABLE payment_allocation (
 payment_id uuid PRIMARY KEY REFERENCES customer_payment(id) ON DELETE RESTRICT,
 invoice_id uuid NOT NULL REFERENCES invoice(id) ON DELETE RESTRICT,
 amount numeric(18,2) NOT NULL CHECK(amount>0 AND amount<>'NaN'::numeric)
);
CREATE INDEX payment_allocation_invoice_idx ON payment_allocation(invoice_id);
CREATE TRIGGER fulfillment_event_immutable BEFORE UPDATE OR DELETE ON fulfillment_event FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER invoice_immutable BEFORE UPDATE OR DELETE ON invoice FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER production_return_immutable BEFORE UPDATE OR DELETE ON production_return FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER payment_allocation_immutable BEFORE UPDATE OR DELETE ON payment_allocation FOR EACH ROW EXECUTE FUNCTION procurement_immutable();

CREATE FUNCTION guard_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 IF j.status<>'completed' OR NEW.amount<>j.balance OR NEW.credit_days IS DISTINCT FROM j.invoice_credit_days THEN RAISE EXCEPTION 'Completed job and exact approved terms required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER invoice_guard BEFORE INSERT ON invoice FOR EACH ROW EXECUTE FUNCTION guard_invoice();
CREATE FUNCTION guard_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Delivery history is immutable' USING ERRCODE='23514'; END IF;
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 IF TG_OP='INSERT' THEN
  IF j.status<>'completed' OR NEW.cycle<>j.production_cycle OR NEW.status<>'shipping' OR NOT EXISTS(SELECT 1 FROM invoice WHERE job_id=j.id) OR EXISTS(SELECT 1 FROM delivery WHERE job_id=j.id AND status<>'returned') THEN RAISE EXCEPTION 'Whole completed job shipment required' USING ERRCODE='23514'; END IF;
 ELSE
  IF OLD.status<>'shipping' OR NEW.status='shipping' OR (to_jsonb(NEW)-'status'-'decided_at'-'decided_by'-'decision_event_id'-'reason') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'decided_at'-'decided_by'-'decision_event_id'-'reason')
  OR NOT EXISTS(SELECT 1 FROM account a JOIN customer_member m ON m.account_id=a.id JOIN job_request r ON r.customer_id=m.customer_id WHERE a.id=NEW.decided_by AND a.is_active AND a.role='customer' AND r.id=j.request_id) THEN RAISE EXCEPTION 'Customer-only immutable inspection required' USING ERRCODE='23514'; END IF;
 END IF;RETURN NEW;
END $$;
CREATE TRIGGER delivery_guard BEFORE INSERT OR UPDATE OR DELETE ON delivery FOR EACH ROW EXECUTE FUNCTION guard_delivery();
CREATE FUNCTION guard_customer_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE job_key uuid; total numeric; received numeric;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Payment history is immutable' USING ERRCODE='23514'; END IF;
 SELECT job_id,amount INTO job_key,total FROM invoice WHERE id=NEW.invoice_id FOR UPDATE;
 IF job_key IS NULL THEN RAISE EXCEPTION 'Invoice required' USING ERRCODE='23514'; END IF;
 IF TG_OP='UPDATE' AND (OLD.status<>'pending' OR NEW.status='pending' OR (to_jsonb(NEW)-'status'-'reviewed_by'-'reviewed_at'-'review_event_id'-'reason') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'reviewed_by'-'reviewed_at'-'review_event_id'-'reason')) THEN RAISE EXCEPTION 'Payment submission/review immutable' USING ERRCODE='23514'; END IF;
 IF NEW.reviewed_by IS NOT NULL AND NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.reviewed_by AND is_active AND role='admin') THEN RAISE EXCEPTION 'Admin review required' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' AND NOT EXISTS(SELECT 1 FROM account a JOIN job j ON j.id=job_key JOIN job_request r ON r.id=j.request_id WHERE a.id=NEW.submitted_by AND a.is_active AND ((NEW.method='cash' AND a.role='admin') OR (NEW.method='bank' AND a.role='customer' AND EXISTS(SELECT 1 FROM customer_member WHERE account_id=a.id AND customer_id=r.customer_id)))) THEN RAISE EXCEPTION 'Payment permission required' USING ERRCODE='23514'; END IF;
 IF NEW.evidence_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM customer_evidence WHERE id=NEW.evidence_id AND job_id=job_key AND uploaded_by=NEW.submitted_by AND state='ready') THEN RAISE EXCEPTION 'Own ready private proof required' USING ERRCODE='23514'; END IF;
 SELECT COALESCE(sum(amount),0) INTO received FROM payment_allocation WHERE invoice_id=NEW.invoice_id;
 IF (TG_OP='INSERT' OR NEW.status='approved') AND NEW.amount>total-received THEN RAISE EXCEPTION 'Payment exceeds remaining balance' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER customer_payment_guard BEFORE INSERT OR UPDATE OR DELETE ON customer_payment FOR EACH ROW EXECUTE FUNCTION guard_customer_payment();
CREATE FUNCTION guard_payment_allocation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p customer_payment; total numeric; received numeric;
BEGIN
 SELECT amount INTO total FROM invoice WHERE id=NEW.invoice_id FOR UPDATE;
 SELECT * INTO p FROM customer_payment WHERE id=NEW.payment_id;
 SELECT COALESCE(sum(amount),0) INTO received FROM payment_allocation WHERE invoice_id=NEW.invoice_id;
 IF p.status<>'approved' OR p.invoice_id<>NEW.invoice_id OR p.amount<>NEW.amount OR received+NEW.amount>total THEN RAISE EXCEPTION 'Exact approved payment allocation required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER payment_allocation_guard BEFORE INSERT ON payment_allocation FOR EACH ROW EXECUTE FUNCTION guard_payment_allocation();
CREATE FUNCTION check_customer_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p customer_payment;
BEGIN
 SELECT * INTO p FROM customer_payment WHERE id=NEW.id;
 IF (p.status='approved') IS DISTINCT FROM EXISTS(SELECT 1 FROM payment_allocation WHERE payment_id=p.id AND invoice_id=p.invoice_id AND amount=p.amount) THEN RAISE EXCEPTION 'Payment allocation audit required' USING ERRCODE='23514'; END IF;
 IF NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=p.id AND actor_id=p.submitted_by AND action=p.method) OR (p.review_event_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=p.review_event_id AND actor_id=p.reviewed_by AND action IN ('review','cash'))) THEN RAISE EXCEPTION 'Payment command audit required' USING ERRCODE='23514'; END IF;RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER customer_payment_consistent AFTER INSERT OR UPDATE ON customer_payment DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_customer_payment();

ALTER FUNCTION guard_production_job() RENAME TO uc8_guard_production_job;
ALTER FUNCTION check_production_job() RENAME TO uc8_check_production_job;
DROP TRIGGER production_job_guard ON job;
DROP TRIGGER production_job_consistent ON job;
ALTER TABLE production_event DROP CONSTRAINT production_event_action_check;
ALTER TABLE production_event ADD CONSTRAINT production_event_action_check CHECK(action IN ('start','use','time','complete','labor','reopen'));
CREATE FUNCTION guard_production_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.status,NEW.production_version,NEW.started_at,NEW.completed_at,NEW.actual_labor_cost,NEW.production_cycle) IS NOT DISTINCT FROM (OLD.status,OLD.production_version,OLD.started_at,OLD.completed_at,OLD.actual_labor_cost,OLD.production_cycle) THEN RETURN NEW;END IF;
 IF NEW.production_version<>OLD.production_version+1 OR (OLD.started_at IS NOT NULL AND NEW.started_at IS DISTINCT FROM OLD.started_at) THEN RAISE EXCEPTION 'Versioned immutable production start required' USING ERRCODE='23514'; END IF;
 IF OLD.status='completed' AND NEW.status='in_progress' THEN
  IF NEW.completed_at IS NOT NULL OR NEW.production_cycle<>OLD.production_cycle+1 OR NEW.actual_labor_cost IS DISTINCT FROM OLD.actual_labor_cost OR NOT EXISTS(SELECT 1 FROM production_return pr JOIN delivery d ON d.id=pr.delivery_id WHERE pr.job_id=NEW.id AND pr.cycle=OLD.production_cycle AND pr.prior_completed_at=OLD.completed_at AND pr.prior_started_at=OLD.started_at AND d.status='returned') THEN RAISE EXCEPTION 'Audited customer return required' USING ERRCODE='23514'; END IF;RETURN NEW;
 END IF;
 IF NEW.production_cycle<>OLD.production_cycle OR (OLD.completed_at IS NOT NULL AND NEW.completed_at IS DISTINCT FROM OLD.completed_at) THEN RAISE EXCEPTION 'Immutable production cycle required' USING ERRCODE='23514'; END IF;
 IF OLD.status='queued' AND NEW.status='in_progress' THEN
  IF NEW.started_at IS NULL OR NEW.completed_at IS NOT NULL OR NOT EXISTS(SELECT 1 FROM job_material_requirement WHERE job_id=NEW.id AND required_qty>0) OR EXISTS(SELECT 1 FROM job_material_requirement q LEFT JOIN material_reservation r USING(job_id,material_id) WHERE q.job_id=NEW.id AND q.required_qty>COALESCE(r.qty_reserved,0)) THEN RAISE EXCEPTION 'Fully reserved plan required' USING ERRCODE='23514'; END IF;
 ELSIF OLD.status='in_progress' AND NEW.status='completed' THEN
  IF NEW.completed_at IS NULL OR NEW.completed_at<NEW.started_at OR EXISTS(SELECT 1 FROM material_reservation WHERE job_id=NEW.id AND qty_reserved>0) THEN RAISE EXCEPTION 'Completion must release leftovers' USING ERRCODE='23514'; END IF;
 ELSIF OLD.status IN ('in_progress','completed') AND NEW.status=OLD.status THEN NULL;
 ELSE RAISE EXCEPTION 'Unsupported production transition' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;
CREATE FUNCTION check_production_job() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;e production_event;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.id;IF j.production_version=0 THEN RETURN NULL;END IF;
 SELECT * INTO e FROM production_event WHERE job_id=j.id AND version=j.production_version;
 IF e.id IS NULL OR NOT EXISTS(SELECT 1 FROM account WHERE id=e.actor_id AND ((e.action='labor' AND role='admin') OR(e.action='reopen' AND role='customer') OR(e.action NOT IN ('labor','reopen') AND role='worker'))) THEN RAISE EXCEPTION 'Production audit/role required' USING ERRCODE='23514';END IF;
 IF (e.action IN ('start','use','time','reopen') AND j.status<>'in_progress') OR(e.action='complete' AND j.status<>'completed') THEN RAISE EXCEPTION 'Production status/event mismatch' USING ERRCODE='23514'; END IF;
 IF e.action='reopen' AND NOT EXISTS(SELECT 1 FROM production_return WHERE id=e.id AND job_id=j.id AND cycle=j.production_cycle-1) THEN RAISE EXCEPTION 'Return audit required' USING ERRCODE='23514'; END IF;
 IF e.action='time' AND NOT EXISTS(SELECT 1 FROM job_time_entry WHERE id=e.id AND job_id=j.id AND actor_id=e.actor_id) THEN RAISE EXCEPTION 'Time audit required' USING ERRCODE='23514'; END IF;
 IF e.action='use' AND NOT EXISTS(SELECT 1 FROM job_material_usage WHERE event_id=e.id AND job_id=j.id) THEN RAISE EXCEPTION 'Usage audit required' USING ERRCODE='23514'; END IF;RETURN NULL;
END $$;
CREATE TRIGGER production_job_guard BEFORE UPDATE ON job FOR EACH ROW EXECUTE FUNCTION guard_production_job();
CREATE CONSTRAINT TRIGGER production_job_consistent AFTER UPDATE ON job DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_production_job();
CREATE FUNCTION guard_fulfillment_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.id,NEW.request_id,NEW.quotation_id,NEW.deposit_amount,NEW.balance) IS DISTINCT FROM (OLD.id,OLD.request_id,OLD.quotation_id,OLD.deposit_amount,OLD.balance) THEN RAISE EXCEPTION 'Job contract snapshots are immutable' USING ERRCODE='23514'; END IF;
 IF (NEW.fulfillment_version,NEW.invoice_credit_days) IS DISTINCT FROM (OLD.fulfillment_version,OLD.invoice_credit_days) THEN
  IF NEW.fulfillment_version<>OLD.fulfillment_version+1 OR (OLD.invoice_credit_days IS NOT NULL AND NEW.invoice_credit_days IS DISTINCT FROM OLD.invoice_credit_days) THEN RAISE EXCEPTION 'Versioned immutable invoice terms required' USING ERRCODE='23514'; END IF;
 END IF;RETURN NEW;
END $$;
CREATE TRIGGER fulfillment_job_guard BEFORE UPDATE ON job FOR EACH ROW EXECUTE FUNCTION guard_fulfillment_job();
CREATE FUNCTION check_fulfillment_job() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;e fulfillment_event;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.id;IF j.fulfillment_version=0 THEN RETURN NULL;END IF;
 SELECT * INTO e FROM fulfillment_event WHERE job_id=j.id AND version=j.fulfillment_version;
 IF e.id IS NULL OR NOT EXISTS(SELECT 1 FROM account WHERE id=e.actor_id AND ((e.action IN ('terms','cash','review') AND role='admin') OR(e.action IN ('invoice','ship') AND role IN ('admin','worker')) OR(e.action IN ('accept','return','bank') AND role='customer'))) THEN RAISE EXCEPTION 'Fulfillment audit/role required' USING ERRCODE='23514';END IF;
 IF (e.action='invoice' AND NOT EXISTS(SELECT 1 FROM invoice WHERE id=e.id AND job_id=j.id)) OR(e.action='ship' AND NOT EXISTS(SELECT 1 FROM delivery WHERE id=e.id AND job_id=j.id)) OR(e.action IN ('accept','return') AND NOT EXISTS(SELECT 1 FROM delivery WHERE decision_event_id=e.id AND job_id=j.id AND decided_by=e.actor_id AND status=(CASE WHEN e.action='accept' THEN 'delivered' ELSE 'returned' END))) THEN RAISE EXCEPTION 'Fulfillment detail audit required' USING ERRCODE='23514'; END IF;RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER fulfillment_job_consistent AFTER UPDATE ON job DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fulfillment_job();
CREATE FUNCTION check_fulfillment_detail() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='invoice' THEN
  IF NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=NEW.id AND job_id=NEW.job_id AND action='invoice') THEN RAISE EXCEPTION 'Invoice command audit required' USING ERRCODE='23514'; END IF;
 ELSIF TG_TABLE_NAME='delivery' THEN
  IF NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=NEW.id AND job_id=NEW.job_id AND action='ship') OR (NEW.decision_event_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM fulfillment_event WHERE id=NEW.decision_event_id AND job_id=NEW.job_id AND actor_id=NEW.decided_by AND action=(CASE WHEN NEW.status='returned' THEN 'return' ELSE 'accept' END))) THEN RAISE EXCEPTION 'Delivery command audit required' USING ERRCODE='23514'; END IF;
 ELSIF TG_TABLE_NAME='production_return' THEN
  IF NOT EXISTS(SELECT 1 FROM delivery d JOIN fulfillment_event e ON e.id=NEW.id WHERE d.id=NEW.delivery_id AND d.job_id=NEW.job_id AND d.cycle=NEW.cycle AND d.status='returned' AND d.decision_event_id=NEW.id AND e.job_id=NEW.job_id AND e.action='return') THEN RAISE EXCEPTION 'Customer rework audit required' USING ERRCODE='23514'; END IF;
 END IF;RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER invoice_consistent AFTER INSERT ON invoice DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fulfillment_detail();
CREATE CONSTRAINT TRIGGER delivery_consistent AFTER INSERT OR UPDATE ON delivery DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fulfillment_detail();
CREATE CONSTRAINT TRIGGER production_return_consistent AFTER INSERT ON production_return DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fulfillment_detail();
CREATE FUNCTION guard_request_closure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.status='completed' AND NEW.status<>'completed' THEN RAISE EXCEPTION 'Accepted paid request cannot reopen' USING ERRCODE='23514'; END IF;
 IF NEW.status='completed' AND OLD.status<>'completed' AND NOT EXISTS(SELECT 1 FROM job j JOIN invoice i ON i.job_id=j.id JOIN delivery d ON d.job_id=j.id AND d.cycle=j.production_cycle WHERE j.request_id=NEW.id AND j.status='completed' AND d.status='delivered' AND i.amount=COALESCE((SELECT sum(amount) FROM payment_allocation WHERE invoice_id=i.id),0)) THEN RAISE EXCEPTION 'Customer acceptance and full payment required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;
CREATE TRIGGER request_closure_guard BEFORE UPDATE OF status ON job_request FOR EACH ROW EXECUTE FUNCTION guard_request_closure();

ALTER TABLE outbox DROP CONSTRAINT outbox_request_id_channel_key;
ALTER TABLE outbox ADD COLUMN kind text NOT NULL DEFAULT 'request' CHECK(kind IN ('request','delivery','inspection','payment','review'));
ALTER TABLE outbox ADD COLUMN audience text NOT NULL DEFAULT 'request' CHECK(audience IN ('request','customer','admin','target'));
ALTER TABLE outbox ADD COLUMN target_account_id uuid REFERENCES account(id) ON DELETE RESTRICT;
ALTER TABLE outbox ADD CONSTRAINT outbox_target_check CHECK((audience='target')=(target_account_id IS NOT NULL));
CREATE UNIQUE INDEX outbox_request_channel_unique ON outbox(request_id,channel) WHERE kind='request';
ALTER TABLE outbox ADD CONSTRAINT outbox_uc9_in_system_check CHECK(kind='request' OR channel='in_system');
COMMENT ON TABLE invoice IS 'One exact post-deposit invoice per job; paid/balance/status derived from immutable approved allocations; no VAT.';
COMMENT ON TABLE production_return IS 'Immutable completed-cycle snapshot before free same-job rework; never restores material stock or refunds.';
COMMENT ON TABLE customer_evidence IS 'Private customer-payment proof, uploader/Admin only; never supplier or production-photo access.';
