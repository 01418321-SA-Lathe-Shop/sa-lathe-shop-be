CREATE OR REPLACE FUNCTION protect_unit_conversion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Unit conversions are retained' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' THEN
  IF EXISTS(SELECT 1 FROM material WHERE id=NEW.material_id AND unit_id=NEW.unit_id) THEN
   RAISE EXCEPTION 'Base unit needs no conversion' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
 END IF;
 IF NOT OLD.is_active OR NEW.is_active OR (NEW.id,NEW.material_id,NEW.unit_id,NEW.factor,NEW.created_by,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.material_id,OLD.unit_id,OLD.factor,OLD.created_by,OLD.created_at) THEN
  RAISE EXCEPTION 'Unit conversions can only be deactivated' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION guard_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 IF j.status<>'completed' OR NEW.amount<>j.balance OR NEW.credit_days IS DISTINCT FROM j.invoice_credit_days THEN RAISE EXCEPTION 'Completed job and exact approved terms required' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;

ALTER TABLE delivery DROP CONSTRAINT IF EXISTS delivery_method_check;
ALTER TABLE delivery ADD CONSTRAINT delivery_method_check CHECK(method IN ('self','third_party'));
ALTER TABLE delivery DROP COLUMN IF EXISTS shipping_cost;

CREATE OR REPLACE FUNCTION guard_material_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.supplier_response<>'accepted' OR o.status NOT IN ('ordered','partially_received') OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role IN ('admin','worker')) THEN
  RAISE EXCEPTION 'Order/receipt permission conflict' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION guard_supplier_settlement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.status<>'received' OR o.credit_days IS NULL OR NEW.amount<>o.total_amount OR NEW.paid_at<o.full_received_at OR NEW.paid_at>clock_timestamp()
 OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role='admin') THEN RAISE EXCEPTION 'Full receipt/terms/Admin settlement required' USING ERRCODE='23514'; END IF;
 IF NEW.evidence_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM order_evidence WHERE id=NEW.evidence_id AND order_id=o.id AND state='ready') THEN RAISE EXCEPTION 'Ready order evidence required' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION procurement_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at')
 OR NEW.version<>OLD.version+1 OR OLD.status='cancelled' OR (OLD.credit_days IS NOT NULL AND NEW.credit_days IS DISTINCT FROM OLD.credit_days)
 OR (OLD.full_received_at IS NOT NULL AND NEW.full_received_at IS DISTINCT FROM OLD.full_received_at) THEN RAISE EXCEPTION 'Invalid order edit' USING ERRCODE='23514'; END IF;
 IF NEW.supplier_response IS DISTINCT FROM OLD.supplier_response AND NOT (OLD.status='ordered' AND OLD.supplier_response='pending' AND ((NEW.supplier_response='accepted' AND NEW.status='ordered') OR (NEW.supplier_response='rejected' AND NEW.status='cancelled'))) THEN RAISE EXCEPTION 'Invalid supplier response' USING ERRCODE='23514'; END IF;
 IF NEW.status='cancelled' AND EXISTS(SELECT 1 FROM material_receipt WHERE order_id=OLD.id) THEN RAISE EXCEPTION 'Cannot cancel after any receipt' USING ERRCODE='23514'; END IF;
 IF (NEW.status IN ('partially_received','received') AND NEW.supplier_response<>'accepted') OR (OLD.status='received' AND NEW.status<>'received') OR (OLD.status='partially_received' AND NEW.status NOT IN ('partially_received','received')) THEN RAISE EXCEPTION 'Invalid receipt transition' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION procurement_order_check() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v material_order; line_count int; line_sum numeric; event_action text; full_good boolean;
BEGIN
 SELECT * INTO v FROM material_order WHERE id=NEW.id;
 SELECT count(*),sum(total_amount) INTO line_count,line_sum FROM material_order_item WHERE order_id=v.id;
 IF line_count NOT BETWEEN 1 AND 100 OR line_sum IS DISTINCT FROM v.total_amount THEN RAISE EXCEPTION 'Order total mismatch' USING ERRCODE='23514'; END IF;
 SELECT action INTO event_action FROM material_order_event WHERE order_id=v.id AND version=v.version;
 IF event_action IS NULL OR (v.version=1 AND event_action<>'place') THEN RAISE EXCEPTION 'Order audit required' USING ERRCODE='23514'; END IF;
 IF event_action='receive' THEN
  SELECT bool_and(i.qty_ordered=COALESCE((SELECT sum(good) FROM material_receipt_line l WHERE l.order_id=i.order_id AND l.material_id=i.material_id),0)) INTO full_good FROM material_order_item i WHERE i.order_id=v.id;
  IF v.status<>(CASE WHEN full_good THEN 'received' ELSE 'partially_received' END) THEN RAISE EXCEPTION 'Receipt progress mismatch' USING ERRCODE='23514'; END IF;
 ELSIF event_action IN ('claim','credit_terms','settle') THEN
  IF event_action='settle' AND NOT EXISTS(SELECT 1 FROM supplier_settlement WHERE order_id=v.id) THEN RAISE EXCEPTION 'Settlement missing' USING ERRCODE='23514'; END IF;
 ELSIF v.version>1 AND ((v.status='ordered' AND event_action<>'accept') OR (v.status='cancelled' AND v.supplier_response='rejected' AND event_action<>'reject') OR (v.status='cancelled' AND v.supplier_response<>'rejected' AND event_action<>'cancel')) THEN RAISE EXCEPTION 'Order event mismatch' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;

ALTER TABLE material_order DROP CONSTRAINT IF EXISTS material_order_full_receipt_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_full_receipt_check CHECK((status='received')=(full_received_at IS NOT NULL));
ALTER TABLE material_order DROP CONSTRAINT IF EXISTS material_order_status_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_status_check CHECK(status IN ('ordered','partially_received','received','cancelled'));
ALTER TABLE material_order DROP COLUMN IF EXISTS payment_status;

ALTER TABLE job_request DROP COLUMN IF EXISTS info_requested_at, DROP COLUMN IF EXISTS info_request_note;

ALTER TABLE account DROP CONSTRAINT IF EXISTS account_person_role_unique;
ALTER TABLE account ADD CONSTRAINT staff_account_person_id_key UNIQUE (person_id);

ALTER TABLE customer_address
 DROP COLUMN IF EXISTS is_default,
 DROP COLUMN IF EXISTS postal_code,
 DROP COLUMN IF EXISTS province,
 DROP COLUMN IF EXISTS district,
 DROP COLUMN IF EXISTS address_line2,
 DROP COLUMN IF EXISTS address_line1;
