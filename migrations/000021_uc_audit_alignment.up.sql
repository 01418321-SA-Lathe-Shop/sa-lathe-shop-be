-- UC1, UC2, UC7, UC9, UC10, UC11, UC12 alignment with Use_Case_Description_UC1-UC12.md.

-- H-01 (UC1): Structured customer address columns + is_default flag.
ALTER TABLE customer_address
 ADD COLUMN address_line1 text NOT NULL DEFAULT '' CHECK(length(address_line1)<=300),
 ADD COLUMN address_line2 text NOT NULL DEFAULT '' CHECK(length(address_line2)<=300),
 ADD COLUMN district text NOT NULL DEFAULT '' CHECK(length(district)<=150),
 ADD COLUMN province text NOT NULL DEFAULT '' CHECK(length(province)<=150),
 ADD COLUMN postal_code text NOT NULL DEFAULT '' CHECK(postal_code='' OR postal_code ~ '^[0-9]{5}$'),
 ADD COLUMN is_default boolean NOT NULL DEFAULT false;

-- H-05 (UC11 & UC12): Allow an existing person (e.g. a customer) to be linked as a worker or admin account.
ALTER TABLE account DROP CONSTRAINT IF EXISTS staff_account_person_id_key;
ALTER TABLE account ADD CONSTRAINT account_person_role_unique UNIQUE (person_id, role);

-- M-02 (UC2 Alt-Step 5): Allow Worker to request additional info on a job request.
ALTER TABLE job_request
 ADD COLUMN info_request_note text NOT NULL DEFAULT '' CHECK(length(info_request_note)<=2000),
 ADD COLUMN info_requested_at timestamptz;

-- C-05 & M-05 (UC7): Support 'damaged' order status, completion when SUM(good+damaged) >= qty_ordered,
-- and supplier payment status ('unpaid','partial','paid') without requiring credit_days upfront.
ALTER TABLE material_order ADD COLUMN payment_status text NOT NULL DEFAULT 'unpaid' CHECK(payment_status IN ('unpaid','partial','paid'));
ALTER TABLE material_order DROP CONSTRAINT material_order_status_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_status_check CHECK(status IN ('ordered','partially_received','received','damaged','cancelled'));
ALTER TABLE material_order DROP CONSTRAINT material_order_full_receipt_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_full_receipt_check CHECK((status IN ('received','damaged'))=(full_received_at IS NOT NULL));

CREATE OR REPLACE FUNCTION guard_material_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.supplier_response<>'accepted' OR o.status NOT IN ('ordered','partially_received','received','damaged') OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role IN ('admin','worker')) THEN
  RAISE EXCEPTION 'Order/receipt permission conflict' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION guard_supplier_settlement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.status NOT IN ('received','partially_received','damaged') OR NEW.amount<>o.total_amount OR (o.full_received_at IS NOT NULL AND NEW.paid_at<o.full_received_at) OR NEW.paid_at>clock_timestamp()
 OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role='admin') THEN RAISE EXCEPTION 'Full receipt/terms/Admin settlement required' USING ERRCODE='23514'; END IF;
 IF NEW.evidence_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM order_evidence WHERE id=NEW.evidence_id AND order_id=o.id AND state='ready') THEN RAISE EXCEPTION 'Ready order evidence required' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION procurement_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status')
 OR NEW.version<>OLD.version+1 OR OLD.status='cancelled' OR (OLD.credit_days IS NOT NULL AND NEW.credit_days IS DISTINCT FROM OLD.credit_days)
 OR (OLD.full_received_at IS NOT NULL AND NEW.full_received_at IS DISTINCT FROM OLD.full_received_at) THEN RAISE EXCEPTION 'Invalid order edit' USING ERRCODE='23514'; END IF;
 IF NEW.supplier_response IS DISTINCT FROM OLD.supplier_response AND NOT (OLD.status='ordered' AND OLD.supplier_response='pending' AND ((NEW.supplier_response='accepted' AND NEW.status='ordered') OR (NEW.supplier_response='rejected' AND NEW.status='cancelled'))) THEN RAISE EXCEPTION 'Invalid supplier response' USING ERRCODE='23514'; END IF;
 IF NEW.status='cancelled' AND EXISTS(SELECT 1 FROM material_receipt WHERE order_id=OLD.id) THEN RAISE EXCEPTION 'Cannot cancel after any receipt' USING ERRCODE='23514'; END IF;
 IF (NEW.status IN ('partially_received','received','damaged') AND NEW.supplier_response<>'accepted')
  OR (OLD.status='received' AND NEW.status<>'received')
  OR (OLD.status='damaged' AND NEW.status NOT IN ('damaged','received'))
  OR (OLD.status='partially_received' AND NEW.status NOT IN ('partially_received','received','damaged')) THEN RAISE EXCEPTION 'Invalid receipt transition' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION procurement_order_check() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v material_order; line_count int; line_sum numeric; event_action text; full_delivered boolean; any_good boolean;
BEGIN
 SELECT * INTO v FROM material_order WHERE id=NEW.id;
 SELECT count(*),sum(total_amount) INTO line_count,line_sum FROM material_order_item WHERE order_id=v.id;
 IF line_count NOT BETWEEN 1 AND 100 OR line_sum IS DISTINCT FROM v.total_amount THEN RAISE EXCEPTION 'Order total mismatch' USING ERRCODE='23514'; END IF;
 SELECT action INTO event_action FROM material_order_event WHERE order_id=v.id AND version=v.version;
 IF event_action IS NULL OR (v.version=1 AND event_action<>'place') THEN RAISE EXCEPTION 'Order audit required' USING ERRCODE='23514'; END IF;
 IF event_action='receive' THEN
  SELECT bool_and(i.qty_ordered<=COALESCE((SELECT sum(good+damaged) FROM material_receipt_line l WHERE l.order_id=i.order_id AND l.material_id=i.material_id AND l.replacement_claim_id IS NULL),0)) INTO full_delivered FROM material_order_item i WHERE i.order_id=v.id;
  SELECT EXISTS(SELECT 1 FROM material_receipt_line l WHERE l.order_id=v.id AND l.good>0) INTO any_good;
  IF v.status<>(CASE WHEN NOT full_delivered THEN 'partially_received' WHEN NOT any_good THEN 'damaged' ELSE 'received' END) THEN RAISE EXCEPTION 'Receipt progress mismatch' USING ERRCODE='23514'; END IF;
 ELSIF event_action IN ('claim','credit_terms','settle') THEN
  IF event_action='settle' AND NOT EXISTS(SELECT 1 FROM supplier_settlement WHERE order_id=v.id) THEN RAISE EXCEPTION 'Settlement missing' USING ERRCODE='23514'; END IF;
 ELSIF v.version>1 AND ((v.status='ordered' AND event_action<>'accept') OR (v.status='cancelled' AND v.supplier_response='rejected' AND event_action<>'reject') OR (v.status='cancelled' AND v.supplier_response<>'rejected' AND event_action<>'cancel')) THEN RAISE EXCEPTION 'Order event mismatch' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;

-- M-06 (UC9): Add shipping_cost and 'pickup' method on delivery, and allow issuing invoice with default credit_days = 0.
ALTER TABLE delivery ADD COLUMN shipping_cost numeric(18,2) NOT NULL DEFAULT 0 CHECK(shipping_cost>=0 AND shipping_cost<>'NaN'::numeric);
DO $$ DECLARE c record; BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='delivery'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%method%' LOOP
  EXECUTE format('ALTER TABLE delivery DROP CONSTRAINT %I',c.conname);
 END LOOP;
END $$;
ALTER TABLE delivery ADD CONSTRAINT delivery_method_check CHECK(method IN ('self','third_party','pickup') AND (method<>'pickup' OR shipping_cost=0));

CREATE OR REPLACE FUNCTION guard_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 IF j.status<>'completed' OR NEW.amount<>j.balance OR NEW.credit_days IS DISTINCT FROM COALESCE(j.invoice_credit_days,0) THEN RAISE EXCEPTION 'Completed job and exact approved terms required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION guard_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
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

-- H-04 (UC10 Q10.5): Allow updating unit_conversion.factor while active, in addition to deactivation.
CREATE OR REPLACE FUNCTION protect_unit_conversion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Unit conversions are retained' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' THEN
  IF EXISTS(SELECT 1 FROM material WHERE id=NEW.material_id AND unit_id=NEW.unit_id) THEN
   RAISE EXCEPTION 'Base unit needs no conversion' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
 END IF;
 IF NOT OLD.is_active OR (NEW.id,NEW.material_id,NEW.unit_id,NEW.created_by,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.material_id,OLD.unit_id,OLD.created_by,OLD.created_at)
  OR (NEW.is_active AND NEW.factor IS DISTINCT FROM OLD.factor AND NEW.factor<=0)
  OR (NOT NEW.is_active AND NEW.factor IS DISTINCT FROM OLD.factor) THEN
  RAISE EXCEPTION 'Unit conversions can only update factor while active or be deactivated' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

